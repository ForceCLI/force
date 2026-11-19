package sf

import (
	"fmt"
	"math"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strconv"
	"strings"
	"time"

	"github.com/spf13/cobra"

	"github.com/ForceCLI/force/lib"
)

// tracksSource reports whether deploys to and retrieves from the org update
// source tracking.  The authorization file records the answer.  Without
// one, a scratch org tracks source, a sandbox does when it has the
// SourceMember object, and another org does not; the answer is then
// recorded, as sf records it.
func tracksSource(org *targetOrg) bool {
	if value, ok := org.Fields["tracksSource"].(bool); ok {
		return value
	}
	tracks := false
	switch {
	case org.Fields.Bool("isScratch") || org.Fields.String("devHubUsername") != "":
		tracks = true
	case org.Fields.Bool("isSandbox"):
		tracks, _ = supportsSourceTracking(org)
	}
	lib.UpdateSFDXAuthFields(org.Username, lib.SFDXAuthFields{"tracksSource": tracks})
	return tracks
}

// trackingPollInterval is the time between queries for the source members
// of a deploy.
var trackingPollInterval = time.Second

// componentKeys returns the keys of the components a source set deploys.
func (s *sourceSet) componentKeys() []string {
	var keys []string
	for metadataType, members := range s.types {
		for _, member := range members {
			keys = append(keys, metadataType+"###"+member)
		}
	}
	sort.Strings(keys)
	return keys
}

// conflictRows lists the files of the components in keys that changed both
// locally and in the org: the -meta.xml file of each component first.
func (s *trackingState) conflictRows(root string, keys []string) []*orderedMap {
	conflicts := s.conflicts()
	rows := []*orderedMap{}
	seen := map[string]bool{}
	for _, key := range keys {
		component := s.components[key]
		if component == nil || !conflicts[key] || seen[key] {
			continue
		}
		seen[key] = true
		files := []string{}
		if component.xml != "" {
			files = append(files, component.xml)
		}
		for _, file := range component.files {
			if file != component.xml && s.onDisk[file] {
				files = append(files, file)
			}
		}
		for _, file := range files {
			row := newOrderedMap()
			row.Set("state", "Conflict")
			row.Set("fullName", component.FullName)
			row.Set("type", component.Type)
			row.Set("filePath", filepath.Join(root, filepath.FromSlash(file)))
			rows = append(rows, row)
		}
	}
	return rows
}

// conflictError prints the table of conflicts and returns the error sf
// reports for a deploy or a retrieve that would overwrite changes.
func conflictError(cmd *cobra.Command, rows []*orderedMap, deploy bool) error {
	if !jsonRequested(cmd) {
		table := make([][]string, 0, len(rows))
		for _, row := range rows {
			table = append(table, []string{text(row.values["state"]), text(row.values["fullName"]), text(row.values["type"]), text(row.values["filePath"])})
		}
		printTable(cmd.OutOrStdout(), "", []string{"STATE", "FULL NAME", "TYPE", "FILE PATH"}, table)
		message := "There are changes in your local files that conflict with the org changes you're trying to retrieve."
		actions := []string{
			"To overwrite the local changes, rerun this command with the --ignore-conflicts flag.",
			"To overwrite the remote changes, run the \"" + binName + " project deploy start\" command with the --ignore-conflicts flag.",
		}
		if deploy {
			message = "There are changes in the org that conflict with the local changes you're trying to deploy."
			actions = []string{
				"To overwrite the remote changes, rerun this command with the --ignore-conflicts flag.",
				"To overwrite the local changes, run the \"" + binName + " project retrieve start\" command with the --ignore-conflicts flag.",
			}
		}
		return &Error{Name: "SourceConflictError", Label: "1", Message: message, ExitCode: 1, Actions: actions}
	}
	return &Error{Name: "SourceConflictError", Message: fmt.Sprintf("%d conflicts detected", len(rows)), ExitCode: 1, Data: rows}
}

// localChangeSet is what a deploy without named components deploys: the
// components with added or modified files, and the components whose files
// were all deleted.
type localChangeSet struct {
	deploy, remove []*sourceComponent
}

// localChangeSet returns the components to deploy and to delete, leaving
// out the files .forceignore excludes.
func (s *trackingState) localChangeSet(root string, ignore *forceIgnore) *localChangeSet {
	set := &localChangeSet{}
	seen := map[string]bool{}
	for _, file := range s.changes.deleted {
		if component := s.componentOf(file); component != nil && !s.exists(component) && !seen[component.key()] {
			seen[component.key()] = true
			set.remove = append(set.remove, component)
		}
	}
	changed := append(append(append([]string{}, s.changes.added...), s.changes.modified...), s.changes.deleted...)
	sort.Strings(changed)
	for _, file := range changed {
		component := s.componentOf(file)
		if component == nil || seen[component.key()] {
			continue
		}
		if s.onDisk[file] && ignore.ignores(filepath.Join(root, filepath.FromSlash(file))) {
			continue
		}
		if !s.exists(component) {
			continue
		}
		seen[component.key()] = true
		set.deploy = append(set.deploy, component)
	}
	return set
}

// metadataName returns the name by which a component is deployed.  A field
// or another part of a decomposed custom object is deployed with its
// object.
func (c *sourceComponent) metadataName() string {
	for _, childType := range objectChildTypes {
		if c.Type == childType {
			object, _, _ := strings.Cut(c.FullName, ".")
			return "CustomObject:" + object
		}
	}
	return c.Type + ":" + c.FullName
}

// destructiveChanges returns the manifest that deletes components.
func destructiveChanges(components []*sourceComponent, version string) []byte {
	types := map[string][]string{}
	for _, component := range components {
		types[component.Type] = append(types[component.Type], component.FullName)
	}
	return packageXML(types, version)
}

// trackingProgress is the outcome of the source tracking update of a
// deploy, for the progress display.
type trackingProgress struct {
	found, total int
	elapsed      time.Duration
	// warning is the warning sf gives when the org did not report every
	// member in time, or "".
	warning string
}

// recordDeploy records a successful deploy: the files of the deployed
// components are in sync, the files of the deleted components are gone,
// and the revision the org gave each component is the one last retrieved.
// It waits for the org to report the source members of the components, for
// the time sf waits.
func (t *sourceTracking) recordDeploy(state *trackingState, deployed, deleted []string, unchanged ...string) (*trackingProgress, error) {
	started := time.Now()
	repo, err := openShadowRepo(t.gitDir(), t.project.Root)
	if err != nil {
		return nil, err
	}
	var updated, removed []string
	for _, key := range deployed {
		if component := state.components[key]; component != nil {
			for _, file := range component.files {
				if state.onDisk[file] {
					updated = append(updated, file)
				} else {
					removed = append(removed, file)
				}
			}
		}
	}
	for _, key := range deleted {
		if component := state.components[key]; component != nil {
			removed = append(removed, component.files...)
		}
	}
	if err := t.commitLocal(repo, updated, removed, "sfdx source tracking"); err != nil {
		return nil, err
	}
	keys := append(append([]string{}, deployed...), deleted...)
	// The org is expected to report the components that changed, except
	// the ones it does not track.
	skip := map[string]bool{}
	for _, key := range unchanged {
		skip[key] = true
	}
	expected := []string{}
	for _, key := range keys {
		if !skip[key] && expectsSourceMember(key) {
			expected = append(expected, key)
		}
	}
	found, warning, err := t.syncRemote(keys, expected)
	if err != nil {
		return nil, err
	}
	return &trackingProgress{found: found, total: len(expected), elapsed: time.Since(started), warning: warning}, nil
}

// Types and components for which sf does not wait for a source member
// after a deploy, because the org does not report one or reports it under
// another name.
var (
	typesWithoutSourceMembers = map[string]bool{
		"CustomObject": true, "EmailFolder": true, "EmailTemplateFolder": true, "StandardValueSet": true, "Portal": true,
		"StandardValueSetTranslation": true, "SharingRules": true, "SharingCriteriaRule": true, "GlobalValueSetTranslation": true,
		"AssignmentRules": true, "InstalledPackage": true, "DataCategoryGroup": true, "ManagedContentType": true,
		"CustomObjectTranslation": true, "TopicsForObjects": true, "UIBundle": true,
	}
	typesWithoutSourceMembersInNamespace = map[string]bool{"CustomLabels": true, "CustomMetadata": true, "DuplicateRule": true, "WebLink": true}
	typesWithEncodedNames                = map[string]bool{"Layout": true, "Profile": true, "HomePageComponent": true, "HomePageLayout": true, "MilestoneType": true}
	componentsWithoutSourceMembers       = map[string]bool{
		"AppMenu###Salesforce1": true, "Profile###Standard": true, "Profile###Guest License User": true, "CustomTab###standard-home": true,
		"Profile###Minimum Access - Salesforce": true, "Profile###Salesforce API Only System Integrations": true, "AssignmentRules###Case": true,
		"ListView###CollaborationGroup.All_ChatterGroups": true, "CustomTab###standard-mailapp": true, "ApexEmailNotifications###apexEmailNotifications": true,
	}
)

// encodedNamePattern matches a component name that the Metadata API
// writes with percent-encoded characters in the file name: one with a
// character other than a letter, a digit, a space, an underscore or a
// hyphen.
var encodedNamePattern = regexp.MustCompile(`[^\p{L}\p{N} _-]`)

// expectsSourceMember reports whether sf waits for the org to report the
// source member of a deployed component.  sf decides some cases by the
// path of the file; this decides them by the name of the component.
func expectsSourceMember(key string) bool {
	metadataType, name, _ := strings.Cut(key, "###")
	switch {
	case typesWithoutSourceMembers[metadataType], componentsWithoutSourceMembers[key]:
		return false
	case metadataType == "CustomField" && (!strings.Contains(name, "__c") || strings.Contains(name, "_del__c")):
		return false
	case metadataType == "ReportType" && strings.Contains(name, "screen_flows_prebuilt_crt"):
		return false
	case strings.Contains(metadataType, "Settings"), strings.HasPrefix(metadataType, "Workflow"):
		return false
	case metadataType == "NavigationMenu" && strings.HasPrefix(name, "SFDC_Default_Navigation_"):
		return false
	case typesWithEncodedNames[metadataType] && encodedNamePattern.MatchString(name):
		return false
	case typesWithoutSourceMembersInNamespace[metadataType] && strings.Contains(name, "__"):
		return false
	}
	return true
}

// sourceMemberPollingTimeout is the time to wait for the org to report a
// number of source members: 50 seconds for each 1000 members plus 5
// seconds, or the seconds in SF_SOURCE_MEMBER_POLLING_TIMEOUT.
func sourceMemberPollingTimeout(members int) time.Duration {
	if seconds, err := strconv.Atoi(os.Getenv("SF_SOURCE_MEMBER_POLLING_TIMEOUT")); err == nil && seconds > 0 {
		return time.Duration(seconds) * time.Second
	}
	return time.Duration(math.Ceil(float64(members)*0.05)+5) * time.Second
}

// syncRemote asks the org for the source members changed since the last
// query, records them, and marks the members in keys as retrieved at their
// current revision.  It repeats the query until the org has reported every
// member in expected or the time sf allows has passed.  It returns the
// number of members in expected that the org reported, and the warning sf
// gives when the org did not report all of them in time.
func (t *sourceTracking) syncRemote(keys, expected []string) (int, string, error) {
	tracking, err := loadRemoteTracking(t.revisionPath())
	if err != nil {
		return 0, "", err
	}
	wait := len(expected) > 0
	if value := strings.ToLower(os.Getenv("SF_DISABLE_SOURCE_MEMBER_POLLING")); value == "true" || value == "1" {
		wait = false
	}
	outstanding := map[string]bool{}
	for _, key := range expected {
		outstanding[key] = true
	}
	deadline := time.Now().Add(sourceMemberPollingTimeout(len(expected)))
	attempts, emptyResults := 0, 0
	for {
		members, err := querySourceMembers(t.org, fmt.Sprintf("RevisionCounter > %d", tracking.maxRevision))
		if err != nil {
			return 0, "", err
		}
		attempts++
		if len(members) == 0 {
			emptyResults++
		} else {
			emptyResults = 0
		}
		for _, member := range members {
			delete(outstanding, member.key())
		}
		if err := tracking.track(members, false); err != nil {
			return 0, "", err
		}
		if !wait || len(outstanding) == 0 || time.Now().Add(trackingPollInterval).After(deadline) {
			break
		}
		time.Sleep(trackingPollInterval)
	}
	for _, key := range keys {
		if member := tracking.members[key]; member != nil {
			revision := member.RevisionCounter
			member.LastRetrieved = &revision
		}
	}
	warning := ""
	if wait && len(outstanding) > 0 {
		// The missing members by type, in the order they were asked for.
		var types []string
		names := map[string][]string{}
		for _, key := range expected {
			if !outstanding[key] {
				continue
			}
			outstanding[key] = false
			metadataType, name, _ := strings.Cut(key, "###")
			if _, seen := names[metadataType]; !seen {
				types = append(types, metadataType)
			}
			names[metadataType] = append(names[metadataType], name)
		}
		lines := make([]string, 0, len(types))
		missing := 0
		for _, metadataType := range types {
			missing += len(names[metadataType])
			lines = append(lines, fmt.Sprintf("  - %s: %s", metadataType, strings.Join(names[metadataType], ", ")))
		}
		warning = fmt.Sprintf("Polling for %d SourceMembers timed out after %d attempts (last %d were empty).\n\nMissing SourceMembers:\n%s", missing, attempts, emptyResults, strings.Join(lines, "\n"))
		return len(expected) - missing, warning, tracking.write()
	}
	return len(expected) - len(outstanding), warning, tracking.write()
}

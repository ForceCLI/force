package sf

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/url"
	"os"
	"path/filepath"
	"sort"
	"strconv"
	"strings"
	"time"

	"github.com/spf13/cobra"

	"github.com/ForceCLI/force/lib"
)

// Source tracking records which local files and which components of an org
// are in sync, in the files sf uses, so that sf and this mode can be used on
// the same project:
//
//   - <project>/.sf/orgs/<org id>/localSourceTracking is a git repository
//     whose last commit holds the local files as they were when they were
//     last deployed or retrieved (see shadowRepo).
//   - <project>/.sf/orgs/<org id>/maxRevision.json lists the SourceMember
//     records of the org, each with its revision in the org and the revision
//     that was last retrieved.

// supportsSourceTracking reports whether the org has the SourceMember
// object.  sf does not ask a scratch org.
func supportsSourceTracking(org *targetOrg) (bool, error) {
	if scratch, _ := org.Fields["isScratch"].(bool); scratch {
		return true, nil
	}
	if _, err := org.Force.GetREST("/tooling/sobjects/SourceMember/describe"); err != nil {
		if strings.Contains(err.Error(), "The requested resource does not exist") {
			return false, nil
		}
		return false, apiError(err)
	}
	return true, nil
}

func newOrgTrackingCmd(enable bool) *cobra.Command {
	short, className, done := "Prevent Salesforce CLI from tracking changes in your source files between your project and an org.", "OrgDisableTracking", "Disabled"
	if enable {
		short, className, done = "Allow Salesforce CLI to track changes in your source files between your project and an org.", "OrgEnableTracking", "Enabled"
	}
	cmd := &cobra.Command{
		Use:         "tracking",
		Short:       short,
		Args:        cobra.NoArgs,
		Annotations: map[string]string{commandNameAnnotation: className},
		RunE: func(cmd *cobra.Command, args []string) error {
			org, err := resolveTargetOrg(cmd)
			if err != nil {
				return err
			}
			if enable {
				supported, err := supportsSourceTracking(org)
				if err != nil {
					return err
				}
				if !supported {
					return &Error{
						Name:     "TrackingNotAvailableError",
						Message:  "You can't enable source tracking on this org because the SourceMember Tooling API object isn't available, or you don't have access to it.",
						ExitCode: 1,
						Actions: []string{
							"- If the org is a Developer or Developer Pro sandbox, make sure that the associated production org has enabled source tracking in sandboxes. See https://developer.salesforce.com/docs/atlas.en-us.sfdx_dev.meta/sfdx_dev/sfdx_setup_enable_source_tracking_sandboxes.htm.",
							"- Make sure that your user can access the SourceMember Tooling API object.",
							"- You can't enable source tracking on Developer Edition orgs, production orgs, Partial Copy sandboxes, or Full sandboxes.",
						},
					}
				}
			}
			if err := lib.UpdateSFDXAuthFields(org.Username, lib.SFDXAuthFields{"tracksSource": enable}); err != nil {
				return err
			}
			result := newOrderedMap()
			result.Set("tracksSource", enable)
			result.Set("username", org.Username)
			return emit(cmd, result, func(w io.Writer) {
				fmt.Fprintln(w, paint(w, styleGreen, fmt.Sprintf("%s source tracking for %s.", done, org.Username)))
			})
		},
	}
	addJSONFlag(cmd)
	addTargetOrgFlag(cmd)
	return cmd
}

// sourceMember is a component of an org in maxRevision.json.
type sourceMember struct {
	Type, Name      string
	RevisionCounter int
	// LastRetrieved is the revision that was last deployed or retrieved,
	// or nil when the component has not been.
	LastRetrieved  *int
	IsNameObsolete bool
	// The fields below come from a query and are not stored in the file.
	IsNewMember      bool
	ChangedBy        string
	MemberIdOrName   string
	LastModifiedDate string
}

func (m *sourceMember) key() string {
	return m.Type + "###" + m.Name
}

// inSync reports whether the revision of the org was the one last deployed
// or retrieved.
func (m *sourceMember) inSync() bool {
	return m.LastRetrieved != nil && *m.LastRetrieved == m.RevisionCounter
}

// remoteTracking is the content of maxRevision.json.
type remoteTracking struct {
	path        string
	maxRevision int
	keys        []string
	members     map[string]*sourceMember
}

// loadRemoteTracking reads maxRevision.json, in the format sf writes by
// default (fileVersion 0) or in the newer format it also reads (fileVersion
// 1).  It writes an empty file when there is none, as sf does.
func loadRemoteTracking(path string) (*remoteTracking, error) {
	tracking := &remoteTracking{path: path, members: map[string]*sourceMember{}}
	content, err := os.ReadFile(path)
	if os.IsNotExist(err) {
		return tracking, tracking.write()
	}
	if err != nil {
		return nil, err
	}
	parsed, err := parseOrderedJSON(content)
	file, ok := parsed.(*orderedMap)
	if err != nil || !ok {
		// sf treats a file it cannot parse as an empty one.
		return tracking, nil
	}
	maxRevision := jsonInt(file.values["serverMaxRevisionCounter"])
	members, _ := file.values["sourceMembers"].(*orderedMap)
	if maxRevision == 0 || members == nil {
		return tracking, nil
	}
	tracking.maxRevision = maxRevision
	version := jsonInt(file.values["fileVersion"])
	for _, key := range members.keys {
		value, ok := members.values[key].(*orderedMap)
		if !ok {
			continue
		}
		member := &sourceMember{}
		if version == 1 {
			member.Type = text(value.values["MemberType"])
			member.Name = text(value.values["MemberName"])
			member.RevisionCounter = jsonInt(value.values["RevisionCounter"])
			member.IsNameObsolete, _ = value.values["IsNameObsolete"].(bool)
		} else {
			// The key is <type>__<name>, with the name encoded as in a URL.
			legacyType, legacyName, _ := strings.Cut(key, "__")
			member.Type = text(value.values["memberType"])
			if member.Type == "" {
				member.Type = legacyType
			}
			member.Name = legacyName
			if decoded, err := url.PathUnescape(legacyName); err == nil {
				member.Name = decoded
			}
			member.RevisionCounter = jsonInt(value.values["serverRevisionCounter"])
			member.IsNameObsolete, _ = value.values["isNameObsolete"].(bool)
		}
		if number, ok := value.values["lastRetrievedFromServer"].(json.Number); ok {
			last := jsonInt(number)
			member.LastRetrieved = &last
		}
		tracking.set(member)
	}
	return tracking, nil
}

func jsonInt(value interface{}) int {
	number, ok := value.(json.Number)
	if !ok {
		return 0
	}
	n, err := strconv.Atoi(number.String())
	if err != nil {
		return 0
	}
	return n
}

func (t *remoteTracking) set(member *sourceMember) {
	key := member.key()
	if _, exists := t.members[key]; !exists {
		t.keys = append(t.keys, key)
	}
	t.members[key] = member
}

// track records members read from the org.  With sync, each is also marked
// as retrieved at its revision; otherwise a member keeps the revision at
// which it was last retrieved.  sf does not write the file when there are no
// members.
func (t *remoteTracking) track(members []*sourceMember, sync bool) error {
	if len(members) == 0 {
		return nil
	}
	for _, member := range members {
		if member.RevisionCounter > t.maxRevision {
			t.maxRevision = member.RevisionCounter
		}
		recorded := *member
		if sync {
			revision := member.RevisionCounter
			recorded.LastRetrieved = &revision
		} else if existing := t.members[member.key()]; existing != nil {
			recorded.LastRetrieved = existing.LastRetrieved
		}
		t.set(&recorded)
	}
	return t.write()
}

// write writes the file in the format sf writes by default: fileVersion 0,
// indented by four spaces.
func (t *remoteTracking) write() error {
	members := newOrderedMap()
	for _, key := range t.keys {
		member := t.members[key]
		value := newOrderedMap()
		value.Set("memberType", member.Type)
		value.Set("serverRevisionCounter", member.RevisionCounter)
		if member.LastRetrieved != nil {
			value.Set("lastRetrievedFromServer", *member.LastRetrieved)
		} else {
			value.Set("lastRetrievedFromServer", nil)
		}
		value.Set("isNameObsolete", member.IsNameObsolete)
		members.Set(member.Type+"__"+member.Name, value)
	}
	file := newOrderedMap()
	file.Set("fileVersion", 0)
	file.Set("serverMaxRevisionCounter", t.maxRevision)
	file.Set("sourceMembers", members)
	var buffer bytes.Buffer
	encoder := json.NewEncoder(&buffer)
	encoder.SetEscapeHTML(false)
	encoder.SetIndent("", "    ")
	if err := encoder.Encode(file); err != nil {
		return err
	}
	if err := os.MkdirAll(filepath.Dir(t.path), 0o755); err != nil {
		return err
	}
	return os.WriteFile(t.path, bytes.TrimRight(buffer.Bytes(), "\n"), 0o644)
}

const sourceMemberFields = "MemberIdOrName, MemberType, MemberName, IsNameObsolete, RevisionCounter, IsNewMember, ChangedBy, LastModifiedDate"

// querySourceMembers reads the SourceMember records that match a condition
// on RevisionCounter.
func querySourceMembers(org *targetOrg, condition string) ([]*sourceMember, error) {
	records, err := toolingRecords(org, "SELECT "+sourceMemberFields+" FROM SourceMember WHERE "+condition)
	if err != nil {
		return nil, err
	}
	members := make([]*sourceMember, 0, len(records))
	for _, record := range records {
		member := &sourceMember{
			Type:             text(record.values["MemberType"]),
			Name:             text(record.values["MemberName"]),
			RevisionCounter:  jsonInt(record.values["RevisionCounter"]),
			ChangedBy:        text(record.values["ChangedBy"]),
			MemberIdOrName:   text(record.values["MemberIdOrName"]),
			LastModifiedDate: text(record.values["LastModifiedDate"]),
		}
		member.IsNameObsolete, _ = record.values["IsNameObsolete"].(bool)
		member.IsNewMember, _ = record.values["IsNewMember"].(bool)
		// The org reports quick actions under another name than the
		// metadata type.
		if member.Type == "QuickActionDefinition" {
			member.Type = "QuickAction"
		}
		members = append(members, member)
	}
	return members, nil
}

// sourceTracking gives access to the tracking files of a project for an
// org.
type sourceTracking struct {
	project *sfProject
	org     *targetOrg
	orgID   string
}

func newSourceTracking(project *sfProject, org *targetOrg) *sourceTracking {
	return &sourceTracking{project: project, org: org, orgID: org.Fields.String("orgId")}
}

func (t *sourceTracking) gitDir() string {
	return filepath.Join(t.project.Root, ".sf", "orgs", t.orgID, "localSourceTracking")
}

func (t *sourceTracking) revisionPath() string {
	return filepath.Join(t.project.Root, ".sf", "orgs", t.orgID, "maxRevision.json")
}

// remote returns the remote tracking of an org that has source tracking.
func (t *sourceTracking) remote() (*remoteTracking, error) {
	supported, err := supportsSourceTracking(t.org)
	if err != nil {
		return nil, err
	}
	if !supported {
		return nil, newError("NonSourceTrackedOrgError", "This org does not have source tracking.")
	}
	return loadRemoteTracking(t.revisionPath())
}

// localChanges lists the files of the package directories that differ from
// the tracked files.  Paths are relative to the project and use forward
// slashes.
type localChanges struct {
	added, modified, deleted []string
	// content holds the id of the content of each file that exists.
	content map[string][20]byte
}

// trackable reports whether sf tracks a file: it leaves out files in a
// directory or with a name that starts with a period, files under
// node_modules, the local tests of Lightning web components, and .gitignore
// files.
func trackable(path string) bool {
	if strings.Contains(path, "/.") || strings.HasSuffix(path, ".gitignore") || strings.Contains(path, "__utam__") || strings.Contains(path, "__tests__") {
		return false
	}
	for _, part := range strings.Split(path, "/") {
		if part == "node_modules" {
			return false
		}
	}
	return true
}

func (t *sourceTracking) localStatus(repo *shadowRepo) (*localChanges, error) {
	tracked, err := repo.trackedFiles()
	if err != nil {
		return nil, err
	}
	changes := &localChanges{content: map[string][20]byte{}}
	var packagePrefixes []string
	for _, dir := range t.project.packageDirs() {
		relative, err := filepath.Rel(t.project.Root, dir)
		if err != nil {
			return nil, err
		}
		packagePrefixes = append(packagePrefixes, filepath.ToSlash(relative)+"/")
		err = filepath.Walk(dir, func(path string, info os.FileInfo, err error) error {
			if err != nil {
				if os.IsNotExist(err) {
					return nil
				}
				return err
			}
			if !info.Mode().IsRegular() {
				return nil
			}
			relative, err := filepath.Rel(t.project.Root, path)
			if err != nil {
				return err
			}
			name := filepath.ToSlash(relative)
			if !trackable(name) {
				return nil
			}
			content, err := os.ReadFile(path)
			if err != nil {
				return err
			}
			id := gitObjectID("blob", content)
			changes.content[name] = id
			if existing, ok := tracked[name]; !ok {
				changes.added = append(changes.added, name)
			} else if existing != id {
				changes.modified = append(changes.modified, name)
			}
			return nil
		})
		if err != nil {
			return nil, err
		}
	}
	for name := range tracked {
		if _, exists := changes.content[name]; exists || !trackable(name) {
			continue
		}
		for _, prefix := range packagePrefixes {
			if strings.HasPrefix(name, prefix) {
				changes.deleted = append(changes.deleted, name)
				break
			}
		}
	}
	sort.Strings(changes.added)
	sort.Strings(changes.modified)
	sort.Strings(changes.deleted)
	return changes, nil
}

// commitLocal records the current content of the files in updated, and the
// absence of the files in deleted, as in sync with the org.
func (t *sourceTracking) commitLocal(repo *shadowRepo, updated, deleted []string, message string) error {
	if len(updated) == 0 && len(deleted) == 0 {
		return nil
	}
	files, err := repo.trackedFiles()
	if err != nil {
		return err
	}
	for _, name := range updated {
		content, err := os.ReadFile(filepath.Join(t.project.Root, filepath.FromSlash(name)))
		if err != nil {
			return err
		}
		id, err := repo.writeObject("blob", content)
		if err != nil {
			return err
		}
		files[name] = id
	}
	for _, name := range deleted {
		delete(files, name)
	}
	_, err = repo.commit(files, message, time.Now())
	return err
}

const trackingPrompt = `WARNING: This operation will modify all your local source tracking files. The operation can have unintended consequences on all the "project deploy" and "project retrieve" commands. Are you sure you want to proceed?`

func newProjectDeleteTrackingCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:         "tracking",
		Short:       "Delete all local source tracking information.",
		Args:        cobra.NoArgs,
		Annotations: map[string]string{commandNameAnnotation: "DeleteTracking"},
		RunE: func(cmd *cobra.Command, args []string) error {
			org, err := resolveTargetOrg(cmd)
			if err != nil {
				return err
			}
			project, err := loadProject()
			if err != nil {
				return err
			}
			cleared := []string{}
			confirmed := confirm(cmd, trackingPrompt)
			if confirmed {
				tracking := newSourceTracking(project, org)
				if err := os.RemoveAll(tracking.gitDir()); err != nil {
					return err
				}
				// sf names the revision file relative to the working
				// directory, not to the project.
				revisionPath, err := filepath.Abs(filepath.Join(".sf", "orgs", tracking.orgID, "maxRevision.json"))
				if err != nil {
					return err
				}
				if err := os.Remove(revisionPath); err != nil && !os.IsNotExist(err) {
					return err
				}
				cleared = []string{tracking.gitDir(), revisionPath}
			}
			result := newOrderedMap()
			result.Set("clearedFiles", cleared)
			return emit(cmd, result, func(w io.Writer) {
				if confirmed {
					fmt.Fprintln(w, paint(w, styleGreen, "Cleared local tracking files."))
				}
			})
		},
	}
	addJSONFlag(cmd)
	addAPIVersionFlag(cmd)
	cmd.Flags().BoolP("no-prompt", "p", false, "Don't prompt for source tracking override confirmation.")
	addTargetOrgFlag(cmd)
	return cmd
}

func newProjectResetTrackingCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:         "tracking",
		Short:       "Reset local and remote source tracking.",
		Args:        cobra.NoArgs,
		Annotations: map[string]string{commandNameAnnotation: "ResetTracking"},
		RunE: func(cmd *cobra.Command, args []string) error {
			revision, _ := cmd.Flags().GetInt("revision")
			hasRevision := cmd.Flags().Changed("revision")
			if hasRevision && revision < 0 {
				return flagParseError("", "revision", fmt.Sprintf("Expected an integer greater than or equal to 0 but received: %d", revision))
			}
			org, err := resolveTargetOrg(cmd)
			if err != nil {
				return err
			}
			project, err := loadProject()
			if err != nil {
				return err
			}
			membersSynced, pathsSynced := 0, 0
			confirmed := confirm(cmd, trackingPrompt)
			if confirmed {
				tracking := newSourceTracking(project, org)
				remote, err := tracking.remote()
				if err != nil {
					return err
				}
				// Every member the org reports, up to the revision when
				// one is given, is recorded as retrieved.
				condition := "RevisionCounter > 0"
				if hasRevision {
					condition = fmt.Sprintf("RevisionCounter <= %d", revision)
				}
				members, err := querySourceMembers(org, condition)
				if err != nil {
					return err
				}
				remote.maxRevision, remote.keys, remote.members = 0, nil, map[string]*sourceMember{}
				if err := remote.track(members, true); err != nil {
					return err
				}
				membersSynced = len(members)

				repo, err := openShadowRepo(tracking.gitDir(), project.Root)
				if err != nil {
					return err
				}
				changes, err := tracking.localStatus(repo)
				if err != nil {
					return err
				}
				updated := append(append([]string{}, changes.added...), changes.modified...)
				if err := tracking.commitLocal(repo, updated, changes.deleted, "via resetLocalTracking"); err != nil {
					return err
				}
				pathsSynced = len(updated) + len(changes.deleted)
			}
			result := newOrderedMap()
			result.Set("sourceMembersSynced", membersSynced)
			result.Set("localPathsSynced", pathsSynced)
			return emit(cmd, result, func(w io.Writer) {
				if !confirmed {
					return
				}
				message := "Reset local tracking files."
				if revision != 0 {
					message = fmt.Sprintf("Reset local tracking files to revision %d.", revision)
				}
				fmt.Fprintln(w, paint(w, styleGreen, message))
			})
		},
	}
	addJSONFlag(cmd)
	addAPIVersionFlag(cmd)
	cmd.Flags().IntP("revision", "r", 0, "SourceMember revision counter number to reset to.")
	cmd.Flags().BoolP("no-prompt", "p", false, "Don't prompt for source tracking override confirmation.")
	addTargetOrgFlag(cmd)
	return cmd
}

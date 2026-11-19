package sf

import (
	"bufio"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"

	"github.com/ForceCLI/force/lib"
	"github.com/spf13/cobra"
)

// projectComponents returns the components of the source format files in
// the package directories of a project, by key.
func projectComponents(project *sfProject) (map[string]*sourceComponent, error) {
	var files []string
	for _, dir := range project.packageDirs() {
		err := filepath.Walk(dir, func(path string, info os.FileInfo, err error) error {
			if err != nil {
				if os.IsNotExist(err) {
					return nil
				}
				return err
			}
			if !info.Mode().IsRegular() {
				return nil
			}
			relative, err := filepath.Rel(project.Root, path)
			if err != nil {
				return err
			}
			if name := filepath.ToSlash(relative); trackable(name) {
				files = append(files, name)
			}
			return nil
		})
		if err != nil {
			return nil, err
		}
	}
	sort.Strings(files)
	return sourceComponents(files), nil
}

// deleteTarget is a component that project delete source deletes.
type deleteTarget struct {
	Type, FullName string
	// xml and files are the local files of the component, relative to the
	// project.  A component that is only in the org has none.
	xml   string
	files []string
}

func (d *deleteTarget) key() string {
	return d.Type + "###" + d.FullName
}

// deleteTargets resolves the components named by --source-dir or
// --metadata.  A component named by --metadata that has no local files is
// deleted in the org only.
func deleteTargets(cmd *cobra.Command, project *sfProject) ([]*deleteTarget, error) {
	local, err := projectComponents(project)
	if err != nil {
		return nil, err
	}
	var targets []*deleteTarget
	seen := map[string]bool{}
	add := func(metadataType, fullName string) {
		target := &deleteTarget{Type: metadataType, FullName: fullName}
		if seen[target.key()] {
			return
		}
		seen[target.key()] = true
		if component := local[target.key()]; component != nil {
			target.xml = component.xml
			target.files = append(target.files, component.files...)
		}
		if metadataType == "CustomObject" {
			// The fields and other parts of a decomposed object are
			// deleted with it.
			for _, component := range local {
				if _, isChild := childTypeNames[component.Type]; isChild && strings.HasPrefix(component.FullName, fullName+".") {
					target.files = append(target.files, component.files...)
				}
			}
			sort.Strings(target.files)
		}
		targets = append(targets, target)
	}
	addSet := func(set *sourceSet) {
		types := make([]string, 0, len(set.types))
		for metadataType := range set.types {
			types = append(types, metadataType)
		}
		sort.Strings(types)
		for _, metadataType := range types {
			members := append([]string{}, set.types[metadataType]...)
			sort.Strings(members)
			for _, member := range members {
				add(metadataType, member)
			}
		}
	}
	ignore := loadForceIgnore(project.Root)
	for _, path := range splitList(cmd, "source-dir") {
		set := newSourceSet()
		set.ignore = ignore
		if err := set.addPath(path); err != nil {
			return nil, err
		}
		addSet(set)
	}
	for _, name := range splitList(cmd, "metadata") {
		set := newSourceSet()
		set.ignore = ignore
		if err := set.addMetadata(project, []string{name}); err != nil {
			if metadataType, member, hasMember := strings.Cut(name, ":"); hasMember && member != "*" {
				add(metadataType, strings.TrimSpace(member))
			}
			continue
		}
		addSet(set)
	}
	return targets, nil
}

// childTypeNames holds the metadata types of the parts of a decomposed
// custom object.
var childTypeNames = func() map[string]bool {
	names := map[string]bool{}
	for _, name := range objectChildTypes {
		names[name] = true
	}
	return names
}()

// deletePrompt returns the question sf asks before it deletes components.
func deletePrompt(root string, targets []*deleteTarget, checkOnly bool) string {
	var remote, local []string
	for _, target := range targets {
		if len(target.files) == 0 {
			remote = append(remote, target.Type+":"+target.FullName)
			continue
		}
		if target.xml != "" {
			local = append(local, filepath.Join(root, filepath.FromSlash(target.xml)))
		}
		for _, file := range target.files {
			if file != target.xml {
				local = append(local, filepath.Join(root, filepath.FromSlash(file)))
			}
		}
	}
	var parts []string
	if len(remote) > 0 {
		parts = append(parts, "This operation will delete the following metadata in your org:\n"+strings.Join(remote, "\n"))
	}
	if len(local) > 0 {
		if len(remote) > 0 {
			parts = append(parts, "\n")
		}
		parts = append(parts, "This operation will delete the following files on your computer and in your org:\n"+strings.Join(local, "\n"))
	}
	if checkOnly {
		parts = append(parts, "Are you sure you want to proceed (this is only a check and won't actually delete anything)?")
	} else {
		parts = append(parts, "Are you sure you want to proceed?")
	}
	return strings.Join(parts, "\n")
}

// removeSourceFiles deletes the local files of the targets, and the
// directories of bundles and decomposed objects that are left empty.
func removeSourceFiles(root string, targets []*deleteTarget) error {
	for _, target := range targets {
		for _, file := range target.files {
			path := filepath.Join(root, filepath.FromSlash(file))
			if err := os.Remove(path); err != nil && !os.IsNotExist(err) {
				return err
			}
			for dir := filepath.Dir(path); !lib.IsMetadataDirectory(filepath.Base(dir)) && dir != root && dir != filepath.Dir(dir); dir = filepath.Dir(dir) {
				if os.Remove(dir) != nil {
					break
				}
			}
		}
	}
	return nil
}

// deletedSourceRows lists the local files of the targets the way sf
// reports them in "deletedSource".  sf builds the entry of a component that
// the org reported as deleted in a different key order than the entry of
// one it did not report.
func deletedSourceRows(root string, targets []*deleteTarget, state *deployState) []*orderedMap {
	reported := map[string]bool{}
	for _, success := range state.status.Details.ComponentSuccesses {
		reported[success.ComponentType+"###"+success.FullName] = true
	}
	rows := []*orderedMap{}
	for _, target := range targets {
		for _, file := range target.files {
			row := newOrderedMap()
			row.Set("fullName", target.FullName)
			row.Set("type", target.Type)
			path := filepath.Join(root, filepath.FromSlash(file))
			if reported[target.key()] {
				row.Set("state", "Deleted")
				row.Set("filePath", path)
			} else {
				row.Set("filePath", path)
				row.Set("state", "Deleted")
			}
			rows = append(rows, row)
		}
	}
	sort.SliceStable(rows, func(i, j int) bool {
		a, b := rows[i], rows[j]
		if a.values["type"] != b.values["type"] {
			return text(a.values["type"]) < text(b.values["type"])
		}
		if a.values["fullName"] != b.values["fullName"] {
			return text(a.values["fullName"]) < text(b.values["fullName"])
		}
		return text(a.values["filePath"]) < text(b.values["filePath"])
	})
	return rows
}

func newProjectDeleteSourceCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:         "source",
		Short:       "Delete source from your project and from a non-source-tracked org.",
		Args:        cobra.NoArgs,
		Annotations: map[string]string{commandNameAnnotation: "Source"},
		RunE: func(cmd *cobra.Command, args []string) error {
			metadata := splitList(cmd, "metadata")
			sourceDirs := splitList(cmd, "source-dir")
			checkOnly, _ := cmd.Flags().GetBool("check-only")
			trackSource, _ := cmd.Flags().GetBool("track-source")
			forceOverwrite, _ := cmd.Flags().GetBool("force-overwrite")
			var problems []string
			switch {
			case len(metadata) == 0 && len(sourceDirs) == 0:
				problems = append(problems, "Exactly one of the following must be provided: --metadata, --source-dir")
			case len(metadata) > 0 && len(sourceDirs) > 0:
				problems = append(problems, "--metadata cannot also be provided when using --source-dir", "--source-dir cannot also be provided when using --metadata")
			}
			if trackSource && checkOnly {
				problems = append(problems, "--check-only=true cannot also be provided when using --track-source")
			}
			if forceOverwrite && !trackSource {
				problems = append(problems, "All of the following must be provided when using --force-overwrite: --track-source")
			}
			if len(problems) > 0 {
				heading := "The following error occurred:"
				if len(problems) > 1 {
					heading = "The following errors occurred:"
				}
				return &Error{Message: heading + "\n  " + strings.Join(problems, "\n  ") + "\nSee more help with --help", ExitCode: 2}
			}
			testLevel, _ := cmd.Flags().GetString("test-level")
			tests := splitList(cmd, "tests")
			switch testLevel {
			case "", "NoTestRun", "RunSpecifiedTests", "RunLocalTests", "RunAllTestsInOrg", "RunRelevantTests":
			default:
				return &Error{Message: fmt.Sprintf("Expected --test-level=%s to be one of: NoTestRun, RunSpecifiedTests, RunLocalTests, RunAllTestsInOrg, RunRelevantTests\nSee more help with --help", testLevel), ExitCode: 1}
			}
			project, err := loadProject()
			if err != nil {
				return err
			}
			org, err := resolveTargetOrg(cmd)
			if err != nil {
				return err
			}
			if testLevel == "RunSpecifiedTests" && len(tests) == 0 {
				return newError("NoTestsSpecifiedError", "You must specify tests using the --tests flag if the --test-level flag is set to RunSpecifiedTests.")
			}
			started := time.Now()
			targets, err := deleteTargets(cmd, project)
			if err != nil {
				return err
			}
			var (
				tracking      *sourceTracking
				trackingState *trackingState
			)
			if trackSource {
				tracking = newSourceTracking(project, org)
				trackingState, err = tracking.state(true)
				if err != nil {
					return err
				}
				if !forceOverwrite {
					keys := make([]string, 0, len(targets))
					for _, target := range targets {
						keys = append(keys, target.key())
					}
					if rows := trackingState.conflictRows(project.Root, keys); len(rows) > 0 {
						return deleteConflictError(cmd, rows)
					}
				}
			}
			out := cmd.OutOrStdout()
			if len(targets) == 0 && !jsonRequested(cmd) {
				fmt.Fprintln(out, "=== Deleted Source")
				fmt.Fprintln(out)
				fmt.Fprintln(out, "No results found")
			}
			if noPrompt, _ := cmd.Flags().GetBool("no-prompt"); !noPrompt {
				fmt.Fprintf(out, "? %s (y/N) ", deletePrompt(project.Root, targets, checkOnly))
				answer, _ := bufio.NewReader(cmd.InOrStdin()).ReadString('\n')
				answer = strings.ToLower(strings.TrimSpace(answer))
				if answer != "y" && answer != "yes" {
					return newError("Prompt.delete.cancelError", "The request to delete metadata was canceled.")
				}
			}
			types := map[string][]string{}
			for _, target := range targets {
				types[target.Type] = append(types[target.Type], target.FullName)
			}
			version := projectAPIVersion(project)
			files := lib.ForceMetadataFiles{
				"package.xml":                packageXML(map[string][]string{}, version),
				"destructiveChangesPost.xml": packageXML(types, version),
			}
			options := lib.ForceDeployOptions{
				CheckOnly:       checkOnly,
				RollbackOnError: true,
				SinglePackage:   true,
				TestLevel:       testLevel,
				RunTests:        tests,
			}
			prepare := time.Since(started)
			id, err := org.Force.Metadata.StartDeploy(files, options)
			if err != nil {
				return apiError(err)
			}
			minutes, _ := cmd.Flags().GetInt("wait")
			state, err := waitForDeploy(org, id, time.Duration(minutes)*time.Minute)
			if state != nil && !jsonRequested(cmd) {
				deployProgress(out, org, "Deleting Metadata", "Deleting with SOAP API", state, false, prepare, time.Since(started))
			}
			if err != nil {
				return err
			}
			if err := recordDeploy(id, orderedFields("status", state.status.Status)); err != nil {
				return err
			}
			succeeded := state.status.Status == "Succeeded"
			result := newOrderedMap()
			for _, key := range state.raw.keys {
				result.Set(key, state.raw.values[key])
			}
			response := newOrderedMap()
			for _, key := range state.raw.keys {
				response.Set(key, state.raw.values[key])
			}
			fillTestResultLists(result)
			rows := deletedSourceRows(project.Root, targets, state)
			result.Set("deletedSource", rows)
			result.Set("outboundFiles", []interface{}{})
			result.Set("deployedSource", []interface{}{})
			result.Set("deletes", []interface{}{response})
			status := 0
			if !succeeded {
				status = 1
			}
			// applyDelete deletes the local files and updates the source
			// tracking files after the org deleted the components.
			applyDelete := func() error {
				if !succeeded || checkOnly {
					return nil
				}
				if err := removeSourceFiles(project.Root, targets); err != nil {
					return err
				}
				if tracking == nil {
					return nil
				}
				deleted := make([]string, 0, len(targets))
				for _, target := range targets {
					deleted = append(deleted, target.key())
				}
				progress, err := tracking.recordDeploy(trackingState, nil, deleted)
				if err != nil {
					return err
				}
				if progress.warning != "" {
					warn(cmd, progress.warning)
				}
				if !jsonRequested(cmd) {
					fmt.Fprintln(cmd.ErrOrStderr(), "Updating source tracking... done")
				}
				return nil
			}
			if jsonRequested(cmd) {
				// The warnings of the tracking update are part of the
				// JSON result.
				if err := applyDelete(); err != nil {
					return err
				}
			}
			if err := emitStatus(cmd, status, result, func(w io.Writer) {
				if succeeded {
					table := make([][]string, 0, len(rows))
					for _, row := range rows {
						table = append(table, []string{text(row.values["fullName"]), text(row.values["type"]), relativePath(text(row.values["filePath"]))})
					}
					// A component with no local files is listed from the
					// result of the org.
					local := map[string]bool{}
					for _, target := range targets {
						if len(target.files) > 0 {
							local[target.key()] = true
						}
					}
					for _, success := range state.status.Details.ComponentSuccesses {
						if success.ComponentType != "" && !local[success.ComponentType+"###"+success.FullName] {
							table = append(table, []string{success.FullName, success.ComponentType, ""})
						}
					}
					fmt.Fprintln(w)
					fmt.Fprintln(w)
					fmt.Fprintln(w, "=== Deleted Source")
					fmt.Fprintln(w)
					printTable(w, "", []string{"FULL NAME", "TYPE", "PROJECT PATH"}, table)
					return
				}
				failures := state.status.Details.ComponentFailures
				if len(failures) == 0 {
					return
				}
				table := make([][]string, 0, len(failures))
				for _, failure := range failures {
					table = append(table, []string{failure.ProblemType, failure.FullName, failure.Problem})
				}
				fmt.Fprintln(w)
				fmt.Fprintln(w)
				printTable(w, paint(w, styleRed, fmt.Sprintf("Component Failures [%d]", len(table))), []string{"Type", "Name", "Problem"}, table)
			}); err != nil {
				return err
			}
			if !succeeded {
				return exitStatus(1)
			}
			if !jsonRequested(cmd) {
				return applyDelete()
			}
			return nil
		},
	}
	addJSONFlag(cmd)
	addAPIVersionFlag(cmd)
	cmd.Flags().BoolP("check-only", "c", false, "Validate delete command but don't delete anything from the org or the local project.")
	cmd.Flags().IntP("wait", "w", 33, "Number of minutes to wait for the command to finish.")
	cmd.Flags().StringArray("tests", nil, "Apex tests to run when --test-level is RunSpecifiedTests.")
	cmd.Flags().StringP("test-level", "l", "", "Deployment Apex testing level.")
	cmd.Flags().BoolP("no-prompt", "r", false, "Don't prompt for delete confirmation.")
	cmd.Flags().StringArrayP("metadata", "m", nil, "Metadata components to delete.")
	cmd.Flags().StringArrayP("source-dir", "p", nil, "Source file paths to delete.")
	cmd.Flags().BoolP("track-source", "t", false, "If the delete succeeds, update the source tracking information.")
	cmd.Flags().BoolP("force-overwrite", "f", false, "Ignore conflict warnings and overwrite changes to the org.")
	cmd.Flags().Bool("verbose", false, "Verbose output of the delete result.")
	addTargetOrgFlag(cmd)
	return cmd
}

// deleteConflictError prints the table of conflicts and returns the error
// sf reports for a delete that would overwrite changes in the org.
func deleteConflictError(cmd *cobra.Command, rows []*orderedMap) error {
	// sf prints the table with --json too, before the JSON error.
	table := make([][]string, 0, len(rows))
	for _, row := range rows {
		table = append(table, []string{text(row.values["state"]), text(row.values["fullName"]), text(row.values["type"]), text(row.values["filePath"])})
	}
	printTable(cmd.OutOrStdout(), "", []string{"STATE", "FULL NAME", "TYPE", "FILE PATH"}, table)
	return &Error{
		Name:     "sourceConflictDetected",
		Message:  "We couldn't complete the operation due to conflicts. Verify that you want to keep the existing versions, then run the command again with the --force-overwrite (-f) option.",
		ExitCode: 1,
		Data:     rows,
	}
}

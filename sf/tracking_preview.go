package sf

import (
	"fmt"
	"io"
	"os"
	"path/filepath"
	"sort"
	"strings"

	"github.com/spf13/cobra"

	"github.com/ForceCLI/force/lib"
)

// sourceComponent is a metadata component in the source format files of a
// project.
type sourceComponent struct {
	Type, FullName string
	// xml is the -meta.xml file of the component, relative to the project
	// with forward slashes, or "" when the component has no local file.
	xml string
	// files are the files of the component that are tracked or on disk.
	files []string
}

func (c *sourceComponent) key() string {
	return c.Type + "###" + c.FullName
}

// componentOfSourceFile returns the type and name of the component that a
// source format file belongs to.  A file of a decomposed custom object,
// such as a field, is a component of its own type.
func componentOfSourceFile(path string) (metadataType, fullName string) {
	parts := strings.Split(path, "/")
	for i := 1; i < len(parts)-1; i++ {
		if _, ok := lib.MetadataTypeForDirectory(parts[i]); !ok {
			continue
		}
		relative := parts[i:]
		metadataType, fullName = componentOfPath(strings.Join(relative, "/"))
		if metadataType == "CustomObject" && len(relative) == 4 {
			if childType, ok := objectChildTypes[relative[2]]; ok {
				name := strings.TrimSuffix(relative[3], "-meta.xml")
				return childType, fullName + "." + strings.TrimSuffix(name, filepath.Ext(name))
			}
		}
		return metadataType, fullName
	}
	return "", ""
}

// sourceComponents groups files into components.  The -meta.xml file of a
// component is its shortest path that ends in -meta.xml.
func sourceComponents(files []string) map[string]*sourceComponent {
	components := map[string]*sourceComponent{}
	for _, file := range files {
		metadataType, fullName := componentOfSourceFile(file)
		if metadataType == "" {
			continue
		}
		component := &sourceComponent{Type: metadataType, FullName: fullName}
		if existing := components[component.key()]; existing != nil {
			component = existing
		} else {
			components[component.key()] = component
		}
		component.files = append(component.files, file)
		if strings.HasSuffix(file, "-meta.xml") && (component.xml == "" || len(file) < len(component.xml)) {
			component.xml = file
		}
	}
	return components
}

// previewFile is one row of a preview.
type previewFile struct {
	component *sourceComponent
	conflict  bool
	ignored   bool
	operation string
}

// json returns the row as sf writes it.  sf builds the row of a component
// with a local file, the row of a component without one, and the row of a
// file that .forceignore excludes in three places, with the keys in three
// orders.
func (f previewFile) json(root string, fromIgnoreList bool) *orderedMap {
	row := newOrderedMap()
	absolute := filepath.Join(root, filepath.FromSlash(f.component.xml))
	switch {
	case fromIgnoreList:
		row.Set("fullName", f.component.FullName)
		row.Set("type", f.component.Type)
		row.Set("path", absolute)
		row.Set("projectRelativePath", f.component.xml)
		row.Set("conflict", false)
		row.Set("ignored", true)
	case f.component.xml == "":
		row.Set("type", f.component.Type)
		row.Set("fullName", f.component.FullName)
		row.Set("conflict", false)
		row.Set("operation", f.operation)
		row.Set("ignored", f.ignored)
	default:
		row.Set("type", f.component.Type)
		row.Set("fullName", f.component.FullName)
		row.Set("conflict", f.conflict)
		row.Set("ignored", f.ignored)
		row.Set("path", absolute)
		row.Set("projectRelativePath", f.component.xml)
		row.Set("operation", f.operation)
	}
	return row
}

// preview is the result of a deploy or retrieve preview.
type preview struct {
	ignored, toDeploy, toRetrieve, toDelete, conflicts []previewFile
}

// add files a row under the list sf reports it in.
func (p *preview) add(file previewFile) {
	switch {
	case file.ignored:
		p.ignored = append(p.ignored, file)
	case file.conflict:
		p.conflicts = append(p.conflicts, file)
	case file.operation == "deploy":
		p.toDeploy = append(p.toDeploy, file)
	case file.operation == "retrieve":
		p.toRetrieve = append(p.toRetrieve, file)
	default:
		p.toDelete = append(p.toDelete, file)
	}
}

func (p *preview) result(root string, ignoredFromList int) *orderedMap {
	rows := func(files []previewFile, fromIgnoreList int) []*orderedMap {
		result := make([]*orderedMap, 0, len(files))
		for i, file := range files {
			result = append(result, file.json(root, i < fromIgnoreList))
		}
		return result
	}
	result := newOrderedMap()
	result.Set("ignored", rows(p.ignored, ignoredFromList))
	result.Set("toDeploy", rows(p.toDeploy, 0))
	result.Set("toRetrieve", rows(p.toRetrieve, 0))
	result.Set("toDelete", rows(p.toDelete, 0))
	result.Set("conflicts", rows(p.conflicts, 0))
	return result
}

// wrapWords breaks text into lines of at most width characters.  sf wraps
// the title of a table at the width of the terminal, which is 80 columns
// when the output is not a terminal.
func wrapWords(text string, width int) string {
	var lines []string
	line := ""
	for _, word := range strings.Fields(text) {
		if line != "" && len(line)+1+len(word) > width {
			lines = append(lines, line)
			line = word
			continue
		}
		if line != "" {
			line += " "
		}
		line += word
	}
	return strings.Join(append(lines, line), "\n")
}

func (p *preview) print(w io.Writer, operation string, concise bool) {
	table := func(files []previewFile, none, header string, headerStyle style, sorted bool) {
		fmt.Fprintln(w)
		if len(files) == 0 {
			fmt.Fprintln(w, paint(w, styleDim, none))
			return
		}
		rows := make([][]string, 0, len(files))
		for _, file := range files {
			rows = append(rows, []string{file.component.Type, file.component.FullName, file.component.xml})
		}
		if sorted {
			sort.SliceStable(rows, func(i, j int) bool { return rows[i][2] < rows[j][2] })
		}
		printTable(w, paint(w, headerStyle, wrapWords(fmt.Sprintf(header, len(files)), 80)), []string{"Type", "Full Name", "Path"}, rows)
	}
	table(p.conflicts, "No conflicts found.", "Conflicts [%d]. Run the command with the --ignore-conflicts flag to override.", styleBoldRed, true)
	table(p.toDelete, "No files will be deleted.", "Will Delete [%d] files.", styleBoldYellow, false)
	if operation == "deploy" {
		table(p.toDeploy, "No files will be deployed.", "Will Deploy [%d] files.", styleBoldGreen, false)
	} else {
		table(p.toRetrieve, "No files will be retrieved.", "Will Retrieve [%d] files.", styleBoldGreen, false)
	}
	if !concise {
		table(p.ignored, "No files were ignored. Update your .forceignore file if you want to ignore certain files.",
			"Ignored [%d] files. These files won't "+operation+" because they're ignored by your .forceignore file.", styleDim, true)
	}
}

// trackingState is what the previews need from source tracking: the local
// changes, the components of the project, and the components of the org
// that are not in sync.
type trackingState struct {
	changes *localChanges
	// components are the components of the tracked files and of the files
	// on disk, by key.
	components map[string]*sourceComponent
	// onDisk holds the files that exist.
	onDisk map[string]bool
	// outOfSync are the members of the org whose revision was not the one
	// last deployed or retrieved, in the order of maxRevision.json.
	outOfSync []*sourceMember
}

// state reads the local changes and, unless remote is false, asks the org
// for the members changed since the last query and records them.
func (t *sourceTracking) state(remote bool) (*trackingState, error) {
	state := &trackingState{onDisk: map[string]bool{}}
	if remote {
		tracking, err := t.remote()
		if err != nil {
			return nil, err
		}
		members, err := querySourceMembers(t.org, fmt.Sprintf("RevisionCounter > %d", tracking.maxRevision))
		if err != nil {
			return nil, err
		}
		if err := tracking.track(members, false); err != nil {
			return nil, err
		}
		for _, key := range tracking.keys {
			if member := tracking.members[key]; !member.inSync() {
				state.outOfSync = append(state.outOfSync, member)
			}
		}
	}
	repo, err := openShadowRepo(t.gitDir(), t.project.Root)
	if err != nil {
		return nil, err
	}
	state.changes, err = t.localStatus(repo)
	if err != nil {
		return nil, err
	}
	tracked, err := repo.trackedFiles()
	if err != nil {
		return nil, err
	}
	var files []string
	for file := range state.changes.content {
		state.onDisk[file] = true
		files = append(files, file)
	}
	for file := range tracked {
		if !state.onDisk[file] {
			files = append(files, file)
		}
	}
	sort.Strings(files)
	state.components = sourceComponents(files)
	return state, nil
}

func (s *trackingState) componentOf(file string) *sourceComponent {
	metadataType, fullName := componentOfSourceFile(file)
	return s.components[metadataType+"###"+fullName]
}

// exists reports whether a component has a file on disk.
func (s *trackingState) exists(component *sourceComponent) bool {
	for _, file := range component.files {
		if s.onDisk[file] {
			return true
		}
	}
	return false
}

// locallyChanged returns the keys of the components with an added or
// modified file.
func (s *trackingState) locallyChanged() map[string]bool {
	changed := map[string]bool{}
	for _, file := range append(append([]string{}, s.changes.added...), s.changes.modified...) {
		if component := s.componentOf(file); component != nil {
			changed[component.key()] = true
		}
	}
	return changed
}

// conflicts returns the keys of the components changed both locally and in
// the org.
func (s *trackingState) conflicts() map[string]bool {
	local := s.locallyChanged()
	conflicts := map[string]bool{}
	for _, member := range s.outOfSync {
		if local[member.key()] {
			conflicts[member.key()] = true
		}
	}
	return conflicts
}

func newProjectDeployPreviewCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:         "preview",
		Short:       "Preview a deployment to see what will deploy to the org, the potential conflicts, and the ignored files.",
		Args:        cobra.NoArgs,
		Annotations: map[string]string{commandNameAnnotation: "DeployMetadataPreview"},
		RunE: func(cmd *cobra.Command, args []string) error {
			ignoreConflicts, _ := cmd.Flags().GetBool("ignore-conflicts")
			sourceDirs := splitList(cmd, "source-dir")
			metadata := splitList(cmd, "metadata")
			manifestPath, _ := cmd.Flags().GetString("manifest")
			if manifestPath != "" {
				if info, err := os.Stat(manifestPath); err != nil || info.IsDir() {
					return flagParseError("", "manifest", "No file found at "+manifestPath)
				}
			}
			specified := len(sourceDirs) > 0 || len(metadata) > 0 || manifestPath != ""
			org, err := resolveTargetOrg(cmd)
			if err != nil {
				return err
			}
			project, err := loadProject()
			if err != nil {
				return err
			}
			tracking := newSourceTracking(project, org)
			// The org is not asked when the components are named and
			// conflicts are ignored.
			state, err := tracking.state(!(ignoreConflicts && specified))
			if err != nil {
				return err
			}
			conflicts := map[string]bool{}
			if !ignoreConflicts {
				conflicts = state.conflicts()
			}
			ignore := loadForceIgnore(project.Root)
			denied := func(file string) bool {
				return ignore.ignores(filepath.Join(project.Root, filepath.FromSlash(file)))
			}
			result := &preview{}
			seen := map[string]bool{}
			addIgnored := func(file string) {
				if component := state.componentOf(file); component != nil && component.xml != "" && !seen["ignored "+component.xml] {
					seen["ignored "+component.xml] = true
					result.ignored = append(result.ignored, previewFile{component: component, ignored: true})
				}
			}
			addComponent := func(component *sourceComponent, operation string) {
				if seen[component.key()] {
					return
				}
				seen[component.key()] = true
				result.add(previewFile{component: component, conflict: conflicts[component.key()], operation: operation})
			}
			if !specified {
				// The components of the files changed since the last
				// deploy or retrieve, in the order of the files.
				for _, file := range state.changes.deleted {
					if component := state.componentOf(file); component != nil && !state.exists(component) {
						addComponent(component, "deletePost")
					}
				}
				changed := append(append(append([]string{}, state.changes.added...), state.changes.modified...), state.changes.deleted...)
				sort.Strings(changed)
				for _, file := range changed {
					if !state.onDisk[file] {
						// A deleted file of a component that still has
						// files deploys the component.
						if component := state.componentOf(file); component != nil && state.exists(component) {
							addComponent(component, "deploy")
						}
						continue
					}
					if denied(file) {
						addIgnored(file)
						continue
					}
					if component := state.componentOf(file); component != nil {
						addComponent(component, "deploy")
					}
				}
			} else {
				var files []string
				for file := range state.onDisk {
					files = append(files, file)
				}
				sort.Strings(files)
				if manifestPath != "" {
					manifest, err := readManifest(manifestPath)
					if err != nil {
						return err
					}
					metadata = append(metadata, manifest.metadataNames()...)
				}
				for _, dir := range sourceDirs {
					absolute, err := filepath.Abs(dir)
					if err != nil {
						return err
					}
					if _, err := os.Stat(absolute); err != nil {
						return newError("SfError", "%s: File or folder not found", dir)
					}
					relative, err := filepath.Rel(project.Root, absolute)
					if err != nil {
						return err
					}
					prefix := filepath.ToSlash(relative)
					for _, file := range files {
						if file != prefix && !strings.HasPrefix(file, prefix+"/") {
							continue
						}
						if denied(file) {
							addIgnored(file)
						} else if component := state.componentOf(file); component != nil {
							addComponent(component, "deploy")
						}
					}
				}
				if len(metadata) > 0 {
					wanted := map[string]bool{}
					for _, name := range metadata {
						metadataType, member, hasMember := strings.Cut(name, ":")
						if !hasMember {
							member = "*"
						}
						wanted[metadataType+"###"+member] = true
					}
					found := map[string]bool{}
					for _, file := range files {
						if denied(file) {
							addIgnored(file)
							continue
						}
						component := state.componentOf(file)
						if component == nil || !(wanted[component.key()] || wanted[component.Type+"###*"]) {
							continue
						}
						found[component.key()] = true
						addComponent(component, "deploy")
					}
					// A named component without local files is listed
					// without a path.
					for _, name := range metadata {
						metadataType, member, hasMember := strings.Cut(name, ":")
						if hasMember && member != "*" && !found[metadataType+"###"+member] {
							addComponent(&sourceComponent{Type: metadataType, FullName: member}, "deploy")
						}
					}
				}
			}
			concise, _ := cmd.Flags().GetBool("concise")
			return emit(cmd, result.result(project.Root, len(result.ignored)), func(w io.Writer) { result.print(w, "deploy", concise) })
		},
	}
	addJSONFlag(cmd)
	cmd.Flags().BoolP("ignore-conflicts", "c", false, "Don't display conflicts in preview of the deployment.")
	cmd.Flags().StringP("manifest", "x", "", "Full file path for manifest (package.xml) of components to preview.")
	cmd.Flags().StringArrayP("metadata", "m", nil, "Metadata component names to preview.")
	cmd.Flags().StringArrayP("source-dir", "d", nil, "Path to the local source files to preview.")
	cmd.Flags().Bool("concise", false, "Show only the changes that will be deployed; omits files that are forceignored.")
	addTargetOrgFlag(cmd)
	return cmd
}

func newProjectRetrievePreviewCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:         "preview",
		Short:       "Preview a retrieval to see what will be retrieved from the org, the potential conflicts, and the ignored files.",
		Args:        cobra.NoArgs,
		Annotations: map[string]string{commandNameAnnotation: "RetrieveMetadataPreview"},
		RunE: func(cmd *cobra.Command, args []string) error {
			ignoreConflicts, _ := cmd.Flags().GetBool("ignore-conflicts")
			org, err := resolveTargetOrg(cmd)
			if err != nil {
				return err
			}
			project, err := loadProject()
			if err != nil {
				return err
			}
			state, err := newSourceTracking(project, org).state(true)
			if err != nil {
				return err
			}
			conflicts := map[string]bool{}
			if !ignoreConflicts {
				conflicts = state.conflicts()
			}
			ignore := loadForceIgnore(project.Root)
			local := func(member *sourceMember) *sourceComponent {
				if component := state.components[member.key()]; component != nil && component.xml != "" && state.exists(component) {
					return component
				}
				return nil
			}
			result := &preview{}
			// sf lists the changed components that have local files first,
			// then the ones that have none, then the components deleted in
			// the org that have local files.
			for _, member := range state.outOfSync {
				if component := local(member); component != nil && !member.IsNameObsolete {
					ignored := ignore.ignores(filepath.Join(project.Root, filepath.FromSlash(component.xml)))
					result.add(previewFile{component: component, conflict: conflicts[member.key()], ignored: ignored, operation: "retrieve"})
				}
			}
			for _, member := range state.outOfSync {
				if local(member) == nil && !member.IsNameObsolete {
					result.add(previewFile{component: &sourceComponent{Type: member.Type, FullName: member.Name}, operation: "retrieve"})
				}
			}
			for _, member := range state.outOfSync {
				if component := local(member); component != nil && member.IsNameObsolete {
					ignored := ignore.ignores(filepath.Join(project.Root, filepath.FromSlash(component.xml)))
					result.add(previewFile{component: component, ignored: ignored, operation: "deletePre"})
				}
			}
			concise, _ := cmd.Flags().GetBool("concise")
			return emit(cmd, result.result(project.Root, 0), func(w io.Writer) { result.print(w, "retrieve", concise) })
		},
	}
	addJSONFlag(cmd)
	cmd.Flags().BoolP("ignore-conflicts", "c", false, "Don't display conflicts in the preview of the retrieval.")
	cmd.Flags().Bool("concise", false, "Show only the changes that will be retrieved; omits files that are forceignored.")
	addTargetOrgFlag(cmd)
	return cmd
}

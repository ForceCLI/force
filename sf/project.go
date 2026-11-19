package sf

import (
	"bytes"
	"crypto/sha256"
	"encoding/xml"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"

	"github.com/spf13/cobra"

	"github.com/ForceCLI/force/lib"
)

func newProjectCmd() *cobra.Command {
	return newGroup("project", "Work with projects, such as deploy and retrieve metadata.",
		newGroup("convert", "Convert between source format and metadata format.",
			newProjectConvertSourceCmd(),
			newProjectConvertMdapiCmd(),
		),
		newGroup("delete", "Delete source or source tracking information.", newProjectDeleteSourceCmd(), newProjectDeleteTrackingCmd()),
		newGroup("reset", "Reset source tracking.", newProjectResetTrackingCmd()),
		newGroup("generate", "Generate a manifest.", newProjectGenerateManifestCmd()),
		newGroup("list", "List ignored files.", newProjectListIgnoredCmd()),
		newGroup("deploy", "Deploy metadata to an org from your local project.",
			newProjectDeployPreviewCmd(),
			newProjectDeployStartCmd(false),
			newProjectDeployStartCmd(true),
			newProjectDeployQuickCmd(),
			newProjectDeployReportCmd(),
			newProjectDeployResumeCmd(),
			newProjectDeployCancelCmd(),
		),
		newGroup("retrieve", "Retrieve metadata from an org to your local project.",
			newProjectRetrievePreviewCmd(),
			newProjectRetrieveStartCmd(),
		),
	)
}

// projectAPIVersion returns the version written to generated manifests:
// sourceApiVersion from sfdx-project.json, or the API version in use.
func projectAPIVersion(project *sfProject) string {
	if project != nil && project.SourceApiVersion != "" {
		return project.SourceApiVersion
	}
	return lib.ApiVersionNumber()
}

// relativePath returns path relative to the working directory, as sf prints
// file paths.
func relativePath(path string) string {
	wd, err := os.Getwd()
	if err != nil {
		return path
	}
	relative, err := filepath.Rel(wd, path)
	if err != nil {
		return path
	}
	return filepath.ToSlash(relative)
}

// collectSource gathers the components named by --source-dir, --metadata
// and --manifest.
func collectSource(cmd *cobra.Command, project *sfProject, sourceDirFlag string) (*sourceSet, error) {
	set := newSourceSet()
	if project != nil {
		set.ignore = loadForceIgnore(project.Root)
	}
	sourceDirs := splitList(cmd, sourceDirFlag)
	metadata := splitList(cmd, "metadata")
	manifest, _ := cmd.Flags().GetString("manifest")
	for _, dir := range sourceDirs {
		if err := set.addPath(dir); err != nil {
			return nil, err
		}
	}
	if manifest != "" {
		parsed, err := readManifest(manifest)
		if err != nil {
			return nil, err
		}
		metadata = append(metadata, parsed.metadataNames()...)
	}
	if len(metadata) > 0 {
		if err := set.addMetadata(project, metadata); err != nil {
			return nil, err
		}
	}
	return set, nil
}

func newProjectConvertSourceCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:         "source",
		Short:       "Convert source-formatted files into metadata that you can deploy using Metadata API.",
		Args:        cobra.NoArgs,
		Annotations: map[string]string{commandNameAnnotation: "Source"},
		RunE: func(cmd *cobra.Command, args []string) error {
			project, err := loadProject()
			if err != nil {
				return err
			}
			if rootDir, _ := cmd.Flags().GetString("root-dir"); rootDir != "" {
				if info, err := os.Stat(rootDir); err != nil {
					return flagParseError("", "root-dir", "No directory found at "+rootDir)
				} else if !info.IsDir() {
					return flagParseError("", "root-dir", rootDir+" exists but is not a directory")
				}
			}
			set, err := collectSource(cmd, project, "source-dir")
			if err != nil {
				return err
			}
			if len(set.files) == 0 {
				rootDir, _ := cmd.Flags().GetString("root-dir")
				if rootDir == "" {
					rootDir = project.defaultPackageDir()
				}
				if err := set.addPath(rootDir); err != nil {
					return err
				}
			}
			outputDir, _ := cmd.Flags().GetString("output-dir")
			if outputDir == "" {
				outputDir = fmt.Sprintf("metadataPackage_%d", time.Now().UnixMilli())
			}
			location, err := filepath.Abs(outputDir)
			if err != nil {
				return err
			}
			packageName, _ := cmd.Flags().GetString("package-name")
			for path, data := range set.deployFiles(projectAPIVersion(project)) {
				if path == "package.xml" && packageName != "" {
					// The package name follows the version in the manifest.
					var escaped strings.Builder
					xml.EscapeText(&escaped, []byte(packageName))
					data = bytes.Replace(data, []byte("</Package>"), []byte("    <fullName>"+escaped.String()+"</fullName>\n</Package>"), 1)
				}
				target := filepath.Join(location, filepath.FromSlash(path))
				if err := os.MkdirAll(filepath.Dir(target), 0o755); err != nil {
					return err
				}
				if err := os.WriteFile(target, data, 0o644); err != nil {
					return err
				}
			}
			return emit(cmd, map[string]string{"location": location}, func(w io.Writer) {
				fmt.Fprintf(w, "Source was successfully converted to Metadata API format and written to the location: %s\n", location)
			})
		},
	}
	addJSONFlag(cmd)
	addAPIVersionFlag(cmd)
	cmd.Flags().StringP("root-dir", "r", "", "Source directory other than the default package to convert.")
	cmd.Flags().StringP("output-dir", "d", "", "Output directory to store the Metadata API–formatted files in.")
	cmd.Flags().StringP("package-name", "n", "", "Name of the package to associate with the metadata-formatted files.")
	cmd.Flags().StringP("manifest", "x", "", "Path to the manifest (package.xml) file that specifies the metadata types to convert.")
	cmd.Flags().StringArrayP("source-dir", "p", nil, "Paths to the local source files to convert.")
	cmd.Flags().StringArrayP("metadata", "m", nil, "Metadata component names to convert.")
	return cmd
}

type fileResult struct {
	FullName string `json:"fullName"`
	Type     string `json:"type"`
	FilePath string `json:"filePath"`
	State    string `json:"state"`
	// parent and childDir are set for a file of a decomposed object: the
	// name of the object and the directory of the child type.  sf lists
	// such files with their object, before the files of the object itself.
	parent   string
	childDir string
}

// componentLocator returns a function that gives the path at which a
// retrieved file belongs when its component already has files in one of
// the package directories of the project, or "" when it has none.  The
// argument is the path of the file under a package directory's main/default
// directory, such as classes/Greeter.cls.
func componentLocator(project *sfProject) (func(relative string) string, error) {
	components, err := projectComponents(project)
	if err != nil {
		return nil, err
	}
	return func(relative string) string {
		metadataType, fullName := componentOfSourceFile("x/" + relative)
		component := components[metadataType+"###"+fullName]
		if component == nil && metadataType != "CustomObject" {
			// A new part of a decomposed object belongs with the object.
			if object, _, isChild := strings.Cut(fullName, "."); isChild {
				component = components["CustomObject###"+object]
			}
		}
		if component == nil || len(component.files) == 0 {
			return ""
		}
		typeDirectory, _, _ := strings.Cut(relative, "/")
		existing := component.files[0]
		index := strings.LastIndex("/"+existing, "/"+typeDirectory+"/")
		if index < 0 {
			return ""
		}
		return filepath.Join(project.Root, filepath.FromSlash(existing[:index]), filepath.FromSlash(relative))
	}, nil
}

// writeSourceFormat writes metadata format files under root in source
// format and returns one entry for each file written, with paths relative
// to the working directory.  The files are converted in a temporary
// directory first, so that each can be compared with the file it replaces.
func writeSourceFormat(root string, files lib.ForceMetadataFiles, query lib.ForceMetadataQuery, locate ...func(relative string) string) ([]fileResult, error) {
	staging, err := os.MkdirTemp("", "sf-source-format")
	if err != nil {
		return nil, err
	}
	defer os.RemoveAll(staging)
	write := lib.SourceFormatEntryWriter(staging, query)
	names := make([]string, 0, len(files))
	for name := range files {
		names = append(names, name)
	}
	sort.Strings(names)
	for _, name := range names {
		// sf does not write a custom labels file that has no labels.
		if strings.HasSuffix(name, ".labels") && !bytes.Contains(files[name], []byte("<labels>")) {
			continue
		}
		if err := write(name, strings.NewReader(string(files[name]))); err != nil {
			return nil, err
		}
	}
	var results []fileResult
	err = filepath.Walk(staging, func(staged string, info os.FileInfo, err error) error {
		if err != nil || !info.Mode().IsRegular() {
			return err
		}
		relative, err := filepath.Rel(staging, staged)
		if err != nil {
			return err
		}
		data, err := os.ReadFile(staged)
		if err != nil {
			return err
		}
		target := filepath.Join(root, relative)
		if len(locate) > 0 {
			if located := locate[0](filepath.ToSlash(relative)); located != "" {
				target = located
			}
		}
		state := "Add"
		if existing, err := os.ReadFile(target); err == nil {
			state = "Changed"
			if sha256.Sum256(existing) == sha256.Sum256(data) {
				state = "Unchanged"
			}
		}
		if state != "Unchanged" {
			if err := os.MkdirAll(filepath.Dir(target), 0o755); err != nil {
				return err
			}
			if err := os.WriteFile(target, data, 0o644); err != nil {
				return err
			}
		}
		metadataType, fullName := componentOfPath(relative)
		if metadataType == "" {
			return nil
		}
		parts := strings.Split(filepath.ToSlash(relative), "/")
		if metadataType == "CustomObject" && len(parts) == 4 {
			// A file of a decomposed object: sf lists it as a component
			// of its own type, named after the object.
			if childType, ok := objectChildTypes[parts[2]]; ok {
				name := strings.TrimSuffix(parts[3], "-meta.xml")
				results = append(results, fileResult{FullName: fullName + "." + strings.TrimSuffix(name, filepath.Ext(name)), Type: childType, FilePath: relativePath(target), State: state, parent: fullName, childDir: parts[2]})
				return nil
			}
		}
		results = append(results, fileResult{FullName: fullName, Type: metadataType, FilePath: relativePath(target), State: state})
		if metadataType == "CustomObject" && len(parts) == 3 && strings.HasSuffix(parts[2], ".object-meta.xml") {
			// sf also lists the directory of the object.
			results = append(results, fileResult{FullName: fullName, Type: metadataType, FilePath: relativePath(filepath.Dir(target)), State: state})
		}
		return nil
	})
	if err != nil {
		return nil, err
	}
	sort.Slice(results, func(i, j int) bool {
		a, b := results[i], results[j]
		// The files of the child components of an object are listed at
		// the place of the object, in the order of their directories,
		// before the files of the object.
		group := func(r fileResult) (string, string) {
			if r.parent != "" {
				return "CustomObject", r.parent
			}
			return r.Type, r.FullName
		}
		aType, aName := group(a)
		bType, bName := group(b)
		if aType != bType {
			// sf writes the custom labels file after the other
			// components.
			if (aType == "CustomLabels") != (bType == "CustomLabels") {
				return bType == "CustomLabels"
			}
			return aType < bType
		}
		if aName != bName {
			return aName < bName
		}
		if (a.parent != "") != (b.parent != "") {
			return a.parent != ""
		}
		if a.childDir != b.childDir {
			return a.childDir < b.childDir
		}
		if a.FullName != b.FullName {
			return a.FullName < b.FullName
		}
		// sf lists the -meta.xml file of a component before its content
		// file.
		aMeta, bMeta := strings.HasSuffix(a.FilePath, "-meta.xml"), strings.HasSuffix(b.FilePath, "-meta.xml")
		if aMeta != bMeta {
			return aMeta
		}
		return a.FilePath < b.FilePath
	})
	return results, nil
}

// objectChildTypes maps a directory of a decomposed custom object to the
// metadata type of the files in it.
var objectChildTypes = map[string]string{
	"businessProcesses": "BusinessProcess",
	"compactLayouts":    "CompactLayout",
	"fields":            "CustomField",
	"fieldSets":         "FieldSet",
	"indexes":           "Index",
	"listViews":         "ListView",
	"recordTypes":       "RecordType",
	"sharingReasons":    "SharingReason",
	"validationRules":   "ValidationRule",
	"webLinks":          "WebLink",
}

// mdapiSelection returns a function that reports whether a component of the
// metadata format directory is one of those named by a manifest, by
// --metadata values, or by paths.  It returns nil when none of the three is
// given, which selects every component.
func mdapiSelection(rootDir string, files lib.ForceMetadataFiles, manifestPath string, metadata, paths []string) (func(metadataType, fullName string) bool, error) {
	// names holds "Type:Name" for a named component and "Type:*" for every
	// component of a type.
	names := map[string]bool{}
	selected := func(metadataType, fullName string) bool {
		return names[metadataType+":"+fullName] || names[metadataType+":*"]
	}
	switch {
	case manifestPath != "":
		manifest, err := readManifest(manifestPath)
		if err != nil {
			return nil, err
		}
		for _, metaType := range manifest.Types {
			for _, member := range metaType.Members {
				names[metaType.Name+":"+member] = true
			}
		}
	case len(metadata) > 0:
		for _, name := range metadata {
			metadataType, member, hasMember := strings.Cut(name, ":")
			if !lib.IsMetadataTypeName(metadataType) && !registryKnowsType(metadataType) {
				return nil, &Error{
					Name:     "RegistryError",
					Message:  fmt.Sprintf("Missing metadata type definition in registry for id '%s'.", metadataType),
					ExitCode: 1,
					Actions:  append(typeNameSuggestions(metadataType), registryTypeAdvice),
				}
			}
			if !hasMember {
				member = "*"
			}
			names[metadataType+":"+member] = true
		}
	case len(paths) > 0:
		root, err := filepath.Abs(rootDir)
		if err != nil {
			return nil, err
		}
		for _, path := range paths {
			absolute, err := filepath.Abs(path)
			if err != nil {
				return nil, err
			}
			if _, err := os.Stat(absolute); err != nil {
				return nil, newError("InvalidFlagPath", "The metadatapath command flag specifies an invalid path: %s\nNo such file or directory.", path)
			}
			for file := range files {
				full := filepath.Join(root, filepath.FromSlash(file))
				if full != absolute && !strings.HasPrefix(full, absolute+string(os.PathSeparator)) {
					continue
				}
				if metadataType, fullName := componentOfPath(file); metadataType != "" {
					names[metadataType+":"+fullName] = true
				}
			}
		}
	default:
		return nil, nil
	}
	return selected, nil
}

// registryTypeAdvice is the advice sf gives for a metadata type name it
// does not know.
const registryTypeAdvice = `Confirm the metadata type name is correct. Validate against the registry at:
<https://github.com/forcedotcom/source-deploy-retrieve/blob/main/src/registry/metadataRegistry.json>

If the type is not listed in the registry, check that it has Metadata API support via the Metadata Coverage Report:
<https://developer.salesforce.com/docs/metadata-coverage>

If the type is available via Metadata API but not in the registry

- Open an issue <https://github.com/forcedotcom/cli/issues>
- Add the type via PR. Instructions: <https://github.com/forcedotcom/source-deploy-retrieve/blob/main/contributing/metadata.md>`

// wildcardQuery requests every component of the types that the source
// format writer handles by name.
var wildcardQuery = lib.ForceMetadataQuery{
	{Name: []string{"CustomObject"}, Members: []string{"*"}},
	{Name: []string{"CustomObjectTranslation"}, Members: []string{"*"}},
}

func newProjectConvertMdapiCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:         "mdapi",
		Short:       "Convert metadata retrieved via Metadata API into the source format used in Salesforce DX projects.",
		Args:        cobra.NoArgs,
		Annotations: map[string]string{commandNameAnnotation: "Mdapi"},
		RunE: func(cmd *cobra.Command, args []string) error {
			rootDir, _ := cmd.Flags().GetString("root-dir")
			if rootDir == "" {
				return missingFlagError("root-dir")
			}
			if info, err := os.Stat(rootDir); err != nil {
				return flagParseError("", "root-dir", "No directory found at "+rootDir)
			} else if !info.IsDir() {
				return flagParseError("", "root-dir", rootDir+" exists but is not a directory")
			}
			manifestPath, _ := cmd.Flags().GetString("manifest")
			metadata, _ := cmd.Flags().GetStringArray("metadata")
			metadataDirs, _ := cmd.Flags().GetStringArray("metadata-dir")
			if manifestPath != "" {
				if info, err := os.Stat(manifestPath); err != nil || info.IsDir() {
					return flagParseError("", "manifest", "No file found at "+manifestPath)
				}
			}
			exclusive := func(flag, value, other string) error {
				return &Error{Message: fmt.Sprintf("The following error occurred:\n  --%s=%s cannot also be provided when using --%s\nSee more help with --help", flag, value, other), ExitCode: 2}
			}
			switch {
			case manifestPath != "" && len(metadata) > 0:
				return exclusive("manifest", manifestPath, "metadata")
			case manifestPath != "" && len(metadataDirs) > 0:
				return exclusive("manifest", manifestPath, "metadata-dir")
			case len(metadata) > 0 && len(metadataDirs) > 0:
				return exclusive("metadata", strings.Join(metadata, ","), "metadata-dir")
			}
			project, err := loadProject()
			if err != nil {
				return err
			}
			outputDir, _ := cmd.Flags().GetString("output-dir")
			if outputDir == "" {
				outputDir = project.defaultPackageDir()
			}
			files, err := readMetadataDir(rootDir)
			if err != nil {
				return err
			}
			query := wildcardQuery
			if manifest, err := readManifest(filepath.Join(rootDir, "package.xml")); err == nil {
				query = manifest.query()
			}
			selected, err := mdapiSelection(rootDir, files, manifestPath, splitList(cmd, "metadata"), metadataDirs)
			if err != nil {
				return err
			}
			components := map[string]bool{}
			for path := range files {
				metadataType, fullName := componentOfPath(path)
				if selected != nil && !selected(metadataType, fullName) {
					delete(files, path)
					continue
				}
				if metadataType != "" {
					components[metadataType+":"+fullName] = true
				}
			}
			if selected != nil && len(components) == 0 {
				return newError("SfError", "No results to format")
			}
			stop := startSpinner(cmd, fmt.Sprintf("Converting %d metadata components", len(components)))
			target, err := filepath.Abs(filepath.Join(outputDir, "main", "default"))
			if err != nil {
				return err
			}
			results, err := writeSourceFormat(target, files, query)
			if err != nil {
				stop("failed")
				return err
			}
			stop("done")
			if results == nil {
				results = []fileResult{}
			}
			return emit(cmd, results, func(w io.Writer) {
				if len(results) == 0 {
					fmt.Fprintln(w, "No metadata found to convert")
					return
				}
				rows := make([][]string, 0, len(results))
				for _, result := range results {
					rows = append(rows, []string{result.State, result.FullName, result.Type, result.FilePath})
				}
				printTable(w, "", []string{"STATE", "FULL NAME", "TYPE", "PROJECT PATH"}, rows)
			})
		},
	}
	addJSONFlag(cmd)
	addAPIVersionFlag(cmd)
	cmd.Flags().StringP("root-dir", "r", "", "Root directory that contains the Metadata API–formatted metadata.")
	cmd.Flags().StringP("output-dir", "d", "", "Directory to store your files in after they're converted to source format.")
	cmd.Flags().StringP("manifest", "x", "", "File path to manifest (package.xml) of metadata types to convert.")
	cmd.Flags().StringArrayP("metadata-dir", "p", nil, "Root of directory or zip file of metadata formatted files to convert.")
	cmd.Flags().StringArrayP("metadata", "m", nil, "Metadata component names to convert.")
	return cmd
}

// stage is one line of the progress display of deploy and retrieve.
type stage struct {
	label   string
	state   string // "", "done", "skipped", "failed"
	elapsed time.Duration
	details []string
}

// formatElapsed formats a duration as the progress display does:
// milliseconds below one second, then seconds, then minutes and seconds.
func formatElapsed(d time.Duration) string {
	switch {
	case d < time.Second:
		return fmt.Sprintf("%dms", d.Milliseconds())
	case d < time.Minute:
		return fmt.Sprintf("%.2fs", d.Seconds())
	}
	minutes := int(d.Minutes())
	return fmt.Sprintf("%dm %.2fs", minutes, d.Seconds()-float64(minutes*60))
}

// wrapLine breaks text at spaces so that no line is wider than width.  As
// in sf's output, the space at a break is dropped, except after a line that
// is exactly width wide, where it starts the next line.
func wrapLine(text string, width int) []string {
	var lines []string
	for len([]rune(text)) > width {
		runes := []rune(text)
		cut := width
		for cut > 0 && runes[cut] != ' ' {
			cut--
		}
		if cut == 0 {
			cut = width
		}
		lines = append(lines, string(runes[:cut]))
		text = string(runes[cut:])
		if cut < width {
			text = strings.TrimPrefix(text, " ")
		}
	}
	return append(lines, text)
}

// writeStages prints the last frame of the progress display sf shows for
// deploy and retrieve.  sf redraws the frame while it waits; this prints it
// once.
func writeStages(w io.Writer, title, subtitle string, stages []stage, info [][2]string) {
	writeStageFrame(w, title, subtitle, true, stages, info)
}

// writeStageFrame is writeStages for a display with or without the message
// block between the title and the stages.  The deploy and retrieve displays
// have one, which takes a line also when it is empty; the scratch org
// displays have none.
func writeStageFrame(w io.Writer, title, subtitle string, messageBlock bool, stages []stage, info [][2]string) {
	const width = 50
	side := (width - len([]rune(title)) - 2) / 2
	rule := strings.Repeat("─", side)
	fmt.Fprintf(w, "\n %s %s %s\n\n", rule, title, rule)
	if subtitle != "" {
		for _, line := range wrapLine(subtitle, 79) {
			fmt.Fprintf(w, " %s\n", line)
		}
	}
	if messageBlock {
		fmt.Fprintln(w)
	}
	for _, s := range stages {
		// In a terminal sf colors the icons and dims the times and the
		// stages that did not run.
		timing := paint(w, styleDim, formatElapsed(s.elapsed))
		switch s.state {
		case "done":
			fmt.Fprintf(w, " %s %s %s\n", paint(w, styleGreen, "✔"), s.label, timing)
		case "failed":
			fmt.Fprintf(w, " %s %s %s\n", paint(w, styleRed, "✘"), s.label, timing)
		case "skipped":
			fmt.Fprintf(w, " %s\n", paint(w, styleDim, "◯ "+s.label+" - Skipped"))
		case "async":
			// A stage that goes on after the command ends.
			fmt.Fprintf(w, " %s %s %s\n", paint(w, styleMagenta, "▶"), s.label, timing)
		default:
			fmt.Fprintf(w, " %s %s\n", paint(w, styleDim, "◼"), s.label)
		}
		for _, detail := range s.details {
			fmt.Fprintf(w, "   ▸ %s\n", detail)
		}
	}
	fmt.Fprintln(w)
	for _, item := range info {
		if 1+len(item[0])+2+len(item[1]) <= 80 {
			fmt.Fprintf(w, " %s: %s\n", item[0], item[1])
			continue
		}
		if strings.Contains(item[1], " ") {
			// A value of several words continues on the next line,
			// under the start of the value.
			indent := strings.Repeat(" ", len(item[0])+3)
			line := ""
			first := true
			flush := func() {
				if first {
					fmt.Fprintf(w, " %s: %s\n", item[0], line)
					first = false
				} else {
					fmt.Fprintf(w, "%s%s\n", indent, line)
				}
			}
			for _, word := range strings.Split(item[1], " ") {
				if line != "" && len(indent)+len(line)+1+len(word) > 80 {
					flush()
					line = word
				} else if line == "" {
					line = word
				} else {
					line += " " + word
				}
			}
			flush()
			continue
		}
		// A value that does not fit on the line follows its label, cut
		// into lines of 79 characters.
		fmt.Fprintf(w, " %s:\n", item[0])
		for value := item[1]; value != ""; {
			cut := 79
			if len(value) < cut {
				cut = len(value)
			}
			fmt.Fprintf(w, " %s\n", value[:cut])
			value = value[cut:]
		}
	}
	fmt.Fprintln(w)
}

func newProjectGenerateManifestCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:         "manifest",
		Short:       "Create a project manifest that lists the metadata components you want to deploy or retrieve.",
		Args:        cobra.NoArgs,
		Annotations: map[string]string{commandNameAnnotation: "ManifestGenerate"},
		RunE: func(cmd *cobra.Command, args []string) error {
			metadata := splitList(cmd, "metadata")
			fromOrg, _ := cmd.Flags().GetString("from-org")
			includePackages := splitList(cmd, "include-packages")
			for _, kind := range includePackages {
				if kind != "managed" && kind != "unlocked" {
					return &Error{Message: fmt.Sprintf("Expected --include-packages=%s to be one of: managed, unlocked\nSee more help with --help", kind), ExitCode: 2}
				}
			}
			if len(includePackages) > 0 && fromOrg == "" {
				return &Error{Message: "The following error occurred:\n  All of the following must be provided when using --include-packages: --from-org\nSee more help with --help", ExitCode: 2}
			}
			sourceDirs := splitList(cmd, "source-dir")
			name, _ := cmd.Flags().GetString("name")
			manifestType, _ := cmd.Flags().GetString("type")
			if name != "" && manifestType != "" {
				return &Error{Message: fmt.Sprintf("The following errors occurred:\n  --name=%s cannot also be provided when using --type\n  --type=%s cannot also be provided when using --name\nSee more help with --help", name, manifestType), ExitCode: 2}
			}
			if len(metadata) == 0 && len(sourceDirs) == 0 && fromOrg == "" {
				return &Error{Message: "provided flags must include at least one of: metadata,source-dir,from-org", ExitCode: 1}
			}
			fileNames := map[string]string{"": "package.xml", "package": "package.xml", "pre": "destructiveChangesPre.xml", "post": "destructiveChangesPost.xml", "destroy": "destructiveChanges.xml"}
			fileName, ok := fileNames[manifestType]
			if !ok {
				return &Error{Message: fmt.Sprintf("Expected --type=%s to be one of: pre, post, destroy, package\nSee more help with --help", manifestType), ExitCode: 1}
			}
			if name != "" {
				fileName = name
				if !strings.HasSuffix(fileName, ".xml") {
					fileName += ".xml"
				}
			}
			project, err := loadProject()
			if err != nil {
				return err
			}
			// Components named with --metadata are listed as given; the
			// components of --source-dir are read from the files.
			types := map[string][]string{}
			add := func(metadataType, member string) {
				if !containsString(types[metadataType], member) {
					types[metadataType] = append(types[metadataType], member)
				}
			}
			for _, entry := range metadata {
				// With --from-org the entries choose among the components
				// of the org, and are not listed themselves.
				if fromOrg != "" {
					break
				}
				metadataType, member, hasMember := strings.Cut(entry, ":")
				if !hasMember {
					member = "*"
				}
				add(metadataType, member)
			}
			if fromOrg != "" {
				org, err := loadOrg(fromOrg)
				if err != nil {
					var actions []string
					if similar := similarOrgName(fromOrg); similar != "" {
						actions = []string{fmt.Sprintf("It looks like you mistyped the username or alias. Did you mean %q?", similar)}
					}
					return &Error{
						Name:     "NamedOrgNotFoundError",
						Message:  fmt.Sprintf("Parsing --from-org \n\tNo authorization information found for %s.\nSee more help with --help", fromOrg),
						ExitCode: 2,
						Actions:  actions,
					}
				}
				// sf asks the org with the API version of the manifest:
				// --api-version, or the sourceApiVersion of the project.
				if version := projectAPIVersion(project); version != "" {
					if err := lib.SetApiVersion(version); err == nil {
						org.Force.Metadata = lib.NewForceMetadata(org.Force)
					}
				}
				orgTypes, err := orgManifestTypes(org, metadata, splitList(cmd, "excluded-metadata"), includePackages)
				if err != nil {
					return err
				}
				for metadataType, members := range orgTypes {
					for _, member := range members {
						add(metadataType, member)
					}
				}
			}
			if len(sourceDirs) > 0 {
				set := newSourceSet()
				set.ignore = loadForceIgnore(project.Root)
				for _, dir := range sourceDirs {
					if err := set.addPath(dir); err != nil {
						return err
					}
				}
				for metadataType, members := range set.types {
					for _, member := range members {
						add(metadataType, member)
					}
				}
			}
			for _, excluded := range splitList(cmd, "excluded-metadata") {
				delete(types, excluded)
			}
			outputDir, _ := cmd.Flags().GetString("output-dir")
			path := fileName
			if outputDir != "" {
				if err := os.MkdirAll(outputDir, 0o755); err != nil {
					return err
				}
				path = filepath.Join(outputDir, fileName)
			}
			if err := os.WriteFile(path, packageXML(types, projectAPIVersion(project)), 0o644); err != nil {
				return err
			}
			result := newOrderedMap()
			if outputDir != "" {
				absolute, _ := filepath.Abs(path)
				result.Set("path", absolute)
			} else {
				result.Set("path", path)
			}
			result.Set("name", fileName)
			return emit(cmd, result, func(w io.Writer) {
				if outputDir != "" {
					fmt.Fprintf(w, "successfully wrote %s to %s\n", fileName, outputDir)
					return
				}
				fmt.Fprintf(w, "successfully wrote %s\n", fileName)
			})
		},
	}
	addJSONFlag(cmd)
	addAPIVersionFlag(cmd)
	cmd.Flags().StringArrayP("metadata", "m", nil, "Names of metadata components to include in the manifest.")
	cmd.Flags().StringArrayP("source-dir", "p", nil, "Paths to the local source files to include in the manifest.")
	cmd.Flags().StringP("name", "n", "", "Name of a custom manifest file to create.")
	cmd.Flags().StringP("type", "t", "", "Type of manifest to create; the type determines the name of the created file.")
	cmd.Flags().StringArrayP("include-packages", "c", nil, "Package types (managed, unlocked) whose metadata is included in the manifest; by default, metadata in managed and unlocked packages is excluded.")
	cmd.Flags().StringArray("excluded-metadata", nil, "Metadata types to exclude when building a manifest from an org.")
	cmd.Flags().String("from-org", "", "Username or alias of the org that contains the metadata components from which to build a manifest.")
	cmd.Flags().StringP("output-dir", "d", "", "Directory to save the created manifest.")
	return cmd
}

func newProjectListIgnoredCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:         "ignored",
		Short:       "Check your local project package directories for forceignored files.",
		Args:        cobra.NoArgs,
		Annotations: map[string]string{commandNameAnnotation: "Ignored"},
		RunE: func(cmd *cobra.Command, args []string) error {
			project, err := loadProject()
			if err != nil {
				return err
			}
			ignore := loadForceIgnore(project.Root)
			var paths []string
			if sourceDir, _ := cmd.Flags().GetString("source-dir"); sourceDir != "" {
				if _, err := os.Stat(sourceDir); err != nil {
					return newError("invalidSourceDir", "File or directory '%s' doesn't exist in your project. Specify one that exists and rerun the command.", sourceDir)
				}
				paths = []string{sourceDir}
			} else {
				for _, directory := range project.PackageDirectories {
					paths = append(paths, directory.Path)
				}
			}
			ignored := []string{}
			for _, path := range paths {
				target := path
				if !filepath.IsAbs(target) && cmd.Flags().Changed("source-dir") == false {
					target = filepath.Join(project.Root, path)
				}
				files, err := ignore.ignoredFiles(target)
				if err != nil {
					return err
				}
				ignored = append(ignored, files...)
			}
			sort.Strings(ignored)
			result := newOrderedMap()
			result.Set("ignoredFiles", ignored)
			return emit(cmd, result, func(w io.Writer) {
				if len(ignored) == 0 {
					fmt.Fprintf(w, "No ignored files found in paths:\n%s\n", strings.Join(paths, "\n"))
					return
				}
				fmt.Fprintf(w, "Found the following ignored files:\n%s\n", strings.Join(ignored, "\n"))
			})
		},
	}
	addJSONFlag(cmd)
	cmd.Flags().StringP("source-dir", "p", "", "File or directory of files that the command checks for foreceignored files.")
	return cmd
}

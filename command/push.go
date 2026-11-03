package command

import (
	"bufio"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/ForceCLI/force/config"
	. "github.com/ForceCLI/force/error"
	. "github.com/ForceCLI/force/lib"
	"github.com/spf13/cobra"
)

func init() {
	// Deploy options
	pushCmd.Flags().BoolP("rollbackonerror", "r", false, "roll back deployment on error")
	pushCmd.Flags().Bool("runalltests", false, "run all tests (equivalent to --testlevel RunAllTestsInOrg)")
	pushCmd.Flags().StringP("testlevel", "l", "NoTestRun", "test level")
	pushCmd.Flags().BoolP("checkonly", "c", false, "check only deploy")
	pushCmd.Flags().BoolP("purgeondelete", "p", false, "purge metadata from org on delete")
	pushCmd.Flags().BoolP("allowmissingfiles", "m", false, "set allow missing files")
	pushCmd.Flags().BoolP("autoupdatepackage", "u", false, "set auto update package")
	pushCmd.Flags().BoolP("ignorewarnings", "i", false, "ignore warnings")

	// Display Options
	pushCmd.Flags().BoolP("ignorecoverage", "w", false, "suppress code coverage warnings")
	pushCmd.Flags().BoolP("suppressunexpected", "U", false, `suppress "An unexpected error occurred" messages`)
	pushCmd.Flags().BoolP("quiet", "q", false, "only output failures")
	pushCmd.Flags().CountP("verbose", "v", "give more verbose output")
	pushCmd.Flags().BoolP("interactive", "I", false, "interactive mode")
	pushCmd.Flags().String("reporttype", "text", "report type format (text or junit)")

	// Ways to push
	pushCmd.Flags().StringSliceP("filepath", "f", []string{}, "Path to resource(s)")
	pushCmd.Flags().StringSliceP("type", "t", []string{}, "Metatdata type")
	pushCmd.Flags().StringSliceP("name", "n", []string{}, "name of metadata object")
	pushCmd.Flags().StringSlice("test", []string{}, "Test(s) to run")
	pushCmd.Flags().Bool("smart-flow-version", false, "enable smart flow versioning (auto-select new version and prune inactive flows)")
	RootCmd.AddCommand(pushCmd)
}

var pushCmd = &cobra.Command{
	Use:   "push [flags]",
	Short: "Deploy metadata from a local directory",
	Long: `
Deploy artifact from a local directory
<metadata>: Accepts either actual directory name or Metadata type
File path can be specified as - to read from stdin; see examples

Files in source (sfdx) format are converted to metadata format before the
deploy. The files of an object (objects/Account/Account.object-meta.xml and
the files under its fields, validationRules, and other component directories)
are composed into objects/Account.object, and a file such as
customMetadata/Type.Record.md-meta.xml is deployed as
customMetadata/Type.Record.md.
`,

	Example: `
  force push -t StaticResource -n MyResource
  force push -t ApexClass
  force push -f metadata/classes/MyClass.cls
  force push -checkonly -test MyClass_Test metadata/classes/MyClass.cls
  force push -n MyApex -n MyObject__c
  force push -f src/objects/Account/fields/Status__c.field-meta.xml
  git diff HEAD^ --name-only --diff-filter=ACM | force push -f -
`,
	DisableFlagsInUseLine: false,
	Run: func(cmd *cobra.Command, args []string) {
		deployOptions := getDeploymentOptions(cmd)
		metadataTypes, _ := cmd.Flags().GetStringSlice("type")
		metadataNames, _ := cmd.Flags().GetStringSlice("name")
		resourcePaths, _ := cmd.Flags().GetStringSlice("filepath")
		// Treat trailing args as file paths
		resourcePaths = append(resourcePaths, args...)

		displayOptions := getDeploymentOutputOptions(cmd)
		if !cmd.Flags().Changed("verbose") {
			displayOptions.verbosity = 1
		}
		smartFlowVersion, _ := cmd.Flags().GetBool("smart-flow-version")
		runPush(metadataTypes, metadataNames, resourcePaths, &deployOptions, displayOptions, smartFlowVersion)
	},
}

func replaceComponentWithBundle(inputPathToFile string) string {
	dirPart, filePart := filepath.Split(inputPathToFile)
	dirPart = filepath.Dir(dirPart)
	if strings.Contains(dirPart, "aura") && filepath.Ext(filePart) != "" && filepath.Base(filepath.Dir(dirPart)) == "aura" {
		inputPathToFile = dirPart
	}
	if strings.Contains(dirPart, "lwc") && filepath.Ext(filePart) != "" && filepath.Base(filepath.Dir(dirPart)) == "lwc" {
		inputPathToFile = dirPart
	}
	return inputPathToFile
}

func runPush(metadataTypes []string, metadataNames []string, resourcePaths []string, deployOptions *ForceDeployOptions, displayOptions *deployOutputOptions, smartFlowVersion bool) {
	if smartFlowVersion {
		// TODO: implement smart flow version logic
	}
	if len(resourcePaths) == 1 && resourcePaths[0] == "-" {
		resourcePaths = make(metaName, 0)
		scanner := bufio.NewScanner(os.Stdin)
		for scanner.Scan() {
			resourcePaths = append(resourcePaths, scanner.Text())
		}
		if err := scanner.Err(); err != nil {
			ErrorAndExit("Error reading stdin")
		}
	}

	if len(metadataTypes) == 0 && len(resourcePaths) == 0 {
		ErrorAndExit("Nothing to push. Please specify metadata components to deploy.")
	}
	if len(metadataNames) > 0 && len(metadataTypes) == 0 {
		ErrorAndExit("The -type (-t) parameter is required.")
	}
	if len(metadataNames) > 0 && len(metadataTypes) > 1 {
		ErrorAndExit("Multiple metadata types and names are not supported")
	}

	if len(resourcePaths) > 0 {
		// It's not a package but does have a path. This could be a path to a file
		// or to a folder. If it is a folder, we pickup the resources a different
		// way than if it's a file.

		// Replace aura/lwc file reference with full bundle folder because only the
		// main component can be deployed by itself.
		resourcepathsToPush := make(metaName, 0)
		for _, fsPath := range resourcePaths {
			resourcepathsToPush = append(resourcepathsToPush, replaceComponentWithBundle(fsPath))
		}
		resourcePaths = resourcepathsToPush

		pushByPaths(resourcePaths, deployOptions, displayOptions, smartFlowVersion)
	} else if len(metadataTypes) == 1 {
		pushByMetadataType(metadataTypes[0], metadataNames, deployOptions, displayOptions, smartFlowVersion)
	} else {
		pushMetadataTypes(metadataTypes, deployOptions, displayOptions, smartFlowVersion)
	}
}

// sourceDirFromPaths finds the directory that holds the metadata type
// directories of the pushed paths. A path in a package directory of an sfdx
// project resolves as described by projectMetadataRoot. For any other path, the
// first path component is that directory when the component after it names a
// metadata type (src/classes/MyClass.cls). Otherwise it is the parent of the
// nearest enclosing metadata type directory, so a path deeper in a project
// (testdata/everything/cachePartitions/aerreg.cachePartition-meta.xml) finds
// testdata/everything. Every path must resolve to the same directory.
func sourceDirFromPaths(resourcePaths []string) string {
	p := ""
	for _, path := range resourcePaths {
		root, ok := projectMetadataRoot(path)
		if !ok {
			root = metadataRootForPath(path)
		}
		root, err := filepath.Abs(root)
		if err != nil {
			fmt.Println("could not detect sourceDir from paths:", err.Error())
			return ""
		}
		if p == "" {
			p = root
		} else if p != root {
			fmt.Println("could not detect sourceDir from paths. " + p + " != " + root)
			return ""
		}
	}
	p, err := filepath.Abs(p)
	if err != nil {
		fmt.Println("could not detect sourceDir from paths:", err.Error())
		return ""
	}
	return p
}

// projectMetadataRoot returns the directory holding the metadata type directory
// that path is in, for a path in a package directory listed in the nearest
// sfdx-project.json above it. Directories below a type's directory can have the
// name of another metadata type (classes/triggers, lwc/settings), so the type's
// directory is the first component below the package directory that names a
// metadata type. It reports false when path is not in a package directory or no
// component below the package directory names a metadata type.
func projectMetadataRoot(path string) (string, bool) {
	absolutePath, err := filepath.Abs(path)
	if err != nil {
		return "", false
	}
	projectDir, packageDirectories, ok := findSfdxProject(filepath.Dir(absolutePath))
	if !ok {
		return "", false
	}
	packageDir := ""
	for _, directory := range packageDirectories {
		candidate := filepath.Join(projectDir, filepath.FromSlash(directory))
		relativePath, err := filepath.Rel(candidate, absolutePath)
		if err == nil && filepath.IsLocal(relativePath) && len(candidate) > len(packageDir) {
			packageDir = candidate
		}
	}
	if packageDir == "" {
		return "", false
	}
	relativePath, err := filepath.Rel(packageDir, absolutePath)
	if err != nil {
		return "", false
	}
	parts := strings.Split(relativePath, string(os.PathSeparator))
	for i, part := range parts {
		if IsMetadataDirectory(part) {
			return filepath.Join(append([]string{packageDir}, parts[:i]...)...), true
		}
	}
	return "", false
}

// findSfdxProject looks in dir and its parents for the nearest
// sfdx-project.json, returning the directory holding it and the paths of its
// package directories. It reports false when there is no readable project file.
func findSfdxProject(dir string) (string, []string, bool) {
	for {
		data, err := os.ReadFile(filepath.Join(dir, "sfdx-project.json"))
		if err == nil {
			var project struct {
				PackageDirectories []struct {
					Path string `json:"path"`
				} `json:"packageDirectories"`
			}
			if err := json.Unmarshal(data, &project); err != nil {
				Log.Info("Could not read", filepath.Join(dir, "sfdx-project.json"), err.Error())
				return "", nil, false
			}
			paths := make([]string, 0, len(project.PackageDirectories))
			for _, packageDirectory := range project.PackageDirectories {
				paths = append(paths, packageDirectory.Path)
			}
			return dir, paths, true
		}
		parent := filepath.Dir(dir)
		if parent == dir {
			return "", nil, false
		}
		dir = parent
	}
}

// metadataRootForPath returns the directory holding the metadata type
// directory that path is in. A path in which no component names a metadata
// type, such as a source directory itself (src), resolves to its first
// component.
func metadataRootForPath(path string) string {
	parts := strings.Split(filepath.Clean(filepath.FromSlash(path)), string(os.PathSeparator))
	if len(parts) > 1 && IsMetadataDirectory(parts[1]) {
		return parts[0]
	}
	for i := len(parts) - 1; i >= 0; i-- {
		if IsMetadataDirectory(parts[i]) {
			if i == 0 {
				return "."
			}
			return strings.Join(parts[:i], string(os.PathSeparator))
		}
	}
	return parts[0]
}

// pushByPaths deploys components by explicit paths, with optional smart flow versioning
func pushByPaths(resourcePaths []string, deployOptions *ForceDeployOptions, displayOptions *deployOutputOptions, smartFlowVersion bool) {
	pb := NewPushBuilder()
	sourceDir := sourceDirFromPaths(resourcePaths)
	var err error
	if sourceDir == "" {
		sourceDir, err = config.GetSourceDir()
		ExitIfNoSourceDir(err)
	}
	pb.Root = sourceDir
	for _, p := range resourcePaths {
		f, err := os.Stat(p)
		if err != nil {
			ErrorAndExit("Could not add %s: %s", p, err.Error())
		}
		if f.Mode().IsDir() {
			err = pb.AddDirectory(p)
		} else {
			err = pb.AddFile(p)
		}
		if err != nil {
			ErrorAndExit("Could not add %s: %s", p, err.Error())
		}
	}
	// Build metadata files
	files := pb.ForceMetadataFiles()

	// Always handle destructive flows (expand non-versioned flows to specific versions)
	files, err = handleDestructiveFlows(force, files)
	if err != nil {
		ErrorAndExit(err.Error())
	}

	// Apply smart flow versioning if enabled
	if smartFlowVersion {
		files, err = processSmartFlowVersion(force, files)
		if err != nil {
			ErrorAndExit(err.Error())
		}
	}
	// Deploy
	err = deploy(force, files, deployOptions, displayOptions)
	if err != nil {
		ErrorAndExit(err.Error())
	}
}

// pushByMetadataType deploys components by metadata type, with optional smart flow versioning
func pushByMetadataType(metadataType string, metadataNames []string, deployOptions *ForceDeployOptions, displayOptions *deployOutputOptions, smartFlowVersion bool) {
	pb := NewPushBuilder()
	sourceDir, err := config.GetSourceDir()
	ExitIfNoSourceDir(err)
	pb.Root = sourceDir
	if len(metadataNames) == 0 {
		err = pb.AddMetadataType(metadataType)
		if err != nil {
			ErrorAndExit(fmt.Sprintf("Failed to add %s: %s", metadataType, err.Error()))
		}
	} else {
		for _, name := range metadataNames {
			err = pb.AddMetadataItem(metadataType, name)
			if err != nil {
				ErrorAndExit(fmt.Sprintf("Failed to add %s: %s", name, err.Error()))
			}
		}
	}

	files := pb.ForceMetadataFiles()

	// Always handle destructive flows (expand non-versioned flows to specific versions)
	files, err = handleDestructiveFlows(force, files)
	if err != nil {
		ErrorAndExit(err.Error())
	}

	// Apply smart flow versioning if enabled
	if smartFlowVersion {
		files, err = processSmartFlowVersion(force, files)
		if err != nil {
			ErrorAndExit(err.Error())
		}
	}
	if err = deploy(force, files, deployOptions, displayOptions); err != nil {
		ErrorAndExit(err.Error())
	}
}

// pushMetadataTypes deploys multiple metadata types, with optional smart flow versioning
func pushMetadataTypes(metadataTypes []string, deployOptions *ForceDeployOptions, displayOptions *deployOutputOptions, smartFlowVersion bool) {
	pb := NewPushBuilder()
	sourceDir, err := config.GetSourceDir()
	ExitIfNoSourceDir(err)
	pb.Root = sourceDir

	for _, metadataType := range metadataTypes {
		err = pb.AddMetadataType(metadataType)
		if err != nil {
			ErrorAndExit(fmt.Sprintf("Failed to add %s: %s", metadataType, err.Error()))
		}
	}

	files := pb.ForceMetadataFiles()

	// Always handle destructive flows (expand non-versioned flows to specific versions)
	files, err = handleDestructiveFlows(force, files)
	if err != nil {
		ErrorAndExit(err.Error())
	}

	// Apply smart flow versioning if enabled
	if smartFlowVersion {
		files, err = processSmartFlowVersion(force, files)
		if err != nil {
			ErrorAndExit(err.Error())
		}
	}
	if err = deploy(force, files, deployOptions, displayOptions); err != nil {
		ErrorAndExit(err.Error())
	}
}

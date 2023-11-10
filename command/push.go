package command

import (
	"bufio"
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
	RootCmd.AddCommand(pushCmd)
}

var pushCmd = &cobra.Command{
	Use:   "push [flags]",
	Short: "Deploy metadata from a local directory",
	Long: `
Deploy artifact from a local directory
<metadata>: Accepts either actual directory name or Metadata type
File path can be specified as - to read from stdin; see examples
`,

	Example: `
  force push -t StaticResource -n MyResource
  force push -t ApexClass
  force push -f metadata/classes/MyClass.cls
  force push -checkonly -test MyClass_Test metadata/classes/MyClass.cls
  force push -n MyApex -n MyObject__c
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
		runPush(metadataTypes, metadataNames, resourcePaths, &deployOptions, displayOptions)
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

func runPush(metadataTypes []string, metadataNames []string, resourcePaths []string, deployOptions *ForceDeployOptions, displayOptions *deployOutputOptions) {
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

		pushByPaths(resourcePaths, deployOptions, displayOptions)
	} else if len(metadataTypes) == 1 {
		pushByMetadataType(metadataTypes[0], metadataNames, deployOptions, displayOptions)
	} else {
		pushMetadataTypes(metadataTypes, deployOptions, displayOptions)
	}
}

func sourceDirFromPaths(resourcePaths []string) string {
	p := ""
	for _, path := range resourcePaths {
		parts := strings.Split(path, string(os.PathSeparator))
		first := parts[0]
		if p == "" {
			p = first
		} else if p != first {
			// We found more than one leading path component
			fmt.Println("could not detect sourceDir from paths. " + p + " != " + first)
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

func pushByPaths(resourcePaths []string, deployOptions *ForceDeployOptions, displayOptions *deployOutputOptions) {
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
	err = deploy(force, pb.ForceMetadataFiles(), deployOptions, displayOptions)
	if err != nil {
		ErrorAndExit(err.Error())
	}
}

func pushByMetadataType(metadataType string, metadataNames []string, deployOptions *ForceDeployOptions, displayOptions *deployOutputOptions) {
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

	err = deploy(force, pb.ForceMetadataFiles(), deployOptions, displayOptions)
	if err != nil {
		ErrorAndExit(err.Error())
	}
}

func pushMetadataTypes(metadataTypes []string, deployOptions *ForceDeployOptions, displayOptions *deployOutputOptions) {
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

	err = deploy(force, pb.ForceMetadataFiles(), deployOptions, displayOptions)
	if err != nil {
		ErrorAndExit(err.Error())
	}
}

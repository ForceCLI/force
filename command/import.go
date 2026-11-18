package command

import (
	"errors"
	"fmt"
	"os"
	"os/user"
	"path/filepath"
	"strings"

	. "github.com/ForceCLI/force/error"
	. "github.com/ForceCLI/force/lib"
	"github.com/spf13/cobra"
)

func init() {
	// Deploy options
	importCmd.Flags().BoolP("rollbackonerror", "r", false, "roll back deployment on error")
	importCmd.Flags().BoolP("runalltests", "t", false, "run all tests (equivalent to --testlevel RunAllTestsInOrg)")
	importCmd.Flags().StringP("testlevel", "l", "NoTestRun", "test level")
	importCmd.Flags().BoolP("checkonly", "c", false, "check only deploy")
	importCmd.Flags().BoolP("purgeondelete", "p", false, "purge metadata from org on delete")
	importCmd.Flags().BoolP("allowmissingfiles", "m", false, "set allow missing files")
	importCmd.Flags().BoolP("autoupdatepackage", "u", false, "set auto update package")
	importCmd.Flags().BoolP("ignorewarnings", "i", false, "ignore warnings")
	importCmd.Flags().StringSliceP("test", "", []string{}, "Test(s) to run")

	// Output options
	importCmd.Flags().BoolP("ignorecoverage", "w", false, "suppress code coverage warnings")
	importCmd.Flags().BoolP("suppressunexpected", "U", true, `suppress "An unexpected error occurred" messages`)
	importCmd.Flags().BoolP("quiet", "q", false, "only output failures")
	importCmd.Flags().BoolP("interactive", "I", false, "interactive mode")
	importCmd.Flags().CountP("verbose", "v", "give more verbose output")
	importCmd.Flags().StringP("reporttype", "f", "text", "report type format (text or junit)")

	importCmd.Flags().StringP("directory", "d", "src", "relative path to package.xml")
	importCmd.Flags().Bool("smart-flow-version", false, "enable smart flow versioning (auto-select new version and prune inactive flows)")

	importCmd.Flags().BoolP("erroronfailure", "E", true, "exit with an error code if any tests fail")

	RootCmd.AddCommand(importCmd)
}

var importCmd = &cobra.Command{
	Use:   "import",
	Short: "Import metadata from a local directory",
	Long: `Deploy the metadata in a directory that holds a package.xml.

The directory may be in metadata format or in source (sfdx) format. Source
format files are converted before they are deployed: the field, record type
and other component files under objects/<Object>/ are composed into the
object's file, a folder file such as reports/Sales.reportFolder-meta.xml is
deployed as reports/Sales-meta.xml, and a -meta.xml file of a type with no
content file (tabs/Project__c.tab-meta.xml) is deployed without the suffix.
The directory's package.xml is deployed as it is.
`,
	Example: `
  force import
  force import -directory=my_metadata -c -r -v
  force import -checkonly -runalltests
`,
	Run: func(cmd *cobra.Command, args []string) {
		options := getDeploymentOptions(cmd)
		srcDir := sourceDir(cmd)

		displayOptions := getDeploymentOutputOptions(cmd)

		smartFlowVersion, _ := cmd.Flags().GetBool("smart-flow-version")
		runImport(srcDir, options, displayOptions, smartFlowVersion)
	},
	Args: cobra.MaximumNArgs(0),
}

func sourceDir(cmd *cobra.Command) string {
	directory, _ := cmd.Flags().GetString("directory")

	wd, _ := os.Getwd()
	var dir string
	usr, err := user.Current()

	//Manually handle shell expansion short cut
	if err != nil {
		if strings.HasPrefix(directory, "~") {
			ErrorAndExit("Cannot determine tilde expansion, please use relative or absolute path to directory.")
		} else {
			dir = directory
		}
	} else {
		if strings.HasPrefix(directory, "~") {
			dir = strings.Replace(directory, "~", usr.HomeDir, 1)
		} else {
			dir = directory
		}
	}

	root := filepath.Join(wd, dir)

	// Check for absolute path
	if filepath.IsAbs(dir) {
		root = dir
	}
	// Normalize so a trailing slash doesn't break path stripping in runImport,
	// where file keys are derived by trimming root+separator from each path.
	return filepath.Clean(root)
}

func runImport(root string, options ForceDeployOptions, displayOptions *deployOutputOptions, smartFlowVersion bool) {
	if _, err := os.Stat(filepath.Join(root, "package.xml")); os.IsNotExist(err) {
		ErrorAndExit(" \n" + filepath.Join(root, "package.xml") + "\ndoes not exist")
	}

	files, err := importFiles(root)
	if err != nil {
		ErrorAndExit(err.Error())
	}

	// Always handle destructive flows (expand non-versioned flows to specific versions)
	files, err = handleDestructiveFlows(force, files)
	if err != nil {
		ErrorAndExit(err.Error())
	}

	// Apply smart flow versioning if enabled
	if smartFlowVersion {
		var err2 error
		files, err2 = processSmartFlowVersion(force, files)
		if err2 != nil {
			ErrorAndExit(err2.Error())
		}
	}
	err = deploy(force, files, &options, displayOptions)
	if err == nil && displayOptions.reportFormat == "text" && !displayOptions.quiet {
		fmt.Printf("Imported from %s\n", root)
	}
	if err != nil && (!errors.Is(err, testFailureError) || displayOptions.errorOnTestFailure) {
		ErrorAndExit(err.Error())
	}
}

// importFiles reads the metadata under root, keyed by the path each file has
// in the deployment. A directory in metadata format is read as it is. A
// directory holding source (sfdx) format files is converted to metadata
// format: an object's field, record type and other component files are
// composed into the object's file, a folder's
// reports/Sales.reportFolder-meta.xml becomes reports/Sales-meta.xml, and a
// -meta.xml file of a type with no content file loses the suffix. The
// directory's own package.xml and destructive changes manifests are deployed
// as they are.
func importFiles(root string) (ForceMetadataFiles, error) {
	sourceFormat, err := HasSourceFormatFiles(root)
	if err != nil {
		return nil, err
	}
	if !sourceFormat {
		return readImportFiles(root, func(string) bool { return true })
	}

	// The manifests, and any file outside a metadata type's directory, are
	// deployed as they are.
	files, err := readImportFiles(root, func(relativePath string) bool {
		topLevel := strings.Split(relativePath, string(os.PathSeparator))[0]
		return !IsMetadataDirectory(topLevel)
	})
	if err != nil {
		return nil, err
	}

	pb := NewPushBuilder()
	pb.Root = root
	entries, err := os.ReadDir(root)
	if err != nil {
		return nil, err
	}
	for _, entry := range entries {
		if !entry.IsDir() || !IsMetadataDirectory(entry.Name()) {
			continue
		}
		if err := pb.AddDirectory(filepath.Join(root, entry.Name())); err != nil {
			return nil, err
		}
	}
	for path, data := range pb.Files {
		files[path] = data
	}
	return files, nil
}

// readImportFiles reads the regular files under root that include accepts,
// keyed by path relative to root.
func readImportFiles(root string, include func(relativePath string) bool) (ForceMetadataFiles, error) {
	files := make(ForceMetadataFiles)
	err := filepath.Walk(root, func(path string, f os.FileInfo, err error) error {
		if err != nil {
			return err
		}
		if !f.Mode().IsRegular() || f.Name() == ".DS_Store" {
			return nil
		}
		relativePath, err := filepath.Rel(root, path)
		if err != nil {
			return err
		}
		if !include(relativePath) {
			return nil
		}
		data, err := os.ReadFile(path)
		if err != nil {
			return err
		}
		files[relativePath] = data
		return nil
	})
	return files, err
}

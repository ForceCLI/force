package main

import (
	"archive/zip"
	"bytes"
	"encoding/xml"
	"errors"
	"fmt"
	"io/ioutil"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"strings"
	"time"
)

var cmdPush = &Command{
	Usage: "push -t <metadata type> -n <metadata name> -f <pathtometadata> [deployment options]",
	Short: "Deploy artifact from a local directory",
	Long: `
Deploy artifact from a local directory
<metadata>: Accepts either actual directory name or Metadata type

Examples:
  force push -t StaticResource -n MyResource
  force push -t ApexClass
  force push -f metadata/classes/MyClass.cls
  force push -checkonly -test MyClass_Test metadata/classes/MyClass.cls
  force push -n MyApex -n MyObject__c

Deployment Options
  -rollbackonerror, -r    Indicates whether any failure causes a complete rollback
  -runalltests, -at       If set all Apex tests defined in the organization are run (equivalent to -l RunAllTestsInOrg)
  -checkonly, -c          Indicates whether classes and triggers are saved during deployment
  -purgeondelete, -p      If set the deleted components are not stored in recycle bin
  -allowmissingfiles, -m  Specifies whether a deploy succeeds even if files missing
  -autoupdatepackage, -u  Auto add files to the package if missing
  -test                   Run tests in class (implies -l RunSpecifiedTests)
  -testlevel, -l          Set test level (NoTestRun, RunSpecifiedTests, RunLocalTests, RunAllTestsInOrg)
  -ignorewarnings, -i     Indicates if warnings should fail deployment or not
`,
}

var (
	namePaths = make(map[string]string)
	byName    = false
)

var (
	resourcepath metaName
	metaFolder   string
)

func init() {
	cmdPush.Flag.BoolVar(rollBackOnErrorFlag, "rollbackonerror", false, "set roll back on error")
	cmdPush.Flag.BoolVar(rollBackOnErrorFlag, "r", false, "set roll back on error")
	cmdPush.Flag.BoolVar(runAllTestsFlag, "runalltests", false, "set run all tests")
	cmdPush.Flag.BoolVar(runAllTestsFlag, "at", false, "set run all tests")
	cmdPush.Flag.StringVar(testLevelFlag, "testlevel", "NoTestRun", "set test level")
	cmdPush.Flag.StringVar(testLevelFlag, "l", "NoTestRun", "set test level")
	cmdPush.Flag.BoolVar(checkOnlyFlag, "checkonly", false, "set check only")
	cmdPush.Flag.BoolVar(checkOnlyFlag, "c", false, "set check only")
	cmdPush.Flag.BoolVar(purgeOnDeleteFlag, "purgeondelete", false, "set purge on delete")
	cmdPush.Flag.BoolVar(purgeOnDeleteFlag, "p", false, "set purge on delete")
	cmdPush.Flag.BoolVar(allowMissingFilesFlag, "allowmissingfiles", false, "set allow missing files")
	cmdPush.Flag.BoolVar(allowMissingFilesFlag, "m", false, "set allow missing files")
	cmdPush.Flag.BoolVar(autoUpdatePackageFlag, "autoupdatepackage", false, "set auto update package")
	cmdPush.Flag.BoolVar(autoUpdatePackageFlag, "u", false, "set auto update package")
	cmdPush.Flag.BoolVar(ignoreWarningsFlag, "ignorewarnings", false, "set ignore warnings")
	cmdPush.Flag.BoolVar(ignoreWarningsFlag, "i", false, "set ignore warnings")

	cmdPush.Flag.Var(&resourcepath, "f", "Path to resource(s)")
	cmdPush.Flag.Var(&resourcepath, "filepath", "Path to resource(s)")
	cmdPush.Flag.Var(&testsToRun, "test", "Test(s) to run")
	cmdPush.Flag.StringVar(&metadataType, "t", "", "Metatdata type")
	cmdPush.Flag.StringVar(&metadataType, "type", "", "Metatdata type")
	cmdPush.Flag.Var(&metadataName, "name", "name of metadata object")
	cmdPush.Flag.Var(&metadataName, "n", "names of metadata object")
	cmdPush.Run = runPush
}

func argIsFile(fpath string) bool {
	if _, err := os.Stat(fpath); err != nil {
		return false
	}
	return true
}

func runPush(cmd *Command, args []string) {
	var subcommand = strings.ToLower(metadataType)

	switch subcommand {
	case "package":
		pushPackage()
	default:
		resourcepath = append(resourcepath, args...)
		if len(resourcepath) != 0 {
			// It's not a package but does have a path. This could be a path to a file
			// or to a folder. If it is a folder, we pickup the resources a different
			// way than if it's a file.
			validatePushByMetadataTypeCommand()
			if len(metadataType) != 0 {
				pushByTypeAndPath()
			} else {
				pushByPathOnly()
			}
		} else {
			if len(metadataName) > 0 {
				if len(metadataType) != 0 {
					validatePushByMetadataTypeCommand()
					pushByMetadataType()
				} else {
					ErrorAndExit("The -type (-t) parameter is required.")
					//isValidMetadataType()
					//pushByName()
					//validatePushByMetadataTypeCommand()
					//pushByMetadataType()
				}
			} else {
				validatePushByMetadataTypeCommand()
				pushByMetadataType()
			}
		}
	}
}

func pushByPathOnly() {
	pushByPath(resourcepath)
}

func pushByTypeAndPath() {
	for _, name := range resourcepath {
		fi, err := os.Stat(name)
		if err != nil {
			ErrorAndExit(err.Error())
		}
		if fi.IsDir() {

		}

		fn := filepath.Base(name)
		fn = strings.Replace(fn, filepath.Ext(fn), "", -1)
		metadataName = append(metadataName, fn)
	}
}

func isValidMetadataType() {
	fmt.Printf("Validating and deploying push...\n")
	// Look to see if we can find any resource for that metadata type
	root, err := GetSourceDir()
	ExitIfNoSourceDir(err)
	metaFolder = findMetadataTypeFolder(metadataType, root)
	if metaFolder == "" {
		ErrorAndExit("No folders that contain %s metadata could be found.", metadataType)
	}
}

func metadataExists() {
	if len(metadataName) == 0 {
		return
	} else {
		valid := true
		message := ""
		// Go throug the metadata folder to find the named resources
		for _, name := range metadataName {
			if len(wildCardSearch(metaFolder, strings.Split(name, ".")[0])) == 0 {
				message += fmt.Sprintf("\nINVALID: No resource named %s found in %s", name, metaFolder)
				valid = false
			}
		}
		if !valid {
			ErrorAndExit(message)
		}
	}
}

func validatePushByMetadataTypeCommand() {
	isValidMetadataType()
	metadataExists()
}

func wildCardSearch(metaFolder string, name string) []string {
	cmd := exec.Command("ls", metaFolder)
	var out bytes.Buffer
	cmd.Stdout = &out
	err := cmd.Run()
	if err != nil {
		ErrorAndExit(err.Error())
	}
	f := strings.Split(out.String(), "\n")
	var ret []string
	for _, s := range f {
		ss := filepath.Base(s)
		ss = strings.Split(ss, ".")[0]
		if ss == name {
			ret = append(ret, s)
		}
	}
	return ret
	//return contains(f, name)
}

func contains(s []string, e string) bool {
	for _, a := range s {
		if strings.Contains(a, e) {
			return true
		}
	}
	return false
}

func pushPackage() {
	if len(resourcepath) == 0 {
		var packageFolder = findPackageFolder(metadataName[0])
		zipResource(packageFolder, metadataName[0])
		resourcepath.Set(packageFolder + ".resource")
		//var dir, _ = os.Getwd();
		//ErrorAndExit(fmt.Sprintf("No resource path sepcified. %s, %s", metadataName[0], dir))
	}
	deployPackage()
}

// Return the name of the first element of an XML file. We need this
// because the metadata xml uses the metadata type as the first element
// in the metadata xml definition. Could be a better way of doing this.
func getMDTypeFromXml(path string) (mdtype string, err error) {
	xmlFile, err := ioutil.ReadFile(path)
	mdtype = getFirstXmlElement(xmlFile)
	return
}

// Helper function to read the first element of an XML file.
func getFirstXmlElement(xmlFile []byte) (firstElement string) {
	decoder := xml.NewDecoder(strings.NewReader(string(xmlFile)))
	for {
		token, _ := decoder.Token()
		if token == nil {
			break
		}
		switch startElement := token.(type) {
		case xml.StartElement:
			firstElement = startElement.Name.Local
			return
		}
	}
	return
}

// Look for xml files. When one is found, check the first element of the
// XML. It should be the metadata type as expected by the platform.  See
// if it matches the type passed in on mdtype, and if so, return the folder
// that contains the xml file, then bail out.  If no file is found for the
// passed in type, then folder is empty.
func findMetadataTypeFolder(mdtype string, root string) (folder string) {
	filepath.Walk(root, func(path string, f os.FileInfo, err error) error {
		//base := filepath.Base(path)
		//if filepath.Ext(base) == ".object" {
		firstEl, _ := getMDTypeFromXml(path)
		if firstEl == mdtype {
			folder = filepath.Dir(path)
			return errors.New("walk canceled")
		}
		//}
		return nil
	})
	return
}

func findPackageFolder(packageName string) (folder string) {
	var wd, _ = os.Getwd()
	// We need to start at the metadata folder, go down first
	folder = findMetadataFolder(wd)
	if len(folder) == 0 {
		// Didn't find it, error out
		fmt.Println("Could not find metadata folder.")
	}
	if _, err := os.Stat(filepath.Join(folder, packageName)); err == nil {
		folder = filepath.Join(folder, packageName)
	}
	return
}

func findMetadataFolder(dir string) (folderPath string) {
	filepath.Walk(dir, func(path string, f os.FileInfo, err error) error {
		if filepath.Base(path) == "metadata" {
			folderPath = path
			return errors.New("walk cancelled")
		}
		return nil
	})
	if len(folderPath) == 0 {
		// not down, so, go up
		for dir != string(os.PathSeparator) {
			dir = filepath.Dir(dir)
			if filepath.Base(dir) == "metadata" {
				folderPath = dir
				return
			}
		}
	}
	return
}

func FilenameMatchesMetadataName(filename string, metadataName string) bool {
	// Strip off the extension, plus "-meta.xml" if it's appended to the
	// extension
	re := regexp.MustCompile(`\.[^.]+(-meta\.xml)?$`)
	trimmed := re.ReplaceAllString(filename, "")
	return trimmed == metadataName
}

// This method will use the type that is passed to the -type flag to find all
// metadata that matches that type.  It will also filter on the metadata
// name(s) passed on the -name flag(s). This method also looks for unpacked
// static resource so that it can repack them and update the actual ".resource"
// file.
func pushByMetadataType() {
	byName = true

	// Walk the metaFolder obtained during validation and compile a list of resources
	// to be added to the package.
	var files []string
	filepath.Walk(metaFolder, func(path string, f os.FileInfo, err error) error {
		// Check to see if this is a folder. This will be the case with static resources
		// that have been unpacked.  Not entirely sure if this is the only time we will
		// find a folder inside a metadata type folder.
		if f.IsDir() {
			// Check to see if any names where specified in the -name flag
			if len(metadataName) == 0 {
				// Take all
				zipResource(path, "")
			} else {
				for _, name := range metadataName {
					fname := filepath.Base(path)
					// Check to see if the resource name matches the one of the ones passed on the -name flag
					if fname == name {
						zipResource(path, "")
					}
				}
			}
			return nil
		}

		// These should be file resources, but, could be child folders of unzipped resources in
		// which case we will have handled them above.
		if filepath.Dir(path) != metaFolder && !f.IsDir() {
			return nil
		}
		// Again, if no names where specifed on -name flag, just add the file.
		if len(metadataName) == 0 {
			files = append(files, path)
		} else {
			// iterate the -name flag values looking for the ones specified
			for _, name := range metadataName {
				// Check if the file matches the metadata named.  For example, for
				// custom objects, the Account.object file matches the metadata
				// name Account.  For metadata types stored with separate -meta.xml
				// files, both files should match, e.g. HelloWorld.cls and
				// HelloWorld.cls-meta.xml.  For custom metadata named
				// My_Type.My_Object, the file My_Type.My_Object.md will match.
				if FilenameMatchesMetadataName(filepath.Base(path), name) {
					files = append(files, path)
				}
			}
		}

		return nil
	})

	// Push these files to the package maker/sender
	pushByPaths(files)
}

// Just zip up what ever is in the path
func zipResource(path string, topLevelFolder string) {
	zipfile := new(bytes.Buffer)
	zipper := zip.NewWriter(zipfile)
	startPath := path + "/"
	filepath.Walk(path, func(path string, f os.FileInfo, err error) error {
		if filepath.Base(path) != ".DS_Store" {
			// Can skip dirs since the dirs will be created when the files are added
			if !f.IsDir() {
				file, err := ioutil.ReadFile(path)
				if err != nil {
					return err
				}
				fl, err := zipper.Create(filepath.Join(topLevelFolder, strings.Replace(path, startPath, "", -1)))
				if err != nil {
					ErrorAndExit(err.Error())
				}
				_, err = fl.Write([]byte(file))
				if err != nil {
					ErrorAndExit(err.Error())
				}
			}
		}
		return nil
	})

	zipper.Close()
	zipdata := zipfile.Bytes()
	ioutil.WriteFile(path+".resource", zipdata, 0644)
	return
}

func pushByName() {

	byName = true

	root, err := GetSourceDir()
	ExitIfNoSourceDir(err)

	// Find file by walking directory and ignoring extension
	var paths []string
	err = filepath.Walk(root, func(path string, f os.FileInfo, err error) error {
		if f.IsDir() {
			// Check to see if any names where specified in the -name flag
			if len(metadataName) == 0 {
				// Take all
				zipResource(path, "")
			} else {
				for _, name := range metadataName {
					fname := filepath.Base(path)
					// Check to see if the resource name matches the one of the ones passed on the -name flag
					if fname == name {
						metadataType = strings.ToLower(filepath.Base(filepath.Dir(path)))
						if metadataType == "staticresources" {
							metadataType = "StaticResource"
						}
						zipResource(path, "")
					}
				}
			}
			return nil
		}

		if f.Mode().IsRegular() {
			fname := strings.TrimSuffix(f.Name(), filepath.Ext(f.Name()))
			for _, name := range metadataName {
				if strings.EqualFold(fname, name) {
					if len(metadataType) == 0 {
						metadataType = strings.ToLower(filepath.Base(filepath.Dir(path)))
						if metadataType == "staticresources" {
							metadataType = "StaticResource"
						}
					}
					paths = append(paths, path)
				}
			}
		}
		return nil
	})
	if err != nil {
		ErrorAndExit(err.Error())
	}
	if len(paths) == 0 {
		ErrorAndExit("Could not find %#v ", metadataName)
	}

}

// Wrapper to handle a single resource path
func pushByPath(fpath []string) {
	pushByPaths(fpath)
}

// Creates a package that includes everything in the passed in string slice
// and then deploys the package to salesforce
func pushByPaths(fpaths []string) {
	pb := NewPushBuilder()

	var badPaths []string
	for _, fpath := range fpaths {
		name, err := pb.AddFile(fpath)
		if err != nil {
			fmt.Println(err.Error())
			badPaths = append(badPaths, fpath)
		} else {
			// Store paths by name for error messages
			namePaths[name] = fpath
		}
	}

	if len(badPaths) == 0 {
		fmt.Println("Deploying now...")
		t0 := time.Now()
		deployFiles(pb.ForceMetadataFiles())
		t1 := time.Now()
		fmt.Printf("The deployment took %v to run.\n", t1.Sub(t0))
	} else {
		ErrorAndExit("Could not add the following files:\n {}", strings.Join(badPaths, "\n"))
	}
}

// Deploy a previously create package. This is used for "force push package". In this case the
// --path flag should be pointing to a zip file that may or may not have come from a different
// org altogether
func deployPackage() {
	force, _ := ActiveForce()
	DeploymentOptions := deployOpts()
	for _, name := range resourcepath {
		zipfile, err := ioutil.ReadFile(name)
		result, err := force.Metadata.DeployZipFile(force.Metadata.MakeDeploySoap(*DeploymentOptions), zipfile)
		processDeployResults(result, err)
	}
	return
}

func deployFiles(files ForceMetadataFiles) {
	force, _ := ActiveForce()
	var DeploymentOptions = deployOpts()
	result, err := force.Metadata.Deploy(files, *DeploymentOptions)
	processDeployResults(result, err)
	return
}

func deployOpts() *ForceDeployOptions {
	var opts ForceDeployOptions
	opts.AllowMissingFiles = *allowMissingFilesFlag
	opts.AutoUpdatePackage = *autoUpdatePackageFlag
	opts.CheckOnly = *checkOnlyFlag
	opts.IgnoreWarnings = *ignoreWarningsFlag
	opts.PurgeOnDelete = *purgeOnDeleteFlag
	opts.RollbackOnError = *rollBackOnErrorFlag
	opts.TestLevel = *testLevelFlag
	if *runAllTestsFlag {
		opts.TestLevel = "RunAllTestsInOrg"
	}
	opts.RunTests = testsToRun
	return &opts
}

// Process and display the result of the push operation
func processDeployResults(result ForceCheckDeploymentStatusResult, err error) {
	if err != nil {
		ErrorAndExit(err.Error())
	}

	problems := result.Details.ComponentFailures
	successes := result.Details.ComponentSuccesses
	testFailures := result.Details.RunTestResult.TestFailures
	testSuccesses := result.Details.RunTestResult.TestSuccesses

	if len(problems) > 0 {
		fmt.Printf("\nFailures - %d\n", len(problems))
		for _, problem := range problems {
			if problem.FullName == "" {
				fmt.Println(problem.Problem)
			} else {
				if byName {
					fmt.Printf("ERROR with %s, line %d\n %s\n", problem.FullName, problem.LineNumber, problem.Problem)
				} else {
					fname, found := namePaths[problem.FullName]
					if !found {
						fname = problem.FullName
					}
					fmt.Printf("\"%s\", line %d: %s %s\n", fname, problem.LineNumber, problem.ProblemType, problem.Problem)
				}
			}
		}
	}

	if len(successes) > 0 {
		fmt.Printf("\nSuccesses - %d\n", len(successes)-1)
		for _, success := range successes {
			if success.FullName != "package.xml" {
				verb := "unchanged"
				if success.Changed {
					verb = "changed"
				} else if success.Deleted {
					verb = "deleted"
				} else if success.Created {
					verb = "created"
				}
				fmt.Printf("\t%s: %s\n", success.FullName, verb)
			}
		}
	}

	fmt.Printf("\nTest Successes - %d\n", len(testSuccesses))
	for _, failure := range testSuccesses {
		fmt.Printf("  [PASS]  %s::%s\n", failure.Name, failure.MethodName)
	}

	fmt.Printf("\nTest Failures - %d\n", len(testFailures))
	for _, failure := range testFailures {
		fmt.Printf("\n  [FAIL]  %s::%s: %s\n", failure.Name, failure.MethodName, failure.Message)
		fmt.Println(failure.StackTrace)
	}

	// Handle notifications
	notifySuccess("push", len(problems) == 0)
}

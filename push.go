package main

import (
	"fmt"
	"io/ioutil"
	"os"
	"path/filepath"
	"strings"
)

var cmdPush = &Command{
	Run:   runPush,
	Usage: "push (<metadata> <name> | <file>...)",
	Short: "Deploy artifact from a local directory",
	Long: `
Deploy artifact from a local directory
<metadata>: Accepts either actual directory name or Metadata type

Examples:
  force push classes MyClass
  force push ApexClass MyClass
  force push src/classes/MyClass.cls
`,
}

var xmlBody = `<?xml version="1.0" encoding="UTF-8"?>
<Package xmlns="http://soap.sforce.com/2006/04/metadata">%s
    <version>29.0</version>
</Package>`

var xmlType = `
    <types>%s
        <name>%s</name>
    </types>`
var xmlMember = `
        <members>%s</members>`

type metapath struct {
	path string
	name string
}

var metapaths = []metapath{
	metapath{"classes", "ApexClass"},
	metapath{"objects", "CustomObject"},
	metapath{"tabs", "CustomTab"},
	metapath{"flexipages", "FlexiPage"},
	metapath{"components", "ApexComponent"},
	metapath{"triggers", "ApexTrigger"},
	metapath{"pages", "ApexPage"},
}

func getPathForMeta(metaname string) string {
	for _, mp := range metapaths {
		if strings.ToLower(mp.name) == strings.ToLower(metaname) {
			return mp.path
		}
	}

	// Unknown, so use metaname
	return metaname
}

func getMetaForPath(path string) string {
	for _, mp := range metapaths {
		if mp.path == path {
			return mp.name
		}
	}

	// Unknown, so use path
	return path
}

func argIsFile(fpath string) bool {
	if _, err := os.Stat(fpath); err != nil {
		return false
	}
	return true
}

func runPush(cmd *Command, args []string) {
	if len(args) == 0 {
		cmd.printUsage()
		return
	}

	if argIsFile(args[0]) {
		fpath := args[0]
		pushByPaths(args)

		fmt.Printf("Pushed %s to Force.com\n", fpath)
		return
	}

	if len(args) == 2 {
		// If arg[0] is already path or meta, the method will return arg[0]
		objPath := getPathForMeta(args[0])
		objName := args[1]
		pushByName(objPath, objName)

		fmt.Printf("Pushed %s to Force.com\n", objName)
		return
	}

	fmt.Println("Could not find file or determine metadata")

	// If we got here, something is not valid
	cmd.printUsage()
}

func pushByName(objPath string, objName string) {
	wd, _ := os.Getwd()

	// First try for metadata directory
	root := filepath.Join(wd, "metadata")
	if _, err := os.Stat(filepath.Join(root, "package.xml")); os.IsNotExist(err) {
		// If not found, try for src directory
		root = filepath.Join(wd, "src")
		if _, err := os.Stat(filepath.Join(root, "package.xml")); os.IsNotExist(err) {
			ErrorAndExit("Current directory must contain a src or metadata directory")
		}
	}

	if _, err := os.Stat(filepath.Join(root, objPath)); os.IsNotExist(err) {
		ErrorAndExit("Folder " + objPath + " not found, must specify valid metadata")
	}

	// Find file by walking directory and ignoring extension
	found := false
	fpath := ""
	err := filepath.Walk(filepath.Join(root, objPath), func(path string, f os.FileInfo, err error) error {
		if f.Mode().IsRegular() {
			fname := strings.ToLower(f.Name())
			fname = strings.TrimSuffix(fname, filepath.Ext(fname))
			if strings.ToLower(fname) == strings.ToLower(objName) {
				found = true
				fpath = filepath.Join(root, objPath, f.Name())
			}
		}
		return nil
	})
	if err != nil {
		ErrorAndExit(err.Error())
	}
	if !found {
		ErrorAndExit("Could not find " + objName + " in " + objPath)
	}

	pushByPath(fpath)
}

func pushByPath(fpath string) {
	pushByPaths([]string{fpath})
}

// Push metadata object by path to a file
func pushByPaths(fpaths []string) {
	files := make(ForceMetadataFiles)
	xmlMap := make(map[string][]string)

	for _, fpath := range fpaths {
		addFile(files, xmlMap, fpath)
	}

	files["package.xml"] = buildXml(xmlMap)

	deployFiles(files)
}

func addFile(files ForceMetadataFiles, xmlMap map[string][]string, fpath string) {
	fpath, err := filepath.Abs(fpath)
	if err != nil {
		ErrorAndExit("Cound not find " + fpath)
	}
	if _, err := os.Stat(fpath); err != nil {
		ErrorAndExit("Cound not open " + fpath)
	}

	hasMeta := true
	fname := filepath.Base(fpath)
	fname = strings.TrimSuffix(fname, filepath.Ext(fname))
	fdir := filepath.Dir(fpath)
	typePath := filepath.Base(fdir)
	srcDir := filepath.Dir(fdir)
	metaType := getMetaForPath(typePath)
	// Should be present since we worked back to srcDir
	frel, _ := filepath.Rel(srcDir, fpath)

	// Try to find meta file
	fmeta := fpath + "-meta.xml"
	fmetarel := ""
	if _, err := os.Stat(fmeta); err != nil {
		if os.IsNotExist(err) {
			hasMeta = false
		} else {
			ErrorAndExit("Cound not open " + fmeta)
		}
	} else {
		// Should be present since we worked back to srcDir
		fmetarel, _ = filepath.Rel(srcDir, fmeta)
	}

	xmlMap[metaType] = append(xmlMap[metaType], fname)

	fdata, err := ioutil.ReadFile(fpath)
	files[frel] = fdata
	if hasMeta {
		fdata, err = ioutil.ReadFile(fmeta)
		files[fmetarel] = fdata
	}
}

func buildXml(xmlMap map[string][]string) []byte {
	var typeXml string
	for metaType, members := range xmlMap {
		fmt.Println("Type: " + metaType)
		var membersXml string
		for _, member := range members {
			fmt.Println("member: " + member)
			membersXml += fmt.Sprintf(xmlMember, member)
		}

		if membersXml != "" {
			typeXml += fmt.Sprintf(xmlType, membersXml, metaType)
		}
	}

	bodyXml := fmt.Sprintf(xmlBody, typeXml)

	return []byte(bodyXml)
}

func deployFiles(files ForceMetadataFiles) {
	force, _ := ActiveForce()
	var DeploymentOptions ForceDeployOptions
	successes, problems, err := force.Metadata.Deploy(files, DeploymentOptions)
	if err != nil {
		ErrorAndExit(err.Error())
	}
	fmt.Printf("\nFailures - %d\n", len(problems))
	for _, problem := range problems {
		if problem.FullName == "" {
			fmt.Println(problem.Problem)
		} else {
			fmt.Printf("ERROR with %s:\n %s\n", problem.FullName, problem.Problem)
		}
	}

	fmt.Printf("\nSuccesses - %d\n", len(successes))
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
			fmt.Printf("%s\n\tstatus: %s\n\tid=%s\n", success.FullName, verb, success.Id)
		}
	}
}

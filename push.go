package main

import (
	"encoding/xml"
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

// Structs for XML building
type Package struct {
	Xmlns   string     `xml:"xmlns,attr"`
	Types   []MetaType `xml:"types"`
	Version string     `xml:"version"`
}

type MetaType struct {
	Members []string `xml:"members"`
	Name    string   `xml:"name"`
}

func createPackage() Package {
	return Package{
		Version: strings.TrimPrefix(apiVersion, "v"),
		Xmlns:   "http://soap.sforce.com/2006/04/metadata",
	}
}

type metapath struct {
	path string
	name string
}

var metapaths = []metapath{
	metapath{"classes", "ApexClass"},
	metapath{"objects", "CustomObject"},
	metapath{"tabs", "CustomTab"},
	metapath{"labels", "CustomLabels"},
	metapath{"flexipages", "FlexiPage"},
	metapath{"components", "ApexComponent"},
	metapath{"triggers", "ApexTrigger"},
	metapath{"pages", "ApexPage"},
}

var namePaths = make(map[string]string)
var byName = false

func getPathForMeta(metaname string) string {
	for _, mp := range metapaths {
		if strings.EqualFold(mp.name, metaname) {
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
		pushByPaths(args)
		return
	}

	if len(args) == 2 {
		// If arg[0] is already path or meta, the method will return arg[0]
		objPath := getPathForMeta(args[0])
		objName := args[1]
		pushByName(objPath, objName)
		return
	}

	fmt.Println("Could not find file or determine metadata")

	// If we got here, something is not valid
	cmd.printUsage()
}

func pushByName(objPath string, objName string) {
	wd, _ := os.Getwd()
	byName = true

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
	var fpath string
	err := filepath.Walk(filepath.Join(root, objPath), func(path string, f os.FileInfo, err error) error {
		if f.Mode().IsRegular() {
			fname := strings.TrimSuffix(f.Name(), filepath.Ext(f.Name()))
			if strings.EqualFold(fname, objName) {
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
		name := addFile(files, xmlMap, fpath)
		// Store paths by name for error messages
		namePaths[name] = fpath
	}

	files["package.xml"] = buildXml(xmlMap)

	deployFiles(files)
}

func addFile(files ForceMetadataFiles, xmlMap map[string][]string, fpath string) string {
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

	return fname
}

func buildXml(xmlMap map[string][]string) []byte {
	p := createPackage()

	for metaType, members := range xmlMap {
		t := MetaType{Name: metaType}
		for _, member := range members {
			t.Members = append(t.Members, member)
		}
		p.Types = append(p.Types, t)
	}

	byteXml, _ := xml.MarshalIndent(p, "", "    ")
	byteXml = append([]byte(xml.Header), byteXml...)

	return byteXml
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
			fmt.Printf("%s\n\tstatus: %s\n\tid=%s\n", success.FullName, verb, success.Id)
		}
	}
}

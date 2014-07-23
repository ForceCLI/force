package main

import (
	"encoding/json"
	"fmt"
	"io/ioutil"
	"os"
	"path/filepath"
	"strings"
)

var cmdPushAura = &Command{
	Usage: "pushAura",
	Short: "force pushAura -filename=<filepath>",
	Long: `
	force pushAura -filename <fullFilePath>

	force pushAura -f=<fullFilePath>

	`,
}

func init() {
	cmdPushAura.Run = runPushAura
	cmdPushAura.Flag.StringVar(fileName, "f", "", "fully qualified file name for entity")
	cmdPushAura.Flag.BoolVar(isBundle, "b", false, "Creating a bundle or not")
	cmdPushAura.Flag.StringVar(createType, "t", "", "Type of entity or bundle to create")
}

var (
	fileName   = cmdPushAura.Flag.String("filepath", "", "fully qualified file name for entity")
	isBundle   = cmdPushAura.Flag.Bool("isBundle", false, "Creating a bundle or not")
	createType = cmdPushAura.Flag.String("auraType", "", "Type of entity or bundle to create")
)

func runPushAura(cmd *Command, args []string) {
	force, _ := ActiveForce()

	if _, err := os.Stat(*fileName); os.IsNotExist(err) {
		ErrorAndExit("File does not exist\n" + *fileName)
	}

	// Verify that the file is in an aura bundles folder
	if !inAuraBundlesFolder() {
		ErrorAndExit("File is not in an aura bundle folder (aurabundles")
	}

	// Check for manifest file
	if _, err := os.Stat(filepath.Join(filepath.Dir(*fileName), ".manifest")); os.IsNotExist(err) {
		// No manifest, but is in aurabundle folder, assume creating a new bundle with this file
		// as the first artifact.
		createNewAuraBundleAndDefinition(*force)
	} else {
		// Got the manifest, let's update the artifact
		updateAuraDefinition(*force)
		return
	}

}

func isValidAuraExtension() bool {
	var ext = strings.Trim(strings.ToLower(filepath.Ext(*fileName)), " ")
	fmt.Printf("[%s]\n", ext)
	if ext == ".app" || ext == ".cmp" || ext == ".evt" {
		return true
	} else {
		ErrorAndExit("You need to create an application (.app) or component (.cmp) or and event (.evt) as the first item in your bundle.")
	}
	return false
}

func createNewAuraBundleAndDefinition(force Force) {
	// 	Creating a new bundle. We need
	// 		the name of the bundle (parent folder of file)
	//		the type of artifact (based on naming convention)
	// 		the contents of the file
	fmt.Println("Creating new bundle")
	if isValidAuraExtension() {
		// Need the parent folder name to name the bundle
		var bundleName = filepath.Base(filepath.Dir(*fileName))
		// Create the manifext
		var manifest BundleManifest
		manifest.Name = bundleName

		_, _ = getFormatByFileName()

		// Create a bundle defintion
		bundle, err := force.CreateAuraBundle(bundleName)
		if err != nil {
			ErrorAndExit(err.Error())
		}
		manifest.Id = bundle.Id
		component, err := createBundleEntity(manifest, force)
		if err != nil {
			ErrorAndExit(err.Error())
		}
		createManifest(manifest, component)
	}
}

func createBundleEntity(manifest BundleManifest, force Force) (component ForceCreateRecordResult, err error) {
	// create the bundle entity
	format, deftype := getFormatByFileName()
	mbody, _ := readFile(*fileName)
	component, err = force.CreateAuraComponent(map[string]string{"AuraDefinitionBundleId": manifest.Id, "DefType": deftype, "Format": format, "Source": mbody})
	return
}

func createManifest(manifest BundleManifest, component ForceCreateRecordResult) {
	cfile := ComponentFile{}
	cfile.FileName = *fileName
	cfile.ComponentId = component.Id

	manifest.Files = append(manifest.Files, cfile)
	bmBody, _ := json.Marshal(manifest)

	ioutil.WriteFile(filepath.Join(filepath.Dir(*fileName), ".manifest"), bmBody, 0644)
	return
}

func updateManifest(manifest BundleManifest, component ForceCreateRecordResult) {
	cfile := ComponentFile{}
	cfile.FileName = *fileName
	cfile.ComponentId = component.Id

	manifest.Files = append(manifest.Files, cfile)
	bmBody, _ := json.Marshal(manifest)

	ioutil.WriteFile(filepath.Join(filepath.Dir(*fileName), ".manifest"), bmBody, 0644)
	return
}

func updateAuraDefinition(force Force) {

	//Get the manifest
	mbody, _ := readFile(filepath.Join(filepath.Dir(*fileName), ".manifest"))

	var manifest BundleManifest
	json.Unmarshal([]byte(mbody), &manifest)

	for i := range manifest.Files {
		component := manifest.Files[i]
		if filepath.Base(component.FileName) == filepath.Base(*fileName) {
			//Here is where we make the call to send the update
			mbody, _ = readFile(*fileName)
			err := force.UpdateAuraComponent(map[string]string{"source": mbody}, component.ComponentId)
			if err != nil {
				ErrorAndExit(err.Error())
			}
			fmt.Printf("Aura definition updated: %s\n", filepath.Base(*fileName))
			return
		}
	}
	fmt.Println("Creating new bundle entity")
	component, err := createBundleEntity(manifest, force)
	if err != nil {
		ErrorAndExit(err.Error())
	}
	updateManifest(manifest, component)
	fmt.Println("New component in the bundle")
}

func getFormatByFileName() (format string, defType string) {
	var fname = strings.ToLower(*fileName)
	if strings.Contains(fname, "application.app") {
		format = "XML"
		defType = "APPLICATION"
	} else if strings.Contains(fname, "component.cmp") {
		format = "XML"
		defType = "COMPONENT"
	} else if strings.Contains(fname, "event.evt") {
		format = "XML"
		defType = "EVENT"
	} else if strings.Contains(fname, "controller.js") {
		format = "JS"
		defType = "CONTROLLER"
	} else if strings.Contains(fname, "model.js") {
		format = "JS"
		defType = "MODEL"
	} else if strings.Contains(fname, "helper.js") {
		format = "JS"
		defType = "HELPER"
	} else if strings.Contains(fname, "style.css") {
		format = "CSS"
		defType = "STYLE"
	} else {
		ErrorAndExit("Could not determine aura definition type.")
	}
	return
}

func getDefinitionFormat(deftype string) (result string) {
	switch strings.ToUpper(deftype) {
	case "APPLICATION", "COMPONENT", "EVENT":
		result = "XML"
	case "CONTROLLER", "MODEL", "HELPER":
		result = "JS"
	case "STYLE":
		result = "CSS"
	}
	return
}

func inAuraBundlesFolder() bool {
	var p = *fileName
	var maxLoop = 3
	for filepath.Base(p) != "aurabundles" && maxLoop != 0 {
		p = filepath.Dir(p)
		maxLoop -= 1
	}
	if filepath.Base(p) == "aurabundles" {
		return true
	} else {
		return false
	}

}
func readFile(filename string) (body string, err error) {
	data, err := ioutil.ReadFile(filename)
	if err != nil {
		return
	}
	body = string(data)
	return
}

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
	Short: "force pushAura -resourcepath=<filepath>",
	Long: `
	force pushAura -resourcepath <fullFilePath>

	force pushAura -f=<fullFilePath>

	`,
}

func init() {
	cmdPushAura.Run = runPushAura
	cmdPushAura.Flag.Var(&resourcepath, "f", "fully qualified file name for entity")
	//	cmdPushAura.Flag.StringVar(&resourcepath, "f", "", "fully qualified file name for entity")
	cmdPushAura.Flag.StringVar(&metadataType, "t", "", "Type of entity or bundle to create")
	cmdPushAura.Flag.StringVar(&metadataType, "type", "", "Type of entity or bundle to create")
}

var (
//resourcepath = cmdPushAura.Flag.String("filepath", "", "fully qualified file name for entity")
//	isBundle   = cmdPushAura.Flag.Bool("isBundle", false, "Creating a bundle or not")
//createType = cmdPushAura.Flag.String("auraType", "", "Type of entity or bundle to create")
)

func runPushAura(cmd *Command, args []string) {
	absPath, _ := filepath.Abs(resourcepath[0])

	if _, err := os.Stat(absPath); os.IsNotExist(err) {
		fmt.Println(err.Error())
		ErrorAndExit("File does not exist\n" + absPath)
	}

	// Verify that the file is in an aura bundles folder
	if !InAuraBundlesFolder(absPath) {
		ErrorAndExit("File is not in an aura bundle folder (aura)")
	}

	// See if this is a directory
	info, _ := os.Stat(absPath)
	if info.IsDir() {
		// If this is a path, then it is expected be a direct child of "metatdata/aura".
		// If so, then we are going to push all the definitions in the bundle one at a time.
		filepath.Walk(absPath, func(path string, inf os.FileInfo, err error) error {
			info, err = os.Stat(filepath.Join(absPath, inf.Name()))
			if err != nil {
				fmt.Println(err.Error())
			} else {
				if info.IsDir() || inf.Name() == ".manifest" {
					fmt.Println("\nSkip")
				} else {
					pushAuraComponent(filepath.Join(absPath, inf.Name()))
				}
			}
			return nil
		})
	} else {
		pushAuraComponent(absPath)
	}
	return
}

func pushAuraComponent(fname string) {
	force, _ := ActiveForce()
	// Check for manifest file
	if _, err := os.Stat(filepath.Join(filepath.Dir(fname), ".manifest")); os.IsNotExist(err) {
		// No manifest, but is in aurabundle folder, assume creating a new bundle with this file
		// as the first artifact.
		createNewAuraBundleAndDefinition(*force, fname)
	} else {
		// Got the manifest, let's update the artifact
		fmt.Println("Updating")
		updateAuraDefinition(*force, fname)
		return
	}
}

func isValidAuraExtension(fname string) bool {
	var ext = strings.Trim(strings.ToLower(filepath.Ext(fname)), " ")
	if ext == ".app" || ext == ".cmp" || ext == ".evt" {
		return true
	} else {
		ErrorAndExit("You need to create an application (.app) or component (.cmp) or and event (.evt) as the first item in your bundle.")
	}
	return false
}

func createNewAuraBundleAndDefinition(force Force, fname string) {
	// 	Creating a new bundle. We need
	// 		the name of the bundle (parent folder of file)
	//		the type of artifact (based on naming convention)
	// 		the contents of the file
	if isValidAuraExtension(fname) {
		// Need the parent folder name to name the bundle
		var bundleName = filepath.Base(filepath.Dir(fname))
		// Create the manifext
		var manifest BundleManifest
		manifest.Name = bundleName

		_, _ = getFormatByresourcepath(fname)
		targetDirectory = SetTargetDirectory(fname)

		// Create a bundle defintion
		bundle, err, emessages := force.CreateAuraBundle(bundleName)
		if err != nil {
			if emessages[0].ErrorCode == "DUPLICATE_VALUE" {
				// Should look up the bundle and get it's id then update it.
				FetchManifest(bundleName)
				updateAuraDefinition(force, fname)
				return
			}
			ErrorAndExit(err.Error())
		} else {
			manifest.Id = bundle.Id
			component, err, emessages := createBundleEntity(manifest, force, fname)
			if err != nil {
				ErrorAndExit(err.Error(), emessages[0].ErrorCode)
			}
			createManifest(manifest, component, fname)
		}
	}
}

func SetTargetDirectory(fname string) string {
	// Need to get the parent of metadata
	return strings.Split(fname, "/metadata/aura")[0]
}

func createBundleEntity(manifest BundleManifest, force Force, fname string) (component ForceCreateRecordResult, err error, emessages []ForceError) {
	// create the bundle entity
	format, deftype := getFormatByresourcepath(fname)
	mbody, _ := readFile(fname)
	component, err, emessages = force.CreateAuraComponent(map[string]string{"AuraDefinitionBundleId": manifest.Id, "DefType": deftype, "Format": format, "Source": mbody})
	return
}

func createManifest(manifest BundleManifest, component ForceCreateRecordResult, fname string) {
	cfile := ComponentFile{}
	cfile.FileName = fname
	cfile.ComponentId = component.Id

	manifest.Files = append(manifest.Files, cfile)
	bmBody, _ := json.Marshal(manifest)

	ioutil.WriteFile(filepath.Join(filepath.Dir(fname), ".manifest"), bmBody, 0644)
	return
}

func updateManifest(manifest BundleManifest, component ForceCreateRecordResult, fname string) {
	cfile := ComponentFile{}
	cfile.FileName = fname
	cfile.ComponentId = component.Id

	manifest.Files = append(manifest.Files, cfile)
	bmBody, _ := json.Marshal(manifest)

	ioutil.WriteFile(filepath.Join(filepath.Dir(fname), ".manifest"), bmBody, 0644)
	return
}

func GetManifest(fname string) (manifest BundleManifest, err error) {
	manifestname := filepath.Join(filepath.Dir(fname), ".manifest")

	if _, err = os.Stat(manifestname); os.IsNotExist(err) {
		return
	}

	mbody, _ := readFile(filepath.Join(filepath.Dir(fname), ".manifest"))
	json.Unmarshal([]byte(mbody), &manifest)
	return
}

func updateAuraDefinition(force Force, fname string) {

	//Get the manifest
	manifest, err := GetManifest(fname)

	for i := range manifest.Files {
		component := manifest.Files[i]
		if filepath.Base(component.FileName) == filepath.Base(fname) {
			//Here is where we make the call to send the update
			mbody, _ := readFile(fname)
			err := force.UpdateAuraComponent(map[string]string{"source": mbody}, component.ComponentId)
			if err != nil {
				ErrorAndExit(err.Error())
			}
			fmt.Printf("Aura definition updated: %s\n", filepath.Base(fname))
			return
		}
	}
	component, err, emessages := createBundleEntity(manifest, force, fname)
	if err != nil {
		ErrorAndExit(err.Error(), emessages[0].ErrorCode)
	}
	updateManifest(manifest, component, fname)
	fmt.Println("New component in the bundle")
}

func getFormatByresourcepath(resourcepath string) (format string, defType string) {
	var fname = strings.ToLower(resourcepath)
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
	} else if strings.Contains(fname, "renderer.js") {
		format = "JS"
		defType = "RENDERER"
	} else if strings.Contains(fname, "style.css") {
		format = "CSS"
		defType = "STYLE"
	} else {
		if filepath.Ext(fname) == ".app" {
			format = "XML"
			defType = "APPLICATION"
		} else if filepath.Ext(fname) == ".cmp" {
			format = "XML"
			defType = "COMPONENT"
		} else if filepath.Ext(fname) == ".evt" {
			format = "XML"
			defType = "EVENT"
		} else if filepath.Ext(fname) == ".css" {
			format = "CSS"
			defType = "STYLE"
		} else if filepath.Ext(fname) == ".auradoc" {
			format = "XML"
			defType = "DOCUMENTATION"
		} else {
			ErrorAndExit("Could not determine aura definition type.", fname)
		}
	}
	return
}

func getDefinitionFormat(deftype string) (result string) {
	switch strings.ToUpper(deftype) {
	case "APPLICATION", "COMPONENT", "EVENT", "DOCUMENTATION":
		result = "XML"
	case "CONTROLLER", "MODEL", "HELPER", "RENDERER":
		result = "JS"
	case "STYLE":
		result = "CSS"
	}
	return
}

func InAuraBundlesFolder(fname string) bool {
	info, _ := os.Stat(fname)
	if info.IsDir() {
		return strings.HasSuffix(filepath.Dir(fname), "metadata/aura")
	} else {
		return strings.HasSuffix(filepath.Dir(filepath.Dir(fname)), "metadata/aura")
	}
}

func readFile(resourcepath string) (body string, err error) {
	data, err := ioutil.ReadFile(resourcepath)
	if err != nil {
		return
	}
	body = string(data)
	return
}

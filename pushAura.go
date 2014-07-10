package main

import (
	"encoding/json"
	"fmt"
	"io/ioutil"
	"path/filepath"
)

var cmdPushAura = &Command{
	Run:   runPushAura,
	Usage: "pushAura",
	Short: "TBD",
	Long: `
	force pushAura fullFilePath entityId

	`,
}

func runPushAura(cmd *Command, args []string) {
	force, _ := ActiveForce()

	fileName := args[0]

	//Get the manifest
	mbody, _ := readFile(filepath.Join(filepath.Dir(fileName), "manifest.json"))

	var manifest BundleManifest
	json.Unmarshal([]byte(mbody), &manifest)

	for i := range manifest.Files {
		component := manifest.Files[i]
		if component.FileName == filepath.Base(fileName) {
			//Here is where we make the call to send the update
			mbody, _ = readFile(fileName)
			err := force.UpdateAuraComponent(map[string]string{"source": mbody}, component.ComponentId)
			if err != nil {
				ErrorAndExit(err.Error())
			}
			fmt.Printf("Aura definition updated: %s\n", filepath.Base(fileName))
			break
		}
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

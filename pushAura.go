package main

import (
	"encoding/json"
	"fmt"
	"io/ioutil"
	"path/filepath"
)

var cmdPushAura = &Command{
	Usage: "pushAura",
	Short: "force pushAura -e=<filepath>",
	Long: `
	force pushAura -e=<fullFilePath>

	`,
}

func init() {
	cmdPushAura.Run = runPushAura
}

var (
	entity = cmdPushAura.Flag.String("e", "", "fully qualified file name for entity")
)

func runPushAura(cmd *Command, args []string) {
	force, _ := ActiveForce()

	fileName := *entity

	//Get the manifest
	mbody, _ := readFile(filepath.Join(filepath.Dir(fileName), ".manifest"))

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

package main

import (
	"os"
	//"encoding/json"
	"fmt"
	"strings"
	//"io/ioutil"
	"path/filepath"
)

var cmdAura = &Command{
	Usage: "aura",
	Short: "force aura push -fileName=<filepath>",
	Long: `
	force aura push -f=<fullFilePath>

	force aura create -t=<entity type> <entityName>

	force aura delete -f=<fullFilePath>

	force aura list

	`,
}

func init() {
	cmdAura.Run = runAura
	cmdAura.Flag.StringVar(fileName, "f", "", "fully qualified file name for entity")
	cmdAura.Flag.StringVar(auraentitytype, "entitytype", "", "fully qualified file name for entity")
	cmdAura.Flag.StringVar(auraentityname, "entityname", "", "fully qualified file name for entity")
}

var (
	auraentitytype = cmdAura.Flag.String("t", "", "aura entity type")
	auraentityname = cmdAura.Flag.String("n", "", "aura entity name")
)

func runAura(cmd *Command, args []string) {
	if err := cmd.Flag.Parse(args[1:]); err != nil {
		os.Exit(2)
	}
	force, _ := ActiveForce()

	subcommand := args[0]

	switch strings.ToLower(subcommand) {
	case "create":
		if *auraentitytype == "" || *auraentityname == "" {
			fmt.Println("Must specify entity type and name")
			os.Exit(2)
		}

	case "delete":
		fmt.Println("Delete", *fileName)
		runDeleteAura()
	case "list":
		bundles, err := force.GetAuraBundlesList()
		if err != nil {
			ErrorAndExit("Ooops")
		}
		for _, bundle := range bundles.Records {
			fmt.Println(bundle["DeveloperName"])
		}
	case "push":
		runPushAura(cmd, args)
	}
}

func runDeleteAura() {
	if InAuraBundlesFolder(*fileName) {
		info, _ := os.Stat(*fileName)
		manifest := GetManifest(*fileName)
		isBundle := false
		if info.IsDir() {
			manifest = GetManifest(filepath.Join(*fileName, ".manifest"))
			isBundle = true
		}

		for key := range manifest.Files {
			mfile := manifest.Files[key].FileName
			cfile := *fileName
			if !filepath.IsAbs(mfile) {
				cfile = filepath.Base(cfile)
			}
			if isBundle {
				if !filepath.IsAbs(mfile) {
					cfile = filepath.Join(*fileName, mfile)
				} else {
					cfile = mfile
					deleteAuraDefinition(manifest, key)
				}
			} else {
				if mfile == cfile {
					fmt.Println("Found the manifest entry: ", manifest.Files[key].ComponentId)
					deleteAuraDefinition(manifest, key)
					return
				}
			}
		}
		if isBundle {
			// Need to remove the bundle using the id in the manifest
			deleteAuraDefinitionBundle(manifest)
		}
	}
}
func deleteAuraDefinitionBundle(manifest BundleManifest) {
	force, err := ActiveForce()
	err = force.DeleteToolingRecord("AuraDefinitionBundle", manifest.Id)
	if err != nil {
		ErrorAndExit(err.Error())
	}
	os.Remove(filepath.Join(*fileName, ".manifest"))
	os.Remove(*fileName)
}

func deleteAuraDefinition(manifest BundleManifest, key int) {
	force, err := ActiveForce()
	err = force.DeleteToolingRecord("AuraDefinition", manifest.Files[key].ComponentId)
	if err != nil {
		ErrorAndExit(err.Error())
	}
	manifest.Files = append(manifest.Files[:key], manifest.Files[key+1:]...)
	os.Remove(*fileName)
}

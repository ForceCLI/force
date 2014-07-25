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
	The aura command needs context to work. If you execute "aura get"
	it will create a folder structure that provides the context for 
	aura components on disk.

	The aura components will be created in "metadata/aurabundles/<componentname>"
	relative to the current working directory and a .manifest file will be
	created that associates components and their artifacts with their ids in
	the database. 

	To create a new component (application, evt or component), create a new
	folder under "aurabundles". Then create a new file in your new folder. You 
	must follow a naming convention for your files to enable proper definition 
	of the component type.

	Naming convention <compnentName><artifact type>.<file type extension>
	Examples: 	metadata
					aurabundles
						MyApp 
							MyAppApplication.app
							MyAppStyle.css
						MyList 
							MyComponent.cmp
							MyComponentHelper.js
							MyComponentStyle.css

	force aura push -f <fullFilePath> -b <bundle name>

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
	absPath, _ := filepath.Abs(*fileName)
	*fileName = absPath

	if InAuraBundlesFolder(*fileName) {
		fmt.Println("Yup, in aura folder")
		info, err := os.Stat(*fileName)
		if err != nil {
			ErrorAndExit(err.Error())
		}
		manifest, err := GetManifest(*fileName)
		isBundle := false
		if info.IsDir() {
			manifest, err = GetManifest(filepath.Join(*fileName, ".manifest"))
			if err != nil {
				// Try to look up the bundle by name
				force, _ := ActiveForce()
				b, err := force.GetAuraBundleByName(filepath.Base(*fileName))
				if err != nil {
					fmt.Println(err.Error())
				} else {
					if len(b.Records) == 0 {
						ErrorAndExit(fmt.Sprintf("No bundle definition named %q", filepath.Base(*fileName)))
					} else {
						bid := b.Records[0]["Id"].(string)
						err = force.DeleteToolingRecord("AuraDefinitionBundle", bid)
						if err != nil {
							ErrorAndExit(err.Error())
						}
						// Now walk the bundle and remove all the atrifacts
						filepath.Walk(*fileName, func(path string, inf os.FileInfo, err error) error {
							os.Remove(path)
							return nil
						})
						os.Remove(*fileName)
						fmt.Println("Bundle ", filepath.Base(*fileName), " deleted.")
						return;					
					}
				}	
			}
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

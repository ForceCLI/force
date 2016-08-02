package main

import (
	"encoding/json"
	"fmt"
	"io/ioutil"
	"os"
	"path/filepath"
	"strings"

	"github.com/heroku/force/salesforce"
	"github.com/heroku/force/util"
)

// Brief comment to fire commit

var cmdAura = &Command{
	Usage: "aura",
	Short: "force aura push -resourcepath=<filepath>",
	Long: `
	The aura command needs context to work. If you execute "aura get"
	it will create a folder structure that provides the context for
	aura components on disk.

	The aura components will be created in "metadata/aurabundles/<componentname>"
	relative to the current working directory and a .manifest file will be
	created that associates components and their artifacts with their ids in
	the database.

	To create a new component (application, evt or component), create a new
	folder under "aura". Then create a new file in your new folder. You
	must follow a naming convention for your files to enable proper definition
	of the component type.

	Naming convention <compnentName><artifact type>.<file type extension>
	Examples: 	metadata
					aura
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
	cmdAura.Flag.Var(&resourcepath, "p", "fully qualified file name for entity")
	cmdAura.Flag.Var(&resourcepath, "f", "fully qualified file name for entity")
	cmdAura.Flag.StringVar(&metadataType, "entitytype", "", "fully qualified file name for entity")
	cmdAura.Flag.StringVar(&auraentityname, "entityname", "", "fully qualified file name for entity")
	cmdAura.Flag.StringVar(&metadataType, "t", "", "fully qualified file name for entity")
	cmdAura.Flag.StringVar(&auraentityname, "n", "", "fully qualified file name for entity")
}

var (
	auraentityname string
)

func runAura(cmd *Command, args []string) {
	if err := cmd.Flag.Parse(args[0:]); err != nil {
		os.Exit(2)
	}

	force, _ := ActiveForce()

	subcommand := args[0]
	// Sublime hack - the way sublime passes parameters seems to
	// break the flag parsing by sending a single element array
	// for the args. ARGH!!!
	if strings.HasPrefix(subcommand, "delete ") || strings.HasPrefix(subcommand, "push ") {
		what := strings.Split(subcommand, " ")
		if err := cmd.Flag.Parse(what[1:]); err != nil {
			util.ErrorAndExit(err.Error())
		}
		subcommand = what[0]
	} else {
		if err := cmd.Flag.Parse(args[1:]); err != nil {
			util.ErrorAndExit(err.Error())
		}
	}

	switch strings.ToLower(subcommand) {
	case "create":
		/*if *auraentitytype == "" || *auraentityname == "" {
			fmt.Println("Must specify entity type and name")
			os.Exit(2)
		}*/
		util.ErrorAndExit("force aura create not yet implemented")

	case "delete":
		runDeleteAura()
	case "list":
		bundles, err := force.GetAuraBundlesList()
		if err != nil {
			util.ErrorAndExit(err.Error())
		}
		for _, bundle := range bundles.Records {
			fmt.Println(bundle["DeveloperName"])
		}
	case "push":
		//		absPath, _ := filepath.Abs(resourcepath[0])
		runPushAura(cmd, resourcepath)
	}
}

func runDeleteAura() {
	absPath, _ := filepath.Abs(resourcepath[0])
	//resourcepath = absPath

	if InAuraBundlesFolder(absPath) {
		info, err := os.Stat(absPath)
		if err != nil {
			util.ErrorAndExit(err.Error())
		}
		manifest, err := GetManifest(absPath)
		isBundle := false
		if info.IsDir() {
			force, _ := ActiveForce()
			manifest, err = GetManifest(filepath.Join(absPath, ".manifest"))
			bid := ""
			if err != nil { // Could not find a manifest, use bundle name
				// Try to look up the bundle by name
				b, err := force.GetAuraBundleByName(filepath.Base(absPath))
				if err != nil {
					util.ErrorAndExit(err.Error())
				} else {
					if len(b.Records) == 0 {
						util.ErrorAndExit(fmt.Sprintf("No bundle definition named %q", filepath.Base(absPath)))
					} else {
						bid = b.Records[0]["Id"].(string)
					}
				}
			} else {
				bid = manifest.Id
			}

			err = force.DeleteToolingRecord("AuraDefinitionBundle", bid)
			if err != nil {
				util.ErrorAndExit(err.Error())
			}
			// Now walk the bundle and remove all the atrifacts
			filepath.Walk(absPath, func(path string, inf os.FileInfo, err error) error {
				os.Remove(path)
				return nil
			})
			os.Remove(absPath)
			fmt.Println("Bundle ", filepath.Base(absPath), " deleted.")
			return
		}

		for key := range manifest.Files {
			mfile := manifest.Files[key].FileName
			cfile := absPath
			if !filepath.IsAbs(mfile) {
				cfile = filepath.Base(cfile)
			}
			if isBundle {
				if !filepath.IsAbs(mfile) {
					cfile = filepath.Join(absPath, mfile)
				} else {
					cfile = mfile
					deleteAuraDefinition(manifest, key)
				}
			} else {
				if mfile == cfile {
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
func deleteAuraDefinitionBundle(manifest salesforce.BundleManifest) {
	force, err := ActiveForce()
	err = force.DeleteToolingRecord("AuraDefinitionBundle", manifest.Id)
	if err != nil {
		util.ErrorAndExit(err.Error())
	}
	os.Remove(filepath.Join(resourcepath[0], ".manifest"))
	os.Remove(resourcepath[0])
}

func deleteAuraDefinition(manifest salesforce.BundleManifest, key int) {
	force, err := ActiveForce()
	err = force.DeleteToolingRecord("AuraDefinition", manifest.Files[key].ComponentId)
	if err != nil {
		util.ErrorAndExit(err.Error())
	}
	fname := manifest.Files[key].FileName
	os.Remove(fname)
	manifest.Files = append(manifest.Files[:key], manifest.Files[key+1:]...)
	bmBody, _ := json.Marshal(manifest)
	ioutil.WriteFile(filepath.Join(filepath.Dir(fname), ".manifest"), bmBody, 0644)
}

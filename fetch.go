package main

import (
	"archive/zip"
	"encoding/json"
	"encoding/xml"
	"fmt"
	"io"
	"io/ioutil"
	"log"
	"os"
	"path/filepath"
	"strings"
)

var cmdFetch = &Command{
	Run:   runFetch,
	Usage: "fetch <type> [<artifact name>] [options]",
	Short: "Export specified artifact(s) to a local directory",
	Long: `
Export specified artifact(s) to a local directory. Use "package" type to retrieve an unmanaged package.

Examples

  force fetch CustomObject Book__c Author__c

  force fetch CustomObject

  force fetch Aura [<entity name>]

  force fetch package MyPackagedApp

  options
      -u, --unpack
      	  will expand static resources if type is StaticResource

      	  example: force fetch StaticResource MyResource --unpack

`,
}

func runFetchAura(cmd *Command, entityname string) {
	force, _ := ActiveForce()

	var bundles AuraDefinitionBundleResult
	var definitions AuraDefinitionBundleResult
	var err error 

	if entityname == "" {
		bundles, definitions, err = force.GetAuraBundles()
		if err != nil {
			ErrorAndExit(err.Error())
		}
	} else {
		bundles, definitions, err = force.GetAuraBundle(entityname)
		if err != nil {
			ErrorAndExit(err.Error())
		}
	}


	var bundleMap = make(map[string]string)
	var bundleRecords = bundles.Records
	for _, bundle := range bundleRecords {
		id := fmt.Sprintf("%s", bundle["Id"])
		bundleMap[id] = fmt.Sprintf("%s", bundle["DeveloperName"])
	}

	var defRecords = definitions.Records
	wd, _ := os.Getwd()
	root := filepath.Join(wd, "metadata", "aurabundles")

	if err := os.MkdirAll(root, 0755); err != nil {
		ErrorAndExit(err.Error())
	}

	for key, value := range bundleMap {
		if err := os.MkdirAll(filepath.Join(root, value), 0755); err != nil {
			ErrorAndExit(err.Error())
		}

		var bundleManifest = BundleManifest{}
		bundleManifest.Name = value
		bundleManifest.Files = []ComponentFile{}

		for _, def := range defRecords {
			var did = fmt.Sprintf("%s", def["AuraDefinitionBundleId"])
			if did == key {
				var entity = fmt.Sprintf("%s%s", value, strings.Title(strings.ToLower(fmt.Sprintf("%s", def["DefType"]))))
				switch fmt.Sprintf("%s", def["DefType"]) {
				case "COMPONENT":
					entity += ".cmp"
				case "APPLICATION":
					entity += ".app"
				case "EVENT":
					entity += ".evt"
				case "STYLE":
					entity += ".css"
				default:
					entity += ".js"
				}
				var componentFile = ComponentFile{entity, fmt.Sprintf("%s", def["Id"])}
				bundleManifest.Files = append(bundleManifest.Files, componentFile)
				ioutil.WriteFile(filepath.Join(root, value, entity), []byte(fmt.Sprintf("%s", def["Source"])), 0644)
			}
		}
		bmBody, _ := json.Marshal(bundleManifest)
		ioutil.WriteFile(filepath.Join(root, value, ".manifest"), bmBody, 0644)
	}
	return
}

func runFetch(cmd *Command, args []string) {
	wd, _ := os.Getwd()
	root := filepath.Join(wd, "metadata")
	if len(args) < 1 {
		ErrorAndExit("must specify object type and/or object name")
	}

	force, _ := ActiveForce()
	var files ForceMetadataFiles
	var err error
	var expandResources bool = false

	artifactType := args[0]
	if strings.ToLower(artifactType) == "aura" {
		if len(args) == 2 {
			runFetchAura(cmd, args[1])
		} else {
			runFetchAura(cmd, "")
		}
	} else if artifactType == "package" {
		files, err = force.Metadata.RetrievePackage(args[1])
		if err != nil {
			ErrorAndExit(err.Error())
		}
		for artifactNames := range args[1:] {
			if args[1:][artifactNames] == "--unpack" || args[1:][artifactNames] == "-u" {
				expandResources = true
			}
		}
	} else {
		query := ForceMetadataQuery{}
		if len(args) >= 2 {
			newargs := args[1:]
			for artifactNames := range newargs {
				if newargs[artifactNames] == "--unpack" || newargs[artifactNames] == "-u" {
					expandResources = true
				} else {
					mq := ForceMetadataQueryElement{artifactType, newargs[artifactNames]}
					query = append(query, mq)
				}
			}
		} else {
			mq := ForceMetadataQueryElement{artifactType, "*"}
			query = append(query, mq)
		}
		files, err = force.Metadata.Retrieve(query)
		if err != nil {
			ErrorAndExit(err.Error())
		}
	}

	var resourcesMap map[string]string
	resourcesMap = make(map[string]string)

	for name, data := range files {
		file := filepath.Join(root, name)
		dir := filepath.Dir(file)

		if err := os.MkdirAll(dir, 0755); err != nil {
			ErrorAndExit(err.Error())
		}
		if err := ioutil.WriteFile(filepath.Join(root, name), data, 0644); err != nil {
			ErrorAndExit(err.Error())
		}
		var isResource = false
		if artifactType == "StaticResource" {
			isResource = true
		} else if strings.HasSuffix(file, ".resource-meta.xml") {
			isResource = true
		}
		//Handle expanding static resources into a "bundle" folder
		if isResource && expandResources && name != "package.xml" {
			pathParts := strings.Split(name, "/")
			resourceName := pathParts[cap(pathParts)-1]

			resourceExt := strings.Split(resourceName, ".")[1]
			resourceName = strings.Split(resourceName, ".")[0]
			if resourceExt == "resource-meta" {
				//Check the xml to determine the mime type of the resource
				// We are looking for application/zip
				var meta struct {
					CacheControl string `xml:"cacheControl"`
					ContentType  string `xml:"contentType"`
				}
				if err = xml.Unmarshal([]byte(data), &meta); err != nil {
					//return
				}
				if meta.ContentType == "application/zip" {
					// this is the meat for a zip file, so add the map
					resourcesMap[resourceName] = filepath.Join(filepath.Dir(file), resourceName+".resource")
				}
			}
		}
	}

	// Now we need to see if we have any zips to expand
	if expandResources && len(resourcesMap) > 0 {
		for key, value := range resourcesMap {
			//resourcefile := filepath.Join(root, "staticresources", value)
			resourcefile := value
			dest := filepath.Join(filepath.Dir(value), key)
			if err := os.MkdirAll(dest, 0755); err != nil {
				ErrorAndExit(err.Error())
			}
			//f, err := os.Open(resourcefile);
			r, err := zip.OpenReader(resourcefile)
			if err != nil {
				log.Fatal(err)
			}
			defer r.Close()

			for _, f := range r.File {
				rc, err := f.Open()
				if err != nil {
					fmt.Println(err)
				}
				defer rc.Close()

				path := filepath.Join(dest, f.Name)
				if !strings.HasPrefix(f.Name, "__") {
					if f.FileInfo().IsDir() {
						os.MkdirAll(path, f.Mode())
					} else {
						zf, err := os.OpenFile(
							path, os.O_WRONLY|os.O_CREATE|os.O_TRUNC, f.Mode())
						if err != nil {
							fmt.Println(err)
						}

						_, err = io.Copy(zf, rc)
						if err != nil {
							fmt.Println(err)
							zf.Close()
						}
						zf.Close()
					}
				}
			}
		}
	}

	fmt.Printf("Exported to %s\n", root)
}

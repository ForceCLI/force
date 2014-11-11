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
	Usage: "fetch -t ApexClass",
	Short: "Export specified artifact(s) to a local directory",
	Long: `
  -t, -type       # type of metadata to retrieve
  -n, -name       # name of specific metadata to retrieve (must be used with -type)
  -d, -directory  # override the default target directory
  -u, -unpack     # unpack any zipped static resources (ignored if type is not StaticResource)

Export specified artifact(s) to a local directory. Use "package" type to retrieve an unmanaged package.

Examples

  force fetch -t=CustomObject n=Book__c n=Author__c
  force fetch -t Aura -n MyComponent -d /Users/me/Documents/Project/home

`,
}

type metaName []string

func (i *metaName) String() string {
	return fmt.Sprint(*i)
}

func (i *metaName) Set(value string) error {
	// That would permit usages such as
	//	-deltaT 10s -deltaT 15s
	for _, name := range strings.Split(value, ",") {
		*i = append(*i, name)
	}
	return nil
}

var (
	metadataType    string
	targetDirectory string
	unpack          bool
	metadataName    metaName
	makefile        bool
)

func init() {
	cmdFetch.Flag.Var(&metadataName, "name", "names of metadata")
	cmdFetch.Flag.Var(&metadataName, "n", "names of metadata")
	cmdFetch.Flag.StringVar(&metadataType, "t", "", "Type of metadata to fetch")
	cmdFetch.Flag.StringVar(&metadataType, "type", "", "Type of metadata to fetch")
	cmdFetch.Flag.StringVar(&targetDirectory, "d", "", "Use to specify the root directory of your project")
	cmdFetch.Flag.StringVar(&targetDirectory, "directory", "", "Use to specify the root directory of your project")
	cmdFetch.Flag.BoolVar(&unpack, "u", false, "Unpage any static resources")
	cmdFetch.Flag.BoolVar(&unpack, "unpack", false, "Unpage any static resources")
	cmdFetch.Run = runFetch
	makefile = true
}

func runFetchAura2(cmd *Command, entityname string) {
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
	_, err = persistBundles(bundles, definitions)
	return
}

func FetchManifest(entityname string) (manifest BundleManifest) {
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
	makefile = false
	_, err = persistBundles(bundles, definitions)
	return

}

func persistBundles(bundles AuraDefinitionBundleResult, definitions AuraDefinitionBundleResult) (bundleManifest BundleManifest, err error) {
	var bundleMap = make(map[string]string)
	var bundleRecords = bundles.Records
	for _, bundle := range bundleRecords {
		id := fmt.Sprintf("%s", bundle["Id"])
		bundleMap[id] = fmt.Sprintf("%s", bundle["DeveloperName"])
	}

	var defRecords = definitions.Records
	root, err := GetSourceDir(targetDirectory)
	root = filepath.Join(targetDirectory, root, "aura")
	if err := os.MkdirAll(root, 0755); err != nil {
		ErrorAndExit(err.Error())
	}

	for key, value := range bundleMap {
		if err := os.MkdirAll(filepath.Join(root, value), 0755); err != nil {
			ErrorAndExit(err.Error())
		}

		bundleManifest = BundleManifest{}
		bundleManifest.Name = value
		bundleManifest.Files = []ComponentFile{}
		bundleManifest.Id = key

		for _, def := range defRecords {
			var did = fmt.Sprintf("%s", def["AuraDefinitionBundleId"])
			if did == key {
				var naming = strings.Title(strings.ToLower(fmt.Sprintf("%s", def["DefType"])))
				var entity = fmt.Sprintf("%s", value) //, strings.Title(strings.ToLower(fmt.Sprintf("%s", def["DefType"]))))
				switch fmt.Sprintf("%s", def["DefType"]) {
				case "COMPONENT":
					entity += ".cmp"
				case "APPLICATION":
					entity += ".app"
				case "EVENT":
					entity += ".evt"
				case "STYLE":
					entity += fmt.Sprintf("%s.css", naming)
				default:
					entity += fmt.Sprintf("%s.js", naming)
				}
				var componentFile = ComponentFile{filepath.Join(root, value, entity), fmt.Sprintf("%s", def["Id"])}
				bundleManifest.Files = append(bundleManifest.Files, componentFile)
				if makefile {
					ioutil.WriteFile(filepath.Join(root, value, entity), []byte(fmt.Sprintf("%s", def["Source"])), 0644)
				}
			}
		}
		bmBody, _ := json.Marshal(bundleManifest)
		ioutil.WriteFile(filepath.Join(root, value, ".manifest"), bmBody, 0644)
	}
	return
}

func runFetch(cmd *Command, args []string) {
	if metadataType == "" {
		ErrorAndExit("must specify object type and/or object name")
	}

	force, _ := ActiveForce()
	var files ForceMetadataFiles
	var err error
	var expandResources bool = unpack

	if strings.ToLower(metadataType) == "aura" {
		if len(metadataName) > 0 {
			for names := range metadataName {
				runFetchAura2(cmd, metadataName[names])
			}
		} else {
			runFetchAura2(cmd, "")
		}
	} else if metadataType == "package" {
		if len(metadataName) > 0 {
			for names := range metadataName {
				files, err = force.Metadata.RetrievePackage(metadataName[names])
				if err != nil {
					ErrorAndExit(err.Error())
				}
			}
		}
	} else {
		query := ForceMetadataQuery{}
		if len(metadataName) > 0 {
			for names := range metadataName {
				mq := ForceMetadataQueryElement{metadataType, metadataName[names]}
				query = append(query, mq)
			}
		} else {
			mq := ForceMetadataQueryElement{metadataType, "*"}
			query = append(query, mq)
		}
		files, err = force.Metadata.Retrieve(query)
		if err != nil {
			ErrorAndExit(err.Error())
		}
	}

	var resourcesMap map[string]string
	resourcesMap = make(map[string]string)

	root, err := GetSourceDir(targetDirectory)
	existingPackage, _ := pathExists(filepath.Join(root, "package.xml"))

	if len(files) == 1 {
		ErrorAndExit("Could not find any objects for " + metadataType + ". (Is the metadata type correct?)")
	}
	for name, data := range files {
		if !existingPackage || name != "package.xml" {
			file := filepath.Join(root, name)
			dir := filepath.Dir(file)

			if err := os.MkdirAll(dir, 0755); err != nil {
				ErrorAndExit(err.Error())
			}
			if err := ioutil.WriteFile(filepath.Join(root, name), data, 0644); err != nil {
				ErrorAndExit(err.Error())
			}
			var isResource = false
			if metadataType == "StaticResource" {
				isResource = true
			} else if strings.HasSuffix(file, ".resource-meta.xml") {
				isResource = true
			}
			//Handle expanding static resources into a "bundle" folder
			if isResource && expandResources {
				pathParts := strings.Split(name, string(os.PathSeparator))
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
	}

	// Now we need to see if we have any zips to expand
	if expandResources && len(resourcesMap) > 0 {
		for _, value := range resourcesMap {
			//resourcefile := filepath.Join(root, "staticresources", value)
			resourcefile := value
			dest := strings.Split(value, ".")[0]
			if err := os.MkdirAll(dest, 0755); err != nil {
				ErrorAndExit(err.Error())
			}
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

				path := dest
				if !f.FileInfo().IsDir() {
					path = filepath.Join(path, filepath.Base(f.Name))
				}
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

func pathExists(path string) (bool, error) {
	_, err := os.Stat(path)
	if err == nil {
		return true, nil
	}
	if os.IsNotExist(err) {
		return false, nil
	}
	return false, err
}

package command

import (
	"archive/zip"
	"encoding/xml"
	"fmt"
	"io"
	"io/ioutil"
	"log"
	"os"
	"path/filepath"
	"strings"

	"github.com/ForceCLI/force/config"
	. "github.com/ForceCLI/force/error"
	. "github.com/ForceCLI/force/lib"
	"github.com/spf13/cobra"
)

func init() {
	fetchCmd.Flags().StringSliceVarP(&metadataName, "name", "n", []string{}, "names of metadata")
	fetchCmd.Flags().StringSliceVarP(&metadataTypes, "type", "t", []string{}, "Type of metadata to fetch")
	fetchCmd.Flags().StringVarP(&targetDirectory, "directory", "d", "", "Use to specify the root directory of your project")
	fetchCmd.Flags().BoolVarP(&unpack, "unpack", "u", false, "Unpack any static resources")
	fetchCmd.Flags().BoolVarP(&preserveZip, "preserve", "p", false, "keep zip file on disk")
	fetchCmd.Flags().StringP("xml", "x", "", "Package.xml file to use for fetch.")
	fetchCmd.MarkFlagsMutuallyExclusive("xml", "type")
	RootCmd.AddCommand(fetchCmd)
}

var fetchCmd = &cobra.Command{
	Use:   "fetch -t ApexClass",
	Short: "Export specified artifact(s) to a local directory",
	Long: `
Export specified artifact(s) to a local directory. Use "package" type to retrieve an unmanaged package.
`,
	Example: `
  force fetch -t=CustomObject -n=Book__c -n=Author__c
  force fetch -t Aura -n MyComponent -d /Users/me/Documents/Project/home
  force fetch -t AuraDefinitionBundle -t ApexClass
  force fetch -x myproj/metadata/package.xml
`,
	Run: func(cmd *cobra.Command, args []string) {
		if len(args) > 0 {
			buildPackageAndFetch(args)
			return
		}
		if packageXml, _ := cmd.Flags().GetString("xml"); packageXml != "" {
			runFetchForPackageXml(packageXml)
			return
		}
		runFetch()
	},
}

type metaName = []string

var (
	metadataTypes   metaName
	targetDirectory string
	unpack          bool
	metadataName    metaName
	preserveZip     bool
)

func getWildcardQuery(force *Force, metadataTypes metaName) (query ForceMetadataQuery, err error) {
	var folders FolderedMetadata
	// For foldered metadata types, which don't support wildcards, get the list
	// of folders and all metadata within each folder.
	for _, metadataType := range metadataTypes {
		switch metadataType {
		case "EmailTemplate":
			fallthrough
		case "Dashboard":
			fallthrough
		case "Report":
			fallthrough
		case "Document":
			if folders == nil {
				folders, err = force.GetAllFolders()
			}
			if err != nil {
				ErrorAndExit(err.Error())
			}
			folderType := FolderType(metadataType)
			members, err := force.GetMetadataInFolders(folderType, folders[folderType])
			if err != nil {
				err = fmt.Errorf("Could not get metadata in folders: %s", err.Error())
				ErrorAndExit(err.Error())
			}
			query = append(query, ForceMetadataQueryElement{Name: []string{string(folderType)}, Members: members})
		default:
			mq := ForceMetadataQueryElement{
				Name:    []string{metadataType},
				Members: []string{"*"},
			}
			query = append(query, mq)
		}
	}
	return
}

func runFetchForPackageXml(packageXml string) {
	files, problems, err := force.Metadata.RetrieveByPackageXml(packageXml)
	if err != nil {
		ErrorAndExit(err.Error())
	}
	for _, problem := range problems {
		fmt.Fprintln(os.Stderr, problem)
	}
	unpackFiles(files)
}

// Fetch by Type
func runFetch() {
	if len(metadataTypes) == 0 {
		ErrorAndExit("must specify object type and/or object name or package xml path")
	}
	if len(metadataTypes) > 1 && len(metadataName) > 1 {
		ErrorAndExit("You cannot specify entity names if you specify more than one metadata type.")
	}

	var files ForceMetadataFiles
	var problems []string
	var err error

	if len(metadataTypes) == 1 && strings.ToLower(metadataTypes[0]) == "package" {
		if len(metadataName) > 0 {
			for names := range metadataName {
				files, problems, err = force.Metadata.RetrievePackage(metadataName[names])
				if err != nil {
					ErrorAndExit(err.Error())
				}
				if preserveZip {
					os.Rename("inbound.zip", fmt.Sprintf("%s.zip", metadataName[names]))
				}
			}
		}
	} else {
		query := ForceMetadataQuery{}
		if len(metadataName) > 0 {
			mq := ForceMetadataQueryElement{
				Name:    metadataTypes,
				Members: metadataName,
			}
			query = append(query, mq)
		} else {
			query, err = getWildcardQuery(force, metadataTypes)
			if err != nil {
				ErrorAndExit(err.Error())
			}
		}
		files, problems, err = force.Metadata.Retrieve(query)
		if err != nil {
			ErrorAndExit(err.Error())
		}
	}
	for _, problem := range problems {
		fmt.Fprintln(os.Stderr, problem)
	}
	unpackFiles(files)
}

func unpackFiles(files ForceMetadataFiles) {
	var err error
	var expandResources bool = unpack

	resourcesMap := make(map[string]string)

	root := targetDirectory
	if root == "" {
		root, err = config.GetSourceDir()
	}
	if err != nil {
		fmt.Printf("Error obtaining root directory\n")
		ErrorAndExit(err.Error())
	}
	existingPackage, _ := pathExists(filepath.Join(root, "package.xml"))

	if len(files) == 1 {
		ErrorAndExit("Could not find any objects for " + strings.Join(metadataTypes, ", ") + ". (Is the metadata type correct?)")
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
			isResource := strings.HasSuffix(file, ".resource-meta.xml")
			//Handle expanding static resources into a "bundle" folder
			if isResource && expandResources {
				if string(os.PathSeparator) != "/" {
					name = strings.Replace(name, "/", string(os.PathSeparator), -1)
				}
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
		unpackResources(resourcesMap)
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

func buildPackageAndFetch(paths []string) {
	// for each argument
	// add name and type to package
	pb := NewFetchBuilder()
	sourceDir, err := config.GetSourceDir()
	if err != nil {
		ErrorAndExit("Could not find source dir")
	}
	pb.Root = sourceDir
	for _, f := range paths {
		if info, err := os.Stat(f); err != nil {
			Log.Info("Cannot fetch", f, err.Error())
		} else if info.IsDir() {
			err := pb.AddDirectory(f)
			if err != nil {
				Log.Info("Could not add", f, err.Error())
			}
			Log.Info("Added", f)
		} else {
			err := pb.AddFile(f)
			if err != nil {
				Log.Info("Could not add", f, err.Error())
			} else {
				Log.Info("Fetching", f)
			}
		}
	}
	packageXml := pb.PackageXml()
	if len(pb.Metadata) == 0 {
		ErrorAndExit("Nothing to fetch")
	}

	files, problems, err := force.Metadata.RetrieveByPackageXmlContents(packageXml)
	if err != nil {
		ErrorAndExit(err.Error())
	}
	for _, problem := range problems {
		fmt.Fprintln(os.Stderr, problem)
	}
	unpackFiles(files)
}

func unpackResources(resourceMap map[string]string) {
	for _, value := range resourceMap {
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
			if !strings.HasPrefix(f.Name, "__") {
				if f.FileInfo().IsDir() {
					path = filepath.Join(dest, f.Name)
					os.MkdirAll(path, f.Mode())
				} else {
					os.MkdirAll(filepath.Join(dest, filepath.Dir(f.Name)), 0777)
					zf, err := os.OpenFile(
						filepath.Join(dest, f.Name), os.O_WRONLY|os.O_CREATE|os.O_TRUNC, f.Mode())
					if err != nil {
						fmt.Println("OpenFile: ", err)
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

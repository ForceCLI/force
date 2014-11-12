package main

import (
	"encoding/xml"
	"io/ioutil"
	"os"
	"path/filepath"
	"strings"
)

// Structs for XML building
type Package struct {
	Xmlns   string     `xml:"xmlns,attr"`
	Types   []MetaType `xml:"types"`
	Version string     `xml:"version"`
}

type MetaType struct {
	Members []string `xml:"members"`
	Name    string   `xml:"name"`
}

func createPackage() Package {
	return Package{
		Version: strings.TrimPrefix(apiVersion, "v"),
		Xmlns:   "http://soap.sforce.com/2006/04/metadata",
	}
}

type metapath struct {
	path string
	name string
}

var metapaths = []metapath{
	metapath{"applications", "CustomApplication"},
	metapath{"assignmentRules", "AssignmentRules"},
	metapath{"aura", "AuraDefinitionBundle"},
	metapath{"autoResponseRules", "AutoResponseRules"},
	metapath{"classes", "ApexClass"},
	metapath{"communities", "Community"},
	metapath{"components", "ApexComponent"},
	metapath{"connectedApps", "ConnectedApp"},
	metapath{"flexipages", "FlexiPage"},
	metapath{"homePageLayouts", "HomePageLayout"},
	metapath{"labels", "CustomLabels"},
	metapath{"layouts", "Layout"},
	metapath{"objects", "CustomObject"},
	metapath{"objectTranslations", "CustomObjectTranslation"},
	metapath{"pages", "ApexPage"},
	metapath{"permissionsets", "PermissionSet"},
	metapath{"profiles", "Profile"},
	metapath{"quickActions", "QuickAction"},
	metapath{"remoteSiteSettings", "RemoteSiteSetting"},
	metapath{"roles", "Role"},
	metapath{"staticresources", "StaticResource"},
	metapath{"tabs", "CustomTab"},
	metapath{"triggers", "ApexTrigger"},
}

type PackageBuilder struct {
	IsPush   bool
	Metadata map[string]MetaType
	Files    ForceMetadataFiles
}

func NewPushBuilder() PackageBuilder {
	pb := PackageBuilder{IsPush: true}
	pb.Metadata = make(map[string]MetaType)
	pb.Files = make(ForceMetadataFiles)

	return pb
}

func NewFetchBuilder() PackageBuilder {
	pb := PackageBuilder{IsPush: false}
	pb.Metadata = make(map[string]MetaType)
	pb.Files = make(ForceMetadataFiles)

	return pb
}

// Build and return package.xml
func (pb PackageBuilder) PackageXml() []byte {
	p := createPackage()

	for _, metaType := range pb.Metadata {
		p.Types = append(p.Types, metaType)
	}

	byteXml, _ := xml.MarshalIndent(p, "", "    ")
	byteXml = append([]byte(xml.Header), byteXml...)
	//if err := ioutil.WriteFile("mypackage.xml", byteXml, 0644); err != nil {
	//ErrorAndExit(err.Error())
	//}
	return byteXml
}

// Returns the full ForceMetadataFiles container
func (pb *PackageBuilder) ForceMetadataFiles() ForceMetadataFiles {
	pb.Files["package.xml"] = pb.PackageXml()
	return pb.Files
}

// Add a file to the builder
func (pb *PackageBuilder) AddFile(fpath string) (fname string, err error) {
	fpath, err = filepath.Abs(fpath)
	if err != nil {
		return
	}
	_, err = os.Stat(fpath)
	if err != nil {
		return
	}

	metaName, fname := getMetaTypeFromPath(fpath)
	if len(strings.Split(fname, ".")) == 1 {
		pb.AddMetaToPackage(metaName, fname)
	}

	// If it's a push, we want to actually add the files
	if pb.IsPush {
		err = pb.addFileToWorkingDir(metaName, fpath)
	}

	return
}

// Adds the file to a temp directory for deploy
func (pb *PackageBuilder) addFileToWorkingDir(metaName string, fpath string) (err error) {
	// Get relative dir from source
	srcDir := filepath.Dir(filepath.Dir(fpath))
	frel, _ := filepath.Rel(srcDir, fpath)

	// Try to find meta file
	hasMeta := true
	fmeta := fpath + "-meta.xml"
	fmetarel := ""
	if _, err = os.Stat(fmeta); err != nil {
		if os.IsNotExist(err) {
			hasMeta = false
		} else {
			// Has error
			return
		}
	} else {
		// Should be present since we worked back to srcDir
		fmetarel, _ = filepath.Rel(srcDir, fmeta)
	}

	fdata, err := ioutil.ReadFile(fpath)
	if err != nil {
		return
	}

	if metaName == "AuraDefinitionBundle" {
		frel = filepath.Join("aura", frel)
	}
	pb.Files[frel] = fdata
	if hasMeta {
		fdata, err = ioutil.ReadFile(fmeta)
		pb.Files[fmetarel] = fdata
		return
	}

	return
}

// Adds a metadata name to the pending package
func (pb *PackageBuilder) AddMetaToPackage(metaName string, name string) {
	mt := pb.Metadata[metaName]
	if mt.Name == "" {
		mt.Name = metaName
	}

	mt.Members = append(mt.Members, name)
	pb.Metadata[metaName] = mt
}

// Gets metadata type name and target name from a file path
func getMetaTypeFromPath(fpath string) (metaName string, name string) {
	fpath, err := filepath.Abs(fpath)
	if err != nil {
		ErrorAndExit("Cound not find " + fpath)
	}
	if _, err := os.Stat(fpath); err != nil {
		ErrorAndExit("Cound not open " + fpath)
	}

	// Get name of file
	name = filepath.Base(fpath)
	name = strings.TrimSuffix(name, filepath.Ext(name))

	// Get the directory containing the file
	fdir := filepath.Dir(fpath)

	// Get the meta type for that directory
	metaName = getMetaForPath(fdir)
	return
}

// Gets partial path based on a meta type name
func getPathForMeta(metaname string) string {
	for _, mp := range metapaths {
		if strings.EqualFold(mp.name, metaname) {
			return mp.path
		}
	}

	// Unknown, so use metaname
	return metaname
}

// Gets meta type name based on a partial path
func getMetaForPath(path string) string {
	mpath := filepath.Base(path)
	for _, mp := range metapaths {
		if mp.path == mpath {
			return mp.name
		}
	}

	// Check to see if this is aura/lightning
	if strings.HasSuffix(filepath.Dir(path), "metadata/aura") {
		return "AuraDefinitionBundle"
	}
	// Unknown, so use path
	return mpath
}

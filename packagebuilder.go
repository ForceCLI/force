package main

import (
	"encoding/xml"
	"io/ioutil"
	"os"
	"path/filepath"
	"regexp"
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
	path       string
	name       string
	hasFolder  bool
	onlyFolder bool
}

var metapaths = []metapath{
	metapath{path: "applications", name: "CustomApplication"},
	metapath{path: "assignmentRules", name: "AssignmentRules"},
	metapath{path: "aura", name: "AuraDefinitionBundle", hasFolder: true, onlyFolder: true},
	metapath{path: "autoResponseRules", name: "AutoResponseRules"},
	metapath{path: "classes", name: "ApexClass"},
	metapath{path: "communities", name: "Community"},
	metapath{path: "components", name: "ApexComponent"},
	metapath{path: "connectedApps", name: "ConnectedApp"},
	metapath{path: "customMetadata", name: "CustomMetadata"},
	metapath{path: "customPermissions", name: "CustomPermission"},
	metapath{path: "dashboards", name: "Dashboard", hasFolder: true},
	metapath{path: "documents", name: "Document", hasFolder: true},
	metapath{path: "email", name: "EmailTemplate", hasFolder: true},
	metapath{path: "flexipages", name: "FlexiPage"},
	metapath{path: "groups", name: "Group"},
	metapath{path: "homePageLayouts", name: "HomePageLayout"},
	metapath{path: "installedPackages", name: "InstalledPackage"},
	metapath{path: "labels", name: "CustomLabels"},
	metapath{path: "layouts", name: "Layout"},
	metapath{path: "objects", name: "CustomObject"},
	metapath{path: "objectTranslations", name: "CustomObjectTranslation"},
	metapath{path: "pages", name: "ApexPage"},
	metapath{path: "permissionsets", name: "PermissionSet"},
	metapath{path: "profiles", name: "Profile"},
	metapath{path: "queues", name: "Queue"},
	metapath{path: "quickActions", name: "QuickAction"},
	metapath{path: "remoteSiteSettings", name: "RemoteSiteSetting"},
	metapath{path: "reports", name: "Report", hasFolder: true},
	metapath{path: "reportTypes", name: "ReportType"},
	metapath{path: "roles", name: "Role"},
	metapath{path: "scontrols", name: "Scontrol"},
	metapath{path: "settings", name: "Settings"},
	metapath{path: "sharingRules", name: "SharingRules"},
	metapath{path: "staticresources", name: "StaticResource"},
	metapath{path: "tabs", name: "CustomTab"},
	metapath{path: "triggers", name: "ApexTrigger"},
	metapath{path: "workflows", name: "Workflow"},
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

	isDestructiveChanges, err := regexp.MatchString("destructiveChanges(Pre|Post)?"+regexp.QuoteMeta(".")+"xml", fpath)
	if err != nil {
		return
	}

	metaName, fname := getMetaTypeFromPath(fpath)
	if !isDestructiveChanges && !strings.HasSuffix(fpath, "-meta.xml") {
		pb.AddMetaToPackage(metaName, fname)
	}

	// If it's a push, we want to actually add the files
	if pb.IsPush {
		if isDestructiveChanges {
			err = pb.addDestructiveChanges(fpath)
		} else {
			err = pb.addFileToWorkingDir(metaName, fpath)
		}
	}

	return
}

// Adds the file to a temp directory for deploy
func (pb *PackageBuilder) addFileToWorkingDir(metaName string, fpath string) (err error) {
	// Get relative dir from source
	srcDir := filepath.Dir(filepath.Dir(fpath))
	for _, mp := range metapaths {
		if metaName == mp.name && mp.hasFolder {
			srcDir = filepath.Dir(srcDir)
		}
	}
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

	pb.Files[frel] = fdata
	if hasMeta {
		fdata, err = ioutil.ReadFile(fmeta)
		pb.Files[fmetarel] = fdata
		return
	}

	return
}

func (pb *PackageBuilder) addDestructiveChanges(fpath string) (err error) {
	fdata, err := ioutil.ReadFile(fpath)
	if err != nil {
		return
	}

	frel, _ := filepath.Rel(filepath.Dir(fpath), fpath)
	pb.Files[frel] = fdata

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

	// Get the metadata type and name for the file
	metaName, fileName := getMetaForPath(fpath)
	name = strings.TrimSuffix(fileName, filepath.Ext(fileName))
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

// Gets meta type and name based on a path
func getMetaForPath(path string) (metaName string, objectName string) {
	parentDir := filepath.Dir(path)
	parentName := filepath.Base(parentDir)
	grandparentName := filepath.Base(filepath.Dir(parentDir))
	fileName := filepath.Base(path)

	for _, mp := range metapaths {
		if mp.hasFolder && grandparentName == mp.path {
			metaName = mp.name
			if mp.onlyFolder {
				objectName = parentName
			} else {
				objectName = parentName + "/" + fileName
			}
			return
		}
		if mp.path == parentName {
			metaName = mp.name
			objectName = fileName
			return
		}
	}

	// Unknown, so use path
	metaName = parentName
	objectName = fileName
	return
}

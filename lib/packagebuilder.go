package lib

import (
	"encoding/xml"
	"fmt"
	"io/ioutil"
	"os"
	"path/filepath"
	"regexp"
	"strings"

	. "github.com/ForceCLI/force/error"
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
	extension  string
}

var metapaths = []metapath{
	metapath{path: "actionLinkGroupTemplates", name: "ActionLinkGroupTemplate"},
	metapath{path: "analyticSnapshots", name: "AnalyticSnapshot"},
	metapath{path: "applications", name: "CustomApplication"},
	metapath{path: "appMenus", name: "AppMenu"},
	metapath{path: "approvalProcesses", name: "ApprovalProcess"},
	metapath{path: "assignmentRules", name: "AssignmentRules"},
	metapath{path: "audience", name: "Audience"},
	metapath{path: "authproviders", name: "AuthProvider"},
	metapath{path: "aura", name: "AuraDefinitionBundle", hasFolder: true, onlyFolder: true},
	metapath{path: "autoResponseRules", name: "AutoResponseRules"},
	metapath{path: "callCenters", name: "CallCenter"},
	metapath{path: "cachePartitions", name: "PlatformCachePartition"},
	metapath{path: "certs", name: "Certificate"},
	metapath{path: "channelLayouts", name: "ChannelLayout"},
	metapath{path: "classes", name: "ApexClass"},
	metapath{path: "communities", name: "Community"},
	metapath{path: "components", name: "ApexComponent"},
	metapath{path: "connectedApps", name: "ConnectedApp"},
	metapath{path: "contentassets", name: "ContentAsset"},
	metapath{path: "corsWhitelistOrigins", name: "CorsWhitelistOrigin"},
	metapath{path: "customApplicationComponents", name: "CustomApplicationComponent"},
	metapath{path: "customMetadata", name: "CustomMetadata"},
	metapath{path: "notificationtypes", name: "CustomNotificationType"},
	metapath{path: "customHelpMenuSections", name: "CustomHelpMenuSection"},
	metapath{path: "customPermissions", name: "CustomPermission"},
	metapath{path: "dashboards", name: "Dashboard", hasFolder: true},
	metapath{path: "dataSources", name: "ExternalDataSource"},
	metapath{path: "datacategorygroups", name: "DataCategoryGroup"},
	metapath{path: "delegateGroups", name: "DelegateGroup"},
	metapath{path: "documents", name: "Document", hasFolder: true},
	metapath{path: "duplicateRules", name: "DuplicateRule"},
	metapath{path: "dw", name: "DataWeaveResource"},
	metapath{path: "EmbeddedServiceConfig", name: "EmbeddedServiceConfig"},
	metapath{path: "email", name: "EmailTemplate", hasFolder: true},
	metapath{path: "escalationRules", name: "EscalationRules"},
	metapath{path: "experiences", name: "ExperienceBundle"},
	metapath{path: "feedFilters", name: "CustomFeedFilter"},
	metapath{path: "flexipages", name: "FlexiPage"},
	metapath{path: "flowDefinitions", name: "FlowDefinition"},
	metapath{path: "flows", name: "Flow"},
	metapath{path: "globalPicklists", name: "GlobalPicklist"},
	metapath{path: "globalValueSets", name: "GlobalValueSet"},
	metapath{path: "globalValueSetTranslations", name: "GlobalValueSetTranslation"},
	metapath{path: "groups", name: "Group"},
	metapath{path: "homePageComponents", name: "HomePageComponent"},
	metapath{path: "homePageLayouts", name: "HomePageLayout"},
	metapath{path: "installedPackages", name: "InstalledPackage"},
	metapath{path: "labels", name: "CustomLabels"},
	metapath{path: "layouts", name: "Layout"},
	metapath{path: "LeadConvertSettings", name: "LeadConvertSettings"},
	metapath{path: "letterhead", name: "Letterhead"},
	metapath{path: "lwc", name: "LightningComponentBundle", hasFolder: true, onlyFolder: true},
	metapath{path: "matchingRules", name: "MatchingRules"},
	metapath{path: "matchingRules", name: "MatchingRule"},
	metapath{path: "namedCredentials", name: "NamedCredential"},
	metapath{path: "networks", name: "Network"},
	metapath{path: "objects", name: "CustomObject"},
	metapath{path: "objectTranslations", name: "CustomObjectTranslation"},
	metapath{path: "pages", name: "ApexPage"},
	metapath{path: "pathAssistants", name: "PathAssistant"},
	metapath{path: "permissionsets", name: "PermissionSet"},
	metapath{path: "permissionsetgroups", name: "PermissionSetGroup"},
	metapath{path: "platformEventChannels", name: "PlatformEventChannel"},
	metapath{path: "platformEventChannelMembers", name: "PlatformEventChannelMember"},
	metapath{path: "PlatformEventSubscriberConfigs", name: "PlatformEventSubscriberConfig"},
	metapath{path: "postTemplates", name: "PostTemplate"},
	metapath{path: "profiles", name: "Profile", extension: ".profile"},
	metapath{path: "postTemplates", name: "PostTemplate"},
	metapath{path: "postTemplates", name: "PostTemplate"},
	metapath{path: "profiles", name: "Profile"},
	metapath{path: "profileSessionSettings", name: "ProfileSessionSetting"},
	metapath{path: "queues", name: "Queue"},
	metapath{path: "quickActions", name: "QuickAction"},
	metapath{path: "restrictionRules", name: "RestrictionRule"},
	metapath{path: "remoteSiteSettings", name: "RemoteSiteSetting"},
	metapath{path: "reports", name: "Report", hasFolder: true},
	metapath{path: "reportTypes", name: "ReportType"},
	metapath{path: "roles", name: "Role"},
	metapath{path: "scontrols", name: "Scontrol"},
	metapath{path: "settings", name: "Settings"},
	metapath{path: "sharingRules", name: "SharingRules"},
	metapath{path: "siteDotComSites", name: "SiteDotCom"},
	metapath{path: "sites", name: "CustomSite"},
	metapath{path: "standardValueSets", name: "StandardValueSet"},
	metapath{path: "staticresources", name: "StaticResource"},
	metapath{path: "synonymDictionaries", name: "SynonymDictionary"},
	metapath{path: "tabs", name: "CustomTab"},
	metapath{path: "translations", name: "Translations"},
	metapath{path: "triggers", name: "ApexTrigger"},
	metapath{path: "weblinks", name: "CustomPageWebLink"},
	metapath{path: "workflows", name: "Workflow"},
	metapath{path: "cspTrustedSites", name: "CspTrustedSite"},
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

// Returns the source file path for a given metadata file path.
func MetaPathToSourcePath(mpath string) (spath string) {
	spath = strings.TrimSuffix(mpath, "-meta.xml")
	if spath == mpath {
		return
	}

	_, err := os.Stat(spath)
	if err != nil {
		spath = mpath
	}
	return
}

// Add a file to the builder
func (pb *PackageBuilder) AddFile(fpath string) (fname string, err error) {
	out, err := pb.addFile(fpath)
	return out.Filename, err
}

type addFileOutput struct {
	Filename string
	IsBad    bool
}

func isFolderMetadata(path string) bool {
	if !strings.HasSuffix(path, "-meta.xml") {
		return false
	}
	dirPath := strings.TrimSuffix(path, "-meta.xml")
	f, err := os.Stat(dirPath)
	if err != nil {
		return false
	}
	return f.Mode().IsDir()
}

func (pb *PackageBuilder) addFile(fpath string) (out addFileOutput, err error) {
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

	if lwcJsTestFile.MatchString(fpath) {
		// If this is a JS test file, just ignore it entirely,
		// don't consider it bad.
		return
	}

	isFolderMetadata := isFolderMetadata(fpath)
	// Path with -meta.xml stripped
	spath := MetaPathToSourcePath(fpath)
	metaName, fname := getMetaTypeFromPath(spath)
	out.Filename = fname
	if isFolderMetadata {
		pb.AddMetaToPackage(metaName, fname)
	} else if !isDestructiveChanges && !strings.HasSuffix(spath, "-meta.xml") {
		pb.AddMetaToPackage(metaName, fname)
	}

	// If it's a push, we want to actually add the files
	if pb.IsPush {
		if isDestructiveChanges {
			err = pb.addFileOnly(fpath)
		} else if isFolderMetadata {
			err = pb.addFileOnlyWithDir(fpath)
		} else {
			err = pb.addFileAndMetadata(metaName, spath)
		}
	}

	out.IsBad = out.Filename == ""
	return
}

// AddDirectory Recursively add files contained in provided directory
func (pb *PackageBuilder) AddDirectory(fpath string) (namePaths map[string]string, badPaths []string, err error) {
	namePaths = make(map[string]string)

	files, err := ioutil.ReadDir(fpath)
	if err != nil {
		badPaths = append(badPaths, fpath)
		return
	}

	for _, f := range files {
		dirOrFilePath := fpath + "/" + f.Name()
		if f.IsDir() {
			if lwcJsTestDir.MatchString(dirOrFilePath) {
				// Normally malformed paths would indicate invalid metadata,
				// but LWC tests should never be deployed. We may want to consider this logic/behavior,
				// such that we don't call `addFile` on directories in some cases; if we could
				// avoid the addFile call on the __tests__ dir, we could avoid this check.
				continue
			}
			dirNamePaths, dirBadPath, err := pb.AddDirectory(dirOrFilePath)
			if err != nil {
				badPaths = append(badPaths, dirBadPath...)
			} else {
				for dirContentName, dirContentPath := range dirNamePaths {
					namePaths[dirContentName] = dirContentPath
				}
			}
		}

		addFileOut, err := pb.addFile(dirOrFilePath)

		if (err != nil) || (addFileOut.IsBad) {
			badPaths = append(badPaths, dirOrFilePath)
		} else {
			namePaths[addFileOut.Filename] = dirOrFilePath
		}
	}
	return
}

// Adds the file to a temp directory for deploy
func (pb *PackageBuilder) addFileAndMetadata(metaName string, fpath string) (err error) {
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

// e.g. add /path/to/src/destructiveChanges.xml to zip file as
// destructiveChanges.xml
func (pb *PackageBuilder) addFileOnly(fpath string) (err error) {
	fdata, err := ioutil.ReadFile(fpath)
	if err != nil {
		return
	}

	frel, _ := filepath.Rel(filepath.Dir(fpath), fpath)
	pb.Files[frel] = fdata

	return
}

// e.g. add /path/to/src/dashboards/Example-meta.xml to zip file as
// dashboards/Example-meta.xml
func (pb *PackageBuilder) addFileOnlyWithDir(fpath string) (err error) {
	fdata, err := ioutil.ReadFile(fpath)
	if err != nil {
		return
	}

	srcDir := filepath.Dir(filepath.Dir(fpath))
	frel, _ := filepath.Rel(srcDir, fpath)
	pb.Files[frel] = fdata

	return
}

func (pb *PackageBuilder) contains(members []string, name string) bool {
	for _, a := range members {
		if a == name {
			return true
		}
	}
	return false
}

// Adds a metadata name to the pending package
func (pb *PackageBuilder) AddMetaToPackage(metaName string, name string) {
	mt := pb.Metadata[metaName]
	if mt.Name == "" {
		mt.Name = metaName
	}

	if !pb.contains(mt.Members, name) {
		mt.Members = append(mt.Members, name)
		pb.Metadata[metaName] = mt
	}
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
	//name = strings.TrimSuffix(name, filepath.Ext(name))
	return
}

func findMetapathForFile(file string) (path metapath) {
	parentDir := filepath.Dir(file)
	parentName := filepath.Base(parentDir)
	grandparentName := filepath.Base(filepath.Dir(parentDir))
	fileExtension := filepath.Ext(file)

	for _, mp := range metapaths {
		if mp.hasFolder && grandparentName == mp.path {
			return mp
		}
		if mp.path == parentName {
			return mp
		}
	}

	// Hmm, maybe we can use the extension to determine the type
	for _, mp := range metapaths {
		if mp.extension == fileExtension {
			return mp
		}
	}
	return
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

var lwcJsTestFile = regexp.MustCompile(".*\\.test\\.js$")
var lwcJsTestDir = regexp.MustCompile(fmt.Sprintf("%s__tests__$", regexp.QuoteMeta(string(os.PathSeparator))))

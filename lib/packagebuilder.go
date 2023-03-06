package lib

import (
	"encoding/xml"
	"fmt"
	"io/ioutil"
	"os"
	"path/filepath"
	"regexp"
	"strings"

	"github.com/pkg/errors"
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
	{path: "actionLinkGroupTemplates", name: "ActionLinkGroupTemplate"},
	{path: "analyticSnapshots", name: "AnalyticSnapshot"},
	{path: "applications", name: "CustomApplication"},
	{path: "appMenus", name: "AppMenu"},
	{path: "approvalProcesses", name: "ApprovalProcess"},
	{path: "assignmentRules", name: "AssignmentRules"},
	{path: "audience", name: "Audience"},
	{path: "authproviders", name: "AuthProvider"},
	{path: "aura", name: "AuraDefinitionBundle", hasFolder: true, onlyFolder: true},
	{path: "autoResponseRules", name: "AutoResponseRules"},
	{path: "callCenters", name: "CallCenter"},
	{path: "cachePartitions", name: "PlatformCachePartition"},
	{path: "certs", name: "Certificate"},
	{path: "channelLayouts", name: "ChannelLayout"},
	{path: "classes", name: "ApexClass"},
	{path: "communities", name: "Community"},
	{path: "components", name: "ApexComponent"},
	{path: "connectedApps", name: "ConnectedApp"},
	{path: "contentassets", name: "ContentAsset"},
	{path: "corsWhitelistOrigins", name: "CorsWhitelistOrigin"},
	{path: "customApplicationComponents", name: "CustomApplicationComponent"},
	{path: "customMetadata", name: "CustomMetadata"},
	{path: "notificationtypes", name: "CustomNotificationType"},
	{path: "customHelpMenuSections", name: "CustomHelpMenuSection"},
	{path: "customPermissions", name: "CustomPermission"},
	{path: "dashboards", name: "Dashboard", hasFolder: true},
	{path: "dataSources", name: "ExternalDataSource"},
	{path: "datacategorygroups", name: "DataCategoryGroup"},
	{path: "delegateGroups", name: "DelegateGroup"},
	{path: "documents", name: "Document", hasFolder: true},
	{path: "duplicateRules", name: "DuplicateRule"},
	{path: "dw", name: "DataWeaveResource"},
	{path: "EmbeddedServiceConfig", name: "EmbeddedServiceConfig"},
	{path: "email", name: "EmailTemplate", hasFolder: true},
	{path: "escalationRules", name: "EscalationRules"},
	{path: "experiences", name: "ExperienceBundle"},
	{path: "feedFilters", name: "CustomFeedFilter"},
	{path: "flexipages", name: "FlexiPage"},
	{path: "flowDefinitions", name: "FlowDefinition"},
	{path: "flows", name: "Flow"},
	{path: "globalPicklists", name: "GlobalPicklist"},
	{path: "globalValueSets", name: "GlobalValueSet"},
	{path: "globalValueSetTranslations", name: "GlobalValueSetTranslation"},
	{path: "groups", name: "Group"},
	{path: "homePageComponents", name: "HomePageComponent"},
	{path: "homePageLayouts", name: "HomePageLayout"},
	{path: "installedPackages", name: "InstalledPackage"},
	{path: "labels", name: "CustomLabels"},
	{path: "layouts", name: "Layout"},
	{path: "LeadConvertSettings", name: "LeadConvertSettings"},
	{path: "letterhead", name: "Letterhead"},
	{path: "lwc", name: "LightningComponentBundle", hasFolder: true, onlyFolder: true},
	{path: "matchingRules", name: "MatchingRules"},
	{path: "matchingRules", name: "MatchingRule"},
	{path: "namedCredentials", name: "NamedCredential"},
	{path: "notificationTypeConfig", name: "NotificationTypeConfig"},
	{path: "networks", name: "Network"},
	{path: "objects", name: "CustomObject"},
	{path: "objectTranslations", name: "CustomObjectTranslation"},
	{path: "pages", name: "ApexPage"},
	{path: "pathAssistants", name: "PathAssistant"},
	{path: "permissionsets", name: "PermissionSet"},
	{path: "permissionsetgroups", name: "PermissionSetGroup"},
	{path: "platformEventChannels", name: "PlatformEventChannel"},
	{path: "platformEventChannelMembers", name: "PlatformEventChannelMember"},
	{path: "PlatformEventSubscriberConfigs", name: "PlatformEventSubscriberConfig"},
	{path: "postTemplates", name: "PostTemplate"},
	{path: "profiles", name: "Profile", extension: ".profile"},
	{path: "postTemplates", name: "PostTemplate"},
	{path: "postTemplates", name: "PostTemplate"},
	{path: "profiles", name: "Profile"},
	{path: "profileSessionSettings", name: "ProfileSessionSetting"},
	{path: "queues", name: "Queue"},
	{path: "quickActions", name: "QuickAction"},
	{path: "restrictionRules", name: "RestrictionRule"},
	{path: "remoteSiteSettings", name: "RemoteSiteSetting"},
	{path: "reports", name: "Report", hasFolder: true},
	{path: "reportTypes", name: "ReportType"},
	{path: "roles", name: "Role"},
	{path: "scontrols", name: "Scontrol"},
	{path: "settings", name: "Settings"},
	{path: "sharingRules", name: "SharingRules"},
	{path: "siteDotComSites", name: "SiteDotCom"},
	{path: "sites", name: "CustomSite"},
	{path: "standardValueSets", name: "StandardValueSet"},
	{path: "staticresources", name: "StaticResource"},
	{path: "synonymDictionaries", name: "SynonymDictionary"},
	{path: "tabs", name: "CustomTab"},
	{path: "translations", name: "Translations"},
	{path: "triggers", name: "ApexTrigger"},
	{path: "weblinks", name: "CustomPageWebLink"},
	{path: "workflows", name: "Workflow"},
	{path: "cspTrustedSites", name: "CspTrustedSite"},
}

type PackageBuilder struct {
	IsPush   bool
	Metadata map[string]MetaType
	Files    ForceMetadataFiles
	Root     string
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

func (pb *PackageBuilder) AddFile(fpath string) error {
	fpath, err := filepath.Abs(fpath)
	if err != nil {
		return err
	}
	_, err = os.Stat(fpath)
	if err != nil {
		return err
	}

	isDestructiveChanges, err := regexp.MatchString("destructiveChanges(Pre|Post)?"+regexp.QuoteMeta(".")+"xml", fpath)
	if err != nil {
		return err
	}
	if isDestructiveChanges {
		err = pb.addFileOnly(fpath)
		return err
	}

	if lwcJsTestFile.MatchString(fpath) {
		// If this is a JS test file, just ignore it entirely,
		// don't consider it bad.
		return nil
	}

	isFolderMetadata := isFolderMetadata(fpath)
	// Path with -meta.xml stripped
	spath := MetaPathToSourcePath(fpath)
	metaName, fname, err := pb.getMetaTypeForRelativePath(spath)
	if err != nil {
		return err
	}
	if isFolderMetadata {
		pb.AddMetaToPackage(metaName, fname)
	} else if !strings.HasSuffix(spath, "-meta.xml") {
		pb.AddMetaToPackage(metaName, fname)
	}
	if pb.isComponent(fpath) {
		pb.AddMetaToPackage(metaName, fname)
	}

	// If it's a push, we want to actually add the files
	if pb.IsPush {
		if isFolderMetadata {
			err = pb.addFileOnly(fpath)
		} else {
			err = pb.addFileAndMetaXml(spath)
		}
	}

	return nil
}

func (pb *PackageBuilder) AddMetadataType(metadataType string) error {
	metaFolder, err := pb.MetadataDir(metadataType)
	if err != nil {
		return fmt.Errorf("Could not get metadata directry: %w", err)
	}
	return pb.AddDirectory(metaFolder)
}

func (pb *PackageBuilder) AddMetadataItem(metadataType string, name string) error {
	metaFolder, err := pb.MetadataDir(metadataType)
	if err != nil {
		return fmt.Errorf("Could not get metadata directry: %w", err)
	}
	if filePath, err := findMetadataPath(metaFolder, name); err != nil {
		return fmt.Errorf("Could not find path for %s of type %s: %w", name, metadataType, err)
	} else {
		return pb.Add(filePath)
	}
}

func (pb *PackageBuilder) Add(path string) error {
	f, err := os.Stat(path)
	if err != nil {
		return err
	}
	if f.Mode().IsDir() {
		return pb.AddDirectory(path)
	} else {
		return pb.AddFile(path)
	}
}

// AddDirectory Recursively add files contained in provided directory
func (pb *PackageBuilder) AddDirectory(fpath string) error {
	fpath, err := filepath.Abs(fpath)
	if err != nil {
		return fmt.Errorf("Cound not find %s: %w", fpath, err)
	}

	isComponent := pb.isComponent(fpath)
	metadataType, metadataName, err := pb.getMetaTypeForRelativePath(fpath)
	if err != nil {
		return fmt.Errorf("Unable to add directory: %w", err)
	}
	if isComponent && metadataName != "" {
		pb.AddMetaToPackage(metadataType, metadataName)
	}

	if m := correspondingMetadata(fpath); m != "" {
		if err = pb.AddFile(m); err != nil {
			return fmt.Errorf("Failed to add metadata for directory: %w", err)
		}
	}

	files, err := ioutil.ReadDir(fpath)
	if err != nil {
		return err
	}

	for _, f := range files {
		dirOrFilePath := fpath + "/" + f.Name()
		if strings.HasPrefix(f.Name(), ".") {
			Log.Info("Ignoring hidden file: " + dirOrFilePath)
			continue
		}

		if f.IsDir() {
			if lwcJsTestDir.MatchString(dirOrFilePath) {
				// Normally malformed paths would indicate invalid metadata,
				// but LWC tests should never be deployed. We may want to consider this logic/behavior,
				// such that we don't call `addFile` on directories in some cases; if we could
				// avoid the addFile call on the __tests__ dir, we could avoid this check.
				continue
			}
			err := pb.AddDirectory(dirOrFilePath)
			if err != nil {
				return err
			}
			continue
		}

		if isComponent {
			err = pb.addFileOnly(dirOrFilePath)
		} else {
			err = pb.AddFile(dirOrFilePath)
		}

	}
	return err
}

func (pb *PackageBuilder) isComponent(fpath string) bool {
	relativePath, _ := filepath.Rel(pb.Root, fpath)
	parts := strings.Split(relativePath, string(os.PathSeparator))
	if len(parts) == 0 {
		return false
	}
	metadataRoot := parts[0]
	for _, mp := range metapaths {
		if metadataRoot == mp.path {
			return mp.onlyFolder
		}
	}
	return false
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

func correspondingMetadata(path string) string {
	fmeta := path + "-meta.xml"
	if _, err := os.Stat(fmeta); err != nil {
		return ""
	}
	return fmeta
}

// Adds the file to a temp directory for deploy
func (pb *PackageBuilder) addFileAndMetaXml(fpath string) error {
	fdata, err := ioutil.ReadFile(fpath)
	if err != nil {
		return errors.Wrap(err, "failed to add file")
	}
	frel, err := filepath.Rel(pb.Root, fpath)
	if err != nil {
		return err
	}
	pb.Files[frel] = fdata

	// Try to find meta file
	fmeta := fpath + "-meta.xml"
	if _, err = os.Stat(fmeta); err != nil {
		if os.IsNotExist(err) {
			return nil
		} else {
			// Has error
			return errors.Wrap(err, "failed to add file metadata")
		}
	}
	fmetarel, _ := filepath.Rel(pb.Root, fmeta)
	fdata, err = ioutil.ReadFile(fmeta)
	if err != nil {
		return err
	}
	pb.Files[fmetarel] = fdata

	return nil
}

// e.g. add /path/to/src/destructiveChanges.xml to zip file as
// destructiveChanges.xml
func (pb *PackageBuilder) addFileOnly(fpath string) (err error) {
	fdata, err := ioutil.ReadFile(fpath)
	if err != nil {
		return
	}

	frel, err := filepath.Rel(pb.Root, fpath)
	if err != nil {
		return err
	}
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
func (pb *PackageBuilder) getMetaTypeForRelativePath(fpath string) (metaName string, name string, err error) {
	fpath, err = filepath.Abs(fpath)
	if err != nil {
		return "", "", fmt.Errorf("Cound not find %s: %w", fpath, err)
	}
	if _, err := os.Stat(fpath); err != nil {
		return "", "", fmt.Errorf("Cound not open %s: %w", fpath, err)
	}

	// Get the metadata type and name for the file
	return pb.GetMetaForAbsolutePath(fpath)
}

func FindMetapathForFile(file string) (string, error) {
	parentDir := filepath.Dir(file)
	parentName := filepath.Base(parentDir)
	grandparentName := filepath.Base(filepath.Dir(parentDir))
	fileExtension := filepath.Ext(file)

	for _, mp := range metapaths {
		if mp.hasFolder && grandparentName == mp.path {
			return mp.path, nil
		}
		if mp.path == parentName {
			return mp.path, nil
		}
	}

	// Hmm, maybe we can use the extension to determine the type
	for _, mp := range metapaths {
		if mp.extension == fileExtension {
			return mp.path, nil
		}
	}
	return "", fmt.Errorf("metadata path not found")
}

// Gets meta type and name based on a path
func (pb *PackageBuilder) GetMetaForAbsolutePath(path string) (metaName string, objectName string, err error) {
	if pb.Root == "" {
		return "", "", errors.Wrap(err, "PackageBuilder.Root is not set")
	}
	relativePath, err := filepath.Rel(pb.Root, path)
	if err != nil {
		return "", "", errors.Wrap(err, "Failed to create relative path")
	}
	parts := strings.Split(relativePath, string(os.PathSeparator))
	metadataRoot := parts[0]
	objectName = ""
	if len(parts) > 1 {
		objectName = strings.TrimSuffix(strings.Join(parts[1:], string(os.PathSeparator)), filepath.Ext(path))
		if pb.isComponent(path) {
			objectName = parts[1]
		}
	}

	for _, mp := range metapaths {
		if metadataRoot == mp.path {
			return mp.name, objectName, nil
		}
	}

	return "", "", fmt.Errorf("Unable to identify metadata type for %s", path)
}

func (pb *PackageBuilder) MetadataDir(metadataType string) (path string, err error) {
	for _, mp := range metapaths {
		if strings.ToLower(metadataType) == strings.ToLower(mp.name) {
			return filepath.Join(pb.Root, mp.path), nil
		}
	}
	return "", fmt.Errorf("Unknown metadata type: %s", metadataType)
}

// Get the path to a metadata file from the source folder and metadata name
func findMetadataPath(folder string, metadataName string) (string, error) {
	info, err := os.Stat(folder)
	if err != nil {
		return "", err
	}
	if !info.IsDir() {
		return "", fmt.Errorf("Invalid directory %s", folder)
	}
	filePath := ""
	err = filepath.Walk(folder, func(path string, f os.FileInfo, err error) error {
		ext := filepath.Ext(f.Name())
		if err != nil {
			Log.Info("Error looking for metadata: " + err.Error())
			return nil
		}
		rel, err := filepath.Rel(folder, path)
		if err != nil {
			return err
		}
		if strings.ToLower(strings.TrimSuffix(rel, ext)) == strings.ToLower(metadataName) {
			filePath = path
		}
		return nil
	})
	if err != nil {
		Log.Info("Error looking for metadata: " + err.Error())
		return "", err
	}
	if filePath == "" {
		return "", fmt.Errorf("Failed to find %s in %s", metadataName, folder)
	}
	return filePath, nil
}

var lwcJsTestFile = regexp.MustCompile(".*\\.test\\.js$")
var lwcJsTestDir = regexp.MustCompile(fmt.Sprintf("%s__tests__$", regexp.QuoteMeta(string(os.PathSeparator))))

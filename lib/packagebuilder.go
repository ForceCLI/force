package lib

import (
	"encoding/xml"
	"fmt"
	"io"
	"io/ioutil"
	"os"
	"path/filepath"
	"regexp"
	"strings"

	"github.com/ForceCLI/force-md/metadata"
	"github.com/ForceCLI/force-md/metadata/objectTranslations"
	"github.com/ForceCLI/force-md/metadata/objects"
	"github.com/ForceCLI/force-md/repo"
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
	// extension is the suffix of the type's file names. Bundles and documents
	// have none: their files keep the extensions of their content.
	extension string
	// contentFile marks types that keep a content file beside the -meta.xml
	// file in both source and metadata format (Apex classes, static
	// resources). Every other type is a single file, which source format
	// names with a -meta.xml suffix.
	contentFile bool
}

var metapaths = []metapath{
	{path: "actionLinkGroupTemplates", name: "ActionLinkGroupTemplate", extension: ".actionLinkGroupTemplate"},
	{path: "analyticSnapshots", name: "AnalyticSnapshot", extension: ".snapshot"},
	{path: "apexEmailNotifications", name: "ApexEmailNotifications", extension: ".notifications"},
	{path: "applications", name: "CustomApplication", extension: ".app"},
	{path: "appMenus", name: "AppMenu", extension: ".appMenu"},
	{path: "approvalProcesses", name: "ApprovalProcess", extension: ".approvalProcess"},
	{path: "assignmentRules", name: "AssignmentRules", extension: ".assignmentRules"},
	{path: "audience", name: "Audience", extension: ".audience"},
	{path: "authproviders", name: "AuthProvider", extension: ".authprovider"},
	{path: "aura", name: "AuraDefinitionBundle", hasFolder: true, onlyFolder: true},
	{path: "autoResponseRules", name: "AutoResponseRules", extension: ".autoResponseRules"},
	{path: "brandingSets", name: "BrandingSet", extension: ".brandingSet"},
	{path: "callCenters", name: "CallCenter", extension: ".callCenter"},
	{path: "cachePartitions", name: "PlatformCachePartition", extension: ".cachePartition"},
	{path: "certs", name: "Certificate", extension: ".crt", contentFile: true},
	{path: "channelLayouts", name: "ChannelLayout", extension: ".channelLayout"},
	{path: "classes", name: "ApexClass", extension: ".cls", contentFile: true},
	{path: "cleanDataServices", name: "CleanDataService", extension: ".cleanDataService"},
	{path: "communities", name: "Community", extension: ".community"},
	{path: "components", name: "ApexComponent", extension: ".component", contentFile: true},
	{path: "connectedApps", name: "ConnectedApp", extension: ".connectedApp"},
	{path: "contentassets", name: "ContentAsset", extension: ".asset", contentFile: true},
	{path: "corsWhitelistOrigins", name: "CorsWhitelistOrigin", extension: ".corsWhitelistOrigin"},
	{path: "customApplicationComponents", name: "CustomApplicationComponent", extension: ".customApplicationComponent"},
	{path: "customMetadata", name: "CustomMetadata", extension: ".md"},
	{path: "notificationtypes", name: "CustomNotificationType", extension: ".notiftype"},
	{path: "customHelpMenuSections", name: "CustomHelpMenuSection", extension: ".customHelpMenuSection"},
	{path: "customPermissions", name: "CustomPermission", extension: ".customPermission"},
	{path: "dashboards", name: "Dashboard", extension: ".dashboard", hasFolder: true},
	{path: "dataSources", name: "ExternalDataSource", extension: ".dataSource"},
	{path: "datacategorygroups", name: "DataCategoryGroup", extension: ".datacategorygroup"},
	{path: "delegateGroups", name: "DelegateGroup", extension: ".delegateGroup"},
	{path: "documents", name: "Document", hasFolder: true, contentFile: true},
	{path: "duplicateRules", name: "DuplicateRule", extension: ".duplicateRule"},
	{path: "dw", name: "DataWeaveResource", extension: ".dwl", contentFile: true},
	{path: "EmbeddedServiceConfig", name: "EmbeddedServiceConfig", extension: ".EmbeddedServiceConfig"},
	{path: "email", name: "EmailTemplate", extension: ".email", hasFolder: true, contentFile: true},
	{path: "escalationRules", name: "EscalationRules", extension: ".escalationRules"},
	{path: "experiences", name: "ExperienceBundle", hasFolder: true, onlyFolder: true},
	{path: "externalClientApps", name: "ExternalClientApplication", extension: ".eca"},
	{path: "extlClntAppConfigurablePolicies", name: "ExtlClntAppConfigurablePolicies", extension: ".ecaCnfgPlcy"},
	{path: "extlClntAppGlobalOauthSets", name: "ExtlClntAppGlobalOauthSettings", extension: ".ecaGlblOauth"},
	{path: "extlClntAppOauthPolicies", name: "ExtlClntAppOauthConfigurablePolicies", extension: ".ecaOauthPlcy"},
	{path: "extlClntAppOauthSettings", name: "ExtlClntAppOauthSettings", extension: ".ecaOauth"},
	{path: "externalCredentials", name: "ExternalCredential", extension: ".externalCredential"},
	{path: "externalServiceRegistrations", name: "ExternalServiceRegistration", extension: ".externalServiceRegistration"},
	{path: "feedFilters", name: "CustomFeedFilter", extension: ".feedFilter"},
	{path: "flexipages", name: "FlexiPage", extension: ".flexipage"},
	{path: "flowDefinitions", name: "FlowDefinition", extension: ".flowDefinition"},
	{path: "flows", name: "Flow", extension: ".flow"},
	{path: "flowtests", name: "FlowTest", extension: ".flowtest"},
	{path: "fundraisingConfigs", name: "FundraisingConfig", extension: ".fundraisingConfig"},
	{path: "globalPicklists", name: "GlobalPicklist", extension: ".globalPicklist"},
	{path: "globalValueSets", name: "GlobalValueSet", extension: ".globalValueSet"},
	{path: "globalValueSetTranslations", name: "GlobalValueSetTranslation", extension: ".globalValueSetTranslation"},
	{path: "groups", name: "Group", extension: ".group"},
	{path: "homePageComponents", name: "HomePageComponent", extension: ".homePageComponent"},
	{path: "homePageLayouts", name: "HomePageLayout", extension: ".homePageLayout"},
	{path: "installedPackages", name: "InstalledPackage", extension: ".installedPackage"},
	{path: "labels", name: "CustomLabels", extension: ".labels"},
	{path: "layouts", name: "Layout", extension: ".layout"},
	{path: "LeadConvertSettings", name: "LeadConvertSettings", extension: ".LeadConvertSetting"},
	{path: "letterhead", name: "Letterhead", extension: ".letter"},
	{path: "lwc", name: "LightningComponentBundle", hasFolder: true, onlyFolder: true},
	{path: "matchingRules", name: "MatchingRules", extension: ".matchingRule"},
	{path: "matchingRules", name: "MatchingRule", extension: ".matchingRule"},
	{path: "messageChannels", name: "LightningMessageChannel", extension: ".messageChannel"},
	{path: "namedCredentials", name: "NamedCredential", extension: ".namedCredential"},
	{path: "navigationMenus", name: "NavigationMenu", extension: ".navigationMenu"},
	{path: "notificationTypeConfig", name: "NotificationTypeConfig", extension: ".config"},
	{path: "networks", name: "Network", extension: ".network"},
	{path: "objects", name: "CustomObject", extension: ".object"},
	{path: "objectTranslations", name: "CustomObjectTranslation", extension: ".objectTranslation"},
	{path: "omniDataTransforms", name: "OmniDataTransform", extension: ".rpt"},
	{path: "omniIntegrationProcedures", name: "OmniIntegrationProcedure", extension: ".oip"},
	{path: "omniScripts", name: "OmniScript", extension: ".os"},
	{path: "omniUiCard", name: "OmniUiCard", extension: ".ouc"},
	{path: "pages", name: "ApexPage", extension: ".page", contentFile: true},
	{path: "pathAssistants", name: "PathAssistant", extension: ".pathAssistant"},
	{path: "permissionsets", name: "PermissionSet", extension: ".permissionset"},
	{path: "permissionsetgroups", name: "PermissionSetGroup", extension: ".permissionsetgroup"},
	{path: "platformEventChannels", name: "PlatformEventChannel", extension: ".platformEventChannel"},
	{path: "platformEventChannelMembers", name: "PlatformEventChannelMember", extension: ".platformEventChannelMember"},
	{path: "PlatformEventSubscriberConfigs", name: "PlatformEventSubscriberConfig", extension: ".platformEventSubscriberConfig"},
	{path: "postTemplates", name: "PostTemplate", extension: ".postTemplate"},
	{path: "profiles", name: "Profile", extension: ".profile"},
	{path: "postTemplates", name: "PostTemplate", extension: ".postTemplate"},
	{path: "postTemplates", name: "PostTemplate", extension: ".postTemplate"},
	{path: "profiles", name: "Profile", extension: ".profile"},
	{path: "profileSessionSettings", name: "ProfileSessionSetting", extension: ".profileSessionSetting"},
	{path: "queues", name: "Queue", extension: ".queue"},
	{path: "quickActions", name: "QuickAction", extension: ".quickAction"},
	{path: "RecordAggregationDefinitions", name: "RecordAggregationDefinition", extension: ".RecordAggregationDefinition"},
	{path: "restrictionRules", name: "RestrictionRule", extension: ".rule"},
	{path: "remoteSiteSettings", name: "RemoteSiteSetting", extension: ".remoteSite"},
	{path: "reports", name: "Report", extension: ".report", hasFolder: true},
	{path: "reportTypes", name: "ReportType", extension: ".reportType"},
	{path: "roles", name: "Role", extension: ".role"},
	{path: "scontrols", name: "Scontrol", extension: ".scf", contentFile: true},
	{path: "settings", name: "Settings", extension: ".settings"},
	{path: "sharingRules", name: "SharingRules", extension: ".sharingRules"},
	{path: "sharingSets", name: "SharingSet", extension: ".sharingSet"},
	{path: "siteDotComSites", name: "SiteDotCom", extension: ".site", contentFile: true},
	{path: "sites", name: "CustomSite", extension: ".site"},
	{path: "slackapps", name: "SlackApp", extension: ".slackapp", contentFile: true},
	{path: "standardValueSets", name: "StandardValueSet", extension: ".standardValueSet"},
	{path: "staticresources", name: "StaticResource", extension: ".resource", contentFile: true},
	{path: "synonymDictionaries", name: "SynonymDictionary", extension: ".synonymDictionary"},
	{path: "tabs", name: "CustomTab", extension: ".tab"},
	{path: "territory2Types", name: "Territory2Type", extension: ".territory2Type"},
	{path: "translations", name: "Translations", extension: ".translation"},
	{path: "triggers", name: "ApexTrigger", extension: ".trigger", contentFile: true},
	{path: "uiFormatSpecificationSets", name: "UiFormatSpecificationSet", extension: ".uiFormatSpecificationSet"},
	{path: "viewdefinitions", name: "ViewDefinition", extension: ".view", contentFile: true},
	{path: "weblinks", name: "CustomPageWebLink", extension: ".weblink"},
	{path: "workflows", name: "Workflow", extension: ".workflow"},
	{path: "cspTrustedSites", name: "CspTrustedSite", extension: ".cspTrustedSite"},
}

type PackageBuilder struct {
	IsPush   bool
	Metadata map[string]MetaType
	Files    ForceMetadataFiles
	Root     string
	// sourceComponents holds the source-format object and object translation
	// components added so far. Source format splits an object into a file per
	// component; metadata format needs them composed into one file.
	sourceComponents *repo.Repo
	// SourceFormat reports whether any file added was in source (sfdx)
	// format and had to be converted to metadata format.
	SourceFormat bool
	// nestedPaths maps the metadata format path of a file added from a
	// subdirectory of its type's directory to the path it was added from,
	// both relative to Root and slash-separated.
	nestedPaths map[string]string
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

	if converted, err := pb.addSourceFormatFile(fpath); converted || err != nil {
		pb.SourceFormat = pb.SourceFormat || converted
		return err
	}

	isFolderMetadata := isFolderMetadata(fpath)
	// Path with -meta.xml stripped
	spath := MetaPathToSourcePath(fpath)
	metaName, fname, err := pb.getMetaTypeForRelativePath(spath)
	if err != nil {
		return err
	}
	pb.recordNestedPath(spath)
	pb.recordNestedPath(spath + "-meta.xml")
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

// addSourceFormatFile adds a file written in source (sfdx) format, converting
// it to metadata format. It reports false for a file that is not in source
// format, which AddFile then adds as is.
//
// A source-format file is a -meta.xml file of a type that has no content
// file, so no file exists at its path without the -meta.xml suffix: an
// Apex class's Foo.cls-meta.xml is the same in both formats, while a custom
// metadata record's Type.Record.md-meta.xml is Type.Record.md in metadata
// format.
func (pb *PackageBuilder) addSourceFormatFile(fpath string) (bool, error) {
	if !strings.HasSuffix(fpath, "-meta.xml") {
		return false, nil
	}
	withoutMetaSuffix := strings.TrimSuffix(fpath, "-meta.xml")
	if _, err := os.Stat(withoutMetaSuffix); err == nil {
		return false, nil
	}
	relativePath, err := filepath.Rel(pb.Root, fpath)
	if err != nil || !filepath.IsLocal(relativePath) {
		return false, nil
	}
	parts := strings.Split(relativePath, string(os.PathSeparator))
	if len(parts) < 2 {
		return false, nil
	}
	mp, ok := metapathForDirectory(parts[0])
	if !ok || mp.onlyFolder {
		return false, nil
	}

	switch {
	case mp.hasFolder && len(parts) == 2:
		// A folder: reports/Sales.reportFolder-meta.xml is
		// reports/Sales-meta.xml in metadata format.
		base := filepath.Base(withoutMetaSuffix)
		folderName := strings.TrimSuffix(base, filepath.Ext(base))
		pb.AddMetaToPackage(mp.name, folderName)
		return true, pb.addConvertedFile(filepath.Join(parts[0], folderName+"-meta.xml"), fpath)
	case mp.contentFile:
		return false, nil
	case (mp.name == "CustomObject" || mp.name == "CustomObjectTranslation") && len(parts) >= 3:
		// objects/Account/Account.object-meta.xml or
		// objects/Account/fields/Name__c.field-meta.xml
		return true, pb.addDecomposedComponent(mp.name, parts, fpath)
	default:
		pb.recordNestedPath(withoutMetaSuffix)
		metadataPath := metadataFormatPath(strings.TrimSuffix(relativePath, "-meta.xml"))
		memberPath := filepath.Join(strings.Split(metadataPath, string(os.PathSeparator))[1:]...)
		pb.AddMetaToPackage(mp.name, strings.TrimSuffix(memberPath, filepath.Ext(memberPath)))
		return true, pb.addConvertedFile(metadataPath, fpath)
	}
}

// addDecomposedComponent adds one component of a source-format object or
// object translation and rewrites the parent's composed metadata-format file.
// parts is the component's path relative to the root: the parent's own file
// (objects/Account/Account.object-meta.xml) or a child component
// (objects/Account/fields/Name__c.field-meta.xml).
func (pb *PackageBuilder) addDecomposedComponent(parentType string, parts []string, fpath string) error {
	parentName := parts[1]
	if pb.sourceComponents == nil {
		pb.sourceComponents = repo.NewRepo()
	}
	opened, err := pb.sourceComponents.Open(fpath)
	if err != nil {
		return fmt.Errorf("Could not read %s: %w", fpath, err)
	}
	component, ok := opened.(metadata.RegisterableMetadata)
	if !ok {
		return fmt.Errorf("Could not read %s: unsupported metadata", fpath)
	}

	var composed metadata.FilesGenerator
	switch parentType {
	case "CustomObject":
		// A component of an object is deployed as its own type
		// (CustomField Account.Name__c) inside the object's file.
		if component.Type() == parentType {
			pb.AddMetaToPackage(parentType, parentName)
		} else {
			base := strings.TrimSuffix(filepath.Base(fpath), "-meta.xml")
			pb.AddMetaToPackage(component.Type(), parentName+"."+strings.TrimSuffix(base, filepath.Ext(base)))
		}
		composed = objects.ComposeFromChildren(parentName, pb.sourceComponents)
	case "CustomObjectTranslation":
		// Field, record type, and validation rule translations have no
		// metadata type of their own; the object translation is deployed.
		pb.AddMetaToPackage(parentType, parentName)
		composed = objectTranslations.ComposeFromChildren(parentName, pb.sourceComponents)
	default:
		return fmt.Errorf("Could not add %s: %s is not decomposed in source format", fpath, parentType)
	}

	files, err := composed.Files(metadata.MetadataFormat)
	if err != nil {
		return fmt.Errorf("Could not compose %s %s: %w", parentType, parentName, err)
	}
	if pb.IsPush {
		for relativePath, content := range files {
			pb.Files[filepath.FromSlash(relativePath)] = content
		}
	}
	return nil
}

// addConvertedFile adds the contents of the source-format file at fpath under
// its metadata-format path.
func (pb *PackageBuilder) addConvertedFile(metadataPath string, fpath string) error {
	if !pb.IsPush {
		return nil
	}
	fdata, err := ioutil.ReadFile(fpath)
	if err != nil {
		return errors.Wrap(err, "failed to add file")
	}
	pb.Files[metadataPath] = fdata
	return nil
}

// metadataFormatPath returns the path, relative to the root, that a file has
// in a metadata format zip. Source format projects can organize a type's files
// into subdirectories (classes/service/Foo.cls), but the Metadata API names
// those components without the subdirectories and expects them directly in the
// type's directory (classes/Foo.cls). Types whose subdirectories are part of
// the component are returned unchanged: foldered types and bundles, the
// decomposed files of objects and object translations, and the expanded
// contents of static resources.
func metadataFormatPath(relativePath string) string {
	parts := strings.Split(relativePath, string(os.PathSeparator))
	if len(parts) <= 2 {
		return relativePath
	}
	mp, ok := metapathForDirectory(parts[0])
	if !ok || mp.hasFolder || mp.onlyFolder {
		return relativePath
	}
	switch mp.name {
	case "CustomObject", "CustomObjectTranslation", "StaticResource":
		return relativePath
	}
	return filepath.Join(parts[0], parts[len(parts)-1])
}

// recordNestedPath remembers where a file in a subdirectory of its type's
// directory came from, so a retrieved copy can be written back to that path.
func (pb *PackageBuilder) recordNestedPath(fpath string) {
	relativePath, err := filepath.Rel(pb.Root, fpath)
	if err != nil {
		return
	}
	metadataPath := metadataFormatPath(relativePath)
	if metadataPath == relativePath {
		return
	}
	if pb.nestedPaths == nil {
		pb.nestedPaths = make(map[string]string)
	}
	pb.nestedPaths[filepath.ToSlash(metadataPath)] = filepath.ToSlash(relativePath)
}

// RestoreNestedPaths returns a RetrieveEntryHandler that passes each retrieved
// entry to write under the path it was requested by, so a class requested as
// classes/service/Foo.cls is written there rather than to classes/Foo.cls.
func (pb *PackageBuilder) RestoreNestedPaths(write RetrieveEntryHandler) RetrieveEntryHandler {
	return func(name string, r io.Reader) error {
		if nested, ok := pb.nestedPaths[name]; ok {
			name = nested
		}
		return write(name, r)
	}
}

func metapathForDirectory(directory string) (metapath, bool) {
	for _, mp := range metapaths {
		if mp.path == directory {
			return mp, true
		}
	}
	return metapath{}, false
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

	// Bundle metadata such as ExperienceBundle stores its -meta.xml file
	// alongside the bundle directory rather than inside it. Include it so the
	// bundle can be deployed by pushing only its directory.
	if isComponent {
		if m := bundleMetadata(fpath); m != "" {
			if err = pb.AddFile(m); err != nil {
				return fmt.Errorf("Failed to add metadata for bundle: %w", err)
			}
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
			if pb.isComponent(dirOrFilePath) && lwcJsTestDir.MatchString(dirOrFilePath) {
				// Normally malformed paths would indicate invalid metadata,
				// but LWC tests should never be deployed. We may want to consider this logic/behavior,
				// such that we don't call `addFile` on directories in some cases; if we could
				// avoid the addFile call on the __tests__ dir, we could avoid this check.
				// A __tests__ directory of another type, such as classes, holds
				// components that are deployed like any other.
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

// bundleMetadata returns the path to a bundle's -meta.xml file when it is a
// sibling of the bundle directory rather than a file inside it. ExperienceBundle
// stores its metadata file alongside the bundle directory, e.g.
// experiences/MySite.site-meta.xml for the experiences/MySite bundle.
func bundleMetadata(dir string) string {
	parent := filepath.Dir(dir)
	base := filepath.Base(dir)
	entries, err := ioutil.ReadDir(parent)
	if err != nil {
		return ""
	}
	for _, e := range entries {
		if e.IsDir() {
			continue
		}
		name := e.Name()
		if !strings.HasSuffix(name, "-meta.xml") {
			continue
		}
		trimmed := strings.TrimSuffix(name, "-meta.xml")
		trimmed = strings.TrimSuffix(trimmed, filepath.Ext(trimmed))
		if trimmed == base {
			return filepath.Join(parent, name)
		}
	}
	return ""
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
	pb.Files[metadataFormatPath(frel)] = fdata

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
	pb.Files[metadataFormatPath(fmetarel)] = fdata

	return nil
}

// e.g. add /path/to/src/destructiveChanges.xml to zip file as
// destructiveChanges.xml
func (pb *PackageBuilder) addFileOnly(fpath string) (err error) {
	fdata, err := ioutil.ReadFile(fpath)
	if err != nil {
		return err
	}

	frel, err := filepath.Rel(pb.Root, fpath)
	if err != nil {
		return err
	}
	if !filepath.IsLocal(frel) {
		frel = filepath.Base(frel)
	}
	pb.Files[frel] = fdata

	return err
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

	canonicalName := filepath.ToSlash(name)
	if !pb.contains(mt.Members, canonicalName) {
		mt.Members = append(mt.Members, canonicalName)
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

// IsMetadataDirectory reports whether name is the directory a metadata type's
// files live in, such as classes or cachePartitions.
func IsMetadataDirectory(name string) bool {
	for _, mp := range metapaths {
		if mp.path == name {
			return true
		}
	}
	return false
}

// MetadataDirectoryIndexBySuffix returns the index in parts, the components of
// a file's path, of the nearest directory whose metadata type has the file's
// suffix, or -1 if there is none. It finds classes in
// classes/triggers/Foo.cls-meta.xml and in reports/main/default/classes/Foo.cls.
func MetadataDirectoryIndexBySuffix(parts []string) int {
	if len(parts) == 0 {
		return -1
	}
	suffix := filepath.Ext(strings.TrimSuffix(parts[len(parts)-1], "-meta.xml"))
	if suffix == "" {
		return -1
	}
	for i := len(parts) - 2; i >= 0; i-- {
		mp, ok := metapathForDirectory(parts[i])
		if ok && (suffix == mp.extension || suffix == folderSuffixes[mp.path]) {
			return i
		}
	}
	return -1
}

// MetadataDirectoryIndex returns the index in parts, the components of a path,
// of the directory of the metadata type the path belongs to, or -1 if no
// component names a metadata type. Directories can have the name of another
// metadata type above or below the type's directory, so it uses the first of
// these that applies:
//
//  1. The nearest directory whose type has the path's file suffix.
//  2. The outermost directory of a type whose files have arbitrary names:
//     lwc/settings/settings.js is in lwc, and
//     staticresources/app/pages/index.html is in staticresources.
//  3. The nearest directory that names a metadata type, which is objects in
//     objects/Account/fields/Name__c.field-meta.xml.
func MetadataDirectoryIndex(parts []string) int {
	if i := MetadataDirectoryIndexBySuffix(parts); i >= 0 {
		return i
	}
	nearest := -1
	for i, part := range parts {
		mp, ok := metapathForDirectory(part)
		if !ok {
			continue
		}
		if mp.onlyFolder || mp.name == "StaticResource" || mp.name == "Document" {
			return i
		}
		nearest = i
	}
	return nearest
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
		metadataParts := strings.Split(metadataFormatPath(relativePath), string(os.PathSeparator))
		objectName = strings.TrimSuffix(strings.Join(metadataParts[1:], string(os.PathSeparator)), filepath.Ext(path))
		if pb.isComponent(path) {
			objectName = parts[1]
			// Bundle metadata files (e.g. ExperienceBundle's
			// experiences/MySite.site-meta.xml) live alongside the bundle
			// directory. Reduce the file name to the bundle name.
			if strings.HasSuffix(objectName, "-meta.xml") {
				objectName = strings.TrimSuffix(objectName, "-meta.xml")
				objectName = strings.TrimSuffix(objectName, filepath.Ext(objectName))
			}
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

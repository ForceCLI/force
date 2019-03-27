package command

import (
	"fmt"
	"io/ioutil"
	"os"
	"path/filepath"
	"sort"
	"strings"

	"github.com/ForceCLI/force/config"
	. "github.com/ForceCLI/force/error"
	. "github.com/ForceCLI/force/lib"
)

var cmdExport = &Command{
	Run:   runExport,
	Usage: "export [options] [dir]",
	Short: "Export metadata to a local directory",
	Long: `
Export metadata to a local directory

Export Options
  -w, -warnings  # Display warnings about metadata that cannot be retrieved
  -x, -exclude   # Exclude given metadata type

Examples:

  force export

  force export org/schema

  force export -x ApexClass -x CustomObject
`,
}

type metadataList []string

func (i *metadataList) String() string {
	return fmt.Sprint(*i)
}

func (i *metadataList) Set(value string) error {
	*i = append(*i, value)
	return nil
}

var (
	showWarnings         bool
	excludeMetadataNames metadataList
)

func init() {
	cmdExport.Flag.BoolVar(&showWarnings, "w", false, "show warnings")
	cmdExport.Flag.BoolVar(&showWarnings, "warnings", false, "show warnings")
	cmdExport.Flag.Var(&excludeMetadataNames, "x", "exclude metadata type")
	cmdExport.Flag.Var(&excludeMetadataNames, "exclude", "exclude metadata type")
}

func runExport(cmd *Command, args []string) {
	// Get path from args if available
	var err error
	var root string
	if len(args) == 1 {
		root, err = filepath.Abs(args[0])
	}
	if err != nil {
		fmt.Printf("Error obtaining file path\n")
		ErrorAndExit(err.Error())
	}
	force, _ := ActiveForce()
	sobjects, err := force.ListSobjects()
	if err != nil {
		ErrorAndExit(err.Error())
	}
	query := make(ForceMetadataQuery, 0)
	customObject := "CustomObject"

	sort.Strings(excludeMetadataNames)

	if !isExcluded(customObject) {
		stdObjects := make([]string, 1, len(sobjects)+1)
		stdObjects[0] = "*"
		for _, sobject := range sobjects {
			name := sobject["name"].(string)
			if !sobject["custom"].(bool) && !strings.HasSuffix(name, "__Tag") && !strings.HasSuffix(name, "__History") && !strings.HasSuffix(name, "__Share") {
				stdObjects = append(stdObjects, name)
			}
		}
		stdObjects = append(stdObjects, "Activity")

		query = append(query, ForceMetadataQueryElement{Name: []string{customObject}, Members: stdObjects})
	}

	metadataNames := []string{"AccountSettings",
		"ActivitiesSettings",
		"AddressSettings",
		"AnalyticSnapshot",
		"ApexClass",
		"ApexComponent",
		"ApexPage",
		"ApexTrigger",
		"ApprovalProcess",
		"AssignmentRules",
		"Audience",
		"AuraDefinitionBundle",
		"AuthProvider",
		"AutoResponseRules",
		"BusinessHoursSettings",
		"BusinessProcess",
		"CallCenter",
		"CaseSettings",
		"ChatterAnswersSettings",
		"CompanySettings",
		"Community",
		"CompactLayout",
		"ConnectedApp",
		"ContentAsset",
		"ContractSettings",
		"CustomApplication",
		"CustomApplicationComponent",
		"CustomField",
		"CustomLabels",
		"CustomMetadata",
		"CustomObjectTranslation",
		"CustomPageWebLink",
		"CustomPermission",
		"CustomSite",
		"CustomTab",
		"DataCategoryGroup",
		"DuplicateRule",
		"EntitlementProcess",
		"EntitlementSettings",
		"EntitlementTemplate",
		"ExternalDataSource",
		"FieldSet",
		"FlexiPage",
		"Flow",
		"FlowDefinition",
		"Folder",
		"ForecastingSettings",
		"Group",
		"HomePageComponent",
		"HomePageLayout",
		"IdeasSettings",
		"KnowledgeSettings",
		"Layout",
		"Letterhead",
		"ListView",
		"LiveAgentSettings",
		"LiveChatAgentConfig",
		"LiveChatButton",
		"LiveChatDeployment",
		"MatchingRules",
		"MilestoneType",
		"MobileSettings",
		"NamedFilter",
		"Network",
		"OpportunitySettings",
		"PermissionSet",
		"Portal",
		"PostTemplate",
		"ProductSettings",
		"Profile",
		"ProfileSessionSetting",
		"Queue",
		"QuickAction",
		"QuoteSettings",
		"RecordType",
		"RemoteSiteSetting",
		"ReportType",
		"Role",
		"SamlSsoConfig",
		"Scontrol",
		"SecuritySettings",
		"SharingReason",
		"SharingRules",
		"Skill",
		"StaticResource",
		"Territory",
		"Translations",
		"ValidationRule",
		"Workflow",
	}

	for _, name := range metadataNames {
		if !isExcluded(name) {
			query = append(query, ForceMetadataQueryElement{Name: []string{name}, Members: []string{"*"}})
		}
	}

	folders, err := force.GetAllFolders()
	if err != nil {
		err = fmt.Errorf("Could not get folders: %s", err.Error())
		ErrorAndExit(err.Error())
	}
	for foldersType, foldersName := range folders {
		if foldersType == "Email" {
			foldersType = "EmailTemplate"
		}
		members, err := force.GetMetadataInFolders(foldersType, foldersName)
		if err != nil {
			err = fmt.Errorf("Could not get metadata in folders: %s", err.Error())
			ErrorAndExit(err.Error())
		}

		if !isExcluded(string(foldersType)) {
			query = append(query, ForceMetadataQueryElement{Name: []string{string(foldersType)}, Members: members})
		}
	}

	if root == "" {
		root, err = config.GetSourceDir()
		if err != nil {
			fmt.Printf("Error obtaining root directory\n")
			ErrorAndExit(err.Error())
		}
	}

	files, problems, err := force.Metadata.Retrieve(query)
	if err != nil {
		fmt.Printf("Encountered and error with retrieve...\n")
		ErrorAndExit(err.Error())
	}
	if showWarnings {
		for _, problem := range problems {
			fmt.Fprintln(os.Stderr, problem)
		}
	}
	for name, data := range files {
		file := filepath.Join(root, name)
		dir := filepath.Dir(file)
		if err := os.MkdirAll(dir, 0755); err != nil {
			ErrorAndExit(err.Error())
		}
		if err := ioutil.WriteFile(filepath.Join(root, name), data, 0644); err != nil {
			ErrorAndExit(err.Error())
		}
	}
	fmt.Printf("Exported to %s\n", root)
}

func isExcluded(name string) bool {
	index := sort.SearchStrings(excludeMetadataNames, name)

	return index < len(excludeMetadataNames) && excludeMetadataNames[index] == name
}

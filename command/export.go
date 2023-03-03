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
	"github.com/spf13/cobra"
)

func init() {
	exportCmd.Flags().BoolP("warnings", "w", false, "display warnings about metadata that cannot be retrieved")
	exportCmd.Flags().StringSliceP("exclude", "x", []string{}, "exclude metadata type")

	RootCmd.AddCommand(exportCmd)
}

var exportCmd = &cobra.Command{
	Use:   "export [dir]",
	Short: "Export metadata to a local directory",
	Example: `
  force export
  force export org/schema
  force export -x ApexClass -x CustomObject
`,
	Args: cobra.MaximumNArgs(1),
	Run: func(cmd *cobra.Command, args []string) {
		var root string
		var err error
		if len(args) == 1 {
			root, err = filepath.Abs(args[0])
			if err != nil {
				fmt.Printf("Error obtaining file path\n")
				ErrorAndExit(err.Error())
			}
		} else {
			root, err = config.GetSourceDir()
			if err != nil {
				fmt.Printf("Error obtaining root directory\n")
				ErrorAndExit(err.Error())
			}
		}
		excludeMetadataNames, _ := cmd.Flags().GetStringSlice("exclude")
		showWarnings, _ := cmd.Flags().GetBool("warnings")
		runExport(root, excludeMetadataNames, showWarnings)
	},
}

func runExport(root string, excludeMetadataNames []string, showWarnings bool) {
	sobjects, err := force.ListSobjects()
	if err != nil {
		ErrorAndExit(err.Error())
	}
	query := make(ForceMetadataQuery, 0)
	customObject := "CustomObject"

	sort.Strings(excludeMetadataNames)

	if !isExcluded(excludeMetadataNames, customObject) {
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
		"CustomHelpMenuSection",
		"CustomLabels",
		"CustomMetadata",
		"CustomNotificationType",
		"CustomObjectTranslation",
		"CustomPageWebLink",
		"CustomPermission",
		"CustomSite",
		"CustomTab",
		"DataCategoryGroup",
		"DataWeaveResource",
		"DuplicateRule",
		"EntitlementProcess",
		"EntitlementSettings",
		"EntitlementTemplate",
		"ExperienceBundle",
		"ExternalDataSource",
		"FieldSet",
		"FlexiPage",
		"Flow",
		"FlowDefinition",
		"Folder",
		"ForecastingSettings",
		"GlobalValueSet",
		"Group",
		"HomePageComponent",
		"HomePageLayout",
		"IdeasSettings",
		"KnowledgeSettings",
		"Layout",
		"Letterhead",
		"LightningComponentBundle",
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
		"PermissionSetGroup",
		"PlatformEventChannel",
		"PlatformEventChannelMember",
		"PlatformEventSubscriberConfig",
		"Portal",
		"PostTemplate",
		"ProductSettings",
		"Profile",
		"ProfileSessionSetting",
		"Queue",
		"QuickAction",
		"QuoteSettings",
		"RecordType",
		"RestrictionRule",
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
		if !isExcluded(excludeMetadataNames, name) {
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

		if !isExcluded(excludeMetadataNames, string(foldersType)) {
			query = append(query, ForceMetadataQueryElement{Name: []string{string(foldersType)}, Members: members})
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

func isExcluded(excludeMetadataNames []string, name string) bool {
	index := sort.SearchStrings(excludeMetadataNames, name)

	return index < len(excludeMetadataNames) && excludeMetadataNames[index] == name
}

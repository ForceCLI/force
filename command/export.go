package command

import (
	"fmt"
	"io/ioutil"
	"os"
	"path/filepath"
	"strings"

	"github.com/heroku/force/config"
	. "github.com/heroku/force/error"
	. "github.com/heroku/force/lib"
)

var cmdExport = &Command{
	Run:   runExport,
	Usage: "export [dir]",
	Short: "Export metadata to a local directory",
	Long: `
Export metadata to a local directory

Examples:

  force export

  force export org/schema
`,
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
	stdObjects := make([]string, 1, len(sobjects)+1)
	stdObjects[0] = "*"
	for _, sobject := range sobjects {
		name := sobject["name"].(string)
		if !sobject["custom"].(bool) && !strings.HasSuffix(name, "__Tag") && !strings.HasSuffix(name, "__History") && !strings.HasSuffix(name, "__Share") {
			stdObjects = append(stdObjects, name)
		}
	}
	query := ForceMetadataQuery{
		{Name: []string{"AccountSettings"}, Members: []string{"*"}},
		{Name: []string{"ActivitiesSettings"}, Members: []string{"*"}},
		{Name: []string{"AddressSettings"}, Members: []string{"*"}},
		{Name: []string{"AnalyticSnapshot"}, Members: []string{"*"}},
		{Name: []string{"ApexClass"}, Members: []string{"*"}},
		{Name: []string{"ApexComponent"}, Members: []string{"*"}},
		{Name: []string{"ApexPage"}, Members: []string{"*"}},
		{Name: []string{"ApexTrigger"}, Members: []string{"*"}},
		{Name: []string{"ApprovalProcess"}, Members: []string{"*"}},
		{Name: []string{"AssignmentRules"}, Members: []string{"*"}},
		{Name: []string{"AuraDefinitionBundle"}, Members: []string{"*"}},
		{Name: []string{"AuthProvider"}, Members: []string{"*"}},
		{Name: []string{"AutoResponseRules"}, Members: []string{"*"}},
		{Name: []string{"BusinessHoursSettings"}, Members: []string{"*"}},
		{Name: []string{"BusinessProcess"}, Members: []string{"*"}},
		{Name: []string{"CallCenter"}, Members: []string{"*"}},
		{Name: []string{"CaseSettings"}, Members: []string{"*"}},
		{Name: []string{"ChatterAnswersSettings"}, Members: []string{"*"}},
		{Name: []string{"CompanySettings"}, Members: []string{"*"}},
		{Name: []string{"Community"}, Members: []string{"*"}},
		{Name: []string{"CompactLayout"}, Members: []string{"*"}},
		{Name: []string{"ConnectedApp"}, Members: []string{"*"}},
		{Name: []string{"ContractSettings"}, Members: []string{"*"}},
		{Name: []string{"CustomApplication"}, Members: []string{"*"}},
		{Name: []string{"CustomApplicationComponent"}, Members: []string{"*"}},
		{Name: []string{"CustomField"}, Members: []string{"*"}},
		{Name: []string{"CustomLabels"}, Members: []string{"*"}},
		{Name: []string{"CustomMetadata"}, Members: []string{"*"}},
		{Name: []string{"CustomObject"}, Members: stdObjects},
		{Name: []string{"CustomObjectTranslation"}, Members: []string{"*"}},
		{Name: []string{"CustomPageWebLink"}, Members: []string{"*"}},
		{Name: []string{"CustomPermission"}, Members: []string{"*"}},
		{Name: []string{"CustomSite"}, Members: []string{"*"}},
		{Name: []string{"CustomTab"}, Members: []string{"*"}},
		{Name: []string{"DataCategoryGroup"}, Members: []string{"*"}},
		{Name: []string{"EntitlementProcess"}, Members: []string{"*"}},
		{Name: []string{"EntitlementSettings"}, Members: []string{"*"}},
		{Name: []string{"EntitlementTemplate"}, Members: []string{"*"}},
		{Name: []string{"ExternalDataSource"}, Members: []string{"*"}},
		{Name: []string{"FieldSet"}, Members: []string{"*"}},
		{Name: []string{"Flow"}, Members: []string{"*"}},
		{Name: []string{"Folder"}, Members: []string{"*"}},
		{Name: []string{"ForecastingSettings"}, Members: []string{"*"}},
		{Name: []string{"Group"}, Members: []string{"*"}},
		{Name: []string{"HomePageComponent"}, Members: []string{"*"}},
		{Name: []string{"HomePageLayout"}, Members: []string{"*"}},
		{Name: []string{"IdeasSettings"}, Members: []string{"*"}},
		{Name: []string{"KnowledgeSettings"}, Members: []string{"*"}},
		{Name: []string{"Layout"}, Members: []string{"*"}},
		{Name: []string{"Letterhead"}, Members: []string{"*"}},
		{Name: []string{"ListView"}, Members: []string{"*"}},
		{Name: []string{"LiveAgentSettings"}, Members: []string{"*"}},
		{Name: []string{"LiveChatAgentConfig"}, Members: []string{"*"}},
		{Name: []string{"LiveChatButton"}, Members: []string{"*"}},
		{Name: []string{"LiveChatDeployment"}, Members: []string{"*"}},
		{Name: []string{"MilestoneType"}, Members: []string{"*"}},
		{Name: []string{"MobileSettings"}, Members: []string{"*"}},
		{Name: []string{"NamedFilter"}, Members: []string{"*"}},
		{Name: []string{"Network"}, Members: []string{"*"}},
		{Name: []string{"OpportunitySettings"}, Members: []string{"*"}},
		{Name: []string{"PermissionSet"}, Members: []string{"*"}},
		{Name: []string{"Portal"}, Members: []string{"*"}},
		{Name: []string{"PostTemplate"}, Members: []string{"*"}},
		{Name: []string{"ProductSettings"}, Members: []string{"*"}},
		{Name: []string{"Profile"}, Members: []string{"*"}},
		{Name: []string{"Queue"}, Members: []string{"*"}},
		{Name: []string{"QuickAction"}, Members: []string{"*"}},
		{Name: []string{"QuoteSettings"}, Members: []string{"*"}},
		{Name: []string{"RecordType"}, Members: []string{"*"}},
		{Name: []string{"RemoteSiteSetting"}, Members: []string{"*"}},
		{Name: []string{"ReportType"}, Members: []string{"*"}},
		{Name: []string{"Role"}, Members: []string{"*"}},
		{Name: []string{"SamlSsoConfig"}, Members: []string{"*"}},
		{Name: []string{"Scontrol"}, Members: []string{"*"}},
		{Name: []string{"SecuritySettings"}, Members: []string{"*"}},
		{Name: []string{"SharingReason"}, Members: []string{"*"}},
		{Name: []string{"SharingRules"}, Members: []string{"*"}},
		{Name: []string{"Skill"}, Members: []string{"*"}},
		{Name: []string{"StaticResource"}, Members: []string{"*"}},
		{Name: []string{"Territory"}, Members: []string{"*"}},
		{Name: []string{"Translations"}, Members: []string{"*"}},
		{Name: []string{"ValidationRule"}, Members: []string{"*"}},
		{Name: []string{"Workflow"}, Members: []string{"*"}},
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
		query = append(query, ForceMetadataQueryElement{Name: []string{string(foldersType)}, Members: members})
	}

	if root == "" {
		root, err = config.GetSourceDir()
		if err != nil {
			fmt.Printf("Error obtaining root directory\n")
			ErrorAndExit(err.Error())
		}
	}
	files, err := force.Metadata.Retrieve(query)
	if err != nil {
		fmt.Printf("Encountered and error with retrieve...\n")
		ErrorAndExit(err.Error())
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

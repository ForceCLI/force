package main

import (
	"fmt"
	"io/ioutil"
	"os"
	"path/filepath"
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
  
  force export ?
  
  force export [Metadata Type] [Name]
  
  force export CustomObject Case
`,
}

func runExport(cmd *Command, args []string) {
	wd, _ := os.Getwd()
	root := filepath.Join(wd, "metadata")
	
	typeMembers := []string{
		"AccountSettings",
		"ActivitiesSettings",
		"AddressSettings",
		"AnalyticSnapshot",
		"ApexClass",
		"ApexComponent",
		"ApexPage",
		"ApexTrigger",
		"ApprovalProcess",
		"AssignmentRules",
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
		"ContractSettings",
		"CustomApplication",
		"CustomApplicationComponent",
		"CustomApplication",
		"CustomField",
		"CustomLabels",
		"CustomObject",
		"CustomObjectTranslation",
		"CustomPageWebLink",
		"CustomSite",
		"CustomTab",
		"Dashboard",
		"DataCategoryGroup",
		"Document",
		"EmailTemplate",
		"EntitlementProcess",
		"EntitlementSettings",
		"EntitlementTemplate",
		"ExternalDataSource",
		"FieldSet",
		"Flow",
		"ForecastingSettings",
		"Group",
		"HomePageComponent",
		"HomePageLayout",
		"IdeasSettings",
		"KnowledgeSettings",
		"Letterhead",
		"ListView",
		"LiveAgentSettings",
		"LiveChatAgentConfig",
		"LiveChatButton",
		"LiveChatDeployment",
		"MilestoneType",
		"MobileSettings",
		"NamedFilter",
		"Network",
		"OpportunitySettings",
		"PermissionSet",
		"Portal",
		"PostTemplate",
		"ProductSettings",
		"Queue",
		"QuickAction",
		"QuoteSettings",
		"RecordType",
		"RemoteSiteSetting",
		"Report",
		"ReportType",
		"Role",
		"SamlSsoConfig",
		"Scontrol",
		"SecuritySettings",
		"SharingReason",
		"Skill",
		"StaticResource",
		"Territory",
		"Translations",
		"ValidationRule",
	}
	
	if len(args) == 1 {
		if (args[0] == "?") {
			fmt.Printf("Available types: %v\n", typeMembers)
			return
		}
		root, _ = filepath.Abs(args[0])
	}
	
	var query ForceMetadataQuery
	
	if len(args) == 2 {
		query = ForceMetadataQuery{
			{Name: "Profile", Members: "*"},
			{Name: args[0], Members: args[1]},
		}
	} else {
		query = make([]ForceMetadataQueryElement, len(typeMembers))
		
		for i := 0; i < len(typeMembers); i++ {
			query[i] = ForceMetadataQueryElement{Name: typeMembers[i], Members: "*"}
		}
	}
	
	force, _ := ActiveForce()
	
	files, err := force.Metadata.Retrieve(query)
	if err != nil {
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

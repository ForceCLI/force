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
`,
}

func runExport(cmd *Command, args []string) {
	wd, _ := os.Getwd()
	root := filepath.Join(wd, "metadata")
	if len(args) == 1 {
		root, _ = filepath.Abs(args[0])
	}
	runReportExport(root)
	runMetadataExport(root)
}

func runReportExport(root string) {
	force, _ := ActiveForce()
	// Get List of Folders
	folders := map[string]string{} // Map ID to DeveloperName
	folder_query := "SELECT Id,DeveloperName FROM Folder WHERE Type = 'Report' and DeveloperName != ''"
	folder_ids := ""
	folder_records, err := force.Query(folder_query)
	if err != nil {
		ErrorAndExit(err.Error())
	}
	for _, record := range folder_records {
		if record != nil {
			folders[record["Id"].(string)] = record["DeveloperName"].(string)
			if len(folder_ids) != 0 {
				folder_ids = folder_ids + ","
			}
			folder_ids = folder_ids + "'" + record["Id"].(string) + "'"
		}
	}

	// Get reports in each folder
	report_query := fmt.Sprintf("SELECT Id,DeveloperName,OwnerId FROM Report WHERE OwnerID IN (%s)", folder_ids)
	report_records, err := force.Query(report_query)
	if err != nil {
		ErrorAndExit(err.Error())
	}

	// Create ForceMetadataQuery Object with every folder / report
	forceObjectsToRetrieve := ForceMetadataQuery{}
	for _, record := range report_records {
		tmpForceMetadataQueryElement := ForceMetadataQueryElement{
			Name: "Report",
			Members: fmt.Sprintf(
				"%s/%s",
				folders[record["OwnerId"].(string)], /* folder dev name */
				record["DeveloperName"].(string),    /* report dev name */
			),
		}
		forceObjectsToRetrieve = append(forceObjectsToRetrieve, tmpForceMetadataQueryElement)
	}
	files, err := force.Metadata.Retrieve(forceObjectsToRetrieve)
	if err != nil {
		ErrorAndExit(err.Error())
	}
	saveFiles(files, root)
}

func runMetadataExport(root string) {
	force, _ := ActiveForce()
	defaultObjects := ForceMetadataQuery{
		{Name: "AccountSettings", Members: "*"},
		{Name: "ActivitiesSettings", Members: "*"},
		{Name: "AddressSettings", Members: "*"},
		{Name: "AnalyticSnapshot", Members: "*"},
		{Name: "ApexClass", Members: "*"},
		{Name: "ApexComponent", Members: "*"},
		{Name: "ApexPage", Members: "*"},
		{Name: "ApexTrigger", Members: "*"},
		{Name: "ApprovalProcess", Members: "*"},
		{Name: "AssignmentRules", Members: "*"},
		{Name: "AuthProvider", Members: "*"},
		{Name: "AutoResponseRules", Members: "*"},
		{Name: "BusinessHoursSettings", Members: "*"},
		{Name: "BusinessProcess", Members: "*"},
		{Name: "CallCenter", Members: "*"},
		{Name: "CaseSettings", Members: "*"},
		{Name: "ChatterAnswersSettings", Members: "*"},
		{Name: "CompanySettings", Members: "*"},
		{Name: "Community", Members: "*"},
		{Name: "CompactLayout", Members: "*"},
		{Name: "ConnectedApp", Members: "*"},
		{Name: "ContractSettings", Members: "*"},
		{Name: "CustomApplication", Members: "*"},
		{Name: "CustomApplicationComponent", Members: "*"},
		{Name: "CustomApplication", Members: "*"},
		{Name: "CustomField", Members: "*"},
		{Name: "CustomLabels", Members: "*"},
		{Name: "CustomObject", Members: "*"},
		{Name: "CustomObjectTranslation", Members: "*"},
		{Name: "CustomPageWebLink", Members: "*"},
		{Name: "CustomSite", Members: "*"},
		{Name: "CustomTab", Members: "*"},
		{Name: "Dashboard", Members: "*"},
		{Name: "DataCategoryGroup", Members: "*"},
		{Name: "Document", Members: "*"},
		{Name: "EmailTemplate", Members: "*"},
		{Name: "EntitlementProcess", Members: "*"},
		{Name: "EntitlementSettings", Members: "*"},
		{Name: "EntitlementTemplate", Members: "*"},
		{Name: "ExternalDataSource", Members: "*"},
		{Name: "FieldSet", Members: "*"},
		{Name: "Flow", Members: "*"},
		{Name: "ForecastingSettings", Members: "*"},
		{Name: "Group", Members: "*"},
		{Name: "HomePageComponent", Members: "*"},
		{Name: "HomePageLayout", Members: "*"},
		{Name: "IdeasSettings", Members: "*"},
		{Name: "KnowledgeSettings", Members: "*"},
		{Name: "Letterhead", Members: "*"},
		{Name: "ListView", Members: "*"},
		{Name: "LiveAgentSettings", Members: "*"},
		{Name: "LiveChatAgentConfig", Members: "*"},
		{Name: "LiveChatButton", Members: "*"},
		{Name: "LiveChatDeployment", Members: "*"},
		{Name: "MilestoneType", Members: "*"},
		{Name: "MobileSettings", Members: "*"},
		{Name: "NamedFilter", Members: "*"},
		{Name: "Network", Members: "*"},
		{Name: "OpportunitySettings", Members: "*"},
		{Name: "PermissionSet", Members: "*"},
		{Name: "Portal", Members: "*"},
		{Name: "PostTemplate", Members: "*"},
		{Name: "ProductSettings", Members: "*"},
		{Name: "Queue", Members: "*"},
		{Name: "QuickAction", Members: "*"},
		{Name: "QuoteSettings", Members: "*"},
		{Name: "RecordType", Members: "*"},
		{Name: "RemoteSiteSetting", Members: "*"},
		{Name: "Report", Members: "*"},
		{Name: "ReportType", Members: "*"},
		{Name: "Role", Members: "*"},
		{Name: "SamlSsoConfig", Members: "*"},
		{Name: "Scontrol", Members: "*"},
		{Name: "SecuritySettings", Members: "*"},
		{Name: "SharingReason", Members: "*"},
		{Name: "Skill", Members: "*"},
		{Name: "StaticResource", Members: "*"},
		{Name: "Territory", Members: "*"},
		{Name: "Translations", Members: "*"},
		{Name: "ValidationRule", Members: "*"},
	}

	files, err := force.Metadata.Retrieve(defaultObjects)
	if err != nil {
		ErrorAndExit(err.Error())
	}
	saveFiles(files, root)
}

func saveFiles(files ForceMetadataFiles, root string) {
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

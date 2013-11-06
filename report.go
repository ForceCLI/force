package main

import (
	"fmt"
	"io/ioutil"
	"os"
	"path/filepath"
)

var cmdReport = &Command{
    Run:   runReport,
    Usage: "report <command> [<args>]",
    Short: "Export reports",
    Long: `
Export reports

Usage:

  force report export <path>

Examples:

  force record export org/reports
`,
}

func runReport(cmd *Command, args []string) {
    if len(args) == 0 {
        cmd.printUsage()
    } else {
        switch args[0] {
        case "export":
            cmdExportReports(args[1:])
        default:
            ErrorAndExit("no such command: %s", args[0])
        }
    }
}

func cmdExportReports(args []string) {
	if len(args) != 1 {
		ErrorAndExit("Path is required")
	}
	root, _ := filepath.Abs(args[0])

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
	report_query := fmt.Sprintf("SELECT Id,DeveloperName,OwnerId FROM Report WHERE OwnerID IN (%s) LIMIT 1", folder_ids)
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
	fmt.Printf("Exported %v reports to %s\n", len(files) - 1, root)
}

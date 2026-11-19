package sf

import (
	"fmt"
	"io"
	"strings"
	"time"

	"github.com/spf13/cobra"
)

// The package push-upgrade commands follow the sf plugin source.  Only the
// output of the list command for a package without push upgrade requests
// was compared with sf.

func newPackagePushUpgradeCmd() *cobra.Command {
	return newGroup("push-upgrade", "Commands to manage package push upgrades.",
		newPackagePushUpgradeAbortCmd(),
		newPackagePushUpgradeListCmd(),
		newPackagePushUpgradeReportCmd(),
		newPackagePushUpgradeScheduleCmd(),
	)
}

// restRecords runs a SOQL query with the REST API and returns the records.
func restRecords(org *targetOrg, soql string) ([]*orderedMap, error) {
	result, err := runQuery(org, "", "query", soql)
	if err != nil {
		return nil, apiError(err)
	}
	items, _ := result.values["records"].([]interface{})
	records := make([]*orderedMap, 0, len(items))
	for _, item := range items {
		if record, ok := item.(*orderedMap); ok {
			records = append(records, record)
		}
	}
	return records, nil
}

// pushJobCount counts the jobs of a push upgrade request, all of them or
// those with the given status.  The result is nil when the query returns no
// row.
func pushJobCount(org *targetOrg, requestID, status string) (interface{}, error) {
	soql := fmt.Sprintf("SELECT Count(Id) FROM PackagePushJob WHERE PackagePushRequestId = '%s'", requestID)
	if status != "" {
		soql += fmt.Sprintf(" AND Status = '%s'", status)
	}
	records, err := restRecords(org, soql+" ")
	if err != nil || len(records) == 0 {
		return nil, err
	}
	return records[0].values["expr0"], nil
}

func pushCell(value interface{}) string {
	if value == nil {
		return ""
	}
	return displayValue(value)
}

func newPackagePushUpgradeListCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:         "list",
		Short:       "Lists the status of push upgrade requests for a given package.",
		Args:        cobra.NoArgs,
		Annotations: map[string]string{commandNameAnnotation: "PackagePushRequestListCommand"},
		RunE: func(cmd *cobra.Command, args []string) error {
			packageID, _ := cmd.Flags().GetString("package")
			if packageID == "" {
				return missingFlagError("package")
			}
			status, _ := cmd.Flags().GetString("status")
			switch status {
			case "", "Created", "Cancelled", "Pending", "In Progress", "Failed", "Succeeded":
			default:
				return &Error{Message: fmt.Sprintf("Expected --status=%s to be one of: Created, Cancelled, Pending, In Progress, Failed, Succeeded\nSee more help with --help", status), ExitCode: 1}
			}
			days, _ := cmd.Flags().GetInt("scheduled-last-days")
			if cmd.Flags().Changed("scheduled-last-days") && days <= 0 {
				return &Error{Message: "Invalid value for --scheduled-last-days. It must be a positive integer.", ExitCode: 1}
			}
			hub, err := resolveTargetDevHub(cmd)
			if err != nil {
				return err
			}
			conditions := []string{fmt.Sprintf("PackageVersion.MetadataPackageId = '%s'", packageID)}
			if status != "" {
				conditions = append(conditions, fmt.Sprintf("Status = '%s'", status))
			}
			if days > 0 {
				conditions = append(conditions, "ScheduledStartTime >= "+time.Now().AddDate(0, 0, -days).UTC().Format("2006-01-02T15:04:05.000Z"))
			}
			if migrations, _ := cmd.Flags().GetBool("show-push-migrations-only"); migrations {
				conditions = append(conditions, "IsMigration = true")
			}
			records, err := restRecords(hub, "SELECT Id, PackageVersionId, PackageVersion.Name, PackageVersion.MajorVersion, PackageVersion.MinorVersion, Status, ScheduledStartTime, StartTime, EndTime, IsMigration FROM PackagePushRequest WHERE "+strings.Join(conditions, " AND "))
			if err != nil {
				return err
			}
			if len(records) == 0 {
				warn(cmd, "No results found")
				return emit(cmd, records, nil)
			}
			rows := make([][]string, 0, len(records))
			for _, record := range records {
				id := text(record.values["Id"])
				total, err := pushJobCount(hub, id, "")
				if err != nil {
					return err
				}
				failed, err := pushJobCount(hub, id, "Failed")
				if err != nil {
					return err
				}
				succeeded, err := pushJobCount(hub, id, "Succeeded")
				if err != nil {
					return err
				}
				name, number := "", ""
				if version, ok := record.values["PackageVersion"].(*orderedMap); ok {
					name = pushCell(version.values["Name"])
					if version.values["MajorVersion"] != nil && version.values["MinorVersion"] != nil {
						number = displayValue(version.values["MajorVersion"]) + "." + displayValue(version.values["MinorVersion"])
					}
				}
				rows = append(rows, []string{
					id, pushCell(record.values["PackageVersionId"]), name, number, pushCell(record.values["Status"]),
					pushCell(record.values["ScheduledStartTime"]), pushCell(record.values["StartTime"]), pushCell(record.values["EndTime"]),
					pushCell(total), pushCell(failed), pushCell(succeeded),
				})
			}
			return emit(cmd, records, func(w io.Writer) {
				printTable(w, "", []string{"Id", "Package Version Id", "Package Version Name", "Package Version Number", "Status", "Scheduled Start Time",
					"Start Time", "End Time", "Num Orgs Scheduled", "Num Orgs Upgraded Fail", "Num Orgs Upgraded Success"}, rows)
			})
		},
	}
	addJSONFlag(cmd)
	addAPIVersionFlag(cmd)
	cmd.Flags().StringP("package", "p", "", "Package ID (starts with 033) of the package that you want push upgrade information for.")
	cmd.Flags().IntP("scheduled-last-days", "l", 0, "Number of days in the past for which to display the list of push upgrade requests that were scheduled.")
	cmd.Flags().StringP("status", "s", "", "Status used to filter the list output Valid values are: Created, Canceled, Pending, In Progress, Failed, or Succeeded")
	cmd.Flags().Bool("show-push-migrations-only", false, "Display only push upgrade requests for package migrations.")
	addTargetDevHubFlag(cmd)
	return cmd
}

// pushErrorLimit is the number of job errors the report shows.
const pushErrorLimit = 12

func newPackagePushUpgradeReportCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:         "report",
		Short:       "Retrieve the status of a package push upgrade.",
		Args:        cobra.NoArgs,
		Annotations: map[string]string{commandNameAnnotation: "PackagePushUpgradeReportCommand"},
		RunE: func(cmd *cobra.Command, args []string) error {
			id, _ := cmd.Flags().GetString("push-request-id")
			if id == "" {
				return missingFlagError("push-request-id")
			}
			if err := salesforceIDFlag("push-request-id", id, "", true); err != nil {
				return err
			}
			hub, err := resolveTargetDevHub(cmd)
			if err != nil {
				return err
			}
			records, err := restRecords(hub, fmt.Sprintf("SELECT PackageVersion.MetadataPackage.Name, PackageVersion.MajorVersion, PackageVersion.MinorVersion, PackageVersion.MetadataPackage.NamespacePrefix, PackageVersion.MetadataPackageId, PackageVersionId, PackageVersion.Name, Id, Status, ScheduledStartTime, StartTime, EndTime, DurationSeconds FROM PackagePushRequest WHERE Id = '%s'", id))
			if err != nil {
				return err
			}
			if len(records) != 1 {
				warn(cmd, "No results found")
				return emit(cmd, nil, nil)
			}
			record := records[0]
			total, err := pushJobCount(hub, id, "")
			if err != nil {
				return err
			}
			var failed, succeeded interface{} = 0, 0
			var reasons []*orderedMap
			switch text(record.values["Status"]) {
			case "Succeeded", "Failed", "In Progress":
				if failed, err = pushJobCount(hub, id, "Failed"); err != nil {
					return err
				}
				if succeeded, err = pushJobCount(hub, id, "Succeeded"); err != nil {
					return err
				}
				if reasons, err = restRecords(hub, fmt.Sprintf("SELECT ErrorMessage, ErrorDetails, ErrorTitle, ErrorSeverity, ErrorType from PackagePushError WHERE PackagePushJob.PackagePushRequestId = '%s' ", id)); err != nil {
					return err
				}
			}
			return emit(cmd, record, func(w io.Writer) {
				version, _ := record.values["PackageVersion"].(*orderedMap)
				if version == nil {
					version = newOrderedMap()
				}
				parent, _ := version.values["MetadataPackage"].(*orderedMap)
				if parent == nil {
					parent = newOrderedMap()
				}
				printTable(w, "", []string{"Name", "Value"}, [][]string{
					{"Package Name", pushCell(parent.values["Name"])},
					{"Package Version Name", pushCell(version.values["Name"])},
					{"Package Version", pushCell(version.values["MajorVersion"]) + "." + pushCell(version.values["MinorVersion"])},
					{"Namespace", pushCell(parent.values["NamespacePrefix"])},
					{"Package Id", pushCell(version.values["MetadataPackageId"])},
					{"Package Version Id", pushCell(record.values["PackageVersionId"])},
					{"Package Push Request Id", pushCell(record.values["Id"])},
					{"Status", pushCell(record.values["Status"])},
					{"Scheduled Start Time", pushCell(record.values["ScheduledStartTime"])},
					{"Start Time", pushCell(record.values["StartTime"])},
					{"End Time", pushCell(record.values["EndTime"])},
					{"Duration Seconds", pushCell(record.values["DurationSeconds"])},
					{"# Orgs Scheduled", pushCell(total)},
					{"# Orgs Upgrade Succeeded", pushCell(succeeded)},
					{"# Orgs Upgrade Failed", pushCell(failed)},
				})
				if len(reasons) == 0 {
					return
				}
				fmt.Fprintln(w)
				var lines []string
				for i, reason := range reasons {
					if i == pushErrorLimit {
						break
					}
					lines = append(lines, fmt.Sprintf("(%d) %s", i+1, pushCell(reason.values["ErrorMessage"])))
				}
				styledHeader(w, paint(w, styleRed, "Errors"))
				warn(cmd, strings.Join(lines, "\n"))
				if len(reasons) > pushErrorLimit {
					warn(cmd, fmt.Sprintf("To see all errors, run: "+binName+" data query -q \"SELECT ErrorMessage FROM PackagePushError WHERE PackagePushJob.PackagePushRequestId='%s'\"", pushCell(record.values["Id"])))
				}
			})
		},
	}
	addJSONFlag(cmd)
	addAPIVersionFlag(cmd)
	cmd.Flags().StringP("push-request-id", "i", "", "ID of the package push request (starts with 0DV).")
	addTargetDevHubFlag(cmd)
	return cmd
}

func newPackagePushUpgradeAbortCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:         "abort",
		Short:       "Abort a package push upgrade that has been scheduled. Only push upgrade requests with a status of Created or Pending can be aborted.",
		Args:        cobra.NoArgs,
		Annotations: map[string]string{commandNameAnnotation: "PackagePushUpgradeAbortCommand"},
		RunE: func(cmd *cobra.Command, args []string) error {
			id, _ := cmd.Flags().GetString("push-request-id")
			if id == "" {
				return missingFlagError("push-request-id")
			}
			if err := salesforceIDFlag("push-request-id", id, "0DV", true); err != nil {
				return err
			}
			hub, err := resolveTargetDevHub(cmd)
			if err != nil {
				return err
			}
			records, err := restRecords(hub, fmt.Sprintf("SELECT Id, Status FROM PackagePushRequest WHERE Id = '%s'", id))
			if err != nil {
				return err
			}
			if len(records) == 0 {
				return &Error{Message: "Can’t abort package push upgrade request. The specified push upgrade ID isn’t valid. Check the ID (starts with 0DV) and retry the command.", ExitCode: 1}
			}
			if status := text(records[0].values["Status"]); status != "Created" && status != "Pending" {
				return &Error{Message: fmt.Sprintf("Can’t abort package push upgrade request with status '%s'. Only push upgrade requests with a status of 'Created' or 'Pending' can be cancelled.", status), ExitCode: 1}
			}
			if _, err := hub.Force.PatchREST("/sobjects/packagepushrequest/"+id, `{"Status":"Canceled"}`); err != nil {
				return apiError(err)
			}
			return emit(cmd, orderedFields("success", true), func(w io.Writer) {
				fmt.Fprintf(w, "Scheduled push upgrade ID [%s] was cancelled.\n", id)
			})
		},
	}
	addJSONFlag(cmd)
	addAPIVersionFlag(cmd)
	cmd.Flags().StringP("push-request-id", "i", "", "ID of the package push request (starts with 0DV).")
	addTargetDevHubFlag(cmd)
	return cmd
}

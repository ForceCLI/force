package sf

import (
	"encoding/json"
	"fmt"
	"io"
	"strings"
	"time"

	"github.com/spf13/cobra"
)

const snapshotFields = "Id,SnapshotName,Description,Status,SourceOrg,CreatedDate,LastModifiedDate,ExpirationDate,Error"

// snapshotError converts the error of a snapshot query.  A Dev Hub without
// the feature does not have the OrgSnapshot object, and sf says so.
func snapshotError(err error) error {
	converted := apiError(err)
	if e, ok := converted.(*Error); ok && e.Name == "INVALID_TYPE" {
		e.Message = "Scratch Org Snapshots isn't enabled for your Dev Hub."
	}
	return converted
}

// snapshotByNameOrID finds a snapshot by its name or, for a value that
// starts with 0Oo, by its Id.
func snapshotByNameOrID(hub *targetOrg, nameOrID string) (*orderedMap, error) {
	field := "SnapshotName"
	if strings.HasPrefix(nameOrID, "0Oo") {
		field = "Id"
	}
	result, err := runQuery(hub, "", "query", fmt.Sprintf("SELECT %s FROM OrgSnapshot WHERE %s = '%s'", snapshotFields, field, nameOrID))
	if err != nil {
		return nil, snapshotError(err)
	}
	records, _ := result.values["records"].([]interface{})
	if len(records) == 0 {
		return nil, newError("SingleRecordQuery_NoRecords", "No snapshot found with the given name or id: %s", nameOrID)
	}
	record, _ := records[0].(*orderedMap)
	return record, nil
}

// snapshotDateTime formats a date and time the way sf does in the en-US
// locale: 01/02/2026, 03:04 PM, in the local time zone.
func snapshotDateTime(value string) string {
	if value == "" {
		return ""
	}
	for _, layout := range []string{"2006-01-02T15:04:05.000-0700", "2006-01-02T15:04:05.000Z", time.RFC3339} {
		if parsed, err := time.Parse(layout, value); err == nil {
			return parsed.Local().Format("01/02/2006, 03:04 PM")
		}
	}
	return value
}

func snapshotDate(value string) string {
	if value == "" {
		return ""
	}
	if parsed, err := time.Parse("2006-01-02", value); err == nil {
		return parsed.Format("1/2/2006")
	}
	return value
}

// printSnapshot prints one snapshot as a table of field names and values.
func printSnapshot(w io.Writer, record *orderedMap) {
	var rows [][]string
	for _, key := range record.keys {
		value := record.values[key]
		if key == "attributes" {
			continue
		}
		content, isText := value.(string)
		if key == "Error" && !isText {
			continue
		}
		if key == "LastModifiedDate" || key == "CreatedDate" {
			content = snapshotDateTime(content)
		}
		rows = append(rows, []string{titleFromCamelCase(key), content})
	}
	printTable(w, "", []string{"Name", "Value"}, rows)
}

func newOrgCreateSnapshotCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:         "snapshot",
		Short:       "Create a snapshot of a scratch org.",
		Args:        cobra.NoArgs,
		Annotations: map[string]string{commandNameAnnotation: "SnapshotCreate"},
		RunE: func(cmd *cobra.Command, args []string) error {
			source, _ := cmd.Flags().GetString("source-org")
			name, _ := cmd.Flags().GetString("name")
			var missing []string
			if name == "" {
				missing = append(missing, "name")
			}
			if source == "" {
				missing = append(missing, "source-org")
			}
			if len(missing) > 0 {
				return missingFlagsError(missing...)
			}
			// The source org is an org Id, or a username or alias of an
			// authorized org.
			if !strings.HasPrefix(source, "00D") {
				org, err := loadOrg(source)
				if err != nil || org.Fields.String("orgId") == "" {
					return flagParseError("", "source-org", "No org found for "+source)
				}
				source = org.Fields.String("orgId")
			}
			hub, err := resolveTargetDevHub(cmd)
			if err != nil {
				return err
			}
			request := orderedFields("SourceOrg", source)
			if cmd.Flags().Changed("description") {
				description, _ := cmd.Flags().GetString("description")
				request.Set("Description", description)
			}
			request.Set("SnapshotName", name)
			request.Set("Content", "metadatadata")
			body, err := json.Marshal(request)
			if err != nil {
				return err
			}
			response, err := hub.Force.PostREST("/sobjects/OrgSnapshot", string(body))
			if err != nil {
				converted := apiError(err)
				// A Dev Hub without the feature does not have the object.
				if e, ok := converted.(*Error); ok && e.Name == "NOT_FOUND" {
					e.Message = "Scratch Org Snapshots isn't enabled for your Dev Hub."
				}
				return converted
			}
			var created struct {
				ID      string `json:"id"`
				Success bool   `json:"success"`
			}
			if json.Unmarshal([]byte(response), &created) != nil || !created.Success {
				return newError("SfError", "An error while created the org snapshot")
			}
			record, err := snapshotByNameOrID(hub, created.ID)
			if err != nil {
				return err
			}
			return emit(cmd, record, func(w io.Writer) { printSnapshot(w, record) })
		},
	}
	addJSONFlag(cmd)
	addAPIVersionFlag(cmd)
	cmd.Flags().StringP("source-org", "o", "", "ID or locally authenticated username or alias of scratch org to snapshot.")
	cmd.Flags().StringP("name", "n", "", "Unique name of snapshot.")
	cmd.Flags().StringP("description", "d", "", "Description of snapshot.")
	addTargetDevHubFlag(cmd)
	return cmd
}

func newOrgListSnapshotCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:         "snapshot",
		Short:       "List scratch org snapshots.",
		Args:        cobra.NoArgs,
		Annotations: map[string]string{commandNameAnnotation: "SnapshotList"},
		RunE: func(cmd *cobra.Command, args []string) error {
			hub, err := resolveTargetDevHub(cmd)
			if err != nil {
				return err
			}
			result, err := runQuery(hub, "", "query", fmt.Sprintf("SELECT %s FROM OrgSnapshot Order by CreatedDate", snapshotFields))
			if err != nil {
				return snapshotError(err)
			}
			records, _ := result.values["records"].([]interface{})
			return emit(cmd, records, func(w io.Writer) {
				if len(records) == 0 {
					fmt.Fprintln(w, "No snapshots found")
					return
				}
				rows := make([][]string, 0, len(records))
				for _, item := range records {
					record, ok := item.(*orderedMap)
					if !ok {
						continue
					}
					rows = append(rows, []string{
						text(record.values["Id"]), text(record.values["SnapshotName"]), text(record.values["Status"]), text(record.values["SourceOrg"]),
						snapshotDateTime(text(record.values["CreatedDate"])), snapshotDateTime(text(record.values["LastModifiedDate"])), snapshotDate(text(record.values["ExpirationDate"])),
					})
				}
				printTable(w, fmt.Sprintf("Org Snapshots [%d]", len(rows)), []string{"Id", "Snapshot Name", "Status", "Source Org Id", "Created Date", "Last Modified Date", "Expiration Date"}, rows)
			})
		},
	}
	addJSONFlag(cmd)
	addAPIVersionFlag(cmd)
	addTargetDevHubFlag(cmd)
	return cmd
}

func newOrgGetSnapshotCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:         "snapshot",
		Short:       "Get details about a scratch org snapshot.",
		Args:        cobra.NoArgs,
		Annotations: map[string]string{commandNameAnnotation: "SnapshotGet"},
		RunE: func(cmd *cobra.Command, args []string) error {
			name, _ := cmd.Flags().GetString("snapshot")
			if name == "" {
				return missingFlagError("snapshot")
			}
			hub, err := resolveTargetDevHub(cmd)
			if err != nil {
				return err
			}
			record, err := snapshotByNameOrID(hub, name)
			if err != nil {
				return err
			}
			return emit(cmd, record, func(w io.Writer) { printSnapshot(w, record) })
		},
	}
	addJSONFlag(cmd)
	addAPIVersionFlag(cmd)
	cmd.Flags().StringP("snapshot", "s", "", "Name or ID of the snapshot to retrieve.")
	addTargetDevHubFlag(cmd)
	return cmd
}

func newOrgDeleteSnapshotCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:         "snapshot",
		Short:       "Delete a scratch org snapshot.",
		Args:        cobra.NoArgs,
		Annotations: map[string]string{commandNameAnnotation: "SnapshotDelete"},
		RunE: func(cmd *cobra.Command, args []string) error {
			name, _ := cmd.Flags().GetString("snapshot")
			if name == "" {
				return missingFlagError("snapshot")
			}
			hub, err := resolveTargetDevHub(cmd)
			if err != nil {
				return err
			}
			if !confirm(cmd, "Are you sure you want to delete the snapshot?") {
				return nil
			}
			record, err := snapshotByNameOrID(hub, name)
			if err != nil {
				return err
			}
			id := text(record.values["Id"])
			if _, err := hub.Force.DeleteREST("/sobjects/OrgSnapshot/" + id); err != nil {
				return apiError(err)
			}
			result := orderedFields("id", id, "success", true, "errors", []interface{}{})
			return emit(cmd, result, func(w io.Writer) {
				fmt.Fprintln(w, paint(w, styleGreen, fmt.Sprintf("Successfully deleted snapshot %s.", name)))
			})
		},
	}
	addJSONFlag(cmd)
	addAPIVersionFlag(cmd)
	cmd.Flags().StringP("snapshot", "s", "", "Name or ID of snapshot to delete.")
	cmd.Flags().BoolP("no-prompt", "p", false, "Don't prompt for confirmation.")
	addTargetDevHubFlag(cmd)
	return cmd
}

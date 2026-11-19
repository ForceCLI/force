package sf

import (
	"encoding/json"
	"fmt"
	"io"

	"github.com/spf13/cobra"
)

// The org shape commands follow the sf plugin source.  Only the output of
// "org list shape" without org shapes was compared with sf.

// shapeEnabled reports whether the org has the org shape preference.  The
// query returns no record when the org has no access to the feature.
func shapeEnabled(org *targetOrg) (bool, error) {
	records, err := toolingRecords(org, "SELECT IsShapeExportPrefEnabled FROM DevHubSettings")
	if err != nil {
		return false, err
	}
	return len(records) > 0 && boolValue(records[0].values["IsShapeExportPrefEnabled"]), nil
}

func isInvalidType(err error) bool {
	converted, ok := apiError(err).(*Error)
	return ok && converted.Name == "INVALID_TYPE"
}

func newOrgListShapeCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:         "shape",
		Short:       "List all org shapes you’ve created.",
		Args:        cobra.NoArgs,
		Annotations: map[string]string{commandNameAnnotation: "OrgShapeListCommand"},
		RunE: func(cmd *cobra.Command, args []string) error {
			// sf asks every authorized org that is not a scratch org.
			var entries []*orgEntry
			for _, entry := range loadOrgEntries() {
				if !entry.isScratch {
					entries = append(entries, entry)
				}
			}
			if len(entries) == 0 {
				return newError("NoAuthFoundError", "No authenticated orgs found.")
			}
			shapes := []*orderedMap{}
			withAlias := false
			for _, entry := range entries {
				result, err := runQuery(entry.org, "", "query", "SELECT Id, Status, CreatedBy.Username, CreatedDate FROM ShapeRepresentation WHERE Status IN ( 'Active', 'InProgress' )")
				if err != nil {
					if !isInvalidType(err) {
						message := err.Error()
						if converted, ok := apiError(err).(*Error); ok {
							message = converted.Message
						}
						warn(cmd, fmt.Sprintf("Org: %s: %s", entry.username, message))
					}
					continue
				}
				records, _ := result.values["records"].([]interface{})
				for _, item := range records {
					record, ok := item.(*orderedMap)
					if !ok {
						continue
					}
					var createdBy interface{}
					if user, ok := record.values["CreatedBy"].(*orderedMap); ok {
						createdBy = user.values["Username"]
					}
					shape := orderedFields("orgId", entry.org.Fields.String("orgId"), "username", entry.username, "shapeId", record.values["Id"],
						"status", record.values["Status"], "createdBy", createdBy, "createdDate", record.values["CreatedDate"])
					if entry.alias != "" {
						shape.Set("alias", entry.alias)
						withAlias = true
					}
					shapes = append(shapes, shape)
				}
			}
			return emit(cmd, shapes, func(w io.Writer) {
				if len(shapes) == 0 {
					fmt.Fprintln(w)
					fmt.Fprintln(w, "No org shapes found.")
					return
				}
				headers := []string{"Org Id", "Username", "Shape Id", "Status", "Created By", "Created Date"}
				if withAlias {
					headers = append(headers, "Alias")
				}
				rows := make([][]string, 0, len(shapes))
				for _, shape := range shapes {
					status := pushCell(shape.values["status"])
					if status == "Active" {
						status = paint(w, styleGreen, status)
					}
					row := []string{pushCell(shape.values["orgId"]), pushCell(shape.values["username"]), pushCell(shape.values["shapeId"]), status,
						pushCell(shape.values["createdBy"]), pushCell(shape.values["createdDate"])}
					if withAlias {
						row = append(row, pushCell(shape.values["alias"]))
					}
					rows = append(rows, row)
				}
				printTable(w, "Org Shapes", headers, rows)
			})
		},
	}
	addJSONFlag(cmd)
	cmd.Flags().Bool("verbose", false, "List more information about each org shape.")
	cmd.Flags().MarkHidden("verbose")
	return cmd
}

func newOrgCreateShapeCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:         "shape",
		Short:       "Create a scratch org configuration (shape) based on the specified source org.",
		Args:        cobra.NoArgs,
		Annotations: map[string]string{commandNameAnnotation: "OrgShapeCreateCommand"},
		RunE: func(cmd *cobra.Command, args []string) error {
			org, err := resolveTargetOrg(cmd)
			if err != nil {
				return err
			}
			enabled, err := shapeEnabled(org)
			if err != nil {
				return err
			}
			if !enabled {
				return newError("ShapeRepresentationNoAccessError", "The org needs to be enabled for org shape before one can be created. %s", org.Username)
			}
			body, err := org.Force.PostREST("/sobjects/ShapeRepresentation", `{"Description":""}`)
			if err != nil {
				return apiError(err)
			}
			var response struct {
				ID      string `json:"id"`
				Success bool   `json:"success"`
			}
			if json.Unmarshal([]byte(body), &response) != nil || !response.Success {
				return newError("ShapeCreateFailedError", "Error creating scratch definition file. Please contact Salesforce support.")
			}
			return emit(cmd, orderedFields("shapeId", response.ID, "success", true, "errors", []interface{}{}), func(w io.Writer) {
				fmt.Fprintln(w, paint(w, styleGreen, fmt.Sprintf("Successfully created org shape for %s.", response.ID)))
			})
		},
	}
	addJSONFlag(cmd)
	addAPIVersionFlag(cmd)
	addTargetOrgFlag(cmd)
	return cmd
}

func newOrgDeleteShapeCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:         "shape",
		Short:       "Delete all org shapes for a target org.",
		Args:        cobra.NoArgs,
		Annotations: map[string]string{commandNameAnnotation: "OrgShapeDeleteCommand"},
		RunE: func(cmd *cobra.Command, args []string) error {
			org, err := resolveTargetOrg(cmd)
			if err != nil {
				return err
			}
			orgID := org.Fields.String("orgId")
			if !confirm(cmd, fmt.Sprintf("Delete shapes for org with name: %s? Are you sure (y/n)?", org.Username)) {
				return nil
			}
			noAccess := newError("NoAccessError", "The org with name: %s needs to be enabled for org shape before shapes can be deleted.", org.Username)
			enabled, err := shapeEnabled(org)
			if err != nil {
				return err
			}
			if !enabled {
				return noAccess
			}
			result, err := runQuery(org, "", "query", "SELECT Id FROM ShapeRepresentation")
			if err != nil {
				if isInvalidType(err) {
					return noAccess
				}
				return apiError(err)
			}
			records, _ := result.values["records"].([]interface{})
			deleted := []string{}
			failures := []*orderedMap{}
			for _, item := range records {
				record, ok := item.(*orderedMap)
				if !ok {
					continue
				}
				id := text(record.values["Id"])
				if _, err := org.Force.DeleteREST("/sobjects/ShapeRepresentation/" + id); err != nil {
					message := err.Error()
					if converted, ok := apiError(err).(*Error); ok {
						message = converted.Message
					}
					failures = append(failures, orderedFields("shapeId", id, "message", message))
					continue
				}
				deleted = append(deleted, id)
			}
			if len(deleted) == 0 {
				// sf reports this also when every deletion failed.
				return emit(cmd, nil, func(w io.Writer) {
					fmt.Fprintf(w, "Can't delete org shape. No org shape found for org %s.\n", orgID)
				})
			}
			status := 0
			if len(failures) > 0 {
				status = 68
			}
			err = emitStatus(cmd, status, orderedFields("orgId", orgID, "shapeIds", deleted, "failures", failures), func(w io.Writer) {
				fmt.Fprintln(w, paint(w, styleGreen, fmt.Sprintf("Successfully deleted org shape for %s.", orgID)))
				if len(failures) == 0 {
					return
				}
				fmt.Fprintln(w)
				rows := make([][]string, 0, len(failures))
				for _, failure := range failures {
					rows = append(rows, []string{text(failure.values["shapeId"]), text(failure.values["message"])})
				}
				printTable(w, "Failures", []string{"Shape ID", "Error Message"}, rows)
			})
			if err != nil || status == 0 {
				return err
			}
			return exitStatus(status)
		},
	}
	addJSONFlag(cmd)
	addAPIVersionFlag(cmd)
	cmd.Flags().BoolP("no-prompt", "p", false, "Don't prompt for confirmation.")
	addTargetOrgFlag(cmd)
	return cmd
}

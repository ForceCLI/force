package sf

import (
	"fmt"
	"io"
	"strings"

	"github.com/spf13/cobra"
)

func newSobjectCmd() *cobra.Command {
	return newGroup("sobject", "Commands to interact with Salesforce objects.",
		newSobjectDescribeCmd(),
		newSobjectListCmd(),
	)
}

func newSobjectListCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:         "list",
		Short:       "List all Salesforce objects of a specified category.",
		Args:        cobra.NoArgs,
		Annotations: map[string]string{commandNameAnnotation: "SObjectList"},
		RunE: func(cmd *cobra.Command, args []string) error {
			category, _ := cmd.Flags().GetString("sobject")
			category = strings.ToLower(category)
			if category != "all" && category != "standard" && category != "custom" {
				return newError("invalid-sobject-type", "You specified an invalid sobject category. Valid values are: all, standard, custom.")
			}
			org, err := resolveTargetOrg(cmd)
			if err != nil {
				return err
			}
			body, err := org.Force.GetREST("/sobjects")
			if err != nil {
				return apiError(err)
			}
			parsed, err := parseOrderedJSON([]byte(body))
			if err != nil {
				return err
			}
			names := []string{}
			if describe, ok := parsed.(*orderedMap); ok {
				sobjects, _ := describe.values["sobjects"].([]interface{})
				for _, item := range sobjects {
					sobject, ok := item.(*orderedMap)
					if !ok {
						continue
					}
					custom := sobject.values["custom"] == true
					if category == "all" || (category == "custom") == custom {
						names = append(names, text(sobject.values["name"]))
					}
				}
			}
			return emit(cmd, names, func(w io.Writer) {
				if len(names) == 0 {
					fmt.Fprintf(w, "No %s objects found.\n", strings.ToUpper(category))
					return
				}
				fmt.Fprintln(w, strings.Join(names, "\n"))
			})
		},
	}
	addJSONFlag(cmd)
	cmd.Flags().StringP("sobject", "s", "ALL", "Category of objects to list.")
	addAPIVersionFlag(cmd)
	addTargetOrgFlag(cmd)
	return cmd
}

func newSobjectDescribeCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:         "describe",
		Short:       "Display the metadata for a standard or custom object or a Tooling API object.",
		Args:        cobra.NoArgs,
		Annotations: map[string]string{commandNameAnnotation: "SObjectDescribe"},
		RunE: func(cmd *cobra.Command, args []string) error {
			sobject, _ := cmd.Flags().GetString("sobject")
			if sobject == "" {
				return missingFlagError("sobject")
			}
			org, err := resolveTargetOrg(cmd)
			if err != nil {
				return err
			}
			body, err := org.Force.GetREST(restBase(cmd) + "/sobjects/" + sobject + "/describe")
			if err != nil {
				return apiError(err)
			}
			result, err := parseOrderedJSON([]byte(body))
			if err != nil {
				return err
			}
			return emit(cmd, result, func(w io.Writer) {
				writeJSON(w, result)
			})
		},
	}
	addJSONFlag(cmd)
	cmd.Flags().StringP("sobject", "s", "", "API name of the object to describe.")
	cmd.Flags().BoolP("use-tooling-api", "t", false, "Use Tooling API to display metadata for Tooling API objects.")
	addAPIVersionFlag(cmd)
	addTargetOrgFlag(cmd)
	return cmd
}

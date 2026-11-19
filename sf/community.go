package sf

import (
	"encoding/json"
	"fmt"
	"io"
	"net/url"
	"strings"

	"github.com/spf13/cobra"
)

// The community commands follow the source of the sf community plugin,
// version 4.0.27, which is not part of the sf installation used for the
// comparisons.  They were not compared with sf.

func newCommunityCmd() *cobra.Command {
	return newGroup("community", "Commands to create and publish Experience Cloud sites.",
		newCommunityCreateCmd(),
		newCommunityPublishCmd(),
		newGroup("list", "List the site templates of an org.", newCommunityListTemplateCmd()),
	)
}

// connectPath escapes a Connect API path the way encodeURI does for the
// characters that can occur in it.
func connectPath(path string) string {
	return (&url.URL{Path: path}).EscapedPath()
}

func newCommunityListTemplateCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:         "template",
		Short:       "Retrieve the list of templates available in your org.",
		Args:        cobra.NoArgs,
		Annotations: map[string]string{commandNameAnnotation: "CommunityListTemplatesCommand"},
		RunE: func(cmd *cobra.Command, args []string) error {
			org, err := resolveTargetOrg(cmd)
			if err != nil {
				return err
			}
			body, err := org.Force.GetREST("/connect/communities/templates")
			if err != nil {
				return apiError(err)
			}
			parsed, err := parseOrderedJSON([]byte(body))
			if err != nil {
				return err
			}
			result, _ := parsed.(*orderedMap)
			if result == nil {
				result = newOrderedMap()
			}
			return emit(cmd, result, func(w io.Writer) {
				templates, _ := result.values["templates"].([]interface{})
				rows := make([][]string, 0, len(templates))
				for _, item := range templates {
					if template, ok := item.(*orderedMap); ok {
						rows = append(rows, []string{pushCell(template.values["templateName"]), pushCell(template.values["publisher"])})
					}
				}
				printTable(w, "Site list template result", []string{"Template Name", "Publisher"}, rows)
				fmt.Fprintf(w, "Total %s\n", pushCell(result.values["total"]))
			})
		},
	}
	addJSONFlag(cmd)
	addAPIVersionFlag(cmd)
	addTargetOrgFlag(cmd)
	return cmd
}

func newCommunityPublishCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:         "publish",
		Short:       "Publish an Experience Builder site to make it live.",
		Args:        cobra.NoArgs,
		Annotations: map[string]string{commandNameAnnotation: "CommunityPublishCommand"},
		RunE: func(cmd *cobra.Command, args []string) error {
			name, _ := cmd.Flags().GetString("name")
			if name == "" {
				return missingFlagError("name")
			}
			org, err := resolveTargetOrg(cmd)
			if err != nil {
				return err
			}
			sites, err := restRecords(org, fmt.Sprintf("SELECT Id, Status FROM NETWORK WHERE NAME = '%s'", name))
			if err != nil {
				return err
			}
			if len(sites) == 0 {
				return newError("CommunityNotExistsError", "The %s site doesn't exist. Verify the site name and try publishing it again.", name)
			}
			body, err := org.Force.PostREST(connectPath("/connect/communities/"+text(sites[0].values["Id"])+"/publish"), "{}")
			if err != nil {
				return apiError(err)
			}
			parsed, err := parseOrderedJSON([]byte(body))
			if err != nil {
				return err
			}
			response, _ := parsed.(*orderedMap)
			if response == nil {
				response = newOrderedMap()
			}
			// sf normalizes the URL, which adds the path "/" to a URL
			// that has none.
			siteURL := text(response.values["url"])
			if normalized, err := url.Parse(siteURL); err == nil && normalized.Host != "" {
				if normalized.Path == "" {
					normalized.Path = "/"
				}
				siteURL = normalized.String()
			}
			result := orderedFields("id", response.values["id"],
				"message", "We’re publishing your changes now. You’ll receive an email confirmation when your changes are live.",
				"name", response.values["name"], "status", sites[0].values["Status"], "url", siteURL)
			if jobID, ok := response.Get("jobId"); ok {
				result.Set("jobId", jobID)
			}
			return emit(cmd, result, func(w io.Writer) {
				printTable(w, "Publish Site Result", []string{"Id", "Message", "Name", "Status", "Url", "Job Id"}, [][]string{{
					pushCell(result.values["id"]), pushCell(result.values["message"]), pushCell(result.values["name"]),
					pushCell(result.values["status"]), pushCell(result.values["url"]), pushCell(result.values["jobId"]),
				}})
			})
		},
	}
	addJSONFlag(cmd)
	cmd.Flags().StringP("name", "n", "", "Name of the Experience Builder site to publish.")
	addAPIVersionFlag(cmd)
	addTargetOrgFlag(cmd)
	return cmd
}

func newCommunityCreateCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:         "create",
		Short:       "Create an Experience Cloud site using a template.",
		Annotations: map[string]string{commandNameAnnotation: "CommunityCreateCommand"},
		RunE: func(cmd *cobra.Command, args []string) error {
			name, _ := cmd.Flags().GetString("name")
			templateName, _ := cmd.Flags().GetString("template-name")
			var missing []string
			if name == "" {
				missing = append(missing, "name")
			}
			if templateName == "" {
				missing = append(missing, "template-name")
			}
			if len(missing) > 0 {
				return missingFlagsError(missing...)
			}
			// The arguments are template parameters:
			// templateParams.AuthenticationType=UNAUTHENTICATED.
			templateParams := newOrderedMap()
			var invalid []string
			if len(args) == 2 && !strings.Contains(args[0], "=") {
				args = []string{args[0] + "=" + args[1]}
			}
			for _, arg := range args {
				parts := strings.Split(arg, "=")
				if len(parts) != 2 {
					return newError("InvalidArgumentFormatError", "Error in the following argument\n%s\nSet varargs with this format: key=value or key=\"value with spaces\"", arg)
				}
				if _, duplicate := templateParams.Get(strings.TrimPrefix(parts[0], "templateParams.")); duplicate {
					return newError("DuplicateArgumentError", "Found duplicate argument %s.", parts[0])
				}
				if !strings.HasPrefix(parts[0], "templateParams") {
					invalid = append(invalid, parts[0])
					continue
				}
				if parts[1] != "" {
					templateParams.Set(strings.Replace(parts[0], "templateParams.", "", 1), parts[1])
				}
			}
			if len(invalid) > 0 {
				return &Error{
					Name:     "InvalidArgument",
					Message:  "Invalid argument(s): " + strings.Join(invalid, ", "),
					ExitCode: 1,
					Actions:  []string{"Arguments should start with templateParams, like templateParams.AuthenticationType=UNAUTHENTICATED"},
				}
			}
			org, err := resolveTargetOrg(cmd)
			if err != nil {
				return err
			}
			prefix, _ := cmd.Flags().GetString("url-path-prefix")
			// sf does not send --description.
			request, err := json.Marshal(orderedFields("name", name, "urlPathPrefix", prefix, "templateName", templateName, "templateParams", templateParams))
			if err != nil {
				return err
			}
			body, err := org.Force.PostREST("/connect/communities", string(request))
			if err != nil {
				return apiError(err)
			}
			parsed, err := parseOrderedJSON([]byte(body))
			if err != nil {
				return err
			}
			response, _ := parsed.(*orderedMap)
			if response == nil {
				response = newOrderedMap()
			}
			result := orderedFields("message", "Your Site is being created.", "name", response.values["name"], "jobId", response.values["jobId"],
				"action", "Site creation is under way. To track its status, query the BackgroundOperation object and include the jobId.")
			return emit(cmd, result, func(w io.Writer) {
				printTable(w, "Create Site Result", []string{"Message", "Name", "Job Id", "Action"}, [][]string{{
					pushCell(result.values["message"]), pushCell(result.values["name"]), pushCell(result.values["jobId"]), pushCell(result.values["action"]),
				}})
			})
		},
	}
	addJSONFlag(cmd)
	cmd.Flags().StringP("name", "n", "", "Name of the site to create.")
	cmd.Flags().StringP("template-name", "t", "", "Template to use to create a site.")
	cmd.Flags().StringP("url-path-prefix", "p", "", "URL to append to the domain created when Digital Experiences was enabled for this org.")
	cmd.Flags().StringP("description", "d", "", "Description of the site.")
	addAPIVersionFlag(cmd)
	addTargetOrgFlag(cmd)
	return cmd
}

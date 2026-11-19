package sf

import (
	"bytes"
	"fmt"
	"io"
	"net/http"
	"os"
	"sort"
	"strings"

	"github.com/spf13/cobra"

	"github.com/ForceCLI/force/lib"
)

func newApiCmd() *cobra.Command {
	return newGroup("api", "Commands to send API requests to your org.",
		newGroup("request", "Make an authenticated HTTP request to your org.",
			newApiRequestRestCmd(),
			newApiRequestGraphqlCmd(),
		),
	)
}

var restMethods = []string{"GET", "POST", "PUT", "PATCH", "HEAD", "DELETE", "OPTIONS", "TRACE"}

func newApiRequestRestCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "rest [url]",
		Short: "Make an authenticated HTTP request using the Salesforce REST API.",
		Args:  cobra.MaximumNArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			warn(cmd, "This command is currently in beta. Any aspect of this command can change without advanced notice. Don't use beta commands in your scripts.")
			// --file holds the request: url, method, header and body.  A
			// value on the command line replaces the one in the file.
			var fileOptions *orderedMap
			if file, _ := cmd.Flags().GetString("file"); file != "" {
				data, err := os.ReadFile(file)
				if err != nil {
					return err
				}
				parsed, err := parseOrderedJSON(data)
				if err != nil {
					return fmt.Errorf("%s: %w", file, err)
				}
				fileOptions, _ = parsed.(*orderedMap)
			}
			fileValue := func(key string) interface{} {
				if fileOptions == nil {
					return nil
				}
				return fileOptions.values[key]
			}
			if len(args) == 0 {
				switch target := fileValue("url").(type) {
				case string:
					args = []string{target}
				case *orderedMap:
					args = []string{text(target.values["raw"])}
				}
			}
			if len(args) == 0 || args[0] == "" {
				return &Error{Message: "The url is required either in --file file's content or as an argument", ExitCode: 1}
			}
			method, _ := cmd.Flags().GetString("method")
			if !cmd.Flags().Changed("method") {
				if fromFile := text(fileValue("method")); fromFile != "" {
					method = fromFile
				}
			}
			validMethod := false
			for _, known := range restMethods {
				if method == known {
					validMethod = true
				}
			}
			if !validMethod {
				return &Error{Message: fmt.Sprintf("\"%s\" must be one of %s", method, strings.Join(restMethods, ", ")), ExitCode: 1}
			}
			org, err := resolveTargetOrg(cmd)
			if err != nil {
				return err
			}
			var body io.Reader
			if method != "GET" {
				content, _ := cmd.Flags().GetString("body")
				switch {
				case strings.HasPrefix(content, "@"):
					data, err := os.ReadFile(content[1:])
					if err != nil {
						return err
					}
					body = bytes.NewReader(data)
				case content == "-":
					data, err := io.ReadAll(cmd.InOrStdin())
					if err != nil {
						return err
					}
					body = bytes.NewReader(data)
				case content != "":
					body = strings.NewReader(content)
				case fileOptions != nil:
					description, _ := fileValue("body").(*orderedMap)
					if description == nil || text(description.values["mode"]) == "" {
						return &Error{Message: "No 'mode' found in 'body' entry", ExitCode: 1, Actions: []string{`add "mode":"raw" | "formdata" to your body`}}
					}
					if text(description.values["mode"]) != "raw" {
						return newError("UnsupportedFlagError", "The body mode \"%s\" in --file is not supported by the sf-compatible mode of force.", text(description.values["mode"]))
					}
					encoded, err := encodeJSON(description.values["raw"])
					if err != nil {
						return err
					}
					body = strings.NewReader(encoded)
				}
			}
			// A request through lib refreshes an expired access token
			// before the request below uses it.
			if _, err := org.Force.GetAbsolute("/services/data/" + lib.ApiVersion()); err != nil {
				return apiError(err)
			}
			target := org.Force.Credentials.InstanceUrl + "/" + strings.TrimLeft(args[0], "/")
			request, err := http.NewRequest(method, target, body)
			if err != nil {
				return err
			}
			request.Header.Set("Authorization", "Bearer "+org.Force.Credentials.AccessToken)
			headers, _ := cmd.Flags().GetStringArray("header")
			if len(headers) == 0 {
				// Headers in the file are "key:value" strings or objects
				// with key and value.
				switch fromFile := fileValue("header").(type) {
				case string:
					headers = []string{fromFile}
				case []interface{}:
					for _, item := range fromFile {
						switch header := item.(type) {
						case string:
							headers = append(headers, header)
						case *orderedMap:
							if header.values["disabled"] != true {
								headers = append(headers, text(header.values["key"])+":"+displayValue(header.values["value"]))
							}
						}
					}
				}
			}
			for _, header := range headers {
				key, value, ok := strings.Cut(header, ":")
				value = strings.TrimSpace(value)
				if !ok || key == "" || value == "" {
					return &Error{
						Name:     "Failed To Parse HTTP Header",
						Message:  fmt.Sprintf("Failed to parse HTTP header: \"%s\".", header),
						ExitCode: 1,
						Actions:  []string{`Make sure the header is in a "key:value" format, e.g. "Accept: application/json"`},
					}
				}
				request.Header.Set(key, value)
			}
			if body != nil && request.Header.Get("Content-Type") == "" {
				request.Header.Set("Content-Type", "application/json")
			}
			client := &http.Client{CheckRedirect: func(*http.Request, []*http.Request) error { return http.ErrUseLastResponse }}
			response, err := client.Do(request)
			if err != nil {
				return err
			}
			defer response.Body.Close()
			out := cmd.OutOrStdout()
			if streamFile, _ := cmd.Flags().GetString("stream-to-file"); streamFile != "" {
				file, err := os.Create(streamFile)
				if err != nil {
					return err
				}
				defer file.Close()
				if _, err := io.Copy(file, response.Body); err != nil {
					return err
				}
				fmt.Fprintf(out, "File saved to %s\n", streamFile)
				return nil
			}
			content, err := io.ReadAll(response.Body)
			if err != nil {
				return err
			}
			if include, _ := cmd.Flags().GetBool("include"); include {
				fmt.Fprintf(out, "HTTP/%d.%d %d\n", response.ProtoMajor, response.ProtoMinor, response.StatusCode)
				names := make([]string, 0, len(response.Header))
				for name := range response.Header {
					names = append(names, name)
				}
				sort.Strings(names)
				for _, name := range names {
					fmt.Fprintf(out, "%s: %s\n", paint(out, style{"\x1b[34m\x1b[1m", "\x1b[22m\x1b[39m"}, strings.ToLower(name)), strings.Join(response.Header[name], ","))
				}
			}
			if parsed, err := parseOrderedJSON(content); err == nil {
				writeJSON(out, parsed)
			} else {
				fmt.Fprintln(out, string(content))
			}
			if response.StatusCode >= 400 {
				return exitStatus(1)
			}
			return nil
		},
	}
	cmd.Flags().StringP("method", "X", "GET", "HTTP method for the request.")
	cmd.Flags().StringArrayP("header", "H", nil, "HTTP header in \"key:value\" format.")
	cmd.Flags().StringP("file", "f", "", "JSON file that contains values for the request header, body, method, and URL.")
	cmd.Flags().StringP("body", "b", "", "File or content for the body of the HTTP request.")
	cmd.Flags().BoolP("include", "i", false, "Include the HTTP response status and headers in the output.")
	cmd.Flags().StringP("stream-to-file", "S", "", "Stream responses to a file.")
	addAPIVersionFlag(cmd)
	addTargetOrgFlag(cmd)
	return cmd
}

func newApiRequestGraphqlCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "graphql",
		Short: "Execute a GraphQL statement.",
		Args:  cobra.NoArgs,
		RunE: func(cmd *cobra.Command, args []string) error {
			warn(cmd, "This command is currently in beta. Any aspect of this command can change without advanced notice. Don't use beta commands in your scripts.")
			body, _ := cmd.Flags().GetString("body")
			if body == "" {
				return missingFlagError("body")
			}
			// --body is a file, "-" for standard input, or the statement.
			query := body
			if body == "-" {
				data, err := io.ReadAll(cmd.InOrStdin())
				if err != nil {
					return err
				}
				query = string(data)
			} else if data, err := os.ReadFile(body); err == nil {
				query = string(data)
			}
			org, err := resolveTargetOrg(cmd)
			if err != nil {
				return err
			}
			if _, err := org.Force.GetAbsolute("/services/data/" + lib.ApiVersion()); err != nil {
				return apiError(err)
			}
			payload, err := encodeJSON(map[string]string{"query": query})
			if err != nil {
				return err
			}
			request, err := http.NewRequest("POST", org.Force.Credentials.InstanceUrl+"/services/data/"+lib.ApiVersion()+"/graphql", strings.NewReader(payload))
			if err != nil {
				return err
			}
			request.Header.Set("Authorization", "Bearer "+org.Force.Credentials.AccessToken)
			request.Header.Set("Content-Type", "application/json")
			response, err := http.DefaultClient.Do(request)
			if err != nil {
				return err
			}
			defer response.Body.Close()
			out := cmd.OutOrStdout()
			if streamFile, _ := cmd.Flags().GetString("stream-to-file"); streamFile != "" {
				file, err := os.Create(streamFile)
				if err != nil {
					return err
				}
				defer file.Close()
				if _, err := io.Copy(file, response.Body); err != nil {
					return err
				}
				fmt.Fprintf(out, "File saved to %s\n", streamFile)
				return nil
			}
			content, err := io.ReadAll(response.Body)
			if err != nil {
				return err
			}
			if include, _ := cmd.Flags().GetBool("include"); include {
				fmt.Fprintf(out, "HTTP/%d.%d %d\n", response.ProtoMajor, response.ProtoMinor, response.StatusCode)
				names := make([]string, 0, len(response.Header))
				for name := range response.Header {
					names = append(names, name)
				}
				sort.Strings(names)
				for _, name := range names {
					fmt.Fprintf(out, "%s: %s\n", strings.ToLower(name), strings.Join(response.Header[name], ","))
				}
			}
			failed := response.StatusCode >= 400
			if parsed, err := parseOrderedJSON(content); err == nil {
				// Like sf, the exit status follows the HTTP status only: a
				// response with status 200 that reports errors in its
				// body exits with 0.
				writeJSON(out, parsed)
			} else {
				fmt.Fprintln(out, string(content))
			}
			if failed {
				return exitStatus(1)
			}
			return nil
		},
	}
	cmd.Flags().String("body", "", "File or content with the GraphQL statement. Specify \"-\" to read from standard input.")
	cmd.Flags().BoolP("include", "i", false, "Include the HTTP response status and headers in the output.")
	cmd.Flags().StringP("stream-to-file", "S", "", "Stream responses to a file.")
	addAPIVersionFlag(cmd)
	addTargetOrgFlag(cmd)
	return cmd
}

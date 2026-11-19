package sf

import (
	"encoding/json"
	"fmt"
	"io"
	"strconv"
	"strings"
	"time"

	"github.com/spf13/cobra"
)

// The package1 version create commands follow the sf plugin source.  The
// messages for a queued request, a request in progress, a failed upload and
// a finished upload, with and without --wait, were compared with sf against
// a packaging org.

// package1UploadInterval is the time between checks of a package upload
// request.
var package1UploadInterval = 5 * time.Second

func packageUploadRequest(org *targetOrg, id string) (*orderedMap, error) {
	body, err := org.Force.GetREST("/tooling/sobjects/PackageUploadRequest/" + id)
	if err != nil {
		return nil, apiError(err)
	}
	parsed, err := parseOrderedJSON([]byte(body))
	if err != nil {
		return nil, err
	}
	record, ok := parsed.(*orderedMap)
	if !ok {
		return nil, fmt.Errorf("unexpected response for PackageUploadRequest %s", id)
	}
	return record, nil
}

// uploadErrors returns the messages of a failed package upload request.
func uploadErrors(request *orderedMap) []string {
	var messages []string
	if errors, ok := request.values["Errors"].(*orderedMap); ok {
		if list, ok := errors.values["errors"].([]interface{}); ok {
			for _, item := range list {
				if entry, ok := item.(*orderedMap); ok {
					messages = append(messages, text(entry.values["message"]))
				}
			}
		}
	}
	return messages
}

// uploadStatusMessage is the line sf prints for a package upload request.
func uploadStatusMessage(request *orderedMap, username string) string {
	switch text(request.values["Status"]) {
	case "SUCCESS":
		return fmt.Sprintf("Successfully uploaded package [%s]", text(request.values["MetadataPackageVersionId"]))
	case "QUEUED":
		return fmt.Sprintf("PackageUploadRequest has been enqueued. You can query the status using\n%s package1:version:create:get -i %s -o %s", binName, text(request.values["Id"]), username)
	}
	return fmt.Sprintf("PackageUploadRequest is still InProgress. You can query the status using\n%s package1:version:create:get -i %s -o %s", binName, text(request.values["Id"]), username)
}

func newPackage1VersionCreateCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:         "create",
		Short:       "Create a first-generation package version in the release org.",
		Args:        cobra.NoArgs,
		Annotations: map[string]string{commandNameAnnotation: "Package1VersionCreateCommand"},
		RunE: func(cmd *cobra.Command, args []string) error {
			packageFlag, _ := cmd.Flags().GetString("package-id")
			name, _ := cmd.Flags().GetString("name")
			var missing []string
			if name == "" {
				missing = append(missing, "name")
			}
			if packageFlag == "" {
				missing = append(missing, "package-id")
			}
			if len(missing) > 0 {
				return missingFlagsError(missing...)
			}
			if err := salesforceIDFlag("package-id", packageFlag, "033", true); err != nil {
				return err
			}
			request := newOrderedMap()
			request.Set("MetadataPackageId", packageFlag)
			request.Set("VersionName", name)
			if cmd.Flags().Changed("description") {
				value, _ := cmd.Flags().GetString("description")
				request.Set("Description", value)
			}
			if version, _ := cmd.Flags().GetString("version"); cmd.Flags().Changed("version") {
				parts := strings.Split(version, ".")
				if len(parts) != 2 {
					return newError("Package1VersionCreateCommandInvalidVersionError", "Version supplied, %s, is not formatted correctly. Enter in major.minor format, for example, 3.2.", version)
				}
				for i, key := range []string{"MajorVersion", "MinorVersion"} {
					// A part that is not a number becomes null, as
					// JSON.stringify writes NaN.
					if number, err := strconv.ParseFloat(strings.TrimSpace(parts[i]), 64); err == nil {
						request.Set(key, number)
					} else {
						request.Set(key, nil)
					}
				}
			}
			if released, _ := cmd.Flags().GetBool("managed-released"); cmd.Flags().Changed("managed-released") {
				request.Set("IsReleaseVersion", released)
			}
			for _, flag := range [][2]string{{"release-notes-url", "ReleaseNotesUrl"}, {"post-install-url", "PostInstallUrl"}, {"installation-key", "Password"}} {
				if cmd.Flags().Changed(flag[0]) {
					value, _ := cmd.Flags().GetString(flag[0])
					request.Set(flag[1], value)
				}
			}
			org, err := resolveTargetOrg(cmd)
			if err != nil {
				return err
			}
			body, err := json.Marshal(request)
			if err != nil {
				return err
			}
			response, err := org.Force.PostREST("/tooling/sobjects/PackageUploadRequest", string(body))
			if err != nil {
				return apiError(err)
			}
			var created struct {
				ID      string `json:"id"`
				Success bool   `json:"success"`
			}
			if json.Unmarshal([]byte(response), &created) != nil || !created.Success {
				return newError("CreateFailedError", "The attempt to create a package version failed. See the following for more information: %s", response)
			}
			minutes, _ := cmd.Flags().GetInt("wait")
			timeout := time.Duration(minutes) * time.Minute
			var result *orderedMap
			deadline := time.Now().Add(timeout)
			for {
				if result, err = packageUploadRequest(org, created.ID); err != nil {
					return err
				}
				if timeout == 0 {
					break
				}
				status := text(result.values["Status"])
				if status == "SUCCESS" {
					break
				}
				if status != "IN_PROGRESS" && status != "QUEUED" {
					if messages := uploadErrors(result); len(messages) > 0 {
						return newError("Package1VersionCreateCommandUploadFailureError", "Package upload failed.\n%s", strings.Join(messages, "\n"))
					}
					return newError("Package1VersionCreateCommandUploadFailureDefaultError", "Package version creation failed with unknown error")
				}
				// sf prints nothing while it waits: it listens for the
				// progress of another kind of package version, so its
				// "Package upload is in progress" line is never printed.
				if !time.Now().Add(package1UploadInterval).Before(deadline) {
					return &Error{Name: "ClientTimeoutError", Message: "The client has timed out.", ExitCode: 1}
				}
				time.Sleep(package1UploadInterval)
			}
			return emit(cmd, result, func(w io.Writer) {
				fmt.Fprintln(w, uploadStatusMessage(result, org.Username))
			})
		},
	}
	addJSONFlag(cmd)
	addAPIVersionFlag(cmd)
	cmd.Flags().StringP("package-id", "i", "", "ID of the metadata package (starts with 033) of which you’re creating a new version.")
	cmd.Flags().StringP("name", "n", "", "Package version name.")
	cmd.Flags().StringP("description", "d", "", "Package version description.")
	cmd.Flags().StringP("version", "v", "", "Package version in major.minor format, for example, 3.2.")
	cmd.Flags().BoolP("managed-released", "m", false, "Create a managed package version.")
	cmd.Flags().StringP("release-notes-url", "r", "", "Release notes URL.")
	cmd.Flags().StringP("post-install-url", "p", "", "Post install URL.")
	cmd.Flags().StringP("installation-key", "k", "", "Installation key for key-protected package (default: null).")
	cmd.Flags().IntP("wait", "w", 0, "Minutes to wait for the package version to be created (default: 2 minutes).")
	addTargetOrgFlag(cmd)
	return cmd
}

func newPackage1VersionCreateGetCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:         "get",
		Short:       "Retrieve the status of a package version creation request.",
		Args:        cobra.NoArgs,
		Annotations: map[string]string{commandNameAnnotation: "Package1VersionCreateGetCommand"},
		RunE: func(cmd *cobra.Command, args []string) error {
			id, _ := cmd.Flags().GetString("request-id")
			if id == "" {
				return missingFlagError("request-id")
			}
			if err := salesforceIDFlag("request-id", id, "0HD", true); err != nil {
				return err
			}
			org, err := resolveTargetOrg(cmd)
			if err != nil {
				return err
			}
			result, err := packageUploadRequest(org, id)
			if err != nil {
				return err
			}
			if text(result.values["Status"]) == "ERROR" {
				detail := "Package version creation failed with unknown error"
				if messages := uploadErrors(result); len(messages) > 0 {
					detail = strings.Join(messages, "\n")
				}
				return newError("UploadFailureError", "Package upload failed.\n%s", detail)
			}
			return emit(cmd, result, func(w io.Writer) {
				fmt.Fprintln(w, uploadStatusMessage(result, org.Username))
			})
		},
	}
	addJSONFlag(cmd)
	addAPIVersionFlag(cmd)
	cmd.Flags().StringP("request-id", "i", "", "ID of the PackageUploadRequest (starts with 0HD).")
	addTargetOrgFlag(cmd)
	return cmd
}

package sf

import (
	"encoding/csv"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"time"

	"github.com/ForceCLI/force/lib"
	"github.com/spf13/cobra"
)

// package push-upgrade schedule follows the sf plugin source and was not
// compared with sf: it upgrades a package in subscriber orgs.

var subscriberOrgID = regexp.MustCompile(`^00D[a-zA-Z0-9]{12}$`)

// pushJobTimeout is the time sf waits for the bulk job that creates the
// PackagePushJob records.
const pushJobTimeout = 10 * time.Minute

func newPackagePushUpgradeScheduleCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:         "schedule",
		Short:       "Schedule a package push upgrade.",
		Args:        cobra.NoArgs,
		Annotations: map[string]string{commandNameAnnotation: "PackagePushScheduleCommand"},
		RunE: func(cmd *cobra.Command, args []string) error {
			versionID, _ := cmd.Flags().GetString("package")
			if versionID == "" {
				return missingFlagError("package")
			}
			if err := salesforceIDFlag("package", versionID, "04t", true); err != nil {
				return err
			}
			orgFile, _ := cmd.Flags().GetString("org-file")
			orgInput, _ := cmd.Flags().GetString("org-list")
			if cmd.Flags().Changed("org-file") && cmd.Flags().Changed("org-list") {
				return &Error{Message: fmt.Sprintf("The following errors occurred:\n  --org-file=%s cannot also be provided when using --org-list\n  --org-list=%s cannot also be provided when using --org-file\nSee more help with --help", orgFile, orgInput), ExitCode: 2}
			}
			var orgs []string
			switch {
			case orgFile != "":
				data, err := os.ReadFile(orgFile)
				if err != nil {
					return flagParseError("", "org-file", "No file found at "+orgFile)
				}
				// sf keeps the lines that are 15 character org Ids.
				for _, line := range strings.Split(strings.ReplaceAll(string(data), "\r\n", "\n"), "\n") {
					if strings.TrimSpace(line) != "" && subscriberOrgID.MatchString(line) {
						orgs = append(orgs, line)
					}
				}
			case orgInput != "":
				for _, item := range strings.Split(orgInput, ",") {
					if item = strings.TrimSpace(item); item != "" {
						orgs = append(orgs, item)
					}
				}
			default:
				return newError("SfError", "Can’t schedule the package push upgrade. You must specify either a list of orgs, or a file containing the list of orgs to be upgraded. Retry this command using either --org-list or --org-file flag and include the required details.")
			}
			hub, err := resolveTargetDevHub(cmd)
			if err != nil {
				return err
			}
			startTime, _ := cmd.Flags().GetString("start-time")
			request := orderedFields("PackageVersionId", versionID)
			if startTime != "" {
				request.Set("ScheduledStartTime", startTime)
			}
			if migrate, _ := cmd.Flags().GetBool("migrate-to-2gp"); migrate {
				request.Set("IsMigration", true)
			}
			body, err := json.Marshal(request)
			if err != nil {
				return err
			}
			response, err := hub.Force.PostREST("/sobjects/packagepushrequest/", string(body))
			if err != nil {
				return apiError(err)
			}
			var created struct {
				ID string `json:"id"`
			}
			if err := json.Unmarshal([]byte(response), &created); err != nil {
				return err
			}

			// One PackagePushJob for each org, created with the Bulk API.
			var jobs strings.Builder
			writer := csv.NewWriter(&jobs)
			writer.Write([]string{"PackagePushRequestId", "SubscriberOrganizationKey"})
			for _, org := range orgs {
				writer.Write([]string{created.ID, org})
			}
			writer.Flush()
			job, err := hub.Force.CreateBulk2IngestJob(lib.Bulk2IngestJobRequest{Object: "PackagePushJob", Operation: lib.Bulk2OperationInsert})
			if err != nil {
				return apiError(err)
			}
			abandon := func(err error) error {
				hub.Force.DeleteBulk2IngestJob(job.Id)
				return err
			}
			if err := hub.Force.UploadBulk2JobData(job.Id, strings.NewReader(jobs.String())); err != nil {
				return abandon(apiError(err))
			}
			if _, err := hub.Force.CloseBulk2IngestJob(job.Id); err != nil {
				return abandon(apiError(err))
			}
			_, finished, err := waitForIngestJob(hub, job.Id, pushJobTimeout)
			if err != nil {
				return abandon(err)
			}
			if !finished {
				return &Error{Name: "JobPollingTimeoutError", Message: fmt.Sprintf("Polling time out. Job Id = %s", job.Id), ExitCode: 1}
			}
			failed, err := hub.Force.GetBulk2FailedResults(job.Id)
			if err != nil {
				return abandon(apiError(err))
			}
			if rows, err := csv.NewReader(strings.NewReader(string(failed))).ReadAll(); err == nil && len(rows) > 1 {
				column := -1
				for i, name := range rows[0] {
					if name == "sf__Error" {
						column = i
					}
				}
				var lines []string
				for i, row := range rows[1:] {
					message := ""
					if column >= 0 && column < len(row) {
						message = row[column]
					}
					encoded, _ := json.Marshal(message)
					lines = append(lines, fmt.Sprintf("Job %d Error:%s", i+1, encoded))
				}
				wd, _ := os.Getwd()
				file := filepath.Join(wd, "job_errors", fmt.Sprintf("push_request_%s_errors.log", created.ID))
				if err := os.MkdirAll(filepath.Dir(file), 0o755); err != nil {
					return abandon(newError("SfError", "Error when saving job errors to file. %s", err.Error()))
				}
				if err := os.WriteFile(file, []byte(strings.Join(lines, "\n")), 0o644); err != nil {
					return abandon(newError("SfError", "Error when saving job errors to file. %s", err.Error()))
				}
				return abandon(newError("SfError", "Push upgrade failed, job errors have been written to file: %s", file))
			}
			if _, err := hub.Force.PatchREST("/sobjects/packagepushrequest/"+created.ID, `{"Status":"Pending"}`); err != nil {
				return abandon(apiError(err))
			}
			result := orderedFields("PushRequestId", created.ID)
			if startTime != "" {
				result.Set("ScheduledStartTime", startTime)
			}
			result.Set("Status", "Pending")
			return emit(cmd, result, func(w io.Writer) {
				fmt.Fprintf(w, "Push upgrade has been scheduled. To check the status of this push upgrade, use push upgrade request ID [%s] with either \"package push-upgrade list\" or \"package push-upgrade report\".\nOrgs scheduled for push upgrade: {%s}\n", created.ID, strings.Join(orgs, ", "))
			})
		},
	}
	addJSONFlag(cmd)
	addAPIVersionFlag(cmd)
	cmd.Flags().StringP("package", "p", "", "ID (starts with 04t) of the package version that the package is being upgraded to.")
	cmd.Flags().StringP("start-time", "t", "", "Date and time (UTC) when the push upgrade is processed.")
	cmd.Flags().StringP("org-list", "l", "", "Comma-separated list of subscriber org IDs that need the package upgrade.")
	cmd.Flags().StringP("org-file", "f", "", "Filename of the CSV file that contains the list of subscriber org IDs that need the package upgrade.")
	cmd.Flags().Bool("migrate-to-2gp", false, "Upgrade from a first-generation managed package (1GP) to a second-generation managed package (2GP).")
	addTargetDevHubFlag(cmd)
	return cmd
}

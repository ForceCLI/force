package sf

import (
	"bytes"
	"encoding/csv"
	"fmt"
	"io"
	"os"
	"strings"
	"time"

	"github.com/spf13/cobra"

	"github.com/ForceCLI/force/lib"
)

// The commands in this file use Bulk API 1.0.  sf keeps them for scripts
// written before the Bulk API 2.0 commands existed.

func newForceLegacyCmd() *cobra.Command {
	return newGroup("force", "Legacy commands.",
		newGroup("data", "Legacy data commands.",
			newGroup("bulk", "Bulk API 1.0 commands.",
				newLegacyBulkIngestCmd("delete", "Bulk delete records from an org using a CSV file. Uses Bulk API 1.0."),
				newLegacyBulkIngestCmd("upsert", "Bulk upsert records to an org from a CSV file. Uses Bulk API 1.0."),
				newLegacyBulkStatusCmd("status", "View the status of a bulk data load job or batch. Uses Bulk API 1.0."),
			),
		),
	)
}

// styledHeader prints a header in the form sf uses: "=== Title" and an
// empty line.
func styledHeader(w io.Writer, title string) {
	fmt.Fprintf(w, "%s%s\n\n", paint(w, styleDim, "=== "), paint(w, styleBold, title))
}

// styledObject prints "key: value" lines with the values aligned.
func styledObject(w io.Writer, object *orderedMap) {
	width := 0
	for _, key := range object.keys {
		if len(key) > width && key != "$" {
			width = len(key)
		}
	}
	width += 2
	for _, key := range object.keys {
		if key == "$" {
			// The XML namespace of the response is part of the JSON
			// result only.
			continue
		}
		fmt.Fprintf(w, "%s:%s%s\n", paint(w, style{"\x1b[34m", "\x1b[39m"}, key), strings.Repeat(" ", width-len(key)-1), displayValue(object.values[key]))
	}
}

func legacyJobInfo(job lib.JobInfo) *orderedMap {
	info := newOrderedMap()
	set := func(key, value string) {
		if value != "" {
			info.Set(key, value)
		}
	}
	number := func(key string, value int) { info.Set(key, fmt.Sprintf("%d", value)) }
	info.Set("$", legacyXMLNamespace())
	set("id", job.Id)
	set("operation", job.Operation)
	set("object", job.Object)
	set("createdById", job.CreatedById)
	set("createdDate", job.CreatedDate)
	set("systemModstamp", job.SystemModStamp)
	set("state", job.State)
	set("externalIdFieldName", job.ExternalIdFieldName)
	set("concurrencyMode", job.ConcurrencyMode)
	set("contentType", job.ContentType)
	number("numberBatchesQueued", job.NumberBatchesQueued)
	number("numberBatchesInProgress", job.NumberBatchesInProgress)
	number("numberBatchesCompleted", job.NumberBatchesCompleted)
	number("numberBatchesFailed", job.NumberBatchesFailed)
	number("numberBatchesTotal", job.NumberBatchesTotal)
	number("numberRecordsProcessed", job.NumberRecordsProcessed)
	number("numberRetries", job.NumberRetries)
	set("apiVersion", job.ApiVersion)
	number("numberRecordsFailed", job.NumberRecordsFailed)
	number("totalProcessingTime", job.TotalProcessingTime)
	number("apiActiveProcessingTime", job.ApiActiveProcessingTime)
	number("apexProcessingTime", job.ApexProcessingTime)
	return info
}

// legacyXMLNamespace is the "$" member sf reports with a job or a batch: the
// attributes of the root element of the Bulk API response.
func legacyXMLNamespace() *orderedMap {
	return orderedFields("xmlns", "http://www.force.com/2009/06/asyncapi/dataload")
}

func legacyBatchInfo(batch lib.BatchInfo) *orderedMap {
	info := newOrderedMap()
	info.Set("$", legacyXMLNamespace())
	info.Set("id", batch.Id)
	info.Set("jobId", batch.JobId)
	info.Set("state", batch.State)
	if batch.StateMessage != "" {
		info.Set("stateMessage", batch.StateMessage)
	}
	info.Set("createdDate", batch.CreatedDate)
	info.Set("systemModstamp", batch.SystemModstamp)
	info.Set("numberRecordsProcessed", fmt.Sprintf("%d", batch.NumberRecordsProcessed))
	info.Set("numberRecordsFailed", fmt.Sprintf("%d", batch.NumberRecordsFailed))
	info.Set("totalProcessingTime", fmt.Sprintf("%d", batch.TotalProcessingTime))
	info.Set("apiActiveProcessingTime", fmt.Sprintf("%d", batch.ApiActiveProcessingTime))
	info.Set("apexProcessingTime", fmt.Sprintf("%d", batch.ApexProcessingTime))
	return info
}

func writeLegacyStatus(w io.Writer, title string, object *orderedMap) {
	fmt.Fprintln(w)
	styledHeader(w, title)
	styledObject(w, object)
}

// newLegacyBulkStatusCmd builds "force data bulk status" and "data resume",
// which behave the same.
func newLegacyBulkStatusCmd(use, short string) *cobra.Command {
	cmd := &cobra.Command{
		Use:         use,
		Short:       short,
		Args:        cobra.NoArgs,
		Annotations: map[string]string{commandNameAnnotation: "Status"},
		RunE: func(cmd *cobra.Command, args []string) error {
			jobID, _ := cmd.Flags().GetString("job-id")
			batchID, _ := cmd.Flags().GetString("batch-id")
			if jobID == "" {
				return missingFlagError("job-id")
			}
			org, err := resolveTargetOrg(cmd)
			if err != nil {
				return err
			}
			stop := startSpinner(cmd, "Getting Status")
			if batchID != "" {
				batches, err := org.Force.GetBatches(jobID)
				if err != nil {
					stop("failed")
					return apiError(err)
				}
				result := make([]*orderedMap, 0, len(batches))
				var found *orderedMap
				for _, batch := range batches {
					info := legacyBatchInfo(batch)
					result = append(result, info)
					if batch.Id == batchID {
						found = info
					}
				}
				if found == nil {
					stop("failed")
					return newError("NoBatchFound", "Unable to find batch %s for job %s.", batchID, jobID)
				}
				err = emit(cmd, result, func(w io.Writer) { writeLegacyStatus(w, "Batch Status", found) })
				stop("done")
				return err
			}
			job, err := org.Force.GetJobInfo(jobID)
			if err != nil {
				stop("failed")
				return apiError(err)
			}
			info := legacyJobInfo(job)
			err = emit(cmd, info, func(w io.Writer) { writeLegacyStatus(w, "Job Status", info) })
			stop("done")
			return err
		},
	}
	addJSONFlag(cmd)
	cmd.Flags().StringP("job-id", "i", "", "ID of the job you want to view or of the job whose batch you want to view.")
	cmd.Flags().StringP("batch-id", "b", "", "ID of the batch whose status you want to view; you must also specify the job ID.")
	addAPIVersionFlag(cmd)
	addTargetOrgFlag(cmd)
	return cmd
}

func newLegacyBulkIngestCmd(operation, short string) *cobra.Command {
	title := "Bulk " + strings.ToUpper(operation[:1]) + operation[1:]
	cmd := &cobra.Command{
		Use:         operation,
		Short:       short,
		Args:        cobra.NoArgs,
		Annotations: map[string]string{commandNameAnnotation: strings.ToUpper(operation[:1]) + operation[1:]},
		RunE: func(cmd *cobra.Command, args []string) error {
			file, _ := cmd.Flags().GetString("file")
			sobject, _ := cmd.Flags().GetString("sobject")
			if file == "" {
				return missingFlagError("file")
			}
			if sobject == "" {
				return missingFlagError("sobject")
			}
			job := lib.JobInfo{Operation: operation, Object: sobject, ContentType: "CSV", ConcurrencyMode: "Parallel"}
			if operation == "upsert" {
				externalID, _ := cmd.Flags().GetString("external-id")
				if externalID == "" {
					return missingFlagError("external-id")
				}
				job.ExternalIdFieldName = externalID
				if serial, _ := cmd.Flags().GetBool("serial"); serial {
					job.ConcurrencyMode = "Serial"
				}
			}
			data, err := os.ReadFile(file)
			if err != nil {
				return err
			}
			org, err := resolveTargetOrg(cmd)
			if err != nil {
				return err
			}
			stop := startSpinner(cmd, title)
			created, err := org.Force.CreateBulkJob(job)
			if err != nil {
				stop("error")
				return apiError(err)
			}
			batch, err := org.Force.AddBatchToJob(string(data), created)
			if err != nil {
				stop("error")
				return apiError(err)
			}
			if _, err := org.Force.CloseBulkJob(created.Id); err != nil {
				stop("error")
				return apiError(err)
			}
			out := cmd.OutOrStdout()
			if !cmd.Flags().Changed("wait") {
				stop("done")
				return emit(cmd, []*orderedMap{legacyBatchInfo(batch)}, func(w io.Writer) {
					fmt.Fprintf(w, "Check batch #1’s status with the command:\nsf force data bulk status -i %s -b %s\n", created.Id, batch.Id)
				})
			}
			minutes, _ := cmd.Flags().GetInt("wait")
			deadline := time.Now().Add(time.Duration(minutes) * time.Minute)
			if !jsonRequested(cmd) {
				fmt.Fprintf(out, "Will poll the batch statuses every 5 seconds.\nTo fetch the status on your own, press CTRL+C and use the command:\nsf force data bulk status -i %s -b %s\n", created.Id, batch.Id)
				fmt.Fprintf(out, "Batch #1 queued (Batch ID: %s). \n\n", batch.Id)
			}
			sleep := pollDelays()
			var current lib.BatchInfo
			for {
				current, err = org.Force.GetBatchInfo(created.Id, batch.Id)
				if err != nil {
					stop("error")
					return apiError(err)
				}
				if current.State == "Completed" || current.State == "Failed" || current.State == "NotProcessed" {
					break
				}
				if time.Now().After(deadline) {
					stop("done")
					message := fmt.Sprintf("The operation timed out. Check the status with command:\nsf force data bulk status -i %s -b %s", created.Id, batch.Id)
					return emit(cmd, []*orderedMap{legacyBatchInfo(current)}, func(w io.Writer) {
						fmt.Fprintln(w)
						fmt.Fprintln(w, message)
					})
				}
				sleep()
			}
			finished, err := org.Force.GetJobInfo(created.Id)
			if err != nil {
				stop("error")
				return apiError(err)
			}
			// The errors of the records of the batch are listed before
			// its status.
			var recordErrors []string
			if results, resultErr := org.Force.RetrieveBulkQuery(created.Id, batch.Id); resultErr == nil {
				reader := csv.NewReader(bytes.NewReader(results))
				reader.FieldsPerRecord = -1
				if lines, csvErr := reader.ReadAll(); csvErr == nil && len(lines) > 0 {
					column := -1
					for i, name := range lines[0] {
						if name == "Error" {
							column = i
						}
					}
					for _, line := range lines[1:] {
						if column >= 0 && column < len(line) && line[column] != "" {
							recordErrors = append(recordErrors, line[column])
						}
					}
				}
			}
			info := legacyJobInfo(finished)
			err = emit(cmd, []*orderedMap{info}, func(w io.Writer) {
				fmt.Fprintln(w)
				styledHeader(w, "Batch #1")
				if len(recordErrors) > 0 {
					styledHeader(w, "Upsert errors")
					for _, message := range recordErrors {
						fmt.Fprintln(w, message)
					}
				}
				styledHeader(w, "Batch Status")
				styledObject(w, legacyBatchInfo(current))
				writeLegacyStatus(w, "Job Status", info)
			})
			stop("done")
			return err
		},
	}
	addJSONFlag(cmd)
	cmd.Flags().StringP("file", "f", "", "CSV file that contains the records.")
	cmd.Flags().StringP("sobject", "s", "", "API name of the Salesforce object, either standard or custom.")
	cmd.Flags().IntP("wait", "w", 0, "Number of minutes to wait for the command to complete before displaying the results.")
	if operation == "upsert" {
		cmd.Flags().StringP("external-id", "i", "", "ID of the external ID field.")
		cmd.Flags().BoolP("serial", "r", false, "Run batches in serial mode.")
	}
	addAPIVersionFlag(cmd)
	addTargetOrgFlag(cmd)
	return cmd
}

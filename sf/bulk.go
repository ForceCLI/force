package sf

import (
	"bytes"
	"encoding/csv"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"

	"github.com/mattn/go-runewidth"
	"github.com/spf13/cobra"

	"github.com/ForceCLI/force/lib"
)

// bulkPollInterval is the longest time between checks of a bulk job.  The
// first check comes after a shorter time, which doubles on each check.  Tests
// shorten it.
var bulkPollInterval = 2 * time.Second

// pollDelays returns a function that sleeps between checks of a job.
func pollDelays() func() {
	delay := bulkPollInterval / 8
	return func() {
		time.Sleep(delay)
		if delay *= 2; delay > bulkPollInterval {
			delay = bulkPollInterval
		}
	}
}

// bulkOperation describes one of the Bulk API 2.0 ingest commands.
type bulkOperation struct {
	name      string // word in the command id: import, update, upsert, delete
	title     string // heading sf prints
	operation lib.Bulk2Operation
	className string
}

var bulkOperations = map[string]bulkOperation{
	"import": {"import", "Importing data", lib.Bulk2OperationInsert, "DataImportBulk"},
	"update": {"update", "Updating data", lib.Bulk2OperationUpdate, "DataUpdateBulk"},
	"upsert": {"upsert", "Upserting data", lib.Bulk2OperationUpsert, "Upsert"},
	"delete": {"delete", "Deleting data", lib.Bulk2OperationDelete, "Delete"},
}

// bulkCacheEntry is one entry of the files in ~/.sf that sf uses to find
// the org of a bulk job when a resume command is run.
type bulkCacheEntry struct {
	JobId      string          `json:"jobId"`
	OutputInfo *bulkOutputInfo `json:"outputInfo,omitempty"`
	Username   string          `json:"username"`
	ApiVersion string          `json:"apiVersion"`
	Timestamp  string          `json:"timestamp"`
}

type bulkOutputInfo struct {
	FilePath        string `json:"filePath"`
	Format          string `json:"format"`
	ColumnDelimiter string `json:"columnDelimiter"`
}

func bulkCachePath(name string) (string, error) {
	home, err := os.UserHomeDir()
	if err != nil {
		return "", err
	}
	return filepath.Join(home, ".sf", "bulk-data-"+name+"-cache.json"), nil
}

func readBulkCache(name string) (map[string]bulkCacheEntry, error) {
	path, err := bulkCachePath(name)
	if err != nil {
		return nil, err
	}
	entries := map[string]bulkCacheEntry{}
	data, err := os.ReadFile(path)
	if err != nil {
		if os.IsNotExist(err) {
			return entries, nil
		}
		return nil, err
	}
	if err := json.Unmarshal(data, &entries); err != nil {
		return nil, err
	}
	return entries, nil
}

func writeBulkCacheEntry(name string, entry bulkCacheEntry) error {
	entries, err := readBulkCache(name)
	if err != nil {
		return err
	}
	entry.Timestamp = time.Now().UTC().Format("2006-01-02T15:04:05.000Z")
	entries[entry.JobId] = entry
	path, err := bulkCachePath(name)
	if err != nil {
		return err
	}
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return err
	}
	data, err := json.MarshalIndent(entries, "", "  ")
	if err != nil {
		return err
	}
	return os.WriteFile(path, data, 0o600)
}

// resumeEntry finds the cache entry for --job-id or, with
// --use-most-recent, the newest entry.
func resumeEntry(cmd *cobra.Command, name string) (bulkCacheEntry, error) {
	jobID, _ := cmd.Flags().GetString("job-id")
	mostRecent, _ := cmd.Flags().GetBool("use-most-recent")
	if (jobID == "") == !mostRecent {
		return bulkCacheEntry{}, exactlyOneError("use-most-recent", "job-id")
	}
	entries, err := readBulkCache(name)
	if err != nil {
		return bulkCacheEntry{}, err
	}
	if mostRecent {
		ids := make([]string, 0, len(entries))
		for id := range entries {
			ids = append(ids, id)
		}
		if len(ids) == 0 {
			return bulkCacheEntry{}, newError("MissingCacheEntryError", "Could not load a most recent cache entry for a bulk request. Please rerun your command with a bulk request id.")
		}
		sort.Slice(ids, func(i, j int) bool { return entries[ids[i]].Timestamp > entries[ids[j]].Timestamp })
		return entries[ids[0]], nil
	}
	entry, ok := entries[jobID]
	if !ok {
		return bulkCacheEntry{}, newError("BulkRequestIdNotFoundError", "Could not find a cache entry for job ID %s.", jobID)
	}
	return entry, nil
}

// bulkStage is one line of the progress display.
type bulkStage struct {
	label   string
	state   string // "" pending, "done", "failed"
	elapsed time.Duration
}

// writeBulkProgress prints the last frame of the progress display sf shows
// for bulk commands.  sf redraws the frame while it waits; this prints it
// once.
func writeBulkProgress(w io.Writer, title string, stages []bulkStage, counts []string, status, jobID, jobURL string, elapsed time.Duration, hiddenURLMarks bool) {
	const width = 50
	side := (width - len([]rune(title)) - 2) / 2
	rule := strings.Repeat("─", side)
	fmt.Fprintf(w, "\n %s %s %s\n\n", rule, title, rule)
	for i, stage := range stages {
		switch stage.state {
		case "done":
			fmt.Fprintf(w, " ✔ %s %dms\n", stage.label, stage.elapsed.Milliseconds())
		case "failed":
			fmt.Fprintf(w, " ✘ %s %dms\n", stage.label, stage.elapsed.Milliseconds())
		case "skipped":
			fmt.Fprintf(w, " ◯ %s - Skipped\n", stage.label)
		default:
			fmt.Fprintf(w, " ◼ %s\n", stage.label)
		}
		if i == 1 && stage.state != "" && counts != nil {
			for _, count := range counts {
				fmt.Fprintf(w, "   ▸ %s\n", count)
			}
		}
	}
	fmt.Fprintf(w, "\n Status: %s\n", status)
	if jobID == "" {
		fmt.Fprintf(w, " Elapsed Time: %dms\n\n", elapsed.Milliseconds())
		return
	}
	job := fmt.Sprintf("%s (%s)", jobID, jobURL)
	if hiddenURLMarks {
		// sf surrounds the URL with zero-width spaces in export output.
		job = fmt.Sprintf("%s (​%s​)", jobID, jobURL)
	}
	// sf wraps lines at 80 columns when the output is not a terminal.
	if line := " Job Id: " + job; runewidth.StringWidth(line) <= 80 {
		fmt.Fprintln(w, line)
	} else {
		fmt.Fprintln(w, " Job Id:")
		var line strings.Builder
		width := 0
		for _, char := range job {
			charWidth := runewidth.RuneWidth(char)
			if width+charWidth > 79 {
				fmt.Fprintf(w, " %s\n", line.String())
				line.Reset()
				width = 0
			}
			line.WriteRune(char)
			width += charWidth
		}
		if line.Len() > 0 {
			fmt.Fprintf(w, " %s\n", line.String())
		}
	}
	fmt.Fprintf(w, " Elapsed Time: %dms\n\n", elapsed.Milliseconds())
}

func bulkJobURL(org *targetOrg, jobID string) string {
	return org.Fields.String("instanceUrl") + "/lightning/setup/AsyncApiJobStatus/page?address=%2F" + jobID
}

func bulkJobActions(org *targetOrg, jobID string, withResults bool) []string {
	var actions []string
	if withResults {
		actions = append(actions, fmt.Sprintf("Get the job results by running: \""+binName+" data bulk results -o %s --job-id %s\".", org.Username, jobID))
	}
	return append(actions, fmt.Sprintf("View the job in the org: \""+binName+" org open -o %s --path '/lightning/setup/AsyncApiJobStatus/page?address=%%2F%s'\".", org.Username, jobID))
}

// bulkWait reads --wait.  ok is false when the flag was not given, which
// makes the command return without waiting for the job.
func bulkWait(cmd *cobra.Command) (time.Duration, bool) {
	if !cmd.Flags().Changed("wait") {
		return 0, false
	}
	minutes, _ := cmd.Flags().GetInt("wait")
	return time.Duration(minutes) * time.Minute, true
}

// waitForIngestJob polls an ingest job until it reaches a final state or
// the timeout passes.
func waitForIngestJob(org *targetOrg, jobID string, timeout time.Duration) (lib.Bulk2IngestJobInfo, bool, error) {
	deadline := time.Now().Add(timeout)
	sleep := pollDelays()
	for {
		info, err := org.Force.GetBulk2IngestJobInfo(jobID)
		if err != nil {
			return info, false, apiError(err)
		}
		if info.IsTerminal() {
			return info, true, nil
		}
		if time.Now().After(deadline) {
			return info, false, nil
		}
		sleep()
	}
}

// finishIngestJob waits for an ingest job and prints the outcome.  It is
// used by the bulk commands and by their resume commands.
func finishIngestJob(cmd *cobra.Command, org *targetOrg, operation bulkOperation, jobID string, timeout time.Duration, started time.Time, createTime time.Duration, resumed ...bool) error {
	processStart := time.Now()
	info, finished, err := waitForIngestJob(org, jobID, timeout)
	if err != nil {
		return err
	}
	processTime := time.Since(processStart)
	out := cmd.OutOrStdout()
	stages := []bulkStage{{"Creating ingest job", "done", createTime}, {"Processing the job", "done", processTime}}
	title := operation.title
	if len(resumed) > 0 && resumed[0] {
		// A resumed job was created by another command.  sf titles the
		// display of "data import resume" as an update.
		stages[0].state = "skipped"
		if operation.name == "import" {
			title = "Updating data"
		}
	}
	frame := func(counts []string) {
		if !jsonRequested(cmd) {
			writeBulkProgress(out, title, stages, counts, string(info.State), jobID, bulkJobURL(org, jobID), time.Since(started), false)
		}
	}
	failedCounts := []string{"Processed records: ✘", "Successful records: ✘", "Failed records: ✘"}
	if !finished {
		stages[1].state = "failed"
		frame(failedCounts)
		return &Error{
			Name:     "TimeoutError",
			Message:  fmt.Sprintf("The operation timed out after %d minutes.\nRun \""+binName+" data %s resume --job-id %s\" to resume it.", int(timeout.Minutes()), operation.name, jobID),
			ExitCode: 1,
		}
	}
	switch info.State {
	case lib.Bulk2JobStateFailed:
		stages[1].state = "failed"
		frame(failedCounts)
		return &Error{Name: "JobFailedError", Message: fmt.Sprintf("Job failed to be processed due to:\n%s", info.ErrorMessage), ExitCode: 1, Actions: bulkJobActions(org, jobID, true)}
	case lib.Bulk2JobStateAborted:
		stages[1].state = "failed"
		frame(failedCounts)
		return &Error{Name: "JobAbortedError", Message: "Job has been aborted.", ExitCode: 1, Actions: bulkJobActions(org, jobID, true)}
	}
	if info.NumberRecordsProcessed == 0 {
		stages[1].state = "failed"
		frame(failedCounts)
		return &Error{
			Name:     "NoProcessedRecordsError",
			Message:  "Job finished successfully but it didn't process any record.",
			ExitCode: 1,
			Actions:  append([]string{"Check that the provided CSV file is valid."}, bulkJobActions(org, jobID, false)...),
		}
	}
	successful := info.NumberRecordsProcessed - info.NumberRecordsFailed
	counts := []string{
		fmt.Sprintf("Processed records: %d", info.NumberRecordsProcessed),
		fmt.Sprintf("Successful records: %d", successful),
		fmt.Sprintf("Failed records: %d", info.NumberRecordsFailed),
	}
	if info.NumberRecordsFailed > 0 {
		stages[1].state = "failed"
		frame(counts)
		return &Error{
			Name:     "FailedRecordDetailsError",
			Message:  fmt.Sprintf("Job finished being processed but failed to process %d records.", info.NumberRecordsFailed),
			ExitCode: 1,
			Actions:  bulkJobActions(org, jobID, true),
			Data:     orderedFields("jobId", jobID, "state", string(info.State)),
		}
	}
	frame(counts)
	if operation.name == "upsert" || operation.name == "delete" {
		result, err := bulkJobResult(org, jobID)
		if err != nil {
			return err
		}
		return emit(cmd, result, nil)
	}
	result := newOrderedMap()
	result.Set("jobId", jobID)
	result.Set("processedRecords", info.NumberRecordsProcessed)
	result.Set("successfulRecords", successful)
	result.Set("failedRecords", info.NumberRecordsFailed)
	return emit(cmd, result, nil)
}

// bulkJobResult returns the result the upsert and delete commands of sf
// report: the job as the org describes it and, for a job that is
// complete, its records by outcome.
func bulkJobResult(org *targetOrg, jobID string) (*orderedMap, error) {
	body, err := org.Force.GetREST("/jobs/ingest/" + jobID)
	if err != nil {
		return nil, apiError(err)
	}
	jobInfo, err := parseOrderedJSON([]byte(body))
	if err != nil {
		return nil, err
	}
	// A whole number such as the API version is printed without a
	// fraction, as JavaScript prints it.
	if info, ok := jobInfo.(*orderedMap); ok {
		for _, key := range info.keys {
			if number, isNumber := info.values[key].(json.Number); isNumber && strings.HasSuffix(string(number), ".0") {
				info.values[key] = json.Number(strings.TrimSuffix(string(number), ".0"))
			}
		}
	}
	records := newOrderedMap()
	fetches := []struct {
		key   string
		fetch func(string) ([]byte, error)
	}{
		{"successfulResults", org.Force.GetBulk2SuccessfulResults},
		{"failedResults", org.Force.GetBulk2FailedResults},
		{"unprocessedRecords", org.Force.GetBulk2UnprocessedRecords},
	}
	complete := false
	if info, ok := jobInfo.(*orderedMap); ok {
		complete = info.values["state"] == "JobComplete"
	}
	for _, f := range fetches {
		rows := []interface{}{}
		if complete {
			data, err := f.fetch(jobID)
			if err != nil {
				return nil, apiError(err)
			}
			reader := csv.NewReader(bytes.NewReader(data))
			reader.FieldsPerRecord = -1
			lines, err := reader.ReadAll()
			if err != nil {
				return nil, err
			}
			for i, line := range lines {
				if i == 0 {
					continue
				}
				row := newOrderedMap()
				for column, value := range line {
					if column < len(lines[0]) {
						row.Set(lines[0][column], value)
					}
				}
				rows = append(rows, row)
			}
		}
		records.Set(f.key, rows)
	}
	return orderedFields("jobInfo", jobInfo, "records", records), nil
}

var bulkDelimiters = map[string]lib.Bulk2ColumnDelimiter{
	"BACKQUOTE": lib.Bulk2DelimiterBackquote, "CARET": lib.Bulk2DelimiterCaret, "COMMA": lib.Bulk2DelimiterComma,
	"PIPE": lib.Bulk2DelimiterPipe, "SEMICOLON": lib.Bulk2DelimiterSemicolon, "TAB": lib.Bulk2DelimiterTab,
}

func bulkFormatFlags(cmd *cobra.Command) (lib.Bulk2ColumnDelimiter, lib.Bulk2LineEnding, error) {
	delimiter, _ := cmd.Flags().GetString("column-delimiter")
	lineEnding, _ := cmd.Flags().GetString("line-ending")
	var columnDelimiter lib.Bulk2ColumnDelimiter
	if delimiter != "" {
		known, ok := bulkDelimiters[delimiter]
		if !ok {
			return "", "", &Error{Message: fmt.Sprintf("Expected --column-delimiter=%s to be one of: BACKQUOTE, CARET, COMMA, PIPE, SEMICOLON, TAB\nSee more help with --help", delimiter), ExitCode: 1}
		}
		columnDelimiter = known
	}
	if lineEnding != "" && lineEnding != "CRLF" && lineEnding != "LF" {
		return "", "", &Error{Message: fmt.Sprintf("Expected --line-ending=%s to be one of: CRLF, LF\nSee more help with --help", lineEnding), ExitCode: 1}
	}
	return columnDelimiter, lib.Bulk2LineEnding(lineEnding), nil
}

func newDataBulkIngestCmd(operation bulkOperation, short string) *cobra.Command {
	cmd := &cobra.Command{
		Use:         "bulk",
		Short:       short,
		Args:        cobra.NoArgs,
		Annotations: map[string]string{commandNameAnnotation: operation.className},
		RunE: func(cmd *cobra.Command, args []string) error {
			file, _ := cmd.Flags().GetString("file")
			sobject, _ := cmd.Flags().GetString("sobject")
			if file == "" {
				return missingFlagError("file")
			}
			if sobject == "" {
				return missingFlagError("sobject")
			}
			request := lib.Bulk2IngestJobRequest{Object: sobject, Operation: operation.operation}
			if operation.name == "upsert" {
				externalID, _ := cmd.Flags().GetString("external-id")
				if externalID == "" {
					return missingFlagError("external-id")
				}
				request.ExternalIdFieldName = externalID
			}
			if hardDelete, _ := cmd.Flags().GetBool("hard-delete"); hardDelete {
				request.Operation = lib.Bulk2OperationHardDelete
			}
			var err error
			request.ColumnDelimiter, request.LineEnding, err = bulkFormatFlags(cmd)
			if err != nil {
				return err
			}
			data, err := os.ReadFile(file)
			if err != nil {
				return err
			}
			org, err := resolveTargetOrg(cmd)
			if err != nil {
				return err
			}
			started := time.Now()
			job, err := org.Force.CreateBulk2IngestJob(request)
			if err != nil {
				return apiError(err)
			}
			if err := org.Force.UploadBulk2JobData(job.Id, bytes.NewReader(data)); err != nil {
				return apiError(err)
			}
			closed, err := org.Force.CloseBulk2IngestJob(job.Id)
			if err != nil {
				return apiError(err)
			}
			createTime := time.Since(started)
			timeout, wait := bulkWait(cmd)
			if !wait {
				if err := writeBulkCacheEntry(operation.name, bulkCacheEntry{JobId: job.Id, Username: org.Username, ApiVersion: lib.ApiVersionNumber()}); err != nil {
					return err
				}
				result := newOrderedMap()
				result.Set("jobId", job.Id)
				if operation.name == "upsert" || operation.name == "delete" {
					if result, err = bulkJobResult(org, job.Id); err != nil {
						return err
					}
				}
				return emit(cmd, result, func(w io.Writer) {
					stages := []bulkStage{{"Creating ingest job", "done", createTime}, {"Processing the job", "", 0}}
					writeBulkProgress(w, operation.title+" (async)", stages, nil, string(closed.State), job.Id, bulkJobURL(org, job.Id), time.Since(started), false)
					fmt.Fprintf(w, "Run \""+binName+" data %s resume --job-id %s\" to resume the operation.\n", operation.name, job.Id)
				})
			}
			return finishIngestJob(cmd, org, operation, job.Id, timeout, started, createTime)
		},
	}
	addJSONFlag(cmd)
	cmd.Flags().StringP("file", "f", "", "CSV file that contains the Salesforce object records.")
	cmd.Flags().StringP("sobject", "s", "", "API name of the Salesforce object, either standard or custom.")
	cmd.Flags().IntP("wait", "w", 0, "Time to wait for the command to finish, in minutes.")
	cmd.Flags().String("line-ending", "", "Line ending used in the CSV file.")
	cmd.Flags().String("column-delimiter", "", "Column delimiter used in the CSV file.")
	cmd.Flags().BoolP("async", "a", false, "Don't wait for the command to complete.")
	cmd.Flags().MarkHidden("async")
	if operation.name == "upsert" {
		cmd.Flags().StringP("external-id", "i", "", "Name of the field that identifies a record; \"Id\" for the record ID.")
	}
	if operation.name == "delete" {
		cmd.Flags().Bool("hard-delete", false, "Mark the records as immediately eligible for deletion by your org.")
	}
	addAPIVersionFlag(cmd)
	addTargetOrgFlag(cmd)
	return cmd
}

// resumeOrg returns the org recorded for a cached bulk job.
func resumeOrg(cmd *cobra.Command, entry bulkCacheEntry) (*targetOrg, error) {
	org, err := loadOrg(entry.Username)
	if err != nil {
		return nil, newError("NamedOrgNotFoundError", "No authorization information found for %s.", entry.Username)
	}
	version := entry.ApiVersion
	if flag, _ := cmd.Flags().GetString("api-version"); flag != "" {
		version = flag
	}
	if version != "" {
		if err := lib.SetApiVersion(version); err != nil {
			return nil, err
		}
		org.Force.Credentials.SessionOptions.ApiVersion = version
	}
	return org, nil
}

func addResumeFlags(cmd *cobra.Command) {
	addJSONFlag(cmd)
	cmd.Flags().StringP("job-id", "i", "", "Job ID of the bulk job.")
	cmd.Flags().Bool("use-most-recent", false, "Use the job ID of the bulk job that was most recently run.")
	cmd.Flags().IntP("wait", "w", 5, "Time to wait for the command to finish, in minutes.")
	addAPIVersionFlag(cmd)
}

func newDataBulkResumeCmd(operation bulkOperation, short string) *cobra.Command {
	cmd := &cobra.Command{
		Use:         "resume",
		Short:       short,
		Args:        cobra.NoArgs,
		Annotations: map[string]string{commandNameAnnotation: "Data" + strings.ToUpper(operation.name[:1]) + operation.name[1:] + "Resume"},
		RunE: func(cmd *cobra.Command, args []string) error {
			entry, err := resumeEntry(cmd, operation.name)
			if err != nil {
				return err
			}
			org, err := resumeOrg(cmd, entry)
			if err != nil {
				return err
			}
			minutes, _ := cmd.Flags().GetInt("wait")
			return finishIngestJob(cmd, org, operation, entry.JobId, time.Duration(minutes)*time.Minute, time.Now(), 0, true)
		},
	}
	addResumeFlags(cmd)
	return cmd
}

// csvToJSONLines converts CSV query results to the JSON array sf writes,
// with one record on each line.
func csvToJSONLines(data []byte, delimiter rune) ([]byte, int, error) {
	reader := csv.NewReader(bytes.NewReader(data))
	reader.Comma = delimiter
	reader.FieldsPerRecord = -1
	rows, err := reader.ReadAll()
	if err != nil {
		return nil, 0, err
	}
	var b bytes.Buffer
	b.WriteString("[\n")
	count := 0
	if len(rows) > 0 {
		headers := rows[0]
		for i, row := range rows[1:] {
			record := newOrderedMap()
			for column, header := range headers {
				value := ""
				if column < len(row) {
					value = row[column]
				}
				record.Set(header, value)
			}
			encoded, err := encodeJSON(record)
			if err != nil {
				return nil, 0, err
			}
			b.WriteString("  " + encoded)
			if i < len(rows)-2 {
				b.WriteString(",")
			}
			b.WriteString("\n")
			count++
		}
	}
	b.WriteString("]")
	return b.Bytes(), count, nil
}

var delimiterRunes = map[string]rune{"BACKQUOTE": '`', "CARET": '^', "COMMA": ',', "PIPE": '|', "SEMICOLON": ';', "TAB": '\t', "": ','}

// finishExportJob waits for a query job, writes the records to the output
// file and prints the outcome.
func finishExportJob(cmd *cobra.Command, org *targetOrg, jobID string, output bulkOutputInfo, timeout time.Duration, started time.Time, createTime time.Duration, resumed ...bool) error {
	out := cmd.OutOrStdout()
	stages := []bulkStage{{"creating query job", "done", createTime}, {"processing the job", "done", 0}, {"exporting records", "", 0}}
	resuming := len(resumed) > 0 && resumed[0]
	frame := func(status string) {
		if jsonRequested(cmd) {
			return
		}
		if resuming {
			// The display of the resume command has two stages and no
			// job id.
			check := bulkStage{"checking query job status", stages[1].state, stages[1].elapsed}
			writeBulkProgress(out, "Exporting data", []bulkStage{check, stages[2]}, nil, status, "", "", time.Since(started), true)
			return
		}
		writeBulkProgress(out, "Exporting data", stages, nil, status, jobID, bulkJobURL(org, jobID), time.Since(started), true)
	}
	processStart := time.Now()
	deadline := processStart.Add(timeout)
	var info lib.Bulk2QueryJobInfo
	sleep := pollDelays()
	for {
		var err error
		info, err = org.Force.GetBulk2QueryJobInfo(jobID)
		if err != nil {
			return apiError(err)
		}
		if info.IsTerminal() {
			break
		}
		if time.Now().After(deadline) {
			stages[1].state = "failed"
			stages[1].elapsed = time.Since(processStart)
			frame(string(info.State))
			return &Error{
				Name:     "TimeoutError",
				Message:  fmt.Sprintf("The operation timed out after %d minutes.\nRun \""+binName+" data export resume --job-id %s\" to get the latest status and results.", int(timeout.Minutes()), jobID),
				ExitCode: 1,
			}
		}
		sleep()
	}
	stages[1].elapsed = time.Since(processStart)
	if info.State != lib.Bulk2JobStateJobComplete {
		stages[1].state = "failed"
		frame(string(info.State))
		message := "Job has been aborted."
		if info.State == lib.Bulk2JobStateFailed {
			message = fmt.Sprintf("Job failed to be processed due to:\n%s", info.ErrorMessage)
		}
		return &Error{Name: "JobFailedError", Message: message, ExitCode: 1}
	}
	exportStart := time.Now()
	var data bytes.Buffer
	locator := ""
	first := true
	for {
		page, err := org.Force.GetBulk2QueryResults(jobID, locator, 0)
		if err != nil {
			return apiError(err)
		}
		chunk := page.Data
		if !first {
			// Later pages repeat the header line.
			if index := bytes.IndexByte(chunk, '\n'); index >= 0 {
				chunk = chunk[index+1:]
			}
		}
		data.Write(chunk)
		first = false
		if page.Locator == "" || page.Locator == "null" {
			break
		}
		locator = page.Locator
	}
	content := data.Bytes()
	total := 0
	if output.Format == "json" {
		converted, count, err := csvToJSONLines(content, delimiterRunes[output.ColumnDelimiter])
		if err != nil {
			return err
		}
		content, total = converted, count
	} else {
		total = bytes.Count(content, []byte("\n"))
		if len(content) > 0 && !bytes.HasSuffix(content, []byte("\n")) {
			total++
		}
		if total > 0 {
			total--
		}
	}
	if err := os.WriteFile(output.FilePath, content, 0o644); err != nil {
		return err
	}
	stages[2].state = "done"
	stages[2].elapsed = time.Since(exportStart)
	frame(string(info.State))
	result := newOrderedMap()
	result.Set("totalSize", total)
	result.Set("filePath", output.FilePath)
	return emit(cmd, result, func(w io.Writer) {
		fmt.Fprintf(w, "%d records written to %s\n", total, output.FilePath)
	})
}

func newDataExportBulkCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:         "bulk",
		Short:       "Bulk export records from an org into a file using a SOQL query. Uses Bulk API 2.0.",
		Args:        cobra.NoArgs,
		Annotations: map[string]string{commandNameAnnotation: "DataExportBulk"},
		RunE: func(cmd *cobra.Command, args []string) error {
			query, _ := cmd.Flags().GetString("query")
			queryFile, _ := cmd.Flags().GetString("query-file")
			if (query == "") == (queryFile == "") {
				return exactlyOneError("query", "query-file")
			}
			if queryFile != "" {
				content, err := os.ReadFile(queryFile)
				if err != nil {
					return err
				}
				query = strings.TrimSpace(string(content))
			}
			outputFile, _ := cmd.Flags().GetString("output-file")
			if outputFile == "" {
				return missingFlagError("output-file")
			}
			format, _ := cmd.Flags().GetString("result-format")
			if format != "csv" && format != "json" {
				return &Error{Message: fmt.Sprintf("Expected --result-format=%s to be one of: csv, json\nSee more help with --help", format), ExitCode: 1}
			}
			delimiter, lineEnding, err := bulkFormatFlags(cmd)
			if err != nil {
				return err
			}
			org, err := resolveTargetOrg(cmd)
			if err != nil {
				return err
			}
			request := lib.Bulk2QueryJobRequest{Operation: lib.Bulk2OperationQuery, Query: query, ColumnDelimiter: delimiter, LineEnding: lineEnding}
			if allRows, _ := cmd.Flags().GetBool("all-rows"); allRows {
				request.Operation = lib.Bulk2OperationQueryAll
			}
			started := time.Now()
			job, err := org.Force.CreateBulk2QueryJob(request)
			if err != nil {
				return apiError(err)
			}
			createTime := time.Since(started)
			output := bulkOutputInfo{FilePath: outputFile, Format: format, ColumnDelimiter: string(delimiter)}
			if output.ColumnDelimiter == "" {
				output.ColumnDelimiter = "COMMA"
			}
			timeout, wait := bulkWait(cmd)
			if !wait {
				if err := writeBulkCacheEntry("export", bulkCacheEntry{JobId: job.Id, OutputInfo: &output, Username: org.Username, ApiVersion: lib.ApiVersionNumber()}); err != nil {
					return err
				}
				result := newOrderedMap()
				result.Set("jobId", job.Id)
				result.Set("filePath", outputFile)
				return emit(cmd, result, func(w io.Writer) {
					// sf has no state of the job to show here and shows a
					// frame of its spinner in its place.
					stages := []bulkStage{{"creating query job", "done", createTime}, {"done", "done", 0}}
					writeBulkProgress(w, "Exporting data (async)", stages, nil, "⠁", job.Id, bulkJobURL(org, job.Id), time.Since(started), true)
					fmt.Fprintf(w, "Run \""+binName+" data export resume --job-id %s\" to get the latest status and results.\n", job.Id)
				})
			}
			return finishExportJob(cmd, org, job.Id, output, timeout, started, createTime)
		},
	}
	addJSONFlag(cmd)
	cmd.Flags().IntP("wait", "w", 0, "Time to wait for the command to finish, in minutes.")
	cmd.Flags().StringP("query", "q", "", "SOQL query to execute.")
	cmd.Flags().String("query-file", "", "File that contains the SOQL query.")
	cmd.Flags().Bool("all-rows", false, "Include records that have been soft-deleted due to a merge or delete.")
	cmd.Flags().String("output-file", "", "File where records are written.")
	cmd.Flags().StringP("result-format", "r", "csv", "Format to write the results.")
	cmd.Flags().String("column-delimiter", "", "Column delimiter to be used when writing CSV output.")
	cmd.Flags().String("line-ending", "", "Line ending to be used when writing CSV output.")
	addAPIVersionFlag(cmd)
	addTargetOrgFlag(cmd)
	return cmd
}

// exportResumeWait is the time data export resume waits for the job.
var exportResumeWait = 30 * time.Second

func newDataExportResumeCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:         "resume",
		Short:       "Resume a bulk export job that you previously started. Uses Bulk API 2.0.",
		Args:        cobra.NoArgs,
		Annotations: map[string]string{commandNameAnnotation: "DataExportResume"},
		RunE: func(cmd *cobra.Command, args []string) error {
			entry, err := resumeEntry(cmd, "export")
			if err != nil {
				return err
			}
			if entry.OutputInfo == nil {
				return newError("BulkRequestIdNotFoundError", "The cache entry for job ID %s has no output file.", entry.JobId)
			}
			org, err := resumeOrg(cmd, entry)
			if err != nil {
				return err
			}
			// sf waits 30 seconds for the job.
			return finishExportJob(cmd, org, entry.JobId, *entry.OutputInfo, exportResumeWait, time.Now(), 0, true)
		},
	}
	addJSONFlag(cmd)
	cmd.Flags().StringP("job-id", "i", "", "Job ID of the bulk export.")
	cmd.Flags().Bool("use-most-recent", false, "Use the job ID of the bulk export job that was most recently run.")
	addAPIVersionFlag(cmd)
	return cmd
}

func newDataBulkResultsCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:         "results",
		Short:       "Get the results of a bulk ingest job that you previously ran.",
		Args:        cobra.NoArgs,
		Annotations: map[string]string{commandNameAnnotation: "DataBulkResults"},
		RunE: func(cmd *cobra.Command, args []string) error {
			jobID, _ := cmd.Flags().GetString("job-id")
			if jobID == "" {
				return missingFlagError("job-id")
			}
			org, err := resolveTargetOrg(cmd)
			if err != nil {
				return err
			}
			info, err := org.Force.GetBulk2IngestJobInfo(jobID)
			if err != nil {
				converted := apiError(err)
				if e, ok := converted.(*Error); ok && e.Message == "The requested resource does not exist" {
					return &Error{
						Name:     "InvalidIdError",
						Message:  fmt.Sprintf("Can't find a bulk job with ID %s.", jobID),
						ExitCode: 1,
						// The actions of sf are one text of several paragraphs.
						Actions: []string{
							"- Ensure the ID is from a previously run bulk ingest job, and not a query job.\n\n" +
								"- Run this command and verify that the job ID for the bulk command exists in your org:\n\n" +
								fmt.Sprintf(binName+" org open --target-org %s --path \"/lightning/setup/AsyncApiJobStatus/page?address=%%2F750\"", org.Username),
						},
					}
				}
				return converted
			}
			out := cmd.OutOrStdout()
			log := func(format string, values ...interface{}) {
				if !jsonRequested(cmd) {
					fmt.Fprintf(out, format, values...)
				}
			}
			bold := func(value string) string { return paint(out, styleBold, value) }
			log("Status: %s\n", bold(string(info.State)))
			log("Operation: %s\n", bold(string(info.Operation)))
			log("Object: %s\n\n", bold(info.Object))
			if info.ErrorMessage != "" {
				warn(cmd, fmt.Sprintf("Job failed due to:\n%s\n", info.ErrorMessage))
			}
			if info.NumberRecordsProcessed == 0 {
				return newError("NoRecordsError", "Unable to get results because the job processed 0 records.")
			}
			successful := info.NumberRecordsProcessed - info.NumberRecordsFailed
			log("Processed records: %s\n", bold(fmt.Sprintf("%d", info.NumberRecordsProcessed)))
			if info.NumberRecordsFailed > 0 {
				log("Failed records: %s\n\n", bold(fmt.Sprintf("%d", info.NumberRecordsFailed)))
				if info.State == lib.Bulk2JobStateJobComplete {
					log("Successful records: %d\n\n", successful)
				}
			} else if info.State == lib.Bulk2JobStateJobComplete {
				log("Successful records: %s\n\n", bold(fmt.Sprintf("%d", info.NumberRecordsProcessed)))
			}
			switch info.State {
			case lib.Bulk2JobStateOpen, lib.Bulk2JobStateUploadComplete, lib.Bulk2JobStateInProgress:
				return newError("JobInProgressError", "Job hasn't finished being processed yet.")
			}
			result := newOrderedMap()
			result.Set("processedRecords", info.NumberRecordsProcessed)
			result.Set("successfulRecords", successful)
			result.Set("failedRecords", info.NumberRecordsFailed)
			result.Set("status", string(info.State))
			result.Set("operation", string(info.Operation))
			result.Set("object", info.Object)
			save := func(key, label, path string, fetch func(string) ([]byte, error)) error {
				data, err := fetch(jobID)
				if err != nil {
					return apiError(err)
				}
				if err := os.WriteFile(path, data, 0o644); err != nil {
					return err
				}
				log("Saved %s results to %s\n", label, bold(path))
				result.Set(key, path)
				return nil
			}
			if err := save("successFilePath", "successful", jobID+"-success-records.csv", org.Force.GetBulk2SuccessfulResults); err != nil {
				return err
			}
			if info.NumberRecordsFailed > 0 {
				if err := save("failedFilePath", "failed", jobID+"-failed-records.csv", org.Force.GetBulk2FailedResults); err != nil {
					return err
				}
			}
			if info.State != lib.Bulk2JobStateJobComplete {
				if err := save("unprocessedRecordsFilePath", "unprocessed", jobID+"-unprocessed-records.csv", org.Force.GetBulk2UnprocessedRecords); err != nil {
					return err
				}
			}
			return emit(cmd, result, nil)
		},
	}
	addJSONFlag(cmd)
	cmd.Flags().StringP("job-id", "i", "", "Job ID of the bulk job.")
	addAPIVersionFlag(cmd)
	addTargetOrgFlag(cmd)
	return cmd
}

package command

import (
	"fmt"
	"os"
	"strings"
	"time"

	"github.com/ForceCLI/force/bubbles"
	. "github.com/ForceCLI/force/error"
	. "github.com/ForceCLI/force/lib"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/spf13/cobra"
)

func init() {
	// Data loading commands
	ingestCmds := []*cobra.Command{bulk2InsertCmd, bulk2UpdateCmd, bulk2UpsertCmd, bulk2DeleteCmd, bulk2HardDeleteCmd}
	for _, cmd := range ingestCmds {
		cmd.Flags().BoolP("wait", "w", false, "Wait for job to complete")
		cmd.Flags().BoolP("interactive", "i", false, "Interactive mode (implies --wait)")
		cmd.Flags().String("delimiter", "COMMA", "Column delimiter (COMMA, TAB, PIPE, SEMICOLON, CARET, BACKQUOTE)")
		cmd.Flags().String("lineending", "LF", "Line ending (LF or CRLF)")
	}

	bulk2UpsertCmd.Flags().StringP("externalid", "e", "", "External ID field for upserting (required)")
	bulk2UpsertCmd.MarkFlagRequired("externalid")

	// Query command flags
	bulk2QueryCmd.Flags().BoolP("wait", "w", false, "Wait for job to complete")
	bulk2QueryCmd.Flags().BoolP("query-all", "A", false, "Include deleted and archived records")
	bulk2QueryCmd.Flags().String("delimiter", "COMMA", "Column delimiter for results (COMMA, TAB, PIPE, SEMICOLON, CARET, BACKQUOTE)")
	bulk2QueryCmd.Flags().String("lineending", "LF", "Line ending for results (LF or CRLF)")

	// Results command flags
	bulk2ResultsCmd.Flags().BoolP("successful", "s", false, "Show successful results only")
	bulk2ResultsCmd.Flags().BoolP("failed", "f", false, "Show failed results only")
	bulk2ResultsCmd.Flags().BoolP("unprocessed", "u", false, "Show unprocessed records only")

	// Jobs list command flags
	bulk2JobsCmd.Flags().BoolP("query", "q", false, "List query jobs instead of ingest jobs")

	// Add subcommands
	bulk2Cmd.AddCommand(bulk2InsertCmd)
	bulk2Cmd.AddCommand(bulk2UpdateCmd)
	bulk2Cmd.AddCommand(bulk2UpsertCmd)
	bulk2Cmd.AddCommand(bulk2DeleteCmd)
	bulk2Cmd.AddCommand(bulk2HardDeleteCmd)
	bulk2Cmd.AddCommand(bulk2QueryCmd)
	bulk2Cmd.AddCommand(bulk2JobCmd)
	bulk2Cmd.AddCommand(bulk2JobsCmd)
	bulk2Cmd.AddCommand(bulk2ResultsCmd)
	bulk2Cmd.AddCommand(bulk2AbortCmd)
	bulk2Cmd.AddCommand(bulk2DeleteJobCmd)

	RootCmd.AddCommand(bulk2Cmd)
}

var bulk2Cmd = &cobra.Command{
	Use:   "bulk2",
	Short: "Use Bulk API 2.0 for data loading and querying",
	Long:  "Bulk API 2.0 provides a REST-based interface for data loading and querying with automatic batch management.",
	Example: `
  force bulk2 insert Account accounts.csv --wait
  force bulk2 update Account updates.csv --wait
  force bulk2 upsert -e External_Id__c Account data.csv --wait
  force bulk2 delete Account deletes.csv --wait
  force bulk2 query "SELECT Id, Name FROM Account LIMIT 100" --wait
  force bulk2 job <jobId>
  force bulk2 jobs
  force bulk2 jobs --query
  force bulk2 results <jobId>
  force bulk2 results <jobId> --failed
  force bulk2 abort <jobId>
  force bulk2 delete-job <jobId>
`,
}

var bulk2InsertCmd = &cobra.Command{
	Use:   "insert <object> <file>",
	Short: "Insert records from CSV file using Bulk API 2.0",
	Args:  cobra.ExactArgs(2),
	Run:   runBulk2IngestCmd,
}

var bulk2UpdateCmd = &cobra.Command{
	Use:   "update <object> <file>",
	Short: "Update records from CSV file using Bulk API 2.0",
	Args:  cobra.ExactArgs(2),
	Run:   runBulk2IngestCmd,
}

var bulk2UpsertCmd = &cobra.Command{
	Use:   "upsert -e <External_Id_Field__c> <object> <file>",
	Short: "Upsert records from CSV file using Bulk API 2.0",
	Args:  cobra.ExactArgs(2),
	Run:   runBulk2IngestCmd,
}

var bulk2DeleteCmd = &cobra.Command{
	Use:   "delete <object> <file>",
	Short: "Delete records using Bulk API 2.0",
	Args:  cobra.ExactArgs(2),
	Run:   runBulk2IngestCmd,
}

var bulk2HardDeleteCmd = &cobra.Command{
	Use:   "hardDelete <object> <file>",
	Short: "Hard delete records using Bulk API 2.0",
	Args:  cobra.ExactArgs(2),
	Run:   runBulk2IngestCmd,
}

var bulk2QueryCmd = &cobra.Command{
	Use:   "query <soql>",
	Short: "Query records using Bulk API 2.0",
	Args:  cobra.ExactArgs(1),
	Run: func(cmd *cobra.Command, args []string) {
		soql := args[0]
		wait, _ := cmd.Flags().GetBool("wait")
		queryAll, _ := cmd.Flags().GetBool("query-all")
		delimiter, _ := cmd.Flags().GetString("delimiter")
		lineEnding, _ := cmd.Flags().GetString("lineending")

		operation := Bulk2OperationQuery
		if queryAll {
			operation = Bulk2OperationQueryAll
		}

		request := Bulk2QueryJobRequest{
			Operation:       operation,
			Query:           soql,
			ColumnDelimiter: Bulk2ColumnDelimiter(strings.ToUpper(delimiter)),
			LineEnding:      Bulk2LineEnding(strings.ToUpper(lineEnding)),
		}

		jobInfo, err := force.CreateBulk2QueryJob(request)
		if err != nil {
			ErrorAndExit("Failed to create query job: " + err.Error())
		}

		fmt.Fprintf(os.Stderr, "Query job created: %s\n", jobInfo.Id)

		if !wait {
			fmt.Fprintf(os.Stderr, "To check job status use:\n  force bulk2 job %s\n", jobInfo.Id)
			return
		}

		finalJobInfo := waitForBulk2QueryJob(jobInfo.Id)
		if finalJobInfo.State == Bulk2JobStateFailed {
			ErrorAndExit("Query job failed: " + finalJobInfo.ErrorMessage)
		}
		if finalJobInfo.State == Bulk2JobStateAborted {
			ErrorAndExit("Query job was aborted")
		}

		displayBulk2QueryResults(finalJobInfo.Id)
	},
}

var bulk2JobCmd = &cobra.Command{
	Use:   "job <jobId>",
	Short: "Show bulk job details",
	Args:  cobra.ExactArgs(1),
	Run: func(cmd *cobra.Command, args []string) {
		jobId := args[0]

		// Try ingest job first
		ingestInfo, err := force.GetBulk2IngestJobInfo(jobId)
		if err == nil {
			DisplayBulk2IngestJobInfo(ingestInfo, os.Stdout)
			return
		}

		// Try query job
		queryInfo, err := force.GetBulk2QueryJobInfo(jobId)
		if err == nil {
			DisplayBulk2QueryJobInfo(queryInfo, os.Stdout)
			return
		}

		ErrorAndExit("Job not found: " + jobId)
	},
}

var bulk2JobsCmd = &cobra.Command{
	Use:   "jobs",
	Short: "List bulk jobs",
	Run: func(cmd *cobra.Command, args []string) {
		isQuery, _ := cmd.Flags().GetBool("query")

		if isQuery {
			jobs, err := force.GetBulk2QueryJobs()
			if err != nil {
				ErrorAndExit("Failed to list query jobs: " + err.Error())
			}
			DisplayBulk2QueryJobList(jobs, os.Stdout)
		} else {
			jobs, err := force.GetBulk2IngestJobs()
			if err != nil {
				ErrorAndExit("Failed to list ingest jobs: " + err.Error())
			}
			DisplayBulk2IngestJobList(jobs, os.Stdout)
		}
	},
}

var bulk2ResultsCmd = &cobra.Command{
	Use:   "results <jobId>",
	Short: "Get job results",
	Args:  cobra.ExactArgs(1),
	Run: func(cmd *cobra.Command, args []string) {
		jobId := args[0]
		successful, _ := cmd.Flags().GetBool("successful")
		failed, _ := cmd.Flags().GetBool("failed")
		unprocessed, _ := cmd.Flags().GetBool("unprocessed")

		// If no specific flag set, try to determine job type and show appropriate results
		if !successful && !failed && !unprocessed {
			// Try ingest job - show all results
			_, err := force.GetBulk2IngestJobInfo(jobId)
			if err == nil {
				showBulk2IngestResults(jobId, true, true, true)
				return
			}

			// Try query job - show query results
			_, err = force.GetBulk2QueryJobInfo(jobId)
			if err == nil {
				displayBulk2QueryResults(jobId)
				return
			}

			ErrorAndExit("Job not found: " + jobId)
			return
		}

		// Show specific result types for ingest jobs
		showBulk2IngestResults(jobId, successful, failed, unprocessed)
	},
}

var bulk2AbortCmd = &cobra.Command{
	Use:   "abort <jobId>",
	Short: "Abort a bulk job",
	Args:  cobra.ExactArgs(1),
	Run: func(cmd *cobra.Command, args []string) {
		jobId := args[0]

		// Try ingest job first
		ingestInfo, err := force.AbortBulk2IngestJob(jobId)
		if err == nil {
			fmt.Fprintf(os.Stdout, "Job %s aborted\n", ingestInfo.Id)
			return
		}

		// Try query job
		queryInfo, err := force.AbortBulk2QueryJob(jobId)
		if err == nil {
			fmt.Fprintf(os.Stdout, "Job %s aborted\n", queryInfo.Id)
			return
		}

		ErrorAndExit("Failed to abort job: " + err.Error())
	},
}

var bulk2DeleteJobCmd = &cobra.Command{
	Use:   "delete-job <jobId>",
	Short: "Delete a bulk job",
	Args:  cobra.ExactArgs(1),
	Run: func(cmd *cobra.Command, args []string) {
		jobId := args[0]

		// Try ingest job first
		err := force.DeleteBulk2IngestJob(jobId)
		if err == nil {
			fmt.Fprintf(os.Stdout, "Job %s deleted\n", jobId)
			return
		}

		// Try query job
		err = force.DeleteBulk2QueryJob(jobId)
		if err == nil {
			fmt.Fprintf(os.Stdout, "Job %s deleted\n", jobId)
			return
		}

		ErrorAndExit("Failed to delete job: " + err.Error())
	},
}

func runBulk2IngestCmd(cmd *cobra.Command, args []string) {
	objectType := args[0]
	filePath := args[1]

	operation := Bulk2Operation(strings.ToLower(cmd.Name()))
	externalId := ""
	if operation == Bulk2OperationUpsert {
		externalId, _ = cmd.Flags().GetString("externalid")
	}

	wait, _ := cmd.Flags().GetBool("wait")
	interactive, _ := cmd.Flags().GetBool("interactive")
	delimiter, _ := cmd.Flags().GetString("delimiter")
	lineEnding, _ := cmd.Flags().GetString("lineending")

	if interactive {
		wait = true
	}

	// Open CSV file
	file, err := os.Open(filePath)
	if err != nil {
		ErrorAndExit("Failed to open file: " + err.Error())
	}
	defer file.Close()

	// Create job
	request := Bulk2IngestJobRequest{
		Object:              objectType,
		Operation:           operation,
		ExternalIdFieldName: externalId,
		ColumnDelimiter:     Bulk2ColumnDelimiter(strings.ToUpper(delimiter)),
		LineEnding:          Bulk2LineEnding(strings.ToUpper(lineEnding)),
	}

	jobInfo, err := force.CreateBulk2IngestJob(request)
	if err != nil {
		ErrorAndExit("Failed to create job: " + err.Error())
	}
	fmt.Fprintf(os.Stderr, "Job created: %s\n", jobInfo.Id)

	// Upload data
	fmt.Fprintf(os.Stderr, "Uploading data...\n")
	err = force.UploadBulk2JobData(jobInfo.Id, file)
	if err != nil {
		force.AbortBulk2IngestJob(jobInfo.Id)
		ErrorAndExit("Failed to upload data: " + err.Error())
	}

	// Close job to start processing
	jobInfo, err = force.CloseBulk2IngestJob(jobInfo.Id)
	if err != nil {
		force.AbortBulk2IngestJob(jobInfo.Id)
		ErrorAndExit("Failed to close job: " + err.Error())
	}
	fmt.Fprintf(os.Stderr, "Data uploaded. Job submitted for processing.\n")

	if !wait {
		fmt.Fprintf(os.Stderr, "To check job status use:\n  force bulk2 job %s\n", jobInfo.Id)
		fmt.Fprintf(os.Stderr, "To get results use:\n  force bulk2 results %s\n", jobInfo.Id)
		return
	}

	if interactive {
		startBulk2BubbleProgram(jobInfo)
	} else {
		waitForBulk2IngestJob(jobInfo.Id)
	}
}

func startBulk2BubbleProgram(jobInfo Bulk2IngestJobInfo) {
	d := bubbles.NewBulk2JobModel()
	p := tea.NewProgram(d, tea.WithOutput(os.Stderr))
	go func() {
		for {
			status, err := force.GetBulk2IngestJobInfo(jobInfo.Id)
			if err != nil {
				ErrorAndExit("Failed to get bulk job status: " + err.Error())
			}
			p.Send(bubbles.NewBulk2JobStatusMsg{Bulk2IngestJobInfo: status})
			if status.IsTerminal() {
				time.Sleep(500 * time.Millisecond)
				p.Send(bubbles.QuitMsg{})
				return
			}
			time.Sleep(2 * time.Second)
		}
	}()
	p.Run()
}

func waitForBulk2IngestJob(jobId string) Bulk2IngestJobInfo {
	for {
		status, err := force.GetBulk2IngestJobInfo(jobId)
		if err != nil {
			ErrorAndExit("Failed to get bulk job status: " + err.Error())
		}
		DisplayBulk2IngestJobInfo(status, os.Stderr)
		if status.IsTerminal() {
			return status
		}
		time.Sleep(2 * time.Second)
	}
}

func waitForBulk2QueryJob(jobId string) Bulk2QueryJobInfo {
	for {
		status, err := force.GetBulk2QueryJobInfo(jobId)
		if err != nil {
			ErrorAndExit("Failed to get bulk job status: " + err.Error())
		}
		DisplayBulk2QueryJobInfo(status, os.Stderr)
		if status.IsTerminal() {
			return status
		}
		time.Sleep(2 * time.Second)
	}
}

func showBulk2IngestResults(jobId string, successful, failed, unprocessed bool) {
	if successful {
		results, err := force.GetBulk2SuccessfulResults(jobId)
		if err != nil {
			fmt.Fprintf(os.Stderr, "Failed to get successful results: %s\n", err.Error())
		} else if len(results) > 0 {
			fmt.Println("=== Successful Results ===")
			fmt.Print(string(results))
		}
	}

	if failed {
		results, err := force.GetBulk2FailedResults(jobId)
		if err != nil {
			fmt.Fprintf(os.Stderr, "Failed to get failed results: %s\n", err.Error())
		} else if len(results) > 0 {
			fmt.Println("=== Failed Results ===")
			fmt.Print(string(results))
		}
	}

	if unprocessed {
		results, err := force.GetBulk2UnprocessedRecords(jobId)
		if err != nil {
			fmt.Fprintf(os.Stderr, "Failed to get unprocessed records: %s\n", err.Error())
		} else if len(results) > 0 {
			fmt.Println("=== Unprocessed Records ===")
			fmt.Print(string(results))
		}
	}
}

func displayBulk2QueryResults(jobId string) {
	locator := ""
	headerDisplayed := false
	for {
		results, err := force.GetBulk2QueryResults(jobId, locator, 0)
		if err != nil {
			ErrorAndExit("Failed to get query results: " + err.Error())
		}

		data := results.Data
		if headerDisplayed && len(data) > 0 {
			// Skip header row for subsequent pages
			data = stripFirstLine(data)
		}

		if len(data) > 0 {
			fmt.Print(string(data))
			headerDisplayed = true
		}

		if results.Locator == "" {
			break
		}
		locator = results.Locator
	}
}

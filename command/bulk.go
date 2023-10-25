package command

/*
bulk command
force bulk insert mydata.csv

The load process involves these steps
	1. Create a job
	https://instance_name—api.salesforce.com/services/async/APIversion/job
		payload:
			<jobInfo xmlns="http://www.force.com/2009/06/asyncapi/dataload">
 				<operation>insert</operation>
 				<object>Account</object>
 				<contentType>CSV</contentType>
			</jobInfo>
	2. Add batches to the created job
	https://instance_name—api.salesforce.com/services/async/APIversion/job/jobid/batch
		payload:
			<sObjects xmlns="http://www.force.com/2009/06/asyncapi/dataload">
  				<sObject>
    				<description>Created from Bulk API on Tue Apr 14 11:15:59 PDT 2009</description>
    				<name>[Bulk API] Account 0 (batch 0)</name>
  				</sObject>
  				<sObject>
    				<description>Created from Bulk API on Tue Apr 14 11:15:59 PDT 2009</description>
    				<name>[Bulk API] Account 1 (batch 0)</name>
  				</sObject>
			</sObjects>
	3. Close job (I assume this submits the job???)
	https://instance_name—api.salesforce.com/services/async/APIversion/job/jobId
		payload:
			<jobInfo xmlns="http://www.force.com/2009/06/asyncapi/dataload">
 				<state>Closed</state>
			</jobInfo>

Jobs and batches can be monitored.

bulk command
force bulk job <jobId>

bulk command
force bulk batches <jobId>

bulk command
force bulk batch <batchId>
*/

import (
	"bufio"
	"bytes"
	"encoding/csv"
	"encoding/xml"
	"fmt"
	"net/http"
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
	cmds := []*cobra.Command{bulkInsertCmd, bulkUpdateCmd, bulkUpsertCmd, bulkDeleteCmd, bulkHardDeleteCmd, bulkQueryCmd}
	for _, cmd := range cmds {
		cmd.Flags().StringP("format", "f", "CSV", "file `format`")
		cmd.Flags().StringP("concurrencymode", "m", "Parallel", "Concurrency `mode`.  Valid options are Serial and Parallel.")
		cmd.Flags().BoolP("wait", "w", false, "Wait for job to complete")
		cmd.Flags().BoolP("interactive", "i", false, "interactive mode.  implies --wait")
	}
	for _, cmd := range cmds[:len(cmds)-1] {
		cmd.Flags().IntP("batchsize", "b", 10000, "Batch size")
	}

	bulkUpsertCmd.Flags().StringP("externalid", "e", "", "The external Id field for upserting data")
	bulkUpsertCmd.MarkFlagRequired("externalid")

	bulkQueryCmd.Flags().IntP("chunk", "p", 0, "PK chunking size (number of `records`)")
	bulkQueryCmd.Flags().String("parent", "", "Parent `object` to use for PK chunking")

	// Start Bulk API Job
	bulkCmd.AddCommand(bulkInsertCmd)
	bulkCmd.AddCommand(bulkUpdateCmd)
	bulkCmd.AddCommand(bulkUpsertCmd)
	bulkCmd.AddCommand(bulkDeleteCmd)
	bulkCmd.AddCommand(bulkHardDeleteCmd)
	bulkCmd.AddCommand(bulkQueryCmd)

	// Get Bulk Job Status
	bulkCmd.AddCommand(bulkRetrieveCmd)
	bulkCmd.AddCommand(bulkJobCmd)
	bulkCmd.AddCommand(bulkWatchCmd)
	bulkCmd.AddCommand(bulkBatchCmd)
	bulkCmd.AddCommand(bulkBatchesCmd)

	RootCmd.AddCommand(bulkCmd)
}

var bulkInsertCmd = &cobra.Command{
	Use:   "insert <object> <file>",
	Short: "Create records from csv file using Bulk API",
	Run:   runBulkCmd,
	Args:  cobra.ExactArgs(2),
}

var bulkUpdateCmd = &cobra.Command{
	Use:   "update <object> <file>",
	Short: "Update records from csv file using Bulk API",
	Run:   runBulkCmd,
	Args:  cobra.ExactArgs(2),
}

var bulkUpsertCmd = &cobra.Command{
	Use:   "upsert -e <External_Id_Field__c> <object> <file>",
	Short: "Upsert records from csv file using Bulk API",
	Run:   runBulkCmd,
	Args:  cobra.ExactArgs(2),
}

var bulkDeleteCmd = &cobra.Command{
	Use:   "delete <object> <file>",
	Short: "Delete records using Bulk API",
	Run:   runBulkCmd,
	Args:  cobra.ExactArgs(2),
}

var bulkHardDeleteCmd = &cobra.Command{
	Use:   "hardDelete <object> <file>",
	Short: "Hard delete records using Bulk API",
	Run:   runBulkCmd,
	Args:  cobra.ExactArgs(2),
}

var bulkQueryCmd = &cobra.Command{
	Use:   "query <object> <query>",
	Short: "Query records using Bulk API",
	Run: func(cmd *cobra.Command, args []string) {
		objectType := args[0]
		query := args[1]
		format, _ := cmd.Flags().GetString("format")
		concurrencyMode, _ := cmd.Flags().GetString("concurrencymode")
		pkChunkSize, _ := cmd.Flags().GetInt("chunk")
		pkChunkParent, _ := cmd.Flags().GetString("parent")
		jobInfo, batchId := startBulkQuery(objectType, query, format, concurrencyMode, pkChunkSize, pkChunkParent)
		wait, _ := cmd.Flags().GetBool("wait")
		interactive, _ := cmd.Flags().GetBool("interactive")
		if interactive {
			wait = true
		}
		if !wait {
			fmt.Println("Query Submitted")
			if pkChunkSize == 0 {
				fmt.Printf("To retrieve batch status use\nforce bulk batch %s %s\n\n", jobInfo.Id, batchId)
				fmt.Printf("To retrieve query data use\nforce bulk retrieve %s %s\n\n", jobInfo.Id, batchId)
			} else {
				fmt.Printf("To retrieve batch status use\nforce bulk batches %s\n\n", jobInfo.Id)
			}
			return
		}
		if interactive {
			startBubbleProgram(jobInfo)
		} else {
			waitForJob(jobInfo)
		}
		displayQueryResults(jobInfo)
	},
	Args: cobra.ExactArgs(2),
}

var bulkRetrieveCmd = &cobra.Command{
	Use:   "retrieve <jobId> <batchId>",
	Short: "Retrieve query results using Bulk API",
	Run: func(cmd *cobra.Command, args []string) {
		fmt.Println(string(getBulkQueryResults(args[0], args[1])))
	},
	Args: cobra.ExactArgs(2),
}

var bulkJobCmd = &cobra.Command{
	Use:   "job <jobId>",
	Short: "Show bulk job details",
	Run: func(cmd *cobra.Command, args []string) {
		showJobDetails(args[0])
	},
	Args: cobra.ExactArgs(1),
}

var bulkWatchCmd = &cobra.Command{
	Use:   "watch <jobId>",
	Short: "Show bulk job details",
	Run: func(cmd *cobra.Command, args []string) {
		watchJob(args[0])
	},
	Args: cobra.ExactArgs(1),
}

var bulkBatchCmd = &cobra.Command{
	Use:   "batch <jobId> <batchId>",
	Short: "Show bulk job batch details",
	Run: func(cmd *cobra.Command, args []string) {
		DisplayBatchInfo(getBatchDetails(args[0], args[1]), os.Stdout)
	},
	Args: cobra.ExactArgs(2),
}

var bulkBatchesCmd = &cobra.Command{
	Use:   "batches <jobId>",
	Short: "List bulk job batches",
	Run: func(cmd *cobra.Command, args []string) {
		listBatches(args[0])
	},
	Args: cobra.ExactArgs(1),
}

var bulkCmd = &cobra.Command{
	Use:   "bulk",
	Short: "Load csv file or query data using Bulk API",
	Example: `
  force bulk insert Account [csv file]
  force bulk update Account [csv file]
  force bulk delete Account [csv file]
  force bulk upsert -e ExternalIdField__c Account [csv file]
  force bulk job [job id]
  force bulk batches [job id]
  force Bulk batch [job id] [batch id]
  force bulk query [-wait | -w] Account [SOQL]
  force bulk query [-chunk | -p]=50000 Account [SOQL]
  force bulk retrieve [job id] [batch id]
`,
}

func runBulkCmd(cmd *cobra.Command, args []string) {
	externalId := ""
	if cmd.Name() == "upsert" {
		externalId, _ = cmd.Flags().GetString("externalid")
	}

	objectType := args[0]
	file := args[1]
	format, _ := cmd.Flags().GetString("format")
	concurrencyMode, _ := cmd.Flags().GetString("concurrencymode")
	wait, _ := cmd.Flags().GetBool("wait")
	interactive, _ := cmd.Flags().GetBool("interactive")
	if interactive {
		wait = true
	}
	batchSize, _ := cmd.Flags().GetInt("batchsize")
	jobInfo, batchInfo := startBulkJob(cmd.Name(), file, objectType, externalId, format, concurrencyMode, batchSize)
	if !wait {
		fmt.Printf("Job created ( %s ) - for job status use\n force bulk batch %s %s\n", jobInfo.Id, jobInfo.Id, batchInfo.Id)
		return
	}
	if interactive {
		startBubbleProgram(jobInfo)
	} else {
		waitForJob(jobInfo)
	}
}

func startBubbleProgram(jobInfo JobInfo) {
	d := bubbles.NewJobModel()
	p := tea.NewProgram(d, tea.WithOutput(os.Stderr))
	go func() {
		for {
			status, err := force.GetJobInfo(jobInfo.Id)
			if err != nil {
				ErrorAndExit("Failed to get bulk job status: " + err.Error())
			}
			done := status.NumberBatchesCompleted+status.NumberBatchesFailed == status.NumberBatchesTotal
			p.Send(bubbles.NewJobStatusMsg{JobInfo: status})
			time.Sleep(2 * time.Second)
			if done {
				p.Send(bubbles.QuitMsg{})
			}
		}
	}()
	p.Run()
}

func waitForJob(jobInfo JobInfo) {
	for {
		status, err := force.GetJobInfo(jobInfo.Id)
		if err != nil {
			fmt.Fprintf(os.Stderr, "Failed to get bulk job status: %s\n", err.Error())
			os.Exit(1)
		}
		DisplayJobInfo(status, os.Stderr)
		if status.NumberBatchesCompleted+status.NumberBatchesFailed == status.NumberBatchesTotal {
			break
		}
		time.Sleep(2000 * time.Millisecond)
	}
}

func startBulkJob(jobType string, csvFilePath string, objectType string, externalId string, format string, concurrencyMode string, batchSize int) (JobInfo, BatchInfo) {
	jobInfo, err := createBulkJob(objectType, jobType, format, externalId, concurrencyMode, nil)
	if err != nil {
		ErrorAndExit(err.Error())
	}
	batchInfo, err := addBatchToJob(csvFilePath, jobInfo, batchSize)
	closeBulkJob(jobInfo.Id)
	if err != nil {
		ErrorAndExit(err.Error())
	}
	return jobInfo, batchInfo
}

func startBulkQuery(objectType string, soql string, contenttype string, concurrencyMode string, pkChunkSize int, pkChunkParent string) (JobInfo, string) {
	headers := make(map[string]string)
	var pkChunkOptions []string
	if pkChunkSize != 0 {
		pkChunkOptions = append(pkChunkOptions, fmt.Sprintf("chunkSize=%d", pkChunkSize))
	}
	if pkChunkParent != "" {
		pkChunkOptions = append(pkChunkOptions, fmt.Sprintf("parent=%s", pkChunkParent))
	}
	if len(pkChunkOptions) > 0 {
		headers["Sforce-Enable-PKChunking"] = strings.Join(pkChunkOptions, ";")
	}
	jobInfo, err := createBulkJob(objectType, "query", contenttype, "", concurrencyMode, headers)
	if err != nil {
		ErrorAndExit(err.Error())
	}

	result, err := force.BulkQuery(soql, jobInfo.Id, contenttype)
	batchId := result.Id
	if err != nil {
		closeBulkJob(jobInfo.Id)
		ErrorAndExit(err.Error())
	}
	// Wait for chunking to complete
	if pkChunkSize > 0 {
		for {
			batchInfo, err := force.GetBatchInfo(jobInfo.Id, batchId)
			if err != nil {
				fmt.Fprintf(os.Stderr, "Failed to get bulk batch status: %s\n", err.Error())
				os.Exit(1)
			}
			DisplayBatchInfo(batchInfo, os.Stderr)
			if batchInfo.State == "Failed" {
				ErrorAndExit(fmt.Sprintf("Job Failed: %s", batchInfo.StateMessage))
			}
			if batchInfo.State == "NotProcessed" {
				// batches have been created
				break
			}
			time.Sleep(2000 * time.Millisecond)
		}
	}

	closeBulkJob(jobInfo.Id)
	return jobInfo, batchId
}

func displayQueryResults(jobInfo JobInfo) {
	// Each result set in each batch will contain the header row.  Display
	// the header only once, for the first result set of the first (non-empty)
	// batch.
	headerDisplayed := false
	for _, batchInfo := range getBatches(jobInfo.Id) {
		if batchInfo.State == "Failed" {
			fmt.Fprintf(os.Stderr, "Batch failed: %s\n", batchInfo.StateMessage)
			os.Exit(1)
		}
		if batchInfo.NumberRecordsProcessed == 0 {
			// With PK Chunking and Parent Object, there may be batches with a
			// result set, but no records.  Skip these batches.
			continue
		}
		results := getBulkQueryResults(jobInfo.Id, batchInfo.Id)
		if len(results) == 0 {
			continue
		}
		if headerDisplayed && strings.ToUpper(jobInfo.ContentType) == "CSV" {
			results = stripFirstLine(results)
		}
		headerDisplayed = true
		fmt.Print(string(results))
	}
}

func stripFirstLine(data []byte) []byte {
	newLineAt := bytes.IndexByte(data, '\n')
	var returnFrom int
	if newLineAt < 0 {
		returnFrom = len(data)
	} else {
		returnFrom = newLineAt + 1
	}
	return data[returnFrom:]
}

func getBulkQueryResults(jobId string, batchId string) (data []byte) {
	resultIds := retrieveBulkQuery(jobId, batchId)

	for row, resultId := range resultIds {
		var newData []byte = retrieveBulkQueryResults(jobId, batchId, resultId)
		if row > 0 {
			newData = stripFirstLine(newData)
		}
		data = append(data[:], newData...)
	}

	return
}

func retrieveBulkQuery(jobId string, batchId string) (resultIds []string) {
	jobInfo, err := force.RetrieveBulkQuery(jobId, batchId)
	if err != nil {
		ErrorAndExit(err.Error())
	}

	var resultList struct {
		Results []string `xml:"result"`
	}

	xml.Unmarshal(jobInfo, &resultList)
	resultIds = resultList.Results
	return
}

func retrieveBulkQueryResults(jobId string, batchId string, resultId string) (data []byte) {
	data, err := force.RetrieveBulkQueryResults(jobId, batchId, resultId)
	if err != nil {
		ErrorAndExit(err.Error())
	}
	return
}

func showJobDetails(jobId string) {
	jobInfo := getJobDetails(jobId)
	DisplayJobInfo(jobInfo, os.Stdout)
}

func watchJob(jobId string) {
	jobInfo := getJobDetails(jobId)
	startBubbleProgram(jobInfo)
}

func listBatches(jobId string) {
	batchInfos := getBatches(jobId)
	DisplayBatchList(batchInfos)
}

func getJobDetails(jobId string) (jobInfo JobInfo) {
	jobInfo, err := force.GetJobInfo(jobId)

	if err != nil {
		ErrorAndExit(err.Error())
	}
	return
}

func getBatches(jobId string) (batchInfos []BatchInfo) {
	batchInfos, err := force.GetBatches(jobId)

	if err != nil {
		ErrorAndExit(err.Error())
	}
	return
}

func getBatchDetails(jobId string, batchId string) (batchInfo BatchInfo) {
	batchInfo, err := force.GetBatchInfo(jobId, batchId)

	if err != nil {
		ErrorAndExit(err.Error())
	}
	return
}

func addBatchToJob(csvFilePath string, job JobInfo, batchSize int) (result BatchInfo, err error) {
	batches, err := SplitCSV(csvFilePath, batchSize)
	if err != nil {
		return
	}
	for b := range batches {
		result, err = force.AddBatchToJob(batches[b], job)
		if err != nil {
			break
		} else {
			fmt.Printf("Batch %d of %d added with Id %s \n", b+1, len(batches), result.Id)
		}
	}
	return
}

func SplitCSV(csvFilePath string, batchsize int) (batches []string, err error) {
	if batchsize <= 0 {
		return nil, fmt.Errorf("Invalid batch size.  Must be greater than zero.")
	}
	f, err := os.Open(csvFilePath)
	if err != nil {
		return
	}
	r := csv.NewReader(bufio.NewReader(f))
	filedata, err := r.ReadAll()
	if err != nil {
		return
	}

	batches = splitFileIntoBatches(filedata, batchsize)
	return
}

func splitFileIntoBatches(rows [][]string, batchsize int) (batches []string) {
	headerRow, rows := rows[0], rows[1:]
	for len(rows) > 0 {
		if len(rows) < batchsize {
			batchsize = len(rows)
		}
		buf := new(bytes.Buffer)
		w := csv.NewWriter(buf)
		w.Write(headerRow)
		w.WriteAll(rows[0:batchsize])
		batch := buf.String()
		batches = append(batches, batch)
		rows = rows[batchsize:]
	}
	return
}

func createBulkJob(objectType string, operation string, fileFormat string, externalId string, concurrencyMode string, jobHeaders map[string]string) (jobInfo JobInfo, err error) {
	if !(strings.EqualFold(concurrencyMode, "serial")) {
		if !(strings.EqualFold(concurrencyMode, "parallel")) {
			ErrorAndExit("Concurrency Mode must be set to either Serial or Parallel")
		}
	}

	job := JobInfo{
		Operation:   operation,
		Object:      objectType,
		ContentType: fileFormat,
	}

	if strings.EqualFold(concurrencyMode, "serial") {
		job.ConcurrencyMode = "Serial"
	}

	if operation == "upsert" {
		job.ExternalIdFieldName = externalId
	}

	var options []func(*http.Request)
	if len(jobHeaders) > 0 {
		options = append(options, func(req *http.Request) {
			for k, v := range jobHeaders {
				req.Header.Add(k, v)
			}
		})
	}

	jobInfo, err = force.CreateBulkJob(job, options...)
	return
}

func closeBulkJob(jobId string) (jobInfo JobInfo, err error) {
	jobInfo, err = force.CloseBulkJob(jobId)
	if err != nil {
		ErrorAndExit(err.Error())
	}
	return
}

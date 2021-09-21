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

	. "github.com/ForceCLI/force/error"
	. "github.com/ForceCLI/force/lib"
)

var cmdBulk = &Command{
	//	Run:   runBulk,
	Usage: "bulk -[command, c]=[insert, update, ...] -flags args",
	Short: "Load csv file use Bulk API",
	Long: `
Load csv file use Bulk API

Commands:
  insert      upload a .csv file to insert records
  update      upload a .csv file to update records
  upsert      upload a .csv file to upsert records
  delete      upload a .csv file to delete records
  hardDelete  upload a .csv file to delete records permanently
  query       run a SOQL statement to generate a .csv file on the server
  retrieve    retrieve a query generated .csv file from the server
  job         get information about a job based on job Id
  batch       get detailed information about a batch within a job based on job Id and batch Id
  batches     get a list of batches associated with a job based on job Id

Examples using flags - more flexible, flags can be in any order with arguments after all flags.

  force bulk -c=insert -[concurrencyMode, m]=Serial -[objectType, o]=Account mydata.csv
  force bulk -c=update -[concurrencyMode, m]=Parallel -[objectType, o]=Account mydata.csv
  force bulk -c=delete -[concurrencyMode, m]=Parallel -[objectType, o]=Account mydata.csv
  force bulk -c=query -[objectType, o]=Account "SOQL"
  force bulk -c=job -[jobId, j]=jobid
  force bulk -c=batches -[jobId, j]=jobid
  force bulk -c=batch -[jobId, j]=jobid -[batchId, b]=batchid
  force bulk -c=retrieve -[jobId, j]=jobid -[batchId, b]=batchid
  force bulk -c=retrieve -j=jobid -b=batchid > mydata.csv
  force bulk -c=upsert -[concurrencyMode, m]=Serial -[objectType, o]=Account -[externalId, e]=ExternalIdField__c mydata.csv

Examples using positional arguments - less flexible, arguments must be in the correct order.

  force bulk insert Account [csv file] [<concurrency mode>]
  force bulk update Account [csv file] [<concurrency mode>]
  force bulk delete Account [csv file] [<concurrency mode>]
  force bulk upsert ExternalIdField__c Account [csv file] [<concurrency mode>]
  force bulk job [job id]
  force bulk batches [job id]
  force Bulk batch [job id] [batch id]
  force bulk batch retrieve [job id] [batch id]
  force bulk [-wait | -w] query Account [SOQL]
  force bulk [-chunk | -p]=50000 query Account [SOQL]
  force bulk query retrieve [job id] [batch id]

`,
	MaxExpectedArgs: -1,
}

var (
	command           string
	objectType        string
	jobId             string
	batchId           string
	fileFormat        string
	externalId        string
	concurrencyMode   string
	pkChunkSize       int
	pkChunkParent     string
	waitForCompletion bool
)
var commandVersion = "old"

func init() {
	cmdBulk.Flag.StringVar(&command, "command", "", "Sub command for bulk api. Can be insert, update, delete, job, batches, batch, retrieve or query.")
	cmdBulk.Flag.StringVar(&command, "c", "", "Sub command for bulk api. Can be insert, update, delete, job, batches, batch, retrieve or query.")
	cmdBulk.Flag.StringVar(&objectType, "objectType", "", "Type of sObject for CRUD commands.")
	cmdBulk.Flag.StringVar(&objectType, "o", "", "Type of sObject for CRUD commands.")
	cmdBulk.Flag.StringVar(&jobId, "jobId", "", "A batch job id.")
	cmdBulk.Flag.StringVar(&jobId, "j", "", "A batch job id.")
	cmdBulk.Flag.StringVar(&batchId, "batchId", "", "A batch id.")
	cmdBulk.Flag.StringVar(&batchId, "b", "", "A batch id.")
	cmdBulk.Flag.StringVar(&fileFormat, "format", "CSV", "File format.")
	cmdBulk.Flag.StringVar(&fileFormat, "f", "CSV", "File format.")
	cmdBulk.Flag.StringVar(&externalId, "externalId", "", "The external Id field for upserts of data.")
	cmdBulk.Flag.StringVar(&externalId, "e", "", "The External Id Field for upserts of data.")
	cmdBulk.Flag.StringVar(&concurrencyMode, "m", "Parallel", "Concurrency mode for bulk api inserts, updates, deletes and upserts.  Valid options are `Serial` and `Parallel` (default).")
	cmdBulk.Flag.StringVar(&concurrencyMode, "concurrencyMode", "Parallel", "Concurrency mode for bulk api inserts, updates, deletes and upserts.  Valid options are `Serial` and `Parallel` (default).")
	cmdBulk.Flag.BoolVar(&waitForCompletion, "wait", false, "Wait for job to complete")
	cmdBulk.Flag.BoolVar(&waitForCompletion, "w", false, "Wait for job to complete")
	cmdBulk.Flag.IntVar(&pkChunkSize, "chunk", 0, "PK chunk size")
	cmdBulk.Flag.IntVar(&pkChunkSize, "p", 0, "PK chunk size")
	cmdBulk.Flag.StringVar(&pkChunkParent, "parent", "", "PK chunk parent")
	cmdBulk.Run = runBulk
}

func runBulk2(cmd *Command, args []string) {
	if len(command) == 0 {
		cmd.PrintUsage()
		return
	}
	commandVersion = "new"
	command = strings.ToLower(command)
	switch command {
	case "insert", "update", "delete", "harddelete", "upsert", "query":
		runDBCommand(args[0])
	case "job", "retrieve", "batch", "batches":
		runBulkInfoCommand()
	default:
		ErrorAndExit("Unknown sub-command: " + command)
	}
}

func runBulkInfoCommand() {
	if len(jobId) == 0 {
		ErrorAndExit("For the " + command + " command you need to specify a job id.")
	}
	switch command {
	case "job":
		showJobDetails(jobId)
	case "batches":
		listBatches(jobId)
	case "batch", "retrieve", "status":
		if len(batchId) == 0 {
			ErrorAndExit("For the " + command + " command you need to provide a batch id in addition to a job id.")
		}
		if command == "retrieve" {
			fmt.Println(string(getBulkQueryResults(jobId, batchId)))
		} else /* batch or status */ {
			DisplayBatchInfo(getBatchDetails(jobId, batchId), os.Stdout)
		}
	default:
		ErrorAndExit("Unknown sub-command " + command + ".")
	}
}

func runDBCommand(arg string) {
	if len(objectType) == 0 {
		ErrorAndExit("Database commands need to have an sObject specified.")
	}
	if len(arg) == 0 {
		ErrorAndExit("You need to supply a path to a data file (csv) for insert and update or a SOQL statement for query.")
	}
	if command == "upsert" && len(externalId) == 0 {
		ErrorAndExit("Upsert commands must have ExternalId specified. -[externalId, e]")
	}

	var jobInfo JobInfo

	switch command {
	case "insert":
		jobInfo = createBulkInsertJob(arg, objectType, fileFormat, concurrencyMode)
	case "update":
		jobInfo = createBulkUpdateJob(arg, objectType, fileFormat, concurrencyMode)
	case "delete":
		jobInfo = createBulkDeleteJob(arg, objectType, fileFormat, concurrencyMode)
	case "harddelete":
		jobInfo = createBulkHardDeleteJob(arg, objectType, fileFormat, concurrencyMode)
	case "upsert":
		jobInfo = createBulkUpsertJob(arg, objectType, fileFormat, externalId, concurrencyMode)
	case "query":
		jobInfo = doBulkQuery(objectType, arg, fileFormat, concurrencyMode)
	}
	if !waitForCompletion {
		return
	}
	force, _ := ActiveForce()
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
	if command == "query" {
		displayQueryResults(jobInfo)
	}
}

func runBulk(cmd *Command, args []string) {
	if len(command) > 0 {
		runBulk2(cmd, args)
		return
	}
	if len(args) == 0 {
		cmd.PrintUsage()
		return
	}

	command = strings.ToLower(args[0])

	switch command {
	case "query":
		handleQuery(args)
	case "insert", "update", "upsert", "delete", "harddelete":
		handleDML(args)
	case "batch", "batches", "job":
		handleInfo(args)
	default:
		ErrorAndExit("Unknown command - " + command + ".")
	}
}

func handleInfo(args []string) {
	if len(args) == 4 && args[1] == "retrieve" {
		jobId = args[2]
		batchId = args[3]
		command = "retrieve"
	} else if len(args) == 3 && command == "batch" {
		jobId = args[1]
		batchId = args[2]
	} else if len(args) == 2 {
		jobId = args[1]
	} else {
		ErrorAndExit("Problem parsing the command.")
	}
	runBulkInfoCommand()
}

func handleDML(args []string) {
	var argLength = len(args)
	if args[0] == "upsert" {
		externalId = args[1]
		objectType = args[2]
		file := args[3]
		if argLength == 5 || argLength == 6 {
			setConcurrencyModeOrFileFormat(args[4])
			if argLength == 6 {
				setConcurrencyModeOrFileFormat(args[5])
			}
		}
		runDBCommand(file)
	} else {
		objectType = args[1]
		file := args[2]
		if argLength == 4 || argLength == 5 {
			setConcurrencyModeOrFileFormat(args[3])
			if argLength == 5 {
				setConcurrencyModeOrFileFormat(args[4])
			}
		}
		runDBCommand(file)
	}
}

func handleQuery(args []string) {
	if len(args) == 3 {
		objectType = args[1]
		runDBCommand(args[2])
	} else if len(args) == 4 {
		jobId = args[2]
		batchId = args[3]
		command = args[1]
		runBulkInfoCommand()
	} else {
		ErrorAndExit("Bad command, check arguments...")
	}
}

func setConcurrencyModeOrFileFormat(argument string) {
	if strings.EqualFold(argument, "parallel") || strings.EqualFold(argument, "serial") {
		concurrencyMode = argument
	} else {
		fileFormat = argument
	}
}

func startBulkQuery(objectType string, soql string, contenttype string, concurrencyMode string) (jobInfo JobInfo, batchId string) {
	jobInfo, err := createBulkJob(objectType, "query", contenttype, "", concurrencyMode)
	if err != nil {
		ErrorAndExit(err.Error())
	}
	force, _ := ActiveForce()

	result, err := force.BulkQuery(soql, jobInfo.Id, contenttype)
	batchId = result.Id
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
	return
}

func doBulkQuery(objectType string, soql string, contenttype string, concurrencyMode string) (jobInfo JobInfo) {
	jobInfo, batchId := startBulkQuery(objectType, soql, contenttype, concurrencyMode)
	if !waitForCompletion {
		fmt.Println("Query Submitted")
		if commandVersion == "new" {
			fmt.Printf("To retrieve query status use\nforce bulk -c=batch -j=%s -b=%s\n\n", jobInfo.Id, batchId)
			fmt.Printf("To retrieve query data use\nforce bulk -c=retrieve -j=%s -b=%s\n\n", jobInfo.Id, batchId)
		} else {
			fmt.Printf("To retrieve query status use\nforce bulk query status %s %s\n\n", jobInfo.Id, batchId)
			fmt.Printf("To retrieve query data use\nforce bulk query retrieve %s %s\n\n", jobInfo.Id, batchId)
		}
	}
	return
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
	force, _ := ActiveForce()

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
	force, _ := ActiveForce()

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

func listBatches(jobId string) {
	batchInfos := getBatches(jobId)
	DisplayBatchList(batchInfos)
}

func getJobDetails(jobId string) (jobInfo JobInfo) {
	force, _ := ActiveForce()

	jobInfo, err := force.GetJobInfo(jobId)

	if err != nil {
		ErrorAndExit(err.Error())
	}
	return
}

func getBatches(jobId string) (batchInfos []BatchInfo) {
	force, _ := ActiveForce()

	batchInfos, err := force.GetBatches(jobId)

	if err != nil {
		ErrorAndExit(err.Error())
	}
	return
}

func getBatchDetails(jobId string, batchId string) (batchInfo BatchInfo) {
	force, _ := ActiveForce()

	batchInfo, err := force.GetBatchInfo(jobId, batchId)

	if err != nil {
		ErrorAndExit(err.Error())
	}
	return
}

func createBulkInsertJob(csvFilePath string, objectType string, format string, concurrencyMode string) (jobInfo JobInfo) {
	jobInfo, err := createBulkJob(objectType, "insert", format, "", concurrencyMode)
	if err != nil {
		ErrorAndExit(err.Error())
	}
	batchInfo, err := addBatchToJob(csvFilePath, jobInfo)
	closeBulkJob(jobInfo.Id)
	if err != nil {
		ErrorAndExit(err.Error())
	}
	if !waitForCompletion {
		if commandVersion == "old" {
			fmt.Printf("Job created ( %s ) - for job status use\n force bulk batch %s %s\n", jobInfo.Id, jobInfo.Id, batchInfo.Id)
		} else {
			fmt.Printf("Job created ( %s ) - for job status use\n force bulk -c=batch -j=%s -b=%s\n", jobInfo.Id, jobInfo.Id, batchInfo.Id)
		}
	}
	return
}

func createBulkUpdateJob(csvFilePath string, objectType string, format string, concurrencyMode string) (jobInfo JobInfo) {
	jobInfo, err := createBulkJob(objectType, "update", format, "", concurrencyMode)
	if err != nil {
		ErrorAndExit(err.Error())
	}
	batchInfo, err := addBatchToJob(csvFilePath, jobInfo)
	closeBulkJob(jobInfo.Id)
	if err != nil {
		ErrorAndExit(err.Error())
	}
	if !waitForCompletion {
		if commandVersion == "old" {
			fmt.Printf("Job created ( %s ) - for job status use\n force bulk batch %s %s\n", jobInfo.Id, jobInfo.Id, batchInfo.Id)
		} else {
			fmt.Printf("Job created ( %s ) - for job status use\n force bulk -c=batch -j=%s -b=%s\n", jobInfo.Id, jobInfo.Id, batchInfo.Id)
		}
	}
	return
}

func createBulkDeleteJob(csvFilePath string, objectType string, format string, concurrencyMode string) (jobInfo JobInfo) {
	jobInfo, err := createBulkJob(objectType, "delete", format, "", concurrencyMode)
	if err != nil {
		ErrorAndExit(err.Error())
	}
	batchInfo, err := addBatchToJob(csvFilePath, jobInfo)
	closeBulkJob(jobInfo.Id)
	if err != nil {
		ErrorAndExit(err.Error())
	}
	if !waitForCompletion {
		if commandVersion == "old" {
			fmt.Printf("Job created ( %s ) - for job status use\n force bulk batch %s %s\n", jobInfo.Id, jobInfo.Id, batchInfo.Id)
		} else {
			fmt.Printf("Job created ( %s ) - for job status use\n force bulk -c=batch -j=%s -b=%s\n", jobInfo.Id, jobInfo.Id, batchInfo.Id)
		}
	}
	return
}

func createBulkHardDeleteJob(csvFilePath string, objectType string, format string, concurrencyMode string) (jobInfo JobInfo) {
	jobInfo, err := createBulkJob(objectType, "hardDelete", format, "", concurrencyMode)
	if err != nil {
		ErrorAndExit(err.Error())
	}
	batchInfo, err := addBatchToJob(csvFilePath, jobInfo)
	closeBulkJob(jobInfo.Id)
	if err != nil {
		ErrorAndExit(err.Error())
	}
	if !waitForCompletion {
		if commandVersion == "old" {
			fmt.Printf("Job created ( %s ) - for job status use\n force bulk batch %s %s\n", jobInfo.Id, jobInfo.Id, batchInfo.Id)
		} else {
			fmt.Printf("Job created ( %s ) - for job status use\n force bulk -c=batch -j=%s -b=%s\n", jobInfo.Id, jobInfo.Id, batchInfo.Id)
		}
	}
	return
}

func createBulkUpsertJob(csvFilePath string, objectType string, format string, externalId string, concurrencyMode string) (jobInfo JobInfo) {
	jobInfo, err := createBulkJob(objectType, "upsert", format, externalId, concurrencyMode)
	if err != nil {
		ErrorAndExit(err.Error())
	}
	batchInfo, err := addBatchToJob(csvFilePath, jobInfo)
	closeBulkJob(jobInfo.Id)
	if err != nil {
		ErrorAndExit(err.Error())
	}
	if !waitForCompletion {
		if commandVersion == "old" {
			fmt.Printf("Job created ( %s ) - for job status use\n force bulk batch %s %s\n", jobInfo.Id, jobInfo.Id, batchInfo.Id)
		} else {
			fmt.Printf("Job created ( %s ) - for job status use\n force bulk -c=batch -j=%s -b=%s\n", jobInfo.Id, jobInfo.Id, batchInfo.Id)
		}
	}
	return
}

func addBatchToJob(csvFilePath string, job JobInfo) (result BatchInfo, err error) {
	force, _ := ActiveForce()

	batches, err := SplitCSV(csvFilePath, 10000)
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

func createBulkJob(objectType string, operation string, fileFormat string, externalId string, concurrencyMode string) (jobInfo JobInfo, err error) {
	if !(strings.EqualFold(concurrencyMode, "serial")) {
		if !(strings.EqualFold(concurrencyMode, "parallel")) {
			ErrorAndExit("Concurrency Mode must be set to either Serial or Parallel")
		}
	}

	force, _ := ActiveForce()

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
	var pkChunkOptions []string
	if pkChunkSize != 0 {
		pkChunkOptions = append(pkChunkOptions, fmt.Sprintf("chunkSize=%d", pkChunkSize))
	}
	if pkChunkParent != "" {
		pkChunkOptions = append(pkChunkOptions, fmt.Sprintf("parent=%s", pkChunkParent))
	}
	if len(pkChunkOptions) > 0 {
		options = append(options, func(req *http.Request) {
			req.Header.Add("Sforce-Enable-PKChunking", strings.Join(pkChunkOptions, ";"))
		})
	}

	jobInfo, err = force.CreateBulkJob(job, options...)
	return
}

func closeBulkJob(jobId string) (jobInfo JobInfo, err error) {
	force, _ := ActiveForce()

	jobInfo, err = force.CloseBulkJob(jobId)
	if err != nil {
		ErrorAndExit(err.Error())
	}
	return
}

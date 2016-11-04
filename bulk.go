package main

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
	"encoding/xml"
	"fmt"
	"io/ioutil"
	"strings"
)

var cmdBulk = &Command{
	//	Run:   runBulk,
	Usage: "bulk -[command, c]=[insert, update, ...] -flags args",
	Short: "Load csv file use Bulk API",
	Long: `
Load csv file use Bulk API

Commands:
  insert   upload a .csv file to insert records
  update   upload a .csv file to update records
  query    run a SOQL statement to generate a .csv file on the server
  retrieve retrieve a query generated .csv file from the server
  job      get information about a job based on job Id
  batch    get detailed information about a batch within a job based on job Id and batch Id
  batches  get a list of batches associated with a job based on job Id

Examples:

  force bulk -c=insert -[objectType, o]=Account mydata.csv

  force bulk -c=update -[objectType, o]=Account mydata.csv

  force bulk -c=query -[objectType, o]=Account "SOQL"

  force bulk -c=job -[jobId, j]=jobid

  force bulk -c=batches -[jobId, j]=jobid

  force bulk -c=batch -[jobId, j]=jobid -[batchId, b]=batchid

  force bulk -c=retrieve -[jobId, j]=jobid -[batchId, b]=batchid

  force bulk -c retrieve -j jobid -b batchid > mydata.csv


`,
}

var (
	command    string
	objectType string
	jobId      string
	batchId    string
)

func init() {
	cmdBulk.Flag.StringVar(&command, "command", "", "Sub command for bulk api. Can be insert, update, job, batches, batch, retrieve or query.")
	cmdBulk.Flag.StringVar(&command, "c", "", "Sub command for bulk api. Can be insert, update, job, batches, batch, retrieve or query.")
	cmdBulk.Flag.StringVar(&objectType, "objectType", "", "Type of sObject for CRUD commands.")
	cmdBulk.Flag.StringVar(&objectType, "o", "", "Type of sObject for CRUD commands.")
	cmdBulk.Flag.StringVar(&jobId, "jobId", "", "A batch job id.")
	cmdBulk.Flag.StringVar(&jobId, "j", "", "A batch job id.")
	cmdBulk.Flag.StringVar(&batchId, "batchId", "", "A batch id.")
	cmdBulk.Flag.StringVar(&batchId, "b", "", "A batch id.")
	cmdBulk.Run = runBulk
}

func runBulk(cmd *Command, args []string) {
	if len(command) == 0 {
		cmd.printUsage()
		return
	}
	switch strings.ToLower(command) {
	case "insert", "update", "query":
		runDBCommand(strings.ToLower(command), args)
	case "job", "retrieve", "batch", "batches":
		runBulkInfoCommand(strings.ToLower(command))
		//fmt.Println(string(getBulkQueryResults(args[2], args[3])))
	default:
		ErrorAndExit("Unknown sub-command: " + command)
	}
}

func runBulkInfoCommand(command string) {
	if len(jobId) == 0 {
		ErrorAndExit("For the " + command + " command you need to specify a job id.")
	}
	switch command {
	case "job":
		showJobDetails(jobId)
	case "batches":
		listBatches(jobId)
	case "batch", "retrieve":
		if len(batchId) == 0 {
			ErrorAndExit("For the " + command + " command you need to provide a batch id in addition to a job id.")
		}
		if command == "batch" {
			DisplayBatchInfo(getBatchDetails(jobId, batchId))
		} else /* retrieve */ {
			fmt.Println(string(getBulkQueryResults(jobId, batchId)))
		}
	}
}
func runDBCommand(subcommand string, args []string) {
	if len(objectType) == 0 {
		fmt.Println(args)
		ErrorAndExit("Database commands need to have an sObject specified.")
	}
	if len(args) == 0 {
		ErrorAndExit("You need to supply a path to a data file (csv) for insert and update or a SOQL statement for query.")
	}

	switch subcommand {
	case "insert":
		createBulkInsertJob(args[0], objectType, "CSV")
	case "update":
		createBulkUpdateJob(args[0], objectType, "CSV")
	case "query":
		fmt.Println(args)
		doBulkQuery(objectType, args[0], "CSV")
	}
}

func doBulkQuery(objectType string, soql string, contenttype string) {
	jobInfo, err := createBulkJob(objectType, "query", contenttype)
	force, _ := ActiveForce()

	result, err := force.BulkQuery(soql, jobInfo.Id, contenttype)
	if err != nil {
		closeBulkJob(jobInfo.Id)
		ErrorAndExit(err.Error())
	}
	fmt.Println("Query Submitted")
	fmt.Printf("To retrieve query status use\nforce bulk -cmd=batch -j=%s -b=%s\n\n", jobInfo.Id, result.Id)
	fmt.Printf("To retrieve query data use\nforce bulk -cmd=retrieve -j=%s -b=%s\n\n", jobInfo.Id, result.Id)
	closeBulkJob(jobInfo.Id)
}

func getBulkQueryResults(jobId string, batchId string) (data []byte) {
	resultIds := retrieveBulkQuery(jobId, batchId)
	hasMultipleResultFiles := len(resultIds) > 1

	for _, resultId := range resultIds {
		//since this is going to stdOut, simply add header to separate "files"
		//if it's all in the same file, don't print this separator.
		if hasMultipleResultFiles {
			resultHeader := fmt.Sprint("ResultId: ", resultId, "\n")
			data = append(data[:], []byte(resultHeader)...)
		}
		//get next file, and append
		var newData []byte = retrieveBulkQueryResults(jobId, batchId, resultId)
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
	DisplayJobInfo(jobInfo)
}

func listBatches(jobId string) {
	batchInfos := getBatches(jobId)
	DisplayBatchList(batchInfos)
}

func getBatchResults(jobId string, batchId string) {
	force, _ := ActiveForce()

	data, err := force.RetrieveBulkBatchResults(jobId, batchId)
	fmt.Println(data)
	if err != nil {
		ErrorAndExit(err.Error())
	}
	return
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

func createBulkInsertJob(csvFilePath string, objectType string, format string) {
	jobInfo, err := createBulkJob(objectType, "insert", format)
	if err != nil {
		ErrorAndExit(err.Error())
	} else {
		batchInfo, err := addBatchToJob(csvFilePath, jobInfo.Id)
		if err != nil {
			closeBulkJob(jobInfo.Id)
			ErrorAndExit(err.Error())
		} else {
			closeBulkJob(jobInfo.Id)
			fmt.Printf("Job created ( %s ) - for job status use\n force bulk batch %s %s\n", jobInfo.Id, jobInfo.Id, batchInfo.Id)
		}
	}
}

func createBulkUpdateJob(csvFilePath string, objectType string, format string) {
	jobInfo, err := createBulkJob(objectType, "update", format)
	if err != nil {
		ErrorAndExit(err.Error())
	} else {
		batchInfo, err := addBatchToJob(csvFilePath, jobInfo.Id)
		if err != nil {
			closeBulkJob(jobInfo.Id)
			ErrorAndExit(err.Error())
		} else {
			closeBulkJob(jobInfo.Id)
			fmt.Printf("Job created ( %s ) - for job status use\n force bulk batch %s %s\n", jobInfo.Id, jobInfo.Id, batchInfo.Id)
		}
	}
}

func addBatchToJob(csvFilePath string, jobId string) (result BatchInfo, err error) {

	force, _ := ActiveForce()

	filedata, err := ioutil.ReadFile(csvFilePath)
	batches := splitFileIntoBatches(filedata)
	for b := range batches {
		result, err = force.AddBatchToJob(batches[b], jobId)
		if err != nil {
			break
		} else {
			fmt.Printf("Batch %d of %d added with Id %s \n", b+1, len(batches), result.Id)
		}
	}
	return
}

func splitFileIntoBatches(filedata []byte) (batches []string) {
	batchsize := 10000
	rows := strings.Split(string(filedata), "\n")
	headerRow, rows := rows[0], rows[1:]
	for len(rows) > 1 {
		if len(rows) < batchsize {
			batchsize = len(rows)
		}
		batch := []string{headerRow}
		batch = append(batch, rows[0:batchsize]...)
		batches = append(batches, strings.Join(batch, "\n"))
		rows = rows[batchsize:]
	}
	return
}

func getBatchInfo(jobId string, batchId string) (batchInfo BatchInfo, err error) {
	force, _ := ActiveForce()
	batchInfo, err = force.GetBatchInfo(jobId, batchId)
	return
}

func createBulkJob(objectType string, operation string, fileFormat string) (jobInfo JobInfo, err error) {
	force, _ := ActiveForce()

	xml := `
	<jobInfo xmlns="http://www.force.com/2009/06/asyncapi/dataload">
 		<operation>%s</operation>
 		<object>%s</object>
 		<contentType>%s</contentType>
	</jobInfo>
	`
	data := fmt.Sprintf(xml, operation, objectType, fileFormat)
	jobInfo, err = force.CreateBulkJob(data)
	return
}

func closeBulkJob(jobId string) (jobInfo JobInfo, err error) {
	force, _ := ActiveForce()

	xml := `
	<jobInfo xmlns="http://www.force.com/2009/06/asyncapi/dataload">
 		<state>Closed</state>
	</jobInfo>
	`
	jobInfo, err = force.CloseBulkJob(jobId, xml)
	if err != nil {
		ErrorAndExit(err.Error())
	}
	return
}

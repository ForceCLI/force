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

	"github.com/heroku/force/salesforce"
	"github.com/heroku/force/util"
)

var cmdBulk = &Command{
	Run:   runBulk,
	Usage: "bulk insert Account [csv file]",
	Short: "Load csv file use Bulk API",
	Long: `
Load csv file use Bulk API

Examples:

  force bulk insert Account [csv file]

  force bulk update Account [csv file]

  force bulk job [job id]

  force bulk batches [job id]

  force Bulk batch [job id] [batch id]

  force bulk batch retrieve [job id] [batch id]

  force bulk query Account [SOQL]

  force bulk query retrieve [job id] [batch id]
`,
}

func runBulk(cmd *Command, args []string) {
	if len(args) == 0 {
		cmd.printUsage()
	} else if len(args) == 1 {
		util.ErrorAndExit("Invalid command")
	} else if len(args) == 2 {
		if args[0] == "insert" {
			util.ErrorAndExit("Missing argument for insert")
		} else if args[0] == "update" {
			util.ErrorAndExit("Missing argument for update")
		} else if args[0] == "job" {
			showJobDetails(args[1])
		} else if args[0] == "batches" {
			listBatches(args[1])
		} else {
			util.ErrorAndExit("Invalid command")
		}
	} else if len(args) == 3 {
		if args[0] == "insert" {
			createBulkInsertJob(args[2], args[1], "CSV")
		} else if args[0] == "update" {
			createBulkUpdateJob(args[2], args[1], "CSV")
		} else if args[0] == "batch" {
			showBatchDetails(args[1], args[2])
		} else if args[0] == "query" {
			if args[1] == "retrieve" {
				util.ErrorAndExit("Query retrieve requires a job id and a batch id")
			} else {
				doBulkQuery(args[1], args[2], "CSV")
			}
		}
	} else if len(args) == 4 {
		if args[0] == "insert" {
			createBulkInsertJob(args[2], args[1], args[3])
		} else if args[0] == "update" {
			createBulkUpdateJob(args[2], args[1], args[3])
		} else if args[0] == "batch" {
			getBatchResults(args[2], args[3])
		} else if args[0] == "query" {
			if args[1] == "retrieve" {
				fmt.Println(string(getBulkQueryResults(args[2], args[3])))
			} else if args[1] == "status" {
				DisplayBatchInfo(getBatchDetails(args[2], args[3]))
			} else {
				doBulkQuery(args[1], args[2], args[3])
			}
		}
	}
}

func doBulkQuery(objectType string, soql string, contenttype string) {
	jobInfo, err := createBulkJob(objectType, "query", contenttype)
	force, _ := ActiveForce()

	result, err := force.BulkQuery(soql, jobInfo.Id, contenttype)
	if err != nil {
		closeBulkJob(jobInfo.Id)
		util.ErrorAndExit(err.Error())
	}
	fmt.Println("Query Submitted")
	fmt.Printf("To retrieve query status use\nforce bulk query status %s %s\n\n", jobInfo.Id, result.Id)
	fmt.Printf("To retrieve query data use\nforce bulk query retrieve %s %s\n\n", jobInfo.Id, result.Id)
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
		util.ErrorAndExit(err.Error())
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
		util.ErrorAndExit(err.Error())
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

func showBatchDetails(jobId string, batchId string) {
	batchInfo := getBatchDetails(jobId, batchId)
	DisplayBatchInfo(batchInfo)
}

func getBatchResults(jobId string, batchId string) {
	force, _ := ActiveForce()

	data, err := force.RetrieveBulkBatchResults(jobId, batchId)
	fmt.Println(data)
	if err != nil {
		util.ErrorAndExit(err.Error())
	}
	return
}

func getJobDetails(jobId string) (jobInfo salesforce.JobInfo) {
	force, _ := ActiveForce()

	jobInfo, err := force.GetJobInfo(jobId)

	if err != nil {
		util.ErrorAndExit(err.Error())
	}
	return
}

func getBatches(jobId string) (batchInfos []salesforce.BatchInfo) {
	force, _ := ActiveForce()

	batchInfos, err := force.GetBatches(jobId)

	if err != nil {
		util.ErrorAndExit(err.Error())
	}
	return
}

func getBatchDetails(jobId string, batchId string) (batchInfo salesforce.BatchInfo) {
	force, _ := ActiveForce()

	batchInfo, err := force.GetBatchInfo(jobId, batchId)

	if err != nil {
		util.ErrorAndExit(err.Error())
	}
	return
}

func createBulkInsertJob(csvFilePath string, objectType string, format string) {
	jobInfo, err := createBulkJob(objectType, "insert", format)
	if err != nil {
		util.ErrorAndExit(err.Error())
	} else {
		batchInfo, err := addBatchToJob(csvFilePath, jobInfo.Id)
		if err != nil {
			closeBulkJob(jobInfo.Id)
			util.ErrorAndExit(err.Error())
		} else {
			closeBulkJob(jobInfo.Id)
			fmt.Printf("Job created ( %s ) - for job status use\n force bulk batch %s %s\n", jobInfo.Id, jobInfo.Id, batchInfo.Id)
		}
	}
}

func createBulkUpdateJob(csvFilePath string, objectType string, format string) {
	jobInfo, err := createBulkJob(objectType, "update", format)
	if err != nil {
		util.ErrorAndExit(err.Error())
	} else {
		batchInfo, err := addBatchToJob(csvFilePath, jobInfo.Id)
		if err != nil {
			closeBulkJob(jobInfo.Id)
			util.ErrorAndExit(err.Error())
		} else {
			closeBulkJob(jobInfo.Id)
			fmt.Printf("Job created ( %s ) - for job status use\n force bulk batch %s %s\n", jobInfo.Id, jobInfo.Id, batchInfo.Id)
		}
	}
}

func addBatchToJob(csvFilePath string, jobId string) (result salesforce.BatchInfo, err error) {

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

func getBatchInfo(jobId string, batchId string) (batchInfo salesforce.BatchInfo, err error) {
	force, _ := ActiveForce()
	batchInfo, err = force.GetBatchInfo(jobId, batchId)
	return
}

func createBulkJob(objectType string, operation string, fileFormat string) (jobInfo salesforce.JobInfo, err error) {
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

func closeBulkJob(jobId string) (jobInfo salesforce.JobInfo, err error) {
	force, _ := ActiveForce()

	xml := `
	<jobInfo xmlns="http://www.force.com/2009/06/asyncapi/dataload">
 		<state>Closed</state>
	</jobInfo>
	`
	jobInfo, err = force.CloseBulkJob(jobId, xml)
	if err != nil {
		util.ErrorAndExit(err.Error())
	}
	return
}

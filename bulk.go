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
force bulk jobs

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

  force bulk batch [job id] [batch id]

  force bulk batch retrieve [job id] [batch id]

  force bulk query Account [SOQL]

  force bulk query retrieve [job id] [batch id]
`,
}

func runBulk(cmd *Command, args []string) {

	if len(args) == 1 {
		if args[0] == "jobs" {
			listJobs()
		} else {
			ErrorAndExit("Invalid command")
		}
	} else if len(args) == 2 {
		if args[0] == "insert" {
			ErrorAndExit("Missing argument for insert")
		} else if args[0] == "job" {
			showJobDetails(args[1])
		} else if args[0] == "batches" {
			listBatches(args[1])
		} else {
			ErrorAndExit("Invalid command")
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
				ErrorAndExit("Query retrieve requires a job id and a batch id")
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
		ErrorAndExit(err.Error())
	}
	fmt.Println("Query Submitted")
	fmt.Printf("To retrieve query status use\nforce bulk query status %s %s\n\n", jobInfo.Id, result.Id)
	fmt.Printf("To retrieve query data use\nforce bulk query retrieve %s %s\n\n", jobInfo.Id, result.Id)
	closeBulkJob(jobInfo.Id)
}

func getBulkQueryResults(jobId string, batchId string) (data []byte) {
	resultId := retrieveBulkQuery(jobId, batchId)
	data = retrieveBulkQueryResults(jobId, batchId, resultId)
	return
}

func retrieveBulkQuery(jobId string, batchId string) (resultId string) {
	force, _ := ActiveForce()

	jobInfo, err := force.RetrieveBulkQuery(jobId, batchId)
	if err != nil {
		ErrorAndExit(err.Error())
	}

	var result struct {
		Result string `xml:"result"`
	}

	xml.Unmarshal(jobInfo, &result)
	resultId = result.Result
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

func listJobs() (jobs []JobInfo, err error) {
	force, _ := ActiveForce()
	jobs, err = force.GetBulkJobs()
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

func showBatchDetails(jobId string, batchId string) {
	batchInfo := getBatchDetails(jobId, batchId)
	DisplayBatchInfo(batchInfo)
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
		_, err := addBatchToJob(csvFilePath, jobInfo.Id)
		if err != nil {
			closeBulkJob(jobInfo.Id)
			ErrorAndExit(err.Error())
		} else {
			closeBulkJob(jobInfo.Id)
		}
	}
}

func addBatchToJob(csvFilePath string, jobId string) (result BatchInfo, err error) {

	force, _ := ActiveForce()

	filedata, err := ioutil.ReadFile(csvFilePath)

	result, err = force.AddBatchToJob(string(filedata), jobId)
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

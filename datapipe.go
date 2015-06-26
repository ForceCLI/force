package main

import (
	"fmt"
	"io/ioutil"
	"os"
)

var cmdDataPipe = &Command{
	Usage: "datapipe <command> [<args>]",
	Short: "Manage DataPipes",
	Long: `
Manage DataPipes

Usage:

  force datapipe create -n <name> [-l masterlabel] [-t scripttype] [-c scriptcontent] [-v apiversion]

  force datapipe update -n <name> [-l masterlabel] [-t scripttype] [-c scriptcontent] [-v apiversion]

  force datapipe delete -n <name>

  force datapipe list -f <"csv" or "json">

  force datapipe query -q <query string>

  force datapipe createjob -n <pipeline name>

Commands:
  create        creates a new dataPipe
  update        update a dataPipe
  delete        delete a datapipe
  list          list all datapipes 
  query         query for a specific datapipe(s)
  createjob     creates a new job for a specific datapipe
  listjobs      list the status of submitted jobs
  queryjob      returns data about a datapipeline job (not implemented)
  retrieve      (not implemented)

Examples:

  force datapipe create -n=MyPipe -l="My Pipe" -t=Pig -v=34.0 \
  -c="A = load 'force://soql/Select Id, Name From Contact' using \
  gridforce.hadoop.pig.loadstore.func.ForceStorage();"

Defaults
  -l Defaults to the name
  -t Pig (only option available currently)
  -c Pig script template
  -v Current API version *Number only

`,
}

var defaultContent = `
-- Sample script for a data pipeline
A = load 'ffx://REPLACE_ME' using gridforce.hadoop.pig.loadstore.func.ForceStorage();
Store A  into 'ffx://REPLACE_ME_TOO' using gridforce.hadoop.pig.loadstore.func.ForceStorage();
`

var (
	dpname        string
	masterlabel   string
	scriptcontent string
	apiversion    string
	scripttype    string
	query         string
	format        string
)

func init() {
	cmdDataPipe.Flag.StringVar(&dpname, "name", "", "set datapipeline name")
	cmdDataPipe.Flag.StringVar(&dpname, "n", "", "set datapipeline name")
	cmdDataPipe.Flag.StringVar(&masterlabel, "masterlabel", "", "set master label")
	cmdDataPipe.Flag.StringVar(&masterlabel, "l", "", "set master label")
	cmdDataPipe.Flag.StringVar(&scriptcontent, "scriptcontent", defaultContent, "set script content")
	cmdDataPipe.Flag.StringVar(&scriptcontent, "c", defaultContent, "set script content")
	cmdDataPipe.Flag.StringVar(&apiversion, "apiversion", apiVersionNumber, "set api version")
	cmdDataPipe.Flag.StringVar(&apiversion, "v", apiVersionNumber, "set api version")
	cmdDataPipe.Flag.StringVar(&scripttype, "scripttype", "Pig", "set script type")
	cmdDataPipe.Flag.StringVar(&scripttype, "t", "Pig", "set script type")
	cmdDataPipe.Flag.StringVar(&query, "q", "", "SOQL query string on DataPipeline object")
	cmdDataPipe.Flag.StringVar(&query, "query", "", "SOQL query string on DataPipeline object")
	cmdDataPipe.Flag.StringVar(&format, "f", "json", "format for listing datapipelines (csv or json)")
	cmdDataPipe.Flag.StringVar(&format, "format", "json", "format for listing datapipelines (csv or json)")
	cmdDataPipe.Run = runDataPipe
}

func runDataPipe(cmd *Command, args []string) {
	if len(args) == 0 {
		cmd.printUsage()
	} else {
		if err := cmd.Flag.Parse(args[1:]); err != nil {
			os.Exit(2)
		}
		switch args[0] {
		case "create":
			runDataPipelineCreate()
		case "update":
			runDataPipelineUpdate()
		case "delete":
			runDataPipelineDelete()
		case "list":
			runDataPipelineList()
		case "query":
			runDataPipelineQuery()
		case "createjob":
			runDataPipelineJob()
		case "listjobs":
			runDataPipelineListJobs()
		default:
			ErrorAndExit("no such command: %s", args[0])
		}
	}
}

func runDataPipelineJob() {
	if len(dpname) == 0 {
		ErrorAndExit("You need to provide the name of a pipeline to create a job for.")
	}
	force, _ := ActiveForce()
	id := GetDataPipelineId(dpname)
	_, err, _ := force.CreateDataPipelineJob(id)
	if err != nil {
		ErrorAndExit(err.Error())
	}
	fmt.Printf("Successfully created DataPipeline job for %s\n", dpname)
}

func runDataPipelineListJobs() {
	//query = "SELECT Id, DataPipeline.DeveloperName, Status, FailureState, LastModifiedDate, CreatedDate, CreatedById, DataPipelineId, JobErrorMessage FROM DataPipelineJob"
	query = "SELECT DataPipeline.DeveloperName, Status, FailureState FROM DataPipelineJob"
	force, _ := ActiveForce()
	result, err := force.QueryDataPipelineJob(query)
	if err != nil {
		ErrorAndExit(err.Error())
	}

	DisplayForceRecordsf(result.Records, "csv")
}

func runDataPipelineQuery() {
	if len(query) == 0 {
		ErrorAndExit("You have to supply a SOQL query using the -q flag.")
	}
	force, _ := ActiveForce()
	result, err := force.QueryDataPipeline(query)
	if err != nil {
		ErrorAndExit(err.Error())
	}

	fmt.Println("Result: \n", result)
}

func runDataPipelineCreate() {
	if len(dpname) == 0 {
		ErrorAndExit("You must specify a name for the datapipeline using the -n flag.")
	}
	if len(masterlabel) == 0 {
		masterlabel = dpname
	}

	force, _ := ActiveForce()
	_, err, _ := force.CreateDataPipeline(dpname, masterlabel, apiversion, scriptcontent, scripttype)
	if err != nil {
		ErrorAndExit(err.Error())
	}
	fmt.Printf("DataPipeline %s successfully created.\n", dpname)
}

func runDataPipelineUpdate() {
	if len(dpname) == 0 { 
		ErrorAndExit("You must specify a name for the datapipeline using the -n flag.")
	}
	if len(masterlabel) == 0 && len(scriptcontent) == 0 {
		ErrorAndExit("You can change the master label or the script content.")
	}

	force, _ := ActiveForce()

	result, err := force.GetDataPipeline(dpname)
	if err != nil {
		ErrorAndExit(err.Error())
	}

	if len(result.Records) == 0 {
		ErrorAndExit("No data pipeline found named " + dpname)
	}
	for _, record := range result.Records {
		var id string
		id = record["Id"].(string)
		if len(masterlabel) == 0 {
			masterlabel = record["MasterLabel"].(string)
		}
		if len(scriptcontent) == 0 {
			scriptcontent = record["ScriptContent"].(string)
		}
		if _, err := os.Stat(scriptcontent); err == nil {
			fmt.Printf("file exists; processing...")
			scriptcontent, err = readScriptFile(scriptcontent)
			if err != nil {
				ErrorAndExit(err.Error())
			}
		}
		err = force.UpdateDataPipeline(id, masterlabel, scriptcontent)
		if err != nil {
			ErrorAndExit(err.Error())
		}
		fmt.Printf("%s successfully updated.\n", dpname)
	}
}

func readScriptFile(path string) (content string, err error) {
	data, err := ioutil.ReadFile(path)
	content = string(data)
	return
}

func GetDataPipelineId(name string) (id string) {
	force, _ := ActiveForce()

	result, err := force.GetDataPipeline(dpname)
	if err != nil {
		ErrorAndExit(err.Error())
	}

	if len(result.Records) == 0 {
		ErrorAndExit("No data pipeline found named " + dpname)
	}

	record := result.Records[0]
	id = record["Id"].(string)
	return
}

func runDataPipelineDelete() {
	force, _ := ActiveForce()

	id := GetDataPipelineId(dpname)
	err := force.DeleteDataPipeline(id)
	if err != nil {
		ErrorAndExit(err.Error())
	}
	fmt.Printf("%s successfully deleted.\n", dpname)
}

func runDataPipelineList() {
	force, _ := ActiveForce()
	query = "SELECT Id, MasterLabel, DeveloperName, ScriptType FROM DataPipeline"
	result, err := force.QueryDataPipeline(query)
	if err != nil {
		ErrorAndExit(err.Error())
	}

	DisplayForceRecordsf(result.Records, format)
}

package command

import (
	"fmt"
	"io/ioutil"
	"os"

	. "github.com/ForceCLI/force/error"
	. "github.com/ForceCLI/force/lib"
	"github.com/spf13/cobra"
)

func init() {
	dataPipeCreateCmd.Flags().StringP("name", "n", "", "data pipeline name")
	dataPipeCreateCmd.Flags().StringP("masterlabel", "l", "", "master label")
	dataPipeCreateCmd.Flags().StringP("scriptcontent", "c", defaultContent, "script content")
	dataPipeCreateCmd.Flags().StringP("apiversion", "v", ApiVersionNumber(), "script content")
	dataPipeCreateCmd.Flags().StringP("scripttype", "t", "Pig", "script type")

	dataPipeUpdateCmd.Flags().StringP("name", "n", "", "data pipeline name")
	dataPipeUpdateCmd.Flags().StringP("masterlabel", "l", "", "master label")
	dataPipeUpdateCmd.Flags().StringP("scriptcontent", "c", defaultContent, "script content")
	dataPipeUpdateCmd.Flags().StringP("apiversion", "v", ApiVersionNumber(), "script content")
	dataPipeUpdateCmd.Flags().StringP("scripttype", "t", "Pig", "script type")

	dataPipeListCmd.Flags().StringP("format", "f", "json", "format (csv or json)")

	dataPipeDeleteCmd.Flags().StringP("name", "n", "", "data pipeline name")

	dataPipeCreateJobCmd.Flags().StringP("name", "n", "", "data pipeline name")
	dataPipeQueryJobCmd.Flags().StringP("jobid", "j", "", "id of data pipeline job")

	dataPipeCreateCmd.MarkFlagRequired("name")
	dataPipeUpdateCmd.MarkFlagRequired("name")
	dataPipeDeleteCmd.MarkFlagRequired("name")

	dataPipeCreateJobCmd.MarkFlagRequired("name")
	dataPipeQueryJobCmd.MarkFlagRequired("jobid")

	dataPipeCmd.AddCommand(dataPipeCreateCmd)
	dataPipeCmd.AddCommand(dataPipeUpdateCmd)
	dataPipeCmd.AddCommand(dataPipeDeleteCmd)
	dataPipeCmd.AddCommand(dataPipeListCmd)
	dataPipeCmd.AddCommand(dataPipeCreateJobCmd)
	dataPipeCmd.AddCommand(dataPipeListJobsCmd)
	dataPipeCmd.AddCommand(dataPipeQueryJobCmd)

	RootCmd.AddCommand(dataPipeCmd)
}

var dataPipeCreateCmd = &cobra.Command{
	Use:   "create",
	Short: "Create Data Pipeline",
	Args:  cobra.MaximumNArgs(0),
	Run: func(cmd *cobra.Command, args []string) {
		name, _ := cmd.Flags().GetString("name")
		masterLabel, _ := cmd.Flags().GetString("masterlabel")
		apiVersion, _ := cmd.Flags().GetString("apiversion")
		scriptContent, _ := cmd.Flags().GetString("scriptcontent")
		scriptType, _ := cmd.Flags().GetString("scriptType")
		runDataPipelineCreate(name, masterLabel, apiVersion, scriptContent, scriptType)
	},
}

var dataPipeUpdateCmd = &cobra.Command{
	Use:   "update",
	Short: "Update Data Pipeline",
	Args:  cobra.MaximumNArgs(0),
	Run: func(cmd *cobra.Command, args []string) {
		name, _ := cmd.Flags().GetString("name")
		masterLabel, _ := cmd.Flags().GetString("masterlabel")
		scriptContent, _ := cmd.Flags().GetString("scriptcontent")
		runDataPipelineUpdate(name, masterLabel, scriptContent)
	},
}

var dataPipeDeleteCmd = &cobra.Command{
	Use:   "delete",
	Short: "Delete Data Pipeline",
	Args:  cobra.MaximumNArgs(0),
	Run: func(cmd *cobra.Command, args []string) {
		name, _ := cmd.Flags().GetString("name")
		runDataPipelineDelete(name)
	},
}

var dataPipeListCmd = &cobra.Command{
	Use:   "list",
	Short: "List Data Pipelines",
	Args:  cobra.MaximumNArgs(0),
	Run: func(cmd *cobra.Command, args []string) {
		format, _ := cmd.Flags().GetString("format")
		runDataPipelineList(format)
	},
}

var dataPipeCreateJobCmd = &cobra.Command{
	Use:   "createjob",
	Short: "Create Data Pipeline Job",
	Args:  cobra.MaximumNArgs(0),
	Run: func(cmd *cobra.Command, args []string) {
		name, _ := cmd.Flags().GetString("name")
		runDataPipelineJob(name)
	},
}

var dataPipeListJobsCmd = &cobra.Command{
	Use:   "listjobs",
	Short: "List Data Pipeline Jobs",
	Args:  cobra.MaximumNArgs(0),
	Run: func(cmd *cobra.Command, args []string) {
		runDataPipelineListJobs()
	},
}

var dataPipeQueryJobCmd = &cobra.Command{
	Use:   "queryjob",
	Short: "Query Data Pipeline Job",
	Args:  cobra.MaximumNArgs(0),
	Run: func(cmd *cobra.Command, args []string) {
		jobId, _ := cmd.Flags().GetString("jobid")
		runDataPipelineQueryJob(jobId)
	},
}

var dataPipeCmd = &cobra.Command{
	Use:   "datapipe <command> [<args>]",
	Short: "Manage DataPipes",
	Example: `
  force datapipe create -n=MyPipe -l="My Pipe" -t=Pig -v=34.0 \
  -c="A = load 'force://soql/Select Id, Name From Contact' using \
  gridforce.hadoop.pig.loadstore.func.ForceStorage();"
`,
	Args: cobra.MaximumNArgs(0),
}

var defaultContent = `
-- Sample script for a data pipeline
A = load 'ffx://REPLACE_ME' using gridforce.hadoop.pig.loadstore.func.ForceStorage();
Store A  into 'ffx://REPLACE_ME_TOO' using gridforce.hadoop.pig.loadstore.func.ForceStorage();
`

func runDataPipelineJob(name string) {
	id := GetDataPipelineId(name)
	_, err, _ := force.CreateDataPipelineJob(id)
	if err != nil {
		ErrorAndExit(err.Error())
	}
	fmt.Printf("Successfully created DataPipeline job for %s\n", name)
}

func runDataPipelineListJobs() {
	query := "SELECT Id, DataPipeline.DeveloperName, Status, FailureState FROM DataPipelineJob"
	result, err := force.QueryDataPipelineJob(query)
	if err != nil {
		ErrorAndExit(err.Error())
	}

	force.DisplayAllForceRecordsf(result, "csv")
}

func runDataPipelineQueryJob(jobId string) {
	query := fmt.Sprintf("SELECT Id, DataPipeline.DeveloperName, Status, FailureState, JobErrorMessage FROM DataPipelineJob Where id = '%s'", jobId)
	result, err := force.QueryDataPipelineJob(query)
	if err != nil {
		ErrorAndExit(err.Error())
	}

	force.DisplayAllForceRecordsf(result, "csv")
}

func runDataPipelineCreate(name, masterlabel, apiversion, scriptcontent, scripttype string) {
	if len(masterlabel) == 0 {
		masterlabel = name
	}
	_, err, _ := force.CreateDataPipeline(name, masterlabel, apiversion, scriptcontent, scripttype)
	if err != nil {
		ErrorAndExit(err.Error())
	}
	fmt.Printf("DataPipeline %s successfully created.\n", name)
}

func runDataPipelineUpdate(name, masterlabel, scriptcontent string) {
	if len(masterlabel) == 0 && len(scriptcontent) == 0 {
		ErrorAndExit("You can change the master label or the script content.")
	}

	result, err := force.GetDataPipeline(name)
	if err != nil {
		ErrorAndExit(err.Error())
	}

	if len(result.Records) == 0 {
		ErrorAndExit("No data pipeline found named " + name)
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
		fmt.Printf("%s successfully updated.\n", name)
	}
}

func readScriptFile(path string) (content string, err error) {
	data, err := ioutil.ReadFile(path)
	if err != nil {
		return "", err
	}
	content = string(data)
	return content, nil
}

func GetDataPipelineId(name string) (id string) {
	result, err := force.GetDataPipeline(name)
	if err != nil {
		ErrorAndExit(err.Error())
	}

	if len(result.Records) == 0 {
		ErrorAndExit("No data pipeline found named " + name)
	}

	record := result.Records[0]
	id = record["Id"].(string)
	return id
}

func runDataPipelineDelete(name string) {
	id := GetDataPipelineId(name)
	err := force.DeleteDataPipeline(id)
	if err != nil {
		ErrorAndExit(err.Error())
	}
	fmt.Printf("%s successfully deleted.\n", name)
}

func runDataPipelineList(format string) {
	query := "SELECT Id, MasterLabel, DeveloperName, ScriptType FROM DataPipeline"
	result, err := force.QueryDataPipeline(query)
	if err != nil {
		ErrorAndExit(err.Error())
	}
	force.DisplayAllForceRecordsf(result, format)
}

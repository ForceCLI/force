package command

import (
	"os"
	"strconv"
	"strings"

	. "github.com/ForceCLI/force/error"
	"github.com/olekukonko/tablewriter"
	"github.com/spf13/cobra"
	"golang.org/x/crypto/ssh/terminal"
)

func init() {
	defaultOutputFormat := "console"
	if !terminal.IsTerminal(int(os.Stdout.Fd())) {
		defaultOutputFormat = "csv"
	}
	cancelDeployCmd.Flags().StringP("deploy-id", "d", "", "Deploy Id to cancel")
	cancelDeployCmd.MarkFlagRequired("deploy-id")

	listDeploysCmd.Flags().StringP("format", "f", defaultOutputFormat, "output format: csv, json, json-pretty, console")

	listDeployErrorsCmd.Flags().StringP("deploy-id", "d", "", "Deploy Id to cancel")
	listDeployErrorsCmd.MarkFlagRequired("deploy-id")

	deploysCmd.AddCommand(listDeploysCmd)
	deploysCmd.AddCommand(cancelDeployCmd)
	deploysCmd.AddCommand(listDeployErrorsCmd)

	RootCmd.AddCommand(deploysCmd)
}

var deploysCmd = &cobra.Command{
	Use:   "deploys",
	Short: "Manage metadata deployments",
	Long: `
List and cancel metadata deployments.
`,

	Example: `
  force deploys list
  force deploys cancel -d 0Af000000000000000
`,
	DisableFlagsInUseLine: false,
}

var listDeploysCmd = &cobra.Command{
	Use:                   "list",
	Short:                 "List metadata deploys",
	DisableFlagsInUseLine: false,
	Run: func(cmd *cobra.Command, args []string) {
		format, _ := cmd.Flags().GetString("format")
		queryDeployRequests(format)
	},
}

var listDeployErrorsCmd = &cobra.Command{
	Use:                   "errors",
	Short:                 "List metadata deploy errors",
	DisableFlagsInUseLine: false,
	Run: func(cmd *cobra.Command, args []string) {
		deployId, _ := cmd.Flags().GetString("deploy-id")
		displayErrors(deployId)
	},
}

func queryDeployRequests(format string) {
	fields := []string{
		"Id",
		"Status",
		"CreatedBy.Name",
		"StartDate",
		"CompletedDate",
		"NumberComponentsDeployed",
		"NumberComponentErrors",
		// "NumberComponentsTotal",
		"NumberTestsCompleted",
		"NumberTestErrors",
		// "NumberTestsTotal",
		"CheckOnly",
		// "IgnoreWarnings",
		// "RollbackOnError",
		// "Type",
		// "CanceledBy.Name",
		// "RunTestsEnabled",
		// "ChangeSetName",
		// "ErrorStatusCode",
		// "StateDetail",
		// "ErrorMessage",
		// "AllowMissingFiles",
		// "AutoUpdatePackage",
		// "PurgeOnDelete",
		// "SinglePackage",
		// "TestLevel",
	}
	query := `
SELECT
` + strings.Join(fields, ",") + `
FROM
	DeployRequest
ORDER BY
	CompletedDate DESC
`
	queryAll := false
	useTooling := true
	explain := false
	runQuery(query, format, queryAll, useTooling, explain)
}

var cancelDeployCmd = &cobra.Command{
	Use:                   "cancel -d <deploy id>",
	Short:                 "Cancel deploy",
	DisableFlagsInUseLine: true,
	Run: func(cmd *cobra.Command, args []string) {
		deployId, _ := cmd.Flags().GetString("deploy-id")
		_, err := force.Metadata.CancelDeploy(deployId)
		if err != nil {
			ErrorAndExit("Error canceling job: " + err.Error())
		}
	},
}

func displayErrors(deployId string) {
	result, err := force.Metadata.CheckDeployStatus(deployId)
	if err != nil {
		ErrorAndExit("Error checking deploy status: " + err.Error())
	}
	if !result.Done {
		ErrorAndExit("Deploy not done: " + result.Status)
	}
	table := tablewriter.NewWriter(os.Stdout)
	table.SetRowLine(true)
	table.SetHeader([]string{
		"Component Type",
		"Name",
		// "File Name",
		"Line Number",
		"Problem Type",
		"Problem",
	})
	for _, f := range result.Details.ComponentFailures {
		table.Append([]string{
			f.ComponentType,
			f.FullName,
			// f.FileName,
			strconv.Itoa(f.LineNumber),
			f.ProblemType,
			f.Problem,
		})
	}
	if table.NumLines() > 0 {
		table.Render()
	}
}

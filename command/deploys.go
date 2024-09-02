package command

import (
	"fmt"
	"os"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/ForceCLI/force/bubbles"
	. "github.com/ForceCLI/force/error"
	. "github.com/ForceCLI/force/lib"
	tea "github.com/charmbracelet/bubbletea"
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
	cancelDeployCmd.Flags().BoolP("all", "A", false, "Cancel all pending and in-progress deploys")
	cancelDeployCmd.MarkFlagsMutuallyExclusive("deploy-id", "all")

	listDeploysCmd.Flags().StringP("format", "f", defaultOutputFormat, "output format: csv, json, json-pretty, console")

	listDeployErrorsCmd.Flags().StringP("deploy-id", "d", "", "Deploy Id to cancel")
	listDeployErrorsCmd.MarkFlagRequired("deploy-id")

	watchDeployCmd.Flags().StringP("deploy-id", "d", "", "Deploy Id to cancel")

	deploysCmd.AddCommand(listDeploysCmd)
	deploysCmd.AddCommand(cancelDeployCmd)
	deploysCmd.AddCommand(listDeployErrorsCmd)
	deploysCmd.AddCommand(watchDeployCmd)

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
  force deploys cancel --all
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

var watchDeployCmd = &cobra.Command{
	Use:                   "watch",
	Short:                 "Monitor metadata deploy",
	DisableFlagsInUseLine: false,
	RunE: func(cmd *cobra.Command, args []string) error {
		deployId, _ := cmd.Flags().GetString("deploy-id")
		if deployId == "" {
			deployId = getCurrentDeploy()
		}
		if deployId == "" {
			return fmt.Errorf("No active deploy.  Use --deploy-id for a past deploy.")
		}
		watchDeploy(deployId)
		return nil
	},
}

func getCurrentDeploy() string {
	query := "SELECT Id FROM DeployRequest WHERE Status = 'InProgress' ORDER BY CreatedDate LIMIT 1"
	queryOptions := func(options *QueryOptions) {
		options.IsTooling = true
	}
	result, err := force.Query(fmt.Sprintf("%s", query), queryOptions)
	if err != nil {
		ErrorAndExit(err.Error())
	}
	if len(result.Records) != 1 {
		return ""
	}
	record := result.Records[0]
	if id, ok := record["Id"].(string); ok {
		return id
	}
	return ""
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
	RunE: func(cmd *cobra.Command, args []string) error {
		all, _ := cmd.Flags().GetBool("all")
		if all {
			err := cancelAll()
			if err != nil {
				ErrorAndExit("Error canceling jobs: " + err.Error())
			}
			return nil
		}
		deployId, _ := cmd.Flags().GetString("deploy-id")
		if deployId != "" {
			_, err := force.Metadata.CancelDeploy(deployId)
			if err != nil {
				ErrorAndExit("Error canceling job: " + err.Error())
			}
			return nil
		}
		return fmt.Errorf("--all or --deploy-id required")
	},
}

func cancelAll() error {
	query := `SELECT Id FROM DeployRequest WHERE Status IN ('Pending', 'InProgress')`
	var queryOptions []func(*QueryOptions)
	queryOptions = append(queryOptions, func(options *QueryOptions) {
		options.IsTooling = true
	})
	result, err := force.Query(query, queryOptions...)
	if err != nil {
		return err
	}
	var wg sync.WaitGroup
	for _, r := range result.Records {
		wg.Add(1)
		deployId := r["Id"].(string)
		go func() {
			defer wg.Done()
			_, err := force.Metadata.CancelDeploy(deployId)
			if err != nil && err != AlreadyCompletedError {
				ErrorAndExit("Error cancelling deploy: " + err.Error())
			}
			for {
				result, err := force.Metadata.CheckDeployStatus(deployId)
				if err != nil {
					ErrorAndExit("Error checking deploy status: " + err.Error())
				}
				if result.Done {
					Log.Info(fmt.Sprintf("Deploy %s finished: %s", deployId, result.Status))
					return
				}
				Log.Info(fmt.Sprintf("Waiting for deploy %s to finish: %s", deployId, result.Status))
				time.Sleep(1 * time.Second)
			}
		}()
	}
	wg.Wait()

	return nil
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

func watchDeploy(deployId string) {
	d := bubbles.NewDeployModel()
	p := tea.NewProgram(d)
	go func() {
		for {
			result, err := force.Metadata.CheckDeployStatus(deployId)
			if err != nil {
				ErrorAndExit("Error checking deploy status: " + err.Error())
			}
			p.Send(bubbles.NewStatusMsg{result})
			time.Sleep(2 * time.Second)
			if result.Done {
				p.Send(bubbles.QuitMsg{})
			}
		}
	}()
	p.Run()
}

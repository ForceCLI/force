package command

import (
	"fmt"
	"os"
	"sort"
	"strings"
	"sync"
	"time"

	"github.com/ForceCLI/force/bubbles"
	. "github.com/ForceCLI/force/error"
	. "github.com/ForceCLI/force/lib"
	tea "github.com/charmbracelet/bubbletea"
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

	statusDeployCmd.Flags().StringP("deploy-id", "d", "", "Deploy Id to get status for")
	statusDeployCmd.Flags().BoolP("verbose", "v", false, "Show detailed information including component changes")
	statusDeployCmd.MarkFlagRequired("deploy-id")

	deploysCmd.AddCommand(listDeploysCmd)
	deploysCmd.AddCommand(cancelDeployCmd)
	deploysCmd.AddCommand(listDeployErrorsCmd)
	deploysCmd.AddCommand(watchDeployCmd)
	deploysCmd.AddCommand(statusDeployCmd)

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

	if len(result.Details.ComponentFailures) == 0 {
		fmt.Println("No component failures found.")
		return
	}

	fmt.Printf("Component Failures (%d):\n", len(result.Details.ComponentFailures))
	fmt.Println(strings.Repeat("-", 80))

	for _, f := range result.Details.ComponentFailures {
		fmt.Printf("\n✗ %s (%s)\n", f.FullName, f.ComponentType)
		if f.LineNumber > 0 {
			fmt.Printf("  Line: %d\n", f.LineNumber)
		}
		fmt.Printf("  Problem: %s\n", f.Problem)
		if f.ProblemType != "" && f.ProblemType != "Error" {
			fmt.Printf("  Type: %s\n", f.ProblemType)
		}
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

var statusDeployCmd = &cobra.Command{
	Use:                   "status",
	Short:                 "Show deployment status",
	DisableFlagsInUseLine: false,
	Run: func(cmd *cobra.Command, args []string) {
		deployId, _ := cmd.Flags().GetString("deploy-id")
		verbose, _ := cmd.Flags().GetBool("verbose")
		displayDeployStatus(deployId, verbose)
	},
}

func displayComponentsByType(components []ComponentSuccess) {
	// Group components by type
	componentsByType := make(map[string][]string)
	for _, comp := range components {
		componentType := comp.ComponentType
		if componentType == "" {
			componentType = "Other"
		}
		componentsByType[componentType] = append(componentsByType[componentType], comp.FullName)
	}

	// Sort component types for consistent output
	var types []string
	for t := range componentsByType {
		types = append(types, t)
	}
	sort.Strings(types)

	// Display grouped components
	for _, componentType := range types {
		names := componentsByType[componentType]
		sort.Strings(names) // Sort component names within each type
		fmt.Printf("\n%s (%d):\n", componentType, len(names))
		for _, name := range names {
			fmt.Printf("  • %s\n", name)
		}
	}
}

func displayDeployStatus(deployId string, verbose bool) {
	result, err := force.Metadata.CheckDeployStatus(deployId)
	if err != nil {
		ErrorAndExit("Error checking deploy status: " + err.Error())
	}

	fmt.Printf("Deploy ID: %s\n", result.Id)
	fmt.Printf("Status: %s\n", result.Status)
	fmt.Printf("Done: %v\n", result.Done)
	fmt.Printf("Success: %v\n", result.Success)
	fmt.Printf("Check Only: %v\n", result.CheckOnly)
	fmt.Printf("Rollback on Error: %v\n", result.RollbackOnError)
	fmt.Printf("Created By: %s\n", result.CreatedByName)
	fmt.Printf("Created Date: %s\n", result.CreatedDate.Format("2006-01-02 15:04:05"))
	if !result.CompletedDate.IsZero() {
		fmt.Printf("Completed Date: %s\n", result.CompletedDate.Format("2006-01-02 15:04:05"))
	}
	fmt.Printf("\nComponents:\n")
	fmt.Printf("  Total: %d\n", result.NumberComponentsTotal)
	fmt.Printf("  Deployed: %d\n", result.NumberComponentsDeployed)
	fmt.Printf("  Errors: %d\n", result.NumberComponentErrors)

	fmt.Printf("\nTests:\n")
	fmt.Printf("  Total: %d\n", result.NumberTestsTotal)
	fmt.Printf("  Completed: %d\n", result.NumberTestsCompleted)
	fmt.Printf("  Errors: %d\n", result.NumberTestErrors)

	if result.ErrorMessage != "" {
		fmt.Printf("\nError Message: %s\n", result.ErrorMessage)
	}
	if result.ErrorStatusCode != "" {
		fmt.Printf("Error Status Code: %s\n", result.ErrorStatusCode)
	}
	if result.StateDetail != "" {
		fmt.Printf("State Detail: %s\n", result.StateDetail)
	}

	// Display component changes (created, changed, deleted) by default
	if len(result.Details.ComponentSuccesses) > 0 {
		var created, changed, deleted, unchanged []ComponentSuccess

		for _, comp := range result.Details.ComponentSuccesses {
			// Skip destructiveChanges.xml entries - they're just warnings for already deleted items
			if comp.FileName == "destructiveChanges.xml" {
				continue
			}

			if comp.Created {
				created = append(created, comp)
			} else if comp.Changed {
				changed = append(changed, comp)
			} else if comp.Deleted {
				deleted = append(deleted, comp)
			} else {
				unchanged = append(unchanged, comp)
			}
		}

		if len(created) > 0 {
			fmt.Printf("\n--- Components Created (%d) ---\n", len(created))
			displayComponentsByType(created)
		}

		if len(changed) > 0 {
			fmt.Printf("\n--- Components Changed (%d) ---\n", len(changed))
			displayComponentsByType(changed)
		}

		if len(deleted) > 0 {
			fmt.Printf("\n--- Components Deleted (%d) ---\n", len(deleted))
			displayComponentsByType(deleted)
		}

		// Only show unchanged components in verbose mode
		if verbose && len(unchanged) > 0 {
			fmt.Printf("\n--- Components Unchanged (%d) ---\n", len(unchanged))
			displayComponentsByType(unchanged)
		}
	}

	// Display component failures by default
	if len(result.Details.ComponentFailures) > 0 {
		fmt.Printf("\n--- Component Failures (%d) ---\n", len(result.Details.ComponentFailures))
		for _, f := range result.Details.ComponentFailures {
			fmt.Printf("  ✗ %s (%s)\n", f.FullName, f.ComponentType)
			if f.LineNumber > 0 {
				fmt.Printf("    Line: %d\n", f.LineNumber)
			}
			fmt.Printf("    Problem: %s\n", f.Problem)
			if f.ProblemType != "" && f.ProblemType != "Error" {
				fmt.Printf("    Type: %s\n", f.ProblemType)
			}
		}
	}

	// Display test results by default
	if result.Details.RunTestResult.NumberOfTestsRun > 0 {
		fmt.Printf("\n--- Test Results ---\n")
		fmt.Printf("Tests Run: %d | Failures: %d | Total Time: %.2fs\n",
			result.Details.RunTestResult.NumberOfTestsRun,
			result.Details.RunTestResult.NumberOfFailures,
			result.Details.RunTestResult.TotalTime/1000.0) // Convert ms to seconds

		if len(result.Details.RunTestResult.TestFailures) > 0 {
			fmt.Printf("\nTest Failures (%d):\n", len(result.Details.RunTestResult.TestFailures))
			for _, f := range result.Details.RunTestResult.TestFailures {
				fmt.Printf("  ✗ %s.%s\n", f.Name, f.MethodName)
				fmt.Printf("    Message: %s\n", f.Message)
				if f.StackTrace != "" && verbose {
					fmt.Printf("    Stack Trace: %s\n", f.StackTrace)
				}
			}
		}

		if len(result.Details.RunTestResult.TestSuccesses) > 0 {
			fmt.Printf("\nTest Successes (%d):\n", len(result.Details.RunTestResult.TestSuccesses))
			for _, s := range result.Details.RunTestResult.TestSuccesses {
				fmt.Printf("  ✓ %s.%s (%.2fs)\n", s.Name, s.MethodName, s.Time/1000.0) // Convert ms to seconds
			}
		}
	}

	if verbose {
		fmt.Printf("\n=== DETAILED DEPLOYMENT INFORMATION ===\n")

		if len(result.Details.RunTestResult.CodeCoverageWarnings) > 0 {
			fmt.Printf("\nCode Coverage Warnings (%d):\n", len(result.Details.RunTestResult.CodeCoverageWarnings))
			for _, w := range result.Details.RunTestResult.CodeCoverageWarnings {
				fmt.Printf("  ⚠ %s: %s\n", w.Name, w.Message)
			}
		}

		if len(result.Details.RunTestResult.CodeCoverage) > 0 {
			fmt.Printf("\nCode Coverage (%d classes):\n", len(result.Details.RunTestResult.CodeCoverage))
			for _, c := range result.Details.RunTestResult.CodeCoverage {
				coverage := float64(c.NumLocations-c.NumLocationsNotCovered) / float64(c.NumLocations) * 100
				fmt.Printf("  • %s: %.1f%% (%d/%d locations covered)\n",
					c.Name, coverage, c.NumLocations-c.NumLocationsNotCovered, c.NumLocations)
			}
		}
	}
}

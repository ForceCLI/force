package sf

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"math"
	"net/url"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strconv"
	"strings"
	"time"

	"github.com/spf13/cobra"

	"github.com/ForceCLI/force/lib"
)

// deployPollInterval is the time between checks of a deployment.  Tests
// shorten it.
var deployPollInterval = time.Second

// Fields of a deploy result that sf reports as numbers and as arrays.  The
// SOAP response gives every value as text and gives a single element where
// there is one entry.
var (
	deployNumberFields = map[string]bool{
		"numberComponentErrors": true, "numberComponentsDeployed": true, "numberComponentsTotal": true,
		"numberTestErrors": true, "numberTestsCompleted": true, "numberTestsTotal": true,
		"columnNumber": true, "lineNumber": true, "numFailures": true, "numTestsRun": true,
		"totalTime": true, "time": true, "numLocations": true, "numLocationsNotCovered": true, "numExecutions": true, "column": true, "line": true,
	}
	deployArrayFields = map[string]bool{
		"componentSuccesses": true, "componentFailures": true, "successes": true, "failures": true,
		"codeCoverage": true, "codeCoverageWarnings": true, "flowCoverage": true, "flowCoverageWarnings": true,
		"locationsNotCovered": true,
	}
)

// typedDeployValue converts the text values of a SOAP deploy result to the
// JSON types sf reports.
func typedDeployValue(key string, value interface{}) interface{} {
	switch v := value.(type) {
	case string:
		if v == "true" {
			return true
		}
		if v == "false" {
			return false
		}
		if deployNumberFields[key] {
			if number, err := strconv.ParseFloat(v, 64); err == nil {
				return json.Number(strconv.FormatFloat(number, 'f', -1, 64))
			}
		}
		return v
	case *orderedMap:
		for _, childKey := range v.keys {
			if childKey == "locationsCovered" {
				// sf leaves the values of the covered locations as text.
				continue
			}
			child := typedDeployValue(childKey, v.values[childKey])
			if deployArrayFields[childKey] {
				if _, isList := child.([]interface{}); !isList {
					child = []interface{}{child}
				}
			}
			v.values[childKey] = child
		}
		return v
	case []interface{}:
		for i, item := range v {
			v[i] = typedDeployValue(key, item)
		}
		return v
	}
	return value
}

// deployState holds the status of a deployment and the response it came
// from.
type deployState struct {
	status lib.ForceCheckDeploymentStatusResult
	raw    *orderedMap
	// verboseInfo holds the lines --verbose adds to the progress display.
	verboseInfo [][2]string
	// finishedBefore is set when the deployment was already finished
	// when the command started to watch it: sf then shows the deploy and
	// test stages as skipped.
	finishedBefore bool
}

func checkDeploy(org *targetOrg, id string) (*deployState, error) {
	status, body, err := org.Force.Metadata.CheckDeployStatusXML(id)
	if err != nil {
		return nil, apiError(err)
	}
	state := &deployState{status: status, raw: newOrderedMap()}
	if results, err := soapResults(body); err == nil && len(results) > 0 {
		state.raw, _ = typedDeployValue("", results[0]).(*orderedMap)
	}
	if state.raw == nil {
		state.raw = newOrderedMap()
	}
	if details, ok := state.raw.values["details"].(*orderedMap); ok {
		for _, key := range []string{"componentSuccesses", "componentFailures"} {
			if _, present := details.Get(key); !present {
				details.Set(key, []interface{}{})
			}
		}
	}
	return state, nil
}

func deployURL(org *targetOrg, id string) string {
	inner := "/changemgmt/monitorDeploymentsDetails.apexp?asyncId=" + id + "&retURL=" + url.QueryEscape("/changemgmt/monitorDeployment.apexp")
	return org.Fields.String("instanceUrl") + "/lightning/setup/DeployStatus/page?address=" + url.QueryEscape(inner)
}

// deployFileResults lists the local files of the components in a deploy
// result, the way sf reports them in "files".
func deployFileResults(state *deployState, source *sourceSet) []*orderedMap {
	results := []*orderedMap{}
	if source != nil && source.noFiles {
		return results
	}
	add := func(fullName, componentType, fileState, fileName string, extra func(*orderedMap)) {
		if fileName == "package.xml" || componentType == "" {
			return
		}
		var paths []string
		if source != nil {
			paths = source.localPaths(fileName)
			if fileState == "Deleted" {
				// sf reports the files of a deleted component relative
				// to the project.
				paths = source.deletedPaths[componentType+"###"+fullName]
			}
		}
		if len(paths) == 0 {
			paths = []string{fileName}
		}
		for _, path := range paths {
			item := newOrderedMap()
			item.Set("fullName", fullName)
			item.Set("type", componentType)
			item.Set("state", fileState)
			item.Set("filePath", path)
			if extra != nil {
				extra(item)
			}
			results = append(results, item)
		}
	}
	for _, success := range state.status.Details.ComponentSuccesses {
		fileState := "Unchanged"
		switch {
		case success.Deleted:
			fileState = "Deleted"
		case success.Created:
			fileState = "Created"
		case success.Changed:
			fileState = "Changed"
		}
		add(success.FullName, success.ComponentType, fileState, success.FileName, nil)
	}
	for _, failure := range state.status.Details.ComponentFailures {
		failure := failure
		add(failure.FullName, failure.ComponentType, "Failed", failure.FileName, func(item *orderedMap) {
			item.Set("error", failure.Problem)
			item.Set("problemType", failure.ProblemType)
			if failure.LineNumber > 0 {
				item.Set("lineNumber", failure.LineNumber)
				item.Set("columnNumber", failure.ColumnNumber)
			}
		})
	}
	sort.SliceStable(results, func(i, j int) bool {
		a, b := results[i], results[j]
		if a.values["type"] != b.values["type"] {
			return text(a.values["type"]) < text(b.values["type"])
		}
		if a.values["fullName"] != b.values["fullName"] {
			return text(a.values["fullName"]) < text(b.values["fullName"])
		}
		return text(a.values["filePath"]) < text(b.values["filePath"])
	})
	return results
}

// fillTestResultLists adds the lists of a test result that the org left
// out of a deploy result.  sf reports them as empty lists.
func fillTestResultLists(result *orderedMap) {
	if details, ok := result.values["details"].(*orderedMap); ok {
		if tests, ok := details.values["runTestResult"].(*orderedMap); ok {
			for _, key := range []string{"codeCoverage", "codeCoverageWarnings", "failures", "flowCoverage", "flowCoverageWarnings", "successes"} {
				if _, present := tests.values[key]; !present {
					tests.Set(key, []interface{}{})
				}
			}
			coverage, _ := tests.values["codeCoverage"].([]interface{})
			for _, item := range coverage {
				if class, ok := item.(*orderedMap); ok {
					for _, key := range []string{"dmlInfo", "locationsNotCovered", "methodInfo", "soqlInfo", "soslInfo"} {
						if _, present := class.values[key]; !present {
							class.Set(key, []interface{}{})
						}
					}
				}
			}
		}
	}
}

// deployJSON returns the result sf reports for a deployment: the fields of
// the Metadata API result and the local files.
func deployJSON(org *targetOrg, state *deployState, source *sourceSet, zipSize, zipFileCount int) *orderedMap {
	result := newOrderedMap()
	for _, key := range state.raw.keys {
		result.Set(key, state.raw.values[key])
	}
	fillTestResultLists(result)
	result.Set("files", deployFileResults(state, source))
	if zipSize > 0 {
		result.Set("zipSize", zipSize)
		result.Set("zipFileCount", zipFileCount)
	}
	result.Set("deployUrl", deployURL(org, state.status.Id))
	return result
}

// writeDeployTables prints the tables sf prints after a deployment: the
// deployed or validated source, the component failures and a summary of
// the tests.
func writeDeployTables(w io.Writer, state *deployState, source *sourceSet, concise bool, verbose ...bool) {
	files := deployFileResults(state, source)
	var successes, failures []*orderedMap
	for _, file := range files {
		if file.values["state"] == "Failed" {
			failures = append(failures, file)
		} else {
			successes = append(successes, file)
		}
	}
	if !concise && len(successes) > 0 && state.status.Status != "Failed" {
		title := "Deployed Source"
		if state.status.CheckOnly {
			title = "Validated Source"
		}
		rows := make([][]string, 0, len(successes))
		for _, file := range successes {
			rows = append(rows, []string{text(file.values["state"]), text(file.values["fullName"]), text(file.values["type"]), relativePath(text(file.values["filePath"]))})
		}
		fmt.Fprintln(w)
		printTable(w, paint(w, style{"\x1b[1m\x1b[34m", "\x1b[39m\x1b[22m"}, title), []string{"State", "Name", "Type", "Path"}, rows)
	}
	if state.status.Status != "Succeeded" && len(state.status.Details.ComponentFailures) > 0 {
		componentFailures := append([]lib.ComponentFailure{}, state.status.Details.ComponentFailures...)
		sort.SliceStable(componentFailures, func(i, j int) bool {
			a, b := componentFailures[i], componentFailures[j]
			if a.ProblemType != b.ProblemType {
				return a.ProblemType < b.ProblemType
			}
			if a.FullName != b.FullName {
				return a.FullName < b.FullName
			}
			return a.LineNumber < b.LineNumber
		})
		rows := make([][]string, 0, len(componentFailures))
		for _, failure := range componentFailures {
			location := ""
			if failure.LineNumber > 0 {
				location = fmt.Sprintf("%d:%d", failure.LineNumber, failure.ColumnNumber)
			}
			rows = append(rows, []string{failure.ComponentType, failure.FullName, failure.Problem, location})
		}
		fmt.Fprintln(w)
		printTable(w, paint(w, styleRed, fmt.Sprintf("Component Failures [%d]", len(rows))), []string{"Type", "Name", "Problem", "Line:Column"}, rows)
	}
	tests := state.status.Details.RunTestResult
	testsEnabled := tests.NumberOfTestsRun > 0
	if enabled, ok := state.raw.values["runTestsEnabled"].(bool); ok {
		testsEnabled = enabled
	}
	if !testsEnabled {
		// sf prints an empty line in place of the test results.
		fmt.Fprintln(w)
		return
	}
	byMethod := func(aMethod, aName, bMethod, bName string) bool {
		if aMethod != bMethod {
			return aMethod < bMethod
		}
		return aName < bName
	}
	if state.status.NumberTestErrors > 0 {
		failures := append([]lib.TestFailure{}, tests.TestFailures...)
		sort.SliceStable(failures, func(i, j int) bool {
			return byMethod(failures[i].MethodName, failures[i].Name, failures[j].MethodName, failures[j].Name)
		})
		fmt.Fprintln(w)
		fmt.Fprintln(w, paint(w, styleRed, fmt.Sprintf("Test Failures [%d]", tests.NumberOfFailures)))
		for _, failure := range failures {
			fmt.Fprintf(w, "• %s.%s\n", failure.Name, failure.MethodName)
			fmt.Fprintf(w, "  message: %s\n", failure.Message)
			if failure.StackTrace != "" {
				fmt.Fprintf(w, "  stacktrace: \n    %s\n", strings.ReplaceAll(failure.StackTrace, "\n", "\n    "))
			}
			fmt.Fprintln(w)
		}
	}
	if len(verbose) > 0 && verbose[0] {
		successes := append([]lib.TestSuccess{}, tests.TestSuccesses...)
		sort.SliceStable(successes, func(i, j int) bool {
			return byMethod(successes[i].MethodName, successes[i].Name, successes[j].MethodName, successes[j].Name)
		})
		if len(successes) > 0 {
			fmt.Fprintln(w)
			fmt.Fprintln(w, paint(w, styleGreen, fmt.Sprintf("Test Success [%d]", len(successes))))
			for _, success := range successes {
				fmt.Fprintf(w, "✓ %s.%s (%vms)\n", success.Name, success.MethodName, success.Time)
			}
		}
		if len(tests.CodeCoverage) > 0 {
			coverage := append([]lib.CodeCoverage{}, tests.CodeCoverage...)
			sort.SliceStable(coverage, func(i, j int) bool { return strings.ToUpper(coverage[i].Name) < strings.ToUpper(coverage[j].Name) })
			rows := make([][]string, 0, len(coverage))
			for _, class := range coverage {
				percent := 100.0
				if class.NumLocations > 0 {
					percent = math.Round(float64(class.NumLocations-class.NumLocationsNotCovered)/float64(class.NumLocations)*100) / 100 * 100
				}
				lines := make([]string, 0, len(class.LocationsNotCovered))
				for _, location := range class.LocationsNotCovered {
					lines = append(lines, strconv.Itoa(location.Line))
				}
				rows = append(rows, []string{class.Name, formatNumber(percent) + "%", strings.Join(lines, ",")})
			}
			fmt.Fprintln(w)
			printTable(w, paint(w, style{"\x1b[1m\x1b[34m", "\x1b[39m\x1b[22m"}, "Apex Code Coverage"), []string{"Name", "% Covered", "Uncovered Lines"}, rows)
		}
	}
	fmt.Fprintln(w)
	fmt.Fprintln(w, paint(w, style{"\x1b[1m\x1b[34m", "\x1b[39m\x1b[22m"}, "Test Results Summary"))
	fmt.Fprintf(w, "Passing: %d\n", state.status.NumberTestsCompleted)
	fmt.Fprintf(w, "Failing: %d\n", state.status.NumberTestErrors)
	fmt.Fprintf(w, "Total: %d\n", state.status.NumberTestsTotal)
	if tests.TotalTime > 0 {
		fmt.Fprintf(w, "Time: %v\n", tests.TotalTime)
	}
}

// zipFileSize formats a number of bytes the way sf does.
func zipFileSize(size int) string {
	bytes := float64(size)
	units := []string{"B", "KB", "MB", "GB"}
	for bytes > 1024 && len(units) > 1 {
		bytes /= 1024
		units = units[1:]
	}
	return formatNumber(math.Round(bytes*100)/100) + " " + units[0]
}

// deployProgress prints the last frame of the progress display for a
// deployment.
func deployProgress(w io.Writer, org *targetOrg, title, subtitle string, state *deployState, queued bool, prepare, elapsed time.Duration, tracked ...*trackingProgress) {
	status := state.status
	stages := []stage{
		{label: "Preparing", state: "done", elapsed: prepare},
		{label: "Waiting for the org to respond", state: "skipped"},
		{label: "Deploying Metadata", state: "done", elapsed: elapsed - prepare},
		{label: "Running Tests", state: "skipped"},
		{label: "Updating Source Tracking", state: "skipped"},
		{label: "Done", state: "done"},
	}
	if queued {
		stages[2].state = "skipped"
	} else {
		if status.NumberComponentsTotal > 0 {
			stages[2].details = []string{fmt.Sprintf("Components: %d/%d (%d%%)", status.NumberComponentsDeployed, status.NumberComponentsTotal, status.NumberComponentsDeployed*100/status.NumberComponentsTotal)}
		} else if status.Status == "Failed" {
			// sf marks the component count as failed when the org
			// reported no count.
			stages[2].details = []string{"Components: ✘"}
		}
		// A deploy that failed without a component error or a test error
		// failed on its code coverage, in the test stage.
		coverageFailed := status.Status == "Failed" && status.NumberComponentErrors == 0 && status.NumberTestErrors == 0 && status.NumberTestsTotal > 0
		if status.NumberComponentErrors > 0 || (status.Status == "Failed" && status.NumberTestErrors == 0 && !coverageFailed) {
			stages[2].state = "failed"
		}
		if status.NumberTestsTotal > 0 {
			stages[3].state = "done"
			stages[3].details = []string{fmt.Sprintf("Successful: %d/%d (%d%%)", status.NumberTestsCompleted, status.NumberTestsTotal, status.NumberTestsCompleted*100/status.NumberTestsTotal)}
			if status.NumberTestErrors > 0 {
				stages[3].state = "failed"
				stages[3].details = append(stages[3].details, fmt.Sprintf("Failed: %d/%d (%d%%)", status.NumberTestErrors, status.NumberTestsTotal, status.NumberTestErrors*100/status.NumberTestsTotal))
			} else if coverageFailed {
				stages[3].state = "failed"
				stages[3].details = append(stages[3].details, "Failed: ✘")
			}
		}
		if status.Status != "Succeeded" {
			stages[5].state = ""
		}
		if status.Status == "Failed" {
			// The stages after the one that failed were not reached.
			stages[4].state = ""
			if stages[2].state == "failed" {
				stages[3] = stage{label: "Running Tests"}
			}
		}
	}
	if state.finishedBefore && status.Status == "Succeeded" {
		stages[2] = stage{label: "Deploying Metadata", state: "skipped"}
		stages[3] = stage{label: "Running Tests", state: "skipped"}
	}
	deployTime := elapsed - prepare
	if len(tracked) > 0 && tracked[0] != nil {
		progress := tracked[0]
		deployTime -= progress.elapsed
		stages[4] = stage{label: "Updating Source Tracking", state: "done", elapsed: progress.elapsed}
		if progress.total > 0 {
			stages[4].details = []string{fmt.Sprintf("Members: %d/%d (%d%%)", progress.found, progress.total, progress.found*100/progress.total)}
		}
	}
	stages[2].elapsed = deployTime
	statusText := status.Status
	if statusText == "InProgress" {
		statusText = "In Progress"
	}
	info := [][2]string{{"Status", statusText}, {"Deploy ID", status.Id}, {"Target Org", org.Username}}
	info = append(info, state.verboseInfo...)
	info = append(info, [2]string{"Elapsed Time", formatElapsed(elapsed)})
	writeStages(w, title, subtitle, stages, info)
}

// waitForDeploy polls a deployment until it is done or the timeout passes.
func waitForDeploy(org *targetOrg, id string, timeout time.Duration) (*deployState, error) {
	deadline := time.Now().Add(timeout)
	for {
		state, err := checkDeploy(org, id)
		if err != nil {
			return nil, err
		}
		if state.status.Done {
			return state, nil
		}
		if time.Now().After(deadline) {
			return state, &Error{
				Name:     "ClientTimeoutError",
				Message:  "The command has timed out, although the deployment is still running. Use \"" + binName + " project deploy resume\" to resume watching the deployment.",
				ExitCode: 69,
			}
		}
		time.Sleep(deployPollInterval)
	}
}

// finishDeploy prints the outcome of a finished deployment and returns the
// exit status as an error when the deployment did not succeed.
func finishDeploy(cmd *cobra.Command, org *targetOrg, state *deployState, source *sourceSet, zipSize, zipFileCount int, after func(w io.Writer)) error {
	status := 0
	if state.status.Status != "Succeeded" {
		status = 1
	}
	reports, err := readDeployReportFlags(cmd)
	if err != nil {
		return err
	}
	tests := state.status.Details.RunTestResult
	if reports.requested() {
		if testLevel, _ := cmd.Flags().GetString("test-level"); (state.status.NumberTestsTotal == 0 && testLevel == "") || testLevel == "NoTestRun" {
			var lines []string
			if len(reports.formats) > 0 {
				lines = append(lines, "`--coverage-formatters` was specified but no tests ran.")
			}
			if reports.junit {
				lines = append(lines, "`--junit` was specified but no tests ran.")
			}
			lines = append(lines, "You can ensure tests run by specifying `--test-level` and setting it to `RunSpecifiedTests`, `RunLocalTests` or `RunAllTestsInOrg`.")
			warn(cmd, strings.Join(lines, "\n"))
		}
		if err := reports.write(org, tests); err != nil {
			return err
		}
	}
	concise, _ := cmd.Flags().GetBool("concise")
	verbose, _ := cmd.Flags().GetBool("verbose")
	human := func(w io.Writer) {
		writeDeployTables(w, state, source, concise, verbose)
		if enabled, _ := state.raw.values["runTestsEnabled"].(bool); enabled || tests.NumberOfTestsRun > 0 {
			for _, warning := range tests.CodeCoverageWarnings {
				message := warning.Message
				if warning.Name != "" {
					message = warning.Name + " - " + message
				}
				uxWarn(cmd.ErrOrStderr(), message)
			}
		}
		if len(reports.formats) > 0 {
			fmt.Fprintf(w, "Code Coverage formats, [%s], written to %s/\n", strings.Join(reports.formats, ", "), filepath.Join(reports.resultsDir, "coverage"))
		}
		if reports.junit {
			fmt.Fprintf(w, "Junit results written to %s/junit/junit.xml\n", reports.resultsDir)
		}
		if status == 0 && after != nil {
			after(w)
		}
	}
	if status != 0 && cmd.Annotations[commandNameAnnotation] == "DeployMetadataValidate" {
		// sf reports a failed validation as an error, also with --json.
		if !jsonRequested(cmd) {
			human(cmd.OutOrStdout())
		}
		return failedValidationError(state)
	}
	if err := emitStatus(cmd, status, deployJSON(org, state, source, zipSize, zipFileCount), human); err != nil {
		return err
	}
	if status != 0 {
		return exitStatus(status)
	}
	return nil
}

// failedValidationError is the error sf reports for a validation that did
// not succeed: the coverage warnings, the component errors and the test
// failures.
func failedValidationError(state *deployState) error {
	tests := state.status.Details.RunTestResult
	var parts []string
	for _, warning := range tests.CodeCoverageWarnings {
		message := warning.Message
		if warning.Name != "" {
			message = warning.Name + " - " + message
		}
		parts = append(parts, message)
	}
	componentErrors := ""
	for _, failure := range state.status.Details.ComponentFailures {
		componentErrors += fmt.Sprintf("%s in %s - %s\n", failure.ProblemType, failure.FullName, failure.Problem)
	}
	count := ""
	if state.status.NumberComponentErrors > 0 {
		count = fmt.Sprintf("%d component error(s)", state.status.NumberComponentErrors)
	}
	failures := make([]string, 0, len(tests.TestFailures))
	for _, failure := range tests.TestFailures {
		failures = append(failures, fmt.Sprintf("%s.%s - %s", failure.Name, failure.MethodName, failure.Message))
	}
	parts = append(parts, componentErrors, count, strings.Join(failures, ","))
	data := newOrderedMap()
	data.Set("deployId", state.status.Id)
	return &Error{
		Name:     "FailedValidationError",
		Message:  fmt.Sprintf("Failed to validate the deployment (%s). Due To:\n%s", state.status.Id, strings.TrimSpace(strings.Join(parts, "\n"))),
		ExitCode: 1,
		Data:     data,
	}
}

func addDeployJobFlags(cmd *cobra.Command) {
	addJSONFlag(cmd)
	cmd.Flags().StringP("job-id", "i", "", "Job ID of the deploy operation.")
	cmd.Flags().BoolP("use-most-recent", "r", false, "Use the job ID of the most recent deploy operation.")
	cmd.Flags().IntP("wait", "w", 33, "Number of minutes to wait for the command to complete and display results.")
	addTargetOrgFlag(cmd)
}

// deployCachePath returns ~/.sf/deploy-cache.json, where sf records the
// deployments it starts.
func deployCachePath() (string, error) {
	home, err := os.UserHomeDir()
	if err != nil {
		return "", err
	}
	return filepath.Join(home, ".sf", "deploy-cache.json"), nil
}

// readDeployCache reads the deploy cache: an object of cache entries by
// deployment id, each an object of the options sf recorded.
func readDeployCache() (*orderedMap, error) {
	path, err := deployCachePath()
	if err != nil {
		return nil, err
	}
	data, err := os.ReadFile(path)
	if err != nil {
		if os.IsNotExist(err) {
			return newOrderedMap(), nil
		}
		return nil, err
	}
	parsed, err := parseOrderedJSON(data)
	if err != nil {
		return nil, fmt.Errorf("%s: %w", path, err)
	}
	cache, _ := parsed.(*orderedMap)
	if cache == nil {
		cache = newOrderedMap()
	}
	return cache, nil
}

// deployCacheEntry returns the entry of a deployment and its 18-character
// id.  A 15-character id matches the entry whose id starts with it.
func deployCacheEntry(cache *orderedMap, id string) (*orderedMap, string) {
	if entry, ok := cache.values[id].(*orderedMap); ok {
		return entry, id
	}
	if len(id) == 15 {
		for _, key := range cache.keys {
			if strings.HasPrefix(key, id) {
				if entry, ok := cache.values[key].(*orderedMap); ok {
					return entry, key
				}
			}
		}
	}
	return nil, id
}

// orderedFields builds an ordered map from keys and values given in turn.
func orderedFields(pairs ...interface{}) *orderedMap {
	fields := newOrderedMap()
	for i := 0; i+1 < len(pairs); i += 2 {
		fields.Set(pairs[i].(string), pairs[i+1])
	}
	return fields
}

// recordDeploy adds or updates the cache entry of a deployment.  The other
// entries, the fields of an existing entry and the order of both are kept.
func recordDeploy(id string, fields *orderedMap) error {
	cache, err := readDeployCache()
	if err != nil {
		return err
	}
	entry, key := deployCacheEntry(cache, id)
	if entry == nil {
		entry = newOrderedMap()
	}
	for _, name := range fields.keys {
		entry.Set(name, fields.values[name])
	}
	return writeDeployCacheWith(cache, key, entry)
}

// writeDeployCacheWith stores an entry with the current time and writes
// the cache.
func writeDeployCacheWith(cache *orderedMap, key string, entry *orderedMap) error {
	entry.Set("timestamp", time.Now().UTC().Format("2006-01-02T15:04:05.000Z"))
	cache.Set(key, entry)
	path, err := deployCachePath()
	if err != nil {
		return err
	}
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return err
	}
	var buffer bytes.Buffer
	encoder := json.NewEncoder(&buffer)
	encoder.SetEscapeHTML(false)
	encoder.SetIndent("", "  ")
	if err := encoder.Encode(cache); err != nil {
		return err
	}
	return os.WriteFile(path, bytes.TrimRight(buffer.Bytes(), "\n"), 0o600)
}

// replaceDeployCacheEntry replaces the cache entry of a deployment, keeping
// its place in the cache.
func replaceDeployCacheEntry(id string, entry *orderedMap) error {
	cache, err := readDeployCache()
	if err != nil {
		return err
	}
	_, key := deployCacheEntry(cache, id)
	cache.Set(key, newOrderedMap())
	return writeDeployCacheWith(cache, key, entry)
}

// writeDeployManifest writes the manifest of a deployment to the manifest
// cache of sf, ~/.sf/manifestCache/<id>.xml, and returns its path.  The
// deploy commands read it to find the local files of the deployment.
func writeDeployManifest(id string, source *sourceSet, version string) (string, error) {
	cachePath, err := deployCachePath()
	if err != nil {
		return "", err
	}
	path := filepath.Join(filepath.Dir(cachePath), "manifestCache", id+".xml")
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return "", err
	}
	return path, os.WriteFile(path, packageXML(source.types, version), 0o644)
}

// deployJobSource collects the local files of a deployment from the
// manifest in its cache entry, or from the package directories of the
// project when there is no entry.  It returns nil when the files cannot be
// found: the commands then report the paths the org gives.
func deployJobSource(entry *orderedMap) *sourceSet {
	project, err := loadProject()
	if err != nil {
		return nil
	}
	set := newSourceSet()
	set.ignore = loadForceIgnore(project.Root)
	if entry == nil {
		for _, dir := range project.packageDirs() {
			if set.addPath(dir) != nil {
				return nil
			}
		}
		return set
	}
	if entry.values["isMdapi"] == true {
		return nil
	}
	manifestPath := text(entry.values["manifest"])
	if manifestPath == "" {
		return nil
	}
	manifest, err := readManifest(manifestPath)
	if err != nil {
		return nil
	}
	if err := set.addMetadata(project, manifest.metadataNames()); err != nil {
		return nil
	}
	return set
}

// deployJobID returns the deployment a command acts on, from --job-id or,
// with --use-most-recent, the newest entry of the deploy cache, and the
// cache entry of the deployment or nil.  With mustBeCached, a deployment
// without a cache entry is an error.
func deployJobID(cmd *cobra.Command, mustBeCached bool) (string, *orderedMap, error) {
	id, _ := cmd.Flags().GetString("job-id")
	mostRecent, _ := cmd.Flags().GetBool("use-most-recent")
	if (id == "") == !mostRecent {
		// sf names the flags in the order the command defines them.
		if cmd.Name() == "report" {
			return "", nil, exactlyOneError("job-id", "use-most-recent")
		}
		return "", nil, exactlyOneError("use-most-recent", "job-id")
	}
	cache, err := readDeployCache()
	if err != nil {
		return "", nil, err
	}
	if mostRecent {
		newest := ""
		for _, key := range cache.keys {
			entry, _ := cache.values[key].(*orderedMap)
			if entry == nil {
				continue
			}
			if stamp := text(entry.values["timestamp"]); id == "" || stamp > newest {
				id, newest = key, stamp
			}
		}
		if id == "" {
			return "", nil, newError("NoRecentJobIdError", "There are no recent job IDs available.")
		}
	}
	if len(id) != 15 && len(id) != 18 {
		return "", nil, newError("InvalidJobIdError", "No job found for ID: %s.", id)
	}
	entry, key := deployCacheEntry(cache, id)
	if entry == nil && mustBeCached {
		return "", nil, newError("NoMatchingJobIdError", "No matching job found in cache for ID: %s.", id)
	}
	return key, entry, nil
}

// deployJobOrg returns the org of a deployment: the org in its cache entry,
// then --target-org, then the default org.
func deployJobOrg(cmd *cobra.Command, entry *orderedMap) (*targetOrg, error) {
	if entry != nil {
		if cached := text(entry.values["target-org"]); cached != "" {
			org, err := loadOrg(cached)
			if err != nil {
				return nil, newError("NamedOrgNotFoundError", "No authorization information found for %s.", cached)
			}
			if version := org.Fields.String("instanceApiVersion"); version != "" {
				lib.SetApiVersion(version)
				org.Force.Metadata = lib.NewForceMetadata(org.Force)
			}
			return org, nil
		}
	}
	return resolveTargetOrg(cmd)
}

// deployJob returns the id and the org of the deployment a command acts on.
func deployJob(cmd *cobra.Command) (string, *targetOrg, error) {
	id, entry, err := deployJobID(cmd, false)
	if err != nil {
		return "", nil, err
	}
	org, err := deployJobOrg(cmd, entry)
	return id, org, err
}

// deployCacheFields returns the cache entry sf records for a deploy that
// deploy start or deploy validate started: the flags of the command in the
// order sf defines them, and what the deploy commands need to find the
// deployment again.
func deployCacheFields(cmd *cobra.Command, validate bool, org *targetOrg, manifestPath string, isMdapi bool) *orderedMap {
	entry := newOrderedMap()
	flags := cmd.Flags()
	given := func(name string) bool {
		flag := flags.Lookup(name)
		return flag != nil && flag.Changed
	}
	boolean := func(name string, always bool) {
		if value, err := flags.GetBool(name); err == nil && (always || given(name)) {
			entry.Set(name, value)
		}
	}
	str := func(name string) {
		if given(name) {
			value, _ := flags.GetString(name)
			entry.Set(name, value)
		}
	}
	list := func(name string) {
		if values := splitList(cmd, name); len(values) > 0 {
			entry.Set(name, values)
		}
	}
	boolean("json", false)
	str("api-version")
	boolean("async", false)
	boolean("concise", false)
	if !validate {
		boolean("dry-run", true)
		boolean("ignore-conflicts", true)
		boolean("ignore-errors", true)
		boolean("ignore-warnings", true)
	}
	if given("manifest") && manifestPath != "" {
		entry.Set("manifest", manifestPath)
	}
	list("metadata")
	boolean("single-package", false)
	entry.Set("target-org", org.Username)
	list("tests")
	str("test-level")
	boolean("verbose", false)
	if validate {
		boolean("ignore-warnings", true)
	} else {
		boolean("purge-on-delete", false)
		str("pre-destructive-changes")
		str("post-destructive-changes")
	}
	list("coverage-formatters")
	boolean("junit", false)
	str("results-dir")
	if validate {
		entry.Set("ignore-conflicts", true)
		entry.Set("dry-run", true)
	}
	entry.Set("api", "SOAP")
	if manifestPath != "" {
		entry.Set("manifest", manifestPath)
	}
	minutes, _ := flags.GetInt("wait")
	entry.Set("wait", minutes)
	entry.Set("isMdapi", isMdapi)
	return entry
}

func newProjectDeployStartCmd(validate bool) *cobra.Command {
	use, short, className, title := "start", "Deploy metadata to an org from your local project.", "DeployMetadata", "Deploying Metadata"
	if validate {
		use, short, className, title = "validate", "Validate a metadata deployment without actually executing it.", "DeployMetadataValidate", "Validating Deployment"
	}
	cmd := &cobra.Command{
		Use:         use,
		Short:       short,
		Args:        cobra.NoArgs,
		Annotations: map[string]string{commandNameAnnotation: className},
		RunE: func(cmd *cobra.Command, args []string) error {
			if _, err := readDeployReportFlags(cmd); err != nil {
				return err
			}
			// Destructive change manifests are deployed under the file
			// names the Metadata API reads.
			destructive := lib.ForceMetadataFiles{}
			for flag, name := range map[string]string{"pre-destructive-changes": "destructiveChangesPre.xml", "post-destructive-changes": "destructiveChangesPost.xml"} {
				path, _ := cmd.Flags().GetString(flag)
				if path == "" {
					continue
				}
				if manifest, _ := cmd.Flags().GetString("manifest"); manifest == "" {
					return &Error{Message: fmt.Sprintf("The following error occurred:\n  All of the following must be provided when using --%s: --manifest\nSee more help with --help", flag), ExitCode: 2}
				}
				data, err := os.ReadFile(path)
				if err != nil {
					return err
				}
				destructive[name] = data
			}
			testLevel, _ := cmd.Flags().GetString("test-level")
			tests := splitList(cmd, "tests")
			switch testLevel {
			case "", "NoTestRun", "RunSpecifiedTests", "RunLocalTests", "RunAllTestsInOrg", "RunRelevantTests":
			default:
				return &Error{Message: fmt.Sprintf("Expected --test-level=%s to be one of: NoTestRun, RunSpecifiedTests, RunLocalTests, RunAllTestsInOrg, RunRelevantTests\nSee more help with --help", testLevel), ExitCode: 1}
			}
			if testLevel == "RunSpecifiedTests" && len(tests) == 0 {
				return newError("NoTestsSpecifiedError", "You must specify tests using the --tests flag if the --test-level flag is set to RunSpecifiedTests.")
			}
			if testLevel == "" {
				switch {
				case len(tests) > 0:
					testLevel = "RunSpecifiedTests"
				case validate:
					testLevel = "RunLocalTests"
				}
			}
			metadataDir, _ := cmd.Flags().GetString("metadata-dir")
			org, err := resolveTargetOrg(cmd)
			if err != nil {
				return err
			}
			started := time.Now()
			var (
				source  *sourceSet
				files   lib.ForceMetadataFiles
				version = "v" + lib.ApiVersionNumber()
				// tracking is set for a deploy from a project to an org
				// that tracks source.
				tracking      *sourceTracking
				trackingState *trackingState
				deletedKeys   []string
			)
			if metadataDir != "" {
				files, err = readMetadataDir(metadataDir)
				if err != nil {
					return err
				}
				version = "<version specified in manifest>"
			} else {
				project, err := loadProject()
				if err != nil {
					return err
				}
				ignoreConflicts, _ := cmd.Flags().GetBool("ignore-conflicts")
				specified := len(splitList(cmd, "source-dir")) > 0 || len(splitList(cmd, "metadata")) > 0 || cmd.Flags().Changed("manifest")
				if tracksSource(org) {
					tracking = newSourceTracking(project, org)
					trackingState, err = tracking.state(true)
					if err != nil {
						return err
					}
				}
				if !specified && tracking != nil {
					// Deploy the components changed since the last deploy
					// or retrieve.
					changes := trackingState.localChangeSet(project.Root, loadForceIgnore(project.Root))
					if len(changes.deploy) == 0 && len(changes.remove) == 0 {
						result := newOrderedMap()
						result.Set("status", "Nothing to deploy")
						result.Set("files", []interface{}{})
						return emit(cmd, result, func(w io.Writer) { fmt.Fprintln(w, "No changes to deploy") })
					}
					source = newSourceSet()
					source.ignore = loadForceIgnore(project.Root)
					var names []string
					for _, component := range changes.deploy {
						names = append(names, component.metadataName())
					}
					if len(names) > 0 {
						if err := source.addMetadata(project, names); err != nil {
							return err
						}
					}
					if len(changes.remove) > 0 {
						source.deletedPaths = map[string][]string{}
						for _, component := range changes.remove {
							source.deletedPaths[component.key()] = component.files
							deletedKeys = append(deletedKeys, component.key())
						}
						destructive["destructiveChangesPost.xml"] = destructiveChanges(changes.remove, projectAPIVersion(project))
					}
				} else {
					source, err = collectSource(cmd, project, "source-dir")
					if err != nil {
						return err
					}
				}
				// A validation does not change the org and ignores conflicts.
				if tracking != nil && !ignoreConflicts && !validate {
					keys := append(source.componentKeys(), deletedKeys...)
					if rows := trackingState.conflictRows(project.Root, keys); len(rows) > 0 {
						return conflictError(cmd, rows, true)
					}
				}
				if len(source.files) == 0 && len(deletedKeys) == 0 {
					return &Error{
						Name:     "NothingToDeploy",
						Message:  "No local changes to deploy.",
						ExitCode: 1,
						Actions:  []string{"To see conflicts and ignored files, run \"" + binName + " project deploy preview\" with any of the manifest, directory, or metadata flags."},
					}
				}
				manifestVersion := projectAPIVersion(project)
				version = "v" + manifestVersion
				files = source.deployFiles(manifestVersion)
			}
			for name, data := range destructive {
				files[name] = data
			}
			archive, err := zipFiles(files, "")
			if err != nil {
				return err
			}
			options := lib.ForceDeployOptions{
				CheckOnly:       validate,
				RollbackOnError: true,
				SinglePackage:   true,
				TestLevel:       testLevel,
				RunTests:        tests,
			}
			if dryRun, _ := cmd.Flags().GetBool("dry-run"); dryRun {
				options.CheckOnly = true
			}
			if ignoreErrors, _ := cmd.Flags().GetBool("ignore-errors"); ignoreErrors {
				options.RollbackOnError = false
			}
			options.IgnoreWarnings, _ = cmd.Flags().GetBool("ignore-warnings")
			options.PurgeOnDelete, _ = cmd.Flags().GetBool("purge-on-delete")
			subtitle := fmt.Sprintf("Deploying %s metadata to %s using the v%s SOAP API.", version, org.Username, lib.ApiVersionNumber())
			out := cmd.OutOrStdout()
			if validate {
				if !jsonRequested(cmd) {
					fmt.Fprintf(out, "Validating Deployment of %s metadata to %s using the v%s SOAP API.\n", version, org.Username, lib.ApiVersionNumber())
				}
				subtitle = ""
			}
			prepare := time.Since(started)
			id, err := org.Force.Metadata.StartDeploy(files, options)
			if err != nil {
				return apiError(err)
			}
			manifestPath := ""
			if source != nil {
				manifestPath, err = writeDeployManifest(id, source, strings.TrimPrefix(version, "v"))
				if err != nil {
					return err
				}
			}
			if err := recordDeploy(id, deployCacheFields(cmd, validate, org, manifestPath, metadataDir != "")); err != nil {
				return err
			}
			async, _ := cmd.Flags().GetBool("async")
			if async {
				state := &deployState{status: lib.ForceCheckDeploymentStatusResult{Id: id, Status: "Queued"}}
				result := newOrderedMap()
				result.Set("id", id)
				result.Set("done", false)
				result.Set("status", "Queued")
				result.Set("files", []interface{}{})
				result.Set("zipSize", len(archive))
				result.Set("zipFileCount", len(files))
				result.Set("deployUrl", deployURL(org, id))
				return emit(cmd, result, func(w io.Writer) {
					deployProgress(w, org, title, subtitle, state, true, prepare, 0)
					fmt.Fprintln(w, "Deploy has been queued.")
					fmt.Fprintln(w)
					fmt.Fprintf(w, "Run \""+binName+" project deploy resume --job-id %s\" to resume watching the deploy.\n", id)
					fmt.Fprintf(w, "Run \""+binName+" project deploy report --job-id %s\" to get the latest status.\n", id)
					fmt.Fprintf(w, "Run \""+binName+" project deploy cancel --job-id %s\" to cancel the deploy.\n", id)
				})
			}
			minutes, _ := cmd.Flags().GetInt("wait")
			state, err := waitForDeploy(org, id, time.Duration(minutes)*time.Minute)
			var tracked *trackingProgress
			if err == nil && tracking != nil && !options.CheckOnly && state.status.Status == "Succeeded" {
				// Record what was deployed in the source tracking files.
				var deployed, deleted, unchanged []string
				for _, success := range state.status.Details.ComponentSuccesses {
					key := success.ComponentType + "###" + success.FullName
					switch {
					case success.ComponentType == "":
					case success.Deleted:
						deleted = append(deleted, key)
					default:
						deployed = append(deployed, key)
						if !success.Changed && !success.Created {
							unchanged = append(unchanged, key)
						}
					}
				}
				tracked, err = tracking.recordDeploy(trackingState, deployed, deleted, unchanged...)
				if err != nil {
					return err
				}
				if tracked.warning != "" {
					warn(cmd, tracked.warning)
				}
			}
			if state != nil && !jsonRequested(cmd) {
				if verbose, _ := cmd.Flags().GetBool("verbose"); verbose {
					state.verboseInfo = [][2]string{{"Deploy URL", deployURL(org, id)}}
					if !validate {
						state.verboseInfo = append(state.verboseInfo, [2]string{"Size", zipFileSize(len(archive)) + " of ~39 MB limit"}, [2]string{"Files", fmt.Sprintf("%d of 10,000 limit", len(files))})
					}
				}
				deployProgress(out, org, title, subtitle, state, false, prepare, time.Since(started), tracked)
			}
			if err != nil {
				return err
			}
			if err := recordDeploy(id, orderedFields("status", state.status.Status)); err != nil {
				return err
			}
			zipSize, zipFileCount := len(archive), len(files)
			if validate {
				// sf reports the size the org gives for a validation.
				zipSize, zipFileCount = 0, 0
			}
			return finishDeploy(cmd, org, state, source, zipSize, zipFileCount, func(w io.Writer) {
				if validate {
					fmt.Fprintln(w)
					fmt.Fprintln(w, paint(w, styleGreen, fmt.Sprintf("Successfully validated the deployment (%s).", id)))
					fmt.Fprintf(w, "Run \""+binName+" project deploy quick --job-id %s\" to execute this deploy\n", id)
				}
			})
		},
	}
	addJSONFlag(cmd)
	cmd.Flags().StringP("api-version", "a", "", "Target API version for the deploy.")
	cmd.Flags().Bool("async", false, "Run the command asynchronously.")
	cmd.Flags().Bool("concise", false, "Show concise output of the deploy result.")
	cmd.Flags().BoolP("ignore-warnings", "g", false, "Ignore warnings and allow a deployment to complete successfully.")
	cmd.Flags().StringP("manifest", "x", "", "Full file path for manifest (package.xml) of components to deploy.")
	cmd.Flags().StringArrayP("metadata", "m", nil, "Metadata component names to deploy.")
	cmd.Flags().String("metadata-dir", "", "Root of directory or zip file of metadata formatted files to deploy.")
	cmd.Flags().Bool("single-package", false, "Indicates that the metadata zip file points to a directory structure for a single package.")
	cmd.Flags().StringArrayP("source-dir", "d", nil, "Path to the local source files to deploy.")
	cmd.Flags().StringArrayP("tests", "t", nil, "Apex tests to run when --test-level is RunSpecifiedTests.")
	cmd.Flags().StringP("test-level", "l", "", "Deployment Apex testing level.")
	cmd.Flags().Bool("verbose", false, "Show verbose output of the deploy result.")
	cmd.Flags().IntP("wait", "w", 33, "Number of minutes to wait for command to complete and display results.")
	cmd.Flags().Bool("purge-on-delete", false, "Specify that deleted components in the destructive changes manifest file are immediately eligible for deletion.")
	cmd.Flags().String("pre-destructive-changes", "", "File path for a manifest of components to delete before the deploy.")
	cmd.Flags().String("post-destructive-changes", "", "File path for a manifest of components to delete after the deploy.")
	addDeployReportFlags(cmd)
	if !validate {
		cmd.Flags().Bool("dry-run", false, "Validate deploy and run Apex tests but don’t save to the org.")
		cmd.Flags().BoolP("ignore-conflicts", "c", false, "Ignore conflicts and deploy local files, even if they overwrite changes in the org.")
		cmd.Flags().BoolP("ignore-errors", "r", false, "Ignore any errors and don’t roll back deployment.")
	}
	addTargetOrgFlag(cmd)
	return cmd
}

func newProjectDeployResumeCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:         "resume",
		Short:       "Resume watching a deploy operation and update source tracking when the deploy completes.",
		Args:        cobra.NoArgs,
		Annotations: map[string]string{commandNameAnnotation: "DeployMetadataResume"},
		RunE: func(cmd *cobra.Command, args []string) error {
			if _, err := readDeployReportFlags(cmd); err != nil {
				return err
			}
			id, entry, err := deployJobID(cmd, true)
			if err != nil {
				return err
			}
			org, err := deployJobOrg(cmd, entry)
			if err != nil {
				return err
			}
			// --verbose adds the deploy URL to the progress display.  The
			// tables follow the verbosity of the command that started
			// the deploy, as in sf.
			stageVerbose, _ := cmd.Flags().GetBool("verbose")
			stageVerbose = stageVerbose || entry.values["verbose"] == true
			for _, name := range []string{"verbose", "concise"} {
				value := "false"
				if entry.values[name] == true {
					value = "true"
				}
				cmd.Flags().Set(name, value)
			}
			started := time.Now()
			source := deployJobSource(entry)
			if source == nil {
				source = &sourceSet{noFiles: true}
			}
			switch status := text(entry.values["status"]); status {
			case "Succeeded", "SucceededPartial", "Failed", "Canceled":
				// The deploy is known to be finished: report it.
				warn(cmd, fmt.Sprintf("Job ID %s is not resumable because it already completed with status: %s. Displaying results...", id, status))
				state, err := checkDeploy(org, id)
				if err != nil {
					return err
				}
				return finishDeploy(cmd, org, state, source, 0, 0, nil)
			}
			first, err := checkDeploy(org, id)
			if err != nil {
				return err
			}
			state := first
			if !first.status.Done {
				minutes, _ := cmd.Flags().GetInt("wait")
				if !cmd.Flags().Changed("wait") {
					if cached := intValue(entry.values["wait"]); cached > 0 {
						minutes = cached
					}
				}
				state, err = waitForDeploy(org, id, time.Duration(minutes)*time.Minute)
				if err != nil {
					if state != nil && !jsonRequested(cmd) {
						deployProgress(cmd.OutOrStdout(), org, "Resuming Deploy", "", state, false, 0, time.Since(started))
					}
					return err
				}
			}
			// A deploy that finished updates the source tracking files,
			// as the command that started it would have.
			var tracked *trackingProgress
			if state.status.Status == "Succeeded" && !state.status.CheckOnly && entry.values["isMdapi"] != true && tracksSource(org) {
				if project, projectErr := loadProject(); projectErr == nil {
					tracking := newSourceTracking(project, org)
					// The org is not asked for its changes here: the wait
					// for the source members of the deploy starts from the
					// revision that was recorded before the deploy.
					trackingState, err := tracking.state(false)
					if err != nil {
						return err
					}
					var deployed, deleted, unchanged []string
					for _, success := range state.status.Details.ComponentSuccesses {
						key := success.ComponentType + "###" + success.FullName
						switch {
						case success.ComponentType == "":
						case success.Deleted:
							deleted = append(deleted, key)
						default:
							deployed = append(deployed, key)
							if !success.Changed && !success.Created {
								unchanged = append(unchanged, key)
							}
						}
					}
					tracked, err = tracking.recordDeploy(trackingState, deployed, deleted, unchanged...)
					if err != nil {
						return err
					}
					if tracked.warning != "" {
						warn(cmd, tracked.warning)
					}
				}
			}
			if !jsonRequested(cmd) {
				if stageVerbose {
					state.verboseInfo = [][2]string{{"Deploy URL", deployURL(org, id)}}
				}
				state.finishedBefore = first.status.Done
				deployProgress(cmd.OutOrStdout(), org, "Resuming Deploy", "", state, false, 0, time.Since(started), tracked)
			}
			if err := recordDeploy(id, orderedFields("status", state.status.Status)); err != nil {
				return err
			}
			return finishDeploy(cmd, org, state, source, 0, 0, nil)
		},
	}
	addDeployJobFlags(cmd)
	cmd.Flags().Bool("concise", false, "Show concise output of the deploy result.")
	cmd.Flags().Bool("verbose", false, "Show verbose output of the deploy result.")
	addDeployReportFlags(cmd)
	return cmd
}

func newProjectDeployReportCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:         "report",
		Short:       "Check or poll for the status of a deploy operation.",
		Args:        cobra.NoArgs,
		Annotations: map[string]string{commandNameAnnotation: "DeployMetadataReport"},
		RunE: func(cmd *cobra.Command, args []string) error {
			reports, err := readDeployReportFlags(cmd)
			if err != nil {
				return err
			}
			id, entry, err := deployJobID(cmd, false)
			if err != nil {
				return err
			}
			org, err := deployJobOrg(cmd, entry)
			if err != nil {
				if e, ok := err.(*Error); ok && e.Name == "NoDefaultEnvError" {
					return newError("NoOrgError", "No environment found. Use -o or --target-org to specify an environment")
				}
				return err
			}
			// The local files are looked up unless the deploy is known to
			// have succeeded.
			var source *sourceSet
			if entry == nil || text(entry.values["status"]) != "Succeeded" {
				source = deployJobSource(entry)
			}
			if source == nil {
				source = &sourceSet{noFiles: true}
			}
			started := time.Now()
			var state *deployState
			if cmd.Flags().Changed("wait") {
				minutes, _ := cmd.Flags().GetInt("wait")
				first, checkErr := checkDeploy(org, id)
				if checkErr != nil {
					return checkErr
				}
				state, err = waitForDeploy(org, id, time.Duration(minutes)*time.Minute)
				if state != nil && !jsonRequested(cmd) {
					state.finishedBefore = first.status.Done
					deployProgress(cmd.OutOrStdout(), org, "Deploying Metadata", "", state, false, 0, time.Since(started))
				}
				if err != nil {
					// The time is over: report the status as it is.
					state, err = checkDeploy(org, id)
				}
			} else {
				state, err = checkDeploy(org, id)
			}
			if err != nil {
				return err
			}
			if reports.requested() {
				if err := reports.write(org, state.status.Details.RunTestResult); err != nil {
					return err
				}
			}
			// The result of a report has no deploy URL.
			result := deployJSON(org, state, source, 0, 0)
			result.Delete("deployUrl")
			return emit(cmd, result, func(w io.Writer) {
				fmt.Fprintf(w, "%s... %s\n", state.status.Id, state.status.Status)
				var rows [][]string
				for _, key := range state.raw.keys {
					switch value := state.raw.values[key].(type) {
					case string, bool, json.Number:
						rows = append(rows, []string{key, displayValue(value)})
					}
				}
				fmt.Fprintln(w)
				printTable(w, paint(w, style{"\x1b[1m\x1b[34m", "\x1b[39m\x1b[22m"}, "Deploy Info"), []string{"Key", "Value"}, rows)
				fmt.Fprintln(w)
				printTable(w, paint(w, style{"\x1b[1m\x1b[34m", "\x1b[39m\x1b[22m"}, "Deploy Options"), []string{"Key", "Value"}, deployOptionRows(cmd, entry, org))
				writeDeployTables(w, state, source, false)
				if len(reports.formats) > 0 {
					fmt.Fprintf(w, "Code Coverage formats, [%s], written to %s/\n", strings.Join(reports.formats, ", "), filepath.Join(reports.resultsDir, "coverage"))
				}
				if reports.junit {
					fmt.Fprintf(w, "Junit results written to %s/junit/junit.xml\n", reports.resultsDir)
				}
			})
		},
	}
	addDeployJobFlags(cmd)
	addDeployReportFlags(cmd)
	return cmd
}

// deployOptionRows lists the options of a deployment for deploy report:
// the cache entry of the deployment, then the flags of the report command
// in the order sf defines them.  A flag replaces the option of the same
// name in place.
func deployOptionRows(cmd *cobra.Command, entry *orderedMap, org *targetOrg) [][]string {
	options := newOrderedMap()
	if entry != nil {
		for _, key := range entry.keys {
			if key != "timestamp" {
				options.Set(key, entry.values[key])
			}
		}
	}
	if targetOrgFlag(cmd) != "" {
		options.Set("target-org", org.Username)
	}
	if id, _ := cmd.Flags().GetString("job-id"); id != "" {
		options.Set("job-id", id)
	}
	if mostRecent, _ := cmd.Flags().GetBool("use-most-recent"); mostRecent {
		options.Set("use-most-recent", true)
	}
	if formats := splitList(cmd, "coverage-formatters"); len(formats) > 0 {
		options.Set("coverage-formatters", formats)
	}
	if junit, _ := cmd.Flags().GetBool("junit"); junit {
		options.Set("junit", true)
	}
	if dir, _ := cmd.Flags().GetString("results-dir"); dir != "" {
		options.Set("results-dir", dir)
	}
	if cmd.Flags().Changed("wait") {
		minutes, _ := cmd.Flags().GetInt("wait")
		options.Set("wait", minutes)
	}
	options.Set("target-org", org.Username)
	rows := make([][]string, 0, len(options.keys))
	for _, key := range options.keys {
		value := options.values[key]
		text := displayValue(value)
		switch v := value.(type) {
		case []interface{}:
			parts := make([]string, 0, len(v))
			for _, item := range v {
				parts = append(parts, displayValue(item))
			}
			text = strings.Join(parts, ",")
		case []string:
			text = strings.Join(v, ",")
		}
		if key == "wait" {
			text += " minutes"
		}
		rows = append(rows, []string{key, text})
	}
	return rows
}

func newProjectDeployQuickCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:         "quick",
		Short:       "Quickly deploy a validated deployment to an org.",
		Args:        cobra.NoArgs,
		Annotations: map[string]string{commandNameAnnotation: "DeployMetadataQuick"},
		RunE: func(cmd *cobra.Command, args []string) error {
			// The id of the validation.  One that is not in the cache
			// is used as it was given.
			id, entry, err := deployJobID(cmd, false)
			if err != nil {
				return err
			}
			if mostRecent, _ := cmd.Flags().GetBool("use-most-recent"); mostRecent && entry == nil {
				return newError("NoMatchingJobIdError", "No matching job found in cache for ID: %s.", id)
			}
			org, err := deployJobOrg(cmd, entry)
			if err != nil {
				if e, ok := err.(*Error); ok && e.Name == "NoDefaultEnvError" {
					return newError("NoTargetOrgError", "No target org found in cache, from a flag, or in the environment.")
				}
				return err
			}
			// The org gives the deploy of the validated metadata an id of
			// its own.
			quick, err := org.Force.Metadata.DeployRecentValidation(id)
			if err != nil {
				if strings.Contains(err.Error(), "INVALID_ID_FIELD") {
					return newError("CannotQuickDeployError", "Job ID can't be used for quick deployment. Possible reasons include the deployment hasn't been validated, has already been deployed, or the validation expired because you ran it more than 10 days ago.")
				}
				return apiError(err)
			}
			deployID := quick.Id
			if deployID == "" {
				deployID = id
			}
			out := cmd.OutOrStdout()
			if !jsonRequested(cmd) {
				fmt.Fprintf(out, "Deploy ID: %s\n", deployID)
				fmt.Fprintf(out, "Deploy URL: %s\n", deployURL(org, deployID))
			}
			if async, _ := cmd.Flags().GetBool("async"); async {
				result := newOrderedMap()
				result.Set("id", deployID)
				result.Set("done", false)
				result.Set("status", "Queued")
				result.Set("files", []interface{}{})
				result.Set("deployUrl", deployURL(org, deployID))
				return emit(cmd, result, func(w io.Writer) {
					fmt.Fprintln(w, "Deploy has been queued.")
					fmt.Fprintln(w)
					fmt.Fprintf(w, "Run \""+binName+" project deploy resume --job-id %s\" to resume watching the deploy.\n", deployID)
					fmt.Fprintf(w, "Run \""+binName+" project deploy report --job-id %s\" to get the latest status.\n", deployID)
					fmt.Fprintf(w, "Run \""+binName+" project deploy cancel --job-id %s\" to cancel the deploy.\n", deployID)
				})
			}
			minutes, _ := cmd.Flags().GetInt("wait")
			state, err := waitForDeploy(org, deployID, time.Duration(minutes)*time.Minute)
			if err != nil {
				return err
			}
			if err := recordDeploy(deployID, orderedFields("status", state.status.Status)); err != nil {
				return err
			}
			status := 0
			if state.status.Status != "Succeeded" {
				status = 1
			}
			concise, _ := cmd.Flags().GetBool("concise")
			verbose, _ := cmd.Flags().GetBool("verbose")
			if err := emitStatus(cmd, status, deployJSON(org, state, nil, 0, 0), func(w io.Writer) {
				writeDeployTables(w, state, nil, concise, verbose)
				if status == 0 {
					fmt.Fprintln(w)
					fmt.Fprintln(w, paint(w, styleGreen, fmt.Sprintf("Successfully deployed (%s).", deployID)))
				} else {
					fmt.Fprintf(w, "Deployment %s exited with status code: %s.\n", deployID, state.status.Status)
				}
			}); err != nil {
				return err
			}
			if status != 0 {
				return exitStatus(status)
			}
			return nil
		},
	}
	addDeployJobFlags(cmd)
	cmd.Flags().Bool("async", false, "Run the command asynchronously.")
	cmd.Flags().Bool("concise", false, "Show concise output of the deploy result.")
	cmd.Flags().Bool("verbose", false, "Show verbose output of the deploy result.")
	cmd.Flags().StringP("api-version", "a", "", "Target API version for the deploy.")
	return cmd
}

func newProjectDeployCancelCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:         "cancel",
		Short:       "Cancel a deploy operation.",
		Args:        cobra.NoArgs,
		Annotations: map[string]string{commandNameAnnotation: "DeployMetadataCancel"},
		RunE: func(cmd *cobra.Command, args []string) error {
			id, entry, err := deployJobID(cmd, false)
			if err != nil {
				return err
			}
			org, err := deployJobOrg(cmd, entry)
			if err != nil {
				return err
			}
			async, _ := cmd.Flags().GetBool("async")
			if !async {
				// Like sf, record the options of the cancel as the cache
				// entry of the deploy: the org first and the wait last.
				options := orderedFields("target-org", org.Username)
				if entry != nil {
					for _, key := range entry.keys {
						switch key {
						case "wait", "timestamp", "metadata-dir", "source-dir":
						default:
							options.Set(key, entry.values[key])
						}
					}
				}
				options.Set("ignore-conflicts", true)
				minutes := 33
				if entry != nil && intValue(entry.values["wait"]) > 0 {
					minutes = intValue(entry.values["wait"])
				}
				if cmd.Flags().Changed("wait") {
					minutes, _ = cmd.Flags().GetInt("wait")
				}
				options.Set("wait", minutes)
				if _, present := options.values["isMdapi"]; !present {
					options.Set("isMdapi", false)
				}
				if err := replaceDeployCacheEntry(id, options); err != nil {
					return err
				}
			}
			if _, err := org.Force.Metadata.CancelDeploy(id); err != nil {
				if err == lib.AlreadyCompletedError {
					return &Error{
						Name:     "CannotCancelDeployError",
						Message:  "Can't cancel deploy because it's already completed.",
						ExitCode: 1,
						Data:     orderedFields("errorCode", "sf:INVALID_ID_FIELD", "message", "INVALID_ID_FIELD: Deployment already completed"),
					}
				}
				return apiError(err)
			}
			if async {
				result := newOrderedMap()
				result.Set("id", id)
				return emit(cmd, result, func(w io.Writer) {
					fmt.Fprintln(w, "Deploy has been queued for cancellation.")
					fmt.Fprintln(w)
					fmt.Fprintf(w, "Run \""+binName+" project deploy resume --job-id %s\" to resume watching the deploy.\n", id)
					fmt.Fprintf(w, "Run \""+binName+" project deploy report --job-id %s\" to get the latest status.\n", id)
				})
			}
			minutes, _ := cmd.Flags().GetInt("wait")
			state, err := waitForDeploy(org, id, time.Duration(minutes)*time.Minute)
			if err != nil {
				return err
			}
			if err := recordDeploy(id, orderedFields("status", state.status.Status)); err != nil {
				return err
			}
			status := 0
			if state.status.Status != "Canceled" {
				status = 1
			}
			if err := emitStatus(cmd, status, deployJSON(org, state, nil, 0, 0), func(w io.Writer) {
				if status == 0 {
					fmt.Fprintf(w, "Successfully canceled %s\n", id)
				} else {
					fmt.Fprintf(w, "Could not cancel %s\n", id)
				}
			}); err != nil {
				return err
			}
			if status != 0 {
				return exitStatus(status)
			}
			return nil
		},
	}
	addDeployJobFlags(cmd)
	cmd.Flags().Bool("async", false, "Run the command asynchronously.")
	return cmd
}

// retrieveJSON returns the fields of the retrieve responses that sf reports,
// and the messages of the org about components it did not retrieve.  A
// retrieve that was split into several requests is reported as the first
// response with the file properties and messages of all of them.
func retrieveJSON(responses []*orderedMap, problems []string) (*orderedMap, []*orderedMap) {
	result := newOrderedMap()
	var properties []interface{}
	messages := []*orderedMap{}
	list := func(value interface{}) []interface{} {
		switch v := value.(type) {
		case nil:
			return nil
		case []interface{}:
			return v
		}
		return []interface{}{value}
	}
	for i, response := range responses {
		properties = append(properties, list(response.values["fileProperties"])...)
		for _, message := range list(response.values["messages"]) {
			if object, ok := message.(*orderedMap); ok {
				messages = append(messages, object)
			}
		}
		if i > 0 {
			continue
		}
		for _, key := range response.keys {
			switch key {
			case "zipFile":
			case "done", "success":
				result.Set(key, response.values[key] == "true")
			default:
				result.Set(key, response.values[key])
			}
		}
	}
	if len(responses) == 0 {
		// The org was not asked, or the response could not be read.
		result.Set("done", true)
		result.Set("status", "Succeeded")
		result.Set("success", true)
		for _, problem := range problems {
			message := newOrderedMap()
			message.Set("problem", problem)
			messages = append(messages, message)
		}
		return result, messages
	}
	if properties == nil {
		properties = []interface{}{}
	}
	if _, present := result.values["fileProperties"]; present {
		result.Set("fileProperties", properties)
	}
	// The messages keep their place in the response; sf adds an empty
	// list at the end when the org sent none.
	result.Set("messages", messages)
	return result, messages
}

var retrieveProblemPattern = regexp.MustCompile(`.+'(.+)'.+'(.+)'`)

// retrieveFailures returns the entries sf lists in "files" for the
// components the org reported a problem with.
func retrieveFailures(messages []*orderedMap) []*orderedMap {
	var failures []*orderedMap
	for _, message := range messages {
		problem := text(message.values["problem"])
		item := newOrderedMap()
		if match := retrieveProblemPattern.FindStringSubmatch(problem); match != nil {
			item.Set("fullName", match[2])
			item.Set("type", match[1])
			item.Set("state", "Failed")
			item.Set("error", problem)
			item.Set("problemType", "Error")
		} else {
			item.Set("fullName", "")
			item.Set("type", "")
			item.Set("problemType", "Error")
			item.Set("state", "Failed")
			item.Set("error", problem)
		}
		failures = append(failures, item)
	}
	return failures
}

func newProjectRetrieveStartCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:         "start",
		Short:       "Retrieve metadata from an org to your local project.",
		Args:        cobra.NoArgs,
		Annotations: map[string]string{commandNameAnnotation: "RetrieveMetadata"},
		RunE: func(cmd *cobra.Command, args []string) error {
			metadata := splitList(cmd, "metadata")
			sourceDirs := splitList(cmd, "source-dir")
			packageNames := splitList(cmd, "package-name")
			manifest, _ := cmd.Flags().GetString("manifest")
			targetMetadataDir, _ := cmd.Flags().GetString("target-metadata-dir")
			outputDir, _ := cmd.Flags().GetString("output-dir")
			// With --single-package the zip file has the files of the one
			// package at its top; without it they are in a directory named
			// unpackaged.
			zipPrefix := "unpackaged/"
			if flagSet(cmd, "single-package") {
				if targetMetadataDir == "" {
					return &Error{Message: "The following error occurred:\n  All of the following must be provided when using --single-package: --target-metadata-dir\nSee more help with --help", ExitCode: 2}
				}
				zipPrefix = ""
			}
			if len(packageNames) > 1 {
				return newError("UnsupportedFlagError", "Only one --package-name value is supported by the sf-compatible mode of force.")
			}
			org, err := resolveTargetOrg(cmd)
			if err != nil {
				return err
			}
			var project *sfProject
			if targetMetadataDir == "" || len(sourceDirs) > 0 {
				project, err = loadProject()
				if err != nil {
					return err
				}
			}
			// A retrieve in metadata format does not need a project, and
			// uses the API version of one when there is one.
			versionProject := project
			if versionProject == nil {
				versionProject, _ = loadProject()
			}
			var query lib.ForceMetadataQuery
			for _, name := range metadata {
				metadataType, member, hasMember := strings.Cut(name, ":")
				if !hasMember {
					member = "*"
				}
				query = append(query, lib.ForceMetadataQueryElement{Name: []string{metadataType}, Members: []string{member}})
			}
			if manifest != "" {
				parsed, err := readManifest(manifest)
				if err != nil {
					return err
				}
				query = append(query, parsed.query()...)
			}
			if len(sourceDirs) > 0 {
				// Retrieve the components that exist in the directories.
				set := newSourceSet()
				for _, dir := range sourceDirs {
					if err := set.addPath(dir); err != nil {
						return err
					}
				}
				names := make([]string, 0, len(set.types))
				for name := range set.types {
					names = append(names, name)
				}
				sort.Strings(names)
				for _, name := range names {
					query = append(query, lib.ForceMetadataQueryElement{Name: []string{name}, Members: set.types[name]})
				}
			}
			// tracking is set for a retrieve into a project from an org
			// that tracks source.
			var (
				tracking      *sourceTracking
				trackingState *trackingState
				retrieveKeys  []string
				remoteDeletes []*sourceComponent
			)
			started := time.Now()
			out := cmd.OutOrStdout()
			earlyFrame := func(sending, status string) {
				if !jsonRequested(cmd) {
					writeStages(out, "Retrieving Metadata", "", []stage{
						{label: "Preparing retrieve request", state: "done", elapsed: time.Since(started)},
						{label: "Sending request to org", state: sending},
						{label: "Waiting for the org to respond"},
						{label: "Done"},
					}, [][2]string{{"Status", status}, {"Elapsed Time", formatElapsed(0)}})
				}
			}
			if targetMetadataDir == "" && project != nil && len(packageNames) == 0 && tracksSource(org) {
				tracking = newSourceTracking(project, org)
				trackingState, err = tracking.state(true)
				if err != nil {
					return err
				}
				if len(query) == 0 {
					// Retrieve the components changed in the org since
					// the last deploy or retrieve, and delete the local
					// files of the ones deleted there.
					byType := map[string][]string{}
					var types []string
					for _, member := range trackingState.outOfSync {
						retrieveKeys = append(retrieveKeys, member.key())
						if member.IsNameObsolete {
							if component := trackingState.components[member.key()]; component != nil && trackingState.exists(component) {
								remoteDeletes = append(remoteDeletes, component)
							}
							continue
						}
						if _, seen := byType[member.Type]; !seen {
							types = append(types, member.Type)
						}
						byType[member.Type] = append(byType[member.Type], member.Name)
					}
					for _, metadataType := range types {
						query = append(query, lib.ForceMetadataQueryElement{Name: []string{metadataType}, Members: byType[metadataType]})
					}
					if len(query) == 0 && len(remoteDeletes) == 0 {
						result := newOrderedMap()
						result.Set("files", []interface{}{})
						// sf stops its progress display here before it
						// is complete.
						earlyFrame("done", "⠁")
						return emit(cmd, result, func(w io.Writer) { fmt.Fprintln(w, "Nothing retrieved") })
					}
				} else {
					for _, element := range query {
						for _, member := range element.Members {
							for _, name := range element.Name {
								retrieveKeys = append(retrieveKeys, name+"###"+member)
							}
						}
					}
				}
				if ignoreConflicts, _ := cmd.Flags().GetBool("ignore-conflicts"); !ignoreConflicts {
					if rows := trackingState.conflictRows(project.Root, retrieveKeys); len(rows) > 0 {
						earlyFrame("failed", "Failed")
						return conflictError(cmd, rows, false)
					}
				}
			}
			if len(query) == 0 && len(packageNames) == 0 && len(remoteDeletes) == 0 {
				return &Error{
					Name:     "NoSourceTrackingError",
					Message:  "Unable to track changes in your source files.\nThis command expects the org to support source tracking. If it doesn't, you must specify the metadata you want to retrieve.",
					ExitCode: 1,
					Actions: []string{
						"Use the `--source-dir`, `--manifest` or `--package-name` flags to retrieve metadata in source format.",
						"Use the `--target-metadata-dir` flag to retrieve metadata in metadata format to a directory.",
					},
				}
			}
			version := lib.ApiVersionNumber()
			sourceVersion := version
			if versionProject != nil {
				sourceVersion = projectAPIVersion(versionProject)
			}
			subtitle := fmt.Sprintf("Retrieving v%s metadata from %s using the v%s SOAP API", sourceVersion, org.Username, version)
			stages := []stage{
				{label: "Preparing retrieve request", state: "done", elapsed: time.Since(started)},
				{label: "Sending request to org"},
				{label: "Waiting for the org to respond"},
				{label: "Done"},
			}
			frame := func(status string) {
				if !jsonRequested(cmd) {
					writeStages(out, "Retrieving Metadata", subtitle, stages, [][2]string{{"Status", status}, {"Elapsed Time", formatElapsed(time.Since(started))}})
				}
			}
			requestStart := time.Now()
			var (
				files     lib.ForceMetadataFiles
				problems  []string
				responses []*orderedMap
			)
			org.Force.Metadata.RetrieveStatusHook = func(body []byte) {
				if results, err := soapResults(body); err == nil {
					responses = append(responses, results...)
				}
			}
			switch {
			case len(packageNames) == 1:
				files, problems, err = org.Force.Metadata.RetrievePackage(packageNames[0])
			case len(query) > 0:
				files, problems, err = org.Force.Metadata.Retrieve(query)
			default:
				// Only deletions in the org to apply locally.
				files = lib.ForceMetadataFiles{}
			}
			if err != nil {
				stages[1].state = "failed"
				stages[1].elapsed = time.Since(requestStart)
				frame("✘")
				converted := apiError(err)
				if e, ok := converted.(*Error); ok {
					e.Label = "1"
				}
				return converted
			}
			stages[1] = stage{label: "Sending request to org", state: "done", elapsed: time.Since(requestStart)}
			stages[2] = stage{label: "Waiting for the org to respond", state: "done"}
			stages[3] = stage{label: "Done", state: "done"}
			result, messages := retrieveJSON(responses, problems)
			failed := retrieveFailures(messages)
			if targetMetadataDir != "" {
				zipName, _ := cmd.Flags().GetString("zip-file-name")
				if zipName == "" {
					zipName = "unpackaged.zip"
				}
				if !strings.HasSuffix(zipName, ".zip") {
					zipName += ".zip"
				}
				archive, err := zipFiles(files, zipPrefix)
				if err != nil {
					return err
				}
				targetMetadataDir, err = filepath.Abs(targetMetadataDir)
				if err != nil {
					return err
				}
				if err := os.MkdirAll(targetMetadataDir, 0o755); err != nil {
					return err
				}
				zipPath := filepath.Join(targetMetadataDir, zipName)
				if err := os.WriteFile(zipPath, archive, 0o644); err != nil {
					return err
				}
				unzip, _ := cmd.Flags().GetBool("unzip")
				extracted := filepath.Join(targetMetadataDir, strings.TrimSuffix(zipName, ".zip"))
				if unzip {
					write := lib.EntryDiskWriter(filepath.Join(extracted, strings.TrimSuffix(zipPrefix, "/")))
					for name, data := range files {
						if err := write(name, strings.NewReader(string(data))); err != nil {
							return err
						}
					}
					// sf deletes the zip file after it extracted it.
					if err := os.Remove(zipPath); err != nil {
						return err
					}
				}
				result.Set("zipFilePath", zipPath)
				result.Set("files", []interface{}{})
				frame("Succeeded")
				return emit(cmd, result, func(w io.Writer) {
					fmt.Fprintf(w, "Wrote retrieve zip file to %s.\n", zipPath)
					if unzip {
						fmt.Fprintf(w, "Extracted %s to %s.\n", zipPath, extracted)
					}
				})
			}
			// Without --output-dir, a component that the project already
			// has is written where it is, and a new one to the default
			// package directory.
			var locate []func(string) string
			if outputDir == "" {
				outputDir = project.defaultPackageDir()
				locator, err := componentLocator(project)
				if err != nil {
					return err
				}
				locate = append(locate, locator)
			}
			target, err := filepath.Abs(filepath.Join(outputDir, "main", "default"))
			if err != nil {
				return err
			}
			if len(query) == 0 {
				query = wildcardQuery
			}
			written, err := writeSourceFormat(target, files, query, locate...)
			if err != nil {
				return err
			}
			// sf lists the directory of an object when it converts
			// metadata, and not when it retrieves.
			kept := written[:0]
			for _, file := range written {
				if info, err := os.Stat(file.FilePath); err == nil && info.IsDir() {
					continue
				}
				kept = append(kept, file)
			}
			written = kept
			// sf lists the files by type and path, and reports every file
			// of a component that existed as changed.
			sort.SliceStable(written, func(i, j int) bool {
				a, b := written[i], written[j]
				if a.Type != b.Type {
					return a.Type < b.Type
				}
				if a.FilePath != b.FilePath {
					return a.FilePath < b.FilePath
				}
				return a.FullName < b.FullName
			})
			existed := map[string]bool{}
			for _, file := range written {
				if file.State != "Add" {
					existed[file.Type+"###"+file.FullName] = true
				}
			}
			fileItems := append(make([]*orderedMap, 0, len(written)+len(failed)), failed...)
			rows := make([][]string, 0, len(written))
			for _, file := range written {
				state := "Created"
				if existed[file.Type+"###"+file.FullName] {
					state = "Changed"
				}
				absolute, _ := filepath.Abs(file.FilePath)
				item := newOrderedMap()
				item.Set("fullName", file.FullName)
				item.Set("type", file.Type)
				item.Set("state", state)
				item.Set("filePath", absolute)
				fileItems = append(fileItems, item)
				rows = append(rows, []string{state, file.FullName, file.Type, file.FilePath})
			}
			if tracking != nil {
				// Delete the files of the components deleted in the org,
				// and record what was retrieved in the tracking files.
				var updated, removed []string
				for _, file := range written {
					if absolute, err := filepath.Abs(file.FilePath); err == nil {
						if relative, err := filepath.Rel(project.Root, absolute); err == nil {
							if info, err := os.Stat(absolute); err == nil && info.Mode().IsRegular() {
								updated = append(updated, filepath.ToSlash(relative))
							}
						}
					}
				}
				for _, component := range remoteDeletes {
					for _, file := range component.files {
						if !trackingState.onDisk[file] {
							continue
						}
						absolute := filepath.Join(project.Root, filepath.FromSlash(file))
						if err := os.Remove(absolute); err != nil {
							return err
						}
						removed = append(removed, file)
						item := newOrderedMap()
						item.Set("fullName", component.FullName)
						item.Set("type", component.Type)
						item.Set("state", "Deleted")
						item.Set("filePath", absolute)
						fileItems = append(fileItems, item)
						rows = append(rows, []string{"Deleted", component.FullName, component.Type, relativePath(absolute)})
					}
				}
				repo, err := openShadowRepo(tracking.gitDir(), project.Root)
				if err != nil {
					return err
				}
				if err := tracking.commitLocal(repo, updated, removed, "sfdx source tracking"); err != nil {
					return err
				}
				// The members that were asked for, and the members of the
				// components that came back, are now in sync.
				keys := append([]string{}, retrieveKeys...)
				for _, file := range written {
					keys = append(keys, file.Type+"###"+file.FullName)
				}
				if _, _, err := tracking.syncRemote(keys, nil); err != nil {
					return err
				}
			}
			result.Set("files", fileItems)
			frame("Succeeded")
			return emit(cmd, result, func(w io.Writer) {
				if len(rows) == 0 {
					fmt.Fprintln(cmd.ErrOrStderr(), " ›   Warning: Nothing retrieved")
				} else {
					fmt.Fprintln(w)
					printTable(w, paint(w, style{"\x1b[1m\x1b[34m", "\x1b[39m\x1b[22m"}, "Retrieved Source"), []string{"State", "Name", "Type", "Path"}, rows)
				}
				if len(messages) > 0 {
					table := make([][]string, 0, len(messages))
					for _, message := range messages {
						table = append(table, []string{text(message.values["fileName"]), text(message.values["problem"])})
					}
					fmt.Fprintln(w)
					printTable(w, paint(w, style{"\x1b[1m\x1b[34m", "\x1b[39m\x1b[22m"}, "Warnings"), []string{"File", "Problem"}, table)
				}
			})
		},
	}
	addJSONFlag(cmd)
	cmd.Flags().StringP("api-version", "a", "", "Target API version for the retrieve.")
	cmd.Flags().BoolP("ignore-conflicts", "c", false, "Ignore conflicts and retrieve and save files to your local filesystem, even if they overwrite your local changes.")
	cmd.Flags().StringP("manifest", "x", "", "File path for the manifest (package.xml) that specifies the components to retrieve.")
	cmd.Flags().StringArrayP("metadata", "m", nil, "Metadata component names to retrieve.")
	cmd.Flags().StringArrayP("package-name", "n", nil, "Installed packages to retrieve.")
	cmd.Flags().StringP("output-dir", "r", "", "Directory root for the retrieved source files.")
	cmd.Flags().Bool("single-package", false, "Indicates that the zip file points to a directory structure for a single package.")
	cmd.Flags().StringArrayP("source-dir", "d", nil, "File paths for source to retrieve from the org.")
	cmd.Flags().StringP("target-metadata-dir", "t", "", "Directory that will contain the retrieved metadata format files or ZIP.")
	cmd.Flags().IntP("wait", "w", 33, "Number of minutes to wait for the command to complete and display results to the terminal window.")
	cmd.Flags().BoolP("unzip", "z", false, "Extract all files from the retrieved zip file.")
	cmd.Flags().String("zip-file-name", "", "File name to use for the retrieved zip file.")
	addTargetOrgFlag(cmd)
	return cmd
}

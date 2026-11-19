package sf

import (
	"encoding/json"
	"encoding/xml"
	"fmt"
	"io"
	"math"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"

	"github.com/spf13/cobra"
)

// apexTest is one test method result, with the fields sf reports.
type apexTest struct {
	Id              string
	QueueItemId     string
	StackTrace      interface{}
	Message         interface{}
	AsyncApexJobId  string
	MethodName      string
	Outcome         string
	ClassId         string
	ClassName       string
	NamespacePrefix interface{}
	RunTime         int
	FullName        string
	// Category is Apex or Flow.  The logic commands report it.
	Category string
}

func (t apexTest) json(withCategory bool) *orderedMap {
	class := newOrderedMap()
	class.Set("Id", t.ClassId)
	class.Set("Name", t.ClassName)
	class.Set("NamespacePrefix", t.NamespacePrefix)
	result := newOrderedMap()
	result.Set("Id", t.Id)
	result.Set("QueueItemId", t.QueueItemId)
	result.Set("StackTrace", t.StackTrace)
	result.Set("Message", t.Message)
	result.Set("AsyncApexJobId", t.AsyncApexJobId)
	result.Set("MethodName", t.MethodName)
	result.Set("Outcome", t.Outcome)
	result.Set("ApexClass", class)
	result.Set("RunTime", t.RunTime)
	result.Set("FullName", t.FullName)
	if withCategory {
		result.Set("Category", t.Category)
	}
	return result
}

func text(value interface{}) string {
	s, _ := value.(string)
	return s
}

// apexTestRun is the outcome of a test run.
type apexTestRun struct {
	TestRunId       string
	StartTime       string
	UserId          string
	ExecutionTimeMs int
	SetupTimeMs     int
	Tests           []apexTest
	Coverage        *apexCoverage
	// Logic is set for the logic commands, which run Apex and Flow tests
	// and report the category of each test.
	Logic bool
}

type apexClassCoverage struct {
	Id             string
	Name           string
	Covered        []int
	Uncovered      []int
	CoveredPercent int
}

// apexTestCoverage is the coverage of one class by one test method.
type apexTestCoverage struct {
	TestClassId string
	Method      string
	ClassId     string
	ClassName   string
	Covered     []int
	Uncovered   []int
}

func (c apexTestCoverage) percentage() string {
	return percent(len(c.Covered), len(c.Covered)+len(c.Uncovered))
}

type apexCoverage struct {
	PerTest         []apexTestCoverage
	Classes         []apexClassCoverage
	Records         []*orderedMap
	OrgWideCoverage string
	TestRunCoverage string
	TotalLines      int
	CoveredLines    int
}

func percent(part, total int) string {
	if total == 0 {
		return "0%"
	}
	return fmt.Sprintf("%d%%", int(math.Round(float64(part)*100/float64(total))))
}

func (r *apexTestRun) counts() (passing, failing, skipped int) {
	for _, test := range r.Tests {
		switch test.Outcome {
		case "Pass":
			passing++
		case "Skip":
			skipped++
		default:
			failing++
		}
	}
	return
}

func (r *apexTestRun) outcome() string {
	passing, failing, skipped := r.counts()
	switch {
	case failing > 0:
		return "Failed"
	case passing == 0 && skipped > 0:
		return "Skipped"
	}
	return "Passed"
}

// testKind tells the three families of test commands apart.
type testKind string

const (
	// apexTests is "apex run test" and "apex get test".
	apexTests testKind = "apex"
	// logicTests is the logic commands, which run Apex and Flow tests
	// and report the category of each test.
	logicTests testKind = "logic"
	// flowTests is the flow commands, which run Flow tests.
	flowTests testKind = "flow"
)

// flowTestPrefix starts the name of a Flow test: FlowTesting.<flow>.<test>.
const flowTestPrefix = "FlowTesting."

// flowTestItem adds a Flow test given as FlowTesting.<flow>.<test> or
// FlowTesting.<namespace>.<flow>.<test> to the items of a test request, as
// the Apex library of sf does.  It reports whether name is a Flow test.
// The library asks the org for its namespaces to tell a three-part name
// with a namespace from one with a test; this takes the third part as the
// test.
func flowTestItem(name string, items *[]interface{}, byClass map[string]*orderedMap) bool {
	if !strings.HasPrefix(name, flowTestPrefix) {
		return false
	}
	parts := strings.Split(name, ".")
	class, method, namespace := "", "", ""
	switch len(parts) {
	case 4:
		namespace = parts[0] + "." + parts[1]
		class, method = namespace+"."+parts[2], parts[3]
	case 3:
		class, method = parts[0]+"."+parts[1], parts[2]
	default:
		return false
	}
	item, known := byClass[class]
	if !known {
		item = newOrderedMap()
		if namespace != "" {
			item.Set("namespace", namespace)
		}
		item.Set("className", class)
		item.Set("testMethods", []string{})
		byClass[class] = item
		*items = append(*items, item)
	}
	methods, _ := item.values["testMethods"].([]string)
	item.Set("testMethods", append(methods, method))
	return true
}

// apexTestRequest builds the request body for runTestsAsynchronous and
// runTestsSynchronous from --class-names, --suite-names, --tests and
// --test-level.  logic is set for "logic run test", which also takes
// --test-category.
func apexTestRequest(cmd *cobra.Command, kind testKind) (*orderedMap, error) {
	logic := kind == logicTests
	classNames := splitList(cmd, "class-names")
	suiteNames := splitList(cmd, "suite-names")
	tests := splitList(cmd, "tests")
	if kind == flowTests {
		// The flow commands put this prefix before each name.
		for i := range classNames {
			classNames[i] = "flowtesting." + classNames[i]
		}
		for i := range tests {
			tests[i] = "flowtesting." + tests[i]
		}
	}
	testLevel, _ := cmd.Flags().GetString("test-level")
	codeCoverage, _ := cmd.Flags().GetBool("code-coverage")
	specified := 0
	for _, list := range [][]string{classNames, suiteNames, tests} {
		if len(list) > 0 {
			specified++
		}
	}
	if specified > 1 {
		return nil, &Error{Message: "--class-names=" + strings.Join(classNames, ",") + " cannot also be provided when using --suite-names or --tests\nSee more help with --help", ExitCode: 2}
	}
	// sf checks these combinations before it sends the request.
	if synchronous, _ := cmd.Flags().GetBool("synchronous"); synchronous && (len(suiteNames) > 0 || len(classNames) > 1) {
		switch kind {
		case logicTests:
			return nil, &Error{Message: "-- Synchronous test runs can include test methods from only one Apex class or Flow test. Omit the --synchronous flag or include tests from only one class", ExitCode: 1}
		case flowTests:
			return nil, &Error{Message: "Synchronous test runs can include tests from only one Flow. Omit the --synchronous flag or include tests from only one flow.", ExitCode: 1}
		}
		return nil, &Error{Message: "Synchronous test runs can include test methods from only one Apex class. Omit the --synchronous flag or include tests from only one class", ExitCode: 1}
	}
	var categories []string
	if logic {
		categories = splitList(cmd, "test-category")
		for _, category := range categories {
			if category != "Apex" && category != "Flow" {
				return nil, &Error{Message: fmt.Sprintf("Expected --test-category=%s to be one of: Apex, Flow\nSee more help with --help", category), ExitCode: 2}
			}
		}
		if len(categories) > 0 && testLevel == "" {
			return nil, &Error{Message: "When using --test-category, you must also specify --test-level.", ExitCode: 1}
		}
	}
	if specified > 0 && testLevel != "" && testLevel != "RunSpecifiedTests" {
		if kind == flowTests {
			return nil, &Error{Message: "When specifying flows or tests, indicate RunSpecifiedTests as the testlevel.", ExitCode: 1}
		}
		return nil, &Error{Message: "When specifying classnames, suitenames, or tests, indicate RunSpecifiedTests as the testlevel", ExitCode: 1}
	}
	if testLevel == "" {
		testLevel = "RunLocalTests"
		if specified > 0 {
			testLevel = "RunSpecifiedTests"
		}
	}
	switch testLevel {
	case "RunLocalTests", "RunAllTestsInOrg", "RunSpecifiedTests":
	default:
		return nil, &Error{Message: fmt.Sprintf("Expected --test-level=%s to be one of: RunLocalTests, RunAllTestsInOrg, RunSpecifiedTests\nSee more help with --help", testLevel), ExitCode: 1}
	}
	// "apex run test" runs the Apex tests of the org; a run of named
	// tests has no category.
	if kind == apexTests && testLevel != "RunSpecifiedTests" {
		categories = []string{"Apex"}
	}
	if kind == flowTests {
		categories = []string{"flow"}
	}
	request := newOrderedMap()
	classItem := func(name string) *orderedMap {
		item := newOrderedMap()
		if strings.HasPrefix(name, "01p") && (len(name) == 15 || len(name) == 18) {
			item.Set("classId", name)
			return item
		}
		if namespace, class, ok := strings.Cut(name, "."); ok {
			item.Set("namespace", namespace)
			name = class
		}
		item.Set("className", name)
		return item
	}
	switch {
	case len(classNames) > 0 && len(categories) > 0:
		// With a category the names are sent as they are: they can be
		// Flow tests.
		items := make([]interface{}, 0, len(classNames))
		for _, name := range classNames {
			items = append(items, orderedFields("className", name))
		}
		request.Set("tests", items)
		testLevel = "RunSpecifiedTests"
	case len(classNames) > 0:
		items := make([]interface{}, 0, len(classNames))
		for _, name := range classNames {
			items = append(items, classItem(name))
		}
		request.Set("tests", items)
	case len(suiteNames) > 0:
		request.Set("suiteNames", strings.Join(suiteNames, ","))
	case len(tests) > 0:
		var order []string
		methods := map[string][]string{}
		var flowItems []interface{}
		flowClasses := map[string]*orderedMap{}
		for _, test := range tests {
			if flowTestItem(test, &flowItems, flowClasses) {
				continue
			}
			class, method := test, ""
			if index := strings.LastIndex(test, "."); index >= 0 {
				class, method = test[:index], test[index+1:]
			}
			if _, ok := methods[class]; !ok {
				order = append(order, class)
			}
			if method != "" {
				methods[class] = append(methods[class], method)
			} else if methods[class] == nil {
				methods[class] = []string{}
			}
		}
		items := make([]interface{}, 0, len(order))
		for _, class := range order {
			item := classItem(class)
			if len(methods[class]) > 0 {
				item.Set("testMethods", methods[class])
			}
			items = append(items, item)
		}
		request.Set("tests", append(items, flowItems...))
	}
	request.Set("testLevel", testLevel)
	if _, named := request.Get("tests"); !named && len(categories) > 0 {
		request.Set("category", categories)
	}
	request.Set("skipCodeCoverage", !codeCoverage)
	return request, nil
}

// splitList returns the values of a repeatable flag, splitting each value
// on commas.
// commaListFlags are the list flags that sf splits at commas, by command.
// sf takes the values of every other list flag as they are given.
var commaListFlags = map[string]map[string]bool{
	"sf apex run test":                  {"class-names": true, "suite-names": true, "tests": true},
	"sf logic run test":                 {"class-names": true, "suite-names": true, "tests": true, "test-category": true},
	"sf cmdt generate field":            {"picklist-values": true},
	"sf commands":                       {"columns": true},
	"sf data import tree":               {"files": true},
	"sf org assign permsetlicense":      {"on-behalf-of": true},
	"sf org list sobject record-counts": {"sobject": true},
	"sf project convert mdapi":          {"metadata-dir": true, "metadata": true},
	"sf project convert source":         {"source-dir": true, "metadata": true},
	"sf project delete source":          {"metadata": true, "source-dir": true},
	"sf project generate manifest":      {"metadata": true, "source-dir": true, "include-packages": true, "excluded-metadata": true},
	"sf template generate apex trigger": {"event": true},
	"sf template generate flexipage":    {"secondary-fields": true, "detail-fields": true},
}

// commaWarningAfterBeta are the commands of commaListFlags that give the
// warning themselves, after their beta warning, as sf orders them.
var commaWarningAfterBeta = map[string]bool{"sf template generate flexipage": true, "sf logic run test": true}

// commaListWarning is the warning sf gives when a command with such a flag
// is given an argument that contains a comma.
const commaListWarning = "The input format for array arguments has changed. Use this format: --array-flag value1 --array-flag value2 --array-flag value3"

// hasUnescapedComma reports whether an argument has a comma that does not
// follow a backslash.
func hasUnescapedComma(args []string) bool {
	joined := strings.Join(args, " ")
	for i := 0; i < len(joined); i++ {
		if joined[i] == ',' && (i == 0 || joined[i-1] != '\\') {
			return true
		}
	}
	return false
}

// splitList returns the values of a list flag.  The values of the flags in
// commaListFlags are split at commas.
func splitList(cmd *cobra.Command, name string) []string {
	values, _ := cmd.Flags().GetStringArray(name)
	split := commaListFlags[cmd.CommandPath()][name]
	var result []string
	for _, value := range values {
		if !split {
			if value != "" {
				result = append(result, value)
			}
			continue
		}
		for _, part := range strings.Split(value, ",") {
			if part = strings.TrimSpace(part); part != "" {
				result = append(result, part)
			}
		}
	}
	return result
}

func quoteIds(ids []string) string {
	quoted := make([]string, len(ids))
	for i, id := range ids {
		quoted[i] = "'" + id + "'"
	}
	return strings.Join(quoted, ",")
}

func toolingRecords(org *targetOrg, soql string) ([]*orderedMap, error) {
	result, err := runQuery(org, "/tooling", "query", soql)
	if err != nil {
		return nil, apiError(err)
	}
	items, _ := result.values["records"].([]interface{})
	records := make([]*orderedMap, 0, len(items))
	for _, item := range items {
		if record, ok := item.(*orderedMap); ok {
			records = append(records, record)
		}
	}
	return records, nil
}

func intValue(value interface{}) int {
	switch v := value.(type) {
	case json.Number:
		if i, err := v.Int64(); err == nil {
			return int(i)
		}
		if f, err := v.Float64(); err == nil {
			return int(f)
		}
	case float64:
		return int(v)
	case int:
		return v
	}
	return 0
}

// isoTime converts a Salesforce datetime to the form JavaScript's
// toISOString produces.
func isoTime(value string) string {
	for _, layout := range []string{"2006-01-02T15:04:05.000-0700", time.RFC3339Nano, "2006-01-02T15:04:05.000Z0700"} {
		if parsed, err := time.Parse(layout, value); err == nil {
			return parsed.UTC().Format("2006-01-02T15:04:05.000Z")
		}
	}
	return value
}

var testRunDone = map[string]bool{"Completed": true, "Failed": true, "Aborted": true}

// waitForTestRun polls the queue items of a test run until each has
// finished or the timeout passes.  It returns the queue item ids.
// testQueueItems are the queue items of a test run.  An item without an
// Apex class is a Flow test.
type testQueueItems struct {
	apex []string
	flow []string
}

func waitForTestRun(org *targetOrg, testRunID string, timeout time.Duration, interval time.Duration) (testQueueItems, bool, error) {
	deadline := time.Now().Add(timeout)
	for {
		items, err := toolingRecords(org, fmt.Sprintf("SELECT Id, Status, ApexClassId, TestRunResultId, ParentJobId FROM ApexTestQueueItem WHERE ParentJobId = '%s'", testRunID))
		if err != nil {
			return testQueueItems{}, false, err
		}
		done := len(items) > 0
		var ids testQueueItems
		for _, item := range items {
			if item.values["ApexClassId"] == nil {
				ids.flow = append(ids.flow, text(item.values["Id"]))
			} else {
				ids.apex = append(ids.apex, text(item.values["Id"]))
			}
			if !testRunDone[text(item.values["Status"])] {
				done = false
			}
		}
		if done {
			return ids, true, nil
		}
		if time.Now().After(deadline) {
			return ids, false, nil
		}
		time.Sleep(interval)
	}
}

func loadTestRun(org *targetOrg, testRunID string, queueItems testQueueItems) (*apexTestRun, error) {
	queueItemIDs := queueItems.apex
	run := &apexTestRun{TestRunId: testRunID}
	summaries, err := toolingRecords(org, fmt.Sprintf("SELECT AsyncApexJobId, Status, ClassesCompleted, ClassesEnqueued, MethodsEnqueued, StartTime, EndTime, TestTime, TestSetupTime, UserId FROM ApexTestRunResult WHERE AsyncApexJobId = '%s'", testRunID))
	if err != nil {
		return nil, err
	}
	if len(summaries) > 0 {
		summary := summaries[0]
		run.StartTime = isoTime(text(summary.values["StartTime"]))
		run.UserId = text(summary.values["UserId"])
		run.ExecutionTimeMs = intValue(summary.values["TestTime"])
		run.SetupTimeMs = intValue(summary.values["TestSetupTime"])
	}
	for start := 0; start < len(queueItemIDs); start += 100 {
		end := start + 100
		if end > len(queueItemIDs) {
			end = len(queueItemIDs)
		}
		records, err := toolingRecords(org, fmt.Sprintf("SELECT Id, QueueItemId, StackTrace, Message, RunTime, TestTimestamp, AsyncApexJobId, MethodName, Outcome, ApexLogId, IsTestSetup, ApexClass.Id, ApexClass.Name, ApexClass.NamespacePrefix FROM ApexTestResult WHERE QueueItemId IN (%s)", quoteIds(queueItemIDs[start:end])))
		if err != nil {
			return nil, err
		}
		for _, record := range records {
			if record.values["IsTestSetup"] == true {
				continue
			}
			test := apexTest{
				Id:             text(record.values["Id"]),
				QueueItemId:    text(record.values["QueueItemId"]),
				StackTrace:     record.values["StackTrace"],
				Message:        record.values["Message"],
				AsyncApexJobId: text(record.values["AsyncApexJobId"]),
				MethodName:     text(record.values["MethodName"]),
				Outcome:        text(record.values["Outcome"]),
				RunTime:        intValue(record.values["RunTime"]),
				Category:       "Apex",
			}
			if class, ok := record.values["ApexClass"].(*orderedMap); ok {
				test.ClassId = text(class.values["Id"])
				test.ClassName = text(class.values["Name"])
				test.NamespacePrefix = class.values["NamespacePrefix"]
			}
			test.FullName = test.ClassName + "." + test.MethodName
			if namespace := text(test.NamespacePrefix); namespace != "" {
				test.FullName = namespace + "." + test.FullName
			}
			run.Tests = append(run.Tests, test)
		}
	}
	// The results of Flow tests are in another object.  sf reports them
	// in the form of Apex test results: the flow as the class and the
	// Flow test as the method.
	for start := 0; start < len(queueItems.flow); start += 100 {
		end := start + 100
		if end > len(queueItems.flow) {
			end = len(queueItems.flow)
		}
		records, err := toolingRecords(org, fmt.Sprintf("SELECT Id, ApexTestQueueItemId, Result, TestStartDateTime,TestEndDateTime, FlowTest.DeveloperName, FlowDefinition.DeveloperName, FlowDefinition.NamespacePrefix FROM FlowTestResult WHERE ApexTestQueueItemId IN (%s)", quoteIds(queueItems.flow[start:end])))
		if err != nil {
			return nil, err
		}
		for _, record := range records {
			queueItem := text(record.values["ApexTestQueueItemId"])
			test := apexTest{
				Id: text(record.values["Id"]), QueueItemId: queueItem, StackTrace: "", Message: "", AsyncApexJobId: queueItem,
				Outcome: text(record.values["Result"]), ClassId: queueItem, Category: "Flow",
			}
			if flowTest, ok := record.values["FlowTest"].(*orderedMap); ok {
				test.MethodName = text(flowTest.values["DeveloperName"])
			}
			if flow, ok := record.values["FlowDefinition"].(*orderedMap); ok {
				test.ClassName = text(flow.values["DeveloperName"])
				test.NamespacePrefix = flow.values["NamespacePrefix"]
			}
			test.FullName = test.ClassName + "." + test.MethodName
			if namespace := text(test.NamespacePrefix); namespace != "" {
				test.FullName = namespace + "." + test.FullName
			}
			run.Tests = append(run.Tests, test)
		}
	}
	sort.SliceStable(run.Tests, func(i, j int) bool { return run.Tests[i].FullName < run.Tests[j].FullName })
	return run, nil
}

func intList(value interface{}) []int {
	items, _ := value.([]interface{})
	result := make([]int, 0, len(items))
	for _, item := range items {
		result = append(result, intValue(item))
	}
	return result
}

// loadCoverage reads the code coverage of the classes the tests in run
// exercised, and the org-wide coverage.
func loadCoverage(org *targetOrg, run *apexTestRun) error {
	coverage := &apexCoverage{Records: []*orderedMap{}}
	run.Coverage = coverage
	var testClassIDs []string
	seen := map[string]bool{}
	testIDs := map[string]string{}
	for _, test := range run.Tests {
		if test.ClassId != "" && !seen[test.ClassId] {
			seen[test.ClassId] = true
			testClassIDs = append(testClassIDs, test.ClassId)
		}
		testIDs[test.ClassId+"."+test.MethodName] = test.Id
	}
	var coveredClassIDs []string
	if len(testClassIDs) > 0 {
		records, err := toolingRecords(org, fmt.Sprintf("SELECT ApexTestClassId, ApexClassOrTrigger.Id, ApexClassOrTrigger.Name, TestMethodName, NumLinesCovered, NumLinesUncovered, Coverage FROM ApexCodeCoverage WHERE ApexTestClassId IN (%s)", quoteIds(testClassIDs)))
		if err != nil {
			return err
		}
		classNames := map[string]string{}
		for _, test := range run.Tests {
			classNames[test.ClassId] = test.ClassName
		}
		seenCovered := map[string]bool{}
		for _, record := range records {
			testClassID := text(record.values["ApexTestClassId"])
			method := text(record.values["TestMethodName"])
			testClass := newOrderedMap()
			testClass.Set("Id", testIDs[testClassID+"."+method])
			testClass.Set("Name", classNames[testClassID])
			lines := newOrderedMap()
			if covered, ok := record.values["Coverage"].(*orderedMap); ok {
				lines.Set("coveredLines", covered.values["coveredLines"])
				lines.Set("uncoveredLines", covered.values["uncoveredLines"])
			}
			perTest := apexTestCoverage{TestClassId: testClassID, Method: method}
			if covered, ok := record.values["Coverage"].(*orderedMap); ok {
				perTest.Covered = intList(covered.values["coveredLines"])
				perTest.Uncovered = intList(covered.values["uncoveredLines"])
			}
			target := newOrderedMap()
			if class, ok := record.values["ApexClassOrTrigger"].(*orderedMap); ok {
				perTest.ClassId = text(class.values["Id"])
				perTest.ClassName = text(class.values["Name"])
				target.Set("Id", class.values["Id"])
				target.Set("Name", class.values["Name"])
				if id := text(class.values["Id"]); id != "" && !seenCovered[id] {
					seenCovered[id] = true
					coveredClassIDs = append(coveredClassIDs, id)
				}
			}
			item := newOrderedMap()
			item.Set("ApexTestClass", testClass)
			item.Set("Coverage", lines)
			item.Set("TestMethodName", method)
			item.Set("NumLinesCovered", record.values["NumLinesCovered"])
			item.Set("ApexClassOrTrigger", target)
			item.Set("NumLinesUncovered", record.values["NumLinesUncovered"])
			coverage.Records = append(coverage.Records, item)
			coverage.PerTest = append(coverage.PerTest, perTest)
		}
	}
	{
		// Without coverage records for the tests of the run, sf reads the
		// aggregate coverage of every class and trigger: an org can be set
		// to store only the aggregates.
		soql := "SELECT ApexClassOrTrigger.Id, ApexClassOrTrigger.Name, NumLinesCovered, NumLinesUncovered, Coverage FROM ApexCodeCoverageAggregate"
		if len(coveredClassIDs) > 0 {
			soql += fmt.Sprintf(" WHERE ApexClassorTriggerId IN (%s)", quoteIds(coveredClassIDs))
		}
		records, err := toolingRecords(org, soql)
		if err != nil {
			return err
		}
		for _, record := range records {
			class := apexClassCoverage{}
			if target, ok := record.values["ApexClassOrTrigger"].(*orderedMap); ok {
				class.Id = text(target.values["Id"])
				class.Name = text(target.values["Name"])
			}
			if lines, ok := record.values["Coverage"].(*orderedMap); ok {
				class.Covered = intList(lines.values["coveredLines"])
				class.Uncovered = intList(lines.values["uncoveredLines"])
			}
			total := len(class.Covered) + len(class.Uncovered)
			if total > 0 {
				class.CoveredPercent = int(math.Round(float64(len(class.Covered)) * 100 / float64(total)))
			}
			coverage.TotalLines += total
			coverage.CoveredLines += len(class.Covered)
			coverage.Classes = append(coverage.Classes, class)
		}
	}
	coverage.TestRunCoverage = percent(coverage.CoveredLines, coverage.TotalLines)
	orgWide, err := toolingRecords(org, "SELECT PercentCovered FROM ApexOrgWideCoverage")
	if err != nil {
		return err
	}
	coverage.OrgWideCoverage = "0%"
	if len(orgWide) > 0 {
		coverage.OrgWideCoverage = fmt.Sprintf("%d%%", intValue(orgWide[0].values["PercentCovered"]))
	}
	return nil
}

func (c *apexCoverage) json() *orderedMap {
	classes := make([]interface{}, 0, len(c.Classes))
	for _, class := range c.Classes {
		lines := newOrderedMap()
		numbers := map[int]int{}
		var all []int
		for _, line := range class.Covered {
			numbers[line] = 1
			all = append(all, line)
		}
		for _, line := range class.Uncovered {
			numbers[line] = 0
			all = append(all, line)
		}
		sort.Ints(all)
		for _, line := range all {
			lines.Set(fmt.Sprintf("%d", line), numbers[line])
		}
		item := newOrderedMap()
		item.Set("id", class.Id)
		item.Set("name", class.Name)
		item.Set("totalLines", len(all))
		item.Set("lines", lines)
		item.Set("totalCovered", len(class.Covered))
		item.Set("coveredPercent", class.CoveredPercent)
		classes = append(classes, item)
	}
	summary := newOrderedMap()
	summary.Set("totalLines", c.TotalLines)
	summary.Set("coveredLines", c.CoveredLines)
	summary.Set("orgWideCoverage", c.OrgWideCoverage)
	summary.Set("testRunCoverage", c.TestRunCoverage)
	result := newOrderedMap()
	result.Set("coverage", classes)
	result.Set("records", c.Records)
	result.Set("summary", summary)
	return result
}

// json returns the result object sf reports for a test run.
func (r *apexTestRun) json(org *targetOrg, commandTime time.Duration) *orderedMap {
	passing, failing, skipped := r.counts()
	total := len(r.Tests)
	summary := newOrderedMap()
	summary.Set("failRate", percent(failing, total))
	summary.Set("failing", failing)
	summary.Set("hostname", org.Fields.String("instanceUrl"))
	summary.Set("orgId", org.Fields.String("orgId"))
	summary.Set("outcome", r.outcome())
	summary.Set("passRate", percent(passing, total))
	summary.Set("passing", passing)
	summary.Set("skipped", skipped)
	summary.Set("testRunId", r.TestRunId)
	summary.Set("testStartTime", r.StartTime)
	summary.Set("testsRan", total)
	if r.UserId != "" {
		summary.Set("userId", r.UserId)
	}
	summary.Set("username", org.Username)
	summary.Set("commandTime", fmt.Sprintf("%d ms", commandTime.Milliseconds()))
	summary.Set("testExecutionTime", fmt.Sprintf("%d ms", r.ExecutionTimeMs))
	summary.Set("testTotalTime", fmt.Sprintf("%d ms", r.ExecutionTimeMs+r.SetupTimeMs))
	if r.Coverage != nil {
		summary.Set("orgWideCoverage", r.Coverage.OrgWideCoverage)
		summary.Set("testRunCoverage", r.Coverage.TestRunCoverage)
	}
	tests := make([]interface{}, 0, total)
	for _, test := range r.Tests {
		tests = append(tests, test.json(r.Logic))
	}
	result := newOrderedMap()
	result.Set("summary", summary)
	result.Set("tests", tests)
	if r.Coverage != nil {
		result.Set("coverage", r.Coverage.json())
	}
	return result
}

// textTable formats a table the way the sf Apex test reporter does: a
// "=== title" line, a header, a line of box-drawing dashes, and rows whose
// columns are padded to the widest cell and separated by two spaces.  A cell
// with several lines continues on the following output lines.
func textTable(title string, headers []string, rows [][]string) string {
	widths := make([]int, len(headers))
	for i, header := range headers {
		widths[i] = len([]rune(header))
	}
	for _, row := range rows {
		for i, cell := range row {
			for _, line := range strings.Split(cell, "\n") {
				if length := len([]rune(line)); length > widths[i] {
					widths[i] = length
				}
			}
		}
	}
	fill := func(label string, width int, filler string, last bool) string {
		filled := label + strings.Repeat(filler, max(0, width-len([]rune(label))))
		if !last {
			filled += strings.Repeat(filler, 2)
		}
		return filled
	}
	var b strings.Builder
	b.WriteString("=== " + title + "\n")
	var header, separator string
	for i, label := range headers {
		last := i == len(headers)-1
		header += fill(label, widths[i], " ", last)
		// The columns of the separator line are joined by two spaces.
		separator += strings.Repeat("─", widths[i])
		if !last {
			separator += "  "
		}
	}
	b.WriteString(header + "\n" + separator + "\n")
	for _, row := range rows {
		outputRow := ""
		for i, cell := range row {
			last := i == len(row)-1
			rowWidth := len([]rune(outputRow))
			for lineIndex, line := range strings.Split(cell, "\n") {
				if lineIndex == 0 {
					outputRow += fill(line, widths[i], " ", last)
				} else {
					outputRow += "\n" + strings.Repeat(" ", rowWidth) + fill(line, widths[i], " ", last)
				}
			}
		}
		b.WriteString(outputRow + "\n")
	}
	return b.String()
}

func testMessage(test apexTest) string {
	if stack := text(test.StackTrace); stack != "" {
		return text(test.Message) + "\n" + stack
	}
	return text(test.Message)
}

// coverageOf returns the per-class coverage records of one test.
func (c *apexCoverage) coverageOf(test apexTest) []apexTestCoverage {
	var entries []apexTestCoverage
	for _, entry := range c.PerTest {
		if entry.TestClassId == test.ClassId && entry.Method == test.MethodName {
			entries = append(entries, entry)
		}
	}
	return entries
}

// withCategory inserts the category of a test into a table row at the given
// position, for the logic commands.
func withCategory(logic bool, at int, category string, row []string) []string {
	if !logic {
		return row
	}
	result := append([]string{}, row[:at]...)
	result = append(result, category)
	return append(result, row[at:]...)
}

func (r *apexTestRun) human(org *targetOrg, concise bool, detailed bool) string {
	var sections []string
	var rows [][]string
	detailed = detailed && r.Coverage != nil
	for _, test := range r.Tests {
		if concise && test.Outcome != "Fail" && test.Outcome != "CompileFail" {
			continue
		}
		if detailed {
			// One row for each class the test covered.
			runtime := fmt.Sprintf("%d", test.RunTime)
			entries := r.Coverage.coverageOf(test)
			if len(entries) == 0 {
				rows = append(rows, withCategory(r.Logic, 2, test.Category, []string{test.FullName, "", test.Outcome, "", testMessage(test), runtime}))
			}
			for _, entry := range entries {
				rows = append(rows, withCategory(r.Logic, 2, test.Category, []string{test.FullName, entry.ClassName, test.Outcome, entry.percentage(), testMessage(test), runtime}))
			}
			continue
		}
		runtime := ""
		if test.Outcome != "Fail" {
			runtime = fmt.Sprintf("%d", test.RunTime)
		}
		rows = append(rows, withCategory(r.Logic, 1, test.Category, []string{test.FullName, test.Outcome, testMessage(test), runtime}))
	}
	switch {
	case len(rows) == 0:
		sections = append(sections, "")
	case detailed:
		sections = append(sections, textTable("Apex Code Coverage for Test Run "+r.TestRunId, withCategory(r.Logic, 2, "CATEGORY", []string{"TEST NAME", "CLASS BEING TESTED", "OUTCOME", "PERCENT", "MESSAGE", "RUNTIME (MS)"}), rows))
	default:
		sections = append(sections, textTable("Test Results", withCategory(r.Logic, 1, "CATEGORY", []string{"TEST NAME", "OUTCOME", "MESSAGE", "RUNTIME (MS)"}), rows))
	}
	if r.Coverage != nil && !concise {
		var coverageRows [][]string
		for _, class := range r.Coverage.Classes {
			uncovered := make([]string, 0, len(class.Uncovered))
			for i, line := range class.Uncovered {
				if i == 5 {
					uncovered = append(uncovered, "...")
					break
				}
				uncovered = append(uncovered, fmt.Sprintf("%d", line))
			}
			coverageRows = append(coverageRows, []string{class.Name, fmt.Sprintf("%d%%", class.CoveredPercent), strings.Join(uncovered, ",")})
		}
		sections = append(sections, textTable("Apex Code Coverage by Class", []string{"CLASSES", "PERCENT", "UNCOVERED LINES"}, coverageRows))
	}
	if !concise {
		// sf adds an empty section here for test setup methods.
		sections = append(sections, "")
	}
	passing, failing, skipped := r.counts()
	total := len(r.Tests)
	summary := [][]string{
		{"Outcome", r.outcome()},
		{"Tests Ran", fmt.Sprintf("%d", total)},
		{"Pass Rate", percent(passing, total)},
		{"Fail Rate", percent(failing, total)},
		{"Skip Rate", percent(skipped, total)},
		{"Test Run Id", r.TestRunId},
		{"Test Setup Time", fmt.Sprintf("%d ms", r.SetupTimeMs)},
		{"Test Execution Time", fmt.Sprintf("%d ms", r.ExecutionTimeMs)},
		{"Test Total Time", fmt.Sprintf("%d ms", r.ExecutionTimeMs+r.SetupTimeMs)},
		{"Org Id", org.Fields.String("orgId")},
		{"Username", org.Username},
	}
	if r.Coverage != nil {
		summary = append(summary, []string{"Org Wide Coverage", r.Coverage.OrgWideCoverage})
	}
	sections = append(sections, textTable("Test Summary", []string{"NAME", "VALUE"}, summary))
	return strings.Join(sections, "\n\n")
}

func (r *apexTestRun) tap(org *targetOrg) string {
	lines := []string{fmt.Sprintf("1..%d", len(r.Tests))}
	for i, test := range r.Tests {
		outcome := "not ok"
		if test.Outcome == "Pass" {
			outcome = "ok"
		}
		lines = append(lines, fmt.Sprintf("%s %d %s", outcome, i+1, test.FullName))
		if test.Outcome != "Pass" {
			for _, value := range []string{text(test.Message), text(test.StackTrace)} {
				if value == "" {
					continue
				}
				for _, line := range strings.Split(value, "\n") {
					lines = append(lines, "# "+line)
				}
			}
		}
	}
	if r.TestRunId != "" {
		lines = append(lines, fmt.Sprintf(`# Run "`+binName+` apex get test -i %s -o %s --result-format <format>" to retrieve test results in a different format.`, r.TestRunId, org.Username))
	}
	return strings.Join(lines, "\n") + "\n"
}

func xmlEscape(value string) string {
	var b strings.Builder
	xml.EscapeText(&b, []byte(value))
	return b.String()
}

func seconds(ms int) string {
	return fmt.Sprintf("%.2f", float64(ms)/1000)
}

func (r *apexTestRun) junit(org *targetOrg, commandTime time.Duration) string {
	passing, failing, skipped := r.counts()
	total := len(r.Tests)
	start := r.StartTime
	startText := start
	if parsed, err := time.Parse("2006-01-02T15:04:05.000Z", start); err == nil {
		startText = parsed.Local().Format("Mon Jan 02 2006 3:04:05 PM")
	}
	var b strings.Builder
	b.WriteString("<?xml version=\"1.0\" encoding=\"UTF-8\"?>\n<testsuites>\n")
	fmt.Fprintf(&b, "    <testsuite name=\"force.apex\" timestamp=\"%s\" hostname=\"%s\" tests=\"%d\" failures=\"%d\"  errors=\"0\"  time=\"%s\">\n",
		start, xmlEscape(org.Fields.String("instanceUrl")), total, failing, seconds(r.ExecutionTimeMs))
	b.WriteString("        <properties>\n")
	properties := [][2]string{
		{"outcome", r.outcome()},
		{"testsRan", fmt.Sprintf("%d", total)},
		{"passing", fmt.Sprintf("%d", passing)},
		{"failing", fmt.Sprintf("%d", failing)},
		{"skipped", fmt.Sprintf("%d", skipped)},
		{"passRate", percent(passing, total)},
		{"failRate", percent(failing, total)},
		{"testStartTime", startText},
		{"testExecutionTime", seconds(r.ExecutionTimeMs) + " s"},
		{"testTotalTime", seconds(r.ExecutionTimeMs+r.SetupTimeMs) + " s"},
		{"commandTime", seconds(int(commandTime.Milliseconds())) + " s"},
		{"hostname", org.Fields.String("instanceUrl")},
		{"orgId", org.Fields.String("orgId")},
		{"username", org.Username},
		{"testRunId", r.TestRunId},
	}
	if r.UserId != "" {
		properties = append(properties, [2]string{"userId", r.UserId})
	}
	if r.Coverage != nil {
		properties = append(properties, [2]string{"orgWideCoverage", r.Coverage.OrgWideCoverage}, [2]string{"testRunCoverage", r.Coverage.TestRunCoverage})
	}
	for _, property := range properties {
		fmt.Fprintf(&b, "            <property name=\"%s\" value=\"%s\"/>\n", property[0], xmlEscape(property[1]))
	}
	b.WriteString("        </properties>\n")
	for _, test := range r.Tests {
		fmt.Fprintf(&b, "        <testcase name=\"%s\" classname=\"%s\" time=\"%s\">\n", xmlEscape(test.MethodName), xmlEscape(test.ClassName), seconds(test.RunTime))
		if test.Outcome == "Fail" || test.Outcome == "CompileFail" {
			fmt.Fprintf(&b, "            <failure message=\"%s\">", xmlEscape(text(test.Message)))
			if stack := text(test.StackTrace); stack != "" {
				fmt.Fprintf(&b, "<![CDATA[%s]]>", stack)
			}
			b.WriteString("</failure>\n")
		}
		b.WriteString("        </testcase>\n")
	}
	b.WriteString("    </testsuite>\n</testsuites>\n")
	return b.String()
}

// runTestsSynchronously runs tests through runTestsSynchronous, which
// answers with the results.
func runTestsSynchronously(org *targetOrg, request *orderedMap) (*apexTestRun, error) {
	request.Delete("testLevel")
	request.Delete("category")
	content, err := encodeJSON(request)
	if err != nil {
		return nil, err
	}
	started := time.Now().UTC()
	body, err := org.Force.PostREST("/tooling/runTestsSynchronous", content)
	if err != nil {
		return nil, apiError(err)
	}
	var response struct {
		TotalTime float64 `json:"totalTime"`
		Successes []struct {
			Id         string      `json:"id"`
			MethodName string      `json:"methodName"`
			Name       string      `json:"name"`
			Namespace  interface{} `json:"namespace"`
			Time       float64     `json:"time"`
		} `json:"successes"`
		Failures []struct {
			Id         string      `json:"id"`
			MethodName string      `json:"methodName"`
			Name       string      `json:"name"`
			Namespace  interface{} `json:"namespace"`
			Time       float64     `json:"time"`
			Message    string      `json:"message"`
			StackTrace string      `json:"stackTrace"`
		} `json:"failures"`
	}
	if err := json.Unmarshal([]byte(body), &response); err != nil {
		return nil, err
	}
	run := &apexTestRun{
		StartTime:       started.Format("2006-01-02T15:04:05.000Z"),
		ExecutionTimeMs: int(response.TotalTime),
	}
	fullName := func(namespace interface{}, class, method string) string {
		name := class + "." + method
		if prefix := text(namespace); prefix != "" {
			name = prefix + "." + name
		}
		return name
	}
	category := func(namespace interface{}) string {
		if strings.HasPrefix(text(namespace), flowTestPrefix) {
			return "Flow"
		}
		return "Apex"
	}
	for _, success := range response.Successes {
		run.Tests = append(run.Tests, apexTest{
			Category:   category(success.Namespace),
			StackTrace: "", Message: "", MethodName: success.MethodName, Outcome: "Pass",
			ClassId: success.Id, ClassName: success.Name, NamespacePrefix: success.Namespace,
			RunTime: int(success.Time), FullName: fullName(success.Namespace, success.Name, success.MethodName),
		})
	}
	for _, failure := range response.Failures {
		run.Tests = append(run.Tests, apexTest{
			Category:   category(failure.Namespace),
			StackTrace: failure.StackTrace, Message: failure.Message, MethodName: failure.MethodName, Outcome: "Fail",
			ClassId: failure.Id, ClassName: failure.Name, NamespacePrefix: failure.Namespace,
			RunTime: int(failure.Time), FullName: fullName(failure.Namespace, failure.Name, failure.MethodName),
		})
	}
	sort.SliceStable(run.Tests, func(i, j int) bool { return run.Tests[i].FullName < run.Tests[j].FullName })
	return run, nil
}

func testResultFormat(cmd *cobra.Command) (string, error) {
	format, _ := cmd.Flags().GetString("result-format")
	switch format {
	case "human", "tap", "junit", "json":
		return format, nil
	}
	return "", &Error{Message: fmt.Sprintf("Expected --result-format=%s to be one of: human, tap, junit, json\nSee more help with --help", format), ExitCode: 1}
}

// perTestCoverageJSON is the content of the per-run code coverage file: one
// array for each test, with the coverage of each class the test covered.
func (r *apexTestRun) perTestCoverageJSON() []interface{} {
	result := []interface{}{}
	for _, test := range r.Tests {
		entries := r.Coverage.coverageOf(test)
		if len(entries) == 0 {
			continue
		}
		items := make([]interface{}, 0, len(entries))
		for _, entry := range entries {
			lines := newOrderedMap()
			lines.Set("coveredLines", entry.Covered)
			lines.Set("uncoveredLines", entry.Uncovered)
			if entry.Covered == nil {
				lines.Set("coveredLines", []int{})
			}
			if entry.Uncovered == nil {
				lines.Set("uncoveredLines", []int{})
			}
			item := newOrderedMap()
			item.Set("apexClassOrTriggerName", entry.ClassName)
			item.Set("apexClassOrTriggerId", entry.ClassId)
			item.Set("apexTestClassId", entry.TestClassId)
			item.Set("apexTestMethodName", entry.Method)
			item.Set("numLinesCovered", len(entry.Covered))
			item.Set("numLinesUncovered", len(entry.Uncovered))
			item.Set("percentage", entry.percentage())
			item.Set("coverage", lines)
			items = append(items, item)
		}
		result = append(result, items)
	}
	return result
}

// writeTestResultFiles writes the files sf writes to --output-dir.
func writeTestResultFiles(dir string, org *targetOrg, run *apexTestRun, result *orderedMap, human string, format string, elapsed time.Duration) error {
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return err
	}
	compact := func(value interface{}) (string, error) { return encodeJSON(value) }
	files := map[string]string{"test-run-id.txt": run.TestRunId, "test-result.txt": human}
	prefix := "test-result"
	if run.TestRunId != "" {
		prefix += "-" + run.TestRunId
	}
	encoded, err := compact(result)
	if err != nil {
		return err
	}
	files[prefix+".json"] = encoded
	files[prefix+"-junit.xml"] = run.junit(org, elapsed)
	if format == "tap" {
		files[prefix+"-tap.txt"] = run.tap(org)
	}
	if run.Coverage != nil {
		perTest, err := compact(run.perTestCoverageJSON())
		if err != nil {
			return err
		}
		files[prefix+"-codecoverage.json"] = perTest
		classes, _ := run.Coverage.json().Get("coverage")
		byClass, err := compact(classes)
		if err != nil {
			return err
		}
		files["test-result-codecoverage.json"] = byClass
	}
	for name, content := range files {
		if err := os.WriteFile(filepath.Join(dir, name), []byte(content), 0o644); err != nil {
			return err
		}
	}
	return nil
}

// reportTestRun loads coverage when it was requested, prints a test run in
// the requested format, writes the --output-dir files, and returns exit
// status 100 as an error when a test failed.
func reportTestRun(cmd *cobra.Command, org *targetOrg, run *apexTestRun, format string, elapsed time.Duration) error {
	codeCoverage, _ := cmd.Flags().GetBool("code-coverage")
	detailed, _ := cmd.Flags().GetBool("detailed-coverage")
	concise, _ := cmd.Flags().GetBool("concise")
	if codeCoverage || detailed {
		if err := loadCoverage(org, run); err != nil {
			return err
		}
	}
	status := 0
	if run.outcome() == "Failed" {
		status = 100
	}
	result := run.json(org, elapsed)
	human := run.human(org, concise, detailed)
	if outputDir, _ := cmd.Flags().GetString("output-dir"); outputDir != "" {
		if err := writeTestResultFiles(outputDir, org, run, result, human, format, elapsed); err != nil {
			return err
		}
		if !jsonRequested(cmd) {
			fmt.Fprintf(cmd.OutOrStdout(), "Test result files written to %s\n", outputDir)
		}
	}
	out := cmd.OutOrStdout()
	if jsonRequested(cmd) {
		if err := emitStatus(cmd, status, result, nil); err != nil {
			return err
		}
	} else {
		switch format {
		case "json":
			envelope := newOrderedMap()
			envelope.Set("status", status)
			envelope.Set("result", result)
			if err := writeJSON(out, envelope); err != nil {
				return err
			}
		case "tap":
			fmt.Fprintln(out, run.tap(org))
		case "junit":
			fmt.Fprintln(out, run.junit(org, elapsed))
		default:
			fmt.Fprintln(out, human)
		}
	}
	if status != 0 {
		return exitStatus(status)
	}
	return nil
}

func newApexGetTestCmd() *cobra.Command { return newGetTestCmd(apexTests) }

// newFlowGetTestCmd is "flow get test": the results of a run of Flow tests.
func newFlowGetTestCmd() *cobra.Command { return newGetTestCmd(flowTests) }

// newLogicGetTestCmd is "logic get test": "apex get test" for a run of
// Apex and Flow tests, with the category of each test in the results.
func newLogicGetTestCmd() *cobra.Command { return newGetTestCmd(logicTests) }

func newGetTestCmd(kind testKind) *cobra.Command {
	logic := kind == logicTests
	short, class := "Display test results for a specific asynchronous test run.", "Test"
	if logic {
		short, class = "Get the results of a test run.", "TestLogic"
	}
	cmd := &cobra.Command{
		Use:         "test",
		Short:       short,
		Args:        cobra.NoArgs,
		Annotations: map[string]string{commandNameAnnotation: class},
		RunE: func(cmd *cobra.Command, args []string) error {
			if logic {
				warn(cmd, betaWarning)
			}
			format, err := testResultFormat(cmd)
			if err != nil {
				return err
			}
			testRunID, _ := cmd.Flags().GetString("test-run-id")
			if testRunID == "" {
				return missingFlagError("test-run-id")
			}
			org, err := resolveTargetOrg(cmd)
			if err != nil {
				return err
			}
			started := time.Now()
			queueItemIDs, _, err := waitForTestRun(org, testRunID, 0, time.Millisecond)
			if err != nil {
				return err
			}
			run, err := loadTestRun(org, testRunID, queueItemIDs)
			if err != nil {
				return err
			}
			run.Logic = logic
			if logic {
				// sf does not pass the flag on for this command.
				cmd.Flags().Set("detailed-coverage", "false")
			}
			return reportTestRun(cmd, org, run, format, time.Since(started))
		},
	}
	addJSONFlag(cmd)
	cmd.Flags().StringP("test-run-id", "i", "", "ID of the test run.")
	cmd.Flags().BoolP("code-coverage", "c", false, "Retrieve code coverage results.")
	cmd.Flags().Bool("detailed-coverage", false, "Display detailed code coverage per test.")
	cmd.Flags().StringP("output-dir", "d", "", "Directory in which to store test result files.")
	cmd.Flags().StringP("result-format", "r", "human", "Format of the test results.")
	cmd.Flags().Bool("concise", false, "Display only failed test results; works with human-readable output only.")
	addAPIVersionFlag(cmd)
	addTargetOrgFlag(cmd)
	return cmd
}

// testPollInterval is the time between checks of a test run.  Tests
// shorten it.
var testPollInterval = time.Second

func newApexRunTestCmd() *cobra.Command { return newRunTestCmd(apexTests) }

// newFlowRunTestCmd is "flow run test".  It runs Flow tests, and it has no
// --wait flag: without --synchronous it prints the Id of the test run.
func newFlowRunTestCmd() *cobra.Command { return newRunTestCmd(flowTests) }

// newLogicRunTestCmd is "logic run test": "apex run test" for Apex and Flow
// tests, with --test-category to choose between them.
func newLogicRunTestCmd() *cobra.Command { return newRunTestCmd(logicTests) }

func newRunTestCmd(kind testKind) *cobra.Command {
	logic := kind == logicTests
	short, class, topic := "Invoke Apex tests in an org.", "Test", "apex"
	switch kind {
	case logicTests:
		short, class, topic = "Invoke tests for Apex and Flows in an org.", "RunTestLogic", "logic"
	case flowTests:
		short, class, topic = "Invoke flow tests in an org.", "FlowRunTest", "flow"
	}
	cmd := &cobra.Command{
		Use:         "test",
		Short:       short,
		Args:        cobra.NoArgs,
		Annotations: map[string]string{commandNameAnnotation: class},
		RunE: func(cmd *cobra.Command, args []string) error {
			if logic {
				warn(cmd, betaWarning)
				if hasUnescapedComma(currentArgs) {
					warn(cmd, commaListWarning)
				}
			}
			format, err := testResultFormat(cmd)
			if err != nil {
				return err
			}
			request, err := apexTestRequest(cmd, kind)
			if err != nil {
				return err
			}
			org, err := resolveTargetOrg(cmd)
			if err != nil {
				return err
			}
			started := time.Now()
			synchronous, _ := cmd.Flags().GetBool("synchronous")
			var run *apexTestRun
			if synchronous {
				run, err = runTestsSynchronously(org, request)
				if err != nil {
					return err
				}
			} else {
				content, err := encodeJSON(request)
				if err != nil {
					return err
				}
				body, err := org.Force.PostREST("/tooling/runTestsAsynchronous", content)
				if err != nil {
					// sf shows the error code of a refused test run in
					// place of the exit status.
					converted := apiError(err)
					if e, ok := converted.(*Error); ok {
						e.Label = ""
						e.Data = nil
					}
					return converted
				}
				var testRunID string
				if err := json.Unmarshal([]byte(body), &testRunID); err != nil {
					testRunID = strings.Trim(strings.TrimSpace(body), `"`)
				}
				notWaiting := func() error {
					result := newOrderedMap()
					result.Set("testRunId", testRunID)
					return emit(cmd, result, func(w io.Writer) {
						period := ""
						if kind == flowTests {
							period = "."
						}
						fmt.Fprintf(w, "Run \""+binName+" "+topic+" get test -i %s -o %s\" to retrieve test results%s\n", testRunID, org.Username, period)
						fmt.Fprintln(w, "Run with --synchronous or increase --wait timeout to wait for results.")
					})
				}
				wait, _ := cmd.Flags().GetInt("wait")
				if !cmd.Flags().Changed("wait") {
					return notWaiting()
				}
				interval := testPollInterval
				if kind == apexTests {
					if seconds, _ := cmd.Flags().GetInt("poll-interval"); seconds > 0 {
						interval = time.Duration(seconds) * time.Second
					}
				}
				queueItemIDs, finished, err := waitForTestRun(org, testRunID, time.Duration(wait)*time.Minute, interval)
				if err != nil {
					return err
				}
				if !finished {
					return notWaiting()
				}
				run, err = loadTestRun(org, testRunID, queueItemIDs)
				if err != nil {
					return err
				}
			}
			run.Logic = logic
			return reportTestRun(cmd, org, run, format, time.Since(started))
		},
	}
	addJSONFlag(cmd)
	if kind == flowTests {
		cmd.Flags().BoolP("code-coverage", "c", false, "Retrieve code coverage results.")
		cmd.Flags().StringP("output-dir", "d", "", "Directory in which to store test result files.")
		cmd.Flags().StringP("test-level", "l", "", "Level of tests to run; default is RunLocalTests.")
		cmd.Flags().StringArrayP("class-names", "n", nil, "Flow test class names to run; default is all classes.")
		cmd.Flags().StringP("result-format", "r", "human", "Format of the test results.")
		cmd.Flags().StringArrayP("suite-names", "s", nil, "Flow test suite names to run.")
		cmd.Flags().StringArrayP("tests", "t", nil, "Flow test class names or IDs and, if applicable, test methods to run; default is all tests.")
		cmd.Flags().BoolP("synchronous", "y", false, "Runs test methods from a single Flow synchronously.")
		cmd.Flags().Bool("concise", false, "Display only failed test results; works with human-readable output only.")
		addAPIVersionFlag(cmd)
		addTargetOrgFlag(cmd)
		return cmd
	}
	cmd.Flags().BoolP("code-coverage", "c", false, "Retrieve code coverage results.")
	cmd.Flags().StringP("output-dir", "d", "", "Directory in which to store test run files.")
	cmd.Flags().StringP("test-level", "l", "", "Level of tests to run; default is RunLocalTests.")
	cmd.Flags().StringArrayP("class-names", "n", nil, "Apex test class names to run.")
	cmd.Flags().StringP("result-format", "r", "human", "Format of the test results.")
	cmd.Flags().StringArrayP("suite-names", "s", nil, "Apex test suite names to run.")
	if logic {
		cmd.Flags().StringArrayP("tests", "t", nil, "Comma-separated list of test names to run. Can include Apex test classes and Flow tests.")
		cmd.Flags().StringArray("test-category", nil, "Category of tests to run, such as Apex or Flow.")
	} else {
		cmd.Flags().StringArrayP("tests", "t", nil, "Apex test class names or IDs and, if applicable, test methods to run.")
		cmd.Flags().IntP("poll-interval", "i", 0, "Number of seconds to wait between retries.")
	}
	cmd.Flags().IntP("wait", "w", 0, "Sets the streaming client socket timeout in minutes.")
	cmd.Flags().BoolP("synchronous", "y", false, "Runs test methods from a single Apex class synchronously.")
	cmd.Flags().BoolP("detailed-coverage", "v", false, "Display detailed code coverage per test.")
	cmd.Flags().Bool("concise", false, "Display only failed test results; works with human-readable output only.")
	addAPIVersionFlag(cmd)
	addTargetOrgFlag(cmd)
	return cmd
}

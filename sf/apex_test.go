package sf

import (
	"encoding/json"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

// The output formats in this file were captured from sf 2.138.6.

const executeAnonymousResponse = `<?xml version="1.0" encoding="UTF-8"?><soapenv:Envelope xmlns:soapenv="http://schemas.xmlsoap.org/soap/envelope/" xmlns="http://soap.sforce.com/2006/08/apex"><soapenv:Header><DebuggingInfo><debugLog>%LOG%</debugLog></DebuggingInfo></soapenv:Header><soapenv:Body><executeAnonymousResponse><result><column>%COLUMN%</column><compileProblem>%PROBLEM%</compileProblem><compiled>%COMPILED%</compiled><exceptionMessage>%EXCEPTION%</exceptionMessage><exceptionStackTrace></exceptionStackTrace><line>%LINE%</line><success>%SUCCESS%</success></result></executeAnonymousResponse></soapenv:Body></soapenv:Envelope>`

func anonymousResponse(log, problem, exception string, compiled, success bool, line, column string) string {
	replacer := strings.NewReplacer("%LOG%", log, "%PROBLEM%", problem, "%EXCEPTION%", exception,
		"%COMPILED%", map[bool]string{true: "true", false: "false"}[compiled],
		"%SUCCESS%", map[bool]string{true: "true", false: "false"}[success], "%LINE%", line, "%COLUMN%", column)
	return replacer.Replace(executeAnonymousResponse)
}

func TestApexRun(t *testing.T) {
	org := loggedInFakeOrg(t)
	var sent string
	response := anonymousResponse("DEBUG | hello from apex", "", "", true, true, "-1", "-1")
	org.handlers["/services/Soap/s/67.0/00D000000000000"] = func(w http.ResponseWriter, r *http.Request) {
		body, _ := io.ReadAll(r.Body)
		sent = string(body)
		w.Header().Set("Content-Type", "text/xml")
		io.WriteString(w, response)
	}
	file := filepath.Join(t.TempDir(), "hello.apex")
	if err := os.WriteFile(file, []byte("System.debug('hello from apex');\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	expectRun(t, 0, "Compiled successfully.\nExecuted successfully.\n\nDEBUG | hello from apex\n", "apex", "run", "-f", file)
	if !strings.Contains(sent, "System.debug('hello from apex');") {
		t.Errorf("request does not contain the code: %s", sent)
	}
	expectRun(t, 0, `{
  "status": 0,
  "result": {
    "success": true,
    "compiled": true,
    "compileProblem": "",
    "exceptionMessage": "",
    "exceptionStackTrace": "",
    "line": -1,
    "column": -1,
    "logs": "DEBUG | hello from apex"
  },
  "warnings": [
    "The \"force apex execute\" command has been deprecated. Use \"apex run\" instead."
  ]
}
`, "force:apex:execute", "--file", file, "--json")

	response = anonymousResponse("", "Unexpected token ';'.", "", false, false, "1", "12")
	_, errOut, status := run("apex", "run", "-f", file)
	if status != 1 || errOut != "Error (executeCompileFailure): Compilation failed at Line 1 column 12 with the error:\n\nUnexpected token ';'.\n\n\n" {
		t.Errorf("compile failure: status %d, stderr %q", status, errOut)
	}
	out, _, _ := run("apex", "run", "-f", file, "--json")
	if !strings.Contains(out, `"name": "executeCompileFailure"`) || !strings.Contains(out, `"line": "1"`) || !strings.Contains(out, `"context": "Run"`) {
		t.Errorf("compile failure json:\n%s", out)
	}

	response = anonymousResponse("", "", "System.NullPointerException: Attempt to de-reference a null object", true, false, "3", "1")
	_, errOut, status = run("apex", "run", "-f", file)
	if status != 1 || errOut != "Error (executeRuntimeFailure): Execution failed at this code:\n\nSystem.NullPointerException: Attempt to de-reference a null object\n\n\n" {
		t.Errorf("runtime failure: status %d, stderr %q", status, errOut)
	}
}

func TestTextTableMatchesTheSfTestReporter(t *testing.T) {
	got := textTable("Test Results", []string{"TEST NAME", "OUTCOME", "MESSAGE", "RUNTIME (MS)"}, [][]string{
		{"GreeterTest.fails", "Fail", "Assertion Failed\nClass.GreeterTest.fails: line 10, column 1", ""},
		{"GreeterTest.passes", "Pass", "", "10"},
	})
	stack := "Class.GreeterTest.fails: line 10, column 1"
	pad := func(text string) string { return text + strings.Repeat(" ", len(stack)-len(text)) }
	want := "=== Test Results\n" +
		"TEST NAME           OUTCOME  " + pad("MESSAGE") + "  RUNTIME (MS)\n" +
		"──────────────────  ───────  " + strings.Repeat("─", len(stack)) + "  ────────────\n" +
		// The last column is not padded on the first line of a row whose
		// cell is empty, and is padded on a continuation line.
		"GreeterTest.fails   Fail     " + pad("Assertion Failed") + "  \n" +
		"                             " + stack + "              \n" +
		"GreeterTest.passes  Pass     " + pad("") + "  10          \n"
	if got != want {
		t.Errorf("table =\n%s\nwant\n%s", strings.ReplaceAll(got, " ", "·"), strings.ReplaceAll(want, " ", "·"))
	}
}

// serveTestRun makes the fake org answer the requests of an asynchronous
// test run with one failing and one passing test.
func serveTestRun(org *fakeOrg) *string {
	var request string
	org.handlers["/services/data/v67.0/tooling/runTestsAsynchronous"] = func(w http.ResponseWriter, r *http.Request) {
		body, _ := io.ReadAll(r.Body)
		request = string(body)
		io.WriteString(w, `"707000000000001AAA"`)
	}
	org.handlers["/services/data/v67.0/tooling/query"] = func(w http.ResponseWriter, r *http.Request) {
		soql := r.URL.Query().Get("q")
		switch {
		case strings.Contains(soql, "FROM ApexTestQueueItem"):
			io.WriteString(w, `{"done":true,"totalSize":1,"records":[{"Id":"709000000000001AAA","Status":"Completed","ApexClassId":"01p000000000002AAA"}]}`)
		case strings.Contains(soql, "FROM ApexTestRunResult"):
			io.WriteString(w, `{"done":true,"totalSize":1,"records":[{"AsyncApexJobId":"707000000000001AAA","Status":"Completed","StartTime":"2026-01-02T03:04:05.678+0000","TestTime":83,"TestSetupTime":0,"UserId":"005000000000001AAA"}]}`)
		case strings.Contains(soql, "FROM ApexTestResult"):
			io.WriteString(w, `{"done":true,"totalSize":2,"records":[`+
				`{"Id":"07M000000000002AAA","QueueItemId":"709000000000001AAA","StackTrace":null,"Message":null,"RunTime":9,"AsyncApexJobId":"707000000000001AAA","MethodName":"passes","Outcome":"Pass","IsTestSetup":false,"ApexClass":{"Id":"01p000000000002AAA","Name":"GreeterTest","NamespacePrefix":null}},`+
				`{"Id":"07M000000000001AAA","QueueItemId":"709000000000001AAA","StackTrace":"Class.GreeterTest.fails: line 10, column 1","Message":"System.AssertException: Assertion Failed: <wrong>","RunTime":7,"AsyncApexJobId":"707000000000001AAA","MethodName":"fails","Outcome":"Fail","IsTestSetup":false,"ApexClass":{"Id":"01p000000000002AAA","Name":"GreeterTest","NamespacePrefix":null}}]}`)
		case strings.Contains(soql, "FROM ApexCodeCoverageAggregate"):
			io.WriteString(w, `{"done":true,"totalSize":1,"records":[{"ApexClassOrTrigger":{"Id":"01p000000000001AAA","Name":"Greeter"},"NumLinesCovered":3,"NumLinesUncovered":1,"Coverage":{"coveredLines":[2,3,5],"uncoveredLines":[7]}}]}`)
		case strings.Contains(soql, "FROM ApexCodeCoverage"):
			io.WriteString(w, `{"done":true,"totalSize":1,"records":[{"ApexTestClassId":"01p000000000002AAA","ApexClassOrTrigger":{"Id":"01p000000000001AAA","Name":"Greeter"},"TestMethodName":"passes","NumLinesCovered":3,"NumLinesUncovered":1,"Coverage":{"coveredLines":[2,3,5],"uncoveredLines":[7]}}]}`)
		case strings.Contains(soql, "FROM ApexOrgWideCoverage"):
			io.WriteString(w, `{"done":true,"totalSize":1,"records":[{"PercentCovered":61}]}`)
		default:
			w.WriteHeader(http.StatusBadRequest)
			io.WriteString(w, `[{"errorCode":"INVALID_QUERY","message":"unsupported tooling query object"}]`)
		}
	}
	return &request
}

func TestApexRunTestHumanReport(t *testing.T) {
	org := loggedInFakeOrg(t)
	request := serveTestRun(org)
	previous := testPollInterval
	testPollInterval = time.Millisecond
	defer func() { testPollInterval = previous }()

	out, _, status := run("apex", "run", "test", "-n", "GreeterTest", "-w", "2", "-c")
	if status != 100 {
		t.Errorf("status = %d, want 100 for a run with a failing test", status)
	}
	if *request != `{"tests":[{"className":"GreeterTest"}],"testLevel":"RunSpecifiedTests","skipCodeCoverage":false}` {
		t.Errorf("request = %s", *request)
	}
	message := "System.AssertException: Assertion Failed: <wrong>"
	stack := "Class.GreeterTest.fails: line 10, column 1"
	pad := func(text string) string { return text + strings.Repeat(" ", len(message)-len(text)) }
	want := "=== Test Results\n" +
		"TEST NAME           OUTCOME  " + pad("MESSAGE") + "  RUNTIME (MS)\n" +
		"──────────────────  ───────  " + strings.Repeat("─", len(message)) + "  ────────────\n" +
		"GreeterTest.fails   Fail     " + message + "  \n" +
		"                             " + pad(stack) + "              \n" +
		"GreeterTest.passes  Pass     " + pad("") + "  9           \n" +
		"\n\n" +
		"=== Apex Code Coverage by Class\n" +
		"CLASSES  PERCENT  UNCOVERED LINES\n" +
		"───────  ───────  ───────────────\n" +
		"Greeter  75%      7              \n" +
		"\n\n\n\n" +
		"=== Test Summary\n" +
		"NAME                 VALUE             \n" +
		"───────────────────  ──────────────────\n" +
		"Outcome              Failed            \n" +
		"Tests Ran            2                 \n" +
		"Pass Rate            50%               \n" +
		"Fail Rate            50%               \n" +
		"Skip Rate            0%                \n" +
		"Test Run Id          707000000000001AAA\n" +
		"Test Setup Time      0 ms              \n" +
		"Test Execution Time  83 ms             \n" +
		"Test Total Time      83 ms             \n" +
		"Org Id               00D000000000000   \n" +
		"Username             me@example.com    \n" +
		"Org Wide Coverage    61%               \n" +
		"\n"
	if out != want {
		t.Errorf("report =\n%s\nwant\n%s", strings.ReplaceAll(out, " ", "·"), strings.ReplaceAll(want, " ", "·"))
	}
}

func TestApexRunTestJSONTapAndJUnit(t *testing.T) {
	org := loggedInFakeOrg(t)
	serveTestRun(org)
	previous := testPollInterval
	testPollInterval = time.Millisecond
	defer func() { testPollInterval = previous }()

	out, _, status := run("apex", "run", "test", "--class-names", "GreeterTest", "--wait", "2", "--code-coverage", "--json")
	var env struct {
		Status int
		Result struct {
			Summary  map[string]interface{}
			Tests    []map[string]interface{}
			Coverage struct {
				Coverage []map[string]interface{}
				Records  []map[string]interface{}
				Summary  map[string]interface{}
			}
		}
	}
	if err := json.Unmarshal([]byte(out), &env); err != nil {
		t.Fatalf("%v\n%s", err, out)
	}
	summary := env.Result.Summary
	if status != 100 || env.Status != 100 || summary["outcome"] != "Failed" || summary["passRate"] != "50%" || summary["failing"] != float64(1) ||
		summary["testRunId"] != "707000000000001AAA" || summary["testStartTime"] != "2026-01-02T03:04:05.678Z" ||
		summary["testExecutionTime"] != "83 ms" || summary["orgWideCoverage"] != "61%" || summary["testRunCoverage"] != "75%" ||
		summary["userId"] != "005000000000001AAA" {
		t.Errorf("status %d, summary %v", status, summary)
	}
	if len(env.Result.Tests) != 2 || env.Result.Tests[0]["FullName"] != "GreeterTest.fails" || env.Result.Tests[1]["StackTrace"] != nil {
		t.Errorf("tests = %v", env.Result.Tests)
	}
	coverage := env.Result.Coverage
	if len(coverage.Coverage) != 1 || coverage.Coverage[0]["coveredPercent"] != float64(75) || coverage.Coverage[0]["totalLines"] != float64(4) ||
		coverage.Summary["coveredLines"] != float64(3) || len(coverage.Records) != 1 {
		t.Errorf("coverage = %+v", coverage)
	}
	// The ApexTestClass Id sf reports is the Id of the test result.
	if testClass, _ := coverage.Records[0]["ApexTestClass"].(map[string]interface{}); testClass["Id"] != "07M000000000002AAA" {
		t.Errorf("coverage record = %v", coverage.Records[0])
	}
	if !strings.Contains(out, "\"lines\": {\n            \"2\": 1,\n            \"3\": 1,\n            \"5\": 1,\n            \"7\": 0\n          }") {
		t.Errorf("coverage lines are not listed in line order:\n%s", out)
	}

	expectRun(t, 100, "1..2\n"+
		"not ok 1 GreeterTest.fails\n"+
		"# System.AssertException: Assertion Failed: <wrong>\n"+
		"# Class.GreeterTest.fails: line 10, column 1\n"+
		"ok 2 GreeterTest.passes\n"+
		"# Run \"sf apex get test -i 707000000000001AAA -o me@example.com --result-format <format>\" to retrieve test results in a different format.\n\n",
		"apex", "run", "test", "-n", "GreeterTest", "-w", "2", "-r", "tap")

	out, _, _ = run("apex", "run", "test", "-n", "GreeterTest", "-w", "2", "-r", "junit")
	for _, want := range []string{
		`<testsuite name="force.apex" timestamp="2026-01-02T03:04:05.678Z" hostname="` + org.URL + `/" tests="2" failures="1"  errors="0"  time="0.08">`,
		`<property name="passRate" value="50%"/>`,
		`<testcase name="fails" classname="GreeterTest" time="0.01">`,
		`<failure message="System.AssertException: Assertion Failed: &lt;wrong&gt;"><![CDATA[Class.GreeterTest.fails: line 10, column 1]]></failure>`,
	} {
		if !strings.Contains(out, want) {
			t.Errorf("junit output lacks %s:\n%s", want, out)
		}
	}
}

func TestApexRunTestWithoutWaitReportsTheRunId(t *testing.T) {
	org := loggedInFakeOrg(t)
	request := serveTestRun(org)
	expectRun(t, 0, "Run \"sf apex get test -i 707000000000001AAA -o me@example.com\" to retrieve test results\nRun with --synchronous or increase --wait timeout to wait for results.\n",
		"apex", "run", "test", "-t", "GreeterTest.passes", "-t", "GreeterTest.fails,OtherTest.one")
	want := `{"tests":[{"className":"GreeterTest","testMethods":["passes","fails"]},{"className":"OtherTest","testMethods":["one"]}],"testLevel":"RunSpecifiedTests","skipCodeCoverage":true}`
	if *request != want {
		t.Errorf("request = %s\nwant      %s", *request, want)
	}
	expectRun(t, 0, `{
  "status": 0,
  "result": {
    "testRunId": "707000000000001AAA"
  },
  "warnings": []
}
`, "apex", "run", "test", "--json")
	if *request != `{"testLevel":"RunLocalTests","category":["Apex"],"skipCodeCoverage":true}` {
		t.Errorf("default request = %s", *request)
	}
}

func TestApexRunTestSynchronous(t *testing.T) {
	org := loggedInFakeOrg(t)
	org.handlers["/services/data/v67.0/tooling/runTestsSynchronous"] = func(w http.ResponseWriter, r *http.Request) {
		io.WriteString(w, `{"numTestsRun":1,"numFailures":0,"totalTime":86,"successes":[{"id":"01p000000000002AAA","methodName":"passes","name":"GreeterTest","namespace":null,"time":14}],"failures":[]}`)
	}
	out, _, status := run("apex", "run", "test", "-t", "GreeterTest.passes", "-y", "--json")
	if status != 0 || !strings.Contains(out, `"testRunId": ""`) || !strings.Contains(out, `"outcome": "Passed"`) ||
		!strings.Contains(out, `"testExecutionTime": "86 ms"`) || strings.Contains(out, `"userId"`) ||
		!strings.Contains(out, "\"Id\": \"\",\n        \"QueueItemId\": \"\",\n        \"StackTrace\": \"\",") {
		t.Errorf("status %d\n%s", status, out)
	}
}

func TestApexLogCommands(t *testing.T) {
	org := loggedInFakeOrg(t)
	org.handlers["/services/data/v67.0/tooling/query"] = func(w http.ResponseWriter, r *http.Request) {
		if strings.Contains(r.URL.Query().Get("q"), "LIMIT 1") {
			io.WriteString(w, `{"done":true,"totalSize":1,"records":[{"Id":"07L000000000001AAA","StartTime":"2026-01-02T03:04:05.000+0000"}]}`)
			return
		}
		io.WriteString(w, `{"done":true,"totalSize":1,"records":[{"Id":"07L000000000001AAA","Application":"Unknown","DurationMilliseconds":21,"Location":"SystemLog","LogLength":1234,"LogUser":{"Name":"Ada Lovelace"},"Operation":"Api","Request":"Api","StartTime":"2026-01-02T03:04:05.000+0000","Status":"Success"}]}`)
	}
	org.handlers["/services/data/v67.0/sobjects/ApexLog/07L000000000001AAA/Body"] = func(w http.ResponseWriter, r *http.Request) {
		io.WriteString(w, "67.0 APEX_CODE,DEBUG\nUSER_DEBUG|[1]|DEBUG|hi")
	}
	org.handlers["/services/data/v67.0/tooling/sobjects/ApexLog/07L000000000001AAA/Body"] = org.handlers["/services/data/v67.0/sobjects/ApexLog/07L000000000001AAA/Body"]

	expectRun(t, 0, `┌─────────────┬───────────────┬────────────────────┬───────────┬──────────┬──────────────┬───────────┬─────────┬──────────────────────────┬─────────┐
│ Application │ Duration (ms) │ Id                 │ Location  │ Size (B) │ Log User     │ Operation │ Request │ Start Time               │ Status  │
├─────────────┼───────────────┼────────────────────┼───────────┼──────────┼──────────────┼───────────┼─────────┼──────────────────────────┼─────────┤
│ Unknown     │ 21            │ 07L000000000001AAA │ SystemLog │ 1234     │ Ada Lovelace │ Api       │ Api     │ 2026-01-02T03:04:05+0000 │ Success │
└─────────────┴───────────────┴────────────────────┴───────────┴──────────┴──────────────┴───────────┴─────────┴──────────────────────────┴─────────┘

`, "apex", "list", "log")

	expectRun(t, 0, "67.0 APEX_CODE,DEBUG\nUSER_DEBUG|[1]|DEBUG|hi\n", "apex", "get", "log", "-i", "07L000000000001AAA")
	expectRun(t, 0, `{
  "status": 0,
  "result": [
    {
      "log": "67.0 APEX_CODE,DEBUG\nUSER_DEBUG|[1]|DEBUG|hi"
    }
  ],
  "warnings": []
}
`, "apex", "get", "log", "--number", "1", "--json")

	dir := t.TempDir()
	expectRun(t, 0, "Log files written to "+dir+"\n", "apex", "get", "log", "-n", "1", "-d", dir)
	written, err := os.ReadFile(filepath.Join(dir, "07L000000000001AAA.log"))
	if err != nil || !strings.HasPrefix(string(written), "67.0 APEX_CODE") {
		t.Errorf("log file = %q, err = %v", written, err)
	}
}

func TestApexListLogWithNoLogs(t *testing.T) {
	org := loggedInFakeOrg(t)
	org.handlers["/services/data/v67.0/tooling/query"] = func(w http.ResponseWriter, r *http.Request) {
		io.WriteString(w, `{"done":true,"totalSize":0,"records":[]}`)
	}
	expectRun(t, 0, "No debug logs found in org\n", "apex", "list", "log")
	expectRun(t, 0, "No results found\n", "apex", "get", "log", "-n", "1")
	// sf asks for a log id or a number of logs.
	_, errOut, status := run("apex", "get", "log")
	if status != 1 || errOut != "Error (1): To retrieve logs, specify the log ID or the number of logs.\n\n\n" {
		t.Errorf("no log named: status %d, stderr %q", status, errOut)
	}
}

func TestApexTraceCommands(t *testing.T) {
	org := loggedInFakeOrg(t)
	var created map[string]interface{}
	var deleted bool
	org.handlers["/services/data/v67.0/tooling/query"] = func(w http.ResponseWriter, r *http.Request) {
		soql := r.URL.Query().Get("q")
		switch {
		case strings.Contains(soql, "DeveloperName = 'SFDC_DevConsole'"):
			io.WriteString(w, `{"done":true,"totalSize":1,"records":[{"Id":"7dl000000000001AAA"}]}`)
		case strings.Contains(soql, "FROM DebugLevel"):
			io.WriteString(w, `{"done":true,"totalSize":0,"records":[]}`)
		default:
			io.WriteString(w, `{"done":true,"totalSize":2,"records":[`+
				`{"Id":"7tf000000000001AAA","TracedEntityId":"005000000000001AAA","LogType":"DEVELOPER_LOG","DebugLevelId":"7dl000000000001AAA","ExpirationDate":"2999-01-01T00:00:00.000+0000","DebugLevel":{"DeveloperName":"SFDC_DevConsole"},"TracedEntity":{"Name":"Ada Lovelace"}},`+
				`{"Id":"7tf000000000002AAA","TracedEntityId":"005000000000002AAA","LogType":"USER_DEBUG","DebugLevelId":"7dl000000000001AAA","ExpirationDate":"2001-01-01T00:00:00.000+0000","DebugLevel":null,"TracedEntity":null}]}`)
		}
	}
	org.handlers["/services/data/v67.0/tooling/sobjects/TraceFlag"] = func(w http.ResponseWriter, r *http.Request) {
		json.NewDecoder(r.Body).Decode(&created)
		w.WriteHeader(http.StatusCreated)
		io.WriteString(w, `{"id":"7tf000000000003AAA","success":true,"errors":[]}`)
	}
	org.handlers["/services/data/v67.0/tooling/sobjects/TraceFlag/7tf000000000003AAA"] = func(w http.ResponseWriter, r *http.Request) {
		deleted = r.Method == "DELETE"
		w.WriteHeader(http.StatusNoContent)
	}

	expectRun(t, 0, "Successfully created trace flag 7tf000000000003AAA.\n", "apex", "trace", "create", "-i", "005000000000001AAA", "-d", "SFDC_DevConsole", "--duration", "60")
	if created["TracedEntityId"] != "005000000000001AAA" || created["LogType"] != "DEVELOPER_LOG" || created["DebugLevelId"] != "7dl000000000001AAA" {
		t.Errorf("created = %v", created)
	}
	start, _ := time.Parse("2006-01-02T15:04:05.000Z", created["StartDate"].(string))
	end, _ := time.Parse("2006-01-02T15:04:05.000Z", created["ExpirationDate"].(string))
	if end.Sub(start) != time.Hour {
		t.Errorf("trace flag lasts %v, want 1h", end.Sub(start))
	}
	_, errOut, status := run("apex", "trace", "create", "-i", "005000000000001AAA", "-d", "Missing")
	if status != 1 || errOut != "Error (SfError): Debug level \"Missing\" not found. Create one in Setup or specify an existing debug level.\n\n\n" {
		t.Errorf("missing debug level: status %d, stderr %q", status, errOut)
	}

	// sf lists the required flags that are missing together.
	_, errOut, status = run("apex", "trace", "create")
	if status != 2 || errOut != "Error (2): The following errors occurred:\n  Missing required flag debug-level\n  Missing required flag traced-entity-id\nSee more help with --help\n\n\n" {
		t.Errorf("no flags: status %d, stderr %q", status, errOut)
	}
	out, _, status := run("apex", "trace", "create", "--json")
	if status != 2 || !strings.Contains(out, `"name": "Error"`) || !strings.Contains(out, `"code": "2"`) {
		t.Errorf("no flags, json: status %d\n%s", status, out)
	}

	expectRun(t, 0, `┌────────────────────┬────────────────────┬───────────────┬────────────────────┬─────────┬──────────────────────────────┐
│ Id                 │ Traced Entity      │ Log Type      │ Debug Level        │ Status  │ Expiration Date              │
├────────────────────┼────────────────────┼───────────────┼────────────────────┼─────────┼──────────────────────────────┤
│ 7tf000000000001AAA │ Ada Lovelace       │ DEVELOPER_LOG │ SFDC_DevConsole    │ Active  │ 2999-01-01T00:00:00.000+0000 │
│ 7tf000000000002AAA │ 005000000000002AAA │ USER_DEBUG    │ 7dl000000000001AAA │ Expired │ 2001-01-01T00:00:00.000+0000 │
└────────────────────┴────────────────────┴───────────────┴────────────────────┴─────────┴──────────────────────────────┘

`, "apex", "trace", "list")

	expectRun(t, 0, `{
  "status": 0,
  "result": {
    "id": "7tf000000000003AAA",
    "success": true
  },
  "warnings": []
}
`, "apex", "trace", "delete", "--trace-flag-id", "7tf000000000003AAA", "--json")
	if !deleted {
		t.Error("no DELETE request was made")
	}
}

func TestApexGetCompileStatus(t *testing.T) {
	org := loggedInFakeOrg(t)
	response := `{"results":[],"status":"OK"}`
	var requestBody string
	org.handlers["/services/data/v67.0/tooling/apexCompileResults"] = func(w http.ResponseWriter, r *http.Request) {
		data, _ := io.ReadAll(r.Body)
		requestBody = r.Method + " " + string(data)
		w.Write([]byte(response))
	}
	out, errOut, status := run("apex", "get", "compile-status")
	if status != 0 || out != "No Apex compiler issues found.\n" || errOut != "Retrieving Apex compilation results... done\n" {
		t.Errorf("status %d, stdout %q, stderr %q", status, out, errOut)
	}
	if requestBody != "POST {}" {
		t.Errorf("request = %q", requestBody)
	}
	expectRun(t, 0, `{
  "status": 0,
  "result": {
    "results": [],
    "status": "OK"
  },
  "warnings": []
}
`, "apex", "get", "compile-status", "--json")

	_, errOut, status = run("apex", "get", "compile-status", "--api-version", "60.0")
	if status != 1 || errOut != "Error (SfError): Apex compile results available in API versions 68.0 and above\n\n\n" {
		t.Errorf("old version: status %d, stderr %q", status, errOut)
	}

	// The layout of a result with problems follows the plugin source; it
	// was not compared with sf.
	response = `{"results":[{"name":"Broken","namespace":"","success":false,"problems":[{"line":3,"column":12,"message":"Variable does not exist: x"},{"line":9,"column":1,"message":"Missing return"}],"warnings":[]}],"status":"OK"}`
	expectRun(t, 0, `Compile Status Results:
┌────────┬───────────┬─────────┬─────────────────────────────────────┬──────────┐
│ Name   │ Namespace │ Success │ Problems                            │ Warnings │
├────────┼───────────┼─────────┼─────────────────────────────────────┼──────────┤
│ Broken │           │ false   │ L3:C12 - Variable does not exist: x │          │
│        │           │         │ L9:C1 - Missing return              │          │
└────────┴───────────┴─────────┴─────────────────────────────────────┴──────────┘

`, "apex", "get", "compile-status")
}

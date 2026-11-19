package sf

import (
	"io"
	"net/http"
	"strings"
	"testing"
	"time"
)

// The logic commands follow the sf Apex plugin and library source.

func TestLogicRunTestReportsApexAndFlowTests(t *testing.T) {
	org := loggedInFakeOrg(t)
	request := serveTestRun(org)
	apexQueries := org.handlers["/services/data/v67.0/tooling/query"]
	org.handlers["/services/data/v67.0/tooling/query"] = func(w http.ResponseWriter, r *http.Request) {
		soql := r.URL.Query().Get("q")
		switch {
		case strings.Contains(soql, "FROM ApexTestQueueItem"):
			// One Apex class and one Flow test, which has no class.
			io.WriteString(w, `{"done":true,"totalSize":2,"records":[{"Id":"709000000000001AAA","Status":"Completed","ApexClassId":"01p000000000002AAA"},{"Id":"709000000000002AAA","Status":"Completed","ApexClassId":null}]}`)
		case strings.Contains(soql, "FROM FlowTestResult WHERE ApexTestQueueItemId IN ('709000000000002AAA')"):
			io.WriteString(w, `{"done":true,"totalSize":1,"records":[{"Id":"4Ti000000000001AAA","ApexTestQueueItemId":"709000000000002AAA","Result":"Pass","TestStartDateTime":"2026-01-02T03:04:05.000+0000","TestEndDateTime":"2026-01-02T03:04:06.000+0000","FlowTest":{"DeveloperName":"Approves_Small_Orders"},"FlowDefinition":{"DeveloperName":"Approve_Order","NamespacePrefix":null}}]}`)
		default:
			apexQueries(w, r)
		}
	}
	previous := testPollInterval
	testPollInterval = time.Millisecond
	defer func() { testPollInterval = previous }()

	out, errOut, status := run("logic", "run", "test", "-l", "RunLocalTests", "--test-category", "Apex,Flow", "-w", "2")
	if status != 100 || !strings.HasPrefix(errOut, "Warning: This command is currently in beta.") || !strings.Contains(errOut, "Warning: The input format for array arguments has changed.") {
		t.Errorf("status %d, stderr %q", status, errOut)
	}
	if *request != `{"testLevel":"RunLocalTests","category":["Apex","Flow"],"skipCodeCoverage":true}` {
		t.Errorf("request = %s", *request)
	}
	for _, want := range []string{
		"TEST NAME                            CATEGORY  OUTCOME  MESSAGE",
		"Approve_Order.Approves_Small_Orders  Flow      Pass ",
		"GreeterTest.passes                   Apex      Pass ",
		"Tests Ran            3 ",
	} {
		if !strings.Contains(out, want) {
			t.Errorf("report lacks %q:\n%s", want, out)
		}
	}
	out, _, _ = run("logic", "get", "test", "-i", "707000000000001AAA", "--json")
	for _, want := range []string{`"Category": "Flow"`, `"Category": "Apex"`, `"FullName": "Approve_Order.Approves_Small_Orders"`, `"MethodName": "Approves_Small_Orders"`, `"Name": "Approve_Order"`} {
		if !strings.Contains(out, want) {
			t.Errorf("json lacks %s:\n%s", want, out)
		}
	}
	// "apex get test" reports the same run without the category.
	if out, _, _ = run("apex", "get", "test", "-i", "707000000000001AAA", "--json"); strings.Contains(out, `"Category"`) || !strings.Contains(out, "Approve_Order.Approves_Small_Orders") {
		t.Errorf("apex get test:\n%s", out)
	}
}

func TestLogicRunTestRequests(t *testing.T) {
	org := loggedInFakeOrg(t)
	request := serveTestRun(org)
	for _, c := range []struct {
		args []string
		want string
	}{
		// Without a category every test of the level runs.
		{[]string{}, `{"testLevel":"RunLocalTests","skipCodeCoverage":true}`},
		{[]string{"-l", "RunAllTestsInOrg", "--test-category", "Flow"}, `{"testLevel":"RunAllTestsInOrg","category":["Flow"],"skipCodeCoverage":true}`},
		// Flow tests are named FlowTesting.<flow>.<test>.
		{[]string{"-t", "FlowTesting.Approve_Order.Approves_Small_Orders", "-t", "FlowTesting.Approve_Order.Rejects_Large_Orders", "-t", "GreeterTest.passes"},
			`{"tests":[{"className":"GreeterTest","testMethods":["passes"]},{"className":"FlowTesting.Approve_Order","testMethods":["Approves_Small_Orders","Rejects_Large_Orders"]}],"testLevel":"RunSpecifiedTests","skipCodeCoverage":true}`},
		{[]string{"-t", "FlowTesting.acme.Approve_Order.Approves_Small_Orders"},
			`{"tests":[{"namespace":"FlowTesting.acme","className":"FlowTesting.acme.Approve_Order","testMethods":["Approves_Small_Orders"]}],"testLevel":"RunSpecifiedTests","skipCodeCoverage":true}`},
		{[]string{"-n", "FlowTesting.Approve_Order", "-l", "RunSpecifiedTests", "--test-category", "Flow"},
			`{"tests":[{"className":"FlowTesting.Approve_Order"}],"testLevel":"RunSpecifiedTests","skipCodeCoverage":true}`},
	} {
		out, errOut, status := run(append([]string{"logic", "run", "test"}, c.args...)...)
		if status != 0 || *request != c.want {
			t.Errorf("%v: status %d, stderr %q\nrequest %s\nwant    %s", c.args, status, errOut, *request, c.want)
		}
		if !strings.Contains(out, `logic get test -i 707000000000001AAA -o me@example.com" to retrieve test results`) {
			t.Errorf("%v: stdout %q", c.args, out)
		}
	}
	_, errOut, status := run("logic", "run", "test", "--test-category", "Flow")
	if status != 1 || !strings.Contains(errOut, "When using --test-category, you must also specify --test-level.") {
		t.Errorf("category without level: status %d, stderr %q", status, errOut)
	}
	if _, errOut, status = run("logic", "run", "test", "-l", "RunLocalTests", "--test-category", "Unit"); status != 2 || !strings.Contains(errOut, "Expected --test-category=Unit to be one of: Apex, Flow") {
		t.Errorf("other category: status %d, stderr %q", status, errOut)
	}
	if _, errOut, status = run("logic", "run", "test", "-i", "5"); status == 0 {
		t.Errorf("logic run test accepted --poll-interval: stderr %q", errOut)
	}

	// Compared with sf: the checks sf makes before it sends the request,
	// and the error of a refused run.
	for _, c := range []struct {
		args   []string
		stderr string
	}{
		{[]string{"logic", "run", "test", "-n", "ATest", "-n", "BTest", "-y"}, "Error (1): -- Synchronous test runs can include test methods from only one Apex class or Flow test. Omit the --synchronous flag or include tests from only one class\n\n\n"},
		{[]string{"apex", "run", "test", "-n", "ATest", "-n", "BTest", "-y"}, "Error (1): Synchronous test runs can include test methods from only one Apex class. Omit the --synchronous flag or include tests from only one class\n\n\n"},
		{[]string{"apex", "run", "test", "-n", "ATest", "-l", "RunLocalTests"}, "Error (1): When specifying classnames, suitenames, or tests, indicate RunSpecifiedTests as the testlevel\n\n\n"},
	} {
		_, errOut, status := run(c.args...)
		if status != 1 || !strings.HasSuffix(errOut, c.stderr) {
			t.Errorf("%v: status %d, stderr %q", c.args, status, errOut)
		}
	}
	org.handlers["/services/data/v67.0/tooling/runTestsAsynchronous"] = func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusBadRequest)
		io.WriteString(w, `[{"message":"No tests found for category: Flow","errorCode":"INVALID_INPUT"}]`)
	}
	_, errOut, status = run("logic", "run", "test", "-l", "RunLocalTests", "--test-category", "Flow")
	if status != 1 || !strings.HasSuffix(errOut, "Error (INVALID_INPUT): No tests found for category: Flow\n\n\n") {
		t.Errorf("refused run: status %d, stderr %q", status, errOut)
	}
	out, _, _ := run("logic", "run", "test", "-l", "RunLocalTests", "--test-category", "Flow", "--json")
	if !strings.Contains(out, `"name": "INVALID_INPUT"`) || !strings.Contains(out, `"code": "INVALID_INPUT"`) || strings.Contains(out, `"data"`) {
		t.Errorf("refused run json:\n%s", out)
	}
	request = serveTestRun(org)

	// "apex run test" asks for the Apex tests only, unless tests are named.
	run("apex", "run", "test")
	if *request != `{"testLevel":"RunLocalTests","category":["Apex"],"skipCodeCoverage":true}` {
		t.Errorf("apex run test request = %s", *request)
	}
}

// The flow commands follow the source of the sf flow plugin, version
// 2.0.2.  They were not compared with sf.

func TestFlowRunTestRequests(t *testing.T) {
	org := loggedInFakeOrg(t)
	request := serveTestRun(org)
	for _, c := range []struct {
		args []string
		want string
	}{
		{[]string{}, `{"testLevel":"RunLocalTests","category":["flow"],"skipCodeCoverage":true}`},
		// Each name gets the prefix "flowtesting.".
		{[]string{"-n", "Approve_Order", "-c"}, `{"tests":[{"className":"flowtesting.Approve_Order"}],"testLevel":"RunSpecifiedTests","skipCodeCoverage":false}`},
		{[]string{"-t", "Approve_Order.Approves_Small_Orders"}, `{"tests":[{"namespace":"flowtesting","className":"Approve_Order","testMethods":["Approves_Small_Orders"]}],"testLevel":"RunSpecifiedTests","skipCodeCoverage":true}`},
		{[]string{"-s", "Orders"}, `{"suiteNames":"Orders","testLevel":"RunSpecifiedTests","category":["flow"],"skipCodeCoverage":true}`},
	} {
		// The command has no --wait flag: it prints the Id of the run.
		out, errOut, status := run(append([]string{"flow", "run", "test"}, c.args...)...)
		if status != 0 || errOut != "" || *request != c.want {
			t.Errorf("%v: status %d, stderr %q\nrequest %s\nwant    %s", c.args, status, errOut, *request, c.want)
		}
		if out != "Run \"sf flow get test -i 707000000000001AAA -o me@example.com\" to retrieve test results.\nRun with --synchronous or increase --wait timeout to wait for results.\n" {
			t.Errorf("%v: stdout %q", c.args, out)
		}
	}
	for _, c := range []struct {
		args   []string
		stderr string
	}{
		{[]string{"-n", "A", "-n", "B", "-y"}, "Error (1): Synchronous test runs can include tests from only one Flow. Omit the --synchronous flag or include tests from only one flow.\n\n\n"},
		{[]string{"-n", "A", "-l", "RunLocalTests"}, "Error (1): When specifying flows or tests, indicate RunSpecifiedTests as the testlevel.\n\n\n"},
	} {
		_, errOut, status := run(append([]string{"flow", "run", "test"}, c.args...)...)
		if status != 1 || errOut != c.stderr {
			t.Errorf("%v: status %d, stderr %q", c.args, status, errOut)
		}
	}
	if _, _, status := run("flow", "run", "test", "-w", "5"); status == 0 {
		t.Errorf("flow run test accepted --wait")
	}
}

func TestFlowGetTestReportsFlowTestResults(t *testing.T) {
	org := loggedInFakeOrg(t)
	serveTestRun(org)
	apexQueries := org.handlers["/services/data/v67.0/tooling/query"]
	org.handlers["/services/data/v67.0/tooling/query"] = func(w http.ResponseWriter, r *http.Request) {
		soql := r.URL.Query().Get("q")
		switch {
		case strings.Contains(soql, "FROM ApexTestQueueItem"):
			io.WriteString(w, `{"done":true,"totalSize":1,"records":[{"Id":"709000000000002AAA","Status":"Completed","ApexClassId":null}]}`)
		case strings.Contains(soql, "FROM FlowTestResult"):
			io.WriteString(w, `{"done":true,"totalSize":2,"records":[`+
				`{"Id":"4Ti000000000001AAA","ApexTestQueueItemId":"709000000000002AAA","Result":"Pass","FlowTest":{"DeveloperName":"Approves_Small_Orders"},"FlowDefinition":{"DeveloperName":"Approve_Order","NamespacePrefix":null}},`+
				`{"Id":"4Ti000000000002AAA","ApexTestQueueItemId":"709000000000002AAA","Result":"Fail","FlowTest":{"DeveloperName":"Rejects_Large_Orders"},"FlowDefinition":{"DeveloperName":"Approve_Order","NamespacePrefix":"acme"}}]}`)
		default:
			apexQueries(w, r)
		}
	}
	out, errOut, status := run("flow", "get", "test", "-i", "707000000000001AAA")
	if status != 100 || errOut != "" {
		t.Errorf("status %d, stderr %q", status, errOut)
	}
	// The report has no category column.
	for _, want := range []string{
		"TEST NAME                                OUTCOME  MESSAGE  RUNTIME (MS)",
		"Approve_Order.Approves_Small_Orders      Pass              0",
		"acme.Approve_Order.Rejects_Large_Orders  Fail ",
		"Tests Ran            2 ", "Pass Rate            50%",
	} {
		if !strings.Contains(out, want) {
			t.Errorf("report lacks %q:\n%s", want, out)
		}
	}
	if strings.Contains(out, "CATEGORY") {
		t.Errorf("flow get test shows the category:\n%s", out)
	}
}

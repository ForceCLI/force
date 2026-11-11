package lib

import (
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func compileAndTestResponse(result string) []byte {
	return []byte(`<?xml version="1.0" encoding="UTF-8"?>
<soapenv:Envelope xmlns:soapenv="http://schemas.xmlsoap.org/soap/envelope/" xmlns="http://soap.sforce.com/2006/08/apex">
	<soapenv:Header><DebuggingInfo><debugLog>debug log</debugLog></DebuggingInfo></soapenv:Header>
	<soapenv:Body><compileAndTestResponse><result>` + result + `</result></compileAndTestResponse></soapenv:Body>
</soapenv:Envelope>`)
}

func TestCompileAndTestRequestNamesTheOrgNamespace(t *testing.T) {
	request := compileAndTestRequest("System.assert(true);", "myns")
	if !strings.Contains(request, "<runTestsRequest><classes>AnonymousTestClass</classes><namespace>myns</namespace></runTestsRequest>") {
		t.Fatalf("request does not run the class in the org namespace:\n%s", request)
	}
}

func TestCompileAndTestRequestOmitsNamespaceWithoutOne(t *testing.T) {
	request := compileAndTestRequest("System.assert(true);", "")
	if strings.Contains(request, "<namespace>") {
		t.Fatalf("request names a namespace for an org without one:\n%s", request)
	}
}

func TestParseCompileAndTestResponseReturnsTheLogOfAPassingTest(t *testing.T) {
	output, err := parseCompileAndTestResponse(compileAndTestResponse(`
		<runTestsResult><numFailures>0</numFailures><numTestsRun>1</numTestsRun></runTestsResult>
		<success>true</success>`))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if output != "debug log" {
		t.Fatalf("output = %q, want the debug log", output)
	}
}

func TestParseCompileAndTestResponseFailsWhenNoTestRan(t *testing.T) {
	_, err := parseCompileAndTestResponse(compileAndTestResponse(`
		<runTestsResult><numFailures>0</numFailures><numTestsRun>0</numTestsRun></runTestsResult>
		<success>true</success>`))
	if err == nil || err.Error() != "compileAndTest ran no tests" {
		t.Fatalf("err = %v, want compileAndTest ran no tests", err)
	}
}

func TestParseCompileAndTestResponseReturnsTheTestFailure(t *testing.T) {
	_, err := parseCompileAndTestResponse(compileAndTestResponse(`
		<runTestsResult>
			<failures><message>System.AssertException: Assertion Failed: deliberate failure</message></failures>
			<numFailures>1</numFailures><numTestsRun>1</numTestsRun>
		</runTestsResult>
		<success>false</success>`))
	if err == nil || err.Error() != "System.AssertException: Assertion Failed: deliberate failure" {
		t.Fatalf("err = %v, want the assertion failure", err)
	}
}

func TestParseCompileAndTestResponseReturnsTheCompileProblem(t *testing.T) {
	_, err := parseCompileAndTestResponse(compileAndTestResponse(`
		<classes><problem>Variable does not exist: x</problem><success>false</success></classes>
		<success>false</success>`))
	if err == nil || err.Error() != "Variable does not exist: x" {
		t.Fatalf("err = %v, want the compile problem", err)
	}
}

func TestParseCompileAndTestResponseFailsWithoutAReason(t *testing.T) {
	_, err := parseCompileAndTestResponse(compileAndTestResponse(`<success>false</success>`))
	if err == nil {
		t.Fatal("expected an error for an unsuccessful response")
	}
}

func newPartnerTestServer(t *testing.T, response string) (*Force, *string) {
	t.Helper()
	var request string
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		body, _ := io.ReadAll(r.Body)
		request = string(body)
		w.Header().Set("Content-Type", "text/xml")
		fmt.Fprint(w, response)
	}))
	t.Cleanup(server.Close)
	force := NewForce(&ForceSession{
		AccessToken:    "token",
		InstanceUrl:    server.URL,
		UserInfo:       &UserInfo{OrgId: "00D000000000001"},
		SessionOptions: &SessionOptions{ApiVersion: "62.0"},
	})
	return force, &request
}

const executeAnonymousResponse = `<?xml version="1.0" encoding="UTF-8"?>
<soapenv:Envelope xmlns:soapenv="http://schemas.xmlsoap.org/soap/envelope/" xmlns="http://soap.sforce.com/2006/08/apex">
	<soapenv:Header><DebuggingInfo><debugLog>62.0 APEX_CODE,FINEST</debugLog></DebuggingInfo></soapenv:Header>
	<soapenv:Body><executeAnonymousResponse><result><compiled>true</compiled><success>true</success></result></executeAnonymousResponse></soapenv:Body>
</soapenv:Envelope>`

func TestExecuteAnonymousAsksForDebugOnlyLog(t *testing.T) {
	force, request := newPartnerTestServer(t, executeAnonymousResponse)
	if _, err := force.Partner.ExecuteAnonymous("System.debug(1);"); err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(*request, "<apex:debugLevel>DEBUGONLY</apex:debugLevel>") || strings.Contains(*request, "Finest") {
		t.Errorf("request does not ask for the DEBUGONLY log:\n%s", *request)
	}
}

func TestWithReplayDebuggingAsksForFinestApexLog(t *testing.T) {
	force, request := newPartnerTestServer(t, executeAnonymousResponse)
	log, err := force.Partner.WithReplayDebugging().ExecuteAnonymous("System.debug(1);")
	if err != nil {
		t.Fatal(err)
	}
	if log != "62.0 APEX_CODE,FINEST" {
		t.Errorf("log = %q", log)
	}
	if !strings.Contains(*request, "<apex:categories><apex:category>Apex_code</apex:category><apex:level>Finest</apex:level></apex:categories>") {
		t.Errorf("request does not ask for Apex_code at Finest:\n%s", *request)
	}
	if strings.Contains(*request, "DEBUGONLY") {
		t.Errorf("request also asks for the DEBUGONLY log:\n%s", *request)
	}

	if _, err := force.Partner.ExecuteAnonymous("System.debug(1);"); err != nil {
		t.Fatal(err)
	}
	if strings.Contains(*request, "Finest") {
		t.Error("WithReplayDebugging changed the original partner")
	}
}

func TestRunTestsReturnsTheApexLogId(t *testing.T) {
	force, _ := newPartnerTestServer(t, `<?xml version="1.0" encoding="UTF-8"?>
<soapenv:Envelope xmlns:soapenv="http://schemas.xmlsoap.org/soap/envelope/" xmlns="http://soap.sforce.com/2006/08/apex">
	<soapenv:Body><runTestsResponse><result><apexLogId>07L000000000001</apexLogId><numFailures>0</numFailures><numTestsRun>1</numTestsRun></result></runTestsResponse></soapenv:Body>
</soapenv:Envelope>`)
	result, err := force.Partner.RunTests([]string{"CalcTest"}, "")
	if err != nil {
		t.Fatal(err)
	}
	if result.ApexLogId != "07L000000000001" || result.NumberRun != 1 {
		t.Errorf("result = %+v", result)
	}
}

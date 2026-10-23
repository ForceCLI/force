package lib

import (
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

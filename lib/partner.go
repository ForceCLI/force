package lib

import (
	"encoding/xml"
	"errors"
	"fmt"
	"strings"
)

type ForcePartner struct {
	Force *Force
	// debuggingHeader replaces debugOnlyHeader in Apex API calls when set.
	debuggingHeader string
}

const debugOnlyHeader = "<apex:DebuggingHeader><apex:debugLevel>DEBUGONLY</apex:debugLevel></apex:DebuggingHeader>"

// replayDebuggingHeader asks for Apex_code at Finest, which logs each
// statement and variable assignment that the replay debugger steps through.
// Apex_profiling at Info adds the cumulative limit usage.
const replayDebuggingHeader = "<apex:DebuggingHeader>" +
	"<apex:categories><apex:category>Apex_code</apex:category><apex:level>Finest</apex:level></apex:categories>" +
	"<apex:categories><apex:category>Visualforce</apex:category><apex:level>Finer</apex:level></apex:categories>" +
	"<apex:categories><apex:category>Apex_profiling</apex:category><apex:level>Info</apex:level></apex:categories>" +
	"</apex:DebuggingHeader>"

func NewForcePartner(force *Force) (partner *ForcePartner) {
	partner = &ForcePartner{Force: force}
	return
}

// WithReplayDebugging returns a copy of partner whose Apex API calls, such as
// executeAnonymous and runTests, return a debug log that can be replayed in
// the debugger. No trace flag is needed; the log comes back in the response.
func (partner *ForcePartner) WithReplayDebugging() *ForcePartner {
	p := *partner
	p.debuggingHeader = replayDebuggingHeader
	return &p
}

func (partner *ForcePartner) CheckStatus(id string) (err error) {
	body, err := partner.soapExecute("checkStatus", fmt.Sprintf("<id>%s</id>", id))
	if err != nil {
		return
	}
	var status struct {
		Done    bool   `xml:"Body>checkStatusResponse>result>done"`
		State   string `xml:"Body>checkStatusResponse>result>state"`
		Message string `xml:"Body>checkStatusResponse>result>message"`
	}
	if err = xml.Unmarshal(body, &status); err != nil {
		return
	}
	switch {
	case !status.Done:
		return partner.CheckStatus(id)
	case status.State == "Error":
		return errors.New(status.Message)
	}
	return
}

func (partner *ForcePartner) ExecuteAnonymous(apex string) (output string, err error) {
	soap := "<apexcode><![CDATA[%s]]></apexcode>"
	body, err := partner.soapExecute("executeAnonymous", fmt.Sprintf(soap, apex))
	if err != nil {
		return
	}
	var result struct {
		Compiled         bool   `xml:"Body>executeAnonymousResponse>result>compiled"`
		CompileProblem   string `xml:"Body>executeAnonymousResponse>result>compileProblem"`
		ExceptionMessage string `xml:"Body>executeAnonymousResponse>result>exceptionMessage"`
		ExceptionTrace   string `xml:"Body>executeAnonymousResponse>result>exceptionStackTrace"`
		Log              string `xml:"Header>DebuggingInfo>debugLog"`
		Success          bool   `xml:"Body>executeAnonymousResponse>result>success"`
	}
	if err = xml.Unmarshal(body, &result); err != nil {
		return
	}
	if !result.Compiled {
		message := strings.Replace(result.CompileProblem, "%", "%%", -1)
		err = errors.New(message)
		return
	}
	if !result.Success {
		err = errors.New(result.ExceptionMessage)
		return result.Log, err
	}
	output = result.Log
	return
}

func (partner *ForcePartner) ExecuteAnonymousTest(apex string) (output string, err error) {
	if err = partner.Force.EnsureUserInfo(); err != nil {
		return
	}
	body, err := partner.soapExecute("compileAndTest", compileAndTestRequest(apex, partner.Force.Credentials.UserInfo.OrgNamespace))
	if err != nil {
		return
	}
	return parseCompileAndTestResponse(body)
}

// compileAndTestRequest wraps apex in a test class and runs it. In an org with
// a namespace the class compiles into that namespace, and a runTestsRequest
// that names the class without it runs no tests.
func compileAndTestRequest(apex, namespace string) string {
	namespaceElement := ""
	if namespace != "" {
		namespaceElement = fmt.Sprintf("<namespace>%s</namespace>", namespace)
	}
	return fmt.Sprintf(`<CompileAndTestRequest>
		<classes><![CDATA[
		@IsTest(seeAllData=true)
		class AnonymousTestClass {
			@IsTest
			public static void test() {
				%s
			}
		}
		]]></classes>
		<checkOnly>true</checkOnly>
		<runTestsRequest><classes>AnonymousTestClass</classes>%s</runTestsRequest>
		</CompileAndTestRequest>
		`, apex, namespaceElement)
}

// parseCompileAndTestResponse returns the debug log of a compileAndTest call
// that ran the test and passed. Salesforce reports success when the request
// names a test it cannot find, so a run of no tests is an error.
func parseCompileAndTestResponse(body []byte) (output string, err error) {
	var result struct {
		Log             string   `xml:"Header>DebuggingInfo>debugLog"`
		Success         bool     `xml:"Body>compileAndTestResponse>result>success"`
		CompileProblems []string `xml:"Body>compileAndTestResponse>result>classes>problem"`
		TestFailures    []string `xml:"Body>compileAndTestResponse>result>runTestsResult>failures>message"`
		NumTestsRun     int      `xml:"Body>compileAndTestResponse>result>runTestsResult>numTestsRun"`
	}
	if err = xml.Unmarshal(body, &result); err != nil {
		return
	}
	if !result.Success {
		switch {
		case len(result.CompileProblems) > 0:
			err = errors.New(result.CompileProblems[0])
		case len(result.TestFailures) > 0:
			err = errors.New(result.TestFailures[0])
		default:
			err = errors.New("compileAndTest failed without a compile problem or test failure")
		}
		return
	}
	if result.NumTestsRun == 0 {
		err = errors.New("compileAndTest ran no tests")
		return
	}
	output = result.Log
	return
}

func (partner *ForcePartner) SoapExecuteCore(action, query string) (response []byte, err error) {
	url := fmt.Sprintf("%s/services/Soap/u/%s/%s", partner.Force.Credentials.InstanceUrl, partner.Force.Credentials.SessionOptions.ApiVersion, partner.Force.Credentials.UserInfo.OrgId)
	soap := NewSoap(url, "urn:partner.soap.sforce.com", partner.Force.Credentials.AccessToken)
	soap.Header = debugOnlyHeader
	response, err = soap.Execute(action, query)
	if err == SessionExpiredError {
		partner.Force.RefreshSessionOrExit()
		return partner.SoapExecuteCore(action, query)
	}
	return
}

func (partner *ForcePartner) soapExecute(action, query string) (response []byte, err error) {
	url := fmt.Sprintf("%s/services/Soap/s/%s/%s", partner.Force.Credentials.InstanceUrl, partner.Force.Credentials.SessionOptions.ApiVersion, partner.Force.Credentials.UserInfo.OrgId)
	soap := NewSoap(url, "http://soap.sforce.com/2006/08/apex", partner.Force.Credentials.AccessToken)
	soap.Header = debugOnlyHeader
	if partner.debuggingHeader != "" {
		soap.Header = partner.debuggingHeader
	}
	response, err = soap.Execute(action, query)
	if err == SessionExpiredError {
		partner.Force.RefreshSessionOrExit()
		return partner.soapExecute(action, query)
	}
	return
}

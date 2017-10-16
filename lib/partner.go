package lib

import (
	"encoding/xml"
	"errors"
	"fmt"
	"strings"
)

type ForcePartner struct {
	Force *Force
}

type TestRunner interface {
	RunTests(tests []string, namespace string) (output TestCoverage, err error)
}

type TestCoverage struct {
	Log                       string   `xml:"Header>DebuggingInfo>debugLog"`
	NumberRun                 int      `xml:"Body>runTestsResponse>result>numTestsRun"`
	NumberFailures            int      `xml:"Body>runTestsResponse>result>numFailures"`
	NumberLocations           []int    `xml:"Body>runTestsResponse>result>codeCoverage>numLocations"`
	NumberLocationsNotCovered []int    `xml:"Body>runTestsResponse>result>codeCoverage>numLocationsNotCovered"`
	Name                      []string `xml:"Body>runTestsResponse>result>codeCoverage>name"`
	SMethodNames              []string `xml:"Body>runTestsResponse>result>successes>methodName"`
	SClassNames               []string `xml:"Body>runTestsResponse>result>successes>name"`
	FMethodNames              []string `xml:"Body>runTestsResponse>result>failures>methodName"`
	FClassNames               []string `xml:"Body>runTestsResponse>result>failures>name"`
	FMessage                  []string `xml:"Body>runTestsResponse>result>failures>message"`
	FStackTrace               []string `xml:"Body>runTestsResponse>result>failures>stackTrace"`
}

func NewForcePartner(force *Force) (partner *ForcePartner) {
	partner = &ForcePartner{Force: force}
	return
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
	output = result.Log
	return
}

func (partner *ForcePartner) ExecuteAnonymousTest(apex string) (output string, err error) {
	soap := fmt.Sprintf(`<CompileAndTestRequest>
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
		<runTestsRequest><classes>AnonymousTestClass</classes></runTestsRequest>
		</CompileAndTestRequest>
		`, apex)
	body, err := partner.soapExecute("compileAndTest", soap)
	if err != nil {
		return
	}
	var result struct {
		Log             string   `xml:"Header>DebuggingInfo>debugLog"`
		Success         bool     `xml:"Body>compileAndTestResponse>result>success"`
		CompileProblems []string `xml:"Body>compileAndTestResponse>result>classes>problem"`
		TestFailures    []string `xml:"Body>compileAndTestResponse>result>runTestsResult>failures>message"`
	}
	if err = xml.Unmarshal(body, &result); err != nil {
		return
	}
	if !result.Success {
		if len(result.CompileProblems) > 0 {
			err = errors.New(result.CompileProblems[0])
		} else {
			err = errors.New(result.TestFailures[0])
		}
		return
	}
	output = result.Log
	return
}

func (partner *ForcePartner) SoapExecuteCore(action, query string) (response []byte, err error) {
	url := fmt.Sprintf("%s/services/Soap/u/%s/%s", partner.Force.Credentials.InstanceUrl, partner.Force.Credentials.SessionOptions.ApiVersion, partner.Force.Credentials.UserInfo.OrgId)
	soap := NewSoap(url, "urn:partner.soap.sforce.com", partner.Force.Credentials.AccessToken)
	soap.Header = "<apex:DebuggingHeader><apex:debugLevel>DEBUGONLY</apex:debugLevel></apex:DebuggingHeader>"
	response, err = soap.Execute(action, query)
	return
}

func (partner *ForcePartner) RunTests(tests []string, namespace string) (output TestCoverage, err error) {
	soap := "<RunTestsRequest>\n"
	if strings.EqualFold(tests[0], "all") {
		soap += "<allTests>True</allTests>\n"
	} else {
		for _, element := range tests {
			soap += "<classes>" + element + "</classes>\n"
		}
	}
	if namespace != "" {
		soap += "<namespace>" + namespace + "</namespace>\n"
	}
	soap += "</RunTestsRequest>"
	body, err := partner.soapExecute("runTests", soap)
	if err != nil {
		return
	}
	var result TestCoverage
	if err = xml.Unmarshal(body, &result); err != nil {
		return
	}
	output = result
	return
}

func (partner *ForcePartner) soapExecute(action, query string) (response []byte, err error) {
	url := fmt.Sprintf("%s/services/Soap/s/%s/%s", partner.Force.Credentials.InstanceUrl, partner.Force.Credentials.SessionOptions.ApiVersion, partner.Force.Credentials.UserInfo.OrgId)
	soap := NewSoap(url, "http://soap.sforce.com/2006/08/apex", partner.Force.Credentials.AccessToken)
	soap.Header = "<apex:DebuggingHeader><apex:debugLevel>DEBUGONLY</apex:debugLevel></apex:DebuggingHeader>"
	response, err = soap.Execute(action, query)
	if err == SessionExpiredError {
		partner.Force.RefreshSessionOrExit()
		return partner.soapExecute(action, query)
	}
	return
}

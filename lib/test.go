package lib

import (
	"encoding/xml"
	"fmt"
	"os"
	"strings"
	"time"

	"github.com/pkg/errors"

	"github.com/ForceCLI/force/lib/junit"
)

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

type TestNode struct {
	ClassName   string   `xml:"className"`
	TestMethods []string `xml:"testMethods"`
}

type RunTestsRequest struct {
	AllTests       bool       `xml:"allTests"`
	Classes        []string   `xml:"classes"`
	Namespace      string     `xml:"namespace"`
	MaxFailedTests int        `xml:"maxFailedTests"`
	Tests          []TestNode `xml:"tests,omitEmpty"`
}

func containsMethods(tests []string) (result bool, err error) {
	containsMethods := make(map[bool]int)
	classNames := make(map[string]int)
	for _, v := range tests {
		class, method := splitClassMethod(v)
		containsMethods[len(method) > 0]++
		classNames[class]++
	}
	if len(classNames) > 1 && (len(containsMethods) > 1 || containsMethods[true] > 0) {
		err = errors.New("Tests must all be either class names or methods within the same class")
		return
	}
	_, result = containsMethods[true]
	return
}

func splitClassMethod(s string) (string, string) {
	if len(s) == 0 {
		return s, s
	}
	slice := strings.SplitN(s, ".", 2)
	if len(slice) == 1 {
		return slice[0], ""
	}
	return slice[0], slice[1]
}

func NewRunTestsRequest(tests []string, namespace string) (request RunTestsRequest, err error) {
	request = RunTestsRequest{
		MaxFailedTests: -1,
		Namespace:      namespace,
	}
	if len(tests) == 0 || (len(tests) == 1 && strings.EqualFold(tests[0], "all")) {
		request.AllTests = true
		return
	}

	containsMethods, err := containsMethods(tests)
	if err != nil {
		return
	}
	if !containsMethods {
		request.Classes = tests
	} else {
		var methods []string
		var class string
		for _, v := range tests {
			var method string
			class, method = splitClassMethod(v)
			methods = append(methods, method)
		}
		// Per the docs, the list of TestNodes can only contain one element
		request.Tests = make([]TestNode, 1, 1)
		request.Tests[0] = TestNode{
			ClassName:   class,
			TestMethods: methods,
		}
	}
	return
}

func (partner *ForcePartner) RunTests(tests []string, namespace string) (output TestCoverage, err error) {
	request, err := NewRunTestsRequest(tests, namespace)
	if err != nil {
		return
	}
	soap, err := xml.MarshalIndent(request, "  ", "  ")
	if err != nil {
		return
	}
	body, err := partner.soapExecute("runTests", string(soap))
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

func (c TestCoverage) ToJunit() (string, error) {
	testSuite := junit.TestSuite{
		Name:      "apex",
		Timestamp: time.Now(),
	}
	for index := range c.SMethodNames {
		testSuite.TestCases = append(testSuite.TestCases, &junit.TestCase{
			Name:      c.SMethodNames[index],
			Classname: c.SClassNames[index],
		})
	}

	for index := range c.FMethodNames {
		method := junit.TestCase{
			Name:      c.FMethodNames[index],
			Classname: c.FClassNames[index],
		}
		method.Failures = append(method.Failures, &junit.Failure{
			Message: c.FMessage[index],
			Value:   c.FStackTrace[index],
		})
		testSuite.TestCases = append(testSuite.TestCases, &method)
	}
	testSuite.Update()
	output, err := xml.MarshalIndent(testSuite, "", "   ")
	if err != nil {
		return "", errors.Wrap(err, "Unable to format result for junit")
	}
	return string(output), nil
}

func (r ForceCheckDeploymentStatusResult) ToString(duration float64, verbose bool) string {
	c := r.Details
	problems := c.ComponentFailures
	successes := c.ComponentSuccesses
	testFailures := c.RunTestResult.TestFailures
	testSuccesses := c.RunTestResult.TestSuccesses
	var b strings.Builder

	fmt.Fprintf(&b, "\nSuccesses - %d\n", len(successes)-1)
	if verbose {
		for _, success := range successes {
			if success.FullName != "package.xml" {
				verb := "unchanged"
				if success.Changed {
					verb = "changed"
				} else if success.Deleted {
					verb = "deleted"
				} else if success.Created {
					verb = "created"
				}
				fmt.Fprintf(&b, "\t%s: %s\n", success.FullName, verb)
			}
		}
	}

	fmt.Fprintf(&b, "\nTest Successes - %d\n", len(testSuccesses))
	for _, success := range testSuccesses {
		fmt.Fprintf(&b, "  [PASS]  %s::%s\n", success.Name, success.MethodName)
	}

	fmt.Fprintf(&b, "\nFailures - %d\n", len(problems))
	for _, problem := range problems {
		if problem.FullName == "" {
			fmt.Println(problem.Problem)
		} else {
			fmt.Fprintf(&b, "\"%s\", line %d: %s %s\n", problem.FullName, problem.LineNumber, problem.ProblemType, problem.Problem)
		}
	}

	fmt.Fprintf(&b, "\nTest Failures - %d\n", len(testFailures))
	for _, failure := range testFailures {
		fmt.Fprintf(&b, "\n  [FAIL]  %s::%s: %s\n", failure.Name, failure.MethodName, failure.Message)
		fmt.Fprintln(&b, failure.StackTrace)
	}
	return b.String()
}

func (r ForceCheckDeploymentStatusResult) ToJunit(duration float64) (string, error) {
	c := r.Details
	hostname, _ := os.Hostname()
	testSuite := junit.TestSuite{
		Name:      "apex",
		Time:      duration,
		Hostname:  hostname,
		Timestamp: time.Now(),
	}
	problems := c.ComponentFailures
	testFailures := c.RunTestResult.TestFailures
	testSuccesses := c.RunTestResult.TestSuccesses
	for _, problem := range problems {
		if problem.FullName == "" {
			e := junit.TestCase{
				Name:      problem.Problem,
				Classname: "UNKNOWN",
			}
			e.Errors = append(e.Errors, &junit.Error{Message: problem.Problem})
			testSuite.TestCases = append(testSuite.TestCases, &e)
		} else {
			e := junit.TestCase{
				Name:      problem.FullName,
				Classname: problem.ComponentType,
			}
			e.Errors = append(e.Errors, &junit.Error{
				Type:    problem.ProblemType,
				Message: problem.Problem,
				Value:   fmt.Sprintf("Line: %d, Column: %d", problem.LineNumber, problem.ColumnNumber),
			})
			testSuite.TestCases = append(testSuite.TestCases, &e)
		}
	}

	for _, success := range testSuccesses {
		testSuite.TestCases = append(testSuite.TestCases, &junit.TestCase{
			Name:      success.MethodName,
			Classname: success.Name,
			Time:      float64(success.Time),
		})
	}

	for _, failure := range testFailures {
		e := junit.TestCase{
			Name:      failure.MethodName,
			Classname: failure.Name,
			Time:      float64(failure.Time),
		}
		e.Failures = append(e.Failures, &junit.Failure{
			Message: failure.Message,
			Value:   failure.StackTrace,
		})
		testSuite.TestCases = append(testSuite.TestCases, &e)
	}
	testSuite.Update()
	output, err := xml.MarshalIndent(testSuite, "", "   ")
	if err != nil {
		return "", errors.Wrap(err, "Unable to format result for junit")
	}
	return string(output), nil
}

func (r ForceCheckDeploymentStatusResult) HasComponentFailures() bool {
	return len(r.Details.ComponentFailures) > 0
}

func (r ForceCheckDeploymentStatusResult) HasTestFailures() bool {
	return len(r.Details.RunTestResult.TestFailures) > 0
}

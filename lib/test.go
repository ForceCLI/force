package lib

import (
	"encoding/xml"
	"errors"
	"strings"
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

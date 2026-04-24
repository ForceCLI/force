package lib

import (
	"encoding/json"
	"fmt"
	"net/url"
	"strings"
	"time"
)

// IntegrationTestRequest is the JSON body accepted by the Tooling API's
// /tooling/runTestsAsynchronous endpoint (specific-tests form). See
// https://developer.salesforce.com/docs/atlas.en-us.api_tooling.meta/api_tooling/tooling_api_run_tests_async.htm
type IntegrationTestRequest struct {
	Tests []IntegrationTestRequestNode `json:"tests"`
}

// IntegrationTestRequestNode describes a specific class and optional subset of methods.
type IntegrationTestRequestNode struct {
	ClassName   string   `json:"className"`
	TestMethods []string `json:"testMethods,omitempty"`
}

// toolingRunTestsAsyncResponse is the documented response shape: {"root": "707…"}.
// Some Salesforce and aer server implementations return a bare quoted string
// instead, so callers should accept both.
type toolingRunTestsAsyncResponse struct {
	Root string `json:"root"`
}

// IntegrationTestResult captures the outcome of an integration test run.
type IntegrationTestResult struct {
	AsyncApexJobID   string
	Status           string
	ClassesEnqueued  int
	ClassesCompleted int
	MethodsEnqueued  int
	MethodsCompleted int
	MethodsFailed    int
	StartTime        string
	EndTime          string
	TestTime         int
	Results          []IntegrationTestMethodResult
}

// IntegrationTestMethodResult is a single ApexTestResult row.
type IntegrationTestMethodResult struct {
	ClassName  string
	MethodName string
	Outcome    string
	Message    string
	StackTrace string
	RunTime    int
}

type toolingTestRunResultResponse struct {
	Records []struct {
		ID               string `json:"Id"`
		AsyncApexJobID   string `json:"AsyncApexJobId"`
		Status           string `json:"Status"`
		ClassesEnqueued  int    `json:"ClassesEnqueued"`
		ClassesCompleted int    `json:"ClassesCompleted"`
		MethodsEnqueued  int    `json:"MethodsEnqueued"`
		MethodsCompleted int    `json:"MethodsCompleted"`
		MethodsFailed    int    `json:"MethodsFailed"`
		StartTime        string `json:"StartTime"`
		EndTime          string `json:"EndTime"`
		TestTime         int    `json:"TestTime"`
	} `json:"records"`
}

type toolingTestResultResponse struct {
	Records []struct {
		ApexClass struct {
			Name string `json:"Name"`
		} `json:"ApexClass"`
		ApexClassID string `json:"ApexClassId"`
		MethodName  string `json:"MethodName"`
		Outcome     string `json:"Outcome"`
		Message     string `json:"Message"`
		StackTrace  string `json:"StackTrace"`
		RunTime     int    `json:"RunTime"`
	} `json:"records"`
}

// RunIntegrationTest submits the given class to the Tooling API's
// runTestsAsynchronous endpoint, polls for completion, and returns a
// populated IntegrationTestResult. Exactly one class must be specified
// because the Salesforce Tooling API only accepts one concurrent async
// integration test run at a time.
func (f *Force) RunIntegrationTest(className string, pollInterval time.Duration, timeout time.Duration) (IntegrationTestResult, error) {
	var result IntegrationTestResult
	className = strings.TrimSpace(className)
	if className == "" {
		return result, fmt.Errorf("RunIntegrationTest requires a class name")
	}

	class, method := splitClassMethod(className)
	node := IntegrationTestRequestNode{ClassName: class}
	if method != "" {
		node.TestMethods = []string{method}
	}
	req := IntegrationTestRequest{Tests: []IntegrationTestRequestNode{node}}

	body, err := json.Marshal(req)
	if err != nil {
		return result, fmt.Errorf("failed to marshal runTestsAsynchronous request: %w", err)
	}

	response, err := f.PostREST("tooling/runTestsAsynchronous", string(body))
	if err != nil {
		return result, fmt.Errorf("runTestsAsynchronous request failed: %w", err)
	}

	testRunID, err := parseRunTestsAsyncResponse(response)
	if err != nil {
		return result, err
	}
	if testRunID == "" {
		return result, fmt.Errorf("runTestsAsynchronous returned empty test run ID: %s", response)
	}

	if pollInterval <= 0 {
		pollInterval = 2 * time.Second
	}
	deadline := time.Now().Add(timeout)
	if timeout <= 0 {
		// Effectively no timeout; cap to a large duration so the loop terminates eventually.
		deadline = time.Now().Add(24 * time.Hour)
	}

	runResult, err := f.waitForIntegrationTestCompletion(testRunID, pollInterval, deadline)
	if err != nil {
		return result, err
	}

	methodResults, err := f.fetchIntegrationTestResults(runResult.AsyncApexJobID)
	if err != nil {
		return runResult, err
	}
	runResult.Results = methodResults
	return runResult, nil
}

func (f *Force) waitForIntegrationTestCompletion(testRunID string, pollInterval time.Duration, deadline time.Time) (IntegrationTestResult, error) {
	var result IntegrationTestResult
	query := fmt.Sprintf("SELECT Id, AsyncApexJobId, Status, ClassesEnqueued, ClassesCompleted, MethodsEnqueued, MethodsCompleted, MethodsFailed, StartTime, EndTime, TestTime FROM ApexTestRunResult WHERE AsyncApexJobId = '%s'", escapeSoqlLiteral(testRunID))
	queryUrl := fmt.Sprintf("%s/services/data/%s/tooling/query?q=%s", f.Credentials.InstanceUrl, apiVersion, url.QueryEscape(query))

	for {
		body, err := f.makeHttpRequestSync(NewRequest("GET").AbsoluteUrl(queryUrl))
		if err != nil {
			return result, fmt.Errorf("failed to query ApexTestRunResult: %w", err)
		}
		var resp toolingTestRunResultResponse
		if err := json.Unmarshal(body, &resp); err != nil {
			return result, fmt.Errorf("failed to decode ApexTestRunResult response: %w", err)
		}
		if len(resp.Records) > 0 {
			row := resp.Records[0]
			result = IntegrationTestResult{
				AsyncApexJobID:   row.AsyncApexJobID,
				Status:           row.Status,
				ClassesEnqueued:  row.ClassesEnqueued,
				ClassesCompleted: row.ClassesCompleted,
				MethodsEnqueued:  row.MethodsEnqueued,
				MethodsCompleted: row.MethodsCompleted,
				MethodsFailed:    row.MethodsFailed,
				StartTime:        row.StartTime,
				EndTime:          row.EndTime,
				TestTime:         row.TestTime,
			}
			if result.AsyncApexJobID == "" {
				result.AsyncApexJobID = testRunID
			}
			switch row.Status {
			case "Completed", "Failed", "Aborted":
				return result, nil
			}
		}
		if time.Now().After(deadline) {
			return result, fmt.Errorf("timed out waiting for integration test %s to complete", testRunID)
		}
		time.Sleep(pollInterval)
	}
}

func (f *Force) fetchIntegrationTestResults(asyncApexJobID string) ([]IntegrationTestMethodResult, error) {
	query := fmt.Sprintf("SELECT ApexClass.Name, ApexClassId, MethodName, Outcome, Message, StackTrace, RunTime FROM ApexTestResult WHERE AsyncApexJobId = '%s'", escapeSoqlLiteral(asyncApexJobID))
	queryUrl := fmt.Sprintf("%s/services/data/%s/tooling/query?q=%s", f.Credentials.InstanceUrl, apiVersion, url.QueryEscape(query))

	body, err := f.makeHttpRequestSync(NewRequest("GET").AbsoluteUrl(queryUrl))
	if err != nil {
		return nil, fmt.Errorf("failed to query ApexTestResult: %w", err)
	}
	var resp toolingTestResultResponse
	if err := json.Unmarshal(body, &resp); err != nil {
		return nil, fmt.Errorf("failed to decode ApexTestResult response: %w", err)
	}
	results := make([]IntegrationTestMethodResult, 0, len(resp.Records))
	for _, row := range resp.Records {
		results = append(results, IntegrationTestMethodResult{
			ClassName:  row.ApexClass.Name,
			MethodName: row.MethodName,
			Outcome:    row.Outcome,
			Message:    row.Message,
			StackTrace: row.StackTrace,
			RunTime:    row.RunTime,
		})
	}
	return results, nil
}

func escapeSoqlLiteral(s string) string {
	return strings.ReplaceAll(s, "'", "\\'")
}

// parseRunTestsAsyncResponse accepts either the documented object form
// {"root": "707…"} or a bare quoted string returned by some implementations.
func parseRunTestsAsyncResponse(response string) (string, error) {
	trimmed := strings.TrimSpace(response)
	if trimmed == "" {
		return "", fmt.Errorf("empty runTestsAsynchronous response")
	}
	if strings.HasPrefix(trimmed, "{") {
		var obj toolingRunTestsAsyncResponse
		if err := json.Unmarshal([]byte(trimmed), &obj); err != nil {
			return "", fmt.Errorf("failed to decode runTestsAsynchronous object response %q: %w", trimmed, err)
		}
		return obj.Root, nil
	}
	var id string
	if err := json.Unmarshal([]byte(trimmed), &id); err != nil {
		return "", fmt.Errorf("failed to decode runTestsAsynchronous string response %q: %w", trimmed, err)
	}
	return id, nil
}

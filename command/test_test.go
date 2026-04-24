package command_test

import (
	. "github.com/ForceCLI/force/command"
	. "github.com/ForceCLI/force/lib"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
)

type stubTestRunner struct {
}

func (testRunner stubTestRunner) RunTests(tests []string, namespace string) (output TestCoverage, err error) {
	if tests[0] == "NoSuchTest" {
		output = TestCoverage{NumberRun: 0, NumberFailures: 0}
	} else if tests[0] == "Success" {
		output = TestCoverage{NumberRun: 1, NumberFailures: 0}
	} else {
		output = TestCoverage{NumberRun: 1, NumberFailures: 1}
	}
	return
}

var _ = Describe("Test", func() {
	var (
		stub stubTestRunner
	)

	BeforeEach(func() {
		stub = stubTestRunner{}
	})

	Describe("RunTests", func() {
		It("should return an error if no tests can be run", func() {
			_, err := RunTests(stub, []string{"NoSuchTest"}, "")
			Expect(err).To(HaveOccurred())
		})
		It("should not return an error if tests pass", func() {
			_, err := RunTests(stub, []string{"Success"}, "")
			Expect(err).ToNot(HaveOccurred())
		})
		It("should not return an error if tests fail", func() {
			_, err := RunTests(stub, []string{"Fail"}, "")
			Expect(err).ToNot(HaveOccurred())
		})
	})

	Describe("QualifyMethods", func() {
		It("should prepend class to method names", func() {
			methods := QualifyMethods("MyClass", []string{"method1", "method2"})
			Expect(methods).To(Equal([]string{"MyClass.method1", "MyClass.method2"}))
		})
		It("should return class if no methods", func() {
			methods := QualifyMethods("MyClass", make([]string, 0, 0))
			Expect(methods).To(Equal([]string{"MyClass"}))
		})
	})

	Describe("GenerateResults", func() {
		var (
			results = TestCoverage{
				NumberRun:                 5,
				NumberFailures:            2,
				NumberLocations:           []int{1, 1, 1, 1, 1},
				NumberLocationsNotCovered: []int{0, 0, 1, 0, 1},
				Name:                      []string{"Test1", "Test2", "Test3", "Test4", "Test5"}}
		)

		It("should ignore test classes with 0% coverage", func() {
			output := GenerateResults(results)
			Expect(output).ToNot(MatchRegexp(`\D0%`))
		})
	})

	Describe("GenerateIntegrationTestResults", func() {
		It("should mark passing methods with [PASS] and include the run time", func() {
			result := IntegrationTestResult{
				AsyncApexJobID:   "707aer000000001AAA",
				Status:           "Completed",
				ClassesEnqueued:  1,
				ClassesCompleted: 1,
				MethodsEnqueued:  1,
				MethodsCompleted: 1,
				TestTime:         1234,
				Results: []IntegrationTestMethodResult{{
					ClassName:  "SampleIntegrationTest",
					MethodName: "happy_path",
					Outcome:    "Pass",
					RunTime:    42,
				}},
			}
			output := GenerateIntegrationTestResults(result)
			Expect(output).To(ContainSubstring("Integration Test Run: 707aer000000001AAA"))
			Expect(output).To(ContainSubstring("status: Completed"))
			Expect(output).To(ContainSubstring("1/1 completed"))
			Expect(output).To(ContainSubstring("TestTime: 1234ms"))
			Expect(output).To(ContainSubstring("[PASS]  SampleIntegrationTest::happy_path (42ms)"))
		})

		It("should render failures with message and stack trace", func() {
			result := IntegrationTestResult{
				AsyncApexJobID:   "707aer000000002AAA",
				Status:           "Completed",
				ClassesEnqueued:  1,
				ClassesCompleted: 1,
				MethodsEnqueued:  1,
				MethodsCompleted: 1,
				MethodsFailed:    1,
				Results: []IntegrationTestMethodResult{{
					ClassName:  "SampleIntegrationTest",
					MethodName: "broken_path",
					Outcome:    "Fail",
					Message:    "boom",
					StackTrace: "Class.SampleIntegrationTest.broken_path: line 1, column 1",
					RunTime:    10,
				}},
			}
			output := GenerateIntegrationTestResults(result)
			Expect(output).To(ContainSubstring("1 failed"))
			Expect(output).To(ContainSubstring("[FAIL]  SampleIntegrationTest::broken_path: boom"))
			Expect(output).To(ContainSubstring("Class.SampleIntegrationTest.broken_path: line 1, column 1"))
		})
	})
})

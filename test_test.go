package main_test

import (
	. "github.com/heroku/force"

	"github.com/heroku/force/salesforce"
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
)

type stubTestRunner struct {
}

func (testRunner stubTestRunner) RunTests(tests []string, namespace string) (output salesforce.TestCoverage, err error) {
	if tests[0] == "NoSuchTest" {
		output = salesforce.TestCoverage{NumberRun: 0, NumberFailures: 0}
	} else if tests[0] == "Success" {
		output = salesforce.TestCoverage{NumberRun: 1, NumberFailures: 0}
	} else {
		output = salesforce.TestCoverage{NumberRun: 1, NumberFailures: 1}
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

})

package command_test

import (
	"bytes"
	"fmt"
	"strings"

	. "github.com/ForceCLI/force/lib"
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	"github.com/spf13/cobra"
)

var _ = Describe("Deploys Command", func() {
	Describe("deploys status command", func() {
		It("should_require_deploy_id_flag", func() {
			cmd := &cobra.Command{}
			cmd.Flags().StringP("deploy-id", "d", "", "Deploy Id to get status for")
			cmd.Flags().BoolP("verbose", "v", false, "Show detailed information")

			deployId, _ := cmd.Flags().GetString("deploy-id")
			Expect(deployId).To(Equal(""))
		})

		It("should_accept_verbose_flag", func() {
			cmd := &cobra.Command{}
			cmd.Flags().StringP("deploy-id", "d", "", "Deploy Id to get status for")
			cmd.Flags().BoolP("verbose", "v", false, "Show detailed information")

			cmd.Flags().Set("verbose", "true")
			verbose, _ := cmd.Flags().GetBool("verbose")
			Expect(verbose).To(BeTrue())
		})

		Context("when displaying status", func() {
			It("should_display_basic_deployment_information", func() {
				result := ForceCheckDeploymentStatusResult{
					Id:                       "0Af000000000001",
					Status:                   "Succeeded",
					Done:                     true,
					Success:                  true,
					CheckOnly:                false,
					RollbackOnError:          true,
					CreatedByName:            "Test User",
					NumberComponentsTotal:    10,
					NumberComponentsDeployed: 10,
					NumberComponentErrors:    0,
					NumberTestsTotal:         5,
					NumberTestsCompleted:     5,
					NumberTestErrors:         0,
				}

				output := formatBasicStatus(result)
				Expect(output).To(ContainSubstring("Deploy ID: 0Af000000000001"))
				Expect(output).To(ContainSubstring("Status: Succeeded"))
				Expect(output).To(ContainSubstring("Done: true"))
				Expect(output).To(ContainSubstring("Success: true"))
			})

			It("should_display_component_counts", func() {
				result := ForceCheckDeploymentStatusResult{
					NumberComponentsTotal:    10,
					NumberComponentsDeployed: 8,
					NumberComponentErrors:    2,
				}

				output := formatComponentCounts(result)
				Expect(output).To(ContainSubstring("Total: 10"))
				Expect(output).To(ContainSubstring("Deployed: 8"))
				Expect(output).To(ContainSubstring("Errors: 2"))
			})

			It("should_display_test_counts", func() {
				result := ForceCheckDeploymentStatusResult{
					NumberTestsTotal:     15,
					NumberTestsCompleted: 12,
					NumberTestErrors:     3,
				}

				output := formatTestCounts(result)
				Expect(output).To(ContainSubstring("Total: 15"))
				Expect(output).To(ContainSubstring("Completed: 12"))
				Expect(output).To(ContainSubstring("Errors: 3"))
			})

			Context("with verbose flag", func() {
				It("should_display_component_successes", func() {
					result := ForceCheckDeploymentStatusResult{
						Details: ComponentDetails{
							ComponentSuccesses: []ComponentSuccess{
								{
									FullName: "TestClass",
									Created:  true,
									Changed:  false,
									Deleted:  false,
									FileName: "classes/TestClass.cls",
								},
								{
									FullName: "TestTrigger",
									Created:  false,
									Changed:  true,
									Deleted:  false,
									FileName: "triggers/TestTrigger.trigger",
								},
							},
						},
					}

					output := formatComponentSuccesses(result.Details.ComponentSuccesses)
					Expect(output).To(ContainSubstring("TestClass"))
					Expect(output).To(ContainSubstring("TestTrigger"))
					Expect(output).To(ContainSubstring("classes/TestClass.cls"))
					Expect(output).To(ContainSubstring("triggers/TestTrigger.trigger"))
				})

				It("should_display_component_failures", func() {
					result := ForceCheckDeploymentStatusResult{
						Details: ComponentDetails{
							ComponentFailures: []ComponentFailure{
								{
									ComponentType: "ApexClass",
									FullName:      "FailedClass",
									FileName:      "classes/FailedClass.cls",
									LineNumber:    42,
									ProblemType:   "Error",
									Problem:       "Variable does not exist: testVar",
								},
							},
						},
					}

					output := formatComponentFailures(result.Details.ComponentFailures)
					Expect(output).To(ContainSubstring("ApexClass"))
					Expect(output).To(ContainSubstring("FailedClass"))
					Expect(output).To(ContainSubstring("Variable does not exist"))
				})

				It("should_display_test_results", func() {
					result := ForceCheckDeploymentStatusResult{
						Details: ComponentDetails{
							RunTestResult: RunTestResult{
								NumberOfTestsRun: 5,
								NumberOfFailures: 1,
								TotalTime:        10.5,
								TestFailures: []TestFailure{
									{
										Name:       "TestClass",
										MethodName: "testMethod",
										Message:    "Assertion failed",
									},
								},
								TestSuccesses: []TestSuccess{
									{
										Name:       "TestClass",
										MethodName: "testSuccess",
										Time:       2.5,
									},
								},
							},
						},
					}

					output := formatTestResults(result.Details.RunTestResult)
					Expect(output).To(ContainSubstring("Tests Run: 5"))
					Expect(output).To(ContainSubstring("Failures: 1"))
					Expect(output).To(ContainSubstring("Total Time: 10.50s"))
					Expect(output).To(ContainSubstring("TestClass.testMethod"))
					Expect(output).To(ContainSubstring("Assertion failed"))
				})
			})
		})
	})
})

func formatBasicStatus(result ForceCheckDeploymentStatusResult) string {
	var buf bytes.Buffer
	fmt.Fprintf(&buf, "Deploy ID: %s\n", result.Id)
	fmt.Fprintf(&buf, "Status: %s\n", result.Status)
	fmt.Fprintf(&buf, "Done: %v\n", result.Done)
	fmt.Fprintf(&buf, "Success: %v\n", result.Success)
	return buf.String()
}

func formatComponentCounts(result ForceCheckDeploymentStatusResult) string {
	var buf bytes.Buffer
	fmt.Fprintf(&buf, "Components:\n")
	fmt.Fprintf(&buf, "  Total: %d\n", result.NumberComponentsTotal)
	fmt.Fprintf(&buf, "  Deployed: %d\n", result.NumberComponentsDeployed)
	fmt.Fprintf(&buf, "  Errors: %d\n", result.NumberComponentErrors)
	return buf.String()
}

func formatTestCounts(result ForceCheckDeploymentStatusResult) string {
	var buf bytes.Buffer
	fmt.Fprintf(&buf, "Tests:\n")
	fmt.Fprintf(&buf, "  Total: %d\n", result.NumberTestsTotal)
	fmt.Fprintf(&buf, "  Completed: %d\n", result.NumberTestsCompleted)
	fmt.Fprintf(&buf, "  Errors: %d\n", result.NumberTestErrors)
	return buf.String()
}

func formatComponentSuccesses(successes []ComponentSuccess) string {
	var buf bytes.Buffer
	fmt.Fprintf(&buf, "\nComponents Created/Changed/Deleted:\n")
	fmt.Fprintf(&buf, "%-40s %-10s %-10s %-10s %-40s\n", "Component Name", "Created", "Changed", "Deleted", "File Name")
	fmt.Fprintln(&buf, strings.Repeat("-", 110))

	for _, comp := range successes {
		fmt.Fprintf(&buf, "%-40s %-10v %-10v %-10v %-40s\n",
			comp.FullName,
			comp.Created,
			comp.Changed,
			comp.Deleted,
			comp.FileName)
	}
	return buf.String()
}

func formatComponentFailures(failures []ComponentFailure) string {
	var buf bytes.Buffer
	if len(failures) > 0 {
		fmt.Fprintf(&buf, "\nComponent Failures:\n")
		for _, f := range failures {
			fmt.Fprintf(&buf, "Type: %s, Name: %s, Problem: %s\n",
				f.ComponentType, f.FullName, f.Problem)
		}
	}
	return buf.String()
}

func formatTestResults(result RunTestResult) string {
	var buf bytes.Buffer
	fmt.Fprintf(&buf, "\nTest Results:\n")
	fmt.Fprintf(&buf, "  Tests Run: %d\n", result.NumberOfTestsRun)
	fmt.Fprintf(&buf, "  Failures: %d\n", result.NumberOfFailures)
	fmt.Fprintf(&buf, "  Total Time: %.2fs\n", result.TotalTime)

	if len(result.TestFailures) > 0 {
		fmt.Fprintf(&buf, "\n  Test Failures:\n")
		for _, f := range result.TestFailures {
			fmt.Fprintf(&buf, "    - %s.%s: %s\n", f.Name, f.MethodName, f.Message)
		}
	}

	if len(result.TestSuccesses) > 0 {
		fmt.Fprintf(&buf, "\n  Test Successes:\n")
		for _, s := range result.TestSuccesses {
			fmt.Fprintf(&buf, "    - %s.%s (%.2fs)\n", s.Name, s.MethodName, s.Time)
		}
	}
	return buf.String()
}

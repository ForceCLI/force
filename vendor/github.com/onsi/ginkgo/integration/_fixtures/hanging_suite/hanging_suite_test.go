package hanging_suite_test

import (
	"fmt"
	. "github.com/onsi/ginkgo"
	"time"
)

var _ = AfterSuite(func() {
	fmt.Println("Heading Out After Suite")
})

var _ = Describe("HangingSuite", func() {
	BeforeEach(func() {
		fmt.Fprintln(GinkgoWriter, "Just beginning")
	})

	Context("inner context", func() {
		BeforeEach(func() {
			fmt.Fprintln(GinkgoWriter, "Almost there...")
		})

		It("should hang out for a while", func() {
			fmt.Fprintln(GinkgoWriter, "Hanging Out")
			fmt.Println("Sleeping...")
			time.Sleep(time.Hour)
		})
	})
})

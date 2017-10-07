package command_test

import (
	. "github.com/heroku/force/command"
	"io/ioutil"
	"os"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
)

var _ = Describe("Bulk", func() {
	Describe("SplitCSV", func() {
		var (
			tempDir string
		)

		BeforeEach(func() {
			tempDir, _ = ioutil.TempDir("", "bulk-test")
		})

		AfterEach(func() {
			os.RemoveAll(tempDir)
		})

		It("should handle mulit-line field values", func() {
			csvFilePath := tempDir + "/bulk.csv"
			csvContents := `Id,Description
001000000000000000,single-line value
001000000000000001,single-line value
001000000000000002,"multi-line
value"`
			ioutil.WriteFile(csvFilePath, []byte(csvContents), 0644)

			batches := SplitCSV(csvFilePath, 2)

			Expect(len(batches)).To(Equal(2))
			Expect(batches[0]).To(HavePrefix("Id,Description"))
			Expect(batches[1]).To(HavePrefix("Id,Description"))
			Expect(batches[0]).To(HaveSuffix("single-line value\n"))
			Expect(batches[1]).To(HaveSuffix("multi-line\nvalue\"\n"))
		})

		It("should handle single-row files", func() {
			csvFilePath := tempDir + "/bulk.csv"
			csvContents := `Id,Description
001000000000000000,single value`
			ioutil.WriteFile(csvFilePath, []byte(csvContents), 0644)

			batches := SplitCSV(csvFilePath, 2)

			Expect(len(batches)).To(Equal(1))
			Expect(batches[0]).To(HavePrefix("Id,Description"))
			Expect(batches[0]).To(HaveSuffix("single value\n"))
		})
	})
})

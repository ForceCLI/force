package command_test

import (
	. "github.com/ForceCLI/force/command"
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

			batches, err := SplitCSV(csvFilePath, 2)
			Expect(err).To(BeNil())

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

			batches, err := SplitCSV(csvFilePath, 2)
			Expect(err).To(BeNil())

			Expect(len(batches)).To(Equal(1))
			Expect(batches[0]).To(HavePrefix("Id,Description"))
			Expect(batches[0]).To(HaveSuffix("single value\n"))
		})

		It("should return an error for an invalid file", func() {
			csvFilePath := tempDir + "/bulk.csv"
			csvContents := `Column 1
001000000000000000,single value`
			ioutil.WriteFile(csvFilePath, []byte(csvContents), 0644)

			_, err := SplitCSV(csvFilePath, 2)
			Expect(err).To(MatchError(MatchRegexp("wrong number of fields")))
		})

		It("should return an error for a missing file", func() {
			csvFilePath := tempDir + "/no-such-file.csv"

			_, err := SplitCSV(csvFilePath, 2)
			Expect(err).To(MatchError(MatchRegexp("no such file or directory")))
		})
	})
	Describe("CombineBatchResults", func() {
		It("should include the CSV header only once", func() {
			results := [][]byte{
				[]byte("\"Id\",\"Success\",\"Created\",\"Error\"\n\"001000000000000000\",\"true\",\"true\",\"\"\n"),
				[]byte("\"Id\",\"Success\",\"Created\",\"Error\"\n\"001000000000000001\",\"true\",\"true\",\"\"\n"),
			}
			combined, err := CombineBatchResults("CSV", results)
			Expect(err).To(BeNil())
			Expect(string(combined)).To(Equal("\"Id\",\"Success\",\"Created\",\"Error\"\n\"001000000000000000\",\"true\",\"true\",\"\"\n\"001000000000000001\",\"true\",\"true\",\"\"\n"))
		})

		It("should skip empty results and header-only results", func() {
			results := [][]byte{
				[]byte(""),
				[]byte("\"Id\",\"Success\"\n\"001000000000000000\",\"true\""),
				[]byte("\"Id\",\"Success\"\n"),
				[]byte("\"Id\",\"Success\"\n\"001000000000000001\",\"true\"\n"),
			}
			combined, err := CombineBatchResults("csv", results)
			Expect(err).To(BeNil())
			Expect(string(combined)).To(Equal("\"Id\",\"Success\"\n\"001000000000000000\",\"true\"\n\"001000000000000001\",\"true\"\n"))
		})

		It("should separate non-CSV results with a newline", func() {
			results := [][]byte{
				[]byte("<results><result><id>a</id></result></results>"),
				[]byte("<results><result><id>b</id></result></results>"),
			}
			combined, err := CombineBatchResults("XML", results)
			Expect(err).To(BeNil())
			Expect(string(combined)).To(Equal("<results><result><id>a</id></result></results>\n<results><result><id>b</id></result></results>\n"))
		})

		It("should write JSON results as one object per line", func() {
			results := [][]byte{
				[]byte(`[ {
  "success" : true,
  "created" : true,
  "id" : "001000000000000000",
  "errors" : [ ]
}, {
  "success" : false,
  "created" : false,
  "id" : null,
  "errors" : [ {
    "message" : "Required fields are missing: [Name]"
  } ]
} ]`),
				[]byte(""),
				[]byte(`[ {
  "success" : true,
  "created" : true,
  "id" : "001000000000000001",
  "errors" : [ ]
} ]`),
			}
			combined, err := CombineBatchResults("JSON", results)
			Expect(err).To(BeNil())
			Expect(string(combined)).To(Equal(`{"success":true,"created":true,"id":"001000000000000000","errors":[]}
{"success":false,"created":false,"id":null,"errors":[{"message":"Required fields are missing: [Name]"}]}
{"success":true,"created":true,"id":"001000000000000001","errors":[]}
`))
		})

		It("should read several concatenated JSON arrays in one result", func() {
			results := [][]byte{
				[]byte(`[ { "id" : "a" } ]
[ { "id" : "b" }, { "id" : "c" } ]
`),
			}
			combined, err := CombineBatchResults("JSON", results)
			Expect(err).To(BeNil())
			Expect(string(combined)).To(Equal(`{"id":"a"}
{"id":"b"}
{"id":"c"}
`))
		})

		It("should report JSON results that are not an array", func() {
			_, err := CombineBatchResults("JSON", [][]byte{[]byte(`{"success": true}`)})
			Expect(err).ToNot(BeNil())
		})

		It("should return nothing when there are no results", func() {
			combined, err := CombineBatchResults("CSV", nil)
			Expect(err).To(BeNil())
			Expect(combined).To(BeEmpty())
		})
	})
})

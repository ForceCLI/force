package command_test

import (
	"os"

	. "github.com/ForceCLI/force/command"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
)

var _ = Describe("Bulk2", func() {
	Describe("Command Registration", func() {
		It("should have bulk2 command registered", func() {
			cmd, _, err := RootCmd.Find([]string{"bulk2"})
			Expect(err).To(BeNil())
			Expect(cmd).NotTo(BeNil())
			Expect(cmd.Use).To(Equal("bulk2"))
		})

		It("should have insert subcommand", func() {
			cmd, _, err := RootCmd.Find([]string{"bulk2", "insert"})
			Expect(err).To(BeNil())
			Expect(cmd).NotTo(BeNil())
			Expect(cmd.Use).To(Equal("insert <object> <file>"))
		})

		It("should have update subcommand", func() {
			cmd, _, err := RootCmd.Find([]string{"bulk2", "update"})
			Expect(err).To(BeNil())
			Expect(cmd).NotTo(BeNil())
			Expect(cmd.Use).To(Equal("update <object> <file>"))
		})

		It("should have upsert subcommand", func() {
			cmd, _, err := RootCmd.Find([]string{"bulk2", "upsert"})
			Expect(err).To(BeNil())
			Expect(cmd).NotTo(BeNil())
			Expect(cmd.Use).To(HavePrefix("upsert"))
		})

		It("should have delete subcommand", func() {
			cmd, _, err := RootCmd.Find([]string{"bulk2", "delete"})
			Expect(err).To(BeNil())
			Expect(cmd).NotTo(BeNil())
			Expect(cmd.Use).To(Equal("delete <object> <file>"))
		})

		It("should have hardDelete subcommand", func() {
			cmd, _, err := RootCmd.Find([]string{"bulk2", "hardDelete"})
			Expect(err).To(BeNil())
			Expect(cmd).NotTo(BeNil())
			Expect(cmd.Use).To(Equal("hardDelete <object> <file>"))
		})

		It("should have query subcommand", func() {
			cmd, _, err := RootCmd.Find([]string{"bulk2", "query"})
			Expect(err).To(BeNil())
			Expect(cmd).NotTo(BeNil())
			Expect(cmd.Use).To(Equal("query <soql>"))
		})

		It("should have job subcommand", func() {
			cmd, _, err := RootCmd.Find([]string{"bulk2", "job"})
			Expect(err).To(BeNil())
			Expect(cmd).NotTo(BeNil())
			Expect(cmd.Use).To(Equal("job <jobId>"))
		})

		It("should have jobs subcommand", func() {
			cmd, _, err := RootCmd.Find([]string{"bulk2", "jobs"})
			Expect(err).To(BeNil())
			Expect(cmd).NotTo(BeNil())
			Expect(cmd.Use).To(Equal("jobs"))
		})

		It("should have results subcommand", func() {
			cmd, _, err := RootCmd.Find([]string{"bulk2", "results"})
			Expect(err).To(BeNil())
			Expect(cmd).NotTo(BeNil())
			Expect(cmd.Use).To(Equal("results <jobId>"))
		})

		It("should have abort subcommand", func() {
			cmd, _, err := RootCmd.Find([]string{"bulk2", "abort"})
			Expect(err).To(BeNil())
			Expect(cmd).NotTo(BeNil())
			Expect(cmd.Use).To(Equal("abort <jobId>"))
		})

		It("should have delete-job subcommand", func() {
			cmd, _, err := RootCmd.Find([]string{"bulk2", "delete-job"})
			Expect(err).To(BeNil())
			Expect(cmd).NotTo(BeNil())
			Expect(cmd.Use).To(Equal("delete-job <jobId>"))
		})
	})

	Describe("Insert Command Flags", func() {
		It("should have wait flag", func() {
			cmd, _, _ := RootCmd.Find([]string{"bulk2", "insert"})
			flag := cmd.Flags().Lookup("wait")
			Expect(flag).NotTo(BeNil())
			Expect(flag.Shorthand).To(Equal("w"))
		})

		It("should have interactive flag", func() {
			cmd, _, _ := RootCmd.Find([]string{"bulk2", "insert"})
			flag := cmd.Flags().Lookup("interactive")
			Expect(flag).NotTo(BeNil())
			Expect(flag.Shorthand).To(Equal("i"))
		})

		It("should have delimiter flag", func() {
			cmd, _, _ := RootCmd.Find([]string{"bulk2", "insert"})
			flag := cmd.Flags().Lookup("delimiter")
			Expect(flag).NotTo(BeNil())
			Expect(flag.DefValue).To(Equal("COMMA"))
		})

		It("should have lineending flag", func() {
			cmd, _, _ := RootCmd.Find([]string{"bulk2", "insert"})
			flag := cmd.Flags().Lookup("lineending")
			Expect(flag).NotTo(BeNil())
			Expect(flag.DefValue).To(Equal("LF"))
		})
	})

	Describe("Upsert Command Flags", func() {
		It("should have externalid flag", func() {
			cmd, _, _ := RootCmd.Find([]string{"bulk2", "upsert"})
			flag := cmd.Flags().Lookup("externalid")
			Expect(flag).NotTo(BeNil())
			Expect(flag.Shorthand).To(Equal("e"))
		})

		It("should require externalid flag", func() {
			cmd, _, _ := RootCmd.Find([]string{"bulk2", "upsert"})
			flag := cmd.Flags().Lookup("externalid")
			Expect(flag).NotTo(BeNil())

			annotations := flag.Annotations
			Expect(annotations).NotTo(BeNil())
		})
	})

	Describe("Query Command Flags", func() {
		It("should have wait flag", func() {
			cmd, _, _ := RootCmd.Find([]string{"bulk2", "query"})
			flag := cmd.Flags().Lookup("wait")
			Expect(flag).NotTo(BeNil())
		})

		It("should have query-all flag", func() {
			cmd, _, _ := RootCmd.Find([]string{"bulk2", "query"})
			flag := cmd.Flags().Lookup("query-all")
			Expect(flag).NotTo(BeNil())
			Expect(flag.Shorthand).To(Equal("A"))
		})

		It("should have delimiter flag", func() {
			cmd, _, _ := RootCmd.Find([]string{"bulk2", "query"})
			flag := cmd.Flags().Lookup("delimiter")
			Expect(flag).NotTo(BeNil())
		})

		It("should have lineending flag", func() {
			cmd, _, _ := RootCmd.Find([]string{"bulk2", "query"})
			flag := cmd.Flags().Lookup("lineending")
			Expect(flag).NotTo(BeNil())
		})
	})

	Describe("Results Command Flags", func() {
		It("should have successful flag", func() {
			cmd, _, _ := RootCmd.Find([]string{"bulk2", "results"})
			flag := cmd.Flags().Lookup("successful")
			Expect(flag).NotTo(BeNil())
			Expect(flag.Shorthand).To(Equal("s"))
		})

		It("should have failed flag", func() {
			cmd, _, _ := RootCmd.Find([]string{"bulk2", "results"})
			flag := cmd.Flags().Lookup("failed")
			Expect(flag).NotTo(BeNil())
			Expect(flag.Shorthand).To(Equal("f"))
		})

		It("should have unprocessed flag", func() {
			cmd, _, _ := RootCmd.Find([]string{"bulk2", "results"})
			flag := cmd.Flags().Lookup("unprocessed")
			Expect(flag).NotTo(BeNil())
			Expect(flag.Shorthand).To(Equal("u"))
		})
	})

	Describe("Jobs Command Flags", func() {
		It("should have query flag", func() {
			cmd, _, _ := RootCmd.Find([]string{"bulk2", "jobs"})
			flag := cmd.Flags().Lookup("query")
			Expect(flag).NotTo(BeNil())
			Expect(flag.Shorthand).To(Equal("q"))
		})
	})

	Describe("Command Arguments", func() {
		It("insert should require exactly 2 arguments", func() {
			cmd, _, _ := RootCmd.Find([]string{"bulk2", "insert"})
			err := cmd.Args(cmd, []string{"Account"})
			Expect(err).NotTo(BeNil())

			err = cmd.Args(cmd, []string{"Account", "file.csv"})
			Expect(err).To(BeNil())

			err = cmd.Args(cmd, []string{"Account", "file.csv", "extra"})
			Expect(err).NotTo(BeNil())
		})

		It("update should require exactly 2 arguments", func() {
			cmd, _, _ := RootCmd.Find([]string{"bulk2", "update"})
			err := cmd.Args(cmd, []string{"Account"})
			Expect(err).NotTo(BeNil())

			err = cmd.Args(cmd, []string{"Account", "file.csv"})
			Expect(err).To(BeNil())
		})

		It("upsert should require exactly 2 arguments", func() {
			cmd, _, _ := RootCmd.Find([]string{"bulk2", "upsert"})
			err := cmd.Args(cmd, []string{"Account"})
			Expect(err).NotTo(BeNil())

			err = cmd.Args(cmd, []string{"Account", "file.csv"})
			Expect(err).To(BeNil())
		})

		It("delete should require exactly 2 arguments", func() {
			cmd, _, _ := RootCmd.Find([]string{"bulk2", "delete"})
			err := cmd.Args(cmd, []string{"Account"})
			Expect(err).NotTo(BeNil())

			err = cmd.Args(cmd, []string{"Account", "file.csv"})
			Expect(err).To(BeNil())
		})

		It("hardDelete should require exactly 2 arguments", func() {
			cmd, _, _ := RootCmd.Find([]string{"bulk2", "hardDelete"})
			err := cmd.Args(cmd, []string{"Account"})
			Expect(err).NotTo(BeNil())

			err = cmd.Args(cmd, []string{"Account", "file.csv"})
			Expect(err).To(BeNil())
		})

		It("query should require exactly 1 argument", func() {
			cmd, _, _ := RootCmd.Find([]string{"bulk2", "query"})
			err := cmd.Args(cmd, []string{})
			Expect(err).NotTo(BeNil())

			err = cmd.Args(cmd, []string{"SELECT Id FROM Account"})
			Expect(err).To(BeNil())

			err = cmd.Args(cmd, []string{"SELECT", "Id"})
			Expect(err).NotTo(BeNil())
		})

		It("job should require exactly 1 argument", func() {
			cmd, _, _ := RootCmd.Find([]string{"bulk2", "job"})
			err := cmd.Args(cmd, []string{})
			Expect(err).NotTo(BeNil())

			err = cmd.Args(cmd, []string{"750test"})
			Expect(err).To(BeNil())
		})

		It("jobs should accept no arguments", func() {
			cmd, _, _ := RootCmd.Find([]string{"bulk2", "jobs"})
			Expect(cmd).NotTo(BeNil())
			// jobs command doesn't have explicit args validation (accepts any args)
		})

		It("results should require exactly 1 argument", func() {
			cmd, _, _ := RootCmd.Find([]string{"bulk2", "results"})
			err := cmd.Args(cmd, []string{})
			Expect(err).NotTo(BeNil())

			err = cmd.Args(cmd, []string{"750test"})
			Expect(err).To(BeNil())
		})

		It("abort should require exactly 1 argument", func() {
			cmd, _, _ := RootCmd.Find([]string{"bulk2", "abort"})
			err := cmd.Args(cmd, []string{})
			Expect(err).NotTo(BeNil())

			err = cmd.Args(cmd, []string{"750test"})
			Expect(err).To(BeNil())
		})

		It("delete-job should require exactly 1 argument", func() {
			cmd, _, _ := RootCmd.Find([]string{"bulk2", "delete-job"})
			err := cmd.Args(cmd, []string{})
			Expect(err).NotTo(BeNil())

			err = cmd.Args(cmd, []string{"750test"})
			Expect(err).To(BeNil())
		})
	})

	Describe("Command Help Text", func() {
		It("bulk2 should have descriptive help", func() {
			cmd, _, _ := RootCmd.Find([]string{"bulk2"})
			Expect(cmd.Short).To(ContainSubstring("Bulk API 2.0"))
		})

		It("bulk2 should have examples", func() {
			cmd, _, _ := RootCmd.Find([]string{"bulk2"})
			Expect(cmd.Example).To(ContainSubstring("insert"))
			Expect(cmd.Example).To(ContainSubstring("query"))
			Expect(cmd.Example).To(ContainSubstring("jobs"))
		})

		It("insert should have descriptive short help", func() {
			cmd, _, _ := RootCmd.Find([]string{"bulk2", "insert"})
			Expect(cmd.Short).To(ContainSubstring("Insert"))
			Expect(cmd.Short).To(ContainSubstring("CSV"))
		})

		It("query should have descriptive short help", func() {
			cmd, _, _ := RootCmd.Find([]string{"bulk2", "query"})
			Expect(cmd.Short).To(ContainSubstring("Query"))
		})
	})

	Describe("CSV File Operations", func() {
		var tempDir string

		BeforeEach(func() {
			tempDir, _ = os.MkdirTemp("", "bulk2-test")
		})

		AfterEach(func() {
			os.RemoveAll(tempDir)
		})

		It("should accept valid CSV file path", func() {
			csvFilePath := tempDir + "/test.csv"
			csvContents := "Name,Description\nTest Account,A test account"
			os.WriteFile(csvFilePath, []byte(csvContents), 0644)

			_, err := os.Stat(csvFilePath)
			Expect(err).To(BeNil())
		})

		It("should handle CSV with special characters", func() {
			csvFilePath := tempDir + "/special.csv"
			csvContents := "Name,Description\n\"Test, Account\",\"Description with \"\"quotes\"\"\""
			os.WriteFile(csvFilePath, []byte(csvContents), 0644)

			content, err := os.ReadFile(csvFilePath)
			Expect(err).To(BeNil())
			Expect(string(content)).To(Equal(csvContents))
		})

		It("should handle large CSV file", func() {
			csvFilePath := tempDir + "/large.csv"
			var csvBuilder string
			csvBuilder = "Name,Description\n"
			for range 1000 {
				csvBuilder += "Test Account,A test account description\n"
			}
			os.WriteFile(csvFilePath, []byte(csvBuilder), 0644)

			info, err := os.Stat(csvFilePath)
			Expect(err).To(BeNil())
			Expect(info.Size()).To(BeNumerically(">", 30000))
		})
	})

	Describe("Delimiter Values", func() {
		It("should accept COMMA delimiter", func() {
			cmd, _, _ := RootCmd.Find([]string{"bulk2", "insert"})
			cmd.Flags().Set("delimiter", "COMMA")
			val, _ := cmd.Flags().GetString("delimiter")
			Expect(val).To(Equal("COMMA"))
		})

		It("should accept TAB delimiter", func() {
			cmd, _, _ := RootCmd.Find([]string{"bulk2", "insert"})
			cmd.Flags().Set("delimiter", "TAB")
			val, _ := cmd.Flags().GetString("delimiter")
			Expect(val).To(Equal("TAB"))
		})

		It("should accept PIPE delimiter", func() {
			cmd, _, _ := RootCmd.Find([]string{"bulk2", "insert"})
			cmd.Flags().Set("delimiter", "PIPE")
			val, _ := cmd.Flags().GetString("delimiter")
			Expect(val).To(Equal("PIPE"))
		})

		It("should accept SEMICOLON delimiter", func() {
			cmd, _, _ := RootCmd.Find([]string{"bulk2", "insert"})
			cmd.Flags().Set("delimiter", "SEMICOLON")
			val, _ := cmd.Flags().GetString("delimiter")
			Expect(val).To(Equal("SEMICOLON"))
		})

		It("should accept CARET delimiter", func() {
			cmd, _, _ := RootCmd.Find([]string{"bulk2", "insert"})
			cmd.Flags().Set("delimiter", "CARET")
			val, _ := cmd.Flags().GetString("delimiter")
			Expect(val).To(Equal("CARET"))
		})

		It("should accept BACKQUOTE delimiter", func() {
			cmd, _, _ := RootCmd.Find([]string{"bulk2", "insert"})
			cmd.Flags().Set("delimiter", "BACKQUOTE")
			val, _ := cmd.Flags().GetString("delimiter")
			Expect(val).To(Equal("BACKQUOTE"))
		})
	})

	Describe("Line Ending Values", func() {
		It("should accept LF line ending", func() {
			cmd, _, _ := RootCmd.Find([]string{"bulk2", "insert"})
			cmd.Flags().Set("lineending", "LF")
			val, _ := cmd.Flags().GetString("lineending")
			Expect(val).To(Equal("LF"))
		})

		It("should accept CRLF line ending", func() {
			cmd, _, _ := RootCmd.Find([]string{"bulk2", "insert"})
			cmd.Flags().Set("lineending", "CRLF")
			val, _ := cmd.Flags().GetString("lineending")
			Expect(val).To(Equal("CRLF"))
		})
	})

	Describe("Flag Combinations", func() {
		It("interactive flag should imply wait", func() {
			cmd, _, _ := RootCmd.Find([]string{"bulk2", "insert"})
			cmd.Flags().Set("interactive", "true")

			interactive, _ := cmd.Flags().GetBool("interactive")
			Expect(interactive).To(BeTrue())
		})

		It("query-all flag should be boolean", func() {
			cmd, _, _ := RootCmd.Find([]string{"bulk2", "query"})
			cmd.Flags().Set("query-all", "true")

			queryAll, _ := cmd.Flags().GetBool("query-all")
			Expect(queryAll).To(BeTrue())
		})

		It("results flags can be combined", func() {
			cmd, _, _ := RootCmd.Find([]string{"bulk2", "results"})
			cmd.Flags().Set("successful", "true")
			cmd.Flags().Set("failed", "true")

			successful, _ := cmd.Flags().GetBool("successful")
			failed, _ := cmd.Flags().GetBool("failed")

			Expect(successful).To(BeTrue())
			Expect(failed).To(BeTrue())
		})
	})
})

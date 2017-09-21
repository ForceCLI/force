package command_test

import (
	. "github.com/heroku/force/command"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
)

var _ = Describe("Push", func() {
	Describe("FilenameMatchesMetadataName", func() {
		It("should match metadata files", func() {
			Expect(FilenameMatchesMetadataName("Account.object", "Account")).To(BeTrue())
			Expect(FilenameMatchesMetadataName("HelloWorld.cls", "HelloWorld")).To(BeTrue())
		})
		It("should not match filenames that don't match metadata name", func() {
			Expect(FilenameMatchesMetadataName("Account.object", "Contact")).To(BeFalse())
			Expect(FilenameMatchesMetadataName("HelloWorld.cls", "HelloWorld_Test")).To(BeFalse())
		})
		It("should match -meta.xml files", func() {
			Expect(FilenameMatchesMetadataName("HelloWorld.cls-meta.xml", "HelloWorld")).To(BeTrue())
		})
		It("should match Custom Metadata files", func() {
			Expect(FilenameMatchesMetadataName("My_Type.My_Object.md", "My_Type.My_Object")).To(BeTrue())
		})
	})

})

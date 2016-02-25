package main_test

import (
	. "github.com/heroku/force"
	"io/ioutil"
	"os"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
)

var _ = Describe("Packagebuilder", func() {
	Describe("NewPushBuilder", func() {
		It("should return a Packagebuilder", func() {
			pb := NewPushBuilder()
			Expect(pb).To(BeAssignableToTypeOf(PackageBuilder{IsPush: true}))
		})
	})

	Describe("AddFile", func() {
		var (
			pb      PackageBuilder
			tempDir string
		)

		BeforeEach(func() {
			pb = NewPushBuilder()
			tempDir, _ = ioutil.TempDir("", "packagebuilder-test")
		})

		AfterEach(func() {
			os.RemoveAll(tempDir)
		})

		Context("when adding a metadata file", func() {
			var apexClassPath string

			BeforeEach(func() {
				os.MkdirAll(tempDir+"/src/classes", 0755)
				apexClassPath = tempDir + "/src/classes/Test.cls"
				apexClassContents := "class Test {}"
				ioutil.WriteFile(apexClassPath, []byte(apexClassContents), 0644)
			})

			It("should add the file to package", func() {
				_, err := pb.AddFile(apexClassPath)
				Expect(err).ToNot(HaveOccurred())
				Expect(pb.Files).To(HaveKey("classes/Test.cls"))
			})
			It("should add the file to the package.xml", func() {
				pb.AddFile(apexClassPath)
				Expect(pb.Metadata).To(HaveKey("ApexClass"))
				Expect(pb.Metadata["ApexClass"].Members[0]).To(Equal("Test"))
			})
		})

		Context("when adding a meta.xml file", func() {
			var apexClassMetadataPath string

			BeforeEach(func() {
				os.MkdirAll(tempDir+"/src/classes", 0755)
				apexClassMetadataPath = tempDir + "/src/classes/Test.cls-meta.xml"
				apexClassMetadataContents := `<?xml version="1.0" encoding="UTF-8"?>`
				ioutil.WriteFile(apexClassMetadataPath, []byte(apexClassMetadataContents), 0644)
			})

			It("should add the file to package", func() {
				_, err := pb.AddFile(apexClassMetadataPath)
				Expect(err).ToNot(HaveOccurred())
				Expect(pb.Files).To(HaveKey("classes/Test.cls-meta.xml"))
			})
			It("should not add the file to the package.xml", func() {
				pb.AddFile(apexClassMetadataPath)
				Expect(pb.Metadata).ToNot(HaveKey("ApexClass"))
			})
		})

		Context("when adding a CustomMetadata file", func() {
			var customMetadataPath string

			BeforeEach(func() {
				os.MkdirAll(tempDir+"/src/customMetadata", 0755)
				customMetadataPath = tempDir + "/src/customMetadata/My_Type.My_Object.md"
				customMetadataContents := `<?xml version="1.0" encoding="UTF-8"?>`
				ioutil.WriteFile(customMetadataPath, []byte(customMetadataContents), 0644)
			})

			It("should add the file to package", func() {
				_, err := pb.AddFile(customMetadataPath)
				Expect(err).ToNot(HaveOccurred())
				Expect(pb.Files).To(HaveKey("customMetadata/My_Type.My_Object.md"))
			})
			It("should add the file to the package.xml", func() {
				pb.AddFile(customMetadataPath)
				Expect(pb.Metadata).To(HaveKey("CustomMetadata"))
				Expect(pb.Metadata["CustomMetadata"].Members[0]).To(Equal("My_Type.My_Object"))
			})
		})

		Context("when adding a non-existent file", func() {
			It("should not add the file to package", func() {
				_, err := pb.AddFile(tempDir + "/no/such/file")
				Expect(err).To(HaveOccurred())
				Expect(pb.Files).To(BeEmpty())
			})
			It("should not add the file to the package.xml", func() {
				pb.AddFile(tempDir + "/no/such/file")
				Expect(pb.Metadata).To(BeEmpty())
			})
		})

		Context("when adding a destructiveChanges file", func() {
			var destructiveChangesPath string

			BeforeEach(func() {
				pb = NewPushBuilder()
				tempDir, _ := ioutil.TempDir("", "packagebuilder-test")
				destructiveChangesPath = tempDir + "/destructiveChanges.xml"
				destructiveChangesXml := `<?xml version="1.0" encoding="UTF-8"?>
					<Package xmlns="http://soap.sforce.com/2006/04/metadata">
					<version>34.0</version>
					</Package>
				`
				ioutil.WriteFile(destructiveChangesPath, []byte(destructiveChangesXml), 0644)
			})

			It("should add the file to package", func() {
				_, err := pb.AddFile(destructiveChangesPath)
				Expect(err).ToNot(HaveOccurred())
				Expect(pb.Files).To(HaveKey("destructiveChanges.xml"))
			})
			It("should not add the file to the package.xml", func() {
				pb.AddFile(destructiveChangesPath)
				Expect(pb.Metadata).To(BeEmpty())
			})
		})
	})
})

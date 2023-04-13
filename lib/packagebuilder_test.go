package lib_test

import (
	"io/ioutil"
	"os"

	. "github.com/ForceCLI/force/lib"

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
			pb.Root = tempDir + "/src"
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
				err := pb.AddFile(apexClassPath)
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
				err := pb.AddFile(apexClassMetadataPath)
				Expect(err).ToNot(HaveOccurred())
				Expect(pb.Files).To(HaveKey("classes/Test.cls-meta.xml"))
			})
			It("should not add the file to the package.xml", func() {
				pb.AddFile(apexClassMetadataPath)
				Expect(pb.Metadata).ToNot(HaveKey("ApexClass"))
			})
		})

		Context("when adding both a metadata file and a meta.xml file", func() {
			var apexClassPath string
			var apexClassMetadataPath string

			BeforeEach(func() {
				os.MkdirAll(tempDir+"/src/classes", 0755)
				apexClassPath = tempDir + "/src/classes/Test.cls"
				apexClassContents := "class Test {}"
				ioutil.WriteFile(apexClassPath, []byte(apexClassContents), 0644)
				apexClassMetadataPath = tempDir + "/src/classes/Test.cls-meta.xml"
				apexClassMetadataContents := `<?xml version="1.0" encoding="UTF-8"?>`
				ioutil.WriteFile(apexClassMetadataPath, []byte(apexClassMetadataContents), 0644)
			})

			It("should add both files to package", func() {
				err := pb.AddFile(apexClassMetadataPath)
				Expect(err).ToNot(HaveOccurred())
				Expect(pb.Files).To(HaveKey("classes/Test.cls"))
				Expect(pb.Files).To(HaveKey("classes/Test.cls-meta.xml"))
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
				err := pb.AddFile(customMetadataPath)
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
				err := pb.AddFile(tempDir + "/no/such/file")
				Expect(err).To(HaveOccurred())
				Expect(pb.Files).To(BeEmpty())
			})
			It("should not add the file to the package.xml", func() {
				pb.AddFile(tempDir + "/no/such/file")
				Expect(pb.Metadata).To(BeEmpty())
			})
		})

		Context("when adding an LWC file", func() {
			var componentDir string

			BeforeEach(func() {
				componentDir = tempDir + "/src/lwc/mycomponent"
				mustMkdir(componentDir)
			})

			It("should add the file to the package and package.xml", func() {
				filePath := componentDir + "/mycomponent.js"
				mustWrite(filePath, `export default const x = 1;`)
				err := pb.AddFile(filePath)
				Expect(err).ToNot(HaveOccurred())
				Expect(pb.Files).To(HaveKey("lwc/mycomponent/mycomponent.js"))
				Expect(pb.Metadata).To(HaveKey("LightningComponentBundle"))
				Expect(pb.Metadata["LightningComponentBundle"].Members[0]).To(Equal("mycomponent"))
			})

			It("should not add test files to package or package.xml", func() {
				filePath := componentDir + "/mycomponent.test.js"
				mustWrite(filePath, `export default const x = 1;`)
				err := pb.AddFile(filePath)
				Expect(err).ToNot(HaveOccurred())
				Expect(pb.Files).To(BeEmpty())
				Expect(pb.Metadata).To(BeEmpty())
			})
		})

		Context("when adding a destructiveChanges file", func() {
			var tempDir string

			BeforeEach(func() {
				pb = NewPushBuilder()
				tempDir, _ = ioutil.TempDir("", "packagebuilder-test")
				pb.Root = tempDir + "/src"
				destructiveChangesPath := tempDir + "/src/destructiveChanges.xml"
				destructiveChangesXml := `<?xml version="1.0" encoding="UTF-8"?>
					<Package xmlns="http://soap.sforce.com/2006/04/metadata">
					<version>34.0</version>
					</Package>
				`
				mustMkdir(tempDir + "/src")
				mustWrite(destructiveChangesPath, destructiveChangesXml)
				mustWrite(tempDir+"/destructiveChanges.xml", destructiveChangesXml)
			})

			It("should add the file to package", func() {
				err := pb.AddFile(tempDir + "/src/destructiveChanges.xml")
				Expect(err).ToNot(HaveOccurred())
				Expect(pb.Files).To(HaveKey("destructiveChanges.xml"))
			})
			It("should not add the file to the package.xml", func() {
				pb.AddFile(tempDir + "/src/destructiveChanges.xml")
				Expect(pb.Metadata).To(BeEmpty())
			})
			It("should allow adding the file outside the root directory", func() {
				err := pb.AddFile(tempDir + "/destructiveChanges.xml")
				Expect(err).ToNot(HaveOccurred())
				Expect(pb.Files).To(HaveKey("destructiveChanges.xml"))
			})
		})
	})

	Describe("AddDirectory", func() {
		var pb PackageBuilder
		var tempDir string

		BeforeEach(func() {
			pb = NewPushBuilder()
			tempDir, _ = ioutil.TempDir("", "packagebuilder-test")
			pb.Root = tempDir + "/src"
		})

		AfterEach(func() {
			os.RemoveAll(tempDir)
		})

		Describe("adding a folder of lightning web components", func() {
			var lwcRoot string

			BeforeEach(func() {
				lwcRoot = tempDir + "/src/lwc/supercomponent"
				mustMkdir(lwcRoot)
			})

			It("should add directory contents", func() {
				mustWrite(lwcRoot+"/supercomponent.js", "export default const x = 1;")
				err := pb.AddDirectory(lwcRoot)
				Expect(err).ToNot(HaveOccurred())
				Expect(pb.Files).To(HaveKey("lwc/supercomponent/supercomponent.js"))
				Expect(pb.Metadata).To(HaveKey("LightningComponentBundle"))
				Expect(pb.Metadata["LightningComponentBundle"].Members[0]).To(Equal("supercomponent"))
			})

			It("should add components in subdirectories", func() {
				mustWrite(lwcRoot+"/supercomponent.js", "export default const x = 1;")
				err := pb.AddDirectory(tempDir + "/src/lwc")
				Expect(err).ToNot(HaveOccurred())
				Expect(pb.Files).To(HaveKey("lwc/supercomponent/supercomponent.js"))
				Expect(pb.Metadata).To(HaveKey("LightningComponentBundle"))
				Expect(pb.Metadata["LightningComponentBundle"].Members[0]).To(Equal("supercomponent"))
			})

			It("ignores test files and folders", func() {
				mustWrite(lwcRoot+"/supercomponent.js", "export default const x = 1;")
				mustWrite(lwcRoot+"/supercomponent.test.js", "")
				mustMkdir(lwcRoot + "/__tests__")
				mustWrite(lwcRoot+"/__tests__/supercomponent.test.js", "")

				err := pb.AddDirectory(lwcRoot)
				Expect(err).ToNot(HaveOccurred())
				Expect(pb.Files).To(HaveKey("lwc/supercomponent/supercomponent.js"))
				Expect(pb.Metadata).To(HaveKey("LightningComponentBundle"))
			})
		})
	})

	Describe("GetMetaForAbsolutePath", func() {
		var pb PackageBuilder

		BeforeEach(func() {
			pb = NewFetchBuilder()
			pb.Root = "/path/to/src"
		})

		Describe("adding a folder of lightning web components", func() {

			It("should handle LWC component directories", func() {
				metadataType, metadataName, err := pb.GetMetaForAbsolutePath("/path/to/src/lwc/supercomponent")
				Expect(err).ToNot(HaveOccurred())
				Expect(metadataType).To(Equal("LightningComponentBundle"))
				Expect(metadataName).To(Equal("supercomponent"))
			})

			It("should handle LWC component files", func() {
				metadataType, metadataName, err := pb.GetMetaForAbsolutePath("/path/to/src/lwc/supercomponent/component.js")
				Expect(err).ToNot(HaveOccurred())
				Expect(metadataType).To(Equal("LightningComponentBundle"))
				Expect(metadataName).To(Equal("supercomponent"))
			})

			It("should handle normal components", func() {
				metadataType, metadataName, err := pb.GetMetaForAbsolutePath("/path/to/src/classes/MyClass.cls")
				Expect(err).ToNot(HaveOccurred())
				Expect(metadataType).To(Equal("ApexClass"))
				Expect(metadataName).To(Equal("MyClass"))
			})

		})
	})
})

package lib_test

import (
	"io/ioutil"
	"os"

	. "github.com/ForceCLI/force/lib"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/ginkgo/extensions/table"
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

		Context("when adding a SlackApp file", func() {
			var slackAppPath string
			var slackAppMetaPath string

			BeforeEach(func() {
				os.MkdirAll(tempDir+"/src/slackapps", 0755)
				slackAppPath = tempDir + "/src/slackapps/ApexSlackApp.slackapp"
				ioutil.WriteFile(slackAppPath, []byte("description: example\n"), 0644)
				slackAppMetaPath = tempDir + "/src/slackapps/ApexSlackApp.slackapp-meta.xml"
				ioutil.WriteFile(slackAppMetaPath, []byte(`<?xml version="1.0" encoding="UTF-8"?>`), 0644)
			})

			It("should add the payload to the package.xml under SlackApp", func() {
				err := pb.AddFile(slackAppPath)
				Expect(err).ToNot(HaveOccurred())
				Expect(pb.Files).To(HaveKey("slackapps/ApexSlackApp.slackapp"))
				Expect(pb.Files).To(HaveKey("slackapps/ApexSlackApp.slackapp-meta.xml"))
				Expect(pb.Metadata).To(HaveKey("SlackApp"))
				Expect(pb.Metadata["SlackApp"].Members[0]).To(Equal("ApexSlackApp"))
			})

			It("should resolve the metadata type when adding the -meta.xml side", func() {
				err := pb.AddFile(slackAppMetaPath)
				Expect(err).ToNot(HaveOccurred())
				Expect(pb.Files).To(HaveKey("slackapps/ApexSlackApp.slackapp"))
				Expect(pb.Files).To(HaveKey("slackapps/ApexSlackApp.slackapp-meta.xml"))
				Expect(pb.Metadata).To(HaveKey("SlackApp"))
				Expect(pb.Metadata["SlackApp"].Members[0]).To(Equal("ApexSlackApp"))
			})
		})

		Context("when adding a ViewDefinition file", func() {
			var viewPath string
			var viewMetaPath string

			BeforeEach(func() {
				os.MkdirAll(tempDir+"/src/viewdefinitions", 0755)
				viewPath = tempDir + "/src/viewdefinitions/app_home.view"
				ioutil.WriteFile(viewPath, []byte("components: []\n"), 0644)
				viewMetaPath = tempDir + "/src/viewdefinitions/app_home.view-meta.xml"
				ioutil.WriteFile(viewMetaPath, []byte(`<?xml version="1.0" encoding="UTF-8"?>`), 0644)
			})

			It("should add the payload to the package.xml under ViewDefinition", func() {
				err := pb.AddFile(viewPath)
				Expect(err).ToNot(HaveOccurred())
				Expect(pb.Files).To(HaveKey("viewdefinitions/app_home.view"))
				Expect(pb.Files).To(HaveKey("viewdefinitions/app_home.view-meta.xml"))
				Expect(pb.Metadata).To(HaveKey("ViewDefinition"))
				Expect(pb.Metadata["ViewDefinition"].Members[0]).To(Equal("app_home"))
			})

			It("should resolve the metadata type when adding the -meta.xml side", func() {
				err := pb.AddFile(viewMetaPath)
				Expect(err).ToNot(HaveOccurred())
				Expect(pb.Files).To(HaveKey("viewdefinitions/app_home.view"))
				Expect(pb.Files).To(HaveKey("viewdefinitions/app_home.view-meta.xml"))
				Expect(pb.Metadata).To(HaveKey("ViewDefinition"))
				Expect(pb.Metadata["ViewDefinition"].Members[0]).To(Equal("app_home"))
			})
		})

		Context("when adding a UiFormatSpecificationSet file", func() {
			var uiFormatPath string

			BeforeEach(func() {
				os.MkdirAll(tempDir+"/src/uiFormatSpecificationSets", 0755)
				uiFormatPath = tempDir + "/src/uiFormatSpecificationSets/Access_Packages.uiFormatSpecificationSet"
				ioutil.WriteFile(uiFormatPath, []byte(`<?xml version="1.0" encoding="UTF-8"?>`), 0644)
			})

			It("should add the file to the package.xml under UiFormatSpecificationSet", func() {
				err := pb.AddFile(uiFormatPath)
				Expect(err).ToNot(HaveOccurred())
				Expect(pb.Files).To(HaveKey("uiFormatSpecificationSets/Access_Packages.uiFormatSpecificationSet"))
				Expect(pb.Metadata).To(HaveKey("UiFormatSpecificationSet"))
				Expect(pb.Metadata["UiFormatSpecificationSet"].Members[0]).To(Equal("Access_Packages"))
			})
		})

		Context("when adding a RecordAggregationDefinition file", func() {
			var definitionPath string

			BeforeEach(func() {
				os.MkdirAll(tempDir+"/src/RecordAggregationDefinitions", 0755)
				definitionPath = tempDir + "/src/RecordAggregationDefinitions/Donor_Gifts.RecordAggregationDefinition-meta.xml"
				ioutil.WriteFile(definitionPath, []byte(`<?xml version="1.0" encoding="UTF-8"?>`), 0644)
			})

			It("should add the file to the package.xml under RecordAggregationDefinition", func() {
				err := pb.AddFile(definitionPath)
				Expect(err).ToNot(HaveOccurred())
				Expect(pb.Metadata).To(HaveKey("RecordAggregationDefinition"))
				Expect(pb.Metadata["RecordAggregationDefinition"].Members[0]).To(Equal("Donor_Gifts"))
			})
		})

		Context("when adding a BrandingSet file", func() {
			var brandingSetPath string

			BeforeEach(func() {
				os.MkdirAll(tempDir+"/src/brandingSets", 0755)
				brandingSetPath = tempDir + "/src/brandingSets/My_Branding.brandingSet"
				ioutil.WriteFile(brandingSetPath, []byte(`<?xml version="1.0" encoding="UTF-8"?>`), 0644)
			})

			It("should add the file to the package.xml under BrandingSet", func() {
				err := pb.AddFile(brandingSetPath)
				Expect(err).ToNot(HaveOccurred())
				Expect(pb.Files).To(HaveKey("brandingSets/My_Branding.brandingSet"))
				Expect(pb.Metadata).To(HaveKey("BrandingSet"))
				Expect(pb.Metadata["BrandingSet"].Members[0]).To(Equal("My_Branding"))
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

		Describe("adding an experience bundle", func() {
			var experienceRoot string

			BeforeEach(func() {
				experienceRoot = tempDir + "/src/experiences/Catapult_Client_Portal1"
				mustMkdir(experienceRoot + "/routes")
				mustMkdir(experienceRoot + "/views")
				mustMkdir(experienceRoot + "/config")
				mustWrite(experienceRoot+"/routes/home.json", "{}")
				mustWrite(experienceRoot+"/views/home.json", "{}")
				mustWrite(experienceRoot+"/config/mainAppPage.json", "{}")
				// ExperienceBundle stores its metadata file as a sibling of the
				// bundle directory.
				mustWrite(tempDir+"/src/experiences/Catapult_Client_Portal1.site-meta.xml", "<Site/>")
			})

			It("uses the bundle name as the only package member", func() {
				err := pb.AddDirectory(experienceRoot)
				Expect(err).ToNot(HaveOccurred())
				Expect(pb.Metadata).To(HaveKey("ExperienceBundle"))
				Expect(pb.Metadata["ExperienceBundle"].Members).To(ConsistOf("Catapult_Client_Portal1"))
			})

			It("adds nested files with their full relative paths", func() {
				err := pb.AddDirectory(experienceRoot)
				Expect(err).ToNot(HaveOccurred())
				Expect(pb.Files).To(HaveKey("experiences/Catapult_Client_Portal1/routes/home.json"))
				Expect(pb.Files).To(HaveKey("experiences/Catapult_Client_Portal1/views/home.json"))
				Expect(pb.Files).To(HaveKey("experiences/Catapult_Client_Portal1/config/mainAppPage.json"))
			})

			It("includes the sibling metadata file when only the bundle directory is pushed", func() {
				err := pb.AddDirectory(experienceRoot)
				Expect(err).ToNot(HaveOccurred())
				Expect(pb.Files).To(HaveKey("experiences/Catapult_Client_Portal1.site-meta.xml"))
				Expect(pb.Metadata["ExperienceBundle"].Members).To(ConsistOf("Catapult_Client_Portal1"))
			})

			It("uses the bundle name when adding the experiences parent directory", func() {
				err := pb.AddDirectory(tempDir + "/src/experiences")
				Expect(err).ToNot(HaveOccurred())
				Expect(pb.Metadata).To(HaveKey("ExperienceBundle"))
				Expect(pb.Metadata["ExperienceBundle"].Members).To(ConsistOf("Catapult_Client_Portal1"))
				Expect(pb.Files).To(HaveKey("experiences/Catapult_Client_Portal1/routes/home.json"))
				Expect(pb.Files).To(HaveKey("experiences/Catapult_Client_Portal1.site-meta.xml"))
			})
		})
	})

	Describe("source format", func() {
		var pb PackageBuilder
		var tempDir string

		const objectXml = `<?xml version="1.0" encoding="UTF-8"?>
<CustomObject xmlns="http://soap.sforce.com/2006/04/metadata">
    <deploymentStatus>Deployed</deploymentStatus>
    <label>Widget</label>
    <nameField>
        <label>Name</label>
        <type>Text</type>
    </nameField>
    <pluralLabel>Widgets</pluralLabel>
    <sharingModel>ReadWrite</sharingModel>
</CustomObject>
`
		const fieldXml = `<?xml version="1.0" encoding="UTF-8"?>
<CustomField xmlns="http://soap.sforce.com/2006/04/metadata">
    <fullName>Score__c</fullName>
    <label>Score</label>
    <precision>18</precision>
    <scale>0</scale>
    <type>Number</type>
</CustomField>
`

		BeforeEach(func() {
			pb = NewPushBuilder()
			tempDir, _ = ioutil.TempDir("", "packagebuilder-test")
			pb.Root = tempDir + "/src"
			mustMkdir(tempDir + "/src/objects/Widget__c/fields")
			mustWrite(tempDir+"/src/objects/Widget__c/Widget__c.object-meta.xml", objectXml)
			mustWrite(tempDir+"/src/objects/Widget__c/fields/Score__c.field-meta.xml", fieldXml)
		})

		AfterEach(func() {
			os.RemoveAll(tempDir)
		})

		It("composes an object directory into one metadata-format object file", func() {
			err := pb.AddDirectory(tempDir + "/src/objects/Widget__c")
			Expect(err).ToNot(HaveOccurred())
			Expect(pb.Files).To(HaveLen(1))
			Expect(pb.Files).To(HaveKey("objects/Widget__c.object"))
			object := string(pb.Files["objects/Widget__c.object"])
			Expect(object).To(ContainSubstring("<label>Widget</label>"))
			Expect(object).To(ContainSubstring("<fullName>Score__c</fullName>"))
			Expect(pb.Metadata["CustomObject"].Members).To(ConsistOf("Widget__c"))
			Expect(pb.Metadata["CustomField"].Members).To(ConsistOf("Widget__c.Score__c"))
		})

		It("gives the same result when the files are added one at a time", func() {
			Expect(pb.AddFile(tempDir + "/src/objects/Widget__c/fields/Score__c.field-meta.xml")).To(Succeed())
			Expect(pb.AddFile(tempDir + "/src/objects/Widget__c/Widget__c.object-meta.xml")).To(Succeed())
			Expect(pb.Files).To(HaveLen(1))
			object := string(pb.Files["objects/Widget__c.object"])
			Expect(object).To(ContainSubstring("<label>Widget</label>"))
			Expect(object).To(ContainSubstring("<fullName>Score__c</fullName>"))
			Expect(pb.Metadata["CustomObject"].Members).To(ConsistOf("Widget__c"))
			Expect(pb.Metadata["CustomField"].Members).To(ConsistOf("Widget__c.Score__c"))
		})

		It("deploys a field without its object as a CustomField", func() {
			err := pb.AddFile(tempDir + "/src/objects/Widget__c/fields/Score__c.field-meta.xml")
			Expect(err).ToNot(HaveOccurred())
			Expect(pb.Files).To(HaveKey("objects/Widget__c.object"))
			object := string(pb.Files["objects/Widget__c.object"])
			Expect(object).To(ContainSubstring("<fullName>Score__c</fullName>"))
			Expect(object).ToNot(ContainSubstring("<label>Widget</label>"))
			Expect(pb.Metadata).ToNot(HaveKey("CustomObject"))
			Expect(pb.Metadata["CustomField"].Members).To(ConsistOf("Widget__c.Score__c"))
		})

		It("composes an object translation from its components", func() {
			translationDir := tempDir + "/src/objectTranslations/Widget__c-es"
			mustMkdir(translationDir + "/fields")
			mustWrite(translationDir+"/Widget__c-es.objectTranslation-meta.xml", `<?xml version="1.0" encoding="UTF-8"?>
<CustomObjectTranslation xmlns="http://soap.sforce.com/2006/04/metadata">
    <caseValues>
        <plural>false</plural>
        <value>Artilugio</value>
    </caseValues>
</CustomObjectTranslation>
`)
			mustWrite(translationDir+"/fields/Score__c.fieldTranslation-meta.xml", `<?xml version="1.0" encoding="UTF-8"?>
<CustomFieldTranslation xmlns="http://soap.sforce.com/2006/04/metadata">
    <label>Puntuación</label>
    <name>Score__c</name>
</CustomFieldTranslation>
`)
			err := pb.AddDirectory(translationDir)
			Expect(err).ToNot(HaveOccurred())
			Expect(pb.Files).To(HaveLen(1))
			Expect(pb.Files).To(HaveKey("objectTranslations/Widget__c-es.objectTranslation"))
			translation := string(pb.Files["objectTranslations/Widget__c-es.objectTranslation"])
			Expect(translation).To(ContainSubstring("<value>Artilugio</value>"))
			Expect(translation).To(ContainSubstring("<name>Score__c</name>"))
			Expect(pb.Metadata["CustomObjectTranslation"].Members).To(ConsistOf("Widget__c-es"))
		})

		It("drops the -meta.xml suffix from a custom metadata record", func() {
			mustMkdir(tempDir + "/src/customMetadata")
			record := `<CustomMetadata xmlns="http://soap.sforce.com/2006/04/metadata"><label>Primary</label></CustomMetadata>`
			mustWrite(tempDir+"/src/customMetadata/Widget_Setting.Primary.md-meta.xml", record)
			err := pb.AddFile(tempDir + "/src/customMetadata/Widget_Setting.Primary.md-meta.xml")
			Expect(err).ToNot(HaveOccurred())
			Expect(pb.Files).To(HaveKey("customMetadata/Widget_Setting.Primary.md"))
			Expect(string(pb.Files["customMetadata/Widget_Setting.Primary.md"])).To(Equal(record))
			Expect(pb.Metadata["CustomMetadata"].Members).To(ConsistOf("Widget_Setting.Primary"))
		})

		It("converts a report folder and a report in it", func() {
			mustMkdir(tempDir + "/src/reports/Sales")
			mustWrite(tempDir+"/src/reports/Sales.reportFolder-meta.xml", "<ReportFolder/>")
			mustWrite(tempDir+"/src/reports/Sales/Pipeline.report-meta.xml", "<Report/>")
			err := pb.AddDirectory(tempDir + "/src/reports")
			Expect(err).ToNot(HaveOccurred())
			Expect(pb.Files).To(HaveKey("reports/Sales-meta.xml"))
			Expect(pb.Files).To(HaveKey("reports/Sales/Pipeline.report"))
			Expect(pb.Metadata["Report"].Members).To(ConsistOf("Sales", "Sales/Pipeline"))
		})

		It("converts an email template folder", func() {
			mustMkdir(tempDir + "/src/email/Notices")
			mustWrite(tempDir+"/src/email/Notices.emailFolder-meta.xml", "<EmailFolder/>")
			err := pb.AddFile(tempDir + "/src/email/Notices.emailFolder-meta.xml")
			Expect(err).ToNot(HaveOccurred())
			Expect(pb.Files).To(HaveKey("email/Notices-meta.xml"))
			Expect(pb.Metadata["EmailTemplate"].Members).To(ConsistOf("Notices"))
		})

		It("leaves an Apex class, which has the same layout in both formats, unchanged", func() {
			mustMkdir(tempDir + "/src/classes")
			mustWrite(tempDir+"/src/classes/Widget.cls", "public class Widget {}")
			mustWrite(tempDir+"/src/classes/Widget.cls-meta.xml", "<ApexClass/>")
			err := pb.AddDirectory(tempDir + "/src/classes")
			Expect(err).ToNot(HaveOccurred())
			Expect(pb.Files).To(HaveKey("classes/Widget.cls"))
			Expect(pb.Files).To(HaveKey("classes/Widget.cls-meta.xml"))
			Expect(pb.Metadata["ApexClass"].Members).To(ConsistOf("Widget"))
		})
	})

	Describe("GetMetaForAbsolutePath", func() {
		var pb PackageBuilder

		BeforeEach(func() {
			pb = NewFetchBuilder()
			pb.Root = "/path/to/src"
		})

		Describe("external client app metadata", func() {
			DescribeTable("should identify each type from its directory",
				func(path, expectedType, expectedName string) {
					metadataType, metadataName, err := pb.GetMetaForAbsolutePath(path)
					Expect(err).ToNot(HaveOccurred())
					Expect(metadataType).To(Equal(expectedType))
					Expect(metadataName).To(Equal(expectedName))
				},
				Entry("app", "/path/to/src/externalClientApps/DBAmp.eca", "ExternalClientApplication", "DBAmp"),
				Entry("oauth policies", "/path/to/src/extlClntAppOauthPolicies/DBAmp_oauthPlcy.ecaOauthPlcy", "ExtlClntAppOauthConfigurablePolicies", "DBAmp_oauthPlcy"),
				Entry("oauth settings", "/path/to/src/extlClntAppOauthSettings/DBAmp_oauth.ecaOauth", "ExtlClntAppOauthSettings", "DBAmp_oauth"),
				Entry("global oauth settings", "/path/to/src/extlClntAppGlobalOauthSets/DBAmp_glblOauth.ecaGlblOauth", "ExtlClntAppGlobalOauthSettings", "DBAmp_glblOauth"),
				Entry("configurable policies", "/path/to/src/extlClntAppConfigurablePolicies/DBAmp_plcy.ecaCnfgPlcy", "ExtlClntAppConfigurablePolicies", "DBAmp_plcy"),
			)
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

			It("should handle ExperienceBundle directories", func() {
				metadataType, metadataName, err := pb.GetMetaForAbsolutePath("/path/to/src/experiences/Catapult_Client_Portal1")
				Expect(err).ToNot(HaveOccurred())
				Expect(metadataType).To(Equal("ExperienceBundle"))
				Expect(metadataName).To(Equal("Catapult_Client_Portal1"))
			})

			It("should handle ExperienceBundle nested files", func() {
				metadataType, metadataName, err := pb.GetMetaForAbsolutePath("/path/to/src/experiences/Catapult_Client_Portal1/routes/home.json")
				Expect(err).ToNot(HaveOccurred())
				Expect(metadataType).To(Equal("ExperienceBundle"))
				Expect(metadataName).To(Equal("Catapult_Client_Portal1"))
			})

			It("should handle normal components", func() {
				metadataType, metadataName, err := pb.GetMetaForAbsolutePath("/path/to/src/classes/MyClass.cls")
				Expect(err).ToNot(HaveOccurred())
				Expect(metadataType).To(Equal("ApexClass"))
				Expect(metadataName).To(Equal("MyClass"))
			})

			It("should handle ExternalServiceRegistration files", func() {
				metadataType, metadataName, err := pb.GetMetaForAbsolutePath("/path/to/src/externalServiceRegistrations/OpenLibrary.externalServiceRegistration")
				Expect(err).ToNot(HaveOccurred())
				Expect(metadataType).To(Equal("ExternalServiceRegistration"))
				Expect(metadataName).To(Equal("OpenLibrary"))
			})

			It("should handle SlackApp files", func() {
				metadataType, metadataName, err := pb.GetMetaForAbsolutePath("/path/to/src/slackapps/ApexSlackApp.slackapp")
				Expect(err).ToNot(HaveOccurred())
				Expect(metadataType).To(Equal("SlackApp"))
				Expect(metadataName).To(Equal("ApexSlackApp"))
			})

			It("should handle ViewDefinition files", func() {
				metadataType, metadataName, err := pb.GetMetaForAbsolutePath("/path/to/src/viewdefinitions/app_home.view")
				Expect(err).ToNot(HaveOccurred())
				Expect(metadataType).To(Equal("ViewDefinition"))
				Expect(metadataName).To(Equal("app_home"))
			})

			It("should handle UiFormatSpecificationSet files", func() {
				metadataType, metadataName, err := pb.GetMetaForAbsolutePath("/path/to/src/uiFormatSpecificationSets/Access_Packages.uiFormatSpecificationSet")
				Expect(err).ToNot(HaveOccurred())
				Expect(metadataType).To(Equal("UiFormatSpecificationSet"))
				Expect(metadataName).To(Equal("Access_Packages"))
			})

			It("should handle RecordAggregationDefinition files", func() {
				metadataType, metadataName, err := pb.GetMetaForAbsolutePath("/path/to/src/RecordAggregationDefinitions/Donor_Gifts.RecordAggregationDefinition")
				Expect(err).ToNot(HaveOccurred())
				Expect(metadataType).To(Equal("RecordAggregationDefinition"))
				Expect(metadataName).To(Equal("Donor_Gifts"))
			})

			It("should handle BrandingSet files", func() {
				metadataType, metadataName, err := pb.GetMetaForAbsolutePath("/path/to/src/brandingSets/My_Branding.brandingSet")
				Expect(err).ToNot(HaveOccurred())
				Expect(metadataType).To(Equal("BrandingSet"))
				Expect(metadataName).To(Equal("My_Branding"))
			})

		})
	})
})

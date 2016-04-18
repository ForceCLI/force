package project_test

import (
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"

	"github.com/heroku/force/project"
)

var _ = Describe("Project transforms", func() {
	Describe("TransformDeployToIncludeNewFlowVersionsOnly", func() {
		It("should filter out flows already active on the target", func() {
			sourceMetadata := map[string][]byte{
				"flowDefinitions/MyAwesomeFlow.flowDefinition": []byte(`
					<?xml version="1.0" encoding="UTF-8"?>
					<FlowDefinition xmlns="http://soap.sforce.com/2006/04/metadata">
    				<activeVersionNumber>2</activeVersionNumber>
					</FlowDefinition>`),
				"flows/MyAwesomeFlow-2.flow": []byte(""),
				"flows/MyAwesomeFlow-1.flow": []byte(""),
				"flowDefinitions/MyOkayFlow.flowDefinition": []byte(`
					<?xml version="1.0" encoding="UTF-8"?>
					<FlowDefinition xmlns="http://soap.sforce.com/2006/04/metadata">
					</FlowDefinition>`),
				"flows/MyOkayFlow-1.flow": []byte(""),
			}

			targetExistingMetadata := map[string][]byte{
				"flowDefinitions/MyAwesomeFlow.flowDefinition": []byte(`
					<?xml version="1.0" encoding="UTF-8"?>
					<FlowDefinition xmlns="http://soap.sforce.com/2006/04/metadata">
    				<activeVersionNumber>2</activeVersionNumber>
					</FlowDefinition>`),
				"flows/MyAwesomeFlow-2.flow": []byte(""),
				"flows/MyAwesomeFlow-1.flow": []byte(""),
			}

			transformedMetadata := project.TransformDeployToIncludeNewFlowVersionsOnly(
				sourceMetadata,
				targetExistingMetadata,
			)

			Ω(transformedMetadata).Should(HaveKey("flowDefinitions/MyAwesomeFlow.flowDefinition"))
			Ω(transformedMetadata).ShouldNot(HaveKey("flows/MyAwesomeFlow-2.flow"))
			Ω(transformedMetadata).Should(HaveKey("flows/MyAwesomeFlow-1.flow"))
		})
	})

	It("should deploy flows that aren't already active on the target", func() {
		sourceMetadata := map[string][]byte{
			"flowDefinitions/MyAwesomeFlow.flowDefinition": []byte(`
					<?xml version="1.0" encoding="UTF-8"?>
					<FlowDefinition xmlns="http://soap.sforce.com/2006/04/metadata">
    				<activeVersionNumber>2</activeVersionNumber>
					</FlowDefinition>`),
			"flows/MyAwesomeFlow-2.flow": []byte(""),
			"flows/MyAwesomeFlow-1.flow": []byte(""),
		}

		targetExistingMetadata := map[string][]byte{}

		transformedMetadata := project.TransformDeployToIncludeNewFlowVersionsOnly(
			sourceMetadata,
			targetExistingMetadata,
		)

		Ω(transformedMetadata).Should(HaveKey("flowDefinitions/MyAwesomeFlow.flowDefinition"))
		Ω(transformedMetadata).Should(HaveKey("flows/MyAwesomeFlow-2.flow"))
		Ω(transformedMetadata).Should(HaveKey("flows/MyAwesomeFlow-1.flow"))
	})

	It("should indicate the use of the flow versions transform when there are actually flows in the source metadata", func() {
		sourceMetadata := map[string][]byte{
			"flowDefinitions/MyAwesomeFlow.flowDefinition": []byte(`
					<?xml version="1.0" encoding="UTF-8"?>
					<FlowDefinition xmlns="http://soap.sforce.com/2006/04/metadata">
    				<activeVersionNumber>2</activeVersionNumber>
					</FlowDefinition>`),
			"flows/MyAwesomeFlow-2.flow": []byte(""),
			"flows/MyAwesomeFlow-1.flow": []byte(""),
		}

		Ω(project.IsNewFlowVersionsOnlyTransformRequired(sourceMetadata)).Should(BeTrue())
	})

	It("should not indicate the use of the flow versions transform when there are no flows in the source metadata", func() {
		sourceMetadata := map[string][]byte{
			"classes/MyBoringApexClass.cls": []byte(""),
		}

		Ω(project.IsNewFlowVersionsOnlyTransformRequired(sourceMetadata)).Should(BeFalse())
	})
})

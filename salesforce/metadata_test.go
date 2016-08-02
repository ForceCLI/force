package salesforce_test

import (
	"github.com/heroku/force/salesforce"
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
)

var _ = Describe("Metadata", func() {
	Describe("EnumerateMetadataByType", func() {
		It("should be able to find all apex classes", func() {
			files := map[string][]byte{
				"objects/Kase.object":       []byte(""),
				"classes/MyRoutines.cls":    []byte(""),
				"pages/Kase_List_View.page": []byte(""),
			}

			filtered := salesforce.EnumerateMetadataByType(files, "ApexClass", "classes", "cls", "")

			Ω(filtered.Name).Should(Equal("ApexClass"))

			Ω(filtered.Members).Should(ContainElement(salesforce.ForceMetadataItem{
				Name:         "MyRoutines",
				Content:      []byte(""),
				CompletePath: "classes/MyRoutines.cls",
			}))

			Ω(len(filtered.Members)).Should(Equal(1))
		})

		It("should be able to find all apex classes minus a regex", func() {
			files := map[string][]byte{
				"objects/Kase.object":    []byte(""),
				"classes/MyRoutines.cls": []byte(""),
				// this one willbe excluded by the regex:
				"classes/DS_Routines.cls":   []byte(""),
				"pages/Kase_List_View.page": []byte(""),
			}

			filtered := salesforce.EnumerateMetadataByType(files, "ApexClass", "classes", "cls", "^DS")

			Ω(filtered.Name).Should(Equal("ApexClass"))

			Ω(filtered.Members).Should(ContainElement(salesforce.ForceMetadataItem{
				Name:         "MyRoutines",
				Content:      []byte(""),
				CompletePath: "classes/MyRoutines.cls",
			}))

			Ω(len(filtered.Members)).Should(Equal(1))
		})
	})
})

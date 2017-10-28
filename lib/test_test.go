package lib_test

import (
	. "github.com/heroku/force/lib"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
)

var _ = Describe("Test", func() {
	Describe("NewRunTestsRequest", func() {
		It("should support individual test methods", func() {
			request, _ := NewRunTestsRequest([]string{"MyClass.method1"}, "")
			Expect(len(request.Tests)).To(Equal(1))
			Expect(request.Tests[0].ClassName).To(Equal("MyClass"))
			Expect(request.Tests[0].TestMethods[0]).To(Equal("method1"))
		})
		It("should support multiple test methods", func() {
			request, _ := NewRunTestsRequest([]string{"MyClass.method1", "MyClass.method2"}, "")
			Expect(len(request.Tests)).To(Equal(1))
			Expect(request.Tests[0].ClassName).To(Equal("MyClass"))
			Expect(request.Tests[0].TestMethods[0]).To(Equal("method1"))
			Expect(request.Tests[0].TestMethods[1]).To(Equal("method2"))
		})
		It("should support multiple classes", func() {
			request, _ := NewRunTestsRequest([]string{"MyClass", "MyOtherClass"}, "")
			Expect(len(request.Tests)).To(Equal(0))
			Expect(request.Classes[0]).To(Equal("MyClass"))
			Expect(request.Classes[1]).To(Equal("MyOtherClass"))
		})
		It("should fail if only some classes specify methods", func() {
			_, err := NewRunTestsRequest([]string{"MyClass", "MyOtherClass.method2"}, "")
			Expect(err).To(HaveOccurred())
		})
		It("should fail with multiple methods from different classes", func() {
			_, err := NewRunTestsRequest([]string{"MyClass.method1", "MyOtherClass.method2"}, "")
			Expect(err).To(HaveOccurred())
		})
	})
})

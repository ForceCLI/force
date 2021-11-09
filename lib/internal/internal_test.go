package internal_test

import (
	"github.com/ForceCLI/force/lib/internal"
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	"github.com/rgalanakis/golangal"
	"testing"
)

func TestInternal(t *testing.T) {
	RegisterFailHandler(Fail)
	RunSpecs(t, "Internal Suite")
}

var _ = Describe("JsonUnmarshal", func() {
	type T struct {
		A int    `json:"a" xml:"a"`
		B string `json:"b" xml:"b"`
	}
	It("unmarshals json", func() {
		var t T
		Expect(internal.JsonUnmarshal([]byte(`{"A": 1, "B": "2"}`), &t)).To(Succeed())
		Expect(t).To(golangal.MatchField("A", 1))
		Expect(t).To(golangal.MatchField("B", "2"))
	})
	It("uses a nice error message", func() {
		var t T
		Expect(internal.JsonUnmarshal([]byte(`<hi />`), &t)).To(MatchError("error unmarshaling json: invalid character '<' looking for beginning of value. first 10 characters: <hi />"))
		Expect(internal.JsonUnmarshal([]byte(`<this is very long content />`), &t)).To(MatchError("error unmarshaling json: invalid character '<' looking for beginning of value. first 10 characters: <this is v"))
	})
})

var _ = Describe("XmlUnmarshal", func() {
	type T struct {
		A int    `json:"a" xml:"a"`
		B string `json:"b" xml:"b"`
	}
	It("unmarshals xml", func() {
		var t T
		Expect(internal.XmlUnmarshal([]byte(`<Foo><a>1</a><b>2</b></Foo>`), &t)).To(Succeed())
		Expect(t).To(golangal.MatchField("A", 1))
		Expect(t).To(golangal.MatchField("B", "2"))
	})
	It("uses a nice error message", func() {
		var t T
		Expect(internal.XmlUnmarshal([]byte(`{"a": 1}`), &t)).To(MatchError(`error unmarshaling xml: EOF. first 10 characters: {"a": 1}`))
		Expect(internal.XmlUnmarshal([]byte(`{"actually this is long json": 1}`), &t)).To(MatchError(`error unmarshaling xml: EOF. first 10 characters: {"actually`))
	})
})

var _ = Describe("XmlMarshal", func() {
	It("marshals xml", func() {
		type T struct {
			A int    `json:"a" xml:"a"`
			B string `json:"b" xml:"b"`
		}
		t := T{A: 1, B: "2"}
		Expect(internal.XmlMarshal(t)).To(BeEquivalentTo(`<T><a>1</a><b>2</b></T>`))
	})
	It("uses a nice error message", func() {
		type T struct {
			B string   `json:"b" xml:"b"`
			C chan int `json:"c,omitempty" xml:"c,omitempty"`
		}
		t := T{C: make(chan int)}
		b, err := internal.XmlMarshal(t)
		Expect(b).To(BeNil())
		Expect(err).To(MatchError(HavePrefix("error marshaling xml: xml: unsupported type: chan int. object summary: T({ 0x")))

		t = T{C: make(chan int), B: "this is a long field"}
		b, err = internal.XmlMarshal(t)
		Expect(b).To(BeNil())
		Expect(err).To(MatchError("error marshaling xml: xml: unsupported type: chan int. object summary: T({this is a...)"))
	})
})

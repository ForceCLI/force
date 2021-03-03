package lib_test

import (
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	"io"
	"io/ioutil"
	"os"

	"testing"
)

func TestLib(t *testing.T) {
	RegisterFailHandler(Fail)
	RunSpecs(t, "Lib Suite")
}

func mustWrite(path, contents string) {
	Expect(ioutil.WriteFile(path, []byte(contents), 0644)).To(Succeed())
}

func mustMkdir(path string) {
	Expect(os.MkdirAll(path, 0755)).To(Succeed())
}

func mustRead(r io.Reader) []byte {
	b, err := ioutil.ReadAll(r)
	Expect(err).ToNot(HaveOccurred())
	return b
}

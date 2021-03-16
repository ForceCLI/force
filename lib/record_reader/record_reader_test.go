package record_reader_test

import (
	"encoding/csv"
	"errors"
	"github.com/ForceCLI/force/lib/record_reader"
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	"io"
	"strings"
	"testing"
)

func TestRecordReader(t *testing.T) {
	RegisterFailHandler(Fail)
	RunSpecs(t, "RecordReader Suite")
}

var _ = Describe("CsvRecordReader", func() {
	var validStream = `ColumnA,ColumnB,ColumnC
A1,B1,C1
A2,B2,C2
A3,B3,C3
`
	var invalidStream = validStream + "A4,\n"

	It("invokes the callback for all records in a valid stream", func() {
		r := record_reader.NewCsv(sreader(validStream), nil)
		recs, err := record_reader.ReadAll(r)
		Expect(err).ToNot(HaveOccurred())
		Expect(string(recs.Bytes)).To(BeEquivalentTo(validStream))
		Expect(recs.Count).To(BeEquivalentTo(4))
	})
	It("provides and error and discards last record of an invalid stream", func() {
		r := record_reader.NewCsv(sreader(invalidStream), nil)
		recs, err := record_reader.ReadAll(r)
		Expect(err).To(BeAssignableToTypeOf(&csv.ParseError{}))
		Expect(string(recs.Bytes)).To(BeEquivalentTo(validStream))
		Expect(recs.Count).To(BeEquivalentTo(4))
	})
	It("provides an error on a broken but complete stream", func() {
		r := record_reader.NewCsv(&brokenReader{sreader(validStream)}, nil)
		recs, err := record_reader.ReadAll(r)
		Expect(err).To(BeIdenticalTo(brokenReaderErr))
		Expect(string(recs.Bytes)).To(BeEquivalentTo(validStream))
		Expect(recs.Count).To(BeEquivalentTo(4))
	})
	It("provides an error and discards the last record on a broken, incomplete stream", func() {
		r := record_reader.NewCsv(&brokenReader{sreader(invalidStream)}, nil)
		recs, err := record_reader.ReadAll(r)
		Expect(err).To(BeIdenticalTo(brokenReaderErr))
		Expect(string(recs.Bytes)).To(BeEquivalentTo(validStream))
		Expect(recs.Count).To(BeEquivalentTo(4))
	})
})

var _ = Describe("JsonRecordReader", func() {
	var startOfStream = `[ {
  "attributes" : {
    "type" : "Account"
  },
  "Id" : "ID1",
  "OwnerId" : "ID2"
}, {
  "attributes" : {
    "type" : "Account"
  },
  "Id" : "ID3",
  "OwnerId" : "ID4"
}`
	var validStream = startOfStream + " ]"
	var invalidStream = startOfStream + `, {
  "attributes" : {
    "type" : "Account",`
	var expectedLines = `{  "attributes" : {    "type" : "Account"  },  "Id" : "ID1",  "OwnerId" : "ID2"}
{  "attributes" : {    "type" : "Account"  },  "Id" : "ID3",  "OwnerId" : "ID4"}
`

	It("invokes the callback for all records in a valid stream", func() {
		r := record_reader.NewJson(sreader(validStream), nil)
		recs, err := record_reader.ReadAll(r)
		Expect(err).ToNot(HaveOccurred())
		Expect(string(recs.Bytes)).To(BeEquivalentTo(expectedLines))
		Expect(recs.Count).To(BeEquivalentTo(2))
	})
	It("can work across multiple batches of records", func() {
		r := record_reader.NewJson(sreader(validStream), &record_reader.Options{GroupSize: 1})
		recs, err := record_reader.ReadAll(r)
		Expect(err).ToNot(HaveOccurred())
		Expect(string(recs.Bytes)).To(BeEquivalentTo(expectedLines))
		Expect(recs.Count).To(BeEquivalentTo(2))
	})
	It("detects error and discards last record of an invalid stream", func() {
		r := record_reader.NewJson(sreader(invalidStream), nil)
		recs, err := record_reader.ReadAll(r)
		Expect(err).To(BeIdenticalTo(record_reader.ErrIncompleteRecord))
		Expect(string(recs.Bytes)).To(BeEquivalentTo(expectedLines))
		Expect(recs.Count).To(BeEquivalentTo(2))
	})
	It("provides an error on a broken but complete stream", func() {
		r := record_reader.NewJson(&brokenReader{sreader(validStream)}, nil)
		recs, err := record_reader.ReadAll(r)
		Expect(err).To(BeIdenticalTo(brokenReaderErr))
		Expect(string(recs.Bytes)).To(BeEquivalentTo(expectedLines))
		Expect(recs.Count).To(BeEquivalentTo(2))
	})
	It("provides an error and discards the last record on a broken, incomplete stream", func() {
		r := record_reader.NewJson(&brokenReader{sreader(invalidStream)}, nil)
		recs, err := record_reader.ReadAll(r)
		Expect(err).To(BeIdenticalTo(brokenReaderErr))
		Expect(string(recs.Bytes)).To(BeEquivalentTo(expectedLines))
		Expect(recs.Count).To(BeEquivalentTo(2))
	})
})

func sreader(s string) io.Reader {
	return strings.NewReader(s)
}

var brokenReaderErr = errors.New("BrokenReader fake error")

type brokenReader struct {
	Buffer io.Reader
}

func (b *brokenReader) Read(p []byte) (int, error) {
	n, err := b.Buffer.Read(p)
	if err == io.EOF {
		return n, brokenReaderErr
	}
	return n, err
}

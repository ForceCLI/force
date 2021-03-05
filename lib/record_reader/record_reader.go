// Module record_reader supports incremental parsing io.Reader streams
// into complete CSV or JSON records, so that you can callers
// will never be left writing out invalid data (incomplete CSV row or JSON object).
//
// The logic for how this is done is very particular to the format,
// so refer to the code for explanations.

package record_reader

import (
	"errors"
	"io"
)

var ErrIncompleteRecord = errors.New("stream finished in the middle of a record")

const defaultGroupSize = 8

// RecordGroup is a complete chunk of records read from the reader.
// Its bytes can be safely written.
type RecordGroup struct {
	// The bytes of all the records in the group.
	Bytes []byte
	// The number of records present in the bytes.
	Count int
}

type RecordReader interface {
	// Next returns the bytes for the next chunk of records,
	// or an error. error will never be non-nil
	// if the group bytes are > 0.
	Next() (RecordGroup, error)
}

type Options struct {
	// The max number of records in a RecordGroup.
	// Higher numbers will use more memory.
	GroupSize int
}

func initOptions(op *Options) Options {
	if op == nil {
		op = &Options{}
	}
	o := *op
	if o.GroupSize <= 0 {
		o.GroupSize = defaultGroupSize
	}
	return o
}

// ReadAll returns a record group that has read every record
// from the given reader, stopping after the first error.
//
// Note that unlike RecordReader.Next, if an error occurs,
// the RecordGroup would have non-empty bytes
// (if there are any records in the reader stream).
func ReadAll(rr RecordReader) (RecordGroup, error) {
	var composite RecordGroup
	for {
		grp, err := rr.Next()
		if err == io.EOF {
			return composite, nil
		}
		if err != nil {
			return composite, err
		}
		composite.Count += grp.Count
		composite.Bytes = append(composite.Bytes, grp.Bytes...)
	}
}

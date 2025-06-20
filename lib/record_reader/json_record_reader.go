package record_reader

import (
	"bufio"
	"io"
)

const expectedJsonRecordSize = 2048

// NewJson returns a record reader that parses Apex-flavor JSON
// and converts it into bytes of JSONLines format in its records.
//
// Note this is not a generic JSON parser- we cannot parse the JSON
// both because it'd be slower, but especially because it's not really possible
// to parse JSON like `[{}, {}]` incrementally. So we read lines
// and figure out when we are between objects.
//
// Becuase of this, only Apex-flavor pretty JSON is supported. Like:
//
//	[ {
//	     "x": 1
//	}, {
//
//	     "x: 2
//	} ]
func NewJson(in io.Reader, opts *Options) RecordReader {
	bareOpts := initOptions(opts)
	return &jsonRecordReader{
		options:          bareOpts,
		scanner:          bufio.NewScanner(in),
		buf:              make([]byte, 0, expectedJsonRecordSize*bareOpts.GroupSize),
		pendingRecordBuf: make([]byte, 0, expectedJsonRecordSize),
	}
}

type jsonRecordReader struct {
	scanner          *bufio.Scanner
	options          Options
	buf              []byte
	recordsInBuf     int
	pendingRecordBuf []byte
	pendingError     error
}

func (j *jsonRecordReader) Next() (RecordGroup, error) {
	if j.pendingError != nil {
		return RecordGroup{}, j.pendingError
	}
	j.buf = j.buf[:0]
	j.recordsInBuf = 0

	hasFinishedPendingRecord := true
	for j.scanner.Scan() {
		line := j.scanner.Bytes()
		if len(line) == 0 {
			continue
		}
		firstChar := line[0]
		lastChar := line[len(line)-1]
		if firstChar == '[' && lastChar == '{' {
			// First char in file is going to be opening JSON list, skip it
			// ^[ {$
			j.pendingRecordBuf = append(j.pendingRecordBuf, line[2:]...)
			hasFinishedPendingRecord = false
		} else if firstChar == '}' && lastChar == ']' {
			// Final char in file is ending JSON list, skip it
			// ^} ]$
			j.pendingRecordBuf = append(j.pendingRecordBuf, line[:len(line)-2]...)
			hasFinishedPendingRecord = true
		} else if firstChar == '}' && lastChar == '{' {
			// Separator between records in the list
			// ^}, {$
			j.pendingRecordBuf = append(j.pendingRecordBuf, '}')
			hasFinishedPendingRecord = true
		} else {
			// All other lines are part of a normal record
			j.pendingRecordBuf = append(j.pendingRecordBuf, line...)
			hasFinishedPendingRecord = false
		}

		if hasFinishedPendingRecord {
			j.commitPending()
			if j.recordsInBuf == j.options.GroupSize {
				return j.createGroup(), nil
			}
		}
	}
	scanErr := j.scanner.Err()
	if scanErr != nil {
		// This will never be EOF
		j.pendingError = scanErr
	} else if !hasFinishedPendingRecord {
		// We may get no reader error, but not have completed out record
		j.pendingError = ErrIncompleteRecord
	}
	if j.recordsInBuf > 0 {
		return j.createGroup(), nil
	}
	if j.pendingError != nil {
		return RecordGroup{}, j.pendingError
	}
	// Scanner gave us no error, and we completed our records, so we can assume we're at EOF.
	return RecordGroup{}, io.EOF
}

func (j *jsonRecordReader) commitPending() {
	j.recordsInBuf++
	j.buf = append(j.buf, j.pendingRecordBuf...)
	j.buf = append(j.buf, '\n')
	// And reset the pending records buffer
	j.pendingRecordBuf = j.pendingRecordBuf[:1]
	j.pendingRecordBuf[0] = '{'
}

func (j *jsonRecordReader) createGroup() RecordGroup {
	bufcop := make([]byte, len(j.buf))
	copy(bufcop, j.buf)
	return RecordGroup{Bytes: bufcop, Count: j.recordsInBuf}
}

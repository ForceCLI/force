package record_reader

import (
	"bytes"
	"encoding/csv"
	"io"
)

// NewCsv returns a record reader that reports each CSV row as a record.
// Note that CSV rows are not just delimited via '\n'-
// there is in fact no way to know whether a character is inline a record
// without having parsed the file (consider that newlines can be within cells,
// and quotes are escaped via doubling-up, so even something like `","`
// may refer to the string `","` within a cell, or `"",""` in the CSV).
func NewCsv(in io.Reader, opts *Options) RecordReader {
	bareOpts := initOptions(opts)
	csvInputReader := csv.NewReader(in)
	// We write the row to bytes as soon as we get it
	csvInputReader.ReuseRecord = true
	buf := bytes.NewBuffer(nil)
	return &csvRecordReader{
		options:        bareOpts,
		csvInputReader: csvInputReader,
		buf:            buf,
		csvBuf:         csv.NewWriter(buf),
	}
}

type csvRecordReader struct {
	csvInputReader *csv.Reader
	options        Options
	buf            *bytes.Buffer
	csvBuf         *csv.Writer
	rowsInBuf      int
	pendingError   error
}

func (c *csvRecordReader) Next() (RecordGroup, error) {
	// We don't want to return any records plus an error,
	// so if an error occurs during reading, we need to store it
	// and return it when we don't have any records to report.
	if c.pendingError != nil {
		return RecordGroup{}, c.pendingError
	}

	for {
		csvRow, err := c.csvInputReader.Read()
		if err != nil {
			// err can be EOF here, which is fine, we'll store for later
			c.pendingError = err
			// If the underlying reader was broken, and returned partial results,
			// the CSV reader will error during the parse.
			// This is because the bufio buffer used by the CSV reader internally
			// will return the read bytes from Read(), and then return the error.
			//
			// We don't really want the parsing error due to the partial row returned;
			// we want the underyling error from the broken reader.
			// To get that error, we need to call .Read() *again*.
			// This will cause CSV reader to make another call to the bufio buffer,
			// which will now return the underlying reader error.
			//
			// If there is no underlying error, and the parse error was due to actual malformed CSV,
			// we'll end up reporting that.
			if _, isParserError := err.(*csv.ParseError); isParserError {
				if _, nextCsvError := c.csvInputReader.Read(); nextCsvError != nil && nextCsvError != io.EOF {
					c.pendingError = nextCsvError
				}
			}
			if c.rowsInBuf > 0 {
				// Next call will return the error
				return c.commitAndResetBuffer(), nil
			}
			return RecordGroup{}, c.pendingError
		}
		if err := c.addPendingRow(csvRow); err != nil {
			return RecordGroup{}, err
		}
		if c.rowsInBuf == c.options.GroupSize {
			return c.commitAndResetBuffer(), nil
		}
	}
}

func (c *csvRecordReader) addPendingRow(row []string) error {
	c.rowsInBuf++
	return c.csvBuf.Write(row)
}

func (c *csvRecordReader) commitAndResetBuffer() RecordGroup {
	c.csvBuf.Flush()
	grp := RecordGroup{
		Count: c.rowsInBuf,
		Bytes: c.buf.Bytes(),
	}
	c.rowsInBuf = 0
	c.buf.Reset()
	return grp
}

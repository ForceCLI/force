package lib

import (
	"bufio"
	"bytes"
	"encoding/base64"
	"fmt"
	"io"
)

// extractElementText copies the XML document from r into the returned rest
// buffer, except for the text content of the first <element> element, which is
// streamed to w instead. The element's tags are kept in rest so it still
// parses as XML. If the element is not present, the entire document ends up in
// rest. The element's content must not contain child elements. This allows
// very large payloads, like the base64-encoded zip in a checkRetrieveStatus
// response, to be processed without holding them in memory.
func extractElementText(r io.Reader, element string, w io.Writer) (rest []byte, err error) {
	openSuffix := []byte(element + ">")
	closeSuffix := []byte("/" + element + ">")
	br := bufio.NewReaderSize(r, 64*1024)
	var restBuf bytes.Buffer

	found := false
	for !found {
		chunk, rerr := br.ReadSlice('<')
		restBuf.Write(chunk)
		switch rerr {
		case nil:
		case bufio.ErrBufferFull:
			continue
		case io.EOF:
			return restBuf.Bytes(), nil
		default:
			return restBuf.Bytes(), rerr
		}
		peeked, perr := br.Peek(len(openSuffix))
		if perr == io.EOF {
			restBuf.Write(peeked)
			br.Discard(len(peeked))
			return restBuf.Bytes(), nil
		}
		if perr != nil {
			return restBuf.Bytes(), perr
		}
		if bytes.Equal(peeked, openSuffix) {
			br.Discard(len(openSuffix))
			restBuf.Write(openSuffix)
			found = true
		}
	}

	for {
		chunk, rerr := br.ReadSlice('<')
		if rerr == bufio.ErrBufferFull {
			if _, werr := w.Write(chunk); werr != nil {
				return restBuf.Bytes(), werr
			}
			continue
		}
		if rerr != nil {
			return restBuf.Bytes(), fmt.Errorf("unterminated <%s> element: %w", element, rerr)
		}
		if _, werr := w.Write(chunk[:len(chunk)-1]); werr != nil {
			return restBuf.Bytes(), werr
		}
		peeked, perr := br.Peek(len(closeSuffix))
		if perr != nil || !bytes.Equal(peeked, closeSuffix) {
			return restBuf.Bytes(), fmt.Errorf("unexpected markup inside <%s> element", element)
		}
		br.Discard(len(closeSuffix))
		restBuf.WriteByte('<')
		restBuf.Write(closeSuffix)
		break
	}

	if _, cerr := io.Copy(&restBuf, br); cerr != nil {
		return restBuf.Bytes(), cerr
	}
	return restBuf.Bytes(), nil
}

// base64Writer decodes base64 text written to it and streams the decoded
// bytes to the underlying writer, ignoring whitespace. Close must be called
// to verify the payload was complete.
type base64Writer struct {
	w       io.Writer
	enc     []byte
	scratch []byte
}

func newBase64Writer(w io.Writer) *base64Writer {
	return &base64Writer{
		w:       w,
		enc:     make([]byte, 0, 64*1024),
		scratch: make([]byte, base64.StdEncoding.DecodedLen(64*1024)),
	}
}

func (b *base64Writer) Write(p []byte) (int, error) {
	for i := 0; i < len(p); {
		for i < len(p) && len(b.enc) < cap(b.enc) {
			switch c := p[i]; c {
			case '\r', '\n', '\t', ' ':
			default:
				b.enc = append(b.enc, c)
			}
			i++
		}
		if err := b.flush(); err != nil {
			return i, err
		}
	}
	return len(p), nil
}

func (b *base64Writer) flush() error {
	n := len(b.enc) / 4 * 4
	if n == 0 {
		return nil
	}
	nd, err := base64.StdEncoding.Decode(b.scratch, b.enc[:n])
	if err != nil {
		return err
	}
	if _, err := b.w.Write(b.scratch[:nd]); err != nil {
		return err
	}
	rem := copy(b.enc, b.enc[n:])
	b.enc = b.enc[:rem]
	return nil
}

func (b *base64Writer) Close() error {
	if err := b.flush(); err != nil {
		return err
	}
	if len(b.enc) != 0 {
		return fmt.Errorf("truncated base64 payload")
	}
	return nil
}

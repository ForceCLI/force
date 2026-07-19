package lib

import (
	"archive/zip"
	"bytes"
	"encoding/base64"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"testing/iotest"
)

func TestExtractElementText(t *testing.T) {
	doc := `<Envelope><Body><result><zipFile>SGVsbG8=</zipFile><messages><problem>warn</problem></messages></result></Body></Envelope>`
	var content bytes.Buffer
	rest, err := extractElementText(strings.NewReader(doc), "zipFile", &content)
	if err != nil {
		t.Fatal(err)
	}
	if content.String() != "SGVsbG8=" {
		t.Errorf("content = %q, want %q", content.String(), "SGVsbG8=")
	}
	want := `<Envelope><Body><result><zipFile></zipFile><messages><problem>warn</problem></messages></result></Body></Envelope>`
	if string(rest) != want {
		t.Errorf("rest = %q, want %q", rest, want)
	}
}

func TestExtractElementText_OneByteReads(t *testing.T) {
	doc := `<a><zipFile>QUJD</zipFile><b>x</b></a>`
	var content bytes.Buffer
	rest, err := extractElementText(iotest.OneByteReader(strings.NewReader(doc)), "zipFile", &content)
	if err != nil {
		t.Fatal(err)
	}
	if content.String() != "QUJD" {
		t.Errorf("content = %q, want %q", content.String(), "QUJD")
	}
	if string(rest) != `<a><zipFile></zipFile><b>x</b></a>` {
		t.Errorf("rest = %q", rest)
	}
}

func TestExtractElementText_LargeContent(t *testing.T) {
	// Content larger than the internal buffer to exercise the
	// ErrBufferFull path
	payload := strings.Repeat("QUJDRA==", 32*1024)[:256*1024]
	doc := "<a><zipFile>" + payload + "</zipFile></a>"
	var content bytes.Buffer
	rest, err := extractElementText(strings.NewReader(doc), "zipFile", &content)
	if err != nil {
		t.Fatal(err)
	}
	if content.String() != payload {
		t.Errorf("content mismatch: got %d bytes, want %d", content.Len(), len(payload))
	}
	if string(rest) != `<a><zipFile></zipFile></a>` {
		t.Errorf("rest = %q", rest)
	}
}

func TestExtractElementText_ElementMissing(t *testing.T) {
	doc := `<Envelope><Body><Fault><faultcode>sf:INVALID_SESSION_ID</faultcode></Fault></Body></Envelope>`
	var content bytes.Buffer
	rest, err := extractElementText(strings.NewReader(doc), "zipFile", &content)
	if err != nil {
		t.Fatal(err)
	}
	if content.Len() != 0 {
		t.Errorf("content should be empty, got %q", content.String())
	}
	if string(rest) != doc {
		t.Errorf("rest = %q, want original document", rest)
	}
}

func TestExtractElementText_SimilarElementNames(t *testing.T) {
	doc := `<a><zipFileName>ignored</zipFileName><zipFile>QQ==</zipFile></a>`
	var content bytes.Buffer
	rest, err := extractElementText(strings.NewReader(doc), "zipFile", &content)
	if err != nil {
		t.Fatal(err)
	}
	if content.String() != "QQ==" {
		t.Errorf("content = %q, want %q", content.String(), "QQ==")
	}
	if string(rest) != `<a><zipFileName>ignored</zipFileName><zipFile></zipFile></a>` {
		t.Errorf("rest = %q", rest)
	}
}

func TestExtractElementText_EmptyElement(t *testing.T) {
	doc := `<a><zipFile></zipFile></a>`
	var content bytes.Buffer
	rest, err := extractElementText(strings.NewReader(doc), "zipFile", &content)
	if err != nil {
		t.Fatal(err)
	}
	if content.Len() != 0 {
		t.Errorf("content should be empty, got %q", content.String())
	}
	if string(rest) != doc {
		t.Errorf("rest = %q", rest)
	}
}

func TestExtractElementText_Unterminated(t *testing.T) {
	doc := `<a><zipFile>QUJD`
	var content bytes.Buffer
	_, err := extractElementText(strings.NewReader(doc), "zipFile", &content)
	if err == nil {
		t.Fatal("expected error for unterminated element")
	}
}

func TestBase64Writer(t *testing.T) {
	original := bytes.Repeat([]byte("The quick brown fox. "), 20000)
	encoded := base64.StdEncoding.EncodeToString(original)

	var decoded bytes.Buffer
	w := newBase64Writer(&decoded)
	// Write in awkward chunk sizes to exercise quad-boundary buffering
	for len(encoded) > 0 {
		n := 7
		if n > len(encoded) {
			n = len(encoded)
		}
		if _, err := w.Write([]byte(encoded[:n])); err != nil {
			t.Fatal(err)
		}
		encoded = encoded[n:]
	}
	if err := w.Close(); err != nil {
		t.Fatal(err)
	}
	if !bytes.Equal(decoded.Bytes(), original) {
		t.Errorf("decoded %d bytes, want %d; content mismatch", decoded.Len(), len(original))
	}
}

func TestBase64Writer_Whitespace(t *testing.T) {
	var decoded bytes.Buffer
	w := newBase64Writer(&decoded)
	if _, err := w.Write([]byte("SGVs\r\n bG8s \tIFdvcmxk\n")); err != nil {
		t.Fatal(err)
	}
	if err := w.Close(); err != nil {
		t.Fatal(err)
	}
	if decoded.String() != "Hello, World" {
		t.Errorf("decoded = %q, want %q", decoded.String(), "Hello, World")
	}
}

func TestBase64Writer_Truncated(t *testing.T) {
	var decoded bytes.Buffer
	w := newBase64Writer(&decoded)
	if _, err := w.Write([]byte("SGVsbG")); err != nil {
		t.Fatal(err)
	}
	if err := w.Close(); err == nil {
		t.Fatal("expected error for truncated payload")
	}
}

func TestExtractZipToDir(t *testing.T) {
	dir := t.TempDir()
	zipPath := filepath.Join(dir, "test.zip")
	zf, err := os.Create(zipPath)
	if err != nil {
		t.Fatal(err)
	}
	zw := zip.NewWriter(zf)
	entries := map[string]string{
		"unpackaged/classes/Foo.cls": "public class Foo {}",
		"unpackaged/package.xml":     "<Package/>",
	}
	for name, content := range entries {
		f, err := zw.Create(name)
		if err != nil {
			t.Fatal(err)
		}
		if _, err := f.Write([]byte(content)); err != nil {
			t.Fatal(err)
		}
	}
	if err := zw.Close(); err != nil {
		t.Fatal(err)
	}
	if err := zf.Close(); err != nil {
		t.Fatal(err)
	}

	root := filepath.Join(dir, "out")
	if err := extractZipToDir(zipPath, root, "unpackaged/"); err != nil {
		t.Fatal(err)
	}
	for name, content := range entries {
		rel := strings.TrimPrefix(name, "unpackaged/")
		data, err := os.ReadFile(filepath.Join(root, filepath.FromSlash(rel)))
		if err != nil {
			t.Fatal(err)
		}
		if string(data) != content {
			t.Errorf("%s = %q, want %q", rel, data, content)
		}
	}
}

func TestExtractZipToDir_PathTraversal(t *testing.T) {
	dir := t.TempDir()
	zipPath := filepath.Join(dir, "evil.zip")
	zf, err := os.Create(zipPath)
	if err != nil {
		t.Fatal(err)
	}
	zw := zip.NewWriter(zf)
	f, err := zw.Create("../evil.txt")
	if err != nil {
		t.Fatal(err)
	}
	if _, err := f.Write([]byte("bad")); err != nil {
		t.Fatal(err)
	}
	if err := zw.Close(); err != nil {
		t.Fatal(err)
	}
	if err := zf.Close(); err != nil {
		t.Fatal(err)
	}

	root := filepath.Join(dir, "out")
	if err := extractZipToDir(zipPath, root, "unpackaged/"); err == nil {
		t.Fatal("expected error for path traversal entry")
	}
}

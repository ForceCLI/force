package lib

import (
	"archive/zip"
	"bytes"
	"encoding/base64"
	"encoding/xml"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// mockRetrieveServer simulates the Metadata API retrieve flow, rejecting
// retrieve calls whose file count, after expanding wildcard members, exceeds
// limit — mirroring the server-side LIMIT_EXCEEDED behavior.
type mockRetrieveServer struct {
	t            *testing.T
	limit        int
	expansions   map[string]int      // files a type's "*" member expands to
	packageFiles map[string][]string // entries in a named package
	problem      string              // problem message included in each result
	fault        string              // respond to retrieve calls with this SOAP fault
	retrieves    int                 // retrieve calls received
	jobs         map[string]retrieveJob
}

type retrieveRequest struct {
	Types []struct {
		Name    string   `xml:"name"`
		Members []string `xml:"members"`
	} `xml:"types"`
}

type retrieveJob struct {
	request     retrieveRequest
	packageName string
}

func (m *mockRetrieveServer) fileCount(job retrieveJob) int {
	if job.packageName != "" {
		return len(m.packageFiles[job.packageName])
	}
	count := 0
	for _, t := range job.request.Types {
		for _, member := range t.Members {
			if member == "*" {
				count += m.expansions[t.Name]
			} else {
				count++
			}
		}
	}
	return count
}

func (m *mockRetrieveServer) resultZip(job retrieveJob) []byte {
	var buf bytes.Buffer
	zw := zip.NewWriter(&buf)
	pkg := Package{Xmlns: "http://soap.sforce.com/2006/04/metadata", Version: "62.0"}
	if job.packageName != "" {
		mt := MetaType{Name: "Members"}
		for _, entry := range m.packageFiles[job.packageName] {
			mt.Members = append(mt.Members, entry)
			f, err := zw.Create("unpackaged/" + entry)
			if err != nil {
				m.t.Fatal(err)
			}
			fmt.Fprintf(f, "content of %s", entry)
		}
		pkg.Types = append(pkg.Types, mt)
	}
	for _, t := range job.request.Types {
		mt := MetaType{Name: t.Name}
		for _, member := range t.Members {
			if member == "*" {
				for i := range m.expansions[t.Name] {
					name := fmt.Sprintf("wild%d", i)
					mt.Members = append(mt.Members, name)
					m.addZipEntry(zw, t.Name, name)
				}
			} else {
				mt.Members = append(mt.Members, member)
				m.addZipEntry(zw, t.Name, member)
			}
		}
		pkg.Types = append(pkg.Types, mt)
	}
	pkgXml, err := xml.Marshal(pkg)
	if err != nil {
		m.t.Fatal(err)
	}
	f, err := zw.Create("unpackaged/package.xml")
	if err != nil {
		m.t.Fatal(err)
	}
	f.Write(append([]byte(xml.Header), pkgXml...))
	if err := zw.Close(); err != nil {
		m.t.Fatal(err)
	}
	return buf.Bytes()
}

func (m *mockRetrieveServer) addZipEntry(zw *zip.Writer, typeName, member string) {
	f, err := zw.Create(fmt.Sprintf("unpackaged/%s/%s", typeName, member))
	if err != nil {
		m.t.Fatal(err)
	}
	fmt.Fprintf(f, "content of %s/%s", typeName, member)
}

func (m *mockRetrieveServer) handler(w http.ResponseWriter, r *http.Request) {
	body, err := io.ReadAll(r.Body)
	if err != nil {
		m.t.Fatal(err)
	}
	envelope := `<?xml version="1.0" encoding="UTF-8"?>
<soapenv:Envelope xmlns:soapenv="http://schemas.xmlsoap.org/soap/envelope/" xmlns="http://soap.sforce.com/2006/04/metadata"><soapenv:Body>%s</soapenv:Body></soapenv:Envelope>`
	switch {
	case bytes.Contains(body, []byte("<retrieve ")):
		if m.fault != "" {
			fmt.Fprintf(w, envelope, fmt.Sprintf("<soapenv:Fault><faultcode>sf:INVALID_TYPE</faultcode><faultstring>%s</faultstring></soapenv:Fault>", m.fault))
			return
		}
		var req struct {
			Request     retrieveRequest `xml:"Body>retrieve>retrieveRequest>unpackaged"`
			PackageName string          `xml:"Body>retrieve>retrieveRequest>packageNames"`
		}
		if err := xml.Unmarshal(body, &req); err != nil {
			m.t.Fatalf("bad retrieve request: %s", err)
		}
		m.retrieves++
		id := fmt.Sprintf("09S%07d", m.retrieves)
		m.jobs[id] = retrieveJob{request: req.Request, packageName: req.PackageName}
		fmt.Fprintf(w, envelope, fmt.Sprintf("<retrieveResponse><result><id>%s</id></result></retrieveResponse>", id))
	case bytes.Contains(body, []byte("<checkStatus ")):
		id := m.requestId(body, "checkStatus")
		result := `<checkStatusResponse><result><done>true</done><state>Succeeded</state></result></checkStatusResponse>`
		if m.fileCount(m.jobs[id]) > m.limit {
			result = fmt.Sprintf(`<checkStatusResponse><result><done>true</done><state>Error</state><message>LIMIT_EXCEEDED: Too many files in retrieve call, limit is: %d</message></result></checkStatusResponse>`, m.limit)
		}
		fmt.Fprintf(w, envelope, result)
	case bytes.Contains(body, []byte("<checkRetrieveStatus ")):
		id := m.requestId(body, "checkRetrieveStatus")
		zipData := base64.StdEncoding.EncodeToString(m.resultZip(m.jobs[id]))
		problems := ""
		if m.problem != "" {
			problems = fmt.Sprintf("<messages><problem>%s</problem></messages>", m.problem)
		}
		fmt.Fprintf(w, envelope, fmt.Sprintf("<checkRetrieveStatusResponse><result>%s<zipFile>%s</zipFile></result></checkRetrieveStatusResponse>", problems, zipData))
	default:
		m.t.Fatalf("unexpected request: %s", body)
	}
}

func (m *mockRetrieveServer) start() (*ForceMetadata, func()) {
	server := httptest.NewServer(http.HandlerFunc(m.handler))
	force := &Force{
		Credentials: &ForceSession{
			InstanceUrl: server.URL,
			AccessToken: "test-token",
		},
	}
	return NewForceMetadata(force), server.Close
}

func (m *mockRetrieveServer) requestId(body []byte, action string) string {
	var req struct {
		CheckStatusId         string `xml:"Body>checkStatus>id"`
		CheckRetrieveStatusId string `xml:"Body>checkRetrieveStatus>id"`
	}
	if err := xml.Unmarshal(body, &req); err != nil {
		m.t.Fatalf("bad %s request: %s", action, err)
	}
	if action == "checkStatus" {
		return req.CheckStatusId
	}
	return req.CheckRetrieveStatusId
}

type collectingHandler struct {
	names    []string
	contents map[string]string
}

func newCollectingHandler() *collectingHandler {
	return &collectingHandler{contents: make(map[string]string)}
}

func (c *collectingHandler) handle(name string, r io.Reader) error {
	data, err := io.ReadAll(r)
	if err != nil {
		return err
	}
	if _, seen := c.contents[name]; seen {
		return fmt.Errorf("entry %s received more than once", name)
	}
	c.names = append(c.names, name)
	c.contents[name] = string(data)
	return nil
}

func TestRetrieveStream_NoSplitNeeded(t *testing.T) {
	mock := &mockRetrieveServer{t: t, limit: 100, jobs: make(map[string]retrieveJob)}
	fm, closeServer := mock.start()
	defer closeServer()

	query := ForceMetadataQuery{
		{Name: []string{"ApexClass"}, Members: []string{"One", "Two"}},
	}
	collector := newCollectingHandler()
	problems, err := fm.RetrieveStream(query, collector.handle)
	if err != nil {
		t.Fatal(err)
	}
	if len(problems) != 0 {
		t.Errorf("problems = %v", problems)
	}
	if mock.retrieves != 1 {
		t.Errorf("retrieve calls = %d, want 1", mock.retrieves)
	}
	want := []string{"ApexClass/One", "ApexClass/Two", "package.xml"}
	if strings.Join(collector.names, ",") != strings.Join(want, ",") {
		t.Errorf("entries = %v, want %v", collector.names, want)
	}
	if collector.contents["ApexClass/One"] != "content of ApexClass/One" {
		t.Errorf("content = %q", collector.contents["ApexClass/One"])
	}
}

func TestRetrieveStream_BisectsUntilBatchesFit(t *testing.T) {
	// A wildcard expanding to 6 files plus 10 explicit members against a
	// server limit of 8: the explicit member count never predicts the
	// failure, so only server-driven bisection can make progress.
	mock := &mockRetrieveServer{
		t:          t,
		limit:      8,
		expansions: map[string]int{"ApexClass": 6},
		problem:    "some warning",
		jobs:       make(map[string]retrieveJob),
	}
	fm, closeServer := mock.start()
	defer closeServer()

	reports := memberNames(10, "r")
	query := ForceMetadataQuery{
		{Name: []string{"ApexClass"}, Members: []string{"*"}},
		{Name: []string{"Report"}, Members: reports},
	}
	collector := newCollectingHandler()
	problems, err := fm.RetrieveStream(query, collector.handle)
	if err != nil {
		t.Fatal(err)
	}

	// Attempts: full query (16 files, fails), first half (11 files,
	// fails), quarter (8 files, ok), quarter (3 files, ok), second half
	// (5 files, ok)
	if mock.retrieves != 5 {
		t.Errorf("retrieve calls = %d, want 5", mock.retrieves)
	}
	// One problem per successful batch
	if len(problems) != 3 {
		t.Errorf("problems = %v, want 3 entries", problems)
	}

	for i := 0; i < 6; i++ {
		name := fmt.Sprintf("ApexClass/wild%d", i)
		if _, ok := collector.contents[name]; !ok {
			t.Errorf("missing %s", name)
		}
	}
	for _, r := range reports {
		name := "Report/" + r
		if _, ok := collector.contents[name]; !ok {
			t.Errorf("missing %s", name)
		}
	}
	if len(collector.names) != 17 {
		t.Errorf("got %d entries, want 17 (16 files + package.xml)", len(collector.names))
	}
	if collector.names[len(collector.names)-1] != "package.xml" {
		t.Errorf("package.xml should be delivered last, got %v", collector.names[len(collector.names)-1])
	}

	var pkg Package
	if err := xml.Unmarshal([]byte(collector.contents["package.xml"]), &pkg); err != nil {
		t.Fatal(err)
	}
	byName := make(map[string]int)
	for _, mt := range pkg.Types {
		byName[mt.Name] += len(mt.Members)
	}
	if byName["Report"] != 10 {
		t.Errorf("merged package.xml has %d Report members, want 10", byName["Report"])
	}
	if byName["ApexClass"] != 6 {
		t.Errorf("merged package.xml has %d ApexClass members, want 6", byName["ApexClass"])
	}
}

func TestRetrieveStream_SingleWildcardCannotSplit(t *testing.T) {
	mock := &mockRetrieveServer{
		t:          t,
		limit:      8,
		expansions: map[string]int{"Layout": 20},
		jobs:       make(map[string]retrieveJob),
	}
	fm, closeServer := mock.start()
	defer closeServer()

	query := ForceMetadataQuery{
		{Name: []string{"Layout"}, Members: []string{"*"}},
	}
	collector := newCollectingHandler()
	_, err := fm.RetrieveStream(query, collector.handle)
	if !isRetrieveLimitError(err) {
		t.Fatalf("err = %v, want LIMIT_EXCEEDED", err)
	}
	if mock.retrieves != 1 {
		t.Errorf("retrieve calls = %d, want 1 (nothing left to bisect)", mock.retrieves)
	}
	if len(collector.names) != 0 {
		t.Errorf("no entries should be delivered, got %v", collector.names)
	}
}

func TestRetrieveStream_PreserveZipNumbering(t *testing.T) {
	mock := &mockRetrieveServer{t: t, limit: 2, jobs: make(map[string]retrieveJob)}
	fm, closeServer := mock.start()
	defer closeServer()

	t.Chdir(t.TempDir())
	SetPreserveZip(true)
	defer SetPreserveZip(false)

	query := ForceMetadataQuery{
		{Name: []string{"ApexClass"}, Members: memberNames(4, "c")},
	}
	collector := newCollectingHandler()
	if _, err := fm.RetrieveStream(query, collector.handle); err != nil {
		t.Fatal(err)
	}
	// 4 members fail against limit 2, then two halves of 2 succeed; each
	// successful download is preserved
	for _, name := range []string{"inbound.zip", "inbound-2.zip"} {
		if _, err := os.Stat(name); err != nil {
			t.Errorf("expected %s to be preserved: %v", name, err)
		}
	}
	if _, err := os.Stat("inbound-3.zip"); !os.IsNotExist(err) {
		t.Error("only two zips should be preserved")
	}
}

func TestRetrieve_MapAPIBisectsAndMerges(t *testing.T) {
	mock := &mockRetrieveServer{t: t, limit: 2, jobs: make(map[string]retrieveJob)}
	fm, closeServer := mock.start()
	defer closeServer()

	query := ForceMetadataQuery{
		{Name: []string{"ApexClass"}, Members: memberNames(4, "c")},
	}
	files, problems, err := fm.Retrieve(query)
	if err != nil {
		t.Fatal(err)
	}
	if len(problems) != 0 {
		t.Errorf("problems = %v", problems)
	}
	// 4 members fail against limit 2, then two halves of 2 succeed
	if mock.retrieves != 3 {
		t.Errorf("retrieve calls = %d, want 3", mock.retrieves)
	}
	if len(files) != 5 {
		t.Fatalf("got %d files, want 5 (4 classes + package.xml)", len(files))
	}
	for i := range 4 {
		name := fmt.Sprintf("ApexClass/c%d", i)
		if string(files[name]) != "content of "+name {
			t.Errorf("%s = %q", name, files[name])
		}
	}
	var pkg Package
	if err := xml.Unmarshal(files["package.xml"], &pkg); err != nil {
		t.Fatal(err)
	}
	members := 0
	for _, mt := range pkg.Types {
		if mt.Name == "ApexClass" {
			members += len(mt.Members)
		}
	}
	if members != 4 {
		t.Errorf("merged package.xml has %d ApexClass members, want 4", members)
	}
}

func TestRetrievePackageStream(t *testing.T) {
	mock := &mockRetrieveServer{
		t:     t,
		limit: 100,
		packageFiles: map[string][]string{
			"MyPkg": {"classes/Foo.cls", "objects/Bar.object"},
		},
		jobs: make(map[string]retrieveJob),
	}
	fm, closeServer := mock.start()
	defer closeServer()

	collector := newCollectingHandler()
	problems, err := fm.RetrievePackageStream("MyPkg", collector.handle)
	if err != nil {
		t.Fatal(err)
	}
	if len(problems) != 0 {
		t.Errorf("problems = %v", problems)
	}
	if mock.retrieves != 1 {
		t.Errorf("retrieve calls = %d, want 1", mock.retrieves)
	}
	want := []string{"classes/Foo.cls", "objects/Bar.object", "package.xml"}
	if strings.Join(collector.names, ",") != strings.Join(want, ",") {
		t.Errorf("entries = %v, want %v", collector.names, want)
	}
}

func TestRetrievePackage_MapAPI(t *testing.T) {
	mock := &mockRetrieveServer{
		t:     t,
		limit: 100,
		packageFiles: map[string][]string{
			"MyPkg": {"classes/Foo.cls"},
		},
		jobs: make(map[string]retrieveJob),
	}
	fm, closeServer := mock.start()
	defer closeServer()

	files, _, err := fm.RetrievePackage("MyPkg")
	if err != nil {
		t.Fatal(err)
	}
	if string(files["classes/Foo.cls"]) != "content of classes/Foo.cls" {
		t.Errorf("files = %v", files)
	}
}

func TestRetrieveToDir(t *testing.T) {
	mock := &mockRetrieveServer{t: t, limit: 2, jobs: make(map[string]retrieveJob)}
	fm, closeServer := mock.start()
	defer closeServer()

	root := t.TempDir()
	query := ForceMetadataQuery{
		{Name: []string{"ApexClass"}, Members: memberNames(4, "c")},
	}
	if _, err := fm.RetrieveToDir(root, query); err != nil {
		t.Fatal(err)
	}
	for i := range 4 {
		name := fmt.Sprintf("c%d", i)
		data, err := os.ReadFile(filepath.Join(root, "ApexClass", name))
		if err != nil {
			t.Fatal(err)
		}
		if string(data) != "content of ApexClass/"+name {
			t.Errorf("%s = %q", name, data)
		}
	}
	pkgData, err := os.ReadFile(filepath.Join(root, "package.xml"))
	if err != nil {
		t.Fatal(err)
	}
	var pkg Package
	if err := xml.Unmarshal(pkgData, &pkg); err != nil {
		t.Fatal(err)
	}
	members := 0
	for _, mt := range pkg.Types {
		members += len(mt.Members)
	}
	if members != 4 {
		t.Errorf("package.xml on disk has %d members, want 4", members)
	}
}

func TestRetrieveByPackageXmlContents(t *testing.T) {
	mock := &mockRetrieveServer{t: t, limit: 100, jobs: make(map[string]retrieveJob)}
	fm, closeServer := mock.start()
	defer closeServer()

	manifest := []byte(xml.Header + `<Package xmlns="http://soap.sforce.com/2006/04/metadata">
    <types>
        <members>Foo</members>
        <name>ApexClass</name>
    </types>
    <version>62.0</version>
</Package>`)
	files, _, err := fm.RetrieveByPackageXmlContents(manifest)
	if err != nil {
		t.Fatal(err)
	}
	if string(files["ApexClass/Foo"]) != "content of ApexClass/Foo" {
		t.Errorf("files = %v", files)
	}
}

func TestRetrieveStream_SoapFault(t *testing.T) {
	mock := &mockRetrieveServer{t: t, limit: 100, fault: "INVALID_TYPE: This type is not supported", jobs: make(map[string]retrieveJob)}
	fm, closeServer := mock.start()
	defer closeServer()

	query := ForceMetadataQuery{
		{Name: []string{"Bogus"}, Members: []string{"*"}},
	}
	collector := newCollectingHandler()
	_, err := fm.RetrieveStream(query, collector.handle)
	if err == nil || !strings.Contains(err.Error(), "INVALID_TYPE") {
		t.Fatalf("err = %v, want INVALID_TYPE fault", err)
	}
	if len(collector.names) != 0 {
		t.Errorf("no entries should be delivered, got %v", collector.names)
	}
}

func TestRetrieveStream_ProactiveSplitOverKnownLimit(t *testing.T) {
	// When the explicitly-listed members exceed the 10,000-file retrieve
	// limit, the query must be split up front: every retrieve call the
	// server sees fits within the limit and none is wasted on a
	// LIMIT_EXCEEDED round trip.
	mock := &mockRetrieveServer{t: t, limit: maxFilesPerRetrieve, jobs: make(map[string]retrieveJob)}
	fm, closeServer := mock.start()
	defer closeServer()

	query := ForceMetadataQuery{
		{Name: []string{"Report"}, Members: memberNames(12000, "r")},
	}
	collector := newCollectingHandler()
	problems, err := fm.RetrieveStream(query, collector.handle)
	if err != nil {
		t.Fatal(err)
	}
	if len(problems) != 0 {
		t.Errorf("problems = %v", problems)
	}
	if mock.retrieves != 2 {
		t.Errorf("retrieve calls = %d, want exactly 2 (no failed attempts)", mock.retrieves)
	}
	for id, job := range mock.jobs {
		if n := mock.fileCount(job); n > maxFilesPerRetrieve {
			t.Errorf("retrieve %s was sent %d files, over the limit", id, n)
		}
	}
	if len(collector.names) != 12001 {
		t.Errorf("got %d entries, want 12001 (12000 reports + package.xml)", len(collector.names))
	}
}

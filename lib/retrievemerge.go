package lib

import (
	"bytes"
	"encoding/xml"
	"errors"
	"fmt"
	"io"
	"os"
	"path"
	"path/filepath"
	"strings"
)

// containerType describes a metadata type whose metadata format file holds
// components of other types, such as the CustomField components stored in
// objects/<Object>.object. childTags lists the top-level elements that hold
// those components; each is identified by its fullName element.
type containerType struct {
	name      string
	dir       string
	ext       string
	childTags []string
}

var containerTypes = []containerType{
	{"CustomObject", "objects", ".object", []string{"businessProcesses", "compactLayouts", "fieldSets", "fields", "indexes", "listViews", "recordTypes", "sharingReasons", "validationRules", "webLinks"}},
	{"Workflow", "workflows", ".workflow", []string{"alerts", "fieldUpdates", "flowActions", "knowledgePublishes", "outboundMessages", "rules", "send", "tasks"}},
	{"SharingRules", "sharingRules", ".sharingRules", []string{"sharingCriteriaRules", "sharingGuestRules", "sharingOwnerRules", "sharingTerritoryRules"}},
	{"CustomLabels", "labels", ".labels", []string{"labels"}},
	{"AssignmentRules", "assignmentRules", ".assignmentRules", []string{"assignmentRule"}},
	{"AutoResponseRules", "autoResponseRules", ".autoResponseRules", []string{"autoResponseRule"}},
	{"EscalationRules", "escalationRules", ".escalationRules", []string{"escalationRule"}},
	{"MatchingRules", "matchingRules", ".matchingRule", []string{"matchingRules"}},
	{"Bot", "bots", ".bot", []string{"botVersions"}},
}

// containerForEntry returns the container type and member name of a
// retrieved entry such as "objects/Account.object".
func containerForEntry(name string) (c containerType, member string, ok bool) {
	dir, file := path.Split(name)
	dir = strings.TrimSuffix(dir, "/")
	for _, c := range containerTypes {
		if dir == c.dir && strings.HasSuffix(file, c.ext) && len(file) > len(c.ext) {
			return c, strings.TrimSuffix(file, c.ext), true
		}
	}
	return containerType{}, "", false
}

// requestedComponents records the components a retrieve query names, keyed
// by lower-cased type and member, since Salesforce matches them without
// regard to case.
type requestedComponents map[string]bool

func newRequestedComponents(query ForceMetadataQuery) requestedComponents {
	requested := make(requestedComponents)
	for _, element := range query {
		for _, name := range element.Name {
			for _, member := range element.Members {
				requested[strings.ToLower(name+"\x00"+member)] = true
			}
		}
	}
	return requested
}

// has reports whether the query names the member of typeName, either by
// name or with a wildcard.
func (r requestedComponents) has(typeName, member string) bool {
	return r[strings.ToLower(typeName+"\x00*")] || r[strings.ToLower(typeName+"\x00"+member)]
}

// MergingEntryDiskWriter returns a RetrieveEntryHandler that writes each
// entry under root like EntryDiskWriter, except for metadata format files
// that hold child components, such as objects/Account.object. When such a
// file already exists, and either query does not request the parent
// component itself or an earlier entry in this retrieve already wrote the
// file, the retrieved child components are merged into the existing file
// instead of replacing it. Retrieving CustomField Account.Foo__c therefore
// adds or updates the Foo__c field in objects/Account.object and keeps the
// rest of the file.
func MergingEntryDiskWriter(root string, query ForceMetadataQuery) RetrieveEntryHandler {
	write := EntryDiskWriter(root)
	requested := newRequestedComponents(query)
	written := make(map[string]bool)
	cleanRoot := filepath.Clean(root)
	return func(name string, r io.Reader) error {
		c, member, ok := containerForEntry(name)
		if !ok {
			return write(name, r)
		}
		mergeExisting := written[name] || !requested.has(c.name, member)
		written[name] = true
		if !mergeExisting {
			return write(name, r)
		}
		entry := filepath.FromSlash(name)
		if !filepath.IsLocal(entry) {
			return fmt.Errorf("entry %s would be written outside %s", name, root)
		}
		existing, err := os.ReadFile(filepath.Join(cleanRoot, entry))
		if errors.Is(err, os.ErrNotExist) {
			return write(name, r)
		}
		if err != nil {
			return err
		}
		retrieved, err := io.ReadAll(r)
		if err != nil {
			return err
		}
		merged, err := mergeComponentXml(existing, retrieved, c.childTags)
		if err != nil {
			return fmt.Errorf("could not merge retrieved %s into existing file: %w", name, err)
		}
		return write(name, bytes.NewReader(merged))
	}
}

// xmlElement is a top-level element within the root element of a metadata
// file. lead holds the whitespace and comments preceding the element and raw
// holds the element itself.
type xmlElement struct {
	tag      string
	fullName string
	lead     []byte
	raw      []byte
}

type xmlDocument struct {
	rootTag  string
	prefix   []byte
	elements []xmlElement
	suffix   []byte
}

func parseXmlDocument(data []byte) (doc xmlDocument, err error) {
	d := xml.NewDecoder(bytes.NewReader(data))
	var rootEnd int64
	for {
		var tok xml.Token
		if tok, err = d.Token(); err != nil {
			if err == io.EOF {
				err = fmt.Errorf("no root element")
			}
			return
		}
		if start, ok := tok.(xml.StartElement); ok {
			doc.rootTag = start.Name.Local
			rootEnd = d.InputOffset()
			break
		}
	}
	if bytes.HasSuffix(bytes.TrimRight(data[:rootEnd], " \t\r\n"), []byte("/>")) {
		return doc, fmt.Errorf("root element %s is empty", doc.rootTag)
	}
	doc.prefix = data[:rootEnd]
	prevEnd := rootEnd
	for {
		start := d.InputOffset()
		var tok xml.Token
		if tok, err = d.Token(); err != nil {
			if err == io.EOF {
				err = fmt.Errorf("unterminated root element %s", doc.rootTag)
			}
			return
		}
		switch t := tok.(type) {
		case xml.EndElement:
			doc.suffix = data[prevEnd:]
			return
		case xml.StartElement:
			el := xmlElement{tag: t.Name.Local, lead: data[prevEnd:start]}
			if el.fullName, err = readElement(d); err != nil {
				return
			}
			prevEnd = d.InputOffset()
			el.raw = data[start:prevEnd]
			doc.elements = append(doc.elements, el)
		}
	}
}

// readElement consumes the tokens of the element whose start tag was just
// read, returning the text of its fullName child element, if any.
func readElement(d *xml.Decoder) (fullName string, err error) {
	depth := 1
	inFullName := false
	for depth > 0 {
		var tok xml.Token
		if tok, err = d.Token(); err != nil {
			return
		}
		switch t := tok.(type) {
		case xml.StartElement:
			depth++
			inFullName = depth == 2 && t.Name.Local == "fullName"
		case xml.EndElement:
			depth--
			inFullName = false
		case xml.CharData:
			if inFullName {
				fullName += string(t)
			}
		}
	}
	return strings.TrimSpace(fullName), nil
}

func (doc xmlDocument) bytes() []byte {
	var buf bytes.Buffer
	buf.Write(doc.prefix)
	for _, el := range doc.elements {
		buf.Write(el.lead)
		buf.Write(el.raw)
	}
	buf.Write(doc.suffix)
	return buf.Bytes()
}

// mergeComponentXml merges the top-level elements of retrieved into
// existing. Elements named in childTags that have a fullName replace the
// existing element of the same tag and fullName. Other elements replace all
// existing elements with the same tag. Elements with no existing
// counterpart are inserted in order of tag, then fullName, which is the
// order the Metadata API uses.
func mergeComponentXml(existing, retrieved []byte, childTags []string) ([]byte, error) {
	doc, err := parseXmlDocument(existing)
	if err != nil {
		return nil, err
	}
	in, err := parseXmlDocument(retrieved)
	if err != nil {
		return nil, err
	}
	if doc.rootTag != in.rootTag {
		return nil, fmt.Errorf("root element %s does not match %s", in.rootTag, doc.rootTag)
	}
	isChild := make(map[string]bool)
	for _, tag := range childTags {
		isChild[tag] = true
	}
	matches := func(a, b xmlElement) bool {
		if a.tag != b.tag {
			return false
		}
		if isChild[a.tag] && a.fullName != "" {
			return a.fullName == b.fullName
		}
		return true
	}

	for i := 0; i < len(in.elements); {
		// Group consecutive retrieved elements that replace the same
		// existing elements.
		j := i + 1
		for j < len(in.elements) && !(isChild[in.elements[i].tag] && in.elements[i].fullName != "") && matches(in.elements[i], in.elements[j]) {
			j++
		}
		group := in.elements[i:j]

		insertAt := -1
		var kept []xmlElement
		for _, el := range doc.elements {
			if matches(group[0], el) {
				if insertAt < 0 {
					insertAt = len(kept)
					group[0].lead = el.lead
				}
				continue
			}
			kept = append(kept, el)
		}
		if insertAt < 0 {
			insertAt = len(kept)
			for k, el := range kept {
				if el.tag > group[0].tag || (el.tag == group[0].tag && el.fullName > group[0].fullName) {
					insertAt = k
					break
				}
			}
		}
		doc.elements = append(kept[:insertAt:insertAt], append(append([]xmlElement{}, group...), kept[insertAt:]...)...)
		i = j
	}
	return doc.bytes(), nil
}

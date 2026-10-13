package lib

import (
	"bytes"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"

	"github.com/ForceCLI/force-md/metadata"
	"github.com/ForceCLI/force-md/metadata/objectTranslations"
	"github.com/ForceCLI/force-md/metadata/objects"
)

// folderSuffixes maps the directory of a foldered metadata type to the
// suffix source format gives its folder files: a report folder that is
// reports/Sales-meta.xml in metadata format is
// reports/Sales.reportFolder-meta.xml in source format.
var folderSuffixes = map[string]string{
	"dashboards": ".dashboardFolder",
	"documents":  ".documentFolder",
	"email":      ".emailFolder",
	"reports":    ".reportFolder",
}

// SourceFormatEntryWriter returns a RetrieveEntryHandler that converts each
// retrieved metadata format entry to source (sfdx) format and writes it under
// root. Objects and object translations are split into a file per child
// component. The object's own objects/<Object>/<Object>.object-meta.xml file
// is only written when query requests the CustomObject, so retrieving a
// CustomField writes only the field's file. package.xml is not written.
func SourceFormatEntryWriter(root string, query ForceMetadataQuery) RetrieveEntryHandler {
	write := EntryDiskWriter(root)
	requested := newRequestedComponents(query)
	return func(name string, r io.Reader) error {
		if name == "package.xml" {
			_, err := io.Copy(io.Discard, r)
			return err
		}
		parts := strings.Split(name, "/")
		mp, ok := metapathForDirectory(parts[0])
		switch {
		case !ok:
			return write(name, r)
		case len(parts) == 2 && (mp.name == "CustomObject" || mp.name == "CustomObjectTranslation"):
			return writeDecomposed(write, mp.name, name, r, requested)
		case len(parts) == 2 && folderSuffixes[parts[0]] != "" && strings.HasSuffix(name, "-meta.xml"):
			folderName := strings.TrimSuffix(parts[1], "-meta.xml")
			return write(parts[0]+"/"+folderName+folderSuffixes[parts[0]]+"-meta.xml", r)
		case mp.contentFile || mp.onlyFolder || strings.HasSuffix(name, "-meta.xml"):
			return write(name, r)
		default:
			return write(name+"-meta.xml", r)
		}
	}
}

// writeDecomposed splits a metadata format object or object translation into
// its source format files and writes them.
func writeDecomposed(write RetrieveEntryHandler, typeName string, name string, r io.Reader, requested requestedComponents) error {
	// force-md takes the component's name from its file name, so parse a
	// copy stored under the same name.
	tmpDir, err := os.MkdirTemp("", "force-fetch-")
	if err != nil {
		return err
	}
	defer os.RemoveAll(tmpDir)
	if err := EntryDiskWriter(tmpDir)(name, r); err != nil {
		return err
	}
	tmpPath := filepath.Join(tmpDir, filepath.FromSlash(name))

	var generator metadata.FilesGenerator
	if typeName == "CustomObject" {
		generator, err = objects.Open(tmpPath)
	} else {
		generator, err = objectTranslations.Open(tmpPath)
	}
	if err != nil {
		return fmt.Errorf("could not read retrieved %s: %w", name, err)
	}
	files, err := generator.Files(metadata.SourceFormat)
	if err != nil {
		return fmt.Errorf("could not convert retrieved %s to source format: %w", name, err)
	}

	member := strings.TrimSuffix(filepath.Base(tmpPath), filepath.Ext(tmpPath))
	objectFile := "objects/" + member + "/" + member + ".object-meta.xml"
	for path, content := range files {
		path = filepath.ToSlash(path)
		if typeName == "CustomObject" && path == objectFile && !requested.has(typeName, member) {
			continue
		}
		if err := write(path, bytes.NewReader(content)); err != nil {
			return err
		}
	}
	return nil
}

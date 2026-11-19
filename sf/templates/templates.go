// Package templates holds the file templates of the "template generate"
// commands.  They come from the salesforcedx-templates repository (see
// SOURCE and LICENSE.txt) and are stored as one compressed archive for each
// template directory.  "make update-sf-templates" rebuilds the archives.
package templates

import (
	"archive/tar"
	"bytes"
	"compress/gzip"
	"embed"
	"fmt"
	"io"
	"sort"
	"strings"
)

//go:embed *.tar.gz
var archives embed.FS

// License is the license of the templates.
//
//go:embed LICENSE.txt
var License string

// Source names the repository, tag and commit the archives were built
// from.
//
//go:embed SOURCE
var Source string

// Names returns the names of the template directories, such as apexclass
// and project.
func Names() []string {
	entries, _ := archives.ReadDir(".")
	names := make([]string, 0, len(entries))
	for _, entry := range entries {
		names = append(names, strings.TrimSuffix(entry.Name(), ".tar.gz"))
	}
	sort.Strings(names)
	return names
}

// Files returns the files of a template directory, keyed by path relative
// to the directory, with forward slashes.
func Files(name string) (map[string][]byte, error) {
	data, err := archives.ReadFile(name + ".tar.gz")
	if err != nil {
		return nil, fmt.Errorf("no templates named %s", name)
	}
	compressed, err := gzip.NewReader(bytes.NewReader(data))
	if err != nil {
		return nil, err
	}
	files := map[string][]byte{}
	reader := tar.NewReader(compressed)
	for {
		header, err := reader.Next()
		if err == io.EOF {
			return files, nil
		}
		if err != nil {
			return nil, err
		}
		content, err := io.ReadAll(reader)
		if err != nil {
			return nil, err
		}
		files[header.Name] = content
	}
}

// File returns one file of a template directory.
func File(name, path string) ([]byte, error) {
	files, err := Files(name)
	if err != nil {
		return nil, err
	}
	content, ok := files[path]
	if !ok {
		return nil, fmt.Errorf("the %s templates have no file %s", name, path)
	}
	return content, nil
}

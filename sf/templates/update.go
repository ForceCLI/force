//go:build ignore

// This program rebuilds the template archives in this directory from the
// source repository of @salesforce/templates.  Run it with
//
//	make update-sf-templates SF_TEMPLATES_VERSION=<tag>
//
// It clones the repository at the tag, writes one <name>.tar.gz for each
// directory of src/templates, copies the license to LICENSE.txt, records the
// tag and commit in SOURCE, and writes the attribution block
// licenses/salesforcedx-templates.txt, which "make third-party-licenses.txt"
// appends to the license report.  The archives depend only on the content of the
// files: entries are sorted and carry no times or owners.
package main

import (
	"archive/tar"
	"bytes"
	"compress/gzip"
	"fmt"
	"io/fs"
	"os"
	"os/exec"
	"path/filepath"
	"sort"
	"strings"
)

func main() {
	if len(os.Args) != 4 {
		fail("usage: go run update.go <repository url> <tag> <repository root>")
	}
	repository, tag, root := os.Args[1], os.Args[2], os.Args[3]
	out := filepath.Join(root, "sf", "templates")

	clone, err := os.MkdirTemp("", "salesforcedx-templates")
	check(err)
	defer os.RemoveAll(clone)
	run("git", "clone", "--quiet", "--depth", "1", "--branch", tag, repository, clone)
	commit := strings.TrimSpace(output("git", "-C", clone, "rev-parse", "HEAD"))

	// Archives of templates that the tag no longer has are removed.
	old, err := filepath.Glob(filepath.Join(out, "*.tar.gz"))
	check(err)
	for _, path := range old {
		check(os.Remove(path))
	}
	source := filepath.Join(clone, "src", "templates")
	entries, err := os.ReadDir(source)
	check(err)
	for _, entry := range entries {
		if !entry.IsDir() {
			continue
		}
		data, count := archive(filepath.Join(source, entry.Name()))
		check(os.WriteFile(filepath.Join(out, entry.Name()+".tar.gz"), data, 0o644))
		fmt.Printf("%s: %d files, %d bytes\n", entry.Name(), count, len(data))
	}

	license, err := os.ReadFile(filepath.Join(clone, "LICENSE.txt"))
	check(err)
	check(os.WriteFile(filepath.Join(out, "LICENSE.txt"), license, 0o644))
	check(os.WriteFile(filepath.Join(out, "SOURCE"), []byte(fmt.Sprintf("repository: %s\ntag: %s\ncommit: %s\n", repository, tag, commit)), 0o644))

	// The attribution block has the layout of the blocks go-licenses
	// writes with licenses.tpl.
	rule := strings.Repeat("=", 80)
	notice := rule + "\n" +
		"salesforcedx-templates " + tag + " (file templates of the sf \"template generate\" commands)\n" +
		"BSD-3-Clause\n" +
		rule + "\n\n" +
		"Source: " + repository + " commit " + commit + "\n" +
		"The templates are stored in sf/templates as compressed archives and are\n" +
		"compiled into the binary.\n\n" +
		string(license) + "\n"
	check(os.MkdirAll(filepath.Join(root, "licenses"), 0o755))
	check(os.WriteFile(filepath.Join(root, "licenses", "salesforcedx-templates.txt"), []byte(notice), 0o644))
}

// archive returns a gzip-compressed tar of the regular files under dir,
// with paths relative to dir.
func archive(dir string) ([]byte, int) {
	var paths []string
	check(filepath.WalkDir(dir, func(path string, entry fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if entry.Type().IsRegular() {
			paths = append(paths, path)
		}
		return nil
	}))
	sort.Strings(paths)
	var buffer bytes.Buffer
	compressed, err := gzip.NewWriterLevel(&buffer, gzip.BestCompression)
	check(err)
	writer := tar.NewWriter(compressed)
	for _, path := range paths {
		data, err := os.ReadFile(path)
		check(err)
		relative, err := filepath.Rel(dir, path)
		check(err)
		check(writer.WriteHeader(&tar.Header{Name: filepath.ToSlash(relative), Mode: 0o644, Size: int64(len(data)), Format: tar.FormatPAX}))
		_, err = writer.Write(data)
		check(err)
	}
	check(writer.Close())
	check(compressed.Close())
	return buffer.Bytes(), len(paths)
}

func run(name string, args ...string) {
	command := exec.Command(name, args...)
	command.Stdout, command.Stderr = os.Stdout, os.Stderr
	check(command.Run())
}

func output(name string, args ...string) string {
	data, err := exec.Command(name, args...).Output()
	check(err)
	return string(data)
}

func check(err error) {
	if err != nil {
		fail(err.Error())
	}
}

func fail(message string) {
	fmt.Fprintln(os.Stderr, message)
	os.Exit(1)
}

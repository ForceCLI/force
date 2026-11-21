package lib

import (
	"archive/zip"
	"bytes"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"strings"
)

const staticResourceDirectory = "staticresources"

// sourceFormatStaticResource reports whether the file or directory at fpath,
// under root, belongs to a static resource stored in source (sfdx) format, and
// returns the resource's name and the path of its content. In metadata format
// a static resource is staticresources/Name.resource beside
// staticresources/Name.resource-meta.xml. Source format stores it in one of
// two other ways:
//
//   - A zip resource is expanded into the directory staticresources/Name.
//     The directory is used even when staticresources/Name.resource also
//     exists, as it does after force fetch --unpack, because the directory
//     holds the files that are edited.
//   - Any other resource keeps a file extension that matches its content type,
//     such as staticresources/Name.js, beside its -meta.xml file.
//
// fpath can be the -meta.xml file, the content file, the expanded directory,
// or any file or directory inside the expanded directory.
func sourceFormatStaticResource(root string, fpath string) (name string, content string, ok bool) {
	relativePath, err := filepath.Rel(root, fpath)
	if err != nil || !filepath.IsLocal(relativePath) {
		return "", "", false
	}
	parts := strings.Split(relativePath, string(os.PathSeparator))
	if len(parts) < 2 || parts[0] != staticResourceDirectory {
		return "", "", false
	}
	name = parts[1]
	if len(parts) == 2 {
		if info, err := os.Stat(fpath); err != nil || !info.IsDir() {
			// Static resource names cannot contain a period, so the
			// name of Name.resource-meta.xml or Name.js ends at the
			// first one.
			name, _, _ = strings.Cut(name, ".")
		}
	}
	if name == "" || strings.HasPrefix(name, ".") {
		return "", "", false
	}
	content, ok = StaticResourceContent(filepath.Join(root, staticResourceDirectory), name)
	if !ok {
		return "", "", false
	}
	return name, content, true
}

// StaticResourceContent returns the path of the source-format content of the
// static resource name in typeDirectory: the expanded directory, or the single
// file named name with an extension other than .resource that sits beside
// name.resource-meta.xml. It reports false for a resource in metadata format
// or one whose content cannot be found.
func StaticResourceContent(typeDirectory string, name string) (string, bool) {
	expanded := filepath.Join(typeDirectory, name)
	if info, err := os.Stat(expanded); err == nil && info.IsDir() {
		return expanded, true
	}
	meta := name + ".resource-meta.xml"
	if _, err := os.Stat(filepath.Join(typeDirectory, meta)); err != nil {
		return "", false
	}
	if _, err := os.Stat(filepath.Join(typeDirectory, name+".resource")); err == nil {
		return "", false
	}
	entries, err := os.ReadDir(typeDirectory)
	if err != nil {
		return "", false
	}
	content := ""
	for _, entry := range entries {
		if entry.Name() == meta || !entry.Type().IsRegular() || !strings.HasPrefix(entry.Name(), name+".") {
			continue
		}
		if content != "" {
			// More than one file could be the content.
			return "", false
		}
		content = filepath.Join(typeDirectory, entry.Name())
	}
	return content, content != ""
}

// addSourceFormatStaticResource adds the static resource that fpath belongs
// to when the resource is stored in source format, converting it to
// staticresources/Name.resource: an expanded directory is zipped, and a file
// such as Name.js is renamed. It reports false for a path that is not part of
// such a resource, which the caller then adds as it is.
func (pb *PackageBuilder) addSourceFormatStaticResource(fpath string) (bool, error) {
	name, content, ok := sourceFormatStaticResource(pb.Root, fpath)
	if !ok {
		return false, nil
	}
	pb.AddMetaToPackage("StaticResource", name)
	if !pb.IsPush {
		return true, nil
	}
	resourcePath := filepath.Join(staticResourceDirectory, name+".resource")
	if _, added := pb.Files[resourcePath]; added {
		return true, nil
	}

	var data []byte
	var err error
	if info, statErr := os.Stat(content); statErr == nil && info.IsDir() {
		data, err = zipDirectory(content)
	} else {
		data, err = os.ReadFile(content)
	}
	if err != nil {
		return true, fmt.Errorf("Could not add static resource %s: %w", name, err)
	}
	pb.Files[resourcePath] = data

	meta := filepath.Join(pb.Root, resourcePath+"-meta.xml")
	if metaData, err := os.ReadFile(meta); err == nil {
		pb.Files[resourcePath+"-meta.xml"] = metaData
	} else if !os.IsNotExist(err) {
		return true, fmt.Errorf("Could not add static resource %s: %w", name, err)
	}
	return true, nil
}

// zipDirectory returns a zip archive of the files under dir, named by their
// slash-separated paths relative to dir. Hidden files and directories, such
// as .DS_Store, are left out, as they are when a directory is pushed.
func zipDirectory(dir string) ([]byte, error) {
	buffer := new(bytes.Buffer)
	writer := zip.NewWriter(buffer)
	err := filepath.WalkDir(dir, func(path string, entry fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if path == dir {
			return nil
		}
		if strings.HasPrefix(entry.Name(), ".") {
			if entry.IsDir() {
				return filepath.SkipDir
			}
			return nil
		}
		if !entry.Type().IsRegular() {
			return nil
		}
		relativePath, err := filepath.Rel(dir, path)
		if err != nil {
			return err
		}
		data, err := os.ReadFile(path)
		if err != nil {
			return err
		}
		info, err := entry.Info()
		if err != nil {
			return err
		}
		header, err := zip.FileInfoHeader(info)
		if err != nil {
			return err
		}
		header.Name = filepath.ToSlash(relativePath)
		header.Method = zip.Deflate
		w, err := writer.CreateHeader(header)
		if err != nil {
			return err
		}
		_, err = w.Write(data)
		return err
	})
	if err != nil {
		return nil, err
	}
	if err := writer.Close(); err != nil {
		return nil, err
	}
	return buffer.Bytes(), nil
}

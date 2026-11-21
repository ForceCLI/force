package sf

import (
	"archive/zip"
	"bytes"
	"encoding/json"
	"encoding/xml"
	"fmt"
	"io"
	"io/fs"
	"os"
	"path"
	"path/filepath"
	"sort"
	"strings"

	"github.com/ForceCLI/force/lib"
)

// sfProject is the part of sfdx-project.json the project commands use.
type sfProject struct {
	Root               string
	PackageDirectories []struct {
		Path    string `json:"path"`
		Default bool   `json:"default"`
	} `json:"packageDirectories"`
	SourceApiVersion string `json:"sourceApiVersion"`
}

// loadProject reads sfdx-project.json from the working directory or one of
// its parents.
func loadProject() (*sfProject, error) {
	wd, err := os.Getwd()
	if err != nil {
		return nil, err
	}
	root, ok := lib.SFProjectRoot(wd)
	if !ok {
		return nil, &Error{
			Name:     "InvalidProjectWorkspaceError",
			Message:  fmt.Sprintf("%s does not contain a valid Salesforce DX project.", wd),
			ExitCode: 1,
		}
	}
	data, err := os.ReadFile(filepath.Join(root, "sfdx-project.json"))
	if err != nil {
		return nil, err
	}
	project := &sfProject{Root: root}
	if err := json.Unmarshal(data, project); err != nil {
		return nil, fmt.Errorf("sfdx-project.json: %w", err)
	}
	return project, nil
}

// defaultPackageDir returns the absolute path of the default package
// directory.
func (p *sfProject) defaultPackageDir() string {
	for _, dir := range p.PackageDirectories {
		if dir.Default {
			return filepath.Join(p.Root, dir.Path)
		}
	}
	if len(p.PackageDirectories) > 0 {
		return filepath.Join(p.Root, p.PackageDirectories[0].Path)
	}
	return filepath.Join(p.Root, "force-app")
}

func (p *sfProject) packageDirs() []string {
	dirs := make([]string, 0, len(p.PackageDirectories))
	for _, dir := range p.PackageDirectories {
		dirs = append(dirs, filepath.Join(p.Root, dir.Path))
	}
	return dirs
}

// metadataRoots returns the directories under path that hold metadata type
// directories such as classes or objects.  For force-app this is usually
// force-app/main/default.  When path is inside a metadata type directory,
// the root is the parent of that type directory.
func metadataRoots(path string) ([]string, error) {
	absolute, err := filepath.Abs(path)
	if err != nil {
		return nil, err
	}
	// A path inside a type directory: walk up to the type directory.
	for dir := absolute; ; dir = filepath.Dir(dir) {
		info, err := os.Stat(dir)
		if err == nil && info.IsDir() && lib.IsMetadataDirectory(filepath.Base(dir)) {
			return []string{filepath.Dir(dir)}, nil
		}
		if dir == filepath.Dir(dir) {
			break
		}
		if _, err := os.Stat(filepath.Join(dir, "sfdx-project.json")); err == nil {
			break
		}
	}
	var roots []string
	seen := map[string]bool{}
	err = filepath.Walk(absolute, func(walked string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}
		if !info.IsDir() {
			return nil
		}
		if lib.IsMetadataDirectory(info.Name()) && walked != absolute {
			root := filepath.Dir(walked)
			if !seen[root] {
				seen[root] = true
				roots = append(roots, root)
			}
			return filepath.SkipDir
		}
		return nil
	})
	return roots, err
}

// sourceSet is metadata gathered from local source, converted to metadata
// format.
type sourceSet struct {
	// files holds the metadata format files, keyed by path such as
	// classes/Greeter.cls.
	files lib.ForceMetadataFiles
	// types maps a metadata type to its member names.
	types map[string][]string
	// roots are the directories the files were read from.
	roots []string
	// ignore holds the .forceignore patterns of the project, or nil.
	ignore *forceIgnore
	// deletedPaths holds the files of the components a deploy deletes,
	// relative to the project, keyed by type and name ("ApexClass###A").
	deletedPaths map[string][]string
	// noFiles marks a set that stands for a deployment whose local files
	// are not known: sf then lists no files for it.
	noFiles bool
}

func newSourceSet() *sourceSet {
	return &sourceSet{files: lib.ForceMetadataFiles{}, types: map[string][]string{}}
}

func (s *sourceSet) merge(pb *lib.PackageBuilder) {
	for path, data := range pb.Files {
		if path != "package.xml" {
			s.files[path] = data
		}
	}
	for _, metaType := range pb.Metadata {
		for _, member := range metaType.Members {
			if !containsString(s.types[metaType.Name], member) {
				s.types[metaType.Name] = append(s.types[metaType.Name], member)
			}
		}
	}
	if !containsString(s.roots, pb.Root) {
		s.roots = append(s.roots, pb.Root)
	}
}

func containsString(values []string, value string) bool {
	for _, existing := range values {
		if existing == value {
			return true
		}
	}
	return false
}

// buildFromRoot runs build with a package builder rooted at root.  When
// .forceignore leaves out files under root, the builder reads a copy of
// root without those files; path is translated to the copy.
func (s *sourceSet) buildFromRoot(root string, path string, build func(pb *lib.PackageBuilder, path string) error) error {
	buildRoot := root
	if s.ignore != nil {
		mirror, cleanup, err := s.ignore.mirrorWithoutIgnored(root)
		if err != nil {
			return err
		}
		defer cleanup()
		buildRoot = mirror
	}
	if path != "" && buildRoot != root {
		if relative, err := filepath.Rel(root, path); err == nil {
			path = filepath.Join(buildRoot, relative)
		}
	}
	pb := lib.NewPushBuilder()
	pb.Root = buildRoot
	if err := build(&pb, path); err != nil {
		return err
	}
	// Report the directory the files came from, not the copy.
	pb.Root = root
	s.merge(&pb)
	return nil
}

// addPath adds the components under a source directory or file.
func (s *sourceSet) addPath(path string) error {
	if _, err := os.Stat(path); err != nil {
		return newError("SfError", "%s: File or folder not found", path)
	}
	roots, err := metadataRoots(path)
	if err != nil {
		return err
	}
	absolute, _ := filepath.Abs(path)
	for _, root := range roots {
		inside := strings.HasPrefix(absolute, root+string(os.PathSeparator))
		err := s.buildFromRoot(root, absolute, func(pb *lib.PackageBuilder, target string) error {
			if inside {
				// The path is a type directory, or a file or directory
				// inside one.
				if _, err := os.Stat(target); err != nil {
					// Everything at the path is ignored.
					return nil
				}
				return pb.Add(target)
			}
			entries, err := os.ReadDir(pb.Root)
			if err != nil {
				return err
			}
			for _, entry := range entries {
				if entry.IsDir() && lib.IsMetadataDirectory(entry.Name()) {
					if err := pb.AddDirectory(filepath.Join(pb.Root, entry.Name())); err != nil {
						return err
					}
				}
			}
			return nil
		})
		if err != nil {
			return err
		}
	}
	return nil
}

// addMetadata adds components named as Type or Type:Name, found in the
// project's package directories.
func (s *sourceSet) addMetadata(project *sfProject, names []string) error {
	for _, name := range names {
		metadataType, member, hasMember := strings.Cut(name, ":")
		found := false
		for _, dir := range project.packageDirs() {
			roots, err := metadataRoots(dir)
			if err != nil {
				continue
			}
			for _, root := range roots {
				before, beforeMembers := len(s.files), len(s.types[metadataType])
				err := s.buildFromRoot(root, "", func(pb *lib.PackageBuilder, _ string) error {
					if hasMember && member != "*" {
						return pb.AddMetadataItem(metadataType, member)
					}
					return pb.AddMetadataType(metadataType)
				})
				if err != nil {
					continue
				}
				if len(s.files) > before || len(s.types[metadataType]) > beforeMembers {
					found = true
				}
			}
		}
		if !found {
			return newError("SfError", "No source-backed components present in the package for %s.", name)
		}
	}
	return nil
}

type packageManifest struct {
	Types []struct {
		Members []string `xml:"members"`
		Name    string   `xml:"name"`
	} `xml:"types"`
	Version string `xml:"version"`
}

func readManifest(path string) (*packageManifest, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}
	manifest := &packageManifest{}
	if err := xml.Unmarshal(data, manifest); err != nil {
		return nil, fmt.Errorf("%s: %w", path, err)
	}
	return manifest, nil
}

func (m *packageManifest) query() lib.ForceMetadataQuery {
	var query lib.ForceMetadataQuery
	for _, metaType := range m.Types {
		query = append(query, lib.ForceMetadataQueryElement{Name: []string{metaType.Name}, Members: metaType.Members})
	}
	return query
}

// metadataNames returns the manifest entries in the Type:Name form used by
// --metadata.
func (m *packageManifest) metadataNames() []string {
	var names []string
	for _, metaType := range m.Types {
		for _, member := range metaType.Members {
			if member == "*" {
				names = append(names, metaType.Name)
			} else {
				names = append(names, metaType.Name+":"+member)
			}
		}
	}
	return names
}

// packageXML writes a manifest in the layout sf writes: types sorted by
// name, members sorted, four-space indentation.
func packageXML(types map[string][]string, version string) []byte {
	names := make([]string, 0, len(types))
	for name := range types {
		names = append(names, name)
	}
	sort.Strings(names)
	var b bytes.Buffer
	b.WriteString("<?xml version=\"1.0\" encoding=\"UTF-8\"?>\n<Package xmlns=\"http://soap.sforce.com/2006/04/metadata\">\n")
	for _, name := range names {
		members := append([]string{}, types[name]...)
		sort.Strings(members)
		b.WriteString("    <types>\n")
		for _, member := range members {
			b.WriteString("        <members>")
			xml.EscapeText(&b, []byte(member))
			b.WriteString("</members>\n")
		}
		fmt.Fprintf(&b, "        <name>%s</name>\n    </types>\n", name)
	}
	fmt.Fprintf(&b, "    <version>%s</version>\n</Package>\n", version)
	return b.Bytes()
}

// deployFiles returns the files to deploy, with a package.xml for the
// components in the set.
func (s *sourceSet) deployFiles(version string) lib.ForceMetadataFiles {
	files := lib.ForceMetadataFiles{}
	for path, data := range s.files {
		files[path] = data
	}
	files["package.xml"] = packageXML(s.types, version)
	return files
}

// localPaths returns the source files for a metadata format path such as
// classes/Greeter.cls or objects/Account.object: the file and its -meta.xml
// file, or the files of a source format component directory.
func (s *sourceSet) localPaths(metadataPath string) []string {
	var paths []string
	for _, root := range s.roots {
		base := filepath.Join(root, filepath.FromSlash(metadataPath))
		for _, candidate := range []string{base, base + "-meta.xml"} {
			if info, err := os.Stat(candidate); err == nil && !info.IsDir() {
				paths = append(paths, candidate)
			}
		}
		paths = append(paths, staticResourceSourcePaths(metadataPath, root)...)
		if len(paths) > 0 {
			return paths
		}
		// A decomposed component: objects/Account.object is stored as
		// the directory objects/Account.
		directory := strings.TrimSuffix(base, filepath.Ext(base))
		if info, err := os.Stat(directory); err == nil && info.IsDir() {
			filepath.Walk(directory, func(path string, info os.FileInfo, err error) error {
				if err == nil && info.Mode().IsRegular() {
					paths = append(paths, path)
				}
				return nil
			})
			if len(paths) > 0 {
				return paths
			}
		}
	}
	return paths
}

// staticResourceSourcePaths returns the source format content files of the
// static resource at a metadata format path such as
// staticresources/Leaflet.resource: the files in the expanded directory
// staticresources/Leaflet, or a file such as staticresources/Leaflet.js.
func staticResourceSourcePaths(metadataPath string, root string) []string {
	directory, file := path.Split(metadataPath)
	if directory != "staticresources/" || path.Ext(file) != ".resource" {
		return nil
	}
	content, ok := lib.StaticResourceContent(filepath.Join(root, "staticresources"), strings.TrimSuffix(file, ".resource"))
	if !ok {
		return nil
	}
	info, err := os.Stat(content)
	if err != nil {
		return nil
	}
	if !info.IsDir() {
		return []string{content}
	}
	var paths []string
	filepath.WalkDir(content, func(file string, entry fs.DirEntry, err error) error {
		if err != nil {
			return nil
		}
		if file != content && strings.HasPrefix(entry.Name(), ".") {
			if entry.IsDir() {
				return filepath.SkipDir
			}
			return nil
		}
		if entry.Type().IsRegular() {
			paths = append(paths, file)
		}
		return nil
	})
	return paths
}

// zipFiles builds a zip archive of metadata files, each under prefix.
func zipFiles(files lib.ForceMetadataFiles, prefix string) ([]byte, error) {
	var buffer bytes.Buffer
	writer := zip.NewWriter(&buffer)
	names := make([]string, 0, len(files))
	for name := range files {
		names = append(names, name)
	}
	sort.Strings(names)
	for _, name := range names {
		entry, err := writer.Create(prefix + filepath.ToSlash(name))
		if err != nil {
			return nil, err
		}
		if _, err := entry.Write(files[name]); err != nil {
			return nil, err
		}
	}
	if err := writer.Close(); err != nil {
		return nil, err
	}
	return buffer.Bytes(), nil
}

// readMetadataDir reads a directory in metadata format, or a zip file of
// one, into deployable files.
func readMetadataDir(path string) (lib.ForceMetadataFiles, error) {
	info, err := os.Stat(path)
	if err != nil {
		return nil, newError("SfError", "File or folder not found: %s", path)
	}
	files := lib.ForceMetadataFiles{}
	if !info.IsDir() {
		archive, err := zip.OpenReader(path)
		if err != nil {
			return nil, err
		}
		defer archive.Close()
		for _, entry := range archive.File {
			if entry.FileInfo().IsDir() {
				continue
			}
			reader, err := entry.Open()
			if err != nil {
				return nil, err
			}
			data, err := io.ReadAll(reader)
			reader.Close()
			if err != nil {
				return nil, err
			}
			files[entry.Name] = data
		}
		return files, nil
	}
	err = filepath.Walk(path, func(walked string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}
		if !info.Mode().IsRegular() || info.Name() == ".DS_Store" {
			return nil
		}
		relative, err := filepath.Rel(path, walked)
		if err != nil {
			return err
		}
		data, err := os.ReadFile(walked)
		if err != nil {
			return err
		}
		files[filepath.ToSlash(relative)] = data
		return nil
	})
	return files, err
}

// componentOfPath returns the metadata type and component name for a path
// relative to a metadata root, in either format.
func componentOfPath(relative string) (metadataType string, fullName string) {
	parts := strings.Split(filepath.ToSlash(relative), "/")
	metadataType, ok := lib.MetadataTypeForDirectory(parts[0])
	if !ok || len(parts) < 2 {
		return "", ""
	}
	name := strings.TrimSuffix(parts[1], "-meta.xml")
	if extension := filepath.Ext(name); extension != "" && len(parts) == 2 {
		name = strings.TrimSuffix(name, extension)
	}
	if len(parts) == 2 {
		return metadataType, name
	}
	// A file in a component directory: a bundle, a folder, or a
	// decomposed object.
	switch metadataType {
	case "CustomObject", "CustomObjectTranslation", "AuraDefinitionBundle", "LightningComponentBundle", "ExperienceBundle", "StaticResource":
		return metadataType, parts[1]
	}
	leaf := strings.TrimSuffix(parts[len(parts)-1], "-meta.xml")
	leaf = strings.TrimSuffix(leaf, filepath.Ext(leaf))
	return metadataType, strings.Join(append(parts[1:len(parts)-1], leaf), "/")
}

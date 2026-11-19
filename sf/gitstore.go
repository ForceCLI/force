package sf

import (
	"bytes"
	"compress/zlib"
	"crypto/sha1"
	"encoding/binary"
	"encoding/hex"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"
)

// shadowRepo is the git repository in which sf records the local files that
// are in sync with an org.  The repository has no checkout: its work tree is
// the project, and each commit holds the tracked files as they were when
// they were last deployed or retrieved.
//
// sf writes the repository with isomorphic-git, which stores every object
// as a loose object and never packs them.  This type reads and writes that
// subset of the git formats: loose objects, the main branch ref, and a
// version 2 index.
type shadowRepo struct {
	gitDir      string
	projectPath string
}

const shadowRepoConfig = "[core]\n\trepositoryformatversion = 0\n\tfilemode = false\n\tbare = false\n\tlogallrefupdates = true\n\tsymlinks = false\n\tignorecase = true\n"

// openShadowRepo returns the repository at gitDir, which it creates when it
// does not exist.
func openShadowRepo(gitDir, projectPath string) (*shadowRepo, error) {
	repo := &shadowRepo{gitDir: gitDir, projectPath: projectPath}
	if _, err := os.Stat(gitDir); err == nil {
		return repo, nil
	}
	for _, dir := range []string{"hooks", "info", "objects/info", "objects/pack", "refs/heads", "refs/tags"} {
		if err := os.MkdirAll(filepath.Join(gitDir, filepath.FromSlash(dir)), 0o755); err != nil {
			return nil, err
		}
	}
	if err := os.WriteFile(filepath.Join(gitDir, "HEAD"), []byte("ref: refs/heads/main\n"), 0o644); err != nil {
		return nil, err
	}
	if err := os.WriteFile(filepath.Join(gitDir, "config"), []byte(shadowRepoConfig), 0o644); err != nil {
		return nil, err
	}
	return repo, nil
}

func gitObjectID(kind string, data []byte) [20]byte {
	hash := sha1.New()
	fmt.Fprintf(hash, "%s %d\x00", kind, len(data))
	hash.Write(data)
	var id [20]byte
	copy(id[:], hash.Sum(nil))
	return id
}

func (r *shadowRepo) objectPath(id [20]byte) string {
	name := hex.EncodeToString(id[:])
	return filepath.Join(r.gitDir, "objects", name[:2], name[2:])
}

func (r *shadowRepo) writeObject(kind string, data []byte) ([20]byte, error) {
	id := gitObjectID(kind, data)
	path := r.objectPath(id)
	if _, err := os.Stat(path); err == nil {
		return id, nil
	}
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return id, err
	}
	var compressed bytes.Buffer
	writer := zlib.NewWriter(&compressed)
	fmt.Fprintf(writer, "%s %d\x00", kind, len(data))
	writer.Write(data)
	if err := writer.Close(); err != nil {
		return id, err
	}
	return id, os.WriteFile(path, compressed.Bytes(), 0o444)
}

func (r *shadowRepo) readObject(id [20]byte) (string, []byte, error) {
	file, err := os.Open(r.objectPath(id))
	if err != nil {
		return "", nil, fmt.Errorf("source tracking object %x: %w", id, err)
	}
	defer file.Close()
	reader, err := zlib.NewReader(file)
	if err != nil {
		return "", nil, fmt.Errorf("source tracking object %x: %w", id, err)
	}
	defer reader.Close()
	content, err := io.ReadAll(reader)
	if err != nil {
		return "", nil, fmt.Errorf("source tracking object %x: %w", id, err)
	}
	header, data, found := bytes.Cut(content, []byte{0})
	kind, _, _ := strings.Cut(string(header), " ")
	if !found {
		return "", nil, fmt.Errorf("source tracking object %x has no header", id)
	}
	return kind, data, nil
}

// head returns the id of the commit of the main branch, and false when the
// repository has no commit.
func (r *shadowRepo) head() ([20]byte, bool, error) {
	var id [20]byte
	content, err := os.ReadFile(filepath.Join(r.gitDir, "refs", "heads", "main"))
	if os.IsNotExist(err) {
		return id, false, nil
	}
	if err != nil {
		return id, false, err
	}
	decoded, err := hex.DecodeString(strings.TrimSpace(string(content)))
	if err != nil || len(decoded) != 20 {
		return id, false, fmt.Errorf("the source tracking branch does not name a commit: %q", content)
	}
	copy(id[:], decoded)
	return id, true, nil
}

// trackedFiles returns the files of the last commit: the path of each, with
// forward slashes and relative to the project, and the id of its content.
func (r *shadowRepo) trackedFiles() (map[string][20]byte, error) {
	files := map[string][20]byte{}
	commit, ok, err := r.head()
	if err != nil || !ok {
		return files, err
	}
	_, data, err := r.readObject(commit)
	if err != nil {
		return nil, err
	}
	line, _, _ := strings.Cut(string(data), "\n")
	decoded, err := hex.DecodeString(strings.TrimPrefix(line, "tree "))
	if err != nil || len(decoded) != 20 {
		return nil, fmt.Errorf("source tracking commit %x names no tree", commit)
	}
	var tree [20]byte
	copy(tree[:], decoded)
	return files, r.readTree(tree, "", files)
}

func (r *shadowRepo) readTree(id [20]byte, prefix string, files map[string][20]byte) error {
	_, data, err := r.readObject(id)
	if err != nil {
		return err
	}
	for len(data) > 0 {
		header, rest, found := bytes.Cut(data, []byte{0})
		mode, name, _ := strings.Cut(string(header), " ")
		if !found || len(rest) < 20 {
			return fmt.Errorf("source tracking tree %x is malformed", id)
		}
		var entry [20]byte
		copy(entry[:], rest[:20])
		data = rest[20:]
		if mode == "40000" || mode == "040000" {
			if err := r.readTree(entry, prefix+name+"/", files); err != nil {
				return err
			}
			continue
		}
		files[prefix+name] = entry
	}
	return nil
}

// writeTree writes the trees for files under prefix and returns the id of
// the tree for prefix.
func (r *shadowRepo) writeTree(prefix string, paths []string, files map[string][20]byte) ([20]byte, error) {
	type entry struct {
		name, mode string
		id         [20]byte
	}
	var entries []entry
	directories := map[string][]string{}
	for _, path := range paths {
		name, _, nested := strings.Cut(strings.TrimPrefix(path, prefix), "/")
		if nested {
			directories[name] = append(directories[name], path)
			continue
		}
		entries = append(entries, entry{name: name, mode: "100644", id: files[path]})
	}
	for name, children := range directories {
		id, err := r.writeTree(prefix+name+"/", children, files)
		if err != nil {
			return id, err
		}
		entries = append(entries, entry{name: name, mode: "40000", id: id})
	}
	// git orders the entries of a tree by name, with a directory compared
	// as its name followed by a slash.
	sortKey := func(e entry) string {
		if e.mode == "40000" {
			return e.name + "/"
		}
		return e.name
	}
	sort.Slice(entries, func(i, j int) bool { return sortKey(entries[i]) < sortKey(entries[j]) })
	var data bytes.Buffer
	for _, e := range entries {
		fmt.Fprintf(&data, "%s %s\x00", e.mode, e.name)
		data.Write(e.id[:])
	}
	return r.writeObject("tree", data.Bytes())
}

// commit records files as the tracked files, with a commit whose parent is
// the last commit, and returns the id of the commit.
func (r *shadowRepo) commit(files map[string][20]byte, message string, when time.Time) ([20]byte, error) {
	paths := make([]string, 0, len(files))
	for path := range files {
		paths = append(paths, path)
	}
	sort.Strings(paths)
	tree, err := r.writeTree("", paths, files)
	if err != nil {
		return tree, err
	}
	var data bytes.Buffer
	fmt.Fprintf(&data, "tree %x\n", tree)
	if parent, ok, err := r.head(); err != nil {
		return tree, err
	} else if ok {
		fmt.Fprintf(&data, "parent %x\n", parent)
	}
	signature := fmt.Sprintf("sfdx source tracking <> %d %s", when.Unix(), when.Format("-0700"))
	fmt.Fprintf(&data, "author %s\ncommitter %s\n\n%s\n", signature, signature, message)
	commit, err := r.writeObject("commit", data.Bytes())
	if err != nil {
		return commit, err
	}
	if err := r.writeIndex(paths, files); err != nil {
		return commit, err
	}
	return commit, os.WriteFile(filepath.Join(r.gitDir, "refs", "heads", "main"), []byte(hex.EncodeToString(commit[:])+"\n"), 0o644)
}

// writeIndex writes a version 2 index that lists files.  An entry has the
// size and modification time of the file in the project; the other stat
// fields are zero, which makes git and isomorphic-git compare the content of
// the file instead of trusting the entry.
func (r *shadowRepo) writeIndex(paths []string, files map[string][20]byte) error {
	var data bytes.Buffer
	data.WriteString("DIRC")
	binary.Write(&data, binary.BigEndian, uint32(2))
	binary.Write(&data, binary.BigEndian, uint32(len(paths)))
	for _, path := range paths {
		var size, seconds, nanoseconds uint32
		if info, err := os.Stat(filepath.Join(r.projectPath, filepath.FromSlash(path))); err == nil {
			size = uint32(info.Size())
			seconds = uint32(info.ModTime().Unix())
			nanoseconds = uint32(info.ModTime().Nanosecond())
		}
		id := files[path]
		// ctime, mtime, dev, ino, mode, uid, gid, size.
		for _, field := range []uint32{0, 0, seconds, nanoseconds, 0, 0, 0o100644, 0, 0, size} {
			binary.Write(&data, binary.BigEndian, field)
		}
		data.Write(id[:])
		nameLength := len(path)
		if nameLength > 0xfff {
			nameLength = 0xfff
		}
		binary.Write(&data, binary.BigEndian, uint16(nameLength))
		data.WriteString(path)
		// One to eight NUL bytes pad the entry to a multiple of eight.
		data.Write(make([]byte, 8-(62+len(path))%8))
	}
	checksum := sha1.Sum(data.Bytes())
	data.Write(checksum[:])
	return os.WriteFile(filepath.Join(r.gitDir, "index"), data.Bytes(), 0o644)
}

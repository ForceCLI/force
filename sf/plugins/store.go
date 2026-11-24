package plugins

import (
	"encoding/json"
	"errors"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"sort"
	"strings"

	"github.com/ForceCLI/force/sfplugin"
)

// DirEnv names the environment variable that replaces the default plugin
// directory, ~/.sf/plugins.
const DirEnv = "SF_PLUGINS_DIR"

// Record is one installed or linked plugin in plugins.json.
type Record struct {
	Name string `json:"name"`
	// Reference is the registry name without the tag.  It is empty for a
	// linked plugin.
	Reference string `json:"reference,omitempty"`
	Tag       string `json:"tag,omitempty"`
	// IndexDigest is the digest the tag pointed to at install.
	// ManifestDigest is the digest of the manifest for this platform, and
	// names the directory that holds the executable.
	IndexDigest    string `json:"indexDigest,omitempty"`
	ManifestDigest string `json:"manifestDigest,omitempty"`
	PlainHTTP      bool   `json:"plainHTTP,omitempty"`
	// Executable is the absolute path of the plugin executable.
	Executable string `json:"executable"`
	// Linked is true for an executable registered with "plugins link".
	Linked   bool              `json:"linked,omitempty"`
	Manifest sfplugin.Manifest `json:"manifest"`
}

type stateFile struct {
	Version int      `json:"version"`
	Plugins []Record `json:"plugins"`
}

// Store is the plugin directory: plugins.json and the downloaded
// executables.
type Store struct {
	Dir string
}

// DefaultDir returns the plugin directory: $SF_PLUGINS_DIR, or
// ~/.sf/plugins.
func DefaultDir() (string, error) {
	if dir := os.Getenv(DirEnv); dir != "" {
		return dir, nil
	}
	home, err := os.UserHomeDir()
	if err != nil {
		return "", err
	}
	return filepath.Join(home, ".sf", "plugins"), nil
}

// OpenDefault returns the store in the default plugin directory.
func OpenDefault() (*Store, error) {
	dir, err := DefaultDir()
	if err != nil {
		return nil, err
	}
	return &Store{Dir: dir}, nil
}

func (s *Store) statePath() string {
	return filepath.Join(s.Dir, "plugins.json")
}

// Load returns the recorded plugins sorted by name.  A missing plugins.json
// is an empty list.
func (s *Store) Load() ([]Record, error) {
	data, err := os.ReadFile(s.statePath())
	if errors.Is(err, fs.ErrNotExist) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	var state stateFile
	if err := json.Unmarshal(data, &state); err != nil {
		return nil, fmt.Errorf("%s is not valid JSON: %w", s.statePath(), err)
	}
	sort.Slice(state.Plugins, func(i, j int) bool { return state.Plugins[i].Name < state.Plugins[j].Name })
	return state.Plugins, nil
}

// Save replaces plugins.json.  The file is written under a temporary name
// and renamed, so a reader never sees part of it.
func (s *Store) Save(records []Record) error {
	if err := os.MkdirAll(s.Dir, 0o755); err != nil {
		return err
	}
	if records == nil {
		records = []Record{}
	}
	sort.Slice(records, func(i, j int) bool { return records[i].Name < records[j].Name })
	data, err := json.MarshalIndent(stateFile{Version: 1, Plugins: records}, "", "  ")
	if err != nil {
		return err
	}
	return writeFileAtomic(s.statePath(), append(data, '\n'), 0o644)
}

// Get returns the record of the plugin called name.
func (s *Store) Get(name string) (Record, bool, error) {
	records, err := s.Load()
	if err != nil {
		return Record{}, false, err
	}
	for _, record := range records {
		if record.Name == name {
			return record, true, nil
		}
	}
	return Record{}, false, nil
}

// Put adds the record, replacing a record of the same name, and removes the
// replaced plugin's files when no other record uses them.
func (s *Store) Put(record Record) error {
	records, err := s.Load()
	if err != nil {
		return err
	}
	var replaced *Record
	kept := records[:0]
	for _, existing := range records {
		if existing.Name == record.Name {
			previous := existing
			replaced = &previous
			continue
		}
		kept = append(kept, existing)
	}
	kept = append(kept, record)
	if err := s.Save(kept); err != nil {
		return err
	}
	if replaced != nil {
		s.removeUnusedFiles(*replaced, kept)
	}
	return nil
}

// Remove deletes the record of the plugin called name and its files.  The
// executable of a linked plugin is not in the store and is left in place.
func (s *Store) Remove(name string) (Record, bool, error) {
	records, err := s.Load()
	if err != nil {
		return Record{}, false, err
	}
	var removed *Record
	kept := records[:0]
	for _, existing := range records {
		if existing.Name == name {
			previous := existing
			removed = &previous
			continue
		}
		kept = append(kept, existing)
	}
	if removed == nil {
		return Record{}, false, nil
	}
	if err := s.Save(kept); err != nil {
		return Record{}, false, err
	}
	s.removeUnusedFiles(*removed, kept)
	return *removed, true, nil
}

func (s *Store) removeUnusedFiles(removed Record, kept []Record) {
	if removed.Linked || removed.ManifestDigest == "" {
		return
	}
	for _, record := range kept {
		if record.ManifestDigest == removed.ManifestDigest {
			return
		}
	}
	if dir, err := s.digestDir(removed.ManifestDigest); err == nil {
		os.RemoveAll(dir)
	}
}

// digestDir returns the directory for the files of one platform manifest:
// <dir>/store/sha256/<hex>.
func (s *Store) digestDir(digest string) (string, error) {
	algorithm, hex, ok := strings.Cut(digest, ":")
	if !ok || algorithm == "" || hex == "" || strings.ContainsAny(digest, `/\.`) {
		return "", fmt.Errorf("digest %q is not in the form algorithm:hex", digest)
	}
	return filepath.Join(s.Dir, "store", algorithm, hex), nil
}

func writeFileAtomic(path string, data []byte, mode os.FileMode) error {
	temp, err := os.CreateTemp(filepath.Dir(path), "."+filepath.Base(path)+".*")
	if err != nil {
		return err
	}
	defer os.Remove(temp.Name())
	if _, err := temp.Write(data); err != nil {
		temp.Close()
		return err
	}
	if err := temp.Close(); err != nil {
		return err
	}
	if err := os.Chmod(temp.Name(), mode); err != nil {
		return err
	}
	return os.Rename(temp.Name(), path)
}

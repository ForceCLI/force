package config

import (
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
)

type Manager struct {
	Base       string
	globalRoot string
}

var Config = newDefaultManager("force")

func newDefaultManager(base string) *Manager {
	base = normalizeBase(base)
	home, err := os.UserHomeDir()
	if err != nil || strings.TrimSpace(home) == "" {
		home = "."
	}
	root := filepath.Join(home, fmt.Sprintf(".%s", base))
	return &Manager{
		Base:       base,
		globalRoot: root,
	}
}

func newAbsoluteManager(dir string) *Manager {
	base := filepath.Base(dir)
	if base == "" || base == string(filepath.Separator) {
		base = "force"
	}
	return &Manager{
		Base:       base,
		globalRoot: dir,
	}
}

func normalizeBase(base string) string {
	base = strings.TrimSpace(base)
	base = strings.TrimPrefix(base, ".")
	if base == "" {
		base = "force"
	}
	return base
}

func expandPath(path string) (string, error) {
	path = strings.TrimSpace(path)
	if path == "" {
		return "", fmt.Errorf("empty path")
	}
	if strings.HasPrefix(path, "~") {
		home, err := os.UserHomeDir()
		if err != nil {
			return "", err
		}
		switch {
		case path == "~":
			path = home
		case len(path) > 1 && (path[1] == '/' || path[1] == '\\'):
			path = filepath.Join(home, path[2:])
		default:
			path = filepath.Join(home, path[1:])
		}
	}
	return filepath.Clean(path), nil
}

func UseConfigBase(base string) {
	Config = newDefaultManager(base)
}

func UseConfigDirectory(dir string) error {
	resolved, err := expandPath(dir)
	if err != nil {
		return err
	}
	if !filepath.IsAbs(resolved) {
		resolved, err = filepath.Abs(resolved)
		if err != nil {
			return err
		}
	}
	if err := os.MkdirAll(resolved, 0700); err != nil {
		return err
	}
	Config = newAbsoluteManager(resolved)
	return nil
}

func (m *Manager) GlobalRoot() string {
	return m.globalRoot
}

func (m *Manager) List(name string) ([]string, error) {
	dir := filepath.Join(m.globalRoot, name)
	entries, err := os.ReadDir(dir)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, nil
		}
		return nil, err
	}
	var keys []string
	for _, entry := range entries {
		if entry.IsDir() {
			continue
		}
		keys = append(keys, entry.Name())
	}
	sort.Strings(keys)
	return keys, nil
}

func (m *Manager) Save(name, key, value string) error {
	return m.saveFile(m.globalPath(name, key), value)
}

func (m *Manager) SaveGlobal(name, key, value string) error {
	return m.Save(name, key, value)
}

func (m *Manager) SaveLocal(name, key, value string) error {
	path, err := m.localPath(name, key)
	if err != nil {
		return err
	}
	return m.saveFile(path, value)
}

func (m *Manager) Load(name, key string) (string, error) {
	return m.readFile(m.globalPath(name, key))
}

func (m *Manager) LoadGlobal(name, key string) (string, error) {
	return m.Load(name, key)
}

func (m *Manager) LoadLocalOrGlobal(name, key string) (string, error) {
	if path, err := m.localPath(name, key); err == nil {
		if body, err := m.readFile(path); err == nil {
			return body, nil
		} else if !os.IsNotExist(err) {
			return "", err
		}
	}
	return m.readFile(m.globalPath(name, key))
}

func (m *Manager) Delete(name, key string) error {
	return m.DeleteGlobal(name, key)
}

func (m *Manager) DeleteGlobal(name, key string) error {
	filename := m.globalPath(name, key)
	if err := os.Remove(filename); err != nil && !os.IsNotExist(err) {
		return err
	}
	return nil
}

func (m *Manager) DeleteLocalOrGlobal(name, key string) error {
	if path, err := m.localPath(name, key); err == nil {
		if err := os.Remove(path); err == nil || os.IsNotExist(err) {
			return nil
		}
	}
	return m.DeleteGlobal(name, key)
}

func (m *Manager) globalPath(name, key string) string {
	return filepath.Join(m.globalRoot, name, key)
}

func (m *Manager) localPath(name, key string) (string, error) {
	cwd, err := os.Getwd()
	if err != nil {
		return "", err
	}
	dir := filepath.Join(cwd, fmt.Sprintf(".%s", m.Base), name)
	return filepath.Join(dir, key), nil
}

func (m *Manager) saveFile(filename, body string) error {
	dir := filepath.Dir(filename)
	if err := os.MkdirAll(dir, 0700); err != nil {
		return err
	}
	return os.WriteFile(filename, []byte(body), 0600)
}

func (m *Manager) readFile(filename string) (string, error) {
	data, err := os.ReadFile(filename)
	if err != nil {
		return "", err
	}
	return string(data), nil
}

var sourceDirs = []string{
	"src",
	"metadata",
}

// IsSourceDir returns a boolean indicating that dir is actually a Salesforce
// source directory.
func IsSourceDir(dir string) bool {
	if _, err := os.Stat(dir); err == nil {
		return true
	}
	return false
}

// GetSourceDir returns a rooted path name of the Salesforce source directory,
// relative to the current directory. GetSourceDir will look for a source
// directory in the nearest subdirectory. If no such directory exists, it will
// look at its parents, assuming that it is within a source directory already.
func GetSourceDir() (dir string, err error) {
	base, err := os.Getwd()
	if err != nil {
		return
	}

	// Look down to our nearest subdirectories
	for _, src := range sourceDirs {
		if len(src) > 0 {
			dir = filepath.Join(base, src)
			if IsSourceDir(dir) {
				return
			}
		}
	}

	// Check the current directory and then start looking up at our parents.
	// When dir's parent is identical, it means we're at the root.  If we blow
	// past the actual root, we should drop to the next section of code
	for dir != filepath.Dir(dir) {
		dir = filepath.Dir(dir)
		for _, src := range sourceDirs {
			adir := filepath.Join(dir, src)
			if IsSourceDir(adir) {
				dir = adir
				return
			}
		}
	}

	// No source directory found, create a src directory
	dir = filepath.Join(base, "src")
	err = os.Mkdir(dir, 0777)
	return
}

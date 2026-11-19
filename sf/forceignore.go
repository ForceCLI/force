package sf

import (
	"bufio"
	"os"
	"path/filepath"
	"regexp"
	"strings"
)

// forceIgnore holds the patterns of a project's .forceignore file.  The
// patterns use .gitignore syntax and are relative to the project root.
type forceIgnore struct {
	root  string
	rules []ignoreRule
}

type ignoreRule struct {
	pattern *regexp.Regexp
	negate  bool
}

// defaultIgnorePatterns are the patterns sf applies in every project.
var defaultIgnorePatterns = []string{"**/*.dup", "**/.*", "**/package2-descriptor.json", "**/package2-manifest.json"}

// ignoreRuleFor converts a .gitignore pattern to a rule.  A pattern without
// a slash matches at any depth.  A match on a directory covers the files in
// it.
func ignoreRuleFor(line string) (ignoreRule, bool) {
	line = strings.TrimRight(line, " \t\r")
	if line == "" || strings.HasPrefix(line, "#") {
		return ignoreRule{}, false
	}
	rule := ignoreRule{}
	if strings.HasPrefix(line, "!") {
		rule.negate = true
		line = line[1:]
	}
	line = strings.TrimSuffix(strings.ReplaceAll(line, "\\", "/"), "/")
	anchored := strings.Contains(line, "/")
	line = strings.TrimPrefix(line, "/")
	var b strings.Builder
	b.WriteString("^")
	if !anchored {
		b.WriteString("(?:.*/)?")
	}
	for i := 0; i < len(line); i++ {
		switch {
		case strings.HasPrefix(line[i:], "**/"):
			b.WriteString("(?:.*/)?")
			i += 2
		case strings.HasPrefix(line[i:], "**"):
			b.WriteString(".*")
			i++
		case line[i] == '*':
			b.WriteString("[^/]*")
		case line[i] == '?':
			b.WriteString("[^/]")
		default:
			b.WriteString(regexp.QuoteMeta(string(line[i])))
		}
	}
	b.WriteString("(?:/.*)?$")
	pattern, err := regexp.Compile(b.String())
	if err != nil {
		return ignoreRule{}, false
	}
	rule.pattern = pattern
	return rule, true
}

// loadForceIgnore reads <root>/.forceignore.  The default patterns apply
// when the file does not exist.
func loadForceIgnore(root string) *forceIgnore {
	ignore := &forceIgnore{root: root}
	for _, pattern := range defaultIgnorePatterns {
		if rule, ok := ignoreRuleFor(pattern); ok {
			ignore.rules = append(ignore.rules, rule)
		}
	}
	file, err := os.Open(filepath.Join(root, ".forceignore"))
	if err != nil {
		return ignore
	}
	defer file.Close()
	scanner := bufio.NewScanner(file)
	for scanner.Scan() {
		if rule, ok := ignoreRuleFor(scanner.Text()); ok {
			ignore.rules = append(ignore.rules, rule)
		}
	}
	return ignore
}

// ignores reports whether the file at path is ignored.  The last matching
// rule decides.
func (f *forceIgnore) ignores(path string) bool {
	absolute, err := filepath.Abs(path)
	if err != nil {
		return false
	}
	relative, err := filepath.Rel(f.root, absolute)
	if err != nil || strings.HasPrefix(relative, "..") {
		return false
	}
	relative = filepath.ToSlash(relative)
	ignored := false
	for _, rule := range f.rules {
		if rule.pattern.MatchString(relative) {
			ignored = !rule.negate
		}
	}
	return ignored
}

// ignoresComponentFile reports whether a source file is left out: the file
// is ignored, or it is one of a content file and its -meta.xml file and the
// other one is ignored.
func (f *forceIgnore) ignoresComponentFile(path string) bool {
	if f.ignores(path) {
		return true
	}
	if strings.HasSuffix(path, "-meta.xml") {
		return f.ignores(strings.TrimSuffix(path, "-meta.xml"))
	}
	return f.ignores(path + "-meta.xml")
}

// ignoredFiles lists the ignored files under dir, relative to the project
// root.
func (f *forceIgnore) ignoredFiles(dir string) ([]string, error) {
	var files []string
	err := filepath.Walk(dir, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}
		if info.Mode().IsRegular() && f.ignores(path) {
			absolute, _ := filepath.Abs(path)
			relative, _ := filepath.Rel(f.root, absolute)
			files = append(files, filepath.ToSlash(relative))
		}
		return nil
	})
	return files, err
}

// mirrorWithoutIgnored copies dir to a temporary directory, leaving out
// ignored files, and returns the copy.  It returns dir itself when nothing
// under it is ignored.
func (f *forceIgnore) mirrorWithoutIgnored(dir string) (mirror string, cleanup func(), err error) {
	cleanup = func() {}
	anyIgnored := false
	filepath.Walk(dir, func(path string, info os.FileInfo, err error) error {
		if err == nil && info.Mode().IsRegular() && f.ignoresComponentFile(path) {
			anyIgnored = true
		}
		return nil
	})
	if !anyIgnored {
		return dir, cleanup, nil
	}
	mirror, err = os.MkdirTemp("", "sf-forceignore")
	if err != nil {
		return "", cleanup, err
	}
	cleanup = func() { os.RemoveAll(mirror) }
	err = filepath.Walk(dir, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}
		relative, err := filepath.Rel(dir, path)
		if err != nil {
			return err
		}
		target := filepath.Join(mirror, relative)
		if info.IsDir() {
			return os.MkdirAll(target, 0o755)
		}
		if !info.Mode().IsRegular() || f.ignoresComponentFile(path) {
			return nil
		}
		data, err := os.ReadFile(path)
		if err != nil {
			return err
		}
		return os.WriteFile(target, data, 0o644)
	})
	return mirror, cleanup, err
}

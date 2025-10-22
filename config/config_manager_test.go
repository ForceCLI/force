package config

import (
	"fmt"
	"os"
	"path/filepath"
	"testing"
	"time"
)

func TestUseConfigDirectoryWithAbsolutePath(t *testing.T) {
	original := Config
	t.Cleanup(func() { Config = original })

	tempRoot := t.TempDir()
	target := filepath.Join(tempRoot, "custom", "dir")
	if err := UseConfigDirectory(target); err != nil {
		t.Fatalf("UseConfigDirectory returned error: %v", err)
	}

	if Config.GlobalRoot() != target {
		t.Fatalf("expected global root %q, got %q", target, Config.GlobalRoot())
	}

	if info, err := os.Stat(target); err != nil || !info.IsDir() {
		t.Fatalf("expected directory %q to exist", target)
	}
}

func TestUseConfigDirectoryWithTilde(t *testing.T) {
	original := Config
	t.Cleanup(func() { Config = original })

	home, err := os.UserHomeDir()
	if err != nil {
		t.Skip("no home directory available")
	}

	relative := filepath.Join("force-config-tests", fmt.Sprintf("config-%d", time.Now().UnixNano()))
	absolute := filepath.Join(home, relative)
	tildePath := "~/" + filepath.ToSlash(relative)

	if err := UseConfigDirectory(tildePath); err != nil {
		t.Fatalf("UseConfigDirectory returned error: %v", err)
	}

	t.Cleanup(func() { os.RemoveAll(absolute) })

	if Config.GlobalRoot() != absolute {
		t.Fatalf("expected global root %q, got %q", absolute, Config.GlobalRoot())
	}
}

func TestUseConfigBase(t *testing.T) {
	original := Config
	t.Cleanup(func() { Config = original })

	home, err := os.UserHomeDir()
	if err != nil {
		t.Skip("no home directory available")
	}

	base := fmt.Sprintf("force-test-base-%d", time.Now().UnixNano())
	expectedRoot := filepath.Join(home, "."+base)
	t.Cleanup(func() { os.RemoveAll(expectedRoot) })

	UseConfigBase(base)

	if Config.GlobalRoot() != expectedRoot {
		t.Fatalf("expected global root %q, got %q", expectedRoot, Config.GlobalRoot())
	}
}

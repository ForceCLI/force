package command

import (
	"fmt"
	"os"
	"path/filepath"
	"testing"
	"time"

	forceConfig "github.com/ForceCLI/force/config"
)

func TestInitializeConfigWithBaseName(t *testing.T) {
	originalManager := forceConfig.Config
	originalConfigName := configName
	t.Cleanup(func() {
		forceConfig.Config = originalManager
		configName = originalConfigName
	})

	base := fmt.Sprintf("force-test-base-%d", time.Now().UnixNano())
	configName = base
	initializeConfig()

	home, err := os.UserHomeDir()
	if err != nil {
		t.Fatal("expected home directory")
	}
	expected := filepath.Join(home, "."+base)
	if forceConfig.Config.GlobalRoot() != expected {
		t.Fatalf("expected global root %q, got %q", expected, forceConfig.Config.GlobalRoot())
	}

	os.RemoveAll(expected)
}

func TestInitializeConfigWithAbsolutePath(t *testing.T) {
	originalManager := forceConfig.Config
	originalConfigName := configName
	t.Cleanup(func() {
		forceConfig.Config = originalManager
		configName = originalConfigName
	})

	tempDir := t.TempDir()
	target := filepath.Join(tempDir, "custom")
	configName = target
	initializeConfig()

	if forceConfig.Config.GlobalRoot() != target {
		t.Fatalf("expected global root %q, got %q", target, forceConfig.Config.GlobalRoot())
	}

	if info, err := os.Stat(target); err != nil || !info.IsDir() {
		t.Fatalf("expected directory %q to exist", target)
	}
}

func TestInitializeConfigWithRelativePath(t *testing.T) {
	originalManager := forceConfig.Config
	originalConfigName := configName
	originalWD, _ := os.Getwd()
	t.Cleanup(func() {
		forceConfig.Config = originalManager
		configName = originalConfigName
		os.Chdir(originalWD)
	})

	tempRoot := t.TempDir()
	if err := os.Chdir(tempRoot); err != nil {
		t.Fatalf("failed to change directory: %v", err)
	}

	relative := filepath.Join("relative", fmt.Sprintf("config-%d", time.Now().UnixNano()))
	configName = relative
	initializeConfig()

	expected := filepath.Join(tempRoot, relative)
	if forceConfig.Config.GlobalRoot() != expected {
		t.Fatalf("expected global root %q, got %q", expected, forceConfig.Config.GlobalRoot())
	}

	os.RemoveAll(expected)
}

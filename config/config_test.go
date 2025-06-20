package config

import (
	"os"
	"path/filepath"
	"testing"
)

func TestGetSourceDirConsistency(t *testing.T) {
	// Create a temporary directory for testing
	tempDir, err := os.MkdirTemp("", "force-test")
	if err != nil {
		t.Fatalf("Failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tempDir)

	// Change to temp directory
	originalDir, err := os.Getwd()
	if err != nil {
		t.Fatalf("Failed to get current dir: %v", err)
	}
	defer os.Chdir(originalDir)

	err = os.Chdir(tempDir)
	if err != nil {
		t.Fatalf("Failed to change to temp dir: %v", err)
	}

	// Test 1: First call should create src directory and return it
	firstCall, err := GetSourceDir()
	if err != nil {
		t.Fatalf("First GetSourceDir call failed: %v", err)
	}

	expectedFirst := filepath.Join(tempDir, "src")
	// Handle macOS path resolution differences
	expectedResolved, _ := filepath.EvalSymlinks(expectedFirst)
	actualResolved, _ := filepath.EvalSymlinks(firstCall)
	if actualResolved != expectedResolved {
		t.Errorf("First call: expected %s (resolved: %s), got %s (resolved: %s)",
			expectedFirst, expectedResolved, firstCall, actualResolved)
	}

	// Verify src directory was created
	srcDir := filepath.Join(tempDir, "src")
	if !IsSourceDir(srcDir) {
		t.Error("src directory was not created")
	}

	// Verify no metadata directory was created automatically
	metadataDir := filepath.Join(tempDir, "metadata")
	if IsSourceDir(metadataDir) {
		t.Error("metadata directory should not have been created automatically")
	}

	// Test 2: Second call should return the same directory consistently (src)
	secondCall, err := GetSourceDir()
	if err != nil {
		t.Fatalf("Second GetSourceDir call failed: %v", err)
	}

	if firstCall != secondCall {
		t.Errorf("Inconsistent behavior: first call returned %s, second call returned %s", firstCall, secondCall)
	}

	// Test 3: Multiple calls should all return the same directory (src)
	for i := 0; i < 5; i++ {
		call, err := GetSourceDir()
		if err != nil {
			t.Fatalf("GetSourceDir call %d failed: %v", i+3, err)
		}
		if call != firstCall {
			t.Errorf("Call %d: expected %s, got %s", i+3, firstCall, call)
		}
	}
}

func TestGetSourceDirWithExistingMetadataDirectory(t *testing.T) {
	// Create a temporary directory for testing
	tempDir, err := os.MkdirTemp("", "force-test")
	if err != nil {
		t.Fatalf("Failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tempDir)

	// Change to temp directory
	originalDir, err := os.Getwd()
	if err != nil {
		t.Fatalf("Failed to get current dir: %v", err)
	}
	defer os.Chdir(originalDir)

	err = os.Chdir(tempDir)
	if err != nil {
		t.Fatalf("Failed to change to temp dir: %v", err)
	}

	// Pre-create a metadata directory (not a symlink)
	metadataDir := filepath.Join(tempDir, "metadata")
	err = os.Mkdir(metadataDir, 0755)
	if err != nil {
		t.Fatalf("Failed to create metadata dir: %v", err)
	}

	// GetSourceDir should return the existing metadata directory since src doesn't exist
	result, err := GetSourceDir()
	if err != nil {
		t.Fatalf("GetSourceDir failed: %v", err)
	}

	// Handle macOS path resolution differences
	expectedResolved, _ := filepath.EvalSymlinks(metadataDir)
	actualResolved, _ := filepath.EvalSymlinks(result)
	if actualResolved != expectedResolved {
		t.Errorf("Expected %s (resolved: %s), got %s (resolved: %s)",
			metadataDir, expectedResolved, result, actualResolved)
	}
}

func TestGetSourceDirWithExistingSrcDirectory(t *testing.T) {
	// Create a temporary directory for testing
	tempDir, err := os.MkdirTemp("", "force-test")
	if err != nil {
		t.Fatalf("Failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tempDir)

	// Change to temp directory
	originalDir, err := os.Getwd()
	if err != nil {
		t.Fatalf("Failed to get current dir: %v", err)
	}
	defer os.Chdir(originalDir)

	err = os.Chdir(tempDir)
	if err != nil {
		t.Fatalf("Failed to change to temp dir: %v", err)
	}

	// Pre-create only a src directory (no metadata)
	srcDir := filepath.Join(tempDir, "src")
	err = os.Mkdir(srcDir, 0755)
	if err != nil {
		t.Fatalf("Failed to create src dir: %v", err)
	}

	// GetSourceDir should return the existing src directory since it's first in priority
	result, err := GetSourceDir()
	if err != nil {
		t.Fatalf("GetSourceDir failed: %v", err)
	}

	// Handle macOS path resolution differences
	expectedResolved, _ := filepath.EvalSymlinks(srcDir)
	actualResolved, _ := filepath.EvalSymlinks(result)
	if actualResolved != expectedResolved {
		t.Errorf("Expected %s (resolved: %s), got %s (resolved: %s)",
			srcDir, expectedResolved, result, actualResolved)
	}
}

func TestGetSourceDirWithBothDirectories(t *testing.T) {
	// Create a temporary directory for testing
	tempDir, err := os.MkdirTemp("", "force-test")
	if err != nil {
		t.Fatalf("Failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tempDir)

	// Change to temp directory
	originalDir, err := os.Getwd()
	if err != nil {
		t.Fatalf("Failed to get current dir: %v", err)
	}
	defer os.Chdir(originalDir)

	err = os.Chdir(tempDir)
	if err != nil {
		t.Fatalf("Failed to change to temp dir: %v", err)
	}

	// Create both src and metadata directories
	srcDir := filepath.Join(tempDir, "src")
	err = os.Mkdir(srcDir, 0755)
	if err != nil {
		t.Fatalf("Failed to create src dir: %v", err)
	}

	metadataDir := filepath.Join(tempDir, "metadata")
	err = os.Mkdir(metadataDir, 0755)
	if err != nil {
		t.Fatalf("Failed to create metadata dir: %v", err)
	}

	// GetSourceDir should prefer src when both exist (since src is first in sourceDirs)
	result, err := GetSourceDir()
	if err != nil {
		t.Fatalf("GetSourceDir failed: %v", err)
	}

	// Handle macOS path resolution differences
	expectedResolved, _ := filepath.EvalSymlinks(srcDir)
	actualResolved, _ := filepath.EvalSymlinks(result)
	if actualResolved != expectedResolved {
		t.Errorf("Expected %s (src should be preferred, resolved: %s), got %s (resolved: %s)",
			srcDir, expectedResolved, result, actualResolved)
	}
}

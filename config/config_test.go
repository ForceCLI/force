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

	// Test 1: First call should create directories and return metadata symlink
	firstCall, err := GetSourceDir()
	if err != nil {
		t.Fatalf("First GetSourceDir call failed: %v", err)
	}

	expectedFirst := filepath.Join(tempDir, "metadata")
	// Handle macOS path resolution differences
	expectedFirstResolved, _ := filepath.EvalSymlinks(expectedFirst)
	firstCallResolved, _ := filepath.EvalSymlinks(firstCall)
	if firstCallResolved != expectedFirstResolved && firstCall != expectedFirst {
		t.Errorf("First call: expected %s (resolved: %s), got %s (resolved: %s)", 
			expectedFirst, expectedFirstResolved, firstCall, firstCallResolved)
	}

	// Verify directories were created
	srcDir := filepath.Join(tempDir, "src")
	metadataDir := filepath.Join(tempDir, "metadata")

	if !IsSourceDir(srcDir) {
		t.Error("src directory was not created")
	}

	if !IsSourceDir(metadataDir) {
		t.Error("metadata directory was not created")
	}

	// Verify metadata is a symlink to src
	if stat, err := os.Lstat(metadataDir); err != nil {
		t.Errorf("Failed to stat metadata dir: %v", err)
	} else if stat.Mode()&os.ModeSymlink == 0 {
		t.Error("metadata should be a symlink")
	}

	// Test 2: Second call should return the same directory consistently (metadata symlink)
	secondCall, err := GetSourceDir()
	if err != nil {
		t.Fatalf("Second GetSourceDir call failed: %v", err)
	}

	if firstCall != secondCall {
		t.Errorf("Inconsistent behavior: first call returned %s, second call returned %s", firstCall, secondCall)
	}

	// Test 3: Multiple calls should all return the same directory (metadata symlink)
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

	// GetSourceDir should return the existing metadata directory
	result, err := GetSourceDir()
	if err != nil {
		t.Fatalf("GetSourceDir failed: %v", err)
	}

	// Handle macOS path resolution differences
	resultResolved, _ := filepath.EvalSymlinks(result)
	metadataDirResolved, _ := filepath.EvalSymlinks(metadataDir)
	if resultResolved != metadataDirResolved && result != metadataDir {
		t.Errorf("Expected %s (resolved: %s), got %s (resolved: %s)", 
			metadataDir, metadataDirResolved, result, resultResolved)
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

	// GetSourceDir should return the existing src directory since metadata doesn't exist
	result, err := GetSourceDir()
	if err != nil {
		t.Fatalf("GetSourceDir failed: %v", err)
	}

	// Handle macOS path resolution differences
	resultResolved, _ := filepath.EvalSymlinks(result)
	srcDirResolved, _ := filepath.EvalSymlinks(srcDir)
	if resultResolved != srcDirResolved && result != srcDir {
		t.Errorf("Expected %s (resolved: %s), got %s (resolved: %s)", 
			srcDir, srcDirResolved, result, resultResolved)
	}
}
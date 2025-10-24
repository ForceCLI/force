package desktop

import (
	"os"
	"runtime"
	"testing"
)

func TestIsWSL(t *testing.T) {
	// Save original values
	originalWSL := os.Getenv("WSL_DISTRO_NAME")

	// Test 1: Non-Linux system should return false
	if runtime.GOOS != "linux" {
		if isWSL() {
			t.Error("isWSL() should return false on non-Linux systems")
		}
	}

	// Test 2: WSL environment variable detection
	if runtime.GOOS == "linux" {
		// Set WSL environment variable
		os.Setenv("WSL_DISTRO_NAME", "Ubuntu")
		if !isWSL() {
			t.Error("isWSL() should return true when WSL_DISTRO_NAME is set")
		}

		// Restore original value
		if originalWSL != "" {
			os.Setenv("WSL_DISTRO_NAME", originalWSL)
		} else {
			os.Unsetenv("WSL_DISTRO_NAME")
		}
	}
}

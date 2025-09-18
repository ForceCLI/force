package desktop

// taken from https://bitbucket.org/tebeka/go-wise/src/tip/desktop.go

import (
	"fmt"
	"os"
	"os/exec"
	"runtime"
	"strings"
)

var openCommands = map[string][]string{
	"windows": []string{"cmd", "/c", "start"},
	"darwin":  []string{"open"},
	"linux":   []string{"xdg-open"},
}

func isWSL() bool {
	if runtime.GOOS != "linux" {
		return false
	}

	// Check for WSL-specific environment variable
	if os.Getenv("WSL_DISTRO_NAME") != "" {
		return true
	}

	// Check if /proc/version contains Microsoft or WSL
	data, err := os.ReadFile("/proc/version")
	if err != nil {
		return false
	}

	version := strings.ToLower(string(data))
	return strings.Contains(version, "microsoft") || strings.Contains(version, "wsl")
}

func Open(uri string) error {
	// Special handling for WSL
	if isWSL() {
		// Use Windows' cmd.exe to open the browser from WSL
		cmd := exec.Command("cmd.exe", "/c", "start", strings.Replace(uri, "&", "^&", -1))
		return cmd.Start()
	}

	run, ok := openCommands[runtime.GOOS]
	if !ok {
		return fmt.Errorf("don't know how to open things on %s platform", runtime.GOOS)
	}
	if runtime.GOOS == "windows" {
		uri = strings.Replace(uri, "&", "^&", -1)
	}
	run = append(run, uri)
	cmd := exec.Command(run[0], run[1:]...)
	return cmd.Start()
}

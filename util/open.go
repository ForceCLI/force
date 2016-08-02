package util

// taken from https://bitbucket.org/tebeka/go-wise/src/tip/desktop.go

import (
	"fmt"
	"os/exec"
	"runtime"
	"strings"
)

var openCommands = map[string][]string{
	"windows": []string{"cmd", "/c", "start"},
	"darwin":  []string{"open"},
	"linux":   []string{"xdg-open"},
}

func Open(uri string) error {
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

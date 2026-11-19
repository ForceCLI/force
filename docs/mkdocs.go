package main

import (
	"os"

	cmd "github.com/ForceCLI/force/command"
	"github.com/ForceCLI/force/sf"

	"github.com/spf13/cobra/doc"
)

func main() {
	cmd.RootCmd.DisableAutoGenTag = true
	err := doc.GenMarkdownTree(cmd.RootCmd, "./docs")
	if err != nil {
		panic(err.Error())
	}

	// The command tree used when the binary is invoked as sf.
	sfRoot := sf.NewRootCmd()
	sfRoot.DisableAutoGenTag = true
	if err := os.MkdirAll("./docs/sf", 0o755); err != nil {
		panic(err.Error())
	}
	if err := doc.GenMarkdownTree(sfRoot, "./docs/sf"); err != nil {
		panic(err.Error())
	}
}

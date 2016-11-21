package main

import (
	"fmt"
	"os"

	"github.com/bgentry/speakeasy"
)

func main() {
	password, err := speakeasy.Ask("Please enter a password: ")
	if err != nil {
		ConsolePrintln(err)
		os.Exit(1)
	}
	ConsolePrintf("Password result: %q\n", password)
	ConsolePrintf("Password len: %d\n", len(password))
}

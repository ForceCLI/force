package main

import (
	"fmt"
	"os"
)

func ErrorAndExit(format string, args ...interface{}) {
	fmt.Printf(fmt.Sprintf("ERROR: %s\n", format), args...)
	os.Exit(1)
}

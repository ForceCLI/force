package main

import (
	"fmt"
	"os"
)

const (
	LF = 10
)

func ErrorAndExit(format string, args ...interface{}) {
	if format[0] == LF {
		fmt.Printf(format[1:], args...)
	} else {
		fmt.Printf(fmt.Sprintf("ERROR: %s\n", format), args...)
	}
	os.Exit(1)
}

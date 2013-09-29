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
		fmt.Errorf(format[1:]+"\n", args...)
	} else {
		fmt.Errorf(fmt.Sprintf("ERROR: %s\n", format), args...)
	}
	os.Exit(1)
}

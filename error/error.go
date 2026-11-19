package error

import (
	"fmt"
	"os"
)

const (
	LF = 10
)

// ExitHandler, when set, receives the formatted message from ErrorAndExit in
// place of the default "ERROR: " line on stderr.  The process exits with
// status 1 after the handler returns.
var ExitHandler func(message string)

func ErrorAndExit(format string, args ...interface{}) {
	if ExitHandler != nil {
		if len(format) > 0 && format[0] == LF {
			format = format[1:]
		}
		ExitHandler(fmt.Sprintf(format, args...))
		os.Exit(1)
	}
	if format[0] == LF {
		fmt.Fprintf(os.Stderr, format[1:]+"\n", args...)
	} else {
		fmt.Fprintf(os.Stderr, fmt.Sprintf("ERROR: %s\n", format), args...)
	}
	os.Exit(1)
}

func ExitIfError(err error, format string, args ...interface{}) {
	if err != nil {
		ErrorAndExit(format, args...)
	}
}

func ExitIfNoSourceDir(err error) {
	if err != nil {
		if os.IsNotExist(err) {
			ErrorAndExit("Current directory does not contain a metadata or src directory")
		}

		ErrorAndExit(err.Error())
	}
}

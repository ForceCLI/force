package lib

import (
	"fmt"
	"os"
)

type Logger interface {
	Info(...interface{})
}

var Log Logger

func init() {
	Log = defaultLogger{}
}

type defaultLogger struct{}

func (l defaultLogger) Info(args ...interface{}) {
	fmt.Fprintln(os.Stderr, args...)
}

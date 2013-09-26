package main

import (
	"fmt"
	"os"
)

func ErrorAndExit(message string) {
	fmt.Printf("ERROR: %s\n", message)
	os.Exit(1)
}

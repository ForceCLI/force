package main

import (
	"os"
	"path/filepath"

	"github.com/ddollar/config"
)

var Config = config.NewConfig("force")

func GetSourceDir() (src string, err error) {
	// Last element is default
	var sourceDirs = []string{
		"src",
		"metadata",
	}

	wd, _ := os.Getwd()
	for _, src = range sourceDirs {
		_, err = os.Stat(filepath.Join(wd, src, "package.xml"))
		// Found a real source dir
		if err == nil {
			return
		}
	}

	return
}

func ExitIfNoSourceDir(err error) {
	if err != nil {
		if os.IsNotExist(err) {
			ErrorAndExit("Current directory does not contain a metadata or src directory")
		}

		ErrorAndExit(err.Error())
	}
}

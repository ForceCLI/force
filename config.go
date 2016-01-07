package main

import (
	"github.com/devangel/config"
	"os"
	"path/filepath"
	"strings"
	//"fmt"
)

var Config = config.NewConfig("force")

func GetSourceDir() (src string, err error) {
	// Last element is default
	var sourceDirs = []string{
		//"src",
		"metadata",
	}

	wd, err := os.Getwd()

	err = nil
	for _, src = range sourceDirs {
		if strings.Contains(wd, src) {
			// our working directory contains a src dir above us, we need to move up the file syste
			nsrc := wd
			for {
				nsrc = filepath.Dir(nsrc)
				if filepath.Base(nsrc) == src {
					src = nsrc
					return
				}
			}
		} else {
			_, err = os.Stat(filepath.Join(wd, src)) //, "package.xml"))
			// Found a real source dir
			if err == nil {
				return
			}
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

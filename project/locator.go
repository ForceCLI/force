package project

import (
	"os"
	"path/filepath"
	"strings"

	"github.com/heroku/force/util"
)

// GetSourceDir walks up the given path (typically the pwd of the running `force` process) to
// determine the context of the project the tool is being run in (similar to the behaviour of git
// and other tools).  In the typical case, pass err to ExitIfNoSourceDir() to render an error
// message and quit if necessary.
func GetSourceDir() (src string, err error) {
	// Last element is default
	var sourceDirs = []string{
		"src",
		"metadata",
	}

	wd, err := os.Getwd()

	err = nil
	for _, src = range sourceDirs {
		if strings.Contains(wd, src) {
			// our working directory contains a src dir above us, we need to move up the file system.
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

// ExitIfNoSourceDir takes a possible error returned by GetSourceDir() and renders an error message
// and quits if necessary.
func ExitIfNoSourceDir(err error) {
	if err != nil {
		if os.IsNotExist(err) {
			util.ErrorAndExit("Current directory does not contain a metadata or src directory")
		}

		util.ErrorAndExit(err.Error())
	}
}

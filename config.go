package main

import (
	"os"
	"path/filepath"

	"github.com/devangel/config"
)

var Config = config.NewConfig("force")

var sourceDirs = []string{
	"src",
	"metadata",
}

// IsSourceDir returns a boolean indicating that dir is actually a Salesforce
// source directory.
func IsSourceDir(dir string) bool {
	for _, src := range sourceDirs {
		// Look for src properly for case-insensitive filesystems
		manifest := filepath.Join(filepath.Dir(dir), src, "package.xml")
		if _, err := os.Stat(manifest); err == nil {
			return true
		}
	}
	return false
}

// GetSourceDir returns a rooted path name of the Salesforce source directory,
// relative to the current directory. GetSourceDir will look for a source
// directory in the nearest subdirectory. If no such directory exists, it will
// look at its parents, assuming that it is within a source directory already.
func GetSourceDir() (dir string, err error) {
	base, err := os.Getwd()
	if err != nil {
		return
	}

	// Look down to our nearest subdirectories
	for _, src := range sourceDirs {
		dir = filepath.Join(base, src)
		if IsSourceDir(dir) {
			return
		}
	}

	// Check the current directory and then start looking up at our parents.
	// When dir's parent is identical, it means we're at the root
	for dir != filepath.Dir(dir) {
		dir = filepath.Dir(dir)
		if IsSourceDir(dir) {
			return
		}
	}

	// Not a source directory
	return "", os.ErrNotExist
}

func ExitIfNoSourceDir(err error) {
	if err != nil {
		if os.IsNotExist(err) {
			ErrorAndExit("Current directory does not contain a metadata or src directory")
		}

		ErrorAndExit(err.Error())
	}
}

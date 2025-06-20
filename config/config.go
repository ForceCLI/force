package config

import (
	"os"
	"path/filepath"

	"github.com/ForceCLI/config"
)

var Config = config.NewConfig("force")

var sourceDirs = []string{
	"src",
	"metadata",
}

// IsSourceDir returns a boolean indicating that dir is actually a Salesforce
// source directory.
func IsSourceDir(dir string) bool {
	if _, err := os.Stat(dir); err == nil {
		return true
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
	// Special case: if both src and metadata exist, and metadata is a symlink to src,
	// prefer metadata for consistency with the creation behavior
	srcDir := filepath.Join(base, "src")
	metadataDir := filepath.Join(base, "metadata")
	
	if IsSourceDir(srcDir) && IsSourceDir(metadataDir) {
		// Check if metadata is a symlink to src
		if stat, statErr := os.Lstat(metadataDir); statErr == nil && stat.Mode()&os.ModeSymlink != 0 {
			if target, linkErr := os.Readlink(metadataDir); linkErr == nil {
				// Resolve both paths to handle symlinks and path variations
				var targetResolved, srcResolved string
				var resolveErr error
				
				if filepath.IsAbs(target) {
					targetResolved, resolveErr = filepath.EvalSymlinks(target)
				} else {
					targetResolved, resolveErr = filepath.EvalSymlinks(filepath.Join(filepath.Dir(metadataDir), target))
				}
				
				if resolveErr == nil {
					if srcResolved, resolveErr = filepath.EvalSymlinks(srcDir); resolveErr == nil {
						if targetResolved == srcResolved {
							// metadata is a symlink to src, use metadata for consistency
							dir = metadataDir
							return
						}
					}
				}
			}
		}
	}
	
	// Normal directory search
	for _, src := range sourceDirs {
		if len(src) > 0 {
			dir = filepath.Join(base, src)
			if IsSourceDir(dir) {
				return
			}
		}
	}

	// Check the current directory and then start looking up at our parents.
	// When dir's parent is identical, it means we're at the root.  If we blow
	// past the actual root, we should drop to the next section of code
	for dir != filepath.Dir(dir) {
		dir = filepath.Dir(dir)
		for _, src := range sourceDirs {
			adir := filepath.Join(dir, src)
			if IsSourceDir(adir) {
				dir = adir
				return
			}
		}
	}

	// No source directory found, create a src directory and a symlinked "metadata"
	// directory for backward compatibility and return that.
	dir = filepath.Join(base, "src")
	err = os.Mkdir(dir, 0777)
	symlink := filepath.Join(base, "metadata")
	os.Symlink(dir, symlink)
	dir = symlink
	return
}

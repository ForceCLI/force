package project

import (
	"fmt"
	"io/ioutil"
	"os"
	"os/user"
	"path/filepath"
	"strings"

	util "github.com/heroku/force/util"
)

type project struct {
	path string

	// Lazily loaded project contents.
	// file path -> file contents
	lazyProjectContents *map[string][]byte
}

// LoadProject loads the entire project and its config data in from the filesystem,
// but note that it does so lazily.
func LoadProject(directory string) *project {
	newProject := project{
		path: determineProjectPath(directory),
	}
	return &newProject
}

func determineProjectPath(directory string) string {
	wd, _ := os.Getwd()
	usr, err := user.Current()
	var dir string

	//Manually handle shell expansion short cut
	if err != nil {
		if strings.HasPrefix(directory, "~") {
			util.ErrorAndExit("Cannot determine tilde expansion, please use relative or absolute path to directory.")
		} else {
			dir = directory
		}
	} else {
		if strings.HasPrefix(directory, "~") {
			dir = strings.Replace(directory, "~", usr.HomeDir, 1)
		} else {
			dir = directory
		}
	}

	root := filepath.Join(wd, dir)

	// Check for absolute path
	if filepath.IsAbs(dir) {
		root = dir
	}

	if _, err := os.Stat(filepath.Join(root, "package.xml")); os.IsNotExist(err) {
		util.ErrorAndExit(" \n" + filepath.Join(root, "package.xml") + "\ndoes not exist")
	}

	return root
}

func (project *project) LoadedFromPath() string {
	return project.path
}

// ContentsWithInternalTransformsApplied will give you all of the project contents with any project-specific transforms applied.
// Note that "External" transforms, which for instance depend on the state of the current environment,
// are applied elsewhere.
func (project *project) ContentsWithInternalTransformsApplied(environmentConfig *EnvironmentConfigJSON) map[string][]byte {
	transformedContents := project.EnumerateContents()

	// first transform: string interpolation of the vars in the config:
	for name, contents := range transformedContents {
		contentsUnderProcessing := string(contents)
		for placeholder, value := range environmentConfig.Variables {
			token := fmt.Sprintf("$%s", placeholder)
			contentsUnderProcessing = strings.Replace(contentsUnderProcessing, token, value, -1)
		}
		// it's safe to replace the value in the map!
		transformedContents[name] = []byte(contentsUnderProcessing)
	}

	return transformedContents
}

// EnumerateContents enumerates all of the Salesforce metadata files in the project directory
// and loads them into memory, and returns them.  However, note that all relevant
// transforms specified in the project configuration are not yet applied.  RunImport
// uses ContentsWithAllTransformsApplied() for this task.
//
// Note that it only enumerates the filesystem once, and memoizes the result.
func (project *project) EnumerateContents() (enumeratedContents map[string][]byte) {
	root := project.path

	// compute and memoize as needed:
	if project.lazyProjectContents == nil {
		files := make(map[string][]byte)

		err := filepath.Walk(root, func(path string, f os.FileInfo, err error) error {
			if f.Mode().IsRegular() {
				if f.Name() != ".DS_Store" {
					data, err := ioutil.ReadFile(path)
					if err != nil {
						util.ErrorAndExit(err.Error())
					}
					files[strings.Replace(path, fmt.Sprintf("%s%s", root, string(os.PathSeparator)), "", -1)] = data
				}
			}
			return nil
		})
		if err != nil {
			util.ErrorAndExit(err.Error())
		}

		project.lazyProjectContents = &files
	}

	// we return a copy of the memoized data so consumers can't mutate state in the Project
	// unexpectedly.
	enumeratedContents = make(map[string][]byte)
	for key, value := range *project.lazyProjectContents {
		enumeratedContents[key] = value
	}
	return
}

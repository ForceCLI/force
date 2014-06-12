package main

import (
	"fmt"
	"io/ioutil"
	"os"
	"path/filepath"
	"strings"
)

var cmdPush = &Command{
	Run:   runPush,
	Usage: "push path name",
	Short: "Deploy single artifact from a local directory",
	Long: `
Deploy single artifact from a local directory

Examples:

  force push connectedApps name
`,
}

var pxml = `<?xml version="1.0" encoding="UTF-8"?>
<Package xmlns="http://soap.sforce.com/2006/04/metadata">
    <types>
        <members>%s</members>
        <name>%s</name>
    </types>
    <version>29.0</version>
</Package>`

type metapath struct {
    path string
    name string
}

var metapaths = []metapath{
    metapath{"classes"      , "ApexClass"},
    metapath{"objects"      , "CustomObject"},
    metapath{"tabs"         , "CustomTab"},
    metapath{"flexipages"   , "FlexiPage"},
    metapath{"components"   , "ApexComponent"},
    metapath{"triggers"     , "ApexTrigger"},
    metapath{"pages"        , "ApexPage"},
}

func getPathForMeta(metaname string) string {
    for _, mp := range metapaths {
        if mp.name == metaname {
            return mp.path
        }
    }

    // Unknown, so use metaname
    return metaname
}

func getMetaForPath(path string) string {
    for _, mp := range metapaths {
        if mp.path == path {
            return mp.name
        }
    }

    // Unknown, so use path
    return path
}

func runPush(cmd *Command, args []string) {
	if len(args) == 0 {
		cmd.printUsage()
		return
	}

    if len(args) == 1 {
        pushByPath(args[0])
    }

	if len(args) == 2 {
        pushByName(args)
    }
}

func pushByName(args []string) {
	wd, _ := os.Getwd()
	root := filepath.Join(wd, "metadata")
	//if len(args) == 1 {
	//	root, _ = filepath.Abs(args[0])
	//}

	if _, err := os.Stat(filepath.Join(root, "package.xml")); os.IsNotExist(err) {
        root = filepath.Join(wd, "src")
        if _, err := os.Stat(filepath.Join(root, "package.xml")); os.IsNotExist(err) {
            ErrorAndExit("Must specify a directory that contains metadata files")
        }
	}

	if _, err := os.Stat(filepath.Join(root, args[0])); os.IsNotExist(err) {
		ErrorAndExit("Folder " + args[0] + " not found, must specify a metadata folder")
	}

    objType := getMetaForPath(args[0])

	found := false
	err := filepath.Walk(filepath.Join(root, args[0]), func(path string, f os.FileInfo, err error) error {
		if f.Mode().IsRegular() {
            fname := strings.ToLower(f.Name())
            fname = strings.TrimSuffix(fname, filepath.Ext(fname))
			if strings.ToLower(fname) == strings.ToLower(args[1]) {
				found = true
			}
		}
		return nil
	})
	if err != nil {
		ErrorAndExit(err.Error())
	}
	if !found {
		ErrorAndExit("Could not find " + args[1] + " in " + args[0])
	}

	files := make(ForceMetadataFiles)

	err = os.Rename(filepath.Join(root, "package.xml"), filepath.Join(root, "package.copy.xml"))

	if err := ioutil.WriteFile(filepath.Join(root, "package.xml"), []byte(fmt.Sprintf(pxml, args[1], objType)), 0644); err != nil {
		ErrorAndExit(err.Error())
	}
	if err != nil {
		ErrorAndExit(err.Error())
	}

	err = filepath.Walk(root, func(path string, f os.FileInfo, err error) error {
		if f.Mode().IsRegular() {
            fname := strings.ToLower(f.Name())
            fname = strings.TrimSuffix(fname, filepath.Ext(fname))
			if fname == strings.ToLower(args[1]) {
				data, err := ioutil.ReadFile(path)
				if err != nil {
					ErrorAndExit(err.Error())
				}
				files[strings.Replace(path, fmt.Sprintf("%s/", root), "", -1)] = data

                path += "-meta.xml"
				data, err = ioutil.ReadFile(path)
				if err != nil {
					ErrorAndExit(err.Error())
				}
				files[strings.Replace(path, fmt.Sprintf("%s/", root), "", -1)] = data
			}
            if f.Name() == "package.xml" {
				data, err := ioutil.ReadFile(path)
				if err != nil {
					ErrorAndExit(err.Error())
				}
				files[strings.Replace(path, fmt.Sprintf("%s/", root), "", -1)] = data
            }
		}
		return nil
	})
	if err != nil {
		ErrorAndExit(err.Error())
	}

    deployFiles(files)

	fmt.Printf("Pushed %s to Force.com\n", args[1])
}

// Push metadata object by path to a file
func pushByPath(fpath string) {
    fpath, err := filepath.Abs(fpath)
    if err != nil {
        ErrorAndExit("Cound not find " + fpath)
    }
    if _, err := os.Stat(fpath); err != nil {
        ErrorAndExit("Cound not open " + fpath)
    }

    hasMeta := true
    fname := filepath.Base(fpath)
    fname = strings.TrimSuffix(fname, filepath.Ext(fname))
    fdir := filepath.Dir(fpath)
    typePath := filepath.Base(fdir)
    srcDir := filepath.Dir(fdir)
    metaType := getMetaForPath(typePath)
    // Should be present since we worked back to srcDir
    frel, _ := filepath.Rel(srcDir, fpath)

    // Try to find meta file
    fmeta := fpath + "-meta.xml"
    fmetarel := ""
    if _, err := os.Stat(fmeta); err != nil {
        if os.IsNotExist(err) {
            hasMeta = false
        } else {
            ErrorAndExit("Cound not open " + fmeta)
        }
    } else {
        // Should be present since we worked back to srcDir
        fmetarel, _ = filepath.Rel(srcDir, fmeta)
    }

    // DEBUG
    // fmt.Println("Dir: " + fdir)
    // fmt.Println("fname: " + fname)
    // fmt.Println("Dir end: " + typePath)
    // fmt.Println("srcDir: " + srcDir)
    // fmt.Println("Type: " + metaType)
    // fmt.Println("relPath: " + frel)

	files := make(ForceMetadataFiles)

    fdata, err := ioutil.ReadFile(fpath)
    files[frel] = fdata
    // IF META EXISTS
    if hasMeta {
        fdata, err = ioutil.ReadFile(fmeta)
        files[fmetarel] = fdata
    }

    files["package.xml"] = []byte(fmt.Sprintf(pxml, fname, metaType))

    deployFiles(files)
}

func deployFiles(files ForceMetadataFiles) {
	force, _ := ActiveForce()
	var DeploymentOptions ForceDeployOptions
	successes, problems, err := force.Metadata.Deploy(files, DeploymentOptions)
	if err != nil {
		ErrorAndExit(err.Error())
	}
	fmt.Printf("\nFailures - %d\n", len(problems))
	for _, problem := range problems {
		if problem.FullName == "" {
			fmt.Println(problem.Problem)
		} else {
			fmt.Printf("ERROR with %s:\n %s\n", problem.FullName, problem.Problem)
		}
	}

	fmt.Printf("\nSuccesses - %d\n", len(successes))
	for _, success := range successes {
		if success.FullName != "package.xml" {
			verb := "unchanged"
			if success.Changed {
				verb = "changed"
			} else if success.Deleted {
				verb = "deleted"
			} else if success.Created {
				verb = "created"
			}
			fmt.Printf("%s\n\tstatus: %s\n\tid=%s\n", success.FullName, verb, success.Id)
		}
	}

}

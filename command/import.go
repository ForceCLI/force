package command

import (
	"errors"
	"fmt"
	"io/ioutil"
	"os"
	"os/user"
	"path/filepath"
	"slices"
	"sort"
	"strings"
	"sync"

	. "github.com/ForceCLI/force/error"
	. "github.com/ForceCLI/force/lib"

	"github.com/antlr4-go/antlr/v4"
	"github.com/octoberswimmer/apexfmt/parser"
	"github.com/spf13/cobra"
)

var importCmd = &cobra.Command{
	Use:   "import",
	Short: "Import metadata from a local directory",
	Example: `
  force import
  force import -directory=my_metadata -c -r -v
  force import -checkonly -runalltests
`,
	Run: func(cmd *cobra.Command, args []string) {
		options := getDeploymentOptions(cmd)
		srcDir := sourceDir(cmd)

		displayOptions := getDeploymentOutputOptions(cmd)

		runImport(srcDir, options, displayOptions)
	},
	Args: cobra.MaximumNArgs(0),
}

func init() {
	// Deploy options
	importCmd.Flags().BoolP("rollbackonerror", "r", false, "roll back deployment on error")
	importCmd.Flags().BoolP("runalltests", "t", false, "run all tests (equivalent to --testlevel RunAllTestsInOrg)")
	importCmd.Flags().StringP("testlevel", "l", "NoTestRun", "test level")
	importCmd.Flags().BoolP("checkonly", "c", false, "check only deploy")
	importCmd.Flags().BoolP("purgeondelete", "p", false, "purge metadata from org on delete")
	importCmd.Flags().BoolP("allowmissingfiles", "m", false, "set allow missing files")
	importCmd.Flags().BoolP("autoupdatepackage", "u", false, "set auto update package")
	importCmd.Flags().BoolP("ignorewarnings", "i", false, "ignore warnings")
	importCmd.Flags().BoolP("splittests", "s", false, "split tests between deployments")
	importCmd.Flags().StringSliceP("test", "", []string{}, "test(s) to run")

	// Output options
	importCmd.Flags().BoolP("ignorecoverage", "w", false, "suppress code coverage warnings")
	importCmd.Flags().BoolP("suppressunexpected", "U", true, `suppress "An unexpected error occurred" messages`)
	importCmd.Flags().BoolP("quiet", "q", false, "only output failures")
	importCmd.Flags().BoolP("interactive", "I", false, "interactive mode")
	importCmd.Flags().CountP("verbose", "v", "give more verbose output")
	importCmd.Flags().StringP("reporttype", "f", "text", "report type format (text or junit)")

	importCmd.Flags().StringP("directory", "d", "src", "relative path to package.xml")

	importCmd.Flags().BoolP("erroronfailure", "E", true, "exit with an error code if any tests fail")

	RootCmd.AddCommand(importCmd)
}

func sourceDir(cmd *cobra.Command) string {
	directory, _ := cmd.Flags().GetString("directory")

	wd, _ := os.Getwd()
	var dir string
	usr, err := user.Current()

	//Manually handle shell expansion short cut
	if err != nil {
		if strings.HasPrefix(directory, "~") {
			ErrorAndExit("Cannot determine tilde expansion, please use relative or absolute path to directory.")
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
	return root
}

func runImport(root string, baseOptions ForceDeployOptions, displayOptions *deployOutputOptions) {
	if displayOptions.quiet {
		previousLogger := Log
		var l quietLogger
		Log = l
		defer func() {
			Log = previousLogger
		}()
	}
	files := make(ForceMetadataFiles)
	if _, err := os.Stat(filepath.Join(root, "package.xml")); os.IsNotExist(err) {
		ErrorAndExit(" \n" + filepath.Join(root, "package.xml") + "\ndoes not exist")
	}

	err := filepath.Walk(root, func(path string, f os.FileInfo, err error) error {
		if f.Mode().IsRegular() {
			if f.Name() != ".DS_Store" {
				data, err := ioutil.ReadFile(path)
				if err != nil {
					ErrorAndExit(err.Error())
				}
				files[strings.Replace(path, fmt.Sprintf("%s%s", root, string(os.PathSeparator)), "", -1)] = data
			}
		}
		return nil
	})
	if err != nil {
		ErrorAndExit(err.Error())
	}

	var deployments sync.WaitGroup
	status := deployStatus{aborted: false}
	forces := manager.getAllForce()
	var options []ForceDeployOptions
	if baseOptions.TestLevel == "NoTestRun" || len(forces) == 1 {
		options = make([]ForceDeployOptions, len(forces))
		for i := range options {
			options[i] = baseOptions
		}
	} else {
		options = splitTests(baseOptions, files, len(forces))
	}

	for i, f := range forces {
		if status.isAborted() {
			break
		}
		current := f
		index := i
		deployments.Add(1)
		go func() {
			defer deployments.Done()
			err := deployWith(current, &status, files, options[index], displayOptions)
			if err == nil && displayOptions.reportFormat == "text" && !displayOptions.quiet {
				fmt.Printf("Imported from %s\n", root)
			}
			if err != nil && (!errors.Is(err, testFailureError) || displayOptions.errorOnTestFailure) && !status.isAborted() {
				fmt.Fprintf(os.Stderr, "Aborting deploy due to %s\n", err.Error())
				status.abort()
			}
		}()
	}

	deployments.Wait()
}

// Evenly distribute tests to be run among deployments by counting how many test methods each (test) class has
func splitTests(ops ForceDeployOptions, files ForceMetadataFiles, deployments int) []ForceDeployOptions {
	//what about RunAllTestsInOrg?
	options := make([]ForceDeployOptions, deployments)
	testClasses := makeTestClasses(ops, files)
	split := splitByDeployments(testClasses, deployments)

	for i, s := range split {
		options[i] = ops
		options[i].RunTests = make([]string, len(s))

		for j, className := range s {
			options[i].RunTests[j] = className.name
		}
	}

	return options
}

type testClass struct {
	name    string
	methods int
}

func makeTestClasses(ops ForceDeployOptions, files ForceMetadataFiles) []testClass {
	allTests := len(ops.RunTests) == 0
	testClasses := make([]testClass, 0)
	folderPreffix := "classes" + string(os.PathSeparator)

	for fileName, contents := range files {
		name, isClass := strings.CutSuffix(fileName, ".cls")

		if isClass && (allTests || slices.Contains(ops.RunTests, name)) {
			count := testMethods(contents)

			if count > 0 {
				testClasses = append(testClasses, testClass{name: strings.TrimPrefix(name, folderPreffix), methods: count})
			}
		}
	}

	return testClasses
}

type testClassListener struct {
	*parser.BaseApexParserListener
	methods int
}

func (t *testClassListener) EnterModifier(ctx *parser.ModifierContext) {
	if ctx.TESTMETHOD() != nil {
		t.methods += 1
	}
}

func (t *testClassListener) EnterAnnotation(ctx *parser.AnnotationContext) {
	annotation := ctx.QualifiedName()

	if annotation != nil && strings.ToUpper(annotation.GetText()) == "ISTEST" {
		t.methods += 1
	}
}

func testMethods(src []byte) int {
	input := antlr.NewInputStream(string(src))
	lexer := parser.NewApexLexer(input)
	stream := antlr.NewCommonTokenStream(lexer, antlr.TokenDefaultChannel)
	p := parser.NewApexParser(stream)
	listener := new(testClassListener)

	p.RemoveErrorListeners()
	antlr.ParseTreeWalkerDefault.Walk(listener, p.CompilationUnit())

	return listener.methods
}

type byMethods []testClass

func (a byMethods) Len() int           { return len(a) }
func (a byMethods) Swap(i, j int)      { a[i], a[j] = a[j], a[i] }
func (a byMethods) Less(i, j int) bool { return a[i].methods > a[j].methods }

func splitByDeployments(classes []testClass, n int) [][]testClass {
	sort.Sort(byMethods(classes))
	groups := make([][]testClass, n)

	for i := range groups {
		groups[i] = []testClass{}
	}

	methodCounts := make([]int, n)

	for _, class := range classes {
		// Find the group with the least number of methods
		minIdx := 0
		for i := 1; i < n; i++ {
			if methodCounts[i] < methodCounts[minIdx] {
				minIdx = i
			}
		}

		groups[minIdx] = append(groups[minIdx], class)
		methodCounts[minIdx] += class.methods
	}

	return groups
}

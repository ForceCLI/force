// Package sf implements the command tree used when the force binary is
// invoked under the name sf.  Commands accept the names and flags of the
// Salesforce sf CLI and print output in its format.
package sf

import (
	"context"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"sort"
	"strings"

	"github.com/spf13/cobra"
	"github.com/spf13/pflag"

	forceerror "github.com/ForceCLI/force/error"
	"github.com/ForceCLI/force/lib"
)

// CompatVersion is the sf release whose commands, flags and output this
// package reproduces.
const CompatVersion = "2.138.6"

// InvokedAsSf reports whether argv0 names the binary sf.
func InvokedAsSf(argv0 string) bool {
	name := filepath.Base(strings.ReplaceAll(argv0, "\\", "/"))
	if len(name) > 4 && strings.EqualFold(name[len(name)-4:], ".exe") {
		name = name[:len(name)-4]
	}
	return name == "sf"
}

// binName is the command line that starts this mode.  sf names its own
// binary in the commands it suggests ("Run \"sf org resume sandbox ...\"");
// this mode writes the program the way it was started, so that the
// suggested command runs this program and not another sf on the PATH.
var binName = "sf"

// SetInvocation records how sf-compatible mode was started: argv0 as it was
// typed, such as "sf", "/tmp/sf" or "./force", followed by the mode flag
// when the mode was selected with it.  A name that cannot be written in a
// message without quoting, and the temporary binary of "go run", are
// replaced by "sf", or by "force" with the flag.
func SetInvocation(argv0 string, withModeFlag bool) {
	name := argv0
	// "go run" starts a binary in a temporary directory that is removed
	// when the program ends, so its path is of no use in a suggestion.
	slashed := filepath.ToSlash(name)
	temporary := strings.Contains(slashed, "/go-build") && strings.Contains(slashed, "/exe/")
	if name == "" || temporary || strings.ContainsAny(name, " %\"'`") {
		name = "sf"
		if withModeFlag {
			name = "force"
		}
	}
	if withModeFlag {
		name += " " + ModeFlag
	}
	binName = name
}

// ModeFlag is the force flag that selects sf-compatible mode.
const ModeFlag = "--sf"

// StripModeFlag reports whether args contain --sf and returns args without
// it.  An argument after "--" is not a flag.  "force --sf org list" and
// "force org list --sf" both run "sf org list".
func StripModeFlag(args []string) ([]string, bool) {
	for i, arg := range args {
		if arg == "--" {
			break
		}
		if arg == ModeFlag {
			rest := append([]string{}, args[:i]...)
			return append(rest, args[i+1:]...), true
		}
	}
	return args, false
}

// Execute runs the sf command tree with the process arguments and exits.
func Execute() {
	os.Exit(Run(os.Args[1:], os.Stdout, os.Stderr))
}

// Run executes the sf command tree and returns the exit status.
// currentArgs holds the arguments of the command that is running.
var currentArgs []string

func Run(args []string, out, errOut io.Writer) int {
	root := NewRootCmd()
	args, deprecation := normalizeArgsFor(root, args)
	args, flagDeprecations := rewriteLegacyFlags(root, args)
	jsonOutput := hasJSONFlag(args)
	if expanded, err := expandFlagsDir(root, args); err != nil {
		return writeError(out, errOut, nil, err, jsonOutput)
	} else {
		args = expanded
		jsonOutput = hasJSONFlag(args)
	}
	root.SetArgs(args)
	currentArgs = args
	// sf warns about the old comma form when a command with a list flag
	// that is split at commas is given an argument with a comma.
	if target, _, findErr := root.Find(args); findErr == nil && commaListFlags[target.CommandPath()] != nil && !commaWarningAfterBeta[target.CommandPath()] && hasUnescapedComma(args) {
		flagDeprecations = append(flagDeprecations, commaListWarning)
	}
	state := &runState{flagDeprecations: flagDeprecations}
	root.SetContext(context.WithValue(context.Background(), runStateKey{}, state))
	if deprecation != "" {
		if jsonOutput {
			state.deprecations = append(state.deprecations, deprecation)
		} else {
			fmt.Fprintf(errOut, "%s %s\n", paint(errOut, styleBoldYellow, "Warning:"), deprecation)
		}
	}
	if cmd, err := repeatedFlag(root, args); err != nil {
		state.flagDeprecations = nil
		cmd.SetContext(root.Context())
		return writeError(out, errOut, cmd, err, jsonOutput)
	}
	// With --json the flag warnings are part of the envelope.  Otherwise
	// they are printed before the first output of the command.
	reportFlagDeprecations := func() {
		if jsonOutput {
			return
		}
		for _, message := range state.flagDeprecations {
			fmt.Fprintf(errOut, "%s %s\n", paint(errOut, styleBoldYellow, "Warning:"), message)
		}
		state.flagDeprecations = nil
	}
	if len(flagDeprecations) > 0 && !jsonOutput {
		root.SetOut(&deprecationWriter{out, reportFlagDeprecations})
		root.SetErr(&deprecationWriter{errOut, reportFlagDeprecations})
	} else {
		root.SetOut(out)
		root.SetErr(errOut)
	}

	// lib reports progress, such as a session refresh, on stderr.  sf
	// prints nothing for those steps.
	previousLog := lib.Log
	lib.Log = silentLogger{}
	defer func() { lib.Log = previousLog }()

	// Deploys are recorded by the source tracking of an org only for the
	// client name that sf sends.
	previousClient := lib.SoapClientName
	lib.SoapClientName = "sfdx toolbelt:"
	defer func() { lib.SoapClientName = previousClient }()

	// lib reports some failures by calling ErrorAndExit.  Print those in
	// sf's format.  ErrorAndExit exits with status 1 after the handler.
	previous := forceerror.ExitHandler
	forceerror.ExitHandler = func(message string) {
		writeError(out, errOut, nil, &Error{Message: message, ExitCode: 1}, jsonOutput)
	}
	defer func() { forceerror.ExitHandler = previous }()

	cmd, err := root.ExecuteC()
	if err != nil {
		if isParseError(err) {
			state.flagDeprecations = nil
		}
		reportFlagDeprecations()
		return writeError(out, errOut, cmd, err, jsonOutput)
	}
	reportFlagDeprecations()
	return 0
}

// NewRootCmd builds a new sf command tree.  Each call returns independent
// commands and flag values.
func NewRootCmd() *cobra.Command {
	root := &cobra.Command{
		Use:           "sf",
		Short:         "sf-compatible mode of the force CLI",
		Args:          cobra.ArbitraryArgs,
		SilenceErrors: true,
		SilenceUsage:  true,
		Version:       versionString(),
		RunE: func(cmd *cobra.Command, args []string) error {
			if len(args) > 0 {
				return &unknownCommandError{id: args[0]}
			}
			return cmd.Help()
		},
	}
	root.SetVersionTemplate("{{.Version}}\n")
	root.Flags().BoolP("version", "v", false, "Show the version")
	root.SetGlobalNormalizationFunc(normalizeFlagName)
	root.AddCommand(
		newAliasCmd(),
		newApexCmd(),
		newApiCmd(),
		newAutocompleteCmd(),
		newCmdtCmd(),
		newCommandsCmd(),
		newWhichCmd(),
		newConfigCmd(),
		newDataCmd(),
		newForceLegacyCmd(),
		newOrgCmd(),
		newPackageCmd(),
		newPackage1Cmd(),
		newCommunityCmd(),
		newGroup("flow", "Commands to run and get the results of Flow tests.",
			newGroup("run", "Run Flow tests.", newFlowRunTestCmd()),
			newGroup("get", "Get Flow test results.", newFlowGetTestCmd()),
		),
		newGroup("logic", "Commands to run and get the results of Apex and Flow tests.",
			newGroup("run", "Run tests.", newLogicRunTestCmd()),
			newGroup("get", "Get test results.", newLogicGetTestCmd()),
		),
		newProjectCmd(),
		newSchemaCmd(),
		newSobjectCmd(),
		newTemplateCmd(),
		newVersionCmd(),
	)
	return root
}

// newGroup returns a command that only holds subcommands, such as "org" or
// "org login".
func newGroup(use, short string, children ...*cobra.Command) *cobra.Command {
	group := &cobra.Command{
		Use:         use,
		Short:       short,
		Annotations: map[string]string{topicAnnotation: "true"},
		// An unknown command under a topic is reported as an unknown
		// command even when flags follow it.
		FParseErrWhitelist: cobra.FParseErrWhitelist{UnknownFlags: true},
		RunE: func(cmd *cobra.Command, args []string) error {
			if len(args) > 0 {
				id := strings.Join(append(strings.Fields(cmd.CommandPath())[1:], args...), " ")
				return &unknownCommandError{id: id}
			}
			return cmd.Help()
		},
	}
	group.AddCommand(children...)
	return group
}

// withSubcommands adds children to a command that also runs on its own,
// such as "org list", which has the subcommand "org list auth".
func withSubcommands(cmd *cobra.Command, children ...*cobra.Command) *cobra.Command {
	cmd.AddCommand(children...)
	return cmd
}

// legacyFlagNames maps flag names from sfdx to the sf names.
var legacyFlagNames = map[string]string{
	"targetusername":       "target-org",
	"targetdevhubusername": "target-dev-hub",
	"target-hub-org":       "target-dev-hub",
	"apiversion":           "api-version",
}

func normalizeFlagName(f *pflag.FlagSet, name string) pflag.NormalizedName {
	if current, ok := legacyFlagNames[name]; ok {
		return pflag.NormalizedName(current)
	}
	for _, candidate := range commandLegacyFlagNames[name] {
		if f.Lookup(candidate.current) != nil && (candidate.marker == "" || f.Lookup(candidate.marker) != nil) {
			return pflag.NormalizedName(candidate.current)
		}
	}
	return pflag.NormalizedName(name)
}

// commandLegacyFlagNames maps a legacy flag name that only some commands
// accept to the current name.  The name applies to a command that has the
// current flag and, when marker is set, the marker flag as well.
var commandLegacyFlagNames = map[string][]struct{ current, marker string }{
	"outputdir":       {{"output-directory", ""}},
	"outputdirectory": {{"output-directory", ""}},
	"inputdir":        {{"input-directory", ""}},
	"inputdirectory":  {{"input-directory", ""}},
	"typename":        {{"type-name", ""}},
	"plurallabel":     {{"plural-label", ""}},
	"recordname":      {{"record-name", ""}},
	"picklistvalues":  {{"picklist-values", ""}},
	"decimalplaces":   {{"decimal-places", ""}},
	"namecolumn":      {{"name-column", ""}},
	"fieldname":       {{"name", "decimal-places"}},
	"fieldtype":       {{"type", "decimal-places"}},
	"filepath":        {{"csv", "name-column"}},
}

const topicAnnotation = "sfTopic"

// normalizeArgsFor is normalizeArgs for a command tree.  A legacy id is not
// rewritten when the words already name a command of the tree that is at
// least as long: "project generate manifest" is a command, although
// "project generate" is a legacy id of "template generate project".
func normalizeArgsFor(root *cobra.Command, args []string) ([]string, string) {
	if len(args) == 0 || strings.HasPrefix(args[0], "-") {
		return args, ""
	}
	words := append(strings.Split(args[0], ":"), args[1:]...)
	positional := 0
	for positional < len(words) && !strings.HasPrefix(words[positional], "-") {
		positional++
	}
	if found, _, err := root.Find(words[:positional]); err == nil && found != root && found.Annotations[topicAnnotation] == "" {
		return words, ""
	}
	result, legacy, current := rewriteLegacyID(args)
	if legacy == "" || aliasesWithoutWarning[legacy] {
		return result, ""
	}
	// The warning is for a command this mode has.  An alias of a command
	// that is not implemented reports the unknown command only.
	if found, _, err := root.Find(strings.Split(current, ":")); err != nil || found == root || found.Annotations[topicAnnotation] != "" {
		return result, ""
	}
	return result, fmt.Sprintf("The %q command has been deprecated. Use %q instead.", strings.ReplaceAll(legacy, ":", " "), strings.ReplaceAll(current, ":", " "))
}

// Legacy ids that sf accepts without a deprecation warning.
var aliasesWithoutWarning = map[string]bool{
	"force:package:push-upgrade:list": true,
	"whatsnew":                        true,
	"plugins:add":                     true,
	"plugins:unlink":                  true,
	"plugins:remove":                  true,
}

// normalizeArgs converts the colon form of a command id ("org:login:web") to
// separate words and replaces legacy ids ("force:org:list", "deploy
// metadata") with the current id.
func normalizeArgs(args []string) []string {
	result, _, _ := rewriteLegacyID(args)
	return result
}

// rewriteLegacyID is normalizeArgs that also returns the legacy id it
// replaced and the current id, or empty strings when it replaced none.
func rewriteLegacyID(args []string) ([]string, string, string) {
	if len(args) == 0 || strings.HasPrefix(args[0], "-") {
		return args, "", ""
	}
	words := strings.Split(args[0], ":")
	rest := args[1:]
	// The words of an id can also be given as separate arguments.
	positional := 0
	for positional < len(rest) && !strings.HasPrefix(rest[positional], "-") {
		positional++
	}
	candidates := append(append([]string{}, words...), rest[:positional]...)
	for n := len(candidates); n >= 1; n-- {
		if n < len(words) {
			break
		}
		legacy := strings.Join(candidates[:n], ":")
		if current, ok := legacyCommandIds[legacy]; ok {
			result := strings.Split(current, ":")
			result = append(result, candidates[n:]...)
			return append(result, rest[positional:]...), legacy, current
		}
	}
	return append(words, rest...), "", ""
}

// repeatedFlag returns the error sf reports when a flag that takes one value
// is given more than once, with the command the arguments name.  A boolean
// flag and a flag that takes several values can be repeated.
func repeatedFlag(root *cobra.Command, args []string) (*cobra.Command, error) {
	positional := 0
	for positional < len(args) && !strings.HasPrefix(args[positional], "-") {
		positional++
	}
	cmd, _, err := root.Find(args[:positional])
	if err != nil || cmd == root {
		return nil, nil
	}
	seen := map[string]bool{}
	// record counts a flag and reports whether it takes a value.
	record := func(flag *pflag.Flag) (bool, error) {
		switch flag.Value.Type() {
		case "bool":
			return false, nil
		case "stringArray", "stringSlice", "intSlice":
			return true, nil
		}
		if seen[flag.Name] {
			return true, &Error{Message: fmt.Sprintf("Flag --%s can only be specified once", flag.Name), ExitCode: 1}
		}
		seen[flag.Name] = true
		return true, nil
	}
	for i := positional; i < len(args); i++ {
		arg := args[i]
		switch {
		case arg == "--":
			return cmd, nil
		case strings.HasPrefix(arg, "--"):
			name, _, hasValue := strings.Cut(arg[2:], "=")
			flag := cmd.Flags().Lookup(name)
			if flag == nil {
				continue
			}
			takesValue, err := record(flag)
			if err != nil {
				return cmd, err
			}
			if takesValue && !hasValue {
				i++
			}
		case len(arg) > 1 && arg[0] == '-':
			// Short flags can be combined; the first one that takes a
			// value uses the rest of the argument, or the next argument.
			for j := 1; j < len(arg); j++ {
				flag := cmd.Flags().ShorthandLookup(arg[j : j+1])
				if flag == nil {
					break
				}
				takesValue, err := record(flag)
				if err != nil {
					return cmd, err
				}
				if takesValue {
					if j == len(arg)-1 {
						i++
					}
					break
				}
			}
		}
	}
	return cmd, nil
}

// isParseError reports whether sf raises the error while it parses the
// command line: an unknown flag, a missing required flag, a flag value that
// fails its check, or a target org that cannot be resolved.
func isParseError(err error) bool {
	if _, ok := err.(exitStatus); ok {
		return false
	}
	if _, ok := err.(*unknownCommandError); ok {
		return true
	}
	e := asError(err)
	return strings.HasSuffix(e.Message, "See more help with --help") || strings.HasSuffix(e.Message, "can only be specified once") || e.Name == "NoDefaultEnvError" || e.Name == "NoDefaultDevHubError"
}

// rewriteLegacyFlags replaces the legacy flag names of the command that
// args names with the current names, and returns the deprecation warnings
// sf reports for them.  sf warns for "--alias value" and not for
// "--alias=value".
func rewriteLegacyFlags(root *cobra.Command, args []string) ([]string, []string) {
	positional := 0
	for positional < len(args) && !strings.HasPrefix(args[positional], "-") {
		positional++
	}
	cmd, _, err := root.Find(args[:positional])
	if err != nil || cmd == root {
		return args, nil
	}
	aliases := legacyFlagAliases[strings.ReplaceAll(strings.TrimPrefix(cmd.CommandPath(), root.Name()+" "), " ", ":")]
	if len(aliases) == 0 {
		return args, nil
	}
	// A legacy name is replaced when the command has the current flag and
	// no other flag of the legacy name.
	usable := func(alias flagAlias) bool {
		if cmd.Flags().Lookup(alias.current) == nil {
			return false
		}
		existing := cmd.Flags().Lookup(alias.alias)
		if len(alias.alias) == 1 {
			existing = cmd.Flags().ShorthandLookup(alias.alias)
		}
		// A hidden flag of the legacy name is this mode's own definition
		// of the legacy name.
		return existing == nil || existing.Name == alias.current || existing.Hidden
	}
	result := append([]string{}, args...)
	used := map[string]bool{}
	for i := positional; i < len(result); i++ {
		arg := result[i]
		if arg == "--" {
			break
		}
		for _, alias := range aliases {
			if !usable(alias) {
				continue
			}
			long := "--" + alias.alias
			switch {
			case arg == long:
				result[i] = "--" + alias.current
				// sf looks for "-u" on the command line to warn about
				// the one-letter name u, so "--u" has no warning.
				if len(alias.alias) > 1 {
					used[alias.alias] = true
				}
			case strings.HasPrefix(arg, long+"="):
				result[i] = "--" + alias.current + strings.TrimPrefix(arg, long)
			case len(alias.alias) == 1 && arg == "-"+alias.alias:
				result[i] = "--" + alias.current
				used[alias.alias] = true
			}
		}
	}
	var warnings []string
	for _, alias := range aliases {
		if !used[alias.alias] {
			continue
		}
		name := "--" + alias.alias
		if len(alias.alias) == 1 {
			name = "-" + alias.alias
		}
		preferred := "--" + alias.current
		if alias.char != "" {
			preferred += " | -" + alias.char
		}
		warnings = append(warnings, fmt.Sprintf("The %q flag has been deprecated. Use %q instead.", name, preferred))
	}
	return result, warnings
}

// expandFlagsDir implements --flags-dir: each file in the directory is
// named after a flag and holds its value, one value on each line.  An empty
// file sets a boolean flag.  A flag that is also on the command line keeps
// the command line value.
func expandFlagsDir(root *cobra.Command, args []string) ([]string, error) {
	dir := ""
	rest := make([]string, 0, len(args))
	for i := 0; i < len(args); i++ {
		switch {
		case args[i] == "--":
			rest = append(rest, args[i:]...)
			i = len(args)
		case args[i] == "--flags-dir" && i+1 < len(args):
			dir = args[i+1]
			i++
		case strings.HasPrefix(args[i], "--flags-dir="):
			dir = strings.TrimPrefix(args[i], "--flags-dir=")
		default:
			rest = append(rest, args[i])
		}
	}
	if dir == "" {
		return args, nil
	}
	entries, err := os.ReadDir(dir)
	if err != nil {
		return nil, &Error{Message: fmt.Sprintf("Parsing --flags-dir \n\tNo directory found at %s\nSee more help with --help", dir), ExitCode: 2}
	}
	// The long and short names of a flag count as the same flag.
	var command *cobra.Command
	if root != nil {
		positional := 0
		for positional < len(rest) && !strings.HasPrefix(rest[positional], "-") {
			positional++
		}
		command, _, _ = root.Find(rest[:positional])
	}
	forms := func(name string) []string {
		names := []string{name}
		if command != nil {
			flag := command.Flags().Lookup(name)
			if flag == nil && len(name) == 1 {
				flag = command.Flags().ShorthandLookup(name)
			}
			if flag != nil {
				names = []string{flag.Name}
				if flag.Shorthand != "" {
					names = append(names, flag.Shorthand)
				}
			}
		}
		var result []string
		for _, candidate := range names {
			if len(candidate) == 1 {
				result = append(result, "-"+candidate)
			} else {
				result = append(result, "--"+candidate, "--no-"+candidate)
			}
		}
		return result
	}
	given := func(name string) bool {
		for _, arg := range rest {
			if arg == "--" {
				break
			}
			for _, form := range forms(name) {
				if arg == form || strings.HasPrefix(arg, form+"=") {
					return true
				}
			}
		}
		return false
	}
	// Flags from files go before a "--" argument.
	end := len(rest)
	for i, arg := range rest {
		if arg == "--" {
			end = i
			break
		}
	}
	var fromFiles []string
	for _, entry := range entries {
		if entry.IsDir() {
			continue
		}
		name := strings.TrimSuffix(entry.Name(), ".json")
		if given(name) {
			continue
		}
		data, err := os.ReadFile(filepath.Join(dir, entry.Name()))
		if err != nil {
			return nil, err
		}
		prefix := "--" + name
		if len(name) == 1 {
			prefix = "-" + name
		}
		content := strings.TrimSpace(string(data))
		if content == "" {
			fromFiles = append(fromFiles, prefix)
			continue
		}
		for _, line := range strings.Split(content, "\n") {
			if line = strings.TrimSpace(line); line != "" {
				fromFiles = append(fromFiles, prefix+"="+line)
			}
		}
	}
	result := append([]string{}, rest[:end]...)
	result = append(result, fromFiles...)
	return append(result, rest[end:]...), nil
}

func hasJSONFlag(args []string) bool {
	for _, arg := range args {
		if arg == "--" {
			return false
		}
		if arg == "--json" || arg == "--json=true" {
			return true
		}
	}
	return false
}

// Shared flag helpers.  Commands read the values with cmd.Flags().

func addJSONFlag(cmd *cobra.Command) {
	cmd.Flags().Bool("json", false, "Format output as json.")
}

// addTargetOrgFlag adds --target-org/-o.  It also accepts the sfdx short
// flag -u when the command does not use -u for another flag, so call it
// after defining the command's own flags.
func addTargetOrgFlag(cmd *cobra.Command) {
	cmd.Flags().StringP("target-org", "o", "", "Username or alias of the target org.")
	if cmd.Flags().ShorthandLookup("u") == nil {
		cmd.Flags().StringP("targetusername-short", "u", "", "")
		cmd.Flags().MarkHidden("targetusername-short")
	}
}

func addTargetDevHubFlag(cmd *cobra.Command) {
	cmd.Flags().StringP("target-dev-hub", "v", "", "Username or alias of the Dev Hub org.")
}

func addAPIVersionFlag(cmd *cobra.Command) {
	cmd.Flags().String("api-version", "", "Override the api version used for api requests made by this command")
}

// missingFlagError is the error sf reports for a required flag that was not
// given.
func missingFlagError(name string) *Error {
	return &Error{Message: fmt.Sprintf("The following error occurred:\n  Missing required flag %s\nSee more help with --help", name), ExitCode: 2}
}

// missingFlagsError is the error sf reports for the required flags that
// were not given, which it lists in alphabetical order.
func missingFlagsError(names ...string) *Error {
	if len(names) == 1 {
		return missingFlagError(names[0])
	}
	sort.Strings(names)
	lines := make([]string, 0, len(names))
	for _, name := range names {
		lines = append(lines, "  Missing required flag "+name)
	}
	return &Error{Message: "The following errors occurred:\n" + strings.Join(lines, "\n") + "\nSee more help with --help", ExitCode: 2}
}

// integerInRange returns the error sf reports for an integer flag outside
// its range.
func integerInRange(flag string, value, min, max int) error {
	switch {
	case value < min:
		return &Error{Message: fmt.Sprintf("Parsing --%s \n\tExpected an integer greater than or equal to %d but received: %d\nSee more help with --help", flag, min, value), ExitCode: 2}
	case value > max:
		return &Error{Message: fmt.Sprintf("Parsing --%s \n\tExpected an integer less than or equal to %d but received: %d\nSee more help with --help", flag, max, value), ExitCode: 2}
	}
	return nil
}

// targetOrgFlag returns the value of --target-org, or of -u when only the
// sfdx short flag was given.
func targetOrgFlag(cmd *cobra.Command) string {
	if value, _ := cmd.Flags().GetString("target-org"); value != "" {
		return value
	}
	if cmd.Flags().Lookup("targetusername-short") != nil {
		value, _ := cmd.Flags().GetString("targetusername-short")
		return value
	}
	return ""
}

func versionString() string {
	return fmt.Sprintf("@salesforce/cli/%s %s force-%s", CompatVersion, platform(), forceVersion())
}

// silentLogger discards the progress messages of lib.
type silentLogger struct{}

func (silentLogger) Info(...interface{}) {}

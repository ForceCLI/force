package sf

import (
	"errors"
	"fmt"
	"io"
	"io/fs"
	"os"
	"os/exec"
	"os/signal"
	"sort"
	"strconv"
	"strings"
	"syscall"

	"github.com/spf13/cobra"

	"github.com/ForceCLI/force/lib"
	"github.com/ForceCLI/force/sf/plugins"
	"github.com/ForceCLI/force/sfplugin"
)

// dispatchPlugin runs the plugin command that args name and returns its
// exit status.  The second result is false when args name a built-in
// command or no plugin command, and the built-in command tree handles them.
// A built-in command is never replaced by a plugin command.
func dispatchPlugin(root *cobra.Command, args []string, out, errOut io.Writer) (int, bool) {
	words := leadingWords(args)
	if len(words) == 0 {
		return 0, false
	}
	// "help sgd source delta" shows the help of the plugin command.
	help := words[0] == "help"
	if help {
		words = words[1:]
		if len(words) == 0 {
			return 0, false
		}
	}
	if isBuiltinCommand(root, words) {
		return 0, false
	}
	store, err := plugins.OpenDefault()
	if err != nil {
		return 0, false
	}
	records, err := store.Load()
	if err != nil {
		// A plugins.json that cannot be read is reported only when no
		// built-in command can handle the words.
		if found, _, findErr := root.Find(words); findErr == nil && found != root {
			return 0, false
		}
		return writeError(out, errOut, nil, err, hasJSONFlag(args)), true
	}
	match, ok := plugins.Find(records, words)
	if !ok {
		if found, _, findErr := root.Find(words); findErr == nil && found == root && listPluginTopic(out, records, words) {
			return 0, true
		}
		return 0, false
	}
	pluginArgs := args
	if help {
		pluginArgs = append(append([]string{}, words[:match.Words]...), "--help")
	}
	status, err := runPlugin(match, pluginArgs, out, errOut)
	if err != nil {
		return writeError(out, errOut, nil, err, hasJSONFlag(args)), true
	}
	return status, true
}

// leadingWords returns the arguments before the first flag.
func leadingWords(args []string) []string {
	count := 0
	for count < len(args) && !strings.HasPrefix(args[count], "-") {
		count++
	}
	return args[:count]
}

// isBuiltinCommand reports whether the built-in command tree handles words:
// they name a command that runs something, or exactly a topic.
func isBuiltinCommand(root *cobra.Command, words []string) bool {
	found, rest, err := root.Find(words)
	if err != nil || found == root {
		return false
	}
	return found.Annotations[topicAnnotation] == "" || len(rest) == 0
}

// listPluginTopic prints the plugin commands whose ids start with words,
// and reports whether there are any.  It answers a command line that names
// a topic of a plugin, such as "sgd", and no command.
func listPluginTopic(out io.Writer, records []plugins.Record, words []string) bool {
	prefix := strings.Join(words, " ") + " "
	var rows [][2]string
	for _, record := range records {
		for _, command := range record.Manifest.Commands {
			if strings.HasPrefix(command.ID, prefix) {
				rows = append(rows, [2]string{command.ID, command.Summary})
			}
		}
	}
	if len(rows) == 0 {
		return false
	}
	sort.Slice(rows, func(i, j int) bool { return rows[i][0] < rows[j][0] })
	width := 0
	for _, row := range rows {
		if len(row[0]) > width {
			width = len(row[0])
		}
	}
	fmt.Fprintf(out, "%s\n", paint(out, styleBold, "COMMANDS"))
	for _, row := range rows {
		fmt.Fprintf(out, "  %-*s  %s\n", width, row[0], row[1])
	}
	return true
}

// runPlugin starts the executable of a plugin with args, which begin with
// the words of the command id, and waits for it.  The plugin reads stdin
// and writes to out and errOut directly.  The result is the exit status of
// the plugin; an error means the plugin was not started.
func runPlugin(match plugins.Match, args []string, out, errOut io.Writer) (int, error) {
	record := match.Record
	if _, err := os.Stat(record.Executable); err != nil {
		action := fmt.Sprintf("Run \"%s plugins install %s\" to install it again.", binName, record.Name)
		if record.Linked {
			action = fmt.Sprintf("Build the executable, or run \"%s plugins uninstall %s\" to remove the link.", binName, record.Name)
		}
		reason := err.Error()
		if errors.Is(err, fs.ErrNotExist) {
			reason = "the file does not exist"
		}
		return 0, &Error{
			Name:     "PluginExecutableError",
			Message:  fmt.Sprintf("The executable of the plugin %s cannot be used: %s (%s).", record.Name, reason, record.Executable),
			ExitCode: 1,
			Actions:  []string{action},
		}
	}
	env, err := pluginEnvironment(match, args[match.Words:])
	if err != nil {
		return 0, err
	}
	child := exec.Command(record.Executable, args...)
	child.Stdin = os.Stdin
	child.Stdout = out
	child.Stderr = errOut
	child.Env = env

	// An interrupt typed at the terminal is delivered to the plugin as
	// well, because it is in the same process group; this process waits
	// for the plugin to end.  A termination signal sent to this process
	// alone is passed on.
	signals := make(chan os.Signal, 4)
	signal.Notify(signals, os.Interrupt, syscall.SIGTERM)
	defer signal.Stop(signals)
	if err := child.Start(); err != nil {
		return 0, &Error{
			Name:     "PluginExecutableError",
			Message:  fmt.Sprintf("The plugin %s could not be started: %s", record.Name, err.Error()),
			ExitCode: 1,
		}
	}
	done := make(chan struct{})
	go func() {
		for {
			select {
			case received := <-signals:
				if received == syscall.SIGTERM {
					child.Process.Signal(received)
				}
			case <-done:
				return
			}
		}
	}()
	err = child.Wait()
	close(done)
	if err == nil {
		return 0, nil
	}
	var exit *exec.ExitError
	if errors.As(err, &exit) {
		if code := exit.ExitCode(); code > 0 {
			return code, nil
		}
		// The plugin was ended by a signal.
		return 1, nil
	}
	return 0, err
}

// pluginOrgVariables are the variables that describe an org.  Values
// inherited from the environment of this process are never passed to a
// plugin.
var pluginOrgVariables = []string{
	sfplugin.EnvInstanceURL,
	sfplugin.EnvAccessToken,
	sfplugin.EnvAPIVersion,
	sfplugin.EnvUsername,
	sfplugin.EnvOrgID,
}

// pluginEnvironment returns the environment of a plugin process: the
// environment of this process, the protocol variables, and for a command
// that uses an org, the org variables.  flags are the arguments after the
// command id.
func pluginEnvironment(match plugins.Match, flags []string) ([]string, error) {
	replaced := map[string]bool{
		sfplugin.EnvProtocol: true,
		sfplugin.EnvBinName:  true,
		sfplugin.EnvName:     true,
		sfplugin.EnvVersion:  true,
	}
	for _, name := range pluginOrgVariables {
		replaced[name] = true
	}
	var env []string
	for _, entry := range os.Environ() {
		name, _, _ := strings.Cut(entry, "=")
		if !replaced[name] {
			env = append(env, entry)
		}
	}
	env = append(env,
		sfplugin.EnvProtocol+"="+strconv.Itoa(sfplugin.ProtocolVersion),
		sfplugin.EnvBinName+"="+binName,
		sfplugin.EnvName+"="+match.Record.Manifest.Name,
		sfplugin.EnvVersion+"="+match.Record.Manifest.Version,
	)
	requirement := match.Command.OrgRequirement()
	if requirement == sfplugin.OrgNone {
		return env, nil
	}
	// lib reports a session refresh on stderr.  sf prints nothing for it.
	previousLog := lib.Log
	lib.Log = silentLogger{}
	defer func() { lib.Log = previousLog }()
	name := flagValue(flags, []string{"target-org", "targetusername"}, []string{"o", "u"})
	version := flagValue(flags, []string{"api-version", "apiversion"}, nil)
	org, err := resolveOrgNamed(name, "target-org", lib.SFConfigTargetOrg, version)
	if err == nil {
		// One authenticated request, so that an expired access token is
		// replaced before the plugin receives it.
		if _, requestErr := org.Force.GetREST(""); requestErr != nil {
			err = &Error{
				Name:     "PluginOrgConnectionError",
				Message:  fmt.Sprintf("The org %s did not accept a request: %s", org.Username, requestErr.Error()),
				ExitCode: 1,
			}
		}
	}
	if err != nil {
		// An org that was named on the command line must be usable.
		// Without one, a command that can run without an org does.
		if requirement == sfplugin.OrgRequired || name != "" {
			return nil, err
		}
		return env, nil
	}
	apiVersion := org.Force.Credentials.SessionOptions.ApiVersion
	if apiVersion == "" {
		apiVersion = lib.ApiVersionNumber()
	}
	return append(env,
		sfplugin.EnvInstanceURL+"="+org.Force.Credentials.InstanceUrl,
		sfplugin.EnvAccessToken+"="+org.Force.Credentials.AccessToken,
		sfplugin.EnvAPIVersion+"="+apiVersion,
		sfplugin.EnvUsername+"="+org.Username,
		sfplugin.EnvOrgID+"="+org.Fields.String("orgId"),
	), nil
}

// flagValue returns the value of the first flag in args that has one of
// the long or short names, written as "--name value", "--name=value" or
// "-n value".  Arguments after "--" are not flags.
func flagValue(args []string, long, short []string) string {
	for i, arg := range args {
		if arg == "--" {
			break
		}
		for _, name := range long {
			if arg == "--"+name && i+1 < len(args) {
				return args[i+1]
			}
			if value, ok := strings.CutPrefix(arg, "--"+name+"="); ok {
				return value
			}
		}
		for _, name := range short {
			if arg == "-"+name && i+1 < len(args) {
				return args[i+1]
			}
		}
	}
	return ""
}

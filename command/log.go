package command

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"os/signal"
	"strings"

	"github.com/ForceCLI/force/bubbles"
	"github.com/ForceCLI/force/desktop"
	. "github.com/ForceCLI/force/error"
	. "github.com/ForceCLI/force/lib"
	"github.com/ForceCLI/force/lib/apexlog"
	"github.com/ForceCLI/force/lib/bayeux"
	"github.com/ForceCLI/force/webdebug"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/spf13/cobra"
)

func init() {
	debugLogCmd.Flags().StringP("file", "f", "", "read the log from a file instead of fetching it from the org")
	debugLogCmd.Flags().Bool("web", false, webFlagUsage)
	logCmd.AddCommand(deleteLogCmd)
	logCmd.AddCommand(tailLogCmd)
	logCmd.AddCommand(debugLogCmd)
	RootCmd.AddCommand(logCmd)
}

var logCmd = &cobra.Command{
	Use:   "log",
	Short: "Fetch debug logs",
	Example: `
  force log [list]
  force log <id>
  force log delete <id>
  force log tail
  force log debug <id>
`,
	Run: func(cmd *cobra.Command, args []string) {
		if len(args) == 0 || args[0] == "list" {
			getAllLogs()
			return
		}
		getLog(args[0])
	},
	Args:                  cobra.MaximumNArgs(1),
	DisableFlagsInUseLine: true,
}

var deleteLogCmd = &cobra.Command{
	Use:   "delete [logId]",
	Short: "Delete debug logs",
	Example: `
  force log delete <id>
`,
	Run: func(cmd *cobra.Command, args []string) {
		deleteLog(args[0])
	},
	Args:                  cobra.ExactArgs(1),
	DisableFlagsInUseLine: true,
}

var tailLogCmd = &cobra.Command{
	Use:   "tail",
	Short: "Stream debug logs",
	Example: `
  force log tail
`,
	Run: func(cmd *cobra.Command, args []string) {
		tailLogs()
	},
	Args:                  cobra.ExactArgs(0),
	DisableFlagsInUseLine: true,
}

var debugLogCmd = &cobra.Command{
	Use:   "debug [logId]",
	Short: "Replay a debug log in a terminal debugger",
	Long: `Replay a debug log in a terminal debugger.

The log must have been captured with APEX_CODE at FINEST; at lower levels
the log does not record each statement or variable assignment.
"force apex --debug" and "force test --debug" set your trace flag to this
level while they run, then open the log of the run in the debugger.

The source of each class and trigger is fetched from the org by the ID
recorded in the log. If a class has been changed since the log was
captured, the lines shown will not match the code that ran. The source of
anonymous Apex is read from the log.

With --web, the debugger opens in a browser instead of the terminal. It is
served on 127.0.0.1 until the page's Quit button is pressed or force is
stopped with Ctrl-C. Click a line number to set or remove a breakpoint.

Keys:
  n          step over
  s          step in
  o          step out
  c          continue to the next breakpoint
  p          step back to the previous statement in this method or a caller
  b          set or remove a breakpoint on the line under the cursor
  f          open a class or trigger from the log
  r          restart from the beginning of the log
  tab        move between the source, call stack, and variables panes
  up/down    move within the focused pane
  enter      expand or collapse a variable
  q          quit
`,
	Example: `
  force log debug 07L5g00000AbCdEFG
  force log debug --file apex.log
  force log debug --web 07L5g00000AbCdEFG
`,
	Args: func(cmd *cobra.Command, args []string) error {
		file, _ := cmd.Flags().GetString("file")
		switch {
		case file != "" && len(args) > 0:
			return errors.New("specify a log ID or --file, not both")
		case file == "" && len(args) != 1:
			return errors.New("specify a log ID or --file")
		}
		return nil
	},
	Run: func(cmd *cobra.Command, args []string) {
		file, _ := cmd.Flags().GetString("file")
		web, _ := cmd.Flags().GetBool("web")
		logId := ""
		if len(args) > 0 {
			logId = args[0]
		}
		debugLog(logId, file, web)
	},
}

func debugLog(logId string, file string, web bool) {
	var content string
	if file != "" {
		b, err := os.ReadFile(file)
		if err != nil {
			ErrorAndExit(err.Error())
		}
		content = string(b)
	} else {
		var err error
		content, err = force.RetrieveLog(logId)
		if err != nil {
			ErrorAndExit(err.Error())
		}
	}
	if err := replayLog(content, web); err != nil {
		ErrorAndExit(err.Error())
	}
}

// replayLog opens a debug log in the terminal debugger. Keys are read from
// the terminal rather than stdin, so the debugger also works after Apex code
// has been read from stdin.
func replayLog(content string, web bool) error {
	if strings.TrimSpace(content) == "" {
		return errors.New("no debug log to replay")
	}
	log, err := apexlog.Parse(strings.NewReader(content))
	if err != nil {
		return err
	}
	session, err := apexlog.NewSession(log)
	if err != nil {
		return err
	}
	addHeapDumps(session)
	sources, err := debugSources(session)
	if err != nil {
		return err
	}
	if web {
		return replayInBrowser(session, sources)
	}
	p := tea.NewProgram(bubbles.NewLogDebugModel(session, sources), tea.WithAltScreen(), tea.WithInputTTY())
	_, err = p.Run()
	return err
}

const webFlagUsage = "open the debugger in a browser instead of the terminal"

// replayInBrowser serves the debugger on 127.0.0.1 and opens it in a
// browser. It returns when the page's Quit button is pressed or on Ctrl-C.
func replayInBrowser(session *apexlog.Session, sources map[string]bubbles.DebugSource) error {
	webSources := map[string]webdebug.Source{}
	for key, s := range sources {
		webSources[key] = webdebug.Source{Name: s.Name, Lines: s.Lines, Missing: s.Missing}
	}
	server, err := webdebug.NewServer(session, webSources)
	if err != nil {
		return err
	}
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt)
	defer stop()
	return server.Run(ctx, func(url string) {
		fmt.Fprintf(os.Stderr, "Debugger running at %s\nPress Ctrl-C or the Quit button to stop.\n", url)
		if err := desktop.Open(url); err != nil {
			fmt.Fprintf(os.Stderr, "Could not open a browser: %v\n", err)
		}
	})
}

// debugSources returns the source of the anonymous Apex, classes, and
// triggers that ran in a log. Class and trigger source is fetched from the
// org.
func debugSources(session *apexlog.Session) (map[string]bubbles.DebugSource, error) {
	sources := map[string]bubbles.DebugSource{}
	var ids []string
	names := map[string]string{}
	for _, ref := range session.Sources() {
		if ref.Key == apexlog.AnonymousKey {
			sources[ref.Key] = bubbles.DebugSource{Name: ref.Name, Lines: session.Log().Header.AnonymousSource}
			continue
		}
		ids = append(ids, ref.Key)
		names[ref.Key] = ref.Name
	}
	if len(ids) == 0 {
		return sources, nil
	}
	found, err := force.ApexSourceByID(ids)
	if err != nil {
		return nil, err
	}
	for _, id := range ids {
		name := names[id]
		source, ok := found[id]
		switch {
		case !ok:
			sources[id] = bubbles.DebugSource{Name: name, Missing: fmt.Sprintf("Source for %s (%s) was not found in the org.", name, id)}
		case source.Hidden:
			sources[id] = bubbles.DebugSource{Name: qualifiedApexName(source), Missing: "Source is hidden because the class is in a managed package."}
		default:
			body := strings.ReplaceAll(source.Body, "\r\n", "\n")
			sources[id] = bubbles.DebugSource{Name: qualifiedApexName(source), Lines: strings.Split(body, "\n")}
		}
	}
	return sources, nil
}

func qualifiedApexName(source ApexSource) string {
	if source.NamespacePrefix != "" {
		return source.NamespacePrefix + "." + source.Name
	}
	return source.Name
}

func getAllLogs() {
	records, err := force.QueryLogs()
	if err != nil {
		ErrorAndExit(err.Error())
	}
	force.DisplayAllForceRecords(records)
}

func deleteLog(logId string) {
	err := force.DeleteToolingRecord("ApexLog", logId)
	if err != nil {
		ErrorAndExit(err.Error())
	}
	fmt.Println("Debug log deleted")
}

func getLog(logId string) {
	log, err := force.RetrieveLog(logId)
	if err != nil {
		ErrorAndExit(err.Error())
	}
	fmt.Println(log)
}

type logEvent struct {
	Event struct {
		CreatedDate string `json:"createdDate"`
		Type        string `json:"type"`
	} `json:"event"`
	Sobject struct {
		Id string `json:"Id"`
	} `json:"sobject"`
}

func tailLogs() {
	client := bayeux.NewClient(force)
	msgs := make(chan *bayeux.Message)
	if err := client.Subscribe("/systemTopic/Logging", msgs); err != nil {
		ErrorAndExit(err.Error())
	}
	// Disconnect from server and stop background loop.
	defer client.Close()

	// Track processed log IDs to avoid duplicates
	processedLogs := make(map[string]bool)

	for msg := range msgs {
		var newLog logEvent
		if err := json.Unmarshal(msg.Data, &newLog); err == nil {
			// Skip if we've already processed this log ID
			if processedLogs[newLog.Sobject.Id] {
				continue
			}
			processedLogs[newLog.Sobject.Id] = true
			getLog(newLog.Sobject.Id)

			// Clean up old entries to prevent unbounded memory growth
			// Keep last 100 log IDs
			if len(processedLogs) > 100 {
				// Clear and start fresh
				processedLogs = make(map[string]bool)
				processedLogs[newLog.Sobject.Id] = true
			}
		} else {
			Log.Info(fmt.Sprintf("Received unexpected message on channel %s: %s\n", msg.Channel, string(msg.Data)))
		}
	}

}

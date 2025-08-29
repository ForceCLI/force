package command

import (
	"encoding/json"
	"fmt"

	. "github.com/ForceCLI/force/error"
	. "github.com/ForceCLI/force/lib"
	"github.com/ForceCLI/force/lib/bayeux"
	"github.com/spf13/cobra"
)

func init() {
	logCmd.AddCommand(deleteLogCmd)
	logCmd.AddCommand(tailLogCmd)
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

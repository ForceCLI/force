package command

import (
	"fmt"

	. "github.com/ForceCLI/force/error"
	"github.com/ForceCLI/force/lib"
	"github.com/ForceCLI/force/lib/pubsub"
	"github.com/ForceCLI/force/lib/pubsub/proto"
	"github.com/antonmedv/expr"

	"github.com/spf13/cobra"
)

func init() {
	subscribeCmd.Flags().StringP("replayid", "r", "", "replay id to start after")
	subscribeCmd.Flags().BoolP("earliest", "e", false, "start at earliest events (default is latest)")
	subscribeCmd.Flags().BoolP("changes", "c", false, "show only changed fields (for Change Data Capture events)")
	subscribeCmd.Flags().BoolP("quiet", "q", false, "disable status messages to stderr")
	subscribeCmd.MarkFlagsMutuallyExclusive("replayid", "earliest")

	publishCmd.Flags().BoolP("quiet", "q", false, "disable status messages to stderr")

	pubsubCmd.AddCommand(subscribeCmd)
	pubsubCmd.AddCommand(publishCmd)
	RootCmd.AddCommand(pubsubCmd)
}

var pubsubCmd = &cobra.Command{
	Use:                   "pubsub subscribe [channel]",
	Short:                 "Subscribe to a pub/sub channel",
	Long:                  "Subscribe to a pub/sub channel to stream Change Data Capture or custom Platform Events",
	DisableFlagsInUseLine: true,
}

var subscribeCmd = &cobra.Command{
	Use:   "subscribe [channel]",
	Short: "Subscribe to a pub/sub channel",
	Long:  "Subscribe to a pub/sub channel to stream Change Data Capture or custom Platform Events",
	Example: `
	force pubsub subscribe /data/ChangeEvents | jq .
	force pubsub subscribe /data/AccountChangeEvent
	force pubsub subscribe /data/My_Object__ChangeEvent

	force pubsub subscribe /event/My_Event__e
	force pubsub subscribe /event/My_Channel__chn
	`,
	Run: func(cmd *cobra.Command, args []string) {
		quiet, _ := cmd.Flags().GetBool("quiet")
		if quiet {
			var l quietLogger
			lib.Log = l
		}
		replayPreset := proto.ReplayPreset_LATEST
		replayId, _ := cmd.Flags().GetString("replayid")
		if replayId != "" {
			replayPreset = proto.ReplayPreset_CUSTOM
		}
		earliest, _ := cmd.Flags().GetBool("earliest")
		if earliest {
			replayPreset = proto.ReplayPreset_EARLIEST
		}
		parseChanges, _ := cmd.Flags().GetBool("changes")
		err := pubsub.Subscribe(force, args[0], replayId, replayPreset, parseChanges)
		if err != nil {
			ErrorAndExit(err.Error())
		}
	},
	Args: cobra.ExactArgs(1),
}

var publishCmd = &cobra.Command{
	Use:   "publish <channel> <values>",
	Short: "Publish event to a pub/sub channel",
	Long:  "Publish an event to a pub/sub channel",
	Example: `
	force pubsub publish /event/My_Event__e '{My_Field__c: "My Value", CreatedDate: 946706400}'
	`,
	Run: func(cmd *cobra.Command, args []string) {
		quiet, _ := cmd.Flags().GetBool("quiet")
		if quiet {
			var l quietLogger
			lib.Log = l
		}
		channel := args[0]
		message, err := exprToMap(args[1])
		if err != nil {
			ErrorAndExit(err.Error())
		}
		err = pubsub.Publish(force, channel, message)
		if err != nil {
			ErrorAndExit(err.Error())
		}
	},
	Args: cobra.ExactArgs(2),
}

func exprToMap(e string) (map[string]any, error) {
	out, err := expr.Eval(e, nil)
	if err != nil {
		return nil, fmt.Errorf("Invalid expression: %w", err)
	}
	message, ok := out.(map[string]any)
	if !ok {
		return nil, fmt.Errorf("Could not convert expression to map")
	}
	return message, nil
}

package main

import (
	"fmt"
	"strconv"

	"github.com/ViViDboarder/gotifier"
	"github.com/heroku/force/salesforce"
)

var cmdNotifySet = &Command{
	Run:   notifySet,
	Usage: "notify (true | false)",
	Short: "Should notifications be used",
	Long: `
Determines if notifications should be used
`,
}

func notifySet(cmd *Command, args []string) {
	var err error
	shouldNotify := true
	if len(args) == 0 {
		shouldNotify = getShouldNotify()
		fmt.Println("Show notifications: " + strconv.FormatBool(shouldNotify))
	} else if len(args) == 1 {
		shouldNotify, err = strconv.ParseBool(args[0])
		if err != nil {
			fmt.Println("Expecting a boolean parameter.")
		}
	} else {
		fmt.Println("Expecting only one parameter. true/false")
	}

	setShouldNotify(shouldNotify)
}

func setShouldNotify(shouldNotify bool) {
	// Set config
	salesforce.Config.Save("notifications", "shouldNotify", strconv.FormatBool(shouldNotify))
}

func getShouldNotify() bool {
	shouldNotify := false
	notifStr, err := salesforce.Config.Load("notifications", "shouldNotify")
	if err == nil {
		shouldNotify, err = strconv.ParseBool(notifStr)
	}

	return shouldNotify
}

func notify(method string, message string) {
	shouldNotify := getShouldNotify()
	if shouldNotify {
		gotifier.Notification{Title: "Force Cli", Subtitle: method, Message: message}.Push()
	}
}

func notifySuccess(method string, success bool) {
	if success {
		notify(method, "SUCCESS")
	} else {
		notify(method, "FAILURE")
	}
}

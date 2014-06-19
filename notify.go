package main

import (
	"github.com/ViViDboarder/gotifier"
)

func notify(method string, message string) {
	// TODO: Store desire to notify somewhere in a settings file
	gotifier.Notification{Title: "Force Cli", Subtitle: method, Message: message}.Push()
}

func notifySuccess(method string, success bool) {
	if success {
		notify(method, "SUCCESS")
	} else {
		notify(method, "FAILURE")
	}
}

package desktop

import (
	"strconv"

	"github.com/ViViDboarder/gotifier"
	. "github.com/heroku/force/config"
)

func Notify(method string, message string) {
	shouldNotify := GetShouldNotify()
	if shouldNotify {
		gotifier.Notification{Title: "Force Cli", Subtitle: method, Message: message}.Push()
	}
}

func GetShouldNotify() bool {
	shouldNotify := false
	notifStr, err := Config.Load("notifications", "shouldNotify")
	if err == nil {
		shouldNotify, err = strconv.ParseBool(notifStr)
	}

	return shouldNotify
}

func SetShouldNotify(shouldNotify bool) {
	// Set config
	Config.Save("notifications", "shouldNotify", strconv.FormatBool(shouldNotify))
}

func NotifySuccess(method string, success bool) {
	if success {
		Notify(method, "SUCCESS")
	} else {
		Notify(method, "FAILURE")
	}
}

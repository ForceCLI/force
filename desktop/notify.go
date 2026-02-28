package desktop

import (
	"strconv"

	. "github.com/ForceCLI/force/config"
	"github.com/gen2brain/beeep"
)

func Notify(method string, message string) {
	shouldNotify := GetShouldNotify()
	if shouldNotify {
		_ = beeep.Notify("Force Cli: "+method, message, "")
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

package gotifier

import (
	//"log"
	"os/exec"
)

const (
	terminalNotifier = "terminal-notifier"
	notifySend       = "notify-send"
)

type Notification struct {
	Title, Subtitle, Message string
}

func (notif Notification) Push() bool {
	//log.Println("Pushing notification")
	if activeCmd == nil {
		//log.Println("No active notification command found")
		return false
	}

	cmd := (*activeCmd).Command(notif)

	cmd.Run()
	return true
}

type NotifierCommand struct {
	Name, Path string
}

func (cmd NotifierCommand) Command(notif Notification) *exec.Cmd {

	// message is required
	message := nonNull(notif.Message, notif.Title, notif.Subtitle)
	var args = []string{""}

	switch cmd.Name {
	case terminalNotifier:
		// Append message
		args = append(args, "-message")
		args = append(args, "\""+message+"\"")

		if notif.Title != "" {
			args = append(args, "-title")
			args = append(args, "\""+notif.Title+"\"")
		}
		if notif.Subtitle != "" {
			args = append(args, "-subtitle")
			args = append(args, "\""+notif.Subtitle+"\"")
		}

	case notifySend:
		if notif.Title != "" {
			args = append(args, notif.Title)
		}
		args = append(args, message)
	}

	//return log.Sprintf("%s %s", cmd.Path, strings.Join(args, " "))
	return &exec.Cmd{Path: cmd.Path, Args: args}
}

var activeCmd *NotifierCommand

var notifiers = []NotifierCommand{
	NotifierCommand{Name: "terminal-notifier"},
	NotifierCommand{Name: "notify-send"},
}

func nonNull(vals ...string) string {
	for _, val := range vals {
		if val != "" {
			return val
		}
	}

	return ""
}

func init() {
	//log.Println("Initing Gotifier")
	for _, cmd := range notifiers {
		path, err := exec.LookPath(cmd.Name)
		if err == nil && path != "" {
			activeCmd = &cmd
			activeCmd.Path = path
			//log.Println("Found notifier command: " + path)
			return
		}
	}
}

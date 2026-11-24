package plugins

import (
	"fmt"
	"strings"

	"github.com/ForceCLI/force/sfplugin"
)

// Match is the plugin command that the first words of a command line name.
type Match struct {
	Record  Record
	Command sfplugin.Command
	// Words is the number of leading words that form the command id.
	Words int
}

// Find returns the plugin command whose id is the longest prefix of words.
// words are the arguments before the first flag.
func Find(records []Record, words []string) (Match, bool) {
	var best Match
	found := false
	for _, record := range records {
		for _, command := range record.Manifest.Commands {
			id := command.Words()
			if len(id) > len(words) || (found && len(id) <= best.Words) {
				continue
			}
			if strings.Join(words[:len(id)], " ") == command.ID {
				best = Match{Record: record, Command: command, Words: len(id)}
				found = true
			}
		}
	}
	return best, found
}

// CheckConflicts returns an error when a command of manifest has the id of
// a built-in command or of a command of another recorded plugin.  isBuiltin
// reports whether the words name a built-in command.
func CheckConflicts(records []Record, manifest sfplugin.Manifest, isBuiltin func(words []string) bool) error {
	for _, command := range manifest.Commands {
		if isBuiltin != nil && isBuiltin(command.Words()) {
			return fmt.Errorf("plugin %s provides the command %q, which is a built-in command", manifest.Name, command.ID)
		}
		for _, record := range records {
			if record.Name == manifest.Name {
				continue
			}
			for _, existing := range record.Manifest.Commands {
				if existing.ID == command.ID {
					return fmt.Errorf("plugin %s provides the command %q, which the installed plugin %s also provides", manifest.Name, command.ID, record.Name)
				}
			}
		}
	}
	return nil
}

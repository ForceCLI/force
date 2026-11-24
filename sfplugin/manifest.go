// Package sfplugin defines the protocol between the sf-compatible mode of
// the force CLI and its plugins.  A plugin is one executable.  The CLI runs
// it as a child process with the command words and flags as arguments, and
// passes context in environment variables.
//
// The package imports only the standard library so that a plugin can use it
// without the rest of the force module's dependencies.
package sfplugin

import (
	"encoding/json"
	"fmt"
	"strings"
)

// ProtocolVersion is the version of the protocol this package implements.
const ProtocolVersion = 1

// DescribeArg is the argument that asks a plugin executable for its
// manifest.  Run with this single argument, the executable prints the
// manifest as JSON on stdout and exits with status 0.
const DescribeArg = "sf-plugin-manifest"

// OrgRequirement says whether a command uses an org.
type OrgRequirement string

const (
	// OrgNone is for a command that makes no requests to an org.  The CLI
	// sets no org variables.
	OrgNone OrgRequirement = "none"
	// OrgOptional is for a command that uses an org when one is selected.
	// The CLI sets the org variables when it can resolve an org.
	OrgOptional OrgRequirement = "optional"
	// OrgRequired is for a command that cannot run without an org.  The
	// CLI reports an error instead of starting the plugin when it cannot
	// resolve one.
	OrgRequired OrgRequirement = "required"
)

// Manifest describes a plugin: its name and version and the commands it
// provides.
type Manifest struct {
	Name     string    `json:"name"`
	Version  string    `json:"version"`
	Protocol int       `json:"protocol"`
	Commands []Command `json:"commands"`
	Topics   []Topic   `json:"topics,omitempty"`
}

// Command is one command of a plugin.  ID is the words a user types after
// the program name, separated by single spaces, such as "sgd source delta".
type Command struct {
	ID      string         `json:"id"`
	Summary string         `json:"summary,omitempty"`
	Org     OrgRequirement `json:"org,omitempty"`
}

// Topic describes a prefix of command ids, such as "sgd", for listings.
type Topic struct {
	ID      string `json:"id"`
	Summary string `json:"summary,omitempty"`
}

// OrgRequirement returns the org requirement of the command.  A command
// that does not state one uses no org.
func (c Command) OrgRequirement() OrgRequirement {
	if c.Org == "" {
		return OrgNone
	}
	return c.Org
}

// Words returns the words of the command id.
func (c Command) Words() []string {
	return strings.Fields(c.ID)
}

// Validate reports the first problem that makes the manifest unusable.
func (m Manifest) Validate() error {
	if m.Name == "" {
		return fmt.Errorf("plugin manifest has no name")
	}
	if strings.ContainsAny(m.Name, " \t\r\n/\\") {
		return fmt.Errorf("plugin name %q contains a space or a path separator", m.Name)
	}
	if m.Protocol != ProtocolVersion {
		return fmt.Errorf("plugin %s uses protocol %d; this program supports protocol %d", m.Name, m.Protocol, ProtocolVersion)
	}
	if len(m.Commands) == 0 {
		return fmt.Errorf("plugin %s provides no commands", m.Name)
	}
	seen := map[string]bool{}
	for _, command := range m.Commands {
		words := command.Words()
		if len(words) == 0 {
			return fmt.Errorf("plugin %s has a command with an empty id", m.Name)
		}
		if strings.Join(words, " ") != command.ID {
			return fmt.Errorf("plugin %s: command id %q must be words separated by single spaces", m.Name, command.ID)
		}
		for _, word := range words {
			if strings.HasPrefix(word, "-") || strings.Contains(word, ":") {
				return fmt.Errorf("plugin %s: command id %q has a word that starts with - or contains a colon", m.Name, command.ID)
			}
		}
		if seen[command.ID] {
			return fmt.Errorf("plugin %s lists the command %q more than once", m.Name, command.ID)
		}
		seen[command.ID] = true
		switch command.Org {
		case "", OrgNone, OrgOptional, OrgRequired:
		default:
			return fmt.Errorf("plugin %s: command %q has the org requirement %q; use none, optional or required", m.Name, command.ID, command.Org)
		}
	}
	return nil
}

// ParseManifest decodes and validates a manifest.
func ParseManifest(data []byte) (Manifest, error) {
	var manifest Manifest
	if err := json.Unmarshal(data, &manifest); err != nil {
		return Manifest{}, fmt.Errorf("plugin manifest is not valid JSON: %w", err)
	}
	if err := manifest.Validate(); err != nil {
		return Manifest{}, err
	}
	return manifest, nil
}

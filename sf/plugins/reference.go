// Package plugins installs, records and finds the plugins of the
// sf-compatible mode.  A plugin is one executable for each operating system
// and architecture, published as an OCI artifact on a container registry.
package plugins

import (
	"fmt"
	"os"
	"strings"

	"oras.land/oras-go/v2/registry"
)

// DefaultRegistryPrefix is put in front of a plugin name that has no
// registry host: "sfdx-git-delta" is read as
// "ghcr.io/forcecli/sf-plugins/sfdx-git-delta".
const DefaultRegistryPrefix = "ghcr.io/forcecli/sf-plugins"

// RegistryPrefixEnv names the environment variable that replaces
// DefaultRegistryPrefix.
const RegistryPrefixEnv = "SF_PLUGINS_REGISTRY"

// DefaultTag is the tag used when a reference has none.
const DefaultTag = "latest"

// Reference names a plugin in a registry: host, repository path and tag.
type Reference struct {
	Registry   string
	Repository string
	Tag        string
}

// Name returns the reference without the tag.
func (r Reference) Name() string {
	return r.Registry + "/" + r.Repository
}

func (r Reference) String() string {
	return r.Name() + ":" + r.Tag
}

// RegistryPrefix returns the prefix for names without a registry host.
func RegistryPrefix() string {
	if prefix := strings.Trim(os.Getenv(RegistryPrefixEnv), "/"); prefix != "" {
		return prefix
	}
	return DefaultRegistryPrefix
}

// ParseReference reads "name" or "name:tag".  A name whose first path
// segment is not a registry host (it has no "." and no ":", and is not
// "localhost") is placed under RegistryPrefix.  A digest is not accepted;
// the digest of an installed plugin is recorded at install.
func ParseReference(text string) (Reference, error) {
	if text == "" {
		return Reference{}, fmt.Errorf("plugin name is empty")
	}
	if strings.Contains(text, "://") {
		return Reference{}, fmt.Errorf("plugin name %q must not have a URL scheme", text)
	}
	if strings.Contains(text, "@") {
		return Reference{}, fmt.Errorf("plugin name %q must not have a digest; give a tag", text)
	}
	name, tag := text, DefaultTag
	if colon := strings.LastIndex(text, ":"); colon > strings.LastIndex(text, "/") {
		name, tag = text[:colon], text[colon+1:]
		if tag == "" {
			return Reference{}, fmt.Errorf("plugin name %q has an empty tag", text)
		}
	}
	first, _, _ := strings.Cut(name, "/")
	hasHost := strings.Contains(name, "/") && (strings.ContainsAny(first, ".:") || first == "localhost")
	if !hasHost {
		name = RegistryPrefix() + "/" + name
	}
	parsed, err := registry.ParseReference(name + ":" + tag)
	if err != nil {
		return Reference{}, fmt.Errorf("plugin name %q is not a valid registry reference: %w", text, err)
	}
	if parsed.Repository == "" {
		return Reference{}, fmt.Errorf("plugin name %q has no repository", text)
	}
	return Reference{Registry: parsed.Registry, Repository: parsed.Repository, Tag: tag}, nil
}

package sf

import (
	"context"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"sort"
	"strings"
	"time"

	"github.com/spf13/cobra"

	"github.com/ForceCLI/force/sf/plugins"
	"github.com/ForceCLI/force/sfplugin"
)

// pluginInfo is a recorded plugin as the plugins commands print it with
// --json.
type pluginInfo struct {
	Name           string   `json:"name"`
	Version        string   `json:"version"`
	Type           string   `json:"type"`
	Root           string   `json:"root"`
	Reference      string   `json:"reference,omitempty"`
	Tag            string   `json:"tag,omitempty"`
	IndexDigest    string   `json:"indexDigest,omitempty"`
	ManifestDigest string   `json:"manifestDigest,omitempty"`
	Commands       []string `json:"commands"`
}

func pluginInfoOf(record plugins.Record) pluginInfo {
	info := pluginInfo{
		Name:           record.Name,
		Version:        record.Manifest.Version,
		Type:           "user",
		Root:           record.Executable,
		Reference:      record.Reference,
		Tag:            record.Tag,
		IndexDigest:    record.IndexDigest,
		ManifestDigest: record.ManifestDigest,
		Commands:       []string{},
	}
	if record.Linked {
		info.Type = "link"
	}
	for _, command := range record.Manifest.Commands {
		info.Commands = append(info.Commands, strings.ReplaceAll(command.ID, " ", ":"))
	}
	return info
}

func pluginError(err error) error {
	if _, ok := err.(*Error); ok {
		return err
	}
	return &Error{Name: "PluginError", Message: err.Error(), ExitCode: 1}
}

func newPluginsCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "plugins",
		Short: "List installed plugins.",
		Args:  cobra.NoArgs,
		RunE: func(cmd *cobra.Command, args []string) error {
			store, err := plugins.OpenDefault()
			if err != nil {
				return pluginError(err)
			}
			records, err := store.Load()
			if err != nil {
				return pluginError(err)
			}
			infos := make([]pluginInfo, 0, len(records))
			for _, record := range records {
				infos = append(infos, pluginInfoOf(record))
			}
			if jsonRequested(cmd) {
				// sf prints this array without the status envelope.
				return writeJSON(cmd.OutOrStdout(), infos)
			}
			out := cmd.OutOrStdout()
			if len(records) == 0 {
				fmt.Fprintln(out, "No plugins installed.")
				return nil
			}
			for _, record := range records {
				line := record.Name + " " + record.Manifest.Version
				if record.Linked {
					line += " (link) " + record.Executable
				} else {
					line += " (" + record.Reference + ":" + record.Tag + ")"
				}
				fmt.Fprintln(out, line)
			}
			return nil
		},
	}
	addJSONFlag(cmd)
	return withSubcommands(cmd,
		newPluginsInstallCmd(),
		newPluginsUninstallCmd(),
		newPluginsUpdateCmd(),
		newPluginsInspectCmd(),
		newPluginsLinkCmd(),
		newPluginsPublishCmd(),
	)
}

func pluginClient(plainHTTP bool) *plugins.Client {
	client := plugins.NewClient()
	client.PlainHTTP = plainHTTP
	return client
}

// installPlugin resolves a reference for this platform, downloads the
// executable and records the plugin.
func installPlugin(ctx context.Context, cmd *cobra.Command, store *plugins.Store, ref plugins.Reference, plainHTTP bool) (plugins.Record, error) {
	client := pluginClient(plainHTTP)
	resolved, err := client.Resolve(ctx, ref, plugins.HostPlatform())
	if err != nil {
		return plugins.Record{}, err
	}
	records, err := store.Load()
	if err != nil {
		return plugins.Record{}, err
	}
	root := cmd.Root()
	if err := plugins.CheckConflicts(records, resolved.Manifest, func(words []string) bool { return isBuiltinCommand(root, words) }); err != nil {
		return plugins.Record{}, err
	}
	path, err := client.Fetch(ctx, resolved, store)
	if err != nil {
		return plugins.Record{}, err
	}
	record := plugins.Record{
		Name:           resolved.Manifest.Name,
		Reference:      ref.Name(),
		Tag:            ref.Tag,
		IndexDigest:    resolved.IndexDigest,
		ManifestDigest: resolved.ManifestDigest,
		PlainHTTP:      plainHTTP,
		Executable:     path,
		Manifest:       resolved.Manifest,
	}
	if err := store.Put(record); err != nil {
		return plugins.Record{}, err
	}
	return record, nil
}

func newPluginsInstallCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:     "install plugin...",
		Aliases: []string{"add"},
		Short:   "Install a plugin from a container registry.",
		Long: "Install a plugin from a container registry.\n\n" +
			"A plugin is named by a registry reference with an optional tag, such as\n" +
			"ghcr.io/acme/my-plugin:v1.2.0.  A name without a registry host, such as\n" +
			"sfdx-git-delta, is looked up under " + plugins.DefaultRegistryPrefix + ".\n" +
			"The tag defaults to latest.",
		Args: cobra.MinimumNArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			store, err := plugins.OpenDefault()
			if err != nil {
				return pluginError(err)
			}
			plainHTTP, _ := cmd.Flags().GetBool("plain-http")
			var installed []pluginInfo
			for _, arg := range args {
				ref, err := plugins.ParseReference(arg)
				if err != nil {
					return pluginError(err)
				}
				record, err := installPlugin(cmd.Context(), cmd, store, ref, plainHTTP)
				if err != nil {
					return pluginError(err)
				}
				installed = append(installed, pluginInfoOf(record))
			}
			return emit(cmd, installed, func(w io.Writer) {
				for _, info := range installed {
					fmt.Fprintf(w, "Installed %s %s from %s:%s\n", info.Name, info.Version, info.Reference, info.Tag)
					fmt.Fprintf(w, "  index digest:    %s\n", info.IndexDigest)
					fmt.Fprintf(w, "  manifest digest: %s\n", info.ManifestDigest)
				}
			})
		},
	}
	cmd.Flags().Bool("plain-http", false, "Use HTTP instead of HTTPS for the registry.")
	addJSONFlag(cmd)
	return cmd
}

func newPluginsUninstallCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:     "uninstall plugin...",
		Aliases: []string{"remove", "unlink"},
		Short:   "Remove a plugin.",
		Args:    cobra.MinimumNArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			store, err := plugins.OpenDefault()
			if err != nil {
				return pluginError(err)
			}
			var removed []pluginInfo
			for _, name := range args {
				record, found, err := store.Remove(name)
				if err != nil {
					return pluginError(err)
				}
				if !found {
					return &Error{Name: "PluginError", Message: fmt.Sprintf("%s is not installed.", name), ExitCode: 1}
				}
				removed = append(removed, pluginInfoOf(record))
			}
			return emit(cmd, removed, func(w io.Writer) {
				for _, info := range removed {
					fmt.Fprintf(w, "Uninstalled %s %s\n", info.Name, info.Version)
				}
			})
		},
	}
	addJSONFlag(cmd)
	return cmd
}

type pluginUpdate struct {
	pluginInfo
	Updated         bool   `json:"updated"`
	PreviousVersion string `json:"previousVersion,omitempty"`
	PreviousDigest  string `json:"previousDigest,omitempty"`
}

func newPluginsUpdateCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "update [plugin...]",
		Short: "Update installed plugins.",
		Long: "Update installed plugins.\n\n" +
			"The tag each plugin was installed with is looked up again.  A plugin is\n" +
			"downloaded when the tag points to a different digest than the one recorded.\n" +
			"Linked plugins are not changed.  Without arguments every installed plugin\n" +
			"is checked.",
		RunE: func(cmd *cobra.Command, args []string) error {
			store, err := plugins.OpenDefault()
			if err != nil {
				return pluginError(err)
			}
			records, err := store.Load()
			if err != nil {
				return pluginError(err)
			}
			byName := map[string]plugins.Record{}
			for _, record := range records {
				byName[record.Name] = record
			}
			var selected []plugins.Record
			if len(args) == 0 {
				for _, record := range records {
					if !record.Linked {
						selected = append(selected, record)
					}
				}
			}
			for _, name := range args {
				record, ok := byName[name]
				if !ok {
					return &Error{Name: "PluginError", Message: fmt.Sprintf("%s is not installed.", name), ExitCode: 1}
				}
				if record.Linked {
					return &Error{Name: "PluginError", Message: fmt.Sprintf("%s is a linked plugin; it is not updated from a registry.", name), ExitCode: 1}
				}
				selected = append(selected, record)
			}
			results := []pluginUpdate{}
			for _, record := range selected {
				ref, err := plugins.ParseReference(record.Reference + ":" + record.Tag)
				if err != nil {
					return pluginError(err)
				}
				resolved, err := pluginClient(record.PlainHTTP).Resolve(cmd.Context(), ref, plugins.HostPlatform())
				if err != nil {
					return pluginError(err)
				}
				if resolved.IndexDigest == record.IndexDigest {
					results = append(results, pluginUpdate{pluginInfo: pluginInfoOf(record)})
					continue
				}
				updated, err := installPlugin(cmd.Context(), cmd, store, ref, record.PlainHTTP)
				if err != nil {
					return pluginError(err)
				}
				results = append(results, pluginUpdate{
					pluginInfo:      pluginInfoOf(updated),
					Updated:         true,
					PreviousVersion: record.Manifest.Version,
					PreviousDigest:  record.IndexDigest,
				})
			}
			return emit(cmd, results, func(w io.Writer) {
				if len(results) == 0 {
					fmt.Fprintln(w, "No plugins installed from a registry.")
				}
				for _, result := range results {
					if result.Updated {
						fmt.Fprintf(w, "Updated %s from %s to %s (%s)\n", result.Name, result.PreviousVersion, result.Version, result.IndexDigest)
					} else {
						fmt.Fprintf(w, "%s %s is up to date\n", result.Name, result.Version)
					}
				}
			})
		},
	}
	addJSONFlag(cmd)
	return cmd
}

func newPluginsInspectCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "inspect plugin...",
		Short: "Display the recorded details of a plugin.",
		Args:  cobra.MinimumNArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			store, err := plugins.OpenDefault()
			if err != nil {
				return pluginError(err)
			}
			var records []plugins.Record
			for _, name := range args {
				record, found, err := store.Get(name)
				if err != nil {
					return pluginError(err)
				}
				if !found {
					return &Error{Name: "PluginError", Message: fmt.Sprintf("%s is not installed.", name), ExitCode: 1}
				}
				records = append(records, record)
			}
			if jsonRequested(cmd) {
				// sf prints this array without the status envelope.
				return writeJSON(cmd.OutOrStdout(), records)
			}
			out := cmd.OutOrStdout()
			for _, record := range records {
				fmt.Fprintf(out, "%s %s\n", record.Name, record.Manifest.Version)
				if record.Linked {
					fmt.Fprintf(out, "  type:            link\n")
				} else {
					fmt.Fprintf(out, "  type:            user\n")
					fmt.Fprintf(out, "  reference:       %s:%s\n", record.Reference, record.Tag)
					fmt.Fprintf(out, "  index digest:    %s\n", record.IndexDigest)
					fmt.Fprintf(out, "  manifest digest: %s\n", record.ManifestDigest)
				}
				fmt.Fprintf(out, "  executable:      %s\n", record.Executable)
				fmt.Fprintf(out, "  commands:\n")
				for _, command := range record.Manifest.Commands {
					fmt.Fprintf(out, "    %s (org: %s)\n", command.ID, command.OrgRequirement())
				}
			}
			return nil
		},
	}
	addJSONFlag(cmd)
	return cmd
}

// describePlugin runs an executable with the argument that asks for its
// manifest and returns the manifest.
func describePlugin(ctx context.Context, path string) (sfplugin.Manifest, error) {
	ctx, cancel := context.WithTimeout(ctx, 30*time.Second)
	defer cancel()
	output, err := exec.CommandContext(ctx, path, sfplugin.DescribeArg).Output()
	if err != nil {
		return sfplugin.Manifest{}, fmt.Errorf("running \"%s %s\": %w", path, sfplugin.DescribeArg, err)
	}
	return sfplugin.ParseManifest(output)
}

func newPluginsLinkCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "link path",
		Short: "Register a plugin executable on this machine for development.",
		Long: "Register a plugin executable on this machine for development.\n\n" +
			"The executable is run with the argument " + sfplugin.DescribeArg + " to read its\n" +
			"manifest.  Its commands then run the executable at that path, so a rebuilt\n" +
			"executable is used without linking again.  Link again after the manifest\n" +
			"changes.",
		Args: cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			path, err := filepath.Abs(args[0])
			if err != nil {
				return pluginError(err)
			}
			if info, err := os.Stat(path); err != nil {
				return pluginError(err)
			} else if info.IsDir() {
				return &Error{Name: "PluginError", Message: fmt.Sprintf("%s is a directory; give the path of the plugin executable.", path), ExitCode: 1}
			}
			manifest, err := describePlugin(cmd.Context(), path)
			if err != nil {
				return pluginError(err)
			}
			store, err := plugins.OpenDefault()
			if err != nil {
				return pluginError(err)
			}
			records, err := store.Load()
			if err != nil {
				return pluginError(err)
			}
			root := cmd.Root()
			if err := plugins.CheckConflicts(records, manifest, func(words []string) bool { return isBuiltinCommand(root, words) }); err != nil {
				return pluginError(err)
			}
			record := plugins.Record{Name: manifest.Name, Executable: path, Linked: true, Manifest: manifest}
			if err := store.Put(record); err != nil {
				return pluginError(err)
			}
			info := pluginInfoOf(record)
			return emit(cmd, info, func(w io.Writer) {
				fmt.Fprintf(w, "Linked %s %s to %s\n", info.Name, info.Version, info.Root)
			})
		},
	}
	addJSONFlag(cmd)
	return cmd
}

func newPluginsPublishCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "publish name:tag",
		Short: "Publish plugin executables to a container registry.",
		Long: "Publish plugin executables to a container registry.\n\n" +
			"Give one --executable for each platform, in the form os/arch=path, with the\n" +
			"names Go uses for GOOS and GOARCH.  The plugin manifest is read from\n" +
			"--manifest, or from the executable for this platform when --manifest is not\n" +
			"given.  The registry credentials are those of the Docker configuration,\n" +
			"which \"docker login\" writes.  sf has no such command.",
		Args: cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			ref, err := plugins.ParseReference(args[0])
			if err != nil {
				return pluginError(err)
			}
			values, _ := cmd.Flags().GetStringArray("executable")
			if len(values) == 0 {
				return missingFlagError("executable")
			}
			executables := map[plugins.Platform]string{}
			for _, value := range values {
				name, path, ok := strings.Cut(value, "=")
				if !ok || path == "" {
					return &Error{Name: "PluginError", Message: fmt.Sprintf("--executable %q is not in the form os/arch=path", value), ExitCode: 1}
				}
				platform, err := plugins.ParsePlatform(name)
				if err != nil {
					return pluginError(err)
				}
				if _, duplicate := executables[platform]; duplicate {
					return &Error{Name: "PluginError", Message: fmt.Sprintf("--executable is given more than once for %s", platform), ExitCode: 1}
				}
				executables[platform] = path
			}
			var manifest sfplugin.Manifest
			if manifestPath, _ := cmd.Flags().GetString("manifest"); manifestPath != "" {
				data, err := os.ReadFile(manifestPath)
				if err != nil {
					return pluginError(err)
				}
				if manifest, err = sfplugin.ParseManifest(data); err != nil {
					return pluginError(err)
				}
			} else {
				path, ok := executables[plugins.HostPlatform()]
				if !ok {
					return &Error{Name: "PluginError", Message: fmt.Sprintf("Give --manifest, or an --executable for this platform (%s) to read the manifest from.", plugins.HostPlatform()), ExitCode: 1}
				}
				absolute, err := filepath.Abs(path)
				if err != nil {
					return pluginError(err)
				}
				if manifest, err = describePlugin(cmd.Context(), absolute); err != nil {
					return pluginError(err)
				}
			}
			plainHTTP, _ := cmd.Flags().GetBool("plain-http")
			source, _ := cmd.Flags().GetString("source")
			revision, _ := cmd.Flags().GetString("revision")
			overwrite, _ := cmd.Flags().GetBool("overwrite")
			result, err := pluginClient(plainHTTP).Publish(cmd.Context(), ref, manifest, executables, plugins.PublishOptions{
				Source:    source,
				Revision:  revision,
				Overwrite: overwrite,
			})
			if err != nil {
				return pluginError(err)
			}
			output := orderedFields(
				"name", manifest.Name,
				"version", manifest.Version,
				"reference", ref.String(),
				"indexDigest", result.IndexDigest,
				"platforms", result.Platforms,
				"replaced", result.Replaced,
			)
			return emit(cmd, output, func(w io.Writer) {
				fmt.Fprintf(w, "Published %s %s to %s\n", manifest.Name, manifest.Version, ref)
				fmt.Fprintf(w, "  index digest: %s\n", result.IndexDigest)
				platforms := make([]string, 0, len(result.Platforms))
				for platform := range result.Platforms {
					platforms = append(platforms, platform)
				}
				sort.Strings(platforms)
				for _, platform := range platforms {
					fmt.Fprintf(w, "  %s: %s\n", platform, result.Platforms[platform])
				}
				if result.Replaced != "" {
					fmt.Fprintf(w, "  the tag pointed to %s before\n", result.Replaced)
				}
			})
		},
	}
	cmd.Flags().StringArray("executable", nil, "An executable to publish, as os/arch=path.  Repeat for each platform.")
	cmd.Flags().String("manifest", "", "Path of the plugin manifest JSON file.")
	cmd.Flags().String("source", "", "URL of the source repository.")
	cmd.Flags().String("revision", "", "Source control revision the executables were built from.")
	cmd.Flags().Bool("overwrite", false, "Move the tag when it already exists.")
	cmd.Flags().Bool("plain-http", false, "Use HTTP instead of HTTPS for the registry.")
	addJSONFlag(cmd)
	return cmd
}

// pluginCommandInfos returns the commands of the recorded plugins for the
// "commands" listing.
func pluginCommandInfos() []commandInfo {
	store, err := plugins.OpenDefault()
	if err != nil {
		return nil
	}
	records, err := store.Load()
	if err != nil {
		return nil
	}
	var commands []commandInfo
	for _, record := range records {
		for _, command := range record.Manifest.Commands {
			commands = append(commands, commandInfo{
				Id:      strings.ReplaceAll(command.ID, " ", ":"),
				Summary: command.Summary,
				Flags:   []string{},
			})
		}
	}
	return commands
}

// pluginOfCommand returns the name of the recorded plugin that provides the
// command with exactly these words.
func pluginOfCommand(words []string) (string, bool) {
	store, err := plugins.OpenDefault()
	if err != nil {
		return "", false
	}
	records, err := store.Load()
	if err != nil {
		return "", false
	}
	match, ok := plugins.Find(records, words)
	if !ok || match.Words != len(words) {
		return "", false
	}
	return match.Record.Name, true
}

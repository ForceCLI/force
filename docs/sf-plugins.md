# Plugins For sf-Compatible Mode

The Salesforce `sf` CLI loads plugins that are npm packages. The sf-compatible mode of `force` has no Node.js runtime and cannot load them. Its plugins are native executables:

- A plugin is one executable file for each operating system and architecture.
- The mode runs the executable as a child process and passes the command line to it unchanged.
- Plugins are published as OCI artifacts on a container registry, such as GitHub Container Registry (`ghcr.io`).

An sf plugin has to be ported to an executable that follows the protocol below before this mode can run it.

## Using Plugins

```
sf plugins install sfdx-git-delta
sf plugins install ghcr.io/acme/my-plugin:v1.2.0
sf plugins
sf plugins inspect sfdx-git-delta
sf plugins update
sf plugins uninstall sfdx-git-delta
```

| Command | What it does |
|---|---|
| `sf plugins` | Lists installed and linked plugins. `--json` prints an array with `name`, `version`, `type` (`user` or `link`), `root` (the executable path), `reference`, `tag`, `indexDigest`, `manifestDigest` and `commands`. |
| `sf plugins install name[:tag]...` | Downloads the executable for this platform and records the plugin. Also run as `sf plugins add`. `--plain-http` uses HTTP for a registry on a local machine. |
| `sf plugins update [name...]` | Looks up the tag each plugin was installed with. Downloads the plugin when the tag points to a different digest than the one recorded. Without names, checks every plugin installed from a registry. |
| `sf plugins uninstall name...` | Removes the record and the downloaded executable. Also run as `sf plugins remove` and `sf plugins unlink`. |
| `sf plugins inspect name...` | Prints the recorded reference, digests, executable path and commands. |
| `sf plugins link path` | Registers an executable on this machine, for development. |
| `sf plugins publish name:tag` | Publishes executables to a registry. sf has no such command. |

### Names

A plugin is named by a registry reference: host, repository path and an optional tag. The tag defaults to `latest`.

A name whose first path segment is not a registry host (it has no `.` and no `:`, and is not `localhost`) is placed under `ghcr.io/forcecli/sf-plugins`:

| Typed | Reference |
|---|---|
| `sfdx-git-delta` | `ghcr.io/forcecli/sf-plugins/sfdx-git-delta:latest` |
| `sfdx-git-delta:v7.7.2` | `ghcr.io/forcecli/sf-plugins/sfdx-git-delta:v7.7.2` |
| `ghcr.io/acme/my-plugin:v1.2.0` | `ghcr.io/acme/my-plugin:v1.2.0` |
| `localhost:5000/test/my-plugin:dev` | `localhost:5000/test/my-plugin:dev` |

The environment variable `SF_PLUGINS_REGISTRY` replaces the prefix `ghcr.io/forcecli/sf-plugins`. A digest (`name@sha256:...`) is not accepted as a name.

### Where Plugins Are Stored

| Path | Content |
|---|---|
| `~/.sf/plugins/plugins.json` | One record for each plugin: name, reference, tag, the digest the tag pointed to at install, the digest of the manifest for this platform, the executable path, and a copy of the plugin manifest. |
| `~/.sf/plugins/store/sha256/<manifest digest>/<executable>` | The downloaded executable, with mode 0755. |

`SF_PLUGINS_DIR` names another directory in place of `~/.sf/plugins`. The executable of a linked plugin stays where it is; only its path is recorded.

### Which Commands Run A Plugin

- A built-in command always runs the built-in code. A plugin whose manifest lists the id of a built-in command, or of a command of another installed plugin, is refused at install and at link.
- Otherwise the words before the first flag are compared with the command ids of the installed plugins, and the longest id that matches is run.
- Only installed and linked plugins run. Executables on the `PATH` are not searched.
- `sf commands` lists plugin commands, `sf which <command>` names the plugin, and `sf help <command>` runs the plugin command with `--help`.
- A command line that names only the first words of plugin command ids, such as `sf sgd`, lists the commands that start with those words.
- When no built-in command and no plugin command matches, the mode prints the message sf prints for an unknown command and exits with status 127.

### Trust

A plugin executable runs with the permissions of the user who runs `sf`. Install only plugins from publishers you trust.

- A download is checked against the SHA-256 digest in the registry manifest before the file is put in the store. A file that does not match is discarded.
- The digests are recorded in `plugins.json`, and `sf plugins install` prints them.
- Plugins are not signed, and this mode does not verify signatures. A tag can be moved by anyone who can write to the registry repository; `sf plugins update` installs whatever the tag points to at that time.
- A plugin command with the org requirement `optional` or `required` receives an access token for the target org (see below). A command with the requirement `none` receives no org values.

### Registry Credentials

Requests use the credentials of the Docker configuration (`~/.docker/config.json`, or `$DOCKER_CONFIG/config.json`, and its credential helpers). `docker login ghcr.io` stores them. Without stored credentials for a registry the requests are anonymous, which is enough to install a public plugin.

## The Plugin Protocol

Protocol version: 1. The Go package `github.com/ForceCLI/force/sfplugin` holds the types and names below and depends only on the standard library. A plugin can be written in any language that produces an executable.

### Manifest

A plugin describes itself with a manifest:

```json
{
  "name": "sfdx-git-delta",
  "version": "7.7.2",
  "protocol": 1,
  "commands": [
    {
      "id": "sgd source delta",
      "summary": "Generate incremental package manifest and source content",
      "org": "none"
    }
  ],
  "topics": [
    { "id": "sgd", "summary": "Commands of sfdx-git-delta" }
  ]
}
```

| Field | Meaning |
|---|---|
| `name` | The plugin name. No spaces or path separators. It is the file name of the executable (with `.exe` on Windows) and the name `sf plugins uninstall` takes. |
| `version` | The version shown in listings. |
| `protocol` | `1`. A manifest with another value is refused. |
| `commands[].id` | The words a user types after `sf`, separated by single spaces. No word starts with `-` or contains `:`. |
| `commands[].summary` | One line for `sf commands`. |
| `commands[].org` | `none` (the default), `optional` or `required`. See "Org Access". |
| `topics[]` | Optional descriptions of id prefixes. |

Run with the single argument `sf-plugin-manifest`, the executable prints its manifest as JSON on stdout and exits with status 0. `sf plugins link` and `sf plugins publish` read the manifest this way. For an installed plugin the manifest is read from the registry, and the executable is not run until one of its commands is.

### Arguments

The executable is started with the command line that followed the program name, beginning with the words of the command id. For `sf sgd source delta --from main -o out`, the arguments are:

```
sgd source delta --from main -o out
```

- The colon form `sf sgd:source:delta` is split into words first.
- Flags are passed exactly as typed. The mode does not rewrite legacy flag names, expand `--flags-dir`, or check flags for a plugin command; the plugin does what it supports.
- `sf help sgd source delta` starts the executable with `sgd source delta --help`.

### Standard Streams And Exit Status

The plugin reads the stdin of the `sf` process and writes to its stdout and stderr. The exit status of the plugin is the exit status of `sf`. A plugin that follows sf's conventions exits with 0 for success, 2 for a missing or malformed flag and 1 for other failures, and with `--json` prints the sf envelope on stdout:

```json
{ "status": 0, "result": { }, "warnings": [] }
```

`sfplugin.WriteResult` and `sfplugin.WriteError` print the success and error envelopes.

An interrupt typed at the terminal reaches the plugin directly. A termination signal sent to the `sf` process is passed on to the plugin.

### Environment

The plugin inherits the environment of the `sf` process, with these variables set:

| Variable | Value |
|---|---|
| `SF_PLUGIN_PROTOCOL` | `1` |
| `SF_PLUGIN_BIN_NAME` | The command line that starts this mode, such as `sf` or `force --sf`. Use it in messages that suggest a command. |
| `SF_PLUGIN_NAME` | The plugin name in the recorded manifest. |
| `SF_PLUGIN_VERSION` | The plugin version in the recorded manifest. |

### Org Access

For a command with the org requirement `optional` or `required`, the mode resolves the target org before it starts the plugin, the same way a built-in command does:

1. The value of `--target-org` or `-o` (or the legacy `--targetusername` or `-u`) on the command line, written as `--target-org value` or `--target-org=value`.
2. Otherwise the `target-org` config value (`SF_TARGET_ORG`, the project config, the global config).

The API version is the value of `--api-version` on the command line, then the `org-api-version` config value, then the version recorded in the org's authorization file. The mode makes one authenticated request to the org, so that an expired access token is replaced with the stored refresh token before the plugin starts. It then sets:

| Variable | Value |
|---|---|
| `SF_PLUGIN_INSTANCE_URL` | The instance URL of the org, without a trailing slash. |
| `SF_PLUGIN_ACCESS_TOKEN` | A bearer access token. |
| `SF_PLUGIN_API_VERSION` | The API version, such as `67.0`. |
| `SF_PLUGIN_USERNAME` | The username of the org. |
| `SF_PLUGIN_ORG_ID` | The org ID. |

- `required`: when no org can be resolved, or the org refuses the request, the mode prints the error a built-in command prints and does not start the plugin.
- `optional`: when no org is named and none is configured, the plugin starts without the org variables. An org named on the command line must be usable.
- `none`: the org variables are never set, and values of these variables in the environment of the `sf` process are removed.

The refresh token, client ID and client secret are not passed. A plugin whose token expires while it runs reports the failure and exits. The flags stay on the command line, so a plugin that declares an org requirement accepts `--target-org`, `-o` and `--api-version` and can ignore their values. `sfplugin.OrgFromEnv` reads the variables.

### A Plugin In Go

```go
package main

import (
	"fmt"

	"github.com/ForceCLI/force/sfplugin"
)

var manifest = sfplugin.Manifest{
	Name:     "hello",
	Version:  "1.0.0",
	Protocol: sfplugin.ProtocolVersion,
	Commands: []sfplugin.Command{
		{ID: "hello org", Summary: "Print the username of the target org.", Org: sfplugin.OrgRequired},
	},
}

func main() {
	sfplugin.Main(manifest, func(args []string) int {
		org, _ := sfplugin.OrgFromEnv()
		fmt.Println("Hello,", org.Username)
		return 0
	})
}
```

```
go build -o hello .
sf plugins link ./hello
sf hello org --target-org my-org
```

## Publishing A Plugin

### Artifact Format

A tag points to an OCI image index with one image manifest for each platform.

| Part | Media type or value |
|---|---|
| Index | `application/vnd.oci.image.index.v1+json`, `artifactType` `application/vnd.forcecli.sf.plugin.v1`. Each entry has `platform.os` and `platform.architecture` with the names Go uses for `GOOS` and `GOARCH`. |
| Platform manifest | `application/vnd.oci.image.manifest.v1+json`, `artifactType` `application/vnd.forcecli.sf.plugin.v1`. |
| Config | The plugin manifest JSON, `application/vnd.forcecli.sf.plugin.config.v1+json`. |
| Layer | Exactly one: the executable, byte for byte, `application/vnd.forcecli.sf.plugin.executable.v1`, with the annotation `org.opencontainers.image.title` set to the file name (`<name>`, or `<name>.exe` for Windows). |
| Annotations | `org.opencontainers.image.version` (the tag), and `org.opencontainers.image.source` and `org.opencontainers.image.revision` when given. |

`sf plugins install` refuses a tag that points to anything else: another media type, another artifact type, a manifest with more or fewer than one layer, or a file name with a path separator.

### The publish Command

```
sf plugins publish ghcr.io/acme/my-plugin:v1.2.0 \
  --executable linux/amd64=dist/linux-amd64/my-plugin \
  --executable linux/arm64=dist/linux-arm64/my-plugin \
  --executable darwin/arm64=dist/darwin-arm64/my-plugin \
  --executable windows/amd64=dist/windows-amd64/my-plugin.exe \
  --source https://github.com/acme/my-plugin \
  --revision "$(git rev-parse HEAD)"
```

| Flag | Meaning |
|---|---|
| `--executable os/arch=path` | An executable to publish. Repeat for each platform. |
| `--manifest file` | The plugin manifest JSON. Without it, the executable for the platform that runs the command is asked for its manifest. |
| `--source url` | The source repository. GitHub Container Registry links the package to the repository named here, which lets that repository's workflows write to it. |
| `--revision rev` | The source control revision the executables were built from. |
| `--overwrite` | Moves the tag when it already exists. Without it, publishing to an existing tag fails. The digest the tag pointed to stays in the registry. |
| `--plain-http` | Uses HTTP, for a registry on a local machine. |

### GitHub Actions

```yaml
permissions:
  contents: read
  packages: write
steps:
  - uses: actions/checkout@v4
  - uses: actions/setup-go@v5
    with:
      go-version-file: go.mod
  - uses: docker/login-action@v3
    with:
      registry: ghcr.io
      username: ${{ github.actor }}
      password: ${{ github.token }}
  - run: |
      for platform in linux/amd64 linux/arm64 darwin/amd64 darwin/arm64 windows/amd64 windows/arm64; do
        os=${platform%/*} arch=${platform#*/}
        ext=""; [ "$os" = windows ] && ext=.exe
        GOOS=$os GOARCH=$arch CGO_ENABLED=0 go build -o "dist/$os-$arch/my-plugin$ext" .
        args="$args --executable $platform=dist/$os-$arch/my-plugin$ext"
      done
      go build -o my-plugin . && ./my-plugin sf-plugin-manifest > manifest.json
      force --sf plugins publish "ghcr.io/${GITHUB_REPOSITORY_OWNER,,}/my-plugin:${GITHUB_REF_NAME}" \
        --manifest manifest.json $args \
        --source "$GITHUB_SERVER_URL/$GITHUB_REPOSITORY" --revision "$GITHUB_SHA"
```

A package on GitHub Container Registry is private when first published. Make it public in the package settings so that `sf plugins install` works without credentials.

### Trying A Publish On A Local Machine

```
docker run -d -p 5000:5000 registry:2
sf plugins publish localhost:5000/test/my-plugin:dev --plain-http --executable "$(go env GOOS)/$(go env GOARCH)=./my-plugin"
sf plugins install localhost:5000/test/my-plugin:dev --plain-http
```

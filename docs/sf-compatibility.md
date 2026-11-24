# sf-Compatible Mode

When the `force` binary is invoked under the name `sf`, it accepts the command names and flags of the Salesforce `sf` CLI, reads and writes the state files of `sf`, and prints output in the format of `sf`. The reference is `sf` (`@salesforce/cli`) 2.138.6.

This document lists the `sf` commands the mode implements, the `force` command each one corresponds to, and the `sf` commands that are not implemented. `sf/CHECKLIST.md` records how each command was checked.

## Activation

The mode is selected by the base name of the program (`sf`, or `sf.exe` on Windows) or by the `--sf` flag of `force`: `force --sf org list` runs the same command as `sf org list`. `--sf` can be anywhere on the command line before a `--` argument. Without either, `force` runs as before.

Messages that suggest a command write the program the way it was started. Started as `sf` from the PATH, the mode prints `Run "sf org resume sandbox ..."`, as sf does. Started as `/tmp/sf`, it prints `Run "/tmp/sf org resume sandbox ..."`, and started as `force --sf`, it prints `Run "force --sf org resume sandbox ..."`, so that the suggested command runs this program and not another `sf` on the PATH. This also applies to the `[REDACTED] Use ...` text that replaces secrets in results. A path with spaces, quotes or a percent sign, and the temporary binary of `go run`, are replaced by `sf` or `force`. Help and usage text name `sf` in every case.

```
make install-sf        # install force and create an sf symlink next to it
ln -s "$(command -v force)" ~/bin/sf
```

`sf org:login:web` (colon form) and the legacy command ids that `sf` 2.138.6 accepts, such as `sf force:org:list` and `sf deploy metadata`, run the current command. The legacy flag names `--targetusername`, `-u`, `--targetdevhubusername` and `--apiversion` are accepted.

## State

sf mode does not read or write `~/.force`. It uses the files `sf` uses:

| File | Use |
|---|---|
| `~/.sfdx/<username>.json` | Org authorization. Token, password and secret fields are encrypted with the key `sf` uses: the key in the operating system keychain, or `~/.sfdx/key.json` when `SF_USE_GENERIC_UNIX_KEYCHAIN=true` or the keychain has no key. A key is created when none exists. |
| `~/.sfdx/alias.json` | Aliases. |
| `~/.sf/config.json` | Global config values. |
| `<project>/.sf/config.json` | Config values of the project that contains the working directory. |
| `~/.sfdx/sfdx-config.json`, `<project>/.sfdx/sfdx-config.json` | Read for `defaultusername`, `defaultdevhubusername`, `apiVersion` and `instanceUrl` when the `.sf` file does not set the value. Not written. |
| `~/.sf/bulk-data-*-cache.json` | Bulk jobs started without `--wait`, for the resume commands. |
| `~/.sf/plugins/` | Installed plugins: `plugins.json` and the downloaded executables. `SF_PLUGINS_DIR` names another directory. sf keeps its npm plugins in `~/.local/share/sf`, which this mode does not read. |
| `~/.sf/deploy-cache.json` | Deployments started by `project deploy start` and `validate`, for `--use-most-recent` and for finding the org of a job ID. |

A config value is taken from the environment (`SF_TARGET_ORG`, `SF_TARGET_DEV_HUB`, `SF_ORG_API_VERSION`, `SF_ORG_INSTANCE_URL`, `SF_ORG_MAX_QUERY_LIMIT`, `SF_DISABLE_TELEMETRY`, and the `SFDX_` names), then the project file, then the global file. `--target-org` and `--target-dev-hub` are used before any of them.

Logins, aliases and config values written by this mode were read by `sf` 2.138.6 in testing, and the reverse. A refreshed access token is saved to the authorization file.

## Plugins

sf plugins are npm packages, which this mode cannot load. A plugin of this mode is one native executable for each operating system and architecture, published as an OCI artifact on a container registry and run as a child process. `sf plugins install sfdx-git-delta` downloads the executable for this platform from `ghcr.io/forcecli/sf-plugins/sfdx-git-delta`, and `sf sgd source delta ...` then runs it with the same arguments. A built-in command is never replaced by a plugin command. [sf-plugins.md](sf-plugins.md) describes the commands, the protocol between this mode and a plugin, and how to write and publish one.

## Templates

The files written by the `template generate` commands come from the salesforcedx-templates repository (BSD-3-Clause). They are stored in `sf/templates` as one compressed archive for each template directory and are compiled into the binary. `sf/templates/SOURCE` names the tag and commit. `make update-sf-templates SF_TEMPLATES_VERSION=<tag>` rebuilds the archives, the copy of the license, the attribution in `licenses/salesforcedx-templates.txt` and `third-party-licenses.txt`, which every release zip includes.

## Output

- `--json` prints the `sf` envelope: `{"status", "result", "warnings"}`. Result fields keep the order of the API response, as in `sf`.
- An error prints `Error (<name>): <message>` on stderr, or with `--json` an envelope with `name`, `message`, `exitCode`, `context`, `code`, `status` and `commandName`. The envelope has no `stack` or `cause` field, and `data` holds only `errorCode` and `message`.
- A legacy command id, such as `force:org:list`, and a legacy flag name, such as `--targetusername`, `-u` or `--classname`, are accepted where `sf` accepts them, with the deprecation warning `sf` reports: on stderr, or in the `warnings` array with `--json`. The flag names are in `sf/flagaliases.go`, generated from `sf commands --json`.
- Exit status follows `sf`: 2 for a missing or malformed flag, 1 for a flag value outside its list of options, 127 for an unknown command, 100 for failing Apex tests, 1 for other failures.
- Color is used when `FORCE_COLOR` is set to a value other than `0`, when `COLORTERM=truecolor`, or when the output is a terminal. `NO_COLOR` and `FORCE_COLOR=0` turn it off. `--json` output is colored on a terminal and when `FORCE_COLOR` is set, and is not colored in a pipe when only `COLORTERM=truecolor` is set.
- `sf` redraws a progress display while it waits for a bulk job, deploy, retrieve or scratch org. This mode prints the last frame once.
- `--flags-dir` reads one file per flag; a file name can be the long or the short name of the flag.
- Authorization files are written with mode 0600; `sf` writes them with the default file mode.

## sf Commands Implemented

"Compared with sf" means the output of this mode and of `sf` 2.138.6 were compared for the same commands against the same server. "Follows the sf plugin source" means the layout and messages were taken from the plugin source because the command could not be run with `sf` against the test server.

| sf command | force equivalent | Unsupported flags and differences | Output check |
|---|---|---|---|
| `sf alias list` | none; reads `~/.sfdx/alias.json` |  | Compared with sf |
| `sf alias set` | none; writes `~/.sfdx/alias.json` |  | Compared with sf |
| `sf alias unset` | none; writes `~/.sfdx/alias.json` | Does not prompt before `--all`. | Compared with sf |
| `sf apex get compile-status` | none; one request to the Tooling API resource `apexCompileResults` | The table for a result with problems follows the plugin source; the org used for the comparison had none. | Compared with sf |
| `sf apex get log` | `force log <id>` |  | Compared with sf |
| `sf apex get test` | none (`force test` prints results when it runs) |  | Compared with sf |
| `sf apex list log` | `force log list` |  | Compared with sf |
| `sf apex run` | `force apex` |  | Compared with sf |
| `sf apex run test` | `force test` |  | Compared with sf |
| `sf apex tail log` | `force log tail` | `--debug-level`. `--color` is accepted and has no effect. | Follows the sf plugin source |
| `sf apex trace create` | `force trace start` | This command is in plugin-apex versions newer than the one bundled with sf 2.138.6. | Compared with sf |
| `sf apex trace delete` | `force trace delete` | As above. | Compared with sf |
| `sf apex trace list` | `force trace list` | As above. | Compared with sf |
| `sf api request graphql` | none | With `--include`, headers are listed in alphabetical order. | Compared with sf |
| `sf api request rest` | `force rest` | The `formdata` body mode of `--file`. With `--include`, headers are listed in alphabetical order; sf lists them in the order received. | Compared with sf |
| `sf autocomplete` | `force completion` | Prints instructions for the completion scripts cobra generates (`sf autocomplete script <shell>`); sf builds a cache from its plugins. `--refresh-cache` has no effect. | Not compared; the mechanism differs |
| `sf cmdt generate field` | none |  | Compared with sf |
| `sf cmdt generate fromorg` | none; reads the object with the Metadata API and queries its records |  | Compared with sf |
| `sf cmdt generate object` | none |  | Compared with sf |
| `sf cmdt generate record` | none |  | Compared with sf |
| `sf cmdt generate records` | none |  | Compared with sf |
| `sf commands` | none | Lists only the commands this mode implements. sf prints more columns. | Not compared |
| `sf community create` | none | Follows the source of the sf community plugin, version 4.0.27, which is not part of the sf installation used for the comparisons; not compared with sf. `--description` is accepted and not sent, as in sf. | Follows the sf plugin source |
| `sf community list template` | none | Follows the source of the sf community plugin, version 4.0.27, which is not part of the sf installation used for the comparisons; not compared with sf. Run against a scratch org with this mode. | Follows the sf plugin source |
| `sf community publish` | none | Follows the source of the sf community plugin, version 4.0.27, which is not part of the sf installation used for the comparisons; not compared with sf. | Follows the sf plugin source |
| `sf config get` | none; reads sf config files | Does not suggest a name for an unknown config name. | Compared with sf |
| `sf config list` | `force active` (for the default org) |  | Compared with sf |
| `sf config set` | `force active <account>` (for `target-org`) | An unknown config name is reported as a failure; sf asks whether a similar name was meant. | Compared with sf |
| `sf config unset` | none |  | Compared with sf |
| `sf data bulk results` | `force bulk2 results` |  | Compared with sf |
| `sf data create file` | none | Uploads the file as base64 in a JSON request; sf sends a multipart request. | Compared with sf |
| `sf data create record` | `force record create` |  | Compared with sf |
| `sf data delete bulk` | `force bulk2 delete`, `force bulk2 hardDelete` | Prints the last frame of the progress display once. | Compared with sf |
| `sf data delete record` | `force record delete` |  | Compared with sf |
| `sf data delete resume` | `force bulk2 job` |  | Compared with sf |
| `sf data export bulk` | `force bulk2 query` | Prints the last frame of the progress display once. | Compared with sf |
| `sf data export resume` | `force bulk2 job` |  | Compared with sf |
| `sf data export tree` | none | With several `--query` values each file is named after the object type of its query; sf names it after the query text. | Compared with sf |
| `sf data get record` | `force record get` |  | Compared with sf |
| `sf data import bulk` | `force bulk2 insert` | Prints the last frame of the progress display once. | Compared with sf |
| `sf data import resume` | `force bulk2 job` |  | Compared with sf |
| `sf data import tree` | none |  | Compared with sf |
| `sf data query` | `force query` |  | Compared with sf |
| `sf data resume` | `force bulk job`, `force bulk batch` |  | Compared with sf |
| `sf data search` | `force search` |  | Compared with sf |
| `sf data update bulk` | `force bulk2 update` | Prints the last frame of the progress display once. | Compared with sf |
| `sf data update record` | `force record update` |  | Compared with sf |
| `sf data update resume` | `force bulk2 job` |  | Compared with sf |
| `sf data upsert bulk` | `force bulk2 upsert` | Prints the last frame of the progress display once. | Compared with sf |
| `sf data upsert resume` | `force bulk2 job` |  | Compared with sf |
| `sf force data bulk delete` | `force bulk delete` | Sends the file as one batch; sf splits it into batches of 10,000 records. | Compared with sf |
| `sf force data bulk status` | `force bulk job`, `force bulk batch` |  | Compared with sf |
| `sf force data bulk upsert` | `force bulk upsert` | Sends the file as one batch. | Compared with sf |
| `sf flow get test` | none | Follows the source of the sf flow plugin, version 2.0.2, which is not part of the sf installation used for the comparisons; not compared with sf. | Follows the sf plugin source |
| `sf flow run test` | none | Follows the source of the sf flow plugin, version 2.0.2, which is not part of the sf installation used for the comparisons; not compared with sf. The command has no `--wait` flag, as in sf: without `--synchronous` it prints the Id of the test run. | Follows the sf plugin source |
| `sf logic get test` | none | The tests of a run are listed in the order of their names; sf lists them in the order the org returns them. A Flow test named with three parts is taken as `FlowTesting.<flow>.<test>`; sf asks the org for its namespaces to tell it from `FlowTesting.<namespace>.<flow>`. Compared with sf for Apex tests; the scratch org used had no Flow tests. | Compared with sf |
| `sf logic run test` | none | The tests of a run are listed in the order of their names; sf lists them in the order the org returns them. A Flow test named with three parts is taken as `FlowTesting.<flow>.<test>`; sf asks the org for its namespaces to tell it from `FlowTesting.<namespace>.<flow>`. Compared with sf for Apex tests; the scratch org used had no Flow tests. | Compared with sf |
| `sf org assign permset` | none |  | Compared with sf |
| `sf org assign permsetlicense` | none |  | Compared with sf |
| `sf org auth show-access-token` | none |  | Compared with sf |
| `sf org auth show-sfdx-auth-url` | none |  | Error output compared with sf; the URL format follows the sf source |
| `sf org auth show-user-password` | none | No command of this mode stores a password, so it finds one only in an authorization file that sf wrote. | Compared with sf |
| `sf org create sandbox` | none | The output of `--async` was compared with sf. Waiting for the sandbox and logging in to it were not. The progress display is printed once, when the command ends, and the confirmation prompt is a plain question. | Compared with sf |
| `sf org create scratch` | `force login scratch` | Definition file key `objectSettings`. Without `--client-id` the request names the connected app `PlatformCLI` also when the Dev Hub was authorized with another one. `--client-id` follows the sf core source and was run against a Dev Hub with the connected app of force; when the Dev Hub refuses the callback URL of sf, the request is sent again with the callback URL of force. The secret of the connected app is not sent when the access token is refreshed. `--snapshot`, `--source-org` and the package ancestors follow the sf core source. The `--async` output follows a transcript of `sf`; this mode was not run against a Dev Hub. | Follows the sf plugin source |
| `sf org refresh sandbox` | none | Not compared with sf: it needs a production org with sandbox licenses. The progress display is printed once, when the command ends. | Follows the sf plugin source |
| `sf org resume sandbox` | none | The output for a sandbox that is not ready was compared with sf. Logging in to a finished sandbox was run against a production org with this mode; sf could not do it for that org, for the reason that follows. For a production org that was authorized with `org login web` of this mode, which uses the connected app of force, the authorization request names the callback URL of that app; sf names its own callback URL for every org, and the org refuses its request. The progress display is printed once, when the command ends. | Compared with sf |
| `sf org resume scratch` | none; reads `~/.sf/scratch-create-cache.json` and finishes the request as `force login scratch` does | The output follows a transcript of `sf`; this mode was not run against a Dev Hub. The case of an org that is not ready follows the plugin source. | Follows the sf plugin source |
| `sf org create shape` | none |  | Follows the sf plugin source |
| `sf org create snapshot` | none | Not compared with sf: the Dev Hub used for the comparison did not have snapshots enabled. | Follows the sf plugin source |
| `sf org create user` | none; creates the user with the SCIM API and writes its authorization file | An org that was authorized with the JWT flow. | Compared with sf |
| `sf org delete shape` | none |  | Follows the sf plugin source |
| `sf org delete snapshot` | none | The Dev Hub used for the comparison did not have snapshots enabled: the error for that case was compared, and the tables follow the plugin source. Dates are formatted for the en-US locale. | Follows the sf plugin source |
| `sf org delete sandbox` | none | Not compared with sf: it needs a production org with a sandbox. When the sandbox file has no SandboxInfo Id, sf 2.138.6 finds the Id by sandbox name and then does not use it, so it deletes nothing in the production org. This mode deletes the SandboxInfo it finds. | Follows the sf plugin source |
| `sf org delete scratch` | none | For an org that has expired, `sf` removes the local authorization and reports it as marked for deletion; this mode asks the Dev Hub and reports it as deleted. | Compared with sf |
| `sf org display` | `force whoami`, `force active` | An HTML response from the org, such as the one an expired scratch org gives, is reported in other words than in `sf`. | Compared with sf |
| `sf org display user` | `force whoami` |  | Compared with sf |
| `sf org generate password` | `force password reset` (sets a password the org generates) | The password is stored only for users that have an authorization file. | Compared with sf |
| `sf org list` | `force logins --sfdx` | `--verbose`, `--clean` and the `Expires` column of scratch orgs follow the plugin source and were not compared with sf. An authorization file that `sf` cannot decrypt (`AuthDecryptError`) may be decrypted, because every key found is tried. No warning for an http instance URL. | Compared with sf |
| `sf org list auth` | `force logins` |  | Compared with sf |
| `sf org list limits` | `force limits` |  | Compared with sf |
| `sf org list metadata` | `force describe metadata --type` |  | Compared with sf |
| `sf org list metadata-types` | `force describe metadata` |  | Compared with sf |
| `sf org list users` | none; reads `~/.sfdx/<orgId>.json` and queries User and Profile | `SF_TEMP_SHOW_SECRETS` is not read; the access token is always redacted. | Compared with sf |
| `sf org get snapshot` | none | The Dev Hub used for the comparison did not have snapshots enabled: the error for that case was compared, and the tables follow the plugin source. Dates are formatted for the en-US locale. | Follows the sf plugin source |
| `sf org list shape` | none | Only the output without org shapes was compared with sf. The warning for an org that cannot be queried has the error message of this mode, which can differ from the one sf prints. | Follows the sf plugin source |
| `sf org list snapshot` | none | The Dev Hub used for the comparison did not have snapshots enabled: the error for that case was compared, and the tables follow the plugin source. Dates are formatted for the en-US locale. | Follows the sf plugin source |
| `sf org list sobject record-counts` | none |  | Compared with sf |
| `sf org login access-token` | none (`SF_ACCESS_TOKEN` and `SF_INSTANCE_URL` in force) |  | Compared with sf |
| `sf org login jwt` | `force login -u <user> -k <key> --connected-app-client-id <id>` |  | Not compared; no JWT-enabled org was available |
| `sf org login sfdx-url` | none |  | Follows the sf plugin source |
| `sf org disable tracking` | none; writes `tracksSource` to the authorization file |  | Compared with sf |
| `sf org enable tracking` | none; writes `tracksSource` to the authorization file |  | Compared with sf |
| `sf org login web` | `force login` | `--client-app`, `--username`. `--browser` and `--scopes` follow the sf plugin source. Uses the force connected app and callback port unless `--client-id` is given. | Run against an org; the output was not compared with sf |
| `sf org logout` | `force logout` | `--client-app` is accepted and has no effect. | Compared with sf |
| `sf org open` | `force open` | `--private` opens a private window of the default browser when that is Chrome, Brave, Firefox or Edge, and was not compared with sf. `--browser` starts the browser by its program name on Linux and by its application name on macOS and Windows. `--source-file` follows the sf core source and recognizes the metadata type by the file name. | Compared with sf |
| `sf package convert` | none | `--seed-metadata`, and the `seedMetadata` and `unpackagedMetadata` of the package directory. sf 2.138.6 fails with "Cannot read properties of undefined" after it sent the request when `--wait` is not given; this mode reports the request. Follows the source of the sf packaging library. Run against a Dev Hub up to the creation of the second-generation package, which the Dev Hub refused for sf and for this mode because a package with that name existed in the namespace; the conversion request was not sent. | Follows the sf plugin source |
| `sf package create` | `force package create` |  | Follows the sf plugin source |
| `sf package delete` | none |  | Follows the sf plugin source |
| `sf package install` | `force package install` | Does not prompt about remote site settings; they are enabled. | Follows the sf plugin source |
| `sf package install report` | none |  | Follows the sf plugin source |
| `sf package installed list` | `force package installed` |  | Follows the sf plugin source |
| `sf package list` | `force package list` |  | Compared with sf |
| `sf package push-upgrade abort` | none |  | Follows the sf plugin source |
| `sf package push-upgrade list` | none | Only the output for a package without push upgrade requests was compared with sf. | Follows the sf plugin source |
| `sf package push-upgrade report` | none |  | Follows the sf plugin source |
| `sf package push-upgrade schedule` | none | Not compared with sf: it upgrades a package in subscriber orgs. | Follows the sf plugin source |
| `sf package uninstall` | `force package uninstall` |  | Follows the sf plugin source |
| `sf package uninstall report` | none |  | Follows the sf plugin source |
| `sf package update` | none |  | Follows the sf plugin source |
| `sf package version create` | `force package version create` | `objectSettings` in a definition file, and the `unpackagedMetadata` and `seedMetadata` keys of the package directory. Profiles are not added to the package. Dependencies, the ancestor, `--definition-file`, `--skip-ancestor-check`, `--verbose` and the other keys of the package directory follow the source of the sf packaging library and were not compared with sf. `--uninstall-script` is accepted and not sent, as in sf 2.138.6, which sends an uninstall script only from the package directory. | Compared with sf |
| `sf package version create list` | none | `--show-conversions-only` and `--verbose` follow the plugin source and were not compared with sf. | Compared with sf |
| `sf package version create report` | none | Only the error for a request that does not exist was compared with sf. | Follows the sf plugin source |
| `sf package version delete` | none |  | Follows the sf plugin source |
| `sf package version displayancestry` | none | Only the errors for a package and a package version that do not exist were compared with sf. | Follows the sf plugin source |
| `sf package version displaydependencies` | none | Not compared with sf. | Follows the sf plugin source |
| `sf package version list` | `force package version list` | The Dev Hub used for the comparison had no version with an ancestor, so the ancestor version number follows the plugin source, as does `--show-conversions-only`. | Compared with sf |
| `sf package version promote` | `force package version release` |  | Follows the sf plugin source |
| `sf package version report` | `force package version get` | The code coverage rows of `--verbose` follow the sf plugin source; the version compared with sf had no code coverage or dependencies. | Compared with sf |
| `sf package version retrieve` | none | Not compared with sf: it needs a package version whose developer package zip can be downloaded. The rows are listed in the order this mode writes the files. | Follows the sf plugin source |
| `sf package version update` | none |  | Follows the sf plugin source |
| `sf package1 version create` | none | The messages for a queued request, a request in progress, a failed upload and a finished upload were compared with sf against a packaging org. `--wait` was compared too: sf prints nothing while it waits. | Follows the sf plugin source |
| `sf package1 version create get` | none | The messages for a queued request, a request in progress, a failed upload and a finished upload were compared with sf against a packaging org. | Follows the sf plugin source |
| `sf package1 version display` | none; queries MetadataPackageVersion with the Tooling API |  | Compared with sf |
| `sf package1 version list` | none; queries MetadataPackageVersion with the Tooling API |  | Compared with sf |
| `sf plugins` | none | Lists the plugins of this mode, which are native executables and not npm packages; see [sf-plugins.md](sf-plugins.md). `--core` is not supported. The output differs from sf. | Not compared; the mechanism differs |
| `sf plugins inspect` | none | Prints the recorded registry reference, digests, executable path and commands. `--verbose` is not supported. | Not compared; the mechanism differs |
| `sf plugins install` | none | Installs from a container registry instead of npm. Takes `name[:tag]`; a name without a registry host is looked up under `ghcr.io/forcecli/sf-plugins`. `--force`, `--jit`, `--silent` and `--verbose` are not supported. `--plain-http` is an addition. Also run as `sf plugins add`. | Not compared; the mechanism differs |
| `sf plugins link` | none | Takes the path of a plugin executable instead of a package directory. `--install` and `--verbose` are not supported. | Not compared; the mechanism differs |
| `sf plugins publish` | none | Not an sf command. Publishes plugin executables to a container registry. | Not compared; sf has no such command |
| `sf plugins uninstall` | none | Also run as `sf plugins remove` and `sf plugins unlink`. | Not compared; the mechanism differs |
| `sf plugins update` | none | Looks up the tag each plugin was installed with and downloads the plugin when the digest changed. Takes plugin names. | Not compared; the mechanism differs |
| `sf project delete source` | `force import` of a directory that holds only a destructive changes manifest; this mode also deletes the local files | Deleting one file of a bundle, which `sf` turns into a deploy of the remaining files. The test results of a delete that runs tests are not printed. Without `--no-prompt` and without a terminal the command is canceled; `sf` fails with status 13. When the org refuses a delete given with `--track-source`, `sf` still updates the tracking files; this mode leaves them as they are. | Compared with sf |
| `sf project delete tracking` | none; deletes the tracking files in `<project>/.sf/orgs/<orgId>/` | Without `--no-prompt` and without a terminal the command does nothing; `sf` fails with status 13. | Compared with sf |
| `sf project reset tracking` | none; writes the tracking files in `<project>/.sf/orgs/<orgId>/` | Without `--no-prompt` and without a terminal the command does nothing; `sf` fails with status 13. | Compared with sf |
| `sf project convert mdapi` | `force fetch` (source format writer) | `--metadata CustomLabel:<name>` gives "No results to format"; sf reports a `NoChildTypesError` for it. | Compared with sf |
| `sf project convert source` | `force import` (source format conversion) | A composed custom object has its elements in alphabetical order; `sf` writes the child components last. | Compared with sf |
| `sf project deploy cancel` | `force deploys cancel` | The output of a cancel that succeeds follows the plugin source; in the comparison the deploy of sf had always finished before the cancel. | Compared with sf |
| `sf project deploy quick` | `force quickdeploy` |  | Compared with sf |
| `sf project deploy report` | `force deploys status` | The `html` and `html-spa` values of `--coverage-formatters`. | Compared with sf |
| `sf project deploy resume` | `force deploys watch` | The `html` and `html-spa` values of `--coverage-formatters`. | Compared with sf |
| `sf project deploy preview` | none; reads the source tracking files |  | Compared with sf |
| `sf project deploy start` | `force push`, `force import` | The `html` and `html-spa` values of `--coverage-formatters`. Prints the last frame of the progress display once. Without `--source-dir`, `--metadata`, `--manifest` or `--metadata-dir`, deploys the local changes that source tracking reports; the org must track source. | Compared with sf |
| `sf project deploy validate` | `force import --checkonly` | As `project deploy start`. | As `project deploy start` |
| `sf project generate manifest` | none | `--from-org` asks the org which metadata types it has and lists those; sf asks about each of the types in its registry. The result was the same for the scratch org used in the comparison. A name pattern in `--metadata` with `--from-org` supports `*` and `?`. sf reports an error when a content file is ignored by `.forceignore` and its `-meta.xml` file is not; this mode leaves both out. | Compared with sf |
| `sf project list ignored` | none |  | Compared with sf |
| `sf project retrieve preview` | none; reads the source tracking files | A component of the org that has no local files is never reported as ignored. | Compared with sf |
| `sf project retrieve start` | `force fetch`, `force export` | More than one `--package-name` value. `--single-package` follows the sf library source and was not compared with sf. Without `--metadata`, `--manifest`, `--source-dir` or `--package-name`, retrieves the changes in the org that source tracking reports; the org must track source. A component that the project already has is written where it is; a new one is written to the default package directory. | Compared with sf |
| `sf schema generate tab` | none |  | Compared with sf |
| `sf sobject describe` | `force describe sobject --name <name> --json` |  | Compared with sf |
| `sf sobject list` | `force sobject list` |  | Compared with sf |
| `sf template generate analytics template` | none |  | Compared with sf |
| `sf template generate apex class` | `force create apexclass` |  | Compared with sf |
| `sf template generate apex trigger` | `force create apextrigger` |  | Compared with sf |
| `sf template generate flexipage` | none |  | Compared with sf |
| `sf template generate lightning app` | none |  | Compared with sf |
| `sf template generate lightning component` | none |  | Compared with sf |
| `sf template generate lightning event` | none |  | Compared with sf |
| `sf template generate lightning interface` | none |  | Compared with sf |
| `sf template generate lightning test` | none |  | Compared with sf |
| `sf template generate project` | none | The `agent`, `nativemobile`, `reactinternalapp` and `reactexternalapp` project templates. | Compared with sf |
| `sf template generate static-resource` | none |  | Compared with sf |
| `sf template generate visualforce component` | `force create apexcomponent` |  | Compared with sf |
| `sf template generate visualforce page` | `force create apexpage` |  | Compared with sf |
| `sf which` | none | For a built-in command, names the sf plugin that provides the command in sf 2.138.6. For a command of an installed plugin, names that plugin. | Compared with sf |
| `sf version` | `force version` | Prints the sf release this mode matches and the force version in place of the Node.js version. | Compared with sf |

A flag listed as unsupported returns an error that names the flag.

## force Commands And Their sf Equivalents

| force command | sf command | Notes |
|---|---|---|
| `force active` | `sf config get target-org`, `sf config set target-org=<org>` |  |
| `force apex` | `sf apex run` |  |
| `force apiversion` | `sf config set org-api-version=<version>`, or `--api-version` on a command | No sf command lists API versions. |
| `force bigobject` | none |  |
| `force bulk insert, update` | none | sf has Bulk API 1.0 commands only for delete and upsert. |
| `force bulk delete, upsert` | `sf force data bulk delete`, `sf force data bulk upsert` |  |
| `force bulk job, batch, batches` | `sf force data bulk status`, `sf data resume` |  |
| `force bulk query, retrieve, result, request, watch, hardDelete` | none | Use the Bulk API 2.0 commands. |
| `force bulk2 insert, update, upsert, delete, hardDelete` | `sf data import bulk`, `sf data update bulk`, `sf data upsert bulk`, `sf data delete bulk` (`--hard-delete`) |  |
| `force bulk2 query` | `sf data export bulk` |  |
| `force bulk2 job, results` | `sf data import resume` (and the other resume commands), `sf data bulk results` |  |
| `force bulk2 jobs, abort, delete-job` | none |  |
| `force create apexclass, apextrigger, apexpage, apexcomponent` | `sf template generate apex class`, `sf template generate apex trigger`, `sf template generate visualforce page`, `sf template generate visualforce component` |  |
| `force datapipe` | none |  |
| `force deploys list` | none |  |
| `force deploys cancel, status, watch` | `sf project deploy cancel`, `sf project deploy report`, `sf project deploy resume` |  |
| `force deploys errors` | `sf project deploy report` | The report lists component failures. |
| `force describe metadata` | `sf org list metadata-types`, `sf org list metadata -m <type>` |  |
| `force describe sobject` | `sf sobject describe`, `sf sobject list` |  |
| `force eventlogfile` | none |  |
| `force export` | `sf project retrieve start --manifest <package.xml> --target-metadata-dir <dir> --unzip` | sf has no command that retrieves all metadata without a manifest. |
| `force fetch` | `sf project retrieve start` |  |
| `force field` | none | `sf schema generate field` creates a local file and is not implemented. |
| `force import` | `sf project deploy start --metadata-dir <dir>` |  |
| `force limits` | `sf org list limits` |  |
| `force log` | `sf apex list log`, `sf apex get log`, `sf apex tail log` |  |
| `force log delete, debug` | none |  |
| `force login` | `sf org login web`, `sf org login jwt`, `sf org login access-token` | Device flow, client credentials and username-password logins have no sf command. |
| `force login scratch` | `sf org create scratch` |  |
| `force logins` | `sf org list`, `sf org list auth` |  |
| `force logout` | `sf org logout` |  |
| `force notify` | none |  |
| `force oauth` | none |  |
| `force open` | `sf org open` |  |
| `force package create, list, install, installed, uninstall` | `sf package create`, `sf package list`, `sf package install`, `sf package installed list`, `sf package uninstall` |  |
| `force package version create, list, get, release` | `sf package version create`, `sf package version list`, `sf package version report`, `sf package version promote` |  |
| `force password` | none | `sf org generate password` is not implemented. |
| `force pubsub` | none |  |
| `force push` | `sf project deploy start` |  |
| `force query` | `sf data query` |  |
| `force quickdeploy` | `sf project deploy quick` |  |
| `force record get, create, update, delete` | `sf data get record`, `sf data create record`, `sf data update record`, `sf data delete record` |  |
| `force record upsert, merge, undelete` | none |  |
| `force rest` | `sf api request rest` |  |
| `force search` | `sf data search` |  |
| `force security` | none |  |
| `force sobject list` | `sf sobject list` |  |
| `force sobject create, delete, import` | none |  |
| `force test` | `sf apex run test` |  |
| `force trace` | `sf apex trace create`, `sf apex trace list`, `sf apex trace delete` |  |
| `force usedxauth` | not needed | sf mode reads sf authorization files directly. |
| `force version` | `sf version` |  |
| `force whoami` | `sf org display`, `sf org display user` |  |

## sf Commands Not Implemented

72 of the 239 commands of `sf` 2.138.6 and its bundled and install-on-first-use plugins are not implemented. Third-party plugins and the retired Functions plugin are not counted. Running one of these commands prints the message `sf` prints for an unknown command and exits with status 127.

### org

| sf command | Summary | What is required |
|---|---|---|
| `sf org create agent-user` | Create the default Salesforce user that is used to run an agent. | Create an agent User record and assign permission sets. |
| `sf org open agent` | Open an agent in your org's Agentforce Builder UI in a browser. | Build the Agentforce Builder URL for an agent and open it as `org open` does. |

### project

| sf command | Summary | What is required |
|---|---|---|
| `sf project convert source-behavior` | Enable a behavior of your project source files, and then update your Salesforce DX project to implement the behavior. | Rewrite source files between source behaviors; no force code exists. |
| `sf project deploy pipeline quick` | Quickly deploy a validated deployment to an org. | DevOps Center API; no force code exists. |
| `sf project deploy pipeline report` | Check the status of a pipeline deploy operation. | DevOps Center API; no force code exists. |
| `sf project deploy pipeline resume` | Resume watching a pipeline deploy operation. | DevOps Center API; no force code exists. |
| `sf project deploy pipeline start` | Deploy changes from a branch to the pipeline stage’s org. | DevOps Center API; no force code exists. |
| `sf project deploy pipeline validate` | Perform a validate-only deployment from a branch to the pipeline stage’s org. | DevOps Center API; no force code exists. |

### schema

| sf command | Summary | What is required |
|---|---|---|
| `sf schema generate field` | Generate metadata source files for a new custom field on a specified object. | Ask for the properties of the object, field or platform event with prompts, then write the metadata file. `sf` has no flags for these properties, so the command cannot run without a terminal. |
| `sf schema generate platformevent` | Generate metadata source files for a new platform event. | Ask for the properties of the object, field or platform event with prompts, then write the metadata file. `sf` has no flags for these properties, so the command cannot run without a terminal. |
| `sf schema generate sobject` | Generate metadata source files for a new custom object. | Ask for the properties of the object, field or platform event with prompts, then write the metadata file. `sf` has no flags for these properties, so the command cannot run without a terminal. |

### template

| sf command | Summary | What is required |
|---|---|---|
| `sf template generate digital-experience site` | Generate an Experience Cloud site. | Render the files of the matching archive in `sf/templates`. The flexipage templates use EJS loops, which the renderer does not handle; the digital experience site command also creates the site in an org. |
| `sf template generate ui-bundle` | Generate a UI bundle, which contains the code and metadata to build a UI experience that uses non-native Salesforce frameworks, such as React. | Render the files of the matching archive in `sf/templates`. The flexipage templates use EJS loops, which the renderer does not handle; the digital experience site command also creates the site in an org. |

### lightning

| sf command | Summary | What is required |
|---|---|---|
| `sf lightning dev app` | Preview a Lightning Experience app locally and in real-time, without deploying it. | Local development server; no force code exists. |
| `sf lightning dev component` | Preview LWC components in isolation. | Local development server; no force code exists. |
| `sf lightning dev site` | Preview an Experience Builder site locally and in real-time, without deploying it. | Local development server; no force code exists. |

### agent

| sf command | Summary | What is required |
|---|---|---|
| `sf agent activate` | Activate an agent in an org. | Agentforce APIs; no force code exists. |
| `sf agent adl create` | Create an Agentforce Data Library. | Agentforce APIs; no force code exists. |
| `sf agent adl delete` | Delete an Agentforce Data Library. | Agentforce APIs; no force code exists. |
| `sf agent adl file add` | Add files to an existing Agentforce Data Library. | Agentforce APIs; no force code exists. |
| `sf agent adl file delete` | Delete a file from an Agentforce Data Library. | Agentforce APIs; no force code exists. |
| `sf agent adl file list` | List files in an Agentforce Data Library. | Agentforce APIs; no force code exists. |
| `sf agent adl get` | Get details of an Agentforce Data Library. | Agentforce APIs; no force code exists. |
| `sf agent adl list` | List Agentforce Data Libraries in an org. | Agentforce APIs; no force code exists. |
| `sf agent adl status` | Get indexing status of an Agentforce Data Library. | Agentforce APIs; no force code exists. |
| `sf agent adl update` | Update an Agentforce Data Library. | Agentforce APIs; no force code exists. |
| `sf agent adl upload` | Upload a file to an SFDRIVE Agentforce Data Library. | Agentforce APIs; no force code exists. |
| `sf agent create` | Create an agent in your org using a local agent spec file. | Agentforce APIs; no force code exists. |
| `sf agent deactivate` | Deactivate an agent in an org. | Agentforce APIs; no force code exists. |
| `sf agent generate agent-spec` | Generate an agent spec, which is a YAML file that captures what an agent can do. | Agentforce APIs; no force code exists. |
| `sf agent generate authoring-bundle` | Generate an authoring bundle from an existing agent spec YAML file. | Agentforce APIs; no force code exists. |
| `sf agent generate template` | Generate an agent template from an existing agent in your DX project so you can then package the template in a second-generation managed package. | Agentforce APIs; no force code exists. |
| `sf agent generate test-spec` | Generate an agent test spec, which is a YAML file that lists the test cases for testing a specific agent. | Agentforce APIs; no force code exists. |
| `sf agent preview` | Interact with an agent to preview how it responds to your statements, questions, and commands (utterances). | Agentforce APIs; no force code exists. |
| `sf agent preview end` | End an existing programmatic agent preview session and get trace location. | Agentforce APIs; no force code exists. |
| `sf agent preview send` | Send a message to an existing agent preview session. | Agentforce APIs; no force code exists. |
| `sf agent preview sessions` | List all known programmatic agent preview sessions. | Agentforce APIs; no force code exists. |
| `sf agent preview start` | Start a programmatic agent preview session. | Agentforce APIs; no force code exists. |
| `sf agent publish authoring-bundle` | Publish an authoring bundle to your org, which results in a new agent or a new version of an existing agent. | Agentforce APIs; no force code exists. |
| `sf agent test create` | Create an agent test in your org using a local test spec YAML file. | Agentforce APIs; no force code exists. |
| `sf agent test list` | List the available agent tests in your org. | Agentforce APIs; no force code exists. |
| `sf agent test results` | Get the results of a completed agent test run. | Agentforce APIs; no force code exists. |
| `sf agent test resume` | Resume an agent test that you previously started in your org so you can view the test results. | Agentforce APIs; no force code exists. |
| `sf agent test run` | Start an agent test in your org. | Agentforce APIs; no force code exists. |
| `sf agent test run-eval` | Run rich evaluation tests against an Agentforce agent. | Agentforce APIs; no force code exists. |
| `sf agent trace delete` | Delete trace files from an agent preview session. | Agentforce APIs; no force code exists. |
| `sf agent trace list` | List the available trace files that were recorded during all agent preview sessions. | Agentforce APIs; no force code exists. |
| `sf agent trace read` | Read trace files from an agent preview session. | Agentforce APIs; no force code exists. |
| `sf agent validate authoring-bundle` | Validate an authoring bundle to ensure its Agent Script file compiles successfully and can be used to publish an agent. | Agentforce APIs; no force code exists. |

### force

| sf command | Summary | What is required |
|---|---|---|
| `sf force lightning lwc test create` |  | Runs Jest in a Node.js project; it does not apply to force. |
| `sf force lightning lwc test run` |  | Runs Jest in a Node.js project; it does not apply to force. |
| `sf force lightning lwc test setup` |  | Runs Jest in a Node.js project; it does not apply to force. |
| `sf force package push-upgrade list` | Lists the status of push upgrade requests for a given package. | Legacy name of `package push-upgrade list`. |

### ui-bundle

| sf command | Summary | What is required |
|---|---|---|
| `sf ui-bundle dev` | Preview a UI bundle locally and in real-time, without deploying it to your org. | UI bundle upload; no force code exists. |

### dev

| sf command | Summary | What is required |
|---|---|---|
| `sf dev audit messages` | Audit messages in a plugin's messages directory to locate unused messages and missing messages that have references in source code. | Tools for writing sf plugins; they do not apply to force. |
| `sf dev convert messages` | Convert a .json messages file into Markdown. | Tools for writing sf plugins; they do not apply to force. |
| `sf dev convert script` | Convert a script file that contains deprecated sfdx-style commands to use the new sf-style commands instead. | Tools for writing sf plugins; they do not apply to force. |
| `sf dev generate command` | Generate a new sf command. | Tools for writing sf plugins; they do not apply to force. |
| `sf dev generate flag` | Generate a flag for an existing command. | Tools for writing sf plugins; they do not apply to force. |
| `sf dev generate plugin` | Generate a new sf plugin. | Tools for writing sf plugins; they do not apply to force. |

### CLI management

| sf command | Summary | What is required |
|---|---|---|
| `sf doctor` | Gather CLI configuration data and run diagnostic tests to discover and report potential problems in your environment. | Diagnostics of the Node.js CLI installation; they do not apply to force. |
| `sf help` |  | cobra provides `sf help <command>`; its text differs from sf help. |
| `sf info releasenotes display` | Display Salesforce CLI release notes on the command line. | Prints the release notes of the Node.js CLI. |
| `sf plugins discover` | See a list of 3rd-party sf plugins you can install. | The list sf prints is of Node.js packages, which this mode cannot load. |
| `sf plugins reset` | Remove all user-installed and linked plugins. | Not written; `sf plugins uninstall` removes one plugin. |
| `sf plugins trust allowlist add` | Add plugins to the plugin allowlist. | The allowlist is for unsigned npm packages. Plugins of this mode are checked against the digest in their registry manifest. |
| `sf plugins trust allowlist list` | List the plugins on the plugin allowlist. | The allowlist is for unsigned npm packages. Plugins of this mode are checked against the digest in their registry manifest. |
| `sf plugins trust allowlist remove` | Remove plugins from the plugin allowlist. | The allowlist is for unsigned npm packages. Plugins of this mode are checked against the digest in their registry manifest. |
| `sf plugins trust verify` | Validate a digital signature. | Verifies the signature of an npm package. Plugins of this mode are not signed. |
| `sf search` | Search for a command. | Interactive search of commands. |
| `sf update` |  | Updates the Node.js CLI; it does not apply to force. |
| `sf whatsnew` | Display Salesforce CLI release notes on the command line. | Alias of `info releasenotes display`. |

### Features Shared By Several Commands

| Feature | What is required |
|---|---|
| Source tracking | Implemented for an org that tracks source: `project deploy start` and `project retrieve start` without named components deploy the local changes and retrieve the changes in the org, stop on conflicts unless `--ignore-conflicts` is given, and update the tracking files in `<project>/.sf/orgs/<orgId>/`, which are in the formats of `sf`. SOAP requests carry the client name `sf` sends (`sfdx toolbelt:`), because the org records the source members of a deploy only for that client. After a deploy the org is asked for the source members of the changed components for the time `sf` waits; `SF_SOURCE_MEMBER_POLLING_TIMEOUT` and `SF_DISABLE_SOURCE_MEMBER_POLLING` are read. Not implemented: detection of moved files. |
| `html` and `html-spa` coverage reports on the deploy commands | Write the HTML pages istanbul writes; the other coverage formats and the JUnit file are implemented. |
| Interactive prompts | `sf` prompts for a similar config name, for a command when a topic is given without one, and before some destructive actions. This mode prompts before `org logout`, `org delete scratch`, `package version promote`, `project delete source`, `project delete tracking` and `project reset tracking`. |

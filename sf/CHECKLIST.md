# sf-Compatible Mode Checklist

Reference: `sf` 2.138.6. Update this file as each item is finished.

## Step 1: Dispatch And Output
- [x] Binary name detection in `main.go`, and the `force --sf` flag
- [x] sf root command, colon-form ids, legacy command aliases
- [x] Legacy flag names (`--targetusername`, `-u`, `--targetdevhubusername`, `--apiversion`)
- [x] Deprecation warning when a legacy command id is used (`force:org:list`)
- [x] Legacy flag names of each command (`--classname`, `--outputdir`, ...; 488 names in `sf/flagaliases.go`) and their deprecation warnings
- [x] Suggestion of the closest username or alias when an org name has no authorization
- [x] Error when a flag that takes one value is given twice (`Flag --name can only be specified once`)
- [x] `--json` envelope, error envelope, human error format, table renderer
- [x] `ErrorAndExit` handler hook
- [x] `version`, `commands`

## Step 2: sf State
- [x] Config read/write (`~/.sf/config.json`, `<project>/.sf/config.json`, env)
- [x] Alias read/write (`~/.sfdx/alias.json`)
- [x] Auth file write and delete, field encryption
- [x] Org resolution (flag, env, local config, global config)
- [x] Save refreshed tokens to the sf auth file
- [x] `alias list`, `alias set`, `alias unset`
- [x] `config get`, `config list`, `config set`, `config unset`
- [x] `org list`, `org list auth`, `org display`, `org logout`

## Step 3: org
- [x] `org login web` (not exercised against a live org yet)
- [x] `org login jwt` (not exercised against a live org yet)
- [x] `org login access-token`
- [x] `org open` (format taken from the sf plugin source; the local test server has no single-access endpoint)
- [x] `org display user`
- [x] `org create scratch` (including definition file) (follows the sf plugin source; not compared with sf against a Dev Hub)
- [x] `org list limits` (format taken from the sf plugin source; the local test server has no limits resource)
- [x] `org list metadata`
- [x] `org list metadata-types`

## Step 4: data
- [x] `data query`
- [x] `data search` (format taken from the sf plugin source; the local test server has no search resource)
- [x] `data get record`, `data create record`, `data update record`, `data delete record`
- [x] `data import bulk`, `data update bulk`, `data upsert bulk`, `data delete bulk`, `data export bulk` (prints the last frame of the progress display once; sf redraws it while waiting)
- [x] `data import resume`, `data update resume`, `data upsert resume`, `data delete resume`, `data export resume`
- [x] `data bulk results`
- [x] `data resume` (follows the sf plugin source)
- [x] `force data bulk delete`, `force data bulk upsert`, `force data bulk status` (follow the sf plugin source)

## Step 5: apex, sobject, api, template
- [x] `apex run`
- [x] `apex run test` (`--output-dir` and `--detailed-coverage` are not supported)
- [x] `apex get log`, `apex list log`, `apex tail log` (formats taken from the sf plugin source; `apex tail log` is not exercised by a test)
- [x] `apex trace create`, `apex trace list`, `apex trace delete` (these exist in plugin-apex versions newer than the one bundled with sf 2.138.6)
- [x] `sobject describe`, `sobject list`
- [x] `api request rest`
- [x] `template generate apex class`, `template generate apex trigger`
- [x] `template generate visualforce page`, `template generate visualforce component`

## Step 6: project
- [x] Source collection for sf mode is built on `lib.PackageBuilder` in `sf/project_files.go`; the helpers in `command/` did not need to move
- [x] `project deploy start`, `project deploy validate` (queued output and progress layout compared with sf; the tables after a finished deploy follow the sf plugin source, because sf cannot finish a deploy against the local test server)
- [x] `project deploy quick`, `project deploy report`, `project deploy resume`, `project deploy cancel` (compared with sf against a scratch org; the deploy cache entries and the manifest cache are written as sf writes them)
- [x] `project retrieve start` (failure output compared with sf; the result table follows the sf plugin source)
- [x] `project convert source`, `project convert mdapi`

## Step 7: package
- [x] `package create` (follows the sf plugin source; not compared with sf against a Dev Hub), `package list` (compared with sf against a Dev Hub)
- [x] `package install`, `package installed list`, `package uninstall` (follows the sf plugin source; not compared with sf against a Dev Hub)
- [x] `package version create`, `package version report`, `package version promote` (follow the sf plugin source; for the report, only the error for a missing version was compared with sf), `package version list` (compared with sf against a Dev Hub, including `--concise`, `--verbose` and `--released`)

## Step 8: Documentation And Build
- [x] `docs/sf-compatibility.md` (mapping both directions, missing commands); `TestCompatibilityDocumentListsEveryCommand` fails when a command has no row
- [x] Generated `docs/sf/` from the sf command tree (`make docs`)
- [x] README section
- [x] Makefile `install-sf` target. There is no `sf` build target: `go build -o sf` would write into the `sf/` package directory

## Unimplemented Flags

Flags of implemented commands that return an error. A flag marked "not defined" is reported as a nonexistent flag; the others are reported as not supported by this mode.

- [ ] `apex run`: `--debug-level`, `--category-level` (not defined)
- [x] `apex run test`: `--output-dir`, `--detailed-coverage`
- [ ] `apex tail log`: `--debug-level`
- [x] `api request rest`: `--file` (the `formdata` body mode is not supported)
- [ ] `commands`: `--columns`, `--deprecated`, `--extended`, `--hidden`, `--no-truncate`, `--sort`, `--tree` (not defined)
- [ ] `data delete resume`, `data upsert resume`: `--target-org` (not defined; the org comes from the job cache)
- [x] `org create scratch`: `--async`, `--track-source` (the output follows a transcript of sf; not run against a Dev Hub)
- [x] `org create scratch`: `--no-ancestors`, `--source-org`, `--snapshot`, and the package ancestors of sfdx-project.json
- [x] `org create scratch`: `--client-id` (follows the sf core source; run against the Dev Hub with the connected app of force)
- [ ] `org create scratch`: the connected app of the Dev Hub authorization in the request when `--client-id` is not given (this mode requests the org with PlatformCLI)
- [x] `org login web`: `--browser`, `--scopes` (follow the sf plugin source)
- [ ] `org login web`: `--client-app`, `--username`
- [x] `org open`: `--browser`, `--source-file`
- [x] `org open`: `--private` (chrome, brave, firefox and edge as the default browser, as in sf; not compared with sf)
- [x] `package create`: `--no-namespace`, `--org-dependent`, `--error-notification-username`
- [x] `package install`: `--skip-handlers`
- [x] `package install`: `--publish-wait` (follows the sf plugin source)
- [x] `package version create`: `--installation-key`, `--branch`, `--post-install-script`, `--post-install-url`, `--releasenotes-url`, `--uninstall-script`, `--language`, `--generate-pkg-zip` (follow the source of the sf packaging library)
- [x] `package version create`: `--skip-ancestor-check`, `--verbose`, and the `dependencies`, `ancestorId`, `ancestorVersion` and `calculateTransitiveDependencies` keys of the package directory (follow the source of the sf packaging library)
- [x] `package version create`: `--definition-file`, and the `definitionFile`, `features`, `orgPreferences`, `snapshot`, `apexTestAccess`, `packageMetadataAccess`, `postInstallScript`, `uninstallScript`, `postInstallUrl` and `releaseNotesUrl` keys of the package directory (follow the source of the sf packaging library; a version with `--definition-file` was created in the Dev Hub by each program, and the reports of the two versions differ only in ids, build number and build durations; the uploaded requests were not compared)
- [ ] `package version create`: `objectSettings` in a definition file, the `unpackagedMetadata`, `seedMetadata`, `scopeProfiles` and `includeProfileUserLicenses` keys of the package directory, and profiles in the package
- [x] `package update`: `--skip-ancestor-check`, and the ancestor check of a new recommended version
- [x] `package version create list`: `--show-conversions-only`, `--verbose`
- [x] `package version list`: `--show-conversions-only`
- [x] `project convert mdapi`: `--manifest`, `--metadata-dir`, `--metadata`
- [x] `project convert source`: `--package-name`
- [x] `project deploy start`, `project deploy validate`: `--coverage-formatters`, `--junit`, `--results-dir` (the `html` and `html-spa` formats are not supported)
- [x] `project deploy report`, `project deploy resume`: `--coverage-formatters`, `--junit`, `--results-dir` (compared with sf against a scratch org)
- [x] `project deploy quick`, `report`, `resume`, `cancel`: `--use-most-recent`
- [x] `project generate manifest`: `--from-org`, `--include-packages` (compared with sf against a scratch org)
- [x] `project retrieve start`: `--single-package` (follows the sf library source)
- [ ] `project retrieve start`: more than one `--package-name`

## Flags Accepted Without Effect

- [ ] `alias unset`: `--no-prompt` (there is no prompt)
- [ ] `apex tail log`: `--color`
- [ ] `org create scratch`: `--wait` (the wait is the one in lib: up to 10 minutes)
- [x] `org display`: `--verbose` (no sfdxAuthUrl is shown) — now adds the redacted Sfdx Auth Url row, as sf does
- [x] `org list`: `--verbose`, `--clean`, `--no-prompt`, and the `Expires` column of scratch orgs (follow the plugin source)
- [ ] `org login access-token`: `--no-prompt`
- [ ] `org logout`: `--client-app`
- [ ] `package install`: `--no-prompt` (remote site settings are enabled without asking)
- [x] `package version create`: `--wait`, status line while waiting, request Id in the result, version alias in sfdx-project.json (both programs run against the Dev Hub: same output, build numbers 1 to 4 and aliases)
- [x] `package version report`: `--verbose`, and the rows and JSON fields of the report (compared with sf against the Dev Hub for a managed version without code coverage or dependencies)
- [x] `project deploy start`, `project deploy validate`: `--verbose` (deploy URL, size, file count, passed tests and the coverage table; compared with sf against a scratch org), `--ignore-conflicts`
- [ ] `project deploy start`, `project deploy validate`: `--single-package` (accepted without effect)
- [x] `project deploy resume`: `--verbose` (adds the deploy URL to the progress display; the tables follow the verbosity of the command that started the deploy, as in sf)
- [ ] `project deploy quick`: `--verbose` (not compared with sf)
- [ ] `project retrieve start`: `--wait` (`--ignore-conflicts` is done)
- [ ] `template generate project`: the `agent`, `nativemobile`, `reactinternalapp` and `reactexternalapp` templates
- [x] `template generate apex trigger`, `template generate visualforce page`, `template generate visualforce component`: `--template` (each command has one template; another value is an error, as in sf)

## Unimplemented Behavior Of Implemented Commands

- [x] `template generate apex class`: templates other than `DefaultApexClass`
- [ ] `org create scratch`: definition file key `objectSettings`, and keys other than `orgName`, `edition`, `country`, `language`, `adminEmail`, `description`, `hasSampleData`, `username`, `release`, `namespace`, `duration`, `durationDays`, `features`, `settings`
- [x] `org display`: a scratch org is described from its ScratchOrgInfo record in the Dev Hub (status, edition, org name, created by, created date), without a connected status
- [x] `org list`: scratch orgs take their status from the Dev Hub, a scratch org without a record is not listed, and inactive ones are listed only with `--all`
- [x] Connected status from an authenticated request (the list of API versions is answered without a token, so an expired authorization was reported as Connected)
- [ ] `org list`: `AuthDecryptError` for an authorization file that sf cannot decrypt (this mode tries every key it finds and may decrypt it)
- [ ] `org list`, `org display`: sf's message for an HTML response, such as the one an expired scratch org gives (this mode reports "Scratch org has expired")
- [ ] `org list`: the "Using insecure protocol" warning for an http instance URL
- [x] `data query`: sf's layout for child records of a subquery, and columns from the column metadata of the query (`columns=true`)
- [ ] `project convert source`: the order of elements in a composed custom object (sf writes the fields and other child components after the elements of the object file; this mode writes all elements in alphabetical order)
- [x] `project convert mdapi --metadata`, `--manifest`: a child component named on its own, such as `CustomField:Invoice__c.Amount__c`, selects nothing, as in sf (compared with sf). `CustomLabel:<name>` gives "No results to format" where sf reports a `NoChildTypesError`
- [x] Unknown metadata type name: the "Did you mean one of the following types?" line (compared with sf)
- [x] `package version list`: ancestor version numbers and code coverage (code coverage compared with sf; the Dev Hub used for the comparison had no version with an ancestor, so that lookup follows the plugin source)
- [ ] `force data bulk delete`, `force data bulk upsert`: batches of 10,000 records (the file is sent as one batch)
- [x] `project retrieve start`: `fileProperties`, `id` and `messages` in the JSON result, failed entries in `files` and the Warnings table for components the org does not have (compared with sf against a scratch org)
- [ ] `api request rest --include`: headers in the order received
- [ ] `config set`, `config get`: the prompt that suggests a similar config name
- [ ] A topic given without a command (`sf org`): sf's prompt to pick a command
- [ ] Error envelopes: `stack`, `cause`, and `data` fields other than `errorCode` and `message`
- [ ] Redrawn progress display for bulk, deploy, retrieve and scratch org commands

## Unimplemented Features Shared By Several Commands

- [x] `--flags-dir` on every command
- [x] `--use-most-recent` on the deploy commands (needs `~/.sf/deploy-cache.json`)
- [ ] Source tracking
  - [x] Read and write `maxRevision.json` and the `localSourceTracking` git repository in the formats of sf (sf read the files this mode wrote)
  - [x] `org enable tracking`, `org disable tracking`, `project reset tracking`, `project delete tracking`
  - [x] `project deploy preview`, `project retrieve preview` (a component of the org without local files is never reported as ignored; sf checks the paths it would be retrieved to against `.forceignore`)
  - [x] Deploy and retrieve without naming components, and update the tracking files after a deploy or retrieve (compared with sf against a scratch org)
  - [x] Conflict detection (`--ignore-conflicts`)
  - [x] Send the client name of sf (`sfdx toolbelt:`) with SOAP requests; the org records the source members of a deploy only for that client
  - [x] Retrieve a changed component into the package directory it is in
  - [x] The warning sf prints when the org does not report the source members of a deploy in time, `SF_SOURCE_MEMBER_POLLING_TIMEOUT`, `SF_DISABLE_SOURCE_MEMBER_POLLING`, and the rules of sf for the components it does not wait for (follows the source-tracking source; sf decides some of the rules by file path, this mode by component name)
  - [ ] Detection of moved files
  - [ ] Write `maxRevision.json` as fileVersion 1 when `SF_SOURCE_TRACKING_FILE_VERSION=1` (the file is read in both versions)
- [x] `.forceignore` (sf reports an error when a content file is ignored and its `-meta.xml` file is not; this mode leaves both out)
- [x] Destructive changes on deploy
- [x] Coverage and JUnit result files on the deploy commands (the files are the same as the ones sf wrote for a deploy to a scratch org; `html` and `html-spa` are not supported)
- [x] Shell completion for the sf command tree (`sf autocomplete`)

## Needs A Decision

- [x] Template files: the 192 files of the salesforcedx-templates repository (tag 66.9.1) are embedded as compressed archives in `sf/templates`, with the license in `third-party-licenses.txt` and in every release zip. The React app templates of the npm package are not in that repository and are not included

## Not Yet Verified

- [x] Run `org login web` against an org (logged in to a packaging org; the output was not compared with sf)
- [ ] Run `org login jwt` against an org
- [ ] Compare the commands marked "follows the sf plugin source" with sf output against an org or Dev Hub
  - [x] The deploy job commands, the Apex log and trace commands, `org list limits`, `org list sobject record-counts`, `org assign permset` (success, duplicate, partial success and failure), `org assign permsetlicense` (failure cases) and `data search`
  - [x] The bulk resume commands, `data bulk results` and the `force data bulk` commands
  - [x] `data import tree`, `data export tree`, `data create file` (without `--parent-id`) and `org open --url-only`
  - [x] `org generate password`, `org auth show-user-password --json` and `api request graphql` (with `--include`, sf prints the headers in the order received and the protocol as HTTP/1.1; this mode prints them sorted and the protocol the connection used)
  - [x] Against a real org instead of a local server: the `data` record and query commands, `sobject list`, `sobject describe`, `org list metadata`, `org list metadata-types` and `apex run`
  - [ ] `apex tail log`, the package commands

## Unimplemented Commands

163 commands of sf 2.138.6 and its bundled and install-on-first-use plugins. `docs/sf-compatibility.md` says what each needs. Commands under "dev" and most under "CLI management" manage the Node.js CLI and its plugins and do not apply to force.

### org (2 open, 25 done)

- [x] `org assign permset` (follows the sf plugin source)
- [x] `org assign permsetlicense` (follows the sf plugin source)
- [x] `org auth show-access-token`
- [x] `org auth show-sfdx-auth-url`
- [x] `org auth show-user-password` (follows the sf plugin source)
- [ ] `org create agent-user`
- [x] `org create sandbox` (`--async` output compared with sf; waiting and the login to the sandbox follow the source)
- [x] `org create shape` (follows the sf plugin source)
- [x] `org create snapshot` (follows the sf plugin source; not compared with sf)
- [x] `org create user` (compared with sf against a scratch org; an org authorized with the JWT flow is refused)
- [x] `org delete sandbox` (follows the sf plugin source; not compared with sf)
- [x] `org delete scratch` (follows the sf plugin source)
- [x] `org delete shape` (follows the sf plugin source)
- [x] `org delete snapshot` (the error of a Dev Hub without snapshots was compared with sf; the tables follow the plugin source)
- [x] `org disable tracking`
- [x] `org enable tracking`
- [x] `org generate password` (follows the sf plugin source)
- [x] `org get snapshot` (the error of a Dev Hub without snapshots was compared with sf; the tables follow the plugin source)
- [x] `org list shape` (follows the sf plugin source; the output without org shapes was compared with sf)
- [x] `org list snapshot` (the error of a Dev Hub without snapshots was compared with sf; the tables follow the plugin source)
- [x] `org list sobject record-counts` (follows the sf plugin source)
- [x] `org list users` (compared with sf against a scratch org)
- [x] `org login sfdx-url` (follows the sf plugin source)
- [ ] `org open agent`
- [x] `org refresh sandbox` (follows the sf plugin source and sf core; not compared with sf)
- [x] `org resume sandbox` (output for a pending sandbox compared with sf; the login to a finished sandbox was run against a production org)
- [x] `org resume scratch` (the output follows a transcript of sf; not run against a Dev Hub; an org that is not ready follows the plugin source)

### project (6 open, 7 done)

- [ ] `project convert source-behavior`
- [x] `project delete source` (compared with sf against a scratch org; deleting one file of a bundle is not supported; after a delete that the org refuses, sf updates the tracking files with `--track-source` and this mode does not)
- [x] `project delete tracking`
- [ ] `project deploy pipeline quick`
- [ ] `project deploy pipeline report`
- [ ] `project deploy pipeline resume`
- [ ] `project deploy pipeline start`
- [ ] `project deploy pipeline validate`
- [x] `project deploy preview`
- [x] `project generate manifest` (`--from-org` and `--include-packages` are not supported)
- [x] `project list ignored`
- [x] `project reset tracking`
- [x] `project retrieve preview`

### data (0 open, 3 done)

- [x] `data create file` (follows the sf plugin source)
- [x] `data export tree`
- [x] `data import tree` (follows the sf plugin source)

### apex (0 open, 2 done)

- [x] `apex get compile-status` (compared with sf against a scratch org without compile problems; the table of problems follows the plugin source)
- [x] `apex get test`

### logic (0 open, 2 done)

- [x] `logic get test` (compared with sf for Apex tests; Flow tests follow the source)
- [x] `logic run test` (compared with sf for Apex tests; Flow tests follow the source)

### flow (0 open, 2 done)

- [x] `flow get test` (follows the source of the sf flow plugin; not compared with sf)
- [x] `flow run test` (follows the source of the sf flow plugin; not compared with sf)

### api (0 open, 1 done)

- [x] `api request graphql` (follows the sf plugin source)

### schema (3 open, 1 done)

- [ ] `schema generate field`
- [ ] `schema generate platformevent`
- [ ] `schema generate sobject`
- [x] `schema generate tab`

### template (2 open, 9 done)

- [x] `template generate analytics template`
- [ ] `template generate digital-experience site`
- [x] `template generate flexipage` (the files are the same as the ones sf writes)
- [x] `template generate lightning app`
- [x] `template generate lightning component`
- [x] `template generate lightning event`
- [x] `template generate lightning interface`
- [x] `template generate lightning test`
- [x] `template generate project` (the `agent`, `nativemobile` and React app templates are not supported)
- [x] `template generate static-resource`
- [ ] `template generate ui-bundle`

### package (0 open, 16 done)

- [x] `package convert` (follows the source of the sf packaging library; `--seed-metadata` is not supported; run against a Dev Hub up to the creation of the package, which failed in the same way for sf; the conversion request was not sent)
- [x] `package delete` (follows the sf plugin source)
- [x] `package install report` (follows the sf plugin source)
- [x] `package push-upgrade abort` (follows the sf plugin source)
- [x] `package push-upgrade list` (follows the sf plugin source; the output for a package without requests was compared with sf)
- [x] `package push-upgrade report` (follows the sf plugin source)
- [x] `package push-upgrade schedule` (follows the sf plugin source; not compared with sf)
- [x] `package uninstall report` (follows the sf plugin source)
- [x] `package update` (follows the sf plugin source)
- [x] `package version create list` (compared with sf against a Dev Hub)
- [x] `package version create report` (follows the sf plugin source; the error for a missing request was compared with sf)
- [x] `package version delete` (follows the sf plugin source)
- [x] `package version displayancestry` (follows the sf plugin source; the errors for a missing package and a missing version were compared with sf)
- [x] `package version displaydependencies` (follows the sf plugin source; not compared with sf)
- [x] `package version retrieve` (follows the sf plugin source; not compared with sf)
- [x] `package version update` (follows the sf plugin source)

### package1 (0 open, 4 done)

- [x] `package1 version create` (queued, in-progress, failed and finished uploads and `--wait` compared with sf against a packaging org)
- [x] `package1 version create get` (queued, in-progress, failed and finished uploads compared with sf against a packaging org; `--wait` follows the source)
- [x] `package1 version display` (compared with sf against a packaging org with a first-generation package, and against a scratch org without packages)
- [x] `package1 version list` (compared with sf against a packaging org with a first-generation package, a Dev Hub, and a scratch org without packages)

### cmdt (0 open, 5 done)

- [x] `cmdt generate field`
- [x] `cmdt generate fromorg` (the output and the files are the same as the ones of sf for an object of a scratch org)
- [x] `cmdt generate object`
- [x] `cmdt generate record`
- [x] `cmdt generate records`

### community (0 open, 3 done)

- [x] `community create` (follows the source of the sf community plugin; not compared with sf)
- [x] `community list template` (follows the source of the sf community plugin; not compared with sf)
- [x] `community publish` (follows the source of the sf community plugin; not compared with sf)

### lightning (3 open, 0 done)

- [ ] `lightning dev app`
- [ ] `lightning dev component`
- [ ] `lightning dev site`

### agent (33 open, 0 done)

- [ ] `agent activate`
- [ ] `agent adl create`
- [ ] `agent adl delete`
- [ ] `agent adl file add`
- [ ] `agent adl file delete`
- [ ] `agent adl file list`
- [ ] `agent adl get`
- [ ] `agent adl list`
- [ ] `agent adl status`
- [ ] `agent adl update`
- [ ] `agent adl upload`
- [ ] `agent create`
- [ ] `agent deactivate`
- [ ] `agent generate agent-spec`
- [ ] `agent generate authoring-bundle`
- [ ] `agent generate template`
- [ ] `agent generate test-spec`
- [ ] `agent preview`
- [ ] `agent preview end`
- [ ] `agent preview send`
- [ ] `agent preview sessions`
- [ ] `agent preview start`
- [ ] `agent publish authoring-bundle`
- [ ] `agent test create`
- [ ] `agent test list`
- [ ] `agent test results`
- [ ] `agent test resume`
- [ ] `agent test run`
- [ ] `agent test run-eval`
- [ ] `agent trace delete`
- [ ] `agent trace list`
- [ ] `agent trace read`
- [ ] `agent validate authoring-bundle`

### force (3 open, 1 done)

- [ ] `force lightning lwc test create`
- [ ] `force lightning lwc test run`
- [ ] `force lightning lwc test setup`
- [x] `force package push-upgrade list` (alias of `package push-upgrade list`)

### ui-bundle (1 open, 0 done)

- [ ] `ui-bundle dev`

### dev (6 open, 0 done)

- [ ] `dev audit messages`
- [ ] `dev convert messages`
- [ ] `dev convert script`
- [ ] `dev generate command`
- [ ] `dev generate flag`
- [ ] `dev generate plugin`

### CLI management (21 open, 2 done)

- [x] `autocomplete` (instructions for cobra completion scripts; the mechanism differs from sf)
- [ ] `doctor`
- [ ] `help`
- [ ] `info releasenotes display`
- [ ] `plugins`
- [ ] `plugins add`
- [ ] `plugins discover`
- [ ] `plugins inspect`
- [ ] `plugins install`
- [ ] `plugins link`
- [ ] `plugins remove`
- [ ] `plugins reset`
- [ ] `plugins trust allowlist add`
- [ ] `plugins trust allowlist list`
- [ ] `plugins trust allowlist remove`
- [ ] `plugins trust verify`
- [ ] `plugins uninstall`
- [ ] `plugins unlink`
- [ ] `plugins update`
- [ ] `search`
- [ ] `update`
- [ ] `whatsnew`
- [x] `which`

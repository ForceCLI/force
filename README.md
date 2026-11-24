# force CLI

force is a command-line interface to Salesforce. It logs in to orgs, queries and loads data, deploys and retrieves metadata, runs Apex and Apex tests, and replays Apex debug logs in a step debugger. It can also run as a replacement for the Salesforce `sf` CLI.

- [Installation](#installation)
- [Quick Start](#quick-start)
- [Logging In](#logging-in)
- [Working With Data](#working-with-data)
- [Working With Metadata](#working-with-metadata)
- [Apex, Tests, And Debug Logs](#apex-tests-and-debug-logs)
- [Org Administration](#org-administration)
- [sf-Compatible Mode](#sf-compatible-mode)
- [Command Reference](#command-reference)

## Installation

### Homebrew

On macOS or Linux:

```
brew install octoberswimmer/tap/force
```

### Precompiled Binaries

Download the zip for your platform from the [latest release](https://github.com/ForceCLI/force/releases/latest), extract it, and put the `force` executable in a directory on your `PATH`.

### Go

```
go install github.com/ForceCLI/force@latest
```

### Shell Completion

```
source <(force completion bash)
```

`force completion --help` lists the other supported shells (zsh, fish, and PowerShell).

### Windows Subsystem for Linux

Under WSL, the Windows binary can be run as `force.exe`. To call it as `force`, add an alias:

```
alias force=force.exe
```

## Quick Start

```
force login                                   # log in to a production or developer org in the browser
force whoami                                  # show the logged-in user
force query "SELECT Id, Name FROM Account LIMIT 5"
force export                                  # retrieve all metadata into ./src
force push -f src/classes/MyClass.cls         # deploy a file
force test MyClassTest                        # run Apex tests
force open                                    # open the org in a browser, logged in
```

Every command has help with examples:

```
force help
force push --help
```

## Logging In

### Login Methods

`force login` opens a browser and logs in with OAuth. The session's refresh token is saved, so force gets a new access token when the old one expires.

```
force login                                        # production or developer org (or the last instance used)
force login -i test                                # sandbox
force login -i example--dev.sandbox.my.salesforce.com   # a specific My Domain
force login -u user@example.com -p password        # OAuth username-password flow
force login -i test -u user@example.com -p password
force login --device-flow --connected-app-client-id <consumer-key>   # for machines without a browser
force login --connected-app-client-id <consumer-key> -u user@example.com -k jwt.key   # JWT bearer flow
force login --connected-app-client-id <consumer-key> --connected-app-client-secret <secret>   # client credentials flow
force login -s -i my-domain.my.salesforce.com      # skip the login if already authenticated (useful with SSO)
```

The session uses the highest API version that the org supports. Pass `--api-version` to use another version, or change it later with `force apiversion`.

### DPoP-Bound Sessions

`force login --dpop` requests tokens bound to a private key with OAuth 2.0 Demonstrating Proof of Possession (RFC 9449). A copied access token can't be used without the key, which force saves with the session. It works with the browser, device, JWT bearer, and client credentials logins, but not with username-password logins. The external client app or connected app must allow DPoP; if it doesn't, Salesforce returns an ordinary token and the session works as usual.

`force open` can't open a device, JWT bearer, or client credentials DPoP session until the session has been refreshed once. [docs/dpop.md](docs/dpop.md) describes how the proofs are sent.

### Scratch Orgs

With a Dev Hub org active, `force login scratch` creates a scratch org and logs in to it.

```
force login scratch
force login scratch --duration 30 --edition Enterprise
force login scratch --feature Communities --feature PersonAccounts
force login scratch --product fsc --setting enableEnhancedNotes
force login scratch --release preview
```

`force login scratch --help` lists the editions, features, products, and settings.

### Managing Sessions

```
force logins                         # list saved logins; the active one is marked
force logins --namespace myns        # only logins for orgs with the namespace prefix myns
force logins --sfdx                  # include logins saved by the sf/sfdx CLI
force active                         # show the active login
force active user@example.com        # make another login active
force active -l user@example.com     # make a login active for the current directory only
force active --session               # print the access token
force whoami                         # show details of the logged-in user
force open                           # open the active org in a browser
force open -s /lightning/setup/SetupOneHome/home   # open a specific page
force logout -u user@example.com     # delete a saved login
```

Any command can use a login other than the active one with `-a`/`--account`:

```
force -a user@example.com query "SELECT Id FROM User"
```

### Using sf CLI Logins

When `--account` names a user or alias that force has no login for, force uses the login that the `sf`/`sfdx` CLI saved for it. `force usedxauth` makes an `sf` login the active force login:

```
force usedxauth my-scratch-alias
force usedxauth                      # the default org in the sf config
```

To run a command with an access token from somewhere else, set `SF_ACCESS_TOKEN` and `SF_INSTANCE_URL`:

```
SF_ACCESS_TOKEN=00D... SF_INSTANCE_URL=https://example.my.salesforce.com force query "SELECT Id FROM Account"
```

### Configuration Directory

Logins and settings are saved in `~/.force`. `--config` uses another directory, such as one per project or client:

```
force --config ./.force-client login
force --config ./.force-client query "SELECT Id FROM Account"
```

## Working With Data

### Query And Search

```
force query "SELECT Id, Name, Account.Name FROM Contact"
force query --format json-pretty "SELECT Id, Name FROM Account"     # csv (default), json, json-pretty, console
force query --all "SELECT Id, Name FROM Account WHERE IsDeleted = true"
force query --tooling "SELECT Id, TracedEntity.Name FROM TraceFlag"
force query --explain "SELECT Id FROM Account WHERE Name = 'Acme'"  # show the query plan
force search "FIND {Jane Doe} IN ALL FIELDS RETURNING Account (Id, Name)"
```

### Records

Field values are given as `Field:Value`. A record can be identified by its Id or by an external ID field as `Field:Value`.

```
force record get Account 001000000000000
force record get User username:user@example.com
force record create Account Name:"Acme Corp" Phone:5555550100
force record update Account 001000000000000 Industry:Energy
force record upsert Account External_Id__c:ABC123 Name:"Acme Corp"
force record delete Account 001000000000000
force record undelete 001000000000000
force record merge Contact 003000000000001 003000000000002
force record create --no-assign Case Subject:"Test" Status:New   # don't run assignment rules
```

### Bulk API 2.0

```
force bulk2 insert Account accounts.csv --wait
force bulk2 update Account updates.csv --wait
force bulk2 upsert -e External_Id__c Account data.csv --wait
force bulk2 delete Account deletes.csv --wait
force bulk2 hardDelete Account deletes.csv --wait
force bulk2 query "SELECT Id, Name FROM Account" --wait > accounts.csv
force bulk2 query --query-all "SELECT Id, Name FROM Account" --wait

force bulk2 jobs                     # list ingest jobs
force bulk2 jobs --query             # list query jobs
force bulk2 job <jobId>              # show a job's status
force bulk2 results <jobId>          # get a job's results
force bulk2 results <jobId> --failed # get only the failed records
force bulk2 abort <jobId>
force bulk2 delete-job <jobId>
```

### Bulk API 1.0

```
force bulk insert Account accounts.csv
force bulk upsert -e External_Id__c Account data.csv
force bulk query -w Account "SELECT Id, Name FROM Account"
force bulk query -p 50000 Account "SELECT Id, Name FROM Account"   # PK chunking
force bulk job <jobId>
force bulk result <jobId>
```

### Platform Events And Change Data Capture

```
force pubsub subscribe /data/ChangeEvents | jq .
force pubsub subscribe /data/AccountChangeEvent --changes    # only the changed fields
force pubsub subscribe /event/My_Event__e --earliest
force pubsub publish /event/My_Event__e '{My_Field__c: "My Value"}'
```

### REST API

Paths are relative to `/services/data/vXX.0` unless `-A` is given.

```
force rest get /limits
force rest get "/tooling/query?q=SELECT Id FROM ApexClass"
force rest get -A /services/data/
force rest post /sobjects/Account account.json
force rest patch /sobjects/Account/001000000000000 changes.json
force rest delete /sobjects/Account/001000000000000
```

## Working With Metadata

force reads and writes both metadata format (`src/classes/MyClass.cls`, `src/objects/Account.object`) and source format, the layout used by `sf` projects (`force-app/main/default/objects/Account/fields/Status__c.field-meta.xml`). Source format files are converted to metadata format when they are deployed.

### Retrieve

```
force export                                   # all metadata into ./src
force export my-dir -x Report -x Dashboard     # into my-dir, excluding some types
force fetch -t ApexClass                       # all Apex classes
force fetch -t CustomObject -n Book__c -n Author__c
force fetch -t CustomField -n Opportunity.Buyer__c
force fetch -t Aura -n MyComponent -d ~/projects/home
force fetch -x package.xml                     # the components listed in a package.xml
force fetch -t package -n "My Package"         # an unmanaged package
force fetch -t StaticResource -n Logo -u       # unpack static resource archives
```

`fetch` also takes the paths of local files or directories and retrieves the components they hold, writing them back in the same format:

```
force fetch force-app/main/default/classes
force fetch force-app/main/default/objects/Account/fields/Email_ID__c.field-meta.xml
```

When a child component, such as a field, is fetched into an existing file that holds other components, such as `objects/Opportunity.object`, the fetched component is added or replaced and the rest of the file is kept. In source format, fetching a field writes only the field's file.

### Deploy

`push` deploys files, directories, or components by type and name:

```
force push -f src/classes/MyClass.cls
force push -f src/classes/MyClass.cls -f src/pages/MyPage.page
force push -f force-app/main/default/objects/Account
force push -t ApexClass                        # all local Apex classes
force push -t StaticResource -n Logo
force push -c --test MyClassTest -f src/classes/MyClass.cls   # validate only, running a test
force push -l RunLocalTests -f src/classes
git diff HEAD^ --name-only --diff-filter=ACM | force push -f -   # files listed on stdin
```

A static resource can be stored expanded, as `fetch -u` and `sf` write a zip resource: `staticresources/Logo/` holds the archive's files beside `staticresources/Logo.resource-meta.xml`. `push`, `import`, and `force --sf project deploy start` zip the directory and deploy it as `staticresources/Logo.resource`; nothing is written to the local directory. When both the directory and `Logo.resource` exist, the directory is deployed. A resource that `sf` stores under the extension of its content type, such as `staticresources/Logo.png`, is deployed as `Logo.resource`. Pushing any file inside an expanded resource deploys the whole resource.

`import` deploys a whole directory that has a `package.xml`, in metadata or source format:

```
force import                                   # deploys ./src
force import -d my-dir -c -t                   # validate only, running all tests
force import -d my-dir -I                      # show deployment progress interactively
```

Options shared by `push` and `import` include `-c` (check only), `-l`/`--testlevel`, `--test`, `-r` (roll back on error), `-p` (purge on delete), `--reporttype junit`, and `--smart-flow-version` (deploy flows as new versions and delete inactive versions).

### Deployments

```
force deploys list
force deploys status -d 0Af000000000000
force deploys watch -d 0Af000000000000         # progress and failures, updated as the deploy runs
force deploys errors -d 0Af000000000000
force deploys cancel -d 0Af000000000000
force deploys cancel --all
force quickdeploy 0Af000000000000              # deploy a validation that succeeded
```

### Schema

```
force describe metadata                        # metadata types available in the org
force describe sobject -n Account
force sobject list
force sobject create Todo Description:string
force sobject delete Todo
force field list Todo__c
force field create Todo__c Due:DateTime required:true
force field create Inspection__c "Final Outcome":picklist picklist:"Pass, Fail, Redo"
force field delete Todo__c Due
force field type                               # supported field types
force field type email                         # required and optional attributes of a type
force security Case                            # object and field permissions by profile and permission set
force bigobject list
```

### New Apex And Visualforce Files

```
force create apexclass -n MyClass
force create apextrigger -n AccountTrigger -s Account
force create apexpage -n MyPage
force create apexcomponent -n MyComponent
```

## Apex, Tests, And Debug Logs

### Anonymous Apex

```
force apex script.apex
echo "System.debug(UserInfo.getUserName());" | force apex
force apex                           # type code, then Ctrl-D (Ctrl-Z on Windows)
```

### Tests

```
force test all
force test AccountTest ContactTest
force test AccountTest.createsAccount AccountTest.updatesAccount
force test -c AccountTest createsAccount updatesAccount
force test -n myns AccountTest       # a test class in a namespace
force test -f junit AccountTest > results.xml
force test --integration MyIntegrationTest   # an @IntegrationTest class
```

### Debug Logs And Trace Flags

```
force trace start                    # log the current user's activity
force trace start 005000000000000    # log another user's activity
force trace list
force trace delete <id>
force log                            # list debug logs
force log <id>                       # print a debug log
force log tail                       # print new debug logs as they are created
force log delete <id>
```

### Replay Debugger

`force log debug` steps through a debug log in the terminal, showing the call stack, local and static variables, and `System.debug` output at each statement. Breakpoints can be set on any line of a class or trigger that ran.

```
force log debug 07L000000000000      # replay a log from the org
force log debug --file apex.log      # replay a saved log
force log debug --web 07L000000000000   # replay in a browser
force apex --debug script.apex       # run anonymous Apex, then replay it
force test --debug AccountTest.createsAccount   # run tests, show results, then replay them
force trace start --replay           # log at the level the debugger needs, to replay later
```

The log must have been captured with `APEX_CODE` at `FINEST`. `force apex --debug` and `force test --debug` capture such a log: while the code runs, your `DEVELOPER_LOG` trace flag uses a `Force_CLI_Replay` debug level, and afterwards the trace flag gets its previous debug level and expiration date back.

Keys:

| Key | Action |
| --- | --- |
| `n` | step over |
| `s` | step in |
| `o` | step out |
| `c` | continue to the next breakpoint |
| `p` | step back to the previous statement in the current method or a caller |
| `b` | set or remove a breakpoint |
| `f` | open a class or trigger |
| `r` | restart from the beginning of the log |
| `tab` | move between panes |
| `enter` | expand or collapse a variable |
| `q` | quit |

Lines on which a statement ran are shown brighter than the others, and a breakpoint can stop only on those lines.

A debug log records a variable's value only when the variable is assigned. A list, set, or map changed by a method such as `add` or `put` is shown with the value of its last assignment and that assignment's line (`accounts = []  as of line 109`). To see the full contents of variables at a line, set a checkpoint there. Salesforce saves a heap dump the first time execution reaches the line, and the debugger shows the variables, objects, and collections from it:

```
force test --debug --checkpoint AccountService:42 AccountServiceTest
force apex --debug --checkpoint AccountService:42 script.apex
```

Up to 5 checkpoints can be set, including any set in VS Code. They are deleted after the run.

Class and trigger source is fetched from the org, so a class changed after the log was captured shows lines that don't match the code that ran. Classes in managed packages have no source to show.

`--web` serves the debugger on 127.0.0.1 with a random token in the URL, until the page's Quit button is pressed or force is stopped with Ctrl-C. Clicking a line number sets or removes a breakpoint, and the keys are the same as in the terminal.

## Org Administration

```
force limits                         # API and other org limits
force limits -w 25                   # highlight limits with less than 25% remaining
force apiversion                     # show the API version of the session
force apiversion 62.0                # change it
force apiversion list                # versions supported by the org
force password status user@example.com
force password reset user@example.com
force password change user@example.com 'n3w-Pa$$word'
force eventlogfile                   # list event log files
force eventlogfile 0AT000000000000   # download one
force oauth create MyApp http://localhost:3835/oauth/callback   # create a connected app
force notify true                    # show a desktop notification when tests finish
```

### Packages

```
force package installed              # packages installed in the org
force package install -i 04t000000000000   # install a package version
force package install myns 1.2       # install a managed package version by namespace
force package create --help          # create a second-generation package
force package version list --help
force package version create --help
force package version release --help
force package uninstall --help
```

## sf-Compatible Mode

When the binary is run under the name `sf`, or as `force --sf`, it accepts the command names and flags of the Salesforce `sf` CLI, uses the logins, aliases, and config values of `sf` (`~/.sfdx` and `~/.sf`), and prints output in the format of `sf`, including the `--json` envelope. Scripts written for `sf` can run without Node.js.

```
make install-sf                      # when building from source: install force and an sf symlink next to it
ln -s "$(command -v force)" ~/bin/sf # or create the symlink yourself

sf org login web --alias dev --set-default
sf data query -q "SELECT Id, Name FROM Account" --json
sf project deploy start --source-dir force-app
sf template generate apex class --name MyClass --output-dir force-app/main/default/classes
force --sf org list                  # the same mode without the symlink
```

167 `sf` commands are implemented. [docs/sf-compatibility.md](docs/sf-compatibility.md) lists them with the `force` command each corresponds to, the flags that are not supported, and the `sf` commands that are not implemented. [docs/sf/sf.md](docs/sf/sf.md) has the help for each command.

### Plugins

sf plugins are npm packages, which this mode cannot load. A plugin of this mode is a native executable, published for each operating system and architecture as an OCI artifact on a container registry. Installing one downloads the executable for your platform into `~/.sf/plugins`; its commands then run it as a child process.

```
sf plugins install sfdx-git-delta            # ghcr.io/forcecli/sf-plugins/sfdx-git-delta:latest
sf plugins install ghcr.io/acme/my-plugin:v1.2.0
sf sgd source delta --from origin/main --output-dir delta
sf plugins                                   # list installed plugins
sf plugins update
sf plugins uninstall sfdx-git-delta
sf plugins link ./my-plugin                  # register a locally built executable
```

A name without a registry host is looked up under `ghcr.io/forcecli/sf-plugins`. A download is checked against the SHA-256 digest in the registry manifest; plugins are not signed, so install only plugins from publishers you trust. [docs/sf-plugins.md](docs/sf-plugins.md) describes the plugin protocol and how to write and publish a plugin with the `sfplugin` Go package.

## Command Reference

[docs/force.md](docs/force.md) has the help text of every command, which `force <command> --help` also prints.

## Contributing

See [CONTRIBUTING.md](CONTRIBUTING.md).

## License

force is released under the MIT license in [LICENSE](LICENSE). [third-party-licenses.txt](third-party-licenses.txt) holds the licenses of third-party code compiled into the binary, and is included in every release zip.

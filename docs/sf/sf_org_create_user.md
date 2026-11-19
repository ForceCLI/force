## sf org create user

Create a user for a scratch org.

```
sf org create user [flags]
```

### Options

```
      --api-version string       Override the api version used for api requests made by this command
  -f, --definition-file string   File path to a user definition file for customizing the new user.
  -h, --help                     help for user
      --json                     Format output as json.
  -a, --set-alias string         Set an alias for the created username to reference in other CLI commands.
  -s, --set-unique-username      Force the username, if specified in the definition file or at the command line, to be unique by appending the org ID.
  -o, --target-org string        Username or alias of the target org.
```

### SEE ALSO

* [sf org create](sf_org_create.md)	 - Create scratch orgs, sandboxes, users, org shapes and snapshots.


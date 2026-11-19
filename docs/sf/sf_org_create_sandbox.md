## sf org create sandbox

Create a sandbox org.

```
sf org create sandbox [flags]
```

### Options

```
  -a, --alias string                 Alias for the sandbox org.
      --async                        Request the sandbox creation, but don't wait for it to complete.
  -f, --definition-file string       Path to a sandbox definition file.
  -h, --help                         help for sandbox
      --json                         Format output as json.
  -l, --license-type string          Type of sandbox license.
  -n, --name string                  Name of the sandbox org.
      --no-prompt                    Don't prompt for confirmation about the sandbox configuration.
      --no-track-source              Do not use source tracking for this sandbox.
  -i, --poll-interval int            Number of seconds to wait between retries. (default 30)
  -s, --set-default                  Set the sandbox org as your default org.
      --source-id string             ID of the sandbox org to clone.
      --source-sandbox-name string   Name of the sandbox org to clone.
  -o, --target-org string            Username or alias of the target org.
  -w, --wait int                     Number of minutes to wait for the sandbox org to be ready. (default 30)
```

### SEE ALSO

* [sf org create](sf_org_create.md)	 - Create scratch orgs, sandboxes, users, org shapes and snapshots.


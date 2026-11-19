## sf org refresh sandbox

Refresh a sandbox org using the sandbox name.

```
sf org refresh sandbox [flags]
```

### Options

```
      --async                        Request the sandbox refresh, but don't wait for it to complete.
  -f, --definition-file string       Path to a sandbox definition file for overriding its configuration when you refresh it.
  -h, --help                         help for sandbox
      --json                         Format output as json.
  -n, --name string                  Name of the existing sandbox org in your production org that you want to refresh.
      --no-auto-activate             Disable auto-activation of the sandbox after a successful refresh.
      --no-prompt                    Don't prompt for confirmation about the sandbox refresh.
  -i, --poll-interval int            Number of seconds to wait between status polling requests. (default 30)
      --source-id string             ID of the sandbox org that becomes the new source org for the refreshed sandbox.
      --source-sandbox-name string   Name of the sandbox org that becomes the new source org for the refreshed sandbox.
  -o, --target-org string            Username or alias of the target org.
  -w, --wait int                     Number of minutes to poll for sandbox refresh status. (default 30)
```

### SEE ALSO

* [sf org refresh](sf_org_refresh.md)	 - Refresh a sandbox.


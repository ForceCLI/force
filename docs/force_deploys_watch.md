## force deploys watch

Monitor metadata deploy

### Synopsis

Monitor metadata deploy

The deployment status and progress bars stay at the top of the screen. Test
and component failures are listed below them in a scrollable area sized to
the remaining terminal height. Use the arrow keys, PgUp and PgDn to scroll
the failures, and q to quit. Use "force deploys errors" to print the full
list of failures.

```
force deploys watch [flags]
```

### Options

```
  -d, --deploy-id string   Deploy Id to watch
  -h, --help               help for watch
```

### Options inherited from parent commands

```
  -a, --account username    account username to use
  -V, --apiversion string   API version to use
      --config string       config directory to use (default: .force)
```

### SEE ALSO

* [force deploys](force_deploys.md)	 - Manage metadata deployments


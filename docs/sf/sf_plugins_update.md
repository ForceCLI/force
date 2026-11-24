## sf plugins update

Update installed plugins.

### Synopsis

Update installed plugins.

The tag each plugin was installed with is looked up again.  A plugin is
downloaded when the tag points to a different digest than the one recorded.
Linked plugins are not changed.  Without arguments every installed plugin
is checked.

```
sf plugins update [plugin...] [flags]
```

### Options

```
  -h, --help   help for update
      --json   Format output as json.
```

### SEE ALSO

* [sf plugins](sf_plugins.md)	 - List installed plugins.


## sf plugins link

Register a plugin executable on this machine for development.

### Synopsis

Register a plugin executable on this machine for development.

The executable is run with the argument sf-plugin-manifest to read its
manifest.  Its commands then run the executable at that path, so a rebuilt
executable is used without linking again.  Link again after the manifest
changes.

```
sf plugins link path [flags]
```

### Options

```
  -h, --help   help for link
      --json   Format output as json.
```

### SEE ALSO

* [sf plugins](sf_plugins.md)	 - List installed plugins.


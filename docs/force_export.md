## force export

Export metadata to a local directory

```
force export [dir] [flags]
```

### Examples

```

  force export
  force export org/schema
  force export -x ApexClass -x CustomObject

```

### Options

```
  -x, --exclude strings   exclude metadata type
  -h, --help              help for export
  -w, --warnings          display warnings about metadata that cannot be retrieved
```

### Options inherited from parent commands

```
  -a, --account username    account username to use
  -V, --apiversion string   API version to use
      --config string       config directory to use (default: .force)
```

### SEE ALSO

* [force](force.md)	 - force CLI


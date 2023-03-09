## force search

Execute a SOSL statement

```
force search [flags] <sosl statement>
```

### Examples

```

  force search "FIND {Jane Doe} IN ALL FIELDS RETURNING Account (Id, Name)"

```

### Options

```
  -h, --help   help for search
```

### Options inherited from parent commands

```
  -a, --account username    account username to use
  -V, --apiversion string   API version to use
```

### SEE ALSO

* [force](force.md)	 - force CLI


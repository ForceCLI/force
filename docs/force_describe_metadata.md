## force describe metadata

Describe metadata

### Synopsis

List the metadata in the org.  With no type specified, lists all
metadata types supported.  Specifying a type will list the individual metadata
components of that type.


```
force describe metadata [flags]
```

### Examples

```

  force describe metadata
  force describe metadata -t MatchingRule -j
  
```

### Options

```
  -h, --help          help for metadata
  -j, --json          json output
  -t, --type string   type of metadata
```

### Options inherited from parent commands

```
  -a, --account username    account username to use
  -V, --apiversion string   API version to use
```

### SEE ALSO

* [force describe](force_describe.md)	 - Describe the types of metadata available in the org


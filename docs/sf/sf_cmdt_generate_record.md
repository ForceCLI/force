## sf cmdt generate record

Generate a new record for a given custom metadata type in the current project.

```
sf cmdt generate record [flags]
```

### Options

```
  -h, --help                      help for record
  -i, --input-directory string    Directory from which to get the custom metadata type definition from. (default "force-app/main/default/objects")
      --json                      Format output as json.
  -l, --label string              Label for the new record.
  -d, --output-directory string   Directory to store newly-created custom metadata record files. (default "force-app/main/default/customMetadata")
  -p, --protected string          Protect the record when it's in a managed package. (default "false")
  -n, --record-name string        Name of the new record.
  -t, --type-name string          API name of the custom metadata type to create a record for; must end in "__mdt".
```

### SEE ALSO

* [sf cmdt generate](sf_cmdt_generate.md)	 - Generate custom metadata types, fields and records.


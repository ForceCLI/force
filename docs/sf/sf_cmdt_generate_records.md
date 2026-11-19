## sf cmdt generate records

Generate new custom metadata type records from a CSV file.

```
sf cmdt generate records [flags]
```

### Options

```
  -f, --csv string                Pathname of the CSV file.
  -h, --help                      help for records
  -i, --input-directory string    Directory from which to get the custom metadata type definition from. (default "force-app/main/default/objects")
      --json                      Format output as json.
  -n, --name-column string        Column used to determine the name of the record. (default "Name")
  -d, --output-directory string   Directory to store newly-created custom metadata record files. (default "force-app/main/default/customMetadata")
  -t, --type-name string          API name of the custom metadata type to create a record for.
```

### SEE ALSO

* [sf cmdt generate](sf_cmdt_generate.md)	 - Generate custom metadata types, fields and records.


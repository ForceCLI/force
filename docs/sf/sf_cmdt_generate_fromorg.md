## sf cmdt generate fromorg

Generate a custom metadata type and all its records from a Salesforce object.

```
sf cmdt generate fromorg [flags]
```

### Options

```
      --api-version string             Override the api version used for api requests made by this command
  -n, --dev-name string                Name of the custom metadata type.
  -h, --help                           help for fromorg
  -i, --ignore-unsupported             Ignore unsupported field types.
      --json                           Format output as json.
  -l, --label string                   Label for the custom metadata type.
  -p, --plural-label string            Plural version of the label value; if blank, uses label.
  -r, --records-output-dir string      Directory to store newly-created custom metadata record files. (default "force-app/main/default/customMetadata")
  -s, --sobject string                 API name of the source Salesforce object used to generate the custom metadata type.
  -o, --target-org string              Username or alias of the target org.
  -d, --type-output-directory string   Directory to store newly-created custom metadata type files. (default "force-app/main/default/objects")
  -v, --visibility string              Who can see the custom metadata type. (default "Public")
```

### SEE ALSO

* [sf cmdt generate](sf_cmdt_generate.md)	 - Generate custom metadata types, fields and records.


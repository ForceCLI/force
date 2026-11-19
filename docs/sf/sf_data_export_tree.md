## sf data export tree

Export data from an org into one or more JSON files.

```
sf data export tree [flags]
```

### Options

```
      --api-version string   Override the api version used for api requests made by this command
  -h, --help                 help for tree
      --json                 Format output as json.
  -d, --output-dir string    Directory in which to generate the JSON files; default is current directory.
  -p, --plan                 Generate multiple sObject tree files and a plan definition file for aggregated import.
  -x, --prefix string        Prefix of generated files.
  -q, --query stringArray    SOQL query, or filepath of a file that contains the query, to retrieve records.
  -o, --target-org string    Username or alias of the target org.
```

### SEE ALSO

* [sf data export](sf_data_export.md)	 - Export records.


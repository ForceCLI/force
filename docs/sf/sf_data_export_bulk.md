## sf data export bulk

Bulk export records from an org into a file using a SOQL query. Uses Bulk API 2.0.

```
sf data export bulk [flags]
```

### Options

```
      --all-rows                  Include records that have been soft-deleted due to a merge or delete.
      --api-version string        Override the api version used for api requests made by this command
      --column-delimiter string   Column delimiter to be used when writing CSV output.
  -h, --help                      help for bulk
      --json                      Format output as json.
      --line-ending string        Line ending to be used when writing CSV output.
      --output-file string        File where records are written.
  -q, --query string              SOQL query to execute.
      --query-file string         File that contains the SOQL query.
  -r, --result-format string      Format to write the results. (default "csv")
  -o, --target-org string         Username or alias of the target org.
  -w, --wait int                  Time to wait for the command to finish, in minutes.
```

### SEE ALSO

* [sf data export](sf_data_export.md)	 - Export records.


## sf data import bulk

Bulk import records into a Salesforce object from a CSV file. Uses Bulk API 2.0.

```
sf data import bulk [flags]
```

### Options

```
      --api-version string        Override the api version used for api requests made by this command
      --column-delimiter string   Column delimiter used in the CSV file.
  -f, --file string               CSV file that contains the Salesforce object records.
  -h, --help                      help for bulk
      --json                      Format output as json.
      --line-ending string        Line ending used in the CSV file.
  -s, --sobject string            API name of the Salesforce object, either standard or custom.
  -o, --target-org string         Username or alias of the target org.
  -w, --wait int                  Time to wait for the command to finish, in minutes.
```

### SEE ALSO

* [sf data import](sf_data_import.md)	 - Import records.


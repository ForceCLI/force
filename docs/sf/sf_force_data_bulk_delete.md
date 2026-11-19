## sf force data bulk delete

Bulk delete records from an org using a CSV file. Uses Bulk API 1.0.

```
sf force data bulk delete [flags]
```

### Options

```
      --api-version string   Override the api version used for api requests made by this command
  -f, --file string          CSV file that contains the records.
  -h, --help                 help for delete
      --json                 Format output as json.
  -s, --sobject string       API name of the Salesforce object, either standard or custom.
  -o, --target-org string    Username or alias of the target org.
  -w, --wait int             Number of minutes to wait for the command to complete before displaying the results.
```

### SEE ALSO

* [sf force data bulk](sf_force_data_bulk.md)	 - Bulk API 1.0 commands.


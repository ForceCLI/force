## sf force data bulk upsert

Bulk upsert records to an org from a CSV file. Uses Bulk API 1.0.

```
sf force data bulk upsert [flags]
```

### Options

```
      --api-version string   Override the api version used for api requests made by this command
  -i, --external-id string   ID of the external ID field.
  -f, --file string          CSV file that contains the records.
  -h, --help                 help for upsert
      --json                 Format output as json.
  -r, --serial               Run batches in serial mode.
  -s, --sobject string       API name of the Salesforce object, either standard or custom.
  -o, --target-org string    Username or alias of the target org.
  -w, --wait int             Number of minutes to wait for the command to complete before displaying the results.
```

### SEE ALSO

* [sf force data bulk](sf_force_data_bulk.md)	 - Bulk API 1.0 commands.


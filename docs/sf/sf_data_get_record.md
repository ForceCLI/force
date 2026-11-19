## sf data get record

Retrieve and display a single record of a Salesforce or Tooling API object.

```
sf data get record [flags]
```

### Options

```
      --api-version string   Override the api version used for api requests made by this command
  -h, --help                 help for record
      --json                 Format output as json.
  -i, --record-id string     ID of the record.
  -s, --sobject string       API name of the Salesforce or Tooling API object.
  -o, --target-org string    Username or alias of the target org.
  -t, --use-tooling-api      Use Tooling API so you can work with Tooling API objects.
  -w, --where string         List of <fieldName>=<value> pairs that identify the record.
```

### SEE ALSO

* [sf data get](sf_data_get.md)	 - Retrieve a record.


## sf data query

Execute a SOQL query.

```
sf data query [flags]
```

### Options

```
      --all-rows               Include deleted records. By default, deleted records are not returned.
      --api-version string     Override the api version used for api requests made by this command
  -f, --file string            File that contains the SOQL query.
  -h, --help                   help for query
      --json                   Format output as json.
      --output-file string     File where records are written; only CSV and JSON output formats are supported.
  -q, --query string           SOQL query to execute.
  -r, --result-format string   Format to display the results; the --json flag overrides this flag. (default "human")
  -o, --target-org string      Username or alias of the target org.
  -t, --use-tooling-api        Use Tooling API so you can run queries on Tooling API objects.
```

### SEE ALSO

* [sf data](sf_data.md)	 - Manage records in your org.


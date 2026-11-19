## sf apex trace create

Create a trace flag for a user or automated process.

```
sf apex trace create [flags]
```

### Options

```
      --api-version string        Override the api version used for api requests made by this command
  -d, --debug-level string        Developer name of the debug level to assign to the trace flag.
      --duration int              Number of minutes the trace flag remains active. (default 30)
  -h, --help                      help for create
      --json                      Format output as json.
  -l, --log-type string           Type of log to generate. (default "DEVELOPER_LOG")
  -o, --target-org string         Username or alias of the target org.
  -i, --traced-entity-id string   ID of the user or automated process to trace.
```

### SEE ALSO

* [sf apex trace](sf_apex_trace.md)	 - Manage trace flags.


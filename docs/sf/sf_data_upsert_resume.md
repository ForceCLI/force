## sf data upsert resume

Resume a bulk upsert job that you previously started. Uses Bulk API 2.0.

```
sf data upsert resume [flags]
```

### Options

```
      --api-version string   Override the api version used for api requests made by this command
  -h, --help                 help for resume
  -i, --job-id string        Job ID of the bulk job.
      --json                 Format output as json.
      --use-most-recent      Use the job ID of the bulk job that was most recently run.
  -w, --wait int             Time to wait for the command to finish, in minutes. (default 5)
```

### SEE ALSO

* [sf data upsert](sf_data_upsert.md)	 - Upsert records.


## force bulk insert

Create records from csv file using Bulk API

```
force bulk insert <object> <file> [flags]
```

### Options

```
  -m, --concurrencymode mode   Concurrency mode.  Valid options are Serial and Parallel. (default "Parallel")
  -f, --format format          file format (default "CSV")
  -h, --help                   help for insert
  -w, --wait                   Wait for job to complete
```

### Options inherited from parent commands

```
  -a, --account username    account username to use
  -V, --apiversion string   API version to use
```

### SEE ALSO

* [force bulk](force_bulk.md)	 - Load csv file or query data using Bulk API


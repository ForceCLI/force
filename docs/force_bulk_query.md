## force bulk query

Query records using Bulk API

```
force bulk query <object> <query> [flags]
```

### Options

```
  -p, --chunk records          PK chunking size (number of records)
  -m, --concurrencymode mode   Concurrency mode.  Valid options are Serial and Parallel. (default "Parallel")
  -f, --format format          file format (default "CSV")
  -h, --help                   help for query
  -i, --interactive            interactive mode.  implies --wait
      --parent object          Parent object to use for PK chunking
  -A, --query-all              query all records including deleted and archived
  -w, --wait                   Wait for job to complete
```

### Options inherited from parent commands

```
  -a, --account username    account username to use
  -V, --apiversion string   API version to use
```

### SEE ALSO

* [force bulk](force_bulk.md)	 - Load csv file or query data using Bulk API


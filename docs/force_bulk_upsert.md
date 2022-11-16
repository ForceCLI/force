## force bulk upsert

Upsert records from csv file using Bulk API

```
force bulk upsert <External_Id_Field__c> <object> <file> [flags]
```

### Options

```
  -b, --batchsize int          Batch size (default 10000)
  -m, --concurrencymode mode   Concurrency mode.  Valid options are Serial and Parallel. (default "Parallel")
  -e, --externalid string      The external Id field for upserting data
  -f, --format format          file format (default "CSV")
  -h, --help                   help for upsert
  -w, --wait                   Wait for job to complete
```

### Options inherited from parent commands

```
  -a, --account username    account username to use
  -V, --apiversion string   API version to use
```

### SEE ALSO

* [force bulk](force_bulk.md)	 - Load csv file or query data using Bulk API


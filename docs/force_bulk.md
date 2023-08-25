## force bulk

Load csv file or query data using Bulk API

### Examples

```

  force bulk insert Account [csv file]
  force bulk update Account [csv file]
  force bulk delete Account [csv file]
  force bulk upsert -e ExternalIdField__c Account [csv file]
  force bulk job [job id]
  force bulk batches [job id]
  force Bulk batch [job id] [batch id]
  force bulk query [-wait | -w] Account [SOQL]
  force bulk query [-chunk | -p]=50000 Account [SOQL]
  force bulk retrieve [job id] [batch id]

```

### Options

```
  -h, --help   help for bulk
```

### Options inherited from parent commands

```
  -a, --account username    account username to use
  -V, --apiversion string   API version to use
```

### SEE ALSO

* [force](force.md)	 - force CLI
* [force bulk batch](force_bulk_batch.md)	 - Show bulk job batch details
* [force bulk batches](force_bulk_batches.md)	 - List bulk job batches
* [force bulk delete](force_bulk_delete.md)	 - Delete records using Bulk API
* [force bulk hardDelete](force_bulk_hardDelete.md)	 - Hard delete records using Bulk API
* [force bulk insert](force_bulk_insert.md)	 - Create records from csv file using Bulk API
* [force bulk job](force_bulk_job.md)	 - Show bulk job details
* [force bulk query](force_bulk_query.md)	 - Query records using Bulk API
* [force bulk retrieve](force_bulk_retrieve.md)	 - Retrieve query results using Bulk API
* [force bulk update](force_bulk_update.md)	 - Update records from csv file using Bulk API
* [force bulk upsert](force_bulk_upsert.md)	 - Upsert records from csv file using Bulk API


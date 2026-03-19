## force bulk2

Use Bulk API 2.0 for data loading and querying

### Synopsis

Bulk API 2.0 provides a REST-based interface for data loading and querying with automatic batch management.

### Examples

```

  force bulk2 insert Account accounts.csv --wait
  force bulk2 update Account updates.csv --wait
  force bulk2 upsert -e External_Id__c Account data.csv --wait
  force bulk2 delete Account deletes.csv --wait
  force bulk2 query "SELECT Id, Name FROM Account LIMIT 100" --wait
  force bulk2 job <jobId>
  force bulk2 jobs
  force bulk2 jobs --query
  force bulk2 results <jobId>
  force bulk2 results <jobId> --failed
  force bulk2 abort <jobId>
  force bulk2 delete-job <jobId>

```

### Options

```
  -h, --help   help for bulk2
```

### Options inherited from parent commands

```
  -a, --account username    account username to use
  -V, --apiversion string   API version to use
      --config string       config directory to use (default: .force)
```

### SEE ALSO

* [force](force.md)	 - force CLI
* [force bulk2 abort](force_bulk2_abort.md)	 - Abort a bulk job
* [force bulk2 delete](force_bulk2_delete.md)	 - Delete records using Bulk API 2.0
* [force bulk2 delete-job](force_bulk2_delete-job.md)	 - Delete a bulk job
* [force bulk2 hardDelete](force_bulk2_hardDelete.md)	 - Hard delete records using Bulk API 2.0
* [force bulk2 insert](force_bulk2_insert.md)	 - Insert records from CSV file using Bulk API 2.0
* [force bulk2 job](force_bulk2_job.md)	 - Show bulk job details
* [force bulk2 jobs](force_bulk2_jobs.md)	 - List bulk jobs
* [force bulk2 query](force_bulk2_query.md)	 - Query records using Bulk API 2.0
* [force bulk2 results](force_bulk2_results.md)	 - Get job results
* [force bulk2 update](force_bulk2_update.md)	 - Update records from CSV file using Bulk API 2.0
* [force bulk2 upsert](force_bulk2_upsert.md)	 - Upsert records from CSV file using Bulk API 2.0


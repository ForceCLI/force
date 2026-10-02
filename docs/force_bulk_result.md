## force bulk result

Retrieve job results using Bulk API

### Synopsis

Retrieve job results using Bulk API.

When only a job id is given, the results of every batch in the job are
retrieved and combined.  For CSV jobs, the header row is included once.  For
JSON jobs, the results are written as JSON Lines, one object per line.

```
force bulk result <jobId> [batchId] [flags]
```

### Options

```
  -h, --help   help for result
```

### Options inherited from parent commands

```
  -a, --account username    account username to use
  -V, --apiversion string   API version to use
      --config string       config directory to use (default: .force)
```

### SEE ALSO

* [force bulk](force_bulk.md)	 - Load csv file or query data using Bulk API


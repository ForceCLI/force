## force datapipe

Manage DataPipes

### Examples

```

  force datapipe create -n=MyPipe -l="My Pipe" -t=Pig -v=34.0 \
  -c="A = load 'force://soql/Select Id, Name From Contact' using \
  gridforce.hadoop.pig.loadstore.func.ForceStorage();"

```

### Options

```
  -h, --help   help for datapipe
```

### Options inherited from parent commands

```
  -a, --account username    account username to use
  -V, --apiversion string   API version to use
      --config string       config directory to use (default: .force)
```

### SEE ALSO

* [force](force.md)	 - force CLI
* [force datapipe create](force_datapipe_create.md)	 - Create Data Pipeline
* [force datapipe createjob](force_datapipe_createjob.md)	 - Create Data Pipeline Job
* [force datapipe delete](force_datapipe_delete.md)	 - Delete Data Pipeline
* [force datapipe list](force_datapipe_list.md)	 - List Data Pipelines
* [force datapipe listjobs](force_datapipe_listjobs.md)	 - List Data Pipeline Jobs
* [force datapipe queryjob](force_datapipe_queryjob.md)	 - Query Data Pipeline Job
* [force datapipe update](force_datapipe_update.md)	 - Update Data Pipeline


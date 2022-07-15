## force datapipe update

Update Data Pipeline

```
force datapipe update [flags]
```

### Options

```
  -v, --apiversion string      script content (default "45.0")
  -h, --help                   help for update
  -l, --masterlabel string     master label
  -n, --name string            data pipeline name
  -c, --scriptcontent string   script content (default "\n-- Sample script for a data pipeline\nA = load 'ffx://REPLACE_ME' using gridforce.hadoop.pig.loadstore.func.ForceStorage();\nStore A  into 'ffx://REPLACE_ME_TOO' using gridforce.hadoop.pig.loadstore.func.ForceStorage();\n")
  -t, --scripttype string      script type (default "Pig")
```

### Options inherited from parent commands

```
  -a, --account username   account username to use
```

### SEE ALSO

* [force datapipe](force_datapipe.md)	 - Manage DataPipes


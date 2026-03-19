## force bulk2 upsert

Upsert records from CSV file using Bulk API 2.0

```
force bulk2 upsert -e <External_Id_Field__c> <object> <file> [flags]
```

### Options

```
      --delimiter string    Column delimiter (COMMA, TAB, PIPE, SEMICOLON, CARET, BACKQUOTE) (default "COMMA")
  -e, --externalid string   External ID field for upserting (required)
  -h, --help                help for upsert
  -i, --interactive         Interactive mode (implies --wait)
      --lineending string   Line ending (LF or CRLF) (default "LF")
  -w, --wait                Wait for job to complete
```

### Options inherited from parent commands

```
  -a, --account username    account username to use
  -V, --apiversion string   API version to use
      --config string       config directory to use (default: .force)
```

### SEE ALSO

* [force bulk2](force_bulk2.md)	 - Use Bulk API 2.0 for data loading and querying


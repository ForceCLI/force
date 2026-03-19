## force bulk2 query

Query records using Bulk API 2.0

```
force bulk2 query <soql> [flags]
```

### Options

```
      --delimiter string    Column delimiter for results (COMMA, TAB, PIPE, SEMICOLON, CARET, BACKQUOTE) (default "COMMA")
  -h, --help                help for query
      --lineending string   Line ending for results (LF or CRLF) (default "LF")
  -A, --query-all           Include deleted and archived records
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


## force query

Execute a SOQL statement

```
force query [flags] <soql statement>
```

### Examples

```

  force query "SELECT Id, Name, Account.Name FROM Contact"
  force query --format csv "SELECT Id, Name, Account.Name FROM Contact"
  force query --all "SELECT Id, Name FROM Account WHERE IsDeleted = true"
  force query --tooling "SELECT Id, TracedEntity.Name, ApexCode FROM TraceFlag"
  force query --user me@example.com "SELECT Id, Name, Account.Name FROM Contact"

```

### Options

```
  -A, --all             use queryAll to include deleted and archived records in query results
  -e, --explain         return query plans
  -f, --format string   output format: csv, json, json-pretty, console (default "console")
  -h, --help            help for query
  -t, --tooling         use Tooling API
```

### Options inherited from parent commands

```
  -a, --account username    account username to use
  -V, --apiversion string   API version to use
      --config string       config directory to use (default: .force)
```

### SEE ALSO

* [force](force.md)	 - force CLI


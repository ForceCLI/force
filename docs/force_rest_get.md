## force rest get

Execute a REST GET request

```
force rest get <url> [flags]
```

### Examples

```

  force rest get "/tooling/query?q=Select id From Account"
  force rest get /appMenu/AppSwitcher
  force rest get -a /services/data/

```

### Options

```
  -h, --help   help for get
```

### Options inherited from parent commands

```
  -A, --absolute           use URL as-is (do not prepend /services/data/vXX.0)
  -a, --account username   account username to use
```

### SEE ALSO

* [force rest](force_rest.md)	 - Execute a REST request


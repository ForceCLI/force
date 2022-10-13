## force rest

Execute a REST request

### Examples

```

  force rest get "/tooling/query?q=Select id From Account"
  force rest get /appMenu/AppSwitcher
  force rest get -a /services/data/
  force rest post "/tooling/sobjects/CustomField/00D9A0000000TgcUAE" path/to/definition.json
  force rest put "/tooling/sobjects/CustomField/00D9A0000000TgcUAE" path/to/definition.json

```

### Options

```
  -A, --absolute   use URL as-is (do not prepend /services/data/vXX.0)
  -h, --help       help for rest
```

### Options inherited from parent commands

```
  -a, --account username    account username to use
  -V, --apiversion string   API version to use
```

### SEE ALSO

* [force](force.md)	 - force CLI
* [force rest get](force_rest_get.md)	 - Execute a REST GET request
* [force rest patch](force_rest_patch.md)	 - Execute a REST PATCH request
* [force rest post](force_rest_post.md)	 - Execute a REST POST request
* [force rest put](force_rest_put.md)	 - Execute a REST PUT request


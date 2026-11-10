## force rest delete

Execute a REST DELETE request

```
force rest delete <url> [flags]
```

### Examples

```

  force rest delete "/sobjects/Account/001D000000INjVe"
  force rest delete /connect/referral-management/assignment-rules/My_Rule

```

### Options

```
  -h, --help   help for delete
```

### Options inherited from parent commands

```
  -A, --absolute            use URL as-is (do not prepend /services/data/vXX.0)
  -a, --account username    account username to use
  -V, --apiversion string   API version to use
      --config string       config directory to use (default: .force)
```

### SEE ALSO

* [force rest](force_rest.md)	 - Execute a REST request


## force usedxauth

Authenticate with SFDX Scratch Org User

### Synopsis


Authenticate with SFDX Scratch Org User. If a user or alias is passed to the command then an attempt is made to find that user authentication info.  If no user or alias is passed an attempt is made to find the default user based on sfdx config.


```
force usedxauth [dx-username or alias] [flags]
```

### Examples

```

  force usedxauth test-d1df0gyckgpr@dcarroll_company.net
  force usedxauth ScratchUserAlias
  force usedxauth

```

### Options

```
  -h, --help   help for usedxauth
```

### Options inherited from parent commands

```
  -a, --account username    account username to use
  -V, --apiversion string   API version to use
      --config string       config directory to use (default: .force)
```

### SEE ALSO

* [force](force.md)	 - force CLI


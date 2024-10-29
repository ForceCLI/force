## force deploys

Manage metadata deployments

### Synopsis


List and cancel metadata deployments.


### Examples

```

  force deploys list
  force deploys cancel --all
  force deploys cancel -d 0Af000000000000000

```

### Options

```
  -h, --help   help for deploys
```

### Options inherited from parent commands

```
  -a, --account username    account username to use
  -V, --apiversion string   API version to use
      --config string       config directory to use (default: .force)
```

### SEE ALSO

* [force](force.md)	 - force CLI
* [force deploys cancel](force_deploys_cancel.md)	 - Cancel deploy
* [force deploys errors](force_deploys_errors.md)	 - List metadata deploy errors
* [force deploys list](force_deploys_list.md)	 - List metadata deploys
* [force deploys watch](force_deploys_watch.md)	 - Monitor metadata deploy


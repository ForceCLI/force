## force sobject

Manage standard & custom objects

### Synopsis


Manage sobjects

Usage:

  force sobject list
  force sobject create <object> [<field>:<type> [<option>:<value>]]
  force sobject delete <object>
  force sobject import


### Examples

```

  force sobject list
  force sobject create Todo Description:string
  force sobject delete Todo

```

### Options

```
  -h, --help   help for sobject
```

### Options inherited from parent commands

```
  -a, --account username    account username to use
  -V, --apiversion string   API version to use
      --config string       config directory to use (default: .force)
```

### SEE ALSO

* [force](force.md)	 - force CLI
* [force sobject create](force_sobject_create.md)	 - Create custom object
* [force sobject delete](force_sobject_delete.md)	 - Delete custom object
* [force sobject import](force_sobject_import.md)	 - Import custom object
* [force sobject list](force_sobject_list.md)	 - List standard and custom objects


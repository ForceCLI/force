## force field create

Create SObject fields

```
force field create <object> <field>:<type> [<option>:<value>]
```

### Examples

```

  force field create Inspection__c "Final Outcome":picklist picklist:"Pass, Fail, Redo"
  force field create Todo__c Due:DateTime required:true

```

### Options

```
  -h, --help   help for create
```

### Options inherited from parent commands

```
  -a, --account username    account username to use
  -V, --apiversion string   API version to use
      --config string       config directory to use (default: .force)
```

### SEE ALSO

* [force field](force_field.md)	 - Manage SObject fields


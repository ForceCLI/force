## force field

Manage SObject fields

### Synopsis


Manage SObject fields

Usage:

  force field list <object>
  force field create <object> <field>:<type> [<option>:<value>]
  force field delete <object> <field>
  force field type
  force field type <fieldtype>
  

### Examples

```

  force field list Todo__c
  force field create Inspection__c "Final Outcome":picklist picklist:"Pass, Fail, Redo"
  force field create Todo__c Due:DateTime required:true
  force field delete Todo__c Due
  force field type     # displays all the supported field types
  force field type email   # displays the required and optional attributes

```

### Options

```
  -h, --help   help for field
```

### Options inherited from parent commands

```
  -a, --account username    account username to use
  -V, --apiversion string   API version to use
      --config string       config directory to use (default: .force)
```

### SEE ALSO

* [force](force.md)	 - force CLI
* [force field create](force_field_create.md)	 - Create SObject fields
* [force field delete](force_field_delete.md)	 - Delete SObject field
* [force field list](force_field_list.md)	 - List SObject fields
* [force field type](force_field_type.md)	 - Display SObject field type details


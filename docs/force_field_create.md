## force field create

Create SObject fields

### Synopsis

Create SObject fields with various types and options.

Supported field options include:
  required:true/false    - Set field as required
  unique:true/false      - Set field as unique
  externalId:true/false  - Set field as external ID
  helpText:"text"        - Add inline help text for the field
  defaultValue:"value"   - Set default value
  picklist:"val1,val2"   - Define picklist values
  length:number          - Set text field length
  precision:number       - Set number precision
  scale:number           - Set number scale

```
force field create <object> <field>:<type> [<option>:<value>]
```

### Examples

```

  force field create Inspection__c "Final Outcome":picklist picklist:"Pass, Fail, Redo"
  force field create Todo__c Due:DateTime required:true
  force field create Account TestAuto:autoNumber helpText:"This field auto-generates unique numbers"
  force field create Contact Phone:phone helpText:"Primary contact phone number"

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


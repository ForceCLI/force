## force bigobject

Manage big objects

### Synopsis


Manage big objects

Usage:

  force bigobject list

  force bigobject create -n=<name> [-f=<field> ...]
  		A field is defined as a "+" separated list of attributes
  		Attributes depend on the type of the field.

  		Type = text: name, label, length
  		Type = datetime: name, label
  		Type = lookup: name, label, referenceTo, relationshipName


### Examples

```

  force bigobject list

  force bigobject create -n=MyObject -l="My Object" -p="My Objects" \
  -f=name:Field1+label:"Field 1"+type:Text+length:120 \
  -f=name:MyDate+type=dateTime


```

### Options

```
  -h, --help   help for bigobject
```

### Options inherited from parent commands

```
  -a, --account username    account username to use
  -V, --apiversion string   API version to use
```

### SEE ALSO

* [force](force.md)	 - force CLI
* [force bigobject create](force_bigobject_create.md)	 - Create big object
* [force bigobject list](force_bigobject_list.md)	 - List big objects


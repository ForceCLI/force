## force bigobject create

Create big object

```
force bigobject create [flags]
```

### Examples

```

  force bigobject create -n=MyObject -l="My Object" -p="My Objects" \
  -f=name:Field1+label:"Field 1"+type:Text+length:120 \
  -f=name:MyDate+type=dateTime

```

### Options

```
  -f, --field strings   field definition
  -h, --help            help for create
  -l, --label string    big object label
  -p, --plural string   big object plural label
```

### Options inherited from parent commands

```
  -a, --account username   account username to use
```

### SEE ALSO

* [force bigobject](force_bigobject.md)	 - Manage big objects


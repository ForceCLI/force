## force import

Import metadata from a local directory

```
force import [flags]
```

### Examples

```

  force import
  force import -directory=my_metadata -c -r -v
  force import -checkonly -runalltests

```

### Options

```
  -m, --allowmissingfiles   set allow missing files
  -u, --autoupdatepackage   set auto update package
  -c, --checkonly           set check only
  -d, --directory string    relative path to package.xml (default "metadata")
  -h, --help                help for import
  -i, --ignorewarnings      set ignore warnings
  -p, --purgeondelete       set purge on delete
  -r, --rollbackonerror     set roll back on error
  -t, --runalltests         set run all tests
      --test strings        Test(s) to run
  -l, --testLevel string    set test level (default "NoTestRun")
  -v, --verbose             give more verbose output
```

### Options inherited from parent commands

```
  -a, --account username   account username to use
```

### SEE ALSO

* [force](force.md)	 - force CLI


## force push

Deploy metadata from a local directory

### Synopsis


Deploy artifact from a local directory
<metadata>: Accepts either actual directory name or Metadata type
File path can be specified as - to read from stdin; see examples


```
force push [flags]
```

### Examples

```

  force push -t StaticResource -n MyResource
  force push -t ApexClass
  force push -f metadata/classes/MyClass.cls
  force push -checkonly -test MyClass_Test metadata/classes/MyClass.cls
  force push -n MyApex -n MyObject__c
  git diff HEAD^ --name-only --diff-filter=ACM | force push -f -

```

### Options

```
  -m, --allowmissingfiles    set allow missing files
  -u, --autoupdatepackage    set auto update package
  -c, --checkonly            check only deploy
  -f, --filepath strings     Path to resource(s)
  -h, --help                 help for push
  -w, --ignorecoverage       suppress code coverage warnings
  -i, --ignorewarnings       ignore warnings
  -n, --name strings         name of metadata object
  -p, --purgeondelete        purge metadata from org on delete
  -q, --quiet                only output failures
      --reporttype string    report type format (text or junit) (default "text")
  -r, --rollbackonerror      roll back deployment on error
      --runalltests          run all tests (equivalent to --testlevel RunAllTestsInOrg)
  -U, --suppressunexpected   suppress "An unexpected error occurred" messages
      --test strings         Test(s) to run
  -l, --testlevel string     test level (default "NoTestRun")
  -t, --type strings         Metatdata type
  -v, --verbose count        give more verbose output
```

### Options inherited from parent commands

```
  -a, --account username    account username to use
  -V, --apiversion string   API version to use
```

### SEE ALSO

* [force](force.md)	 - force CLI


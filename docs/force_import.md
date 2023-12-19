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
  -m, --allowmissingfiles    set allow missing files
  -u, --autoupdatepackage    set auto update package
  -c, --checkonly            check only deploy
  -d, --directory string     relative path to package.xml (default "src")
  -E, --erroronfailure       exit with an error code if any tests fail (default true)
  -h, --help                 help for import
  -w, --ignorecoverage       suppress code coverage warnings
  -i, --ignorewarnings       ignore warnings
  -I, --interactive          interactive mode
  -p, --purgeondelete        purge metadata from org on delete
  -q, --quiet                only output failures
  -f, --reporttype string    report type format (text or junit) (default "text")
  -r, --rollbackonerror      roll back deployment on error
  -t, --runalltests          run all tests (equivalent to --testlevel RunAllTestsInOrg)
  -U, --suppressunexpected   suppress "An unexpected error occurred" messages (default true)
      --test strings         Test(s) to run
  -l, --testlevel string     test level (default "NoTestRun")
  -v, --verbose count        give more verbose output
```

### Options inherited from parent commands

```
  -a, --account username    account username to use
  -V, --apiversion string   API version to use
```

### SEE ALSO

* [force](force.md)	 - force CLI


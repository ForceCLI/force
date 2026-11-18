## force import

Import metadata from a local directory

### Synopsis

Deploy the metadata in a directory that holds a package.xml.

The directory may be in metadata format or in source (sfdx) format. Source
format files are converted before they are deployed: the field, record type
and other component files under objects/<Object>/ are composed into the
object's file, a folder file such as reports/Sales.reportFolder-meta.xml is
deployed as reports/Sales-meta.xml, and a -meta.xml file of a type with no
content file (tabs/Project__c.tab-meta.xml) is deployed without the suffix.
The directory's package.xml is deployed as it is.

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
      --smart-flow-version   enable smart flow versioning (auto-select new version and prune inactive flows)
  -U, --suppressunexpected   suppress "An unexpected error occurred" messages (default true)
      --test strings         Test(s) to run
  -l, --testlevel string     test level (default "NoTestRun")
  -v, --verbose count        give more verbose output
```

### Options inherited from parent commands

```
  -a, --account username    account username to use
  -V, --apiversion string   API version to use
      --config string       config directory to use (default: .force)
```

### SEE ALSO

* [force](force.md)	 - force CLI


## force test

Run apex tests

### Synopsis


Run apex tests

Examples:

  force test all
  force test Test1 Test2 Test3
  force test Test1.method1 Test1.method2
  force test -namespace=ns Test4
  force test -class=Test1 method1 method2
  force test -v Test1


```
force test (all | classname... | classname.method...) [flags]
```

### Options

```
  -c, --class string       class to run tests from
  -h, --help               help for test
  -n, --namespace string   namespace to run tests in
  -v, --verbose            set verbose logging
```

### Options inherited from parent commands

```
  -a, --account username   account username to use
```

### SEE ALSO

* [force](force.md)	 - force CLI


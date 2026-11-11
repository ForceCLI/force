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
  force test --integration MyIntegrationTest
  force test --debug Test1.method1
  force test --debug --checkpoint MyClass:42 Test1.method1

With --debug, the tests run while your DEVELOPER_LOG trace flag uses the
Force_CLI_Replay debug level, which logs APEX_CODE at FINEST. After the
results are shown, the debug log of the run opens in the replay debugger
described in "force log debug". All of the tests in the run share one log.
The trace flag is restored when the tests finish.


```
force test (all | classname... | classname.method...) [flags]
```

### Options

```
      --checkpoint stringArray   with --debug, save a heap dump when execution first reaches a line, given as Class:line; repeat for up to 5 lines
  -c, --class string             class to run tests from
      --debug                    replay the test run in the debugger after showing the results (see force log debug)
  -h, --help                     help for test
      --integration              run an @IntegrationTest class asynchronously via the Tooling API
  -n, --namespace string         namespace to run tests in
  -f, --reporttype string        report type format (text or junit) (default "text")
  -v, --verbose                  set verbose logging
      --web                      with --debug, open the debugger in a browser instead of the terminal
```

### Options inherited from parent commands

```
  -a, --account username    account username to use
  -V, --apiversion string   API version to use
      --config string       config directory to use (default: .force)
```

### SEE ALSO

* [force](force.md)	 - force CLI


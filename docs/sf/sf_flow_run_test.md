## sf flow run test

Invoke flow tests in an org.

```
sf flow run test [flags]
```

### Options

```
      --api-version string        Override the api version used for api requests made by this command
  -n, --class-names stringArray   Flow test class names to run; default is all classes.
  -c, --code-coverage             Retrieve code coverage results.
      --concise                   Display only failed test results; works with human-readable output only.
  -h, --help                      help for test
      --json                      Format output as json.
  -d, --output-dir string         Directory in which to store test result files.
  -r, --result-format string      Format of the test results. (default "human")
  -s, --suite-names stringArray   Flow test suite names to run.
  -y, --synchronous               Runs test methods from a single Flow synchronously.
  -o, --target-org string         Username or alias of the target org.
  -l, --test-level string         Level of tests to run; default is RunLocalTests.
  -t, --tests stringArray         Flow test class names or IDs and, if applicable, test methods to run; default is all tests.
```

### SEE ALSO

* [sf flow run](sf_flow_run.md)	 - Run Flow tests.


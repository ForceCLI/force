## sf apex run test

Invoke Apex tests in an org.

```
sf apex run test [flags]
```

### Options

```
      --api-version string        Override the api version used for api requests made by this command
  -n, --class-names stringArray   Apex test class names to run.
  -c, --code-coverage             Retrieve code coverage results.
      --concise                   Display only failed test results; works with human-readable output only.
  -v, --detailed-coverage         Display detailed code coverage per test.
  -h, --help                      help for test
      --json                      Format output as json.
  -d, --output-dir string         Directory in which to store test run files.
  -i, --poll-interval int         Number of seconds to wait between retries.
  -r, --result-format string      Format of the test results. (default "human")
  -s, --suite-names stringArray   Apex test suite names to run.
  -y, --synchronous               Runs test methods from a single Apex class synchronously.
  -o, --target-org string         Username or alias of the target org.
  -l, --test-level string         Level of tests to run; default is RunLocalTests.
  -t, --tests stringArray         Apex test class names or IDs and, if applicable, test methods to run.
  -w, --wait int                  Sets the streaming client socket timeout in minutes.
```

### SEE ALSO

* [sf apex run](sf_apex_run.md)	 - Execute anonymous Apex code entered on the command line or from a local file.


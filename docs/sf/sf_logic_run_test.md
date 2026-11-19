## sf logic run test

Invoke tests for Apex and Flows in an org.

```
sf logic run test [flags]
```

### Options

```
      --api-version string          Override the api version used for api requests made by this command
  -n, --class-names stringArray     Apex test class names to run.
  -c, --code-coverage               Retrieve code coverage results.
      --concise                     Display only failed test results; works with human-readable output only.
  -v, --detailed-coverage           Display detailed code coverage per test.
  -h, --help                        help for test
      --json                        Format output as json.
  -d, --output-dir string           Directory in which to store test run files.
  -r, --result-format string        Format of the test results. (default "human")
  -s, --suite-names stringArray     Apex test suite names to run.
  -y, --synchronous                 Runs test methods from a single Apex class synchronously.
  -o, --target-org string           Username or alias of the target org.
      --test-category stringArray   Category of tests to run, such as Apex or Flow.
  -l, --test-level string           Level of tests to run; default is RunLocalTests.
  -t, --tests stringArray           Comma-separated list of test names to run. Can include Apex test classes and Flow tests.
  -w, --wait int                    Sets the streaming client socket timeout in minutes.
```

### SEE ALSO

* [sf logic run](sf_logic_run.md)	 - Run tests.


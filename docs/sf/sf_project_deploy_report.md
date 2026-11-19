## sf project deploy report

Check or poll for the status of a deploy operation.

```
sf project deploy report [flags]
```

### Options

```
      --coverage-formatters stringArray   Format of the code coverage results.
  -h, --help                              help for report
  -i, --job-id string                     Job ID of the deploy operation.
      --json                              Format output as json.
      --junit                             Output JUnit test results.
      --results-dir string                Output directory for code coverage and JUnit results; defaults to the deploy ID.
  -o, --target-org string                 Username or alias of the target org.
  -r, --use-most-recent                   Use the job ID of the most recent deploy operation.
  -w, --wait int                          Number of minutes to wait for the command to complete and display results. (default 33)
```

### SEE ALSO

* [sf project deploy](sf_project_deploy.md)	 - Deploy metadata to an org from your local project.


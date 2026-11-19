## sf project deploy start

Deploy metadata to an org from your local project.

```
sf project deploy start [flags]
```

### Options

```
  -a, --api-version string                Target API version for the deploy.
      --async                             Run the command asynchronously.
      --concise                           Show concise output of the deploy result.
      --coverage-formatters stringArray   Format of the code coverage results.
      --dry-run                           Validate deploy and run Apex tests but don’t save to the org.
  -h, --help                              help for start
  -c, --ignore-conflicts                  Ignore conflicts and deploy local files, even if they overwrite changes in the org.
  -r, --ignore-errors                     Ignore any errors and don’t roll back deployment.
  -g, --ignore-warnings                   Ignore warnings and allow a deployment to complete successfully.
      --json                              Format output as json.
      --junit                             Output JUnit test results.
  -x, --manifest string                   Full file path for manifest (package.xml) of components to deploy.
  -m, --metadata stringArray              Metadata component names to deploy.
      --metadata-dir string               Root of directory or zip file of metadata formatted files to deploy.
      --post-destructive-changes string   File path for a manifest of components to delete after the deploy.
      --pre-destructive-changes string    File path for a manifest of components to delete before the deploy.
      --purge-on-delete                   Specify that deleted components in the destructive changes manifest file are immediately eligible for deletion.
      --results-dir string                Output directory for code coverage and JUnit results; defaults to the deploy ID.
      --single-package                    Indicates that the metadata zip file points to a directory structure for a single package.
  -d, --source-dir stringArray            Path to the local source files to deploy.
  -o, --target-org string                 Username or alias of the target org.
  -l, --test-level string                 Deployment Apex testing level.
  -t, --tests stringArray                 Apex tests to run when --test-level is RunSpecifiedTests.
      --verbose                           Show verbose output of the deploy result.
  -w, --wait int                          Number of minutes to wait for command to complete and display results. (default 33)
```

### SEE ALSO

* [sf project deploy](sf_project_deploy.md)	 - Deploy metadata to an org from your local project.


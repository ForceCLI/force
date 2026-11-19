## sf project delete source

Delete source from your project and from a non-source-tracked org.

```
sf project delete source [flags]
```

### Options

```
      --api-version string       Override the api version used for api requests made by this command
  -c, --check-only               Validate delete command but don't delete anything from the org or the local project.
  -f, --force-overwrite          Ignore conflict warnings and overwrite changes to the org.
  -h, --help                     help for source
      --json                     Format output as json.
  -m, --metadata stringArray     Metadata components to delete.
  -r, --no-prompt                Don't prompt for delete confirmation.
  -p, --source-dir stringArray   Source file paths to delete.
  -o, --target-org string        Username or alias of the target org.
  -l, --test-level string        Deployment Apex testing level.
      --tests stringArray        Apex tests to run when --test-level is RunSpecifiedTests.
  -t, --track-source             If the delete succeeds, update the source tracking information.
      --verbose                  Verbose output of the delete result.
  -w, --wait int                 Number of minutes to wait for the command to finish. (default 33)
```

### SEE ALSO

* [sf project delete](sf_project_delete.md)	 - Delete source or source tracking information.


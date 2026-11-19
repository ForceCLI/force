## sf project deploy quick

Quickly deploy a validated deployment to an org.

```
sf project deploy quick [flags]
```

### Options

```
  -a, --api-version string   Target API version for the deploy.
      --async                Run the command asynchronously.
      --concise              Show concise output of the deploy result.
  -h, --help                 help for quick
  -i, --job-id string        Job ID of the deploy operation.
      --json                 Format output as json.
  -o, --target-org string    Username or alias of the target org.
  -r, --use-most-recent      Use the job ID of the most recent deploy operation.
      --verbose              Show verbose output of the deploy result.
  -w, --wait int             Number of minutes to wait for the command to complete and display results. (default 33)
```

### SEE ALSO

* [sf project deploy](sf_project_deploy.md)	 - Deploy metadata to an org from your local project.


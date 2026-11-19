## sf project deploy preview

Preview a deployment to see what will deploy to the org, the potential conflicts, and the ignored files.

```
sf project deploy preview [flags]
```

### Options

```
      --concise                  Show only the changes that will be deployed; omits files that are forceignored.
  -h, --help                     help for preview
  -c, --ignore-conflicts         Don't display conflicts in preview of the deployment.
      --json                     Format output as json.
  -x, --manifest string          Full file path for manifest (package.xml) of components to preview.
  -m, --metadata stringArray     Metadata component names to preview.
  -d, --source-dir stringArray   Path to the local source files to preview.
  -o, --target-org string        Username or alias of the target org.
```

### SEE ALSO

* [sf project deploy](sf_project_deploy.md)	 - Deploy metadata to an org from your local project.


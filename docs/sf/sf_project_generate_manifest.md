## sf project generate manifest

Create a project manifest that lists the metadata components you want to deploy or retrieve.

```
sf project generate manifest [flags]
```

### Options

```
      --api-version string              Override the api version used for api requests made by this command
      --excluded-metadata stringArray   Metadata types to exclude when building a manifest from an org.
      --from-org string                 Username or alias of the org that contains the metadata components from which to build a manifest.
  -h, --help                            help for manifest
  -c, --include-packages stringArray    Package types (managed, unlocked) whose metadata is included in the manifest; by default, metadata in managed and unlocked packages is excluded.
      --json                            Format output as json.
  -m, --metadata stringArray            Names of metadata components to include in the manifest.
  -n, --name string                     Name of a custom manifest file to create.
  -d, --output-dir string               Directory to save the created manifest.
  -p, --source-dir stringArray          Paths to the local source files to include in the manifest.
  -t, --type string                     Type of manifest to create; the type determines the name of the created file.
```

### SEE ALSO

* [sf project generate](sf_project_generate.md)	 - Generate a manifest.


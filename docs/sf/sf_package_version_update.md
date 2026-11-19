## sf package version update

Update a package version.

```
sf package version update [flags]
```

### Options

```
      --api-version string           Override the api version used for api requests made by this command
  -b, --branch string                New package version branch.
  -h, --help                         help for update
  -k, --installation-key string      New installation key for key-protected package (default: null)
      --json                         Format output as json.
  -p, --package string               ID (starts with 04t) or alias of the package version to update.
  -t, --tag string                   New package version tag.
  -v, --target-dev-hub string        Username or alias of the Dev Hub org.
  -e, --version-description string   New package version description.
  -a, --version-name string          New package version name.
```

### SEE ALSO

* [sf package version](sf_package_version.md)	 - Commands to manage package versions.


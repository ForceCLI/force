## sf package version retrieve

Retrieve package metadata for a specified package version. Package metadata can be retrieved for only second-generation managed package versions or unlocked packages.

```
sf package version retrieve [flags]
```

### Options

```
      --api-version string      Override the api version used for api requests made by this command
  -h, --help                    help for retrieve
      --json                    Format output as json.
  -d, --output-dir string       Path within your Salesforce DX project directory in which to download the metadata. This directory must be empty. (default "force-app")
  -p, --package string          Subscriber package version ID (starts with 04t).
  -v, --target-dev-hub string   Username or alias of the Dev Hub org.
```

### SEE ALSO

* [sf package version](sf_package_version.md)	 - Commands to manage package versions.


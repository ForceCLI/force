## sf package version displaydependencies

Display the dependency graph for an unlocked or 2GP managed package version.

```
sf package version displaydependencies [flags]
```

### Options

```
      --api-version string      Override the api version used for api requests made by this command
      --edge-direction string   Order (root-first or root-last) in which the dependencies are displayed. (default "root-first")
  -h, --help                    help for displaydependencies
      --json                    Format output as json.
  -p, --package string          ID or alias of the package version (starts with 04t) or the package version create request (starts with 08c) to display the dependency graph for.
  -v, --target-dev-hub string   Username or alias of the Dev Hub org.
      --verbose                 Display both the package version ID (starts with 04t) and the version number (major.minor.patch.build) in each node.
```

### SEE ALSO

* [sf package version](sf_package_version.md)	 - Commands to manage package versions.


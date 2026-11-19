## sf package1 version create

Create a first-generation package version in the release org.

```
sf package1 version create [flags]
```

### Options

```
      --api-version string         Override the api version used for api requests made by this command
  -d, --description string         Package version description.
  -h, --help                       help for create
  -k, --installation-key string    Installation key for key-protected package (default: null).
      --json                       Format output as json.
  -m, --managed-released           Create a managed package version.
  -n, --name string                Package version name.
  -i, --package-id string          ID of the metadata package (starts with 033) of which you’re creating a new version.
  -p, --post-install-url string    Post install URL.
  -r, --release-notes-url string   Release notes URL.
  -o, --target-org string          Username or alias of the target org.
  -v, --version string             Package version in major.minor format, for example, 3.2.
  -w, --wait int                   Minutes to wait for the package version to be created (default: 2 minutes).
```

### SEE ALSO

* [sf package1 version](sf_package1_version.md)	 - Commands for first-generation package versions.
* [sf package1 version create get](sf_package1_version_create_get.md)	 - Retrieve the status of a package version creation request.


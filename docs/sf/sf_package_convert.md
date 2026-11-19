## sf package convert

Convert a managed-released first-generation managed package into a second-generation managed package.

```
sf package convert [flags]
```

### Options

```
      --api-version string        Override the api version used for api requests made by this command
  -c, --code-coverage             Calculate and store the code coverage percentage by running the packaged Apex tests included in this package version.
  -f, --definition-file string    Path to a definition file that contains features and org preferences that the metadata of the package version depends on.
  -h, --help                      help for convert
  -k, --installation-key string   Installation key for key-protected package.
  -x, --installation-key-bypass   Bypass the installation key requirement.
      --json                      Format output as json.
  -p, --package string            ID (starts with 033) of the first-generation managed package to convert.
  -a, --patch-version string      Specific released patch version to be converted.
  -m, --seed-metadata string      Directory containing metadata to be deployed prior to conversion.
  -v, --target-dev-hub string     Username or alias of the Dev Hub org.
      --verbose                   Display verbose command output.
  -w, --wait int                  Minutes to wait for the package version to be created.
```

### SEE ALSO

* [sf package](sf_package.md)	 - Commands to develop and install unlocked packages and managed 2GP packages.


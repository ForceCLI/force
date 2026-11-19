## sf package create

Create a package.

```
sf package create [flags]
```

### Options

```
      --api-version string                   Override the api version used for api requests made by this command
  -d, --description string                   Description of the package.
  -o, --error-notification-username string   Active Dev Hub user designated to receive email notifications for package errors.
  -h, --help                                 help for create
      --json                                 Format output as json.
  -n, --name string                          Name of the package to create.
  -e, --no-namespace                         Create the package with no namespace; available only for unlocked packages.
      --org-dependent                        Depends on unpackaged metadata in the installation org; applies to unlocked packages only.
  -t, --package-type string                  Type of package.
  -r, --path string                          Path to directory that contains the contents of the package.
  -v, --target-dev-hub string                Username or alias of the Dev Hub org.
```

### SEE ALSO

* [sf package](sf_package.md)	 - Commands to develop and install unlocked packages and managed 2GP packages.


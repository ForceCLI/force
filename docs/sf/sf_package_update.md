## sf package update

Update package details.

```
sf package update [flags]
```

### Options

```
      --api-version string                   Override the api version used for api requests made by this command
  -d, --description string                   New description of the package.
      --enable-app-analytics                 Enable AppExchange App Analytics usage data collection on this managed package and its components.
  -o, --error-notification-username string   Active Dev Hub user designated to receive email notifications for package errors.
  -h, --help                                 help for update
      --json                                 Format output as json.
  -n, --name string                          New name of the package.
  -p, --package string                       ID (starts with 0Ho) or alias of the package to update.
  -r, --recommended-version-id string        Package version ID to recommend to subscribers.
      --skip-ancestor-check                  Bypass the ancestry check for setting a recommended version.
  -v, --target-dev-hub string                Username or alias of the Dev Hub org.
```

### SEE ALSO

* [sf package](sf_package.md)	 - Commands to develop and install unlocked packages and managed 2GP packages.


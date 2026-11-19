## sf package version list

List all package versions in the Dev Hub org.

```
sf package version list [flags]
```

### Options

```
      --api-version string       Override the api version used for api requests made by this command
  -b, --branch string            Branch in source control repository on which the package version is based.
      --concise                  Display limited package version details.
  -c, --created-last-days int    Number of days since the request was created, starting at 00:00:00 of first day to now.
  -h, --help                     help for list
      --json                     Format output as json.
  -m, --modified-last-days int   Number of days since the items were modified, starting at 00:00:00 of first day to now.
  -o, --order-by string          Package version fields used to order the list.
  -p, --packages string          Comma-delimited list of packages (aliases or 0Ho IDs) to list.
  -r, --released                 Display released versions only (IsReleased=true).
      --show-conversions-only    Filter the list output to display only converted package version.
  -v, --target-dev-hub string    Username or alias of the Dev Hub org.
      --verbose                  Display extended package version details.
```

### SEE ALSO

* [sf package version](sf_package_version.md)	 - Commands to manage package versions.


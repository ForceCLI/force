## sf package push-upgrade schedule

Schedule a package push upgrade.

```
sf package push-upgrade schedule [flags]
```

### Options

```
      --api-version string      Override the api version used for api requests made by this command
  -h, --help                    help for schedule
      --json                    Format output as json.
      --migrate-to-2gp          Upgrade from a first-generation managed package (1GP) to a second-generation managed package (2GP).
  -f, --org-file string         Filename of the CSV file that contains the list of subscriber org IDs that need the package upgrade.
  -l, --org-list string         Comma-separated list of subscriber org IDs that need the package upgrade.
  -p, --package string          ID (starts with 04t) of the package version that the package is being upgraded to.
  -t, --start-time string       Date and time (UTC) when the push upgrade is processed.
  -v, --target-dev-hub string   Username or alias of the Dev Hub org.
```

### SEE ALSO

* [sf package push-upgrade](sf_package_push-upgrade.md)	 - Commands to manage package push upgrades.


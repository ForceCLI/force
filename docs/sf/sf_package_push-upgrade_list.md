## sf package push-upgrade list

Lists the status of push upgrade requests for a given package.

```
sf package push-upgrade list [flags]
```

### Options

```
      --api-version string          Override the api version used for api requests made by this command
  -h, --help                        help for list
      --json                        Format output as json.
  -p, --package string              Package ID (starts with 033) of the package that you want push upgrade information for.
  -l, --scheduled-last-days int     Number of days in the past for which to display the list of push upgrade requests that were scheduled.
      --show-push-migrations-only   Display only push upgrade requests for package migrations.
  -s, --status string               Status used to filter the list output Valid values are: Created, Canceled, Pending, In Progress, Failed, or Succeeded
  -v, --target-dev-hub string       Username or alias of the Dev Hub org.
```

### SEE ALSO

* [sf package push-upgrade](sf_package_push-upgrade.md)	 - Commands to manage package push upgrades.


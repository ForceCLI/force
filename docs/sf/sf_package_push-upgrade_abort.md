## sf package push-upgrade abort

Abort a package push upgrade that has been scheduled. Only push upgrade requests with a status of Created or Pending can be aborted.

```
sf package push-upgrade abort [flags]
```

### Options

```
      --api-version string       Override the api version used for api requests made by this command
  -h, --help                     help for abort
      --json                     Format output as json.
  -i, --push-request-id string   ID of the package push request (starts with 0DV).
  -v, --target-dev-hub string    Username or alias of the Dev Hub org.
```

### SEE ALSO

* [sf package push-upgrade](sf_package_push-upgrade.md)	 - Commands to manage package push upgrades.


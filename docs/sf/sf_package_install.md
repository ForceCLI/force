## sf package install

Install or upgrade a version of a package in the target org.

```
sf package install [flags]
```

### Options

```
  -a, --apex-compile string         Compile all Apex in the org and package, or only Apex in the package; unlocked packages only. (default "all")
      --api-version string          Override the api version used for api requests made by this command
  -h, --help                        help for install
  -k, --installation-key string     Installation key for key-protected package (default: null).
      --json                        Format output as json.
  -r, --no-prompt                   Don't prompt for confirmation.
  -p, --package string              ID (starts with 04t) or alias of the package version to install.
  -b, --publish-wait int            Maximum number of minutes to wait for the Subscriber Package Version ID to become available in the target org before canceling the install request.
  -s, --security-type string        Security access type for the installed package. (default "AdminsOnly")
  -l, --skip-handlers stringArray   Skip install handlers.
  -o, --target-org string           Username or alias of the target org.
  -t, --upgrade-type string         Upgrade type for the package installation; available only for unlocked packages. (default "Mixed")
  -w, --wait int                    Number of minutes to wait for installation status.
```

### SEE ALSO

* [sf package](sf_package.md)	 - Commands to develop and install unlocked packages and managed 2GP packages.
* [sf package install report](sf_package_install_report.md)	 - Retrieve the status of a package installation request.


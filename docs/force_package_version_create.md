## force package version create

Create a new package version

```
force package version create [path] [flags]
```

### Options

```
      --ancestor-id string           Ancestor version ID (optional)
  -y, --async-validation             Async validation
  -c, --code-coverage                Calculate code coverage (default true)
  -h, --help                         help for create
      --namespace string             Package namespace (alternative to --package-id)
  -i, --package-id string            Package ID (required if --namespace not provided)
  -s, --skip-validation              Skip validation
  -d, --version-description string   Version description (required)
  -m, --version-name string          Version name (required)
  -n, --version-number string        Version number (required, e.g., 1.0.0.0)
```

### Options inherited from parent commands

```
  -a, --account username    account username to use
  -V, --apiversion string   API version to use
      --config string       config directory to use (default: .force)
```

### SEE ALSO

* [force package version](force_package_version.md)	 - Manage package versions


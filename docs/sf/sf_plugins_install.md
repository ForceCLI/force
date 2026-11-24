## sf plugins install

Install a plugin from a container registry.

### Synopsis

Install a plugin from a container registry.

A plugin is named by a registry reference with an optional tag, such as
ghcr.io/acme/my-plugin:v1.2.0.  A name without a registry host, such as
sfdx-git-delta, is looked up under ghcr.io/forcecli/sf-plugins.
The tag defaults to latest.

```
sf plugins install plugin... [flags]
```

### Options

```
  -h, --help         help for install
      --json         Format output as json.
      --plain-http   Use HTTP instead of HTTPS for the registry.
```

### SEE ALSO

* [sf plugins](sf_plugins.md)	 - List installed plugins.


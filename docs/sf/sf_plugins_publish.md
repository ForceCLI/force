## sf plugins publish

Publish plugin executables to a container registry.

### Synopsis

Publish plugin executables to a container registry.

Give one --executable for each platform, in the form os/arch=path, with the
names Go uses for GOOS and GOARCH.  The plugin manifest is read from
--manifest, or from the executable for this platform when --manifest is not
given.  The registry credentials are those of the Docker configuration,
which "docker login" writes.  sf has no such command.

```
sf plugins publish name:tag [flags]
```

### Options

```
      --executable stringArray   An executable to publish, as os/arch=path.  Repeat for each platform.
  -h, --help                     help for publish
      --json                     Format output as json.
      --manifest string          Path of the plugin manifest JSON file.
      --overwrite                Move the tag when it already exists.
      --plain-http               Use HTTP instead of HTTPS for the registry.
      --revision string          Source control revision the executables were built from.
      --source string            URL of the source repository.
```

### SEE ALSO

* [sf plugins](sf_plugins.md)	 - List installed plugins.


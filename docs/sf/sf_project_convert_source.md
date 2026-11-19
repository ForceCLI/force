## sf project convert source

Convert source-formatted files into metadata that you can deploy using Metadata API.

```
sf project convert source [flags]
```

### Options

```
      --api-version string       Override the api version used for api requests made by this command
  -h, --help                     help for source
      --json                     Format output as json.
  -x, --manifest string          Path to the manifest (package.xml) file that specifies the metadata types to convert.
  -m, --metadata stringArray     Metadata component names to convert.
  -d, --output-dir string        Output directory to store the Metadata API–formatted files in.
  -n, --package-name string      Name of the package to associate with the metadata-formatted files.
  -r, --root-dir string          Source directory other than the default package to convert.
  -p, --source-dir stringArray   Paths to the local source files to convert.
```

### SEE ALSO

* [sf project convert](sf_project_convert.md)	 - Convert between source format and metadata format.


## sf project convert mdapi

Convert metadata retrieved via Metadata API into the source format used in Salesforce DX projects.

```
sf project convert mdapi [flags]
```

### Options

```
      --api-version string         Override the api version used for api requests made by this command
  -h, --help                       help for mdapi
      --json                       Format output as json.
  -x, --manifest string            File path to manifest (package.xml) of metadata types to convert.
  -m, --metadata stringArray       Metadata component names to convert.
  -p, --metadata-dir stringArray   Root of directory or zip file of metadata formatted files to convert.
  -d, --output-dir string          Directory to store your files in after they're converted to source format.
  -r, --root-dir string            Root directory that contains the Metadata API–formatted metadata.
```

### SEE ALSO

* [sf project convert](sf_project_convert.md)	 - Convert between source format and metadata format.


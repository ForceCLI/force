## sf template generate project

Generate a Salesforce DX project.

```
sf template generate project [flags]
```

### Options

```
      --api-version string           Will set this version as sourceApiVersion in the sfdx-project.json file
  -p, --default-package-dir string   Default package directory name. (default "force-app")
  -h, --help                         help for project
      --json                         Format output as json.
      --lwc-language string          Default language for Lightning Web Components.
  -x, --manifest                     Generate a manifest (package.xml) for change-set based development.
  -n, --name string                  Name of the generated project.
  -s, --namespace string             Namespace associated with this project and any connected scratch orgs.
  -d, --output-dir string            Directory for saving the created files. (default ".")
  -t, --template string              Template to use for project creation. (default "standard")
```

### SEE ALSO

* [sf template generate](sf_template_generate.md)	 - Create metadata from templates.


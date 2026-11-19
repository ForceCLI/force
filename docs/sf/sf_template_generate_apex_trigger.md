## sf template generate apex trigger

Generate an Apex trigger.

```
sf template generate apex trigger [flags]
```

### Options

```
      --api-version string   Override the api version used for api requests made by this command
  -e, --event stringArray    Events that fire the trigger.
  -h, --help                 help for trigger
      --json                 Format output as json.
  -n, --name string          Name of the generated metadata.
  -d, --output-dir string    Directory for saving the created files. (default ".")
  -s, --sobject string       Salesforce object to generate a trigger on. (default "SOBJECT")
  -t, --template string      Template to use for file creation. (default "ApexTrigger")
```

### SEE ALSO

* [sf template generate apex](sf_template_generate_apex.md)	 - Create Apex classes and triggers.


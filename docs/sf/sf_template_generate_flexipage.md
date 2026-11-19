## sf template generate flexipage

Generate a FlexiPage, also known as a Lightning page.

```
sf template generate flexipage [flags]
```

### Options

```
      --api-version string             Override the api version used for api requests made by this command
      --description string             Description for the FlexiPage, which provides context about its purpose.
      --detail-fields stringArray      Fields to display in the Details tab.
  -h, --help                           help for flexipage
      --json                           Format output as json.
      --label string                   Label of this FlexiPage; if not specified, uses the FlexiPage name as the label.
  -n, --name string                    Name of the FlexiPage.
  -d, --output-dir string              Directory for saving the created files. (default ".")
      --primary-field string           Primary field for the dynamic highlights header; typically 'Name'. Used only with RecordPage.
      --secondary-fields stringArray   Secondary fields shown in the dynamic highlights header.
  -s, --sobject string                 API name of the Salesforce object; required when creating a RecordPage.
  -t, --template string                Template type for the FlexiPage.
```

### SEE ALSO

* [sf template generate](sf_template_generate.md)	 - Create metadata from templates.


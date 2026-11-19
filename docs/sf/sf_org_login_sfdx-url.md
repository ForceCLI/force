## sf org login sfdx-url

Authorize an org using a Salesforce DX authorization URL stored in a file or through standard input (stdin).

```
sf org login sfdx-url [flags]
```

### Options

```
  -a, --alias string                  Alias for the org.
  -h, --help                          help for sfdx-url
  -r, --instance-url string           URL of the instance that the org lives on.
      --json                          Format output as json.
  -s, --set-default                   Set the authenticated org as the default that all org-related commands run against.
  -d, --set-default-dev-hub           Set the authenticated org as the default Dev Hub.
  -f, --sfdx-url-file string          Path to a file that contains the Salesforce DX authorization URL.
  -u, --sfdx-url-stdin string[="-"]   Pipe the Salesforce DX authorization URL through standard input (stdin).
```

### SEE ALSO

* [sf org login](sf_org_login.md)	 - Authorize an org for use with Salesforce CLI.


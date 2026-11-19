## sf org login jwt

Log in to a Salesforce org using a JSON web token (JWT).

```
sf org login jwt [flags]
```

### Options

```
  -a, --alias string          Alias for the org.
  -i, --client-id string      OAuth client ID (also called consumer key) of your custom connected app.
  -h, --help                  help for jwt
  -r, --instance-url string   URL of the instance that the org lives on.
      --json                  Format output as json.
  -f, --jwt-key-file string   Path to a file containing the private key.
  -s, --set-default           Set the authenticated org as the default that all org-related commands run against.
  -d, --set-default-dev-hub   Set the authenticated org as the default Dev Hub.
  -o, --username string       Username of the user logging in.
```

### SEE ALSO

* [sf org login](sf_org_login.md)	 - Authorize an org for use with Salesforce CLI.


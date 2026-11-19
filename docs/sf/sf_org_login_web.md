## sf org login web

Log in to a Salesforce org using the web server flow.

```
sf org login web [flags]
```

### Options

```
  -a, --alias string          Alias for the org.
  -b, --browser string        Browser in which to open the org.
  -c, --client-app string     Name to give to the link between the connected app or external client and the already-authenticated user.
  -i, --client-id string      OAuth client ID (also called consumer key) of your custom connected app.
  -h, --help                  help for web
  -r, --instance-url string   URL of the instance that the org lives on.
      --json                  Format output as json.
      --scopes string         Authentication (OAuth) scopes to request.
  -s, --set-default           Set the authenticated org as the default that all org-related commands run against.
  -d, --set-default-dev-hub   Set the authenticated org as the default Dev Hub.
      --username string       Username of the already-authenticated user to link to the connected app or external client app.
```

### SEE ALSO

* [sf org login](sf_org_login.md)	 - Authorize an org for use with Salesforce CLI.


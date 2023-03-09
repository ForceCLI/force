## force login

Log into Salesforce and store a session token

### Synopsis

Log into Salesforce and store a session token.  By default, OAuth is
used and a refresh token will be stored as well.  The refresh token is used
to get a new session token automatically when needed.

```
force login [flags]
```

### Examples

```

    force login
    force login -i test
    force login -i example--dev.sandbox.my.salesforce.com
    force login -u user@example.com -p password
    force login -i test -u user@example.com -p password
    force login -i my-domain.my.salesforce.com -u username -p password
    force login -i my-domain.my.salesforce.com -s[kipLogin]
    force login --connected-app-client-id <my-consumer-key> -u user@example.com -key jwt.key
    force login scratch

```

### Options

```
  -v, --api-version string               API version to use
      --connected-app-client-id string   Client Id (aka Consumer Key) to use instead of default
  -h, --help                             help for login
  -i, --instance string                  Defaults to 'login' or last
                                         logged in system. non-production server to login to (values are 'pre',
                                         'test', or full instance url
  -k, --key string                       JWT signing key filename
  -p, --password string                  password for SOAP login
  -s, --skip                             skip login if already authenticated and only save token (useful with SSO)
  -u, --user string                      username for SOAP login
```

### Options inherited from parent commands

```
  -a, --account username    account username to use
  -V, --apiversion string   API version to use
```

### SEE ALSO

* [force](force.md)	 - force CLI
* [force login scratch](force_login_scratch.md)	 - Create scratch org and log in


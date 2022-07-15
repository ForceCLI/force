## force login

force login [-i=<instance>] [<-u=username> <-p=password>] [-scratch] [-s]

```
force login [flags]
```

### Examples

```

    force login
    force login -i=test
    force login -u=un -p=pw
    force login -i=test -u=un -p=pw
    force login -i=na1-blitz01.soma.salesforce.com -u=un -p=pw -v 39.0
    force login -i my-domain.my.salesforce.com -u username -p password
    force login -i my-domain.my.salesforce.com -s[kipLogin]
    force login --connected-app-client-id <my-consumer-key> -u username -key jwt.key
    force login -scratch

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
      --scratch                          create new scratch org and log in
  -s, --skip                             skip login if already authenticated and only save token (useful with SSO)
  -u, --user string                      username for SOAP login
```

### Options inherited from parent commands

```
  -a, --account username   account username to use
```

### SEE ALSO

* [force](force.md)	 - force CLI


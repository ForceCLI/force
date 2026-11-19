## sf org generate password

Generate a random password for scratch org users.

```
sf org generate password [flags]
```

### Options

```
      --api-version string         Override the api version used for api requests made by this command
  -c, --complexity int             Level of password complexity or strength; the higher the value, the stronger the password. (default 5)
  -h, --help                       help for password
      --json                       Format output as json.
  -l, --length int                 Number of characters in the generated password; valid values are between 20 and 100. (default 20)
  -b, --on-behalf-of stringArray   Comma-separated list of usernames or aliases to assign the password to; must have been created locally with the "org create user" command.
  -o, --target-org string          Username or alias of the target org.
```

### SEE ALSO

* [sf org generate](sf_org_generate.md)	 - Generate a password.


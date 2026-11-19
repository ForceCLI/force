## sf org create scratch

Create a scratch org.

```
sf org create scratch [flags]
```

### Options

```
      --admin-email string       Email address that will be applied to the org's admin user.
  -a, --alias string             Alias for the scratch org.
      --api-version string       Override the api version used for api requests made by this command
      --async                    Request the org, but don't wait for it to complete.
  -i, --client-id string         Consumer key of the Dev Hub connected app.
  -f, --definition-file string   Path to a scratch org definition file.
      --description string       Description of the scratch org in the Dev Hub.
  -y, --duration-days int        Number of days before the org expires. (default 7)
  -e, --edition string           Salesforce edition of the scratch org. Overrides the value of the "edition" option in the definition file, if set.
  -h, --help                     help for scratch
      --json                     Format output as json.
      --name string              Name of the org, such as "Acme Company".
  -c, --no-ancestors             Don't include second-generation managed package (2GP) ancestors in the scratch org.
  -m, --no-namespace             Create the scratch org with no namespace, even if the Dev Hub has a namespace.
      --release string           Release of the scratch org as compared to the Dev Hub release.
  -d, --set-default              Set the scratch org as your default org
  -s, --snapshot string          Name of the snapshot to use when creating this scratch org.
      --source-org string        15-character ID of the org whose shape the new scratch org will be based on.
  -v, --target-dev-hub string    Username or alias of the Dev Hub org.
  -t, --track-source             Use source tracking for this scratch org. (default true)
      --username string          Username of the scratch org admin user.
  -w, --wait int                 Number of minutes to wait for the scratch org to be ready. (default 5)
```

### SEE ALSO

* [sf org create](sf_org_create.md)	 - Create scratch orgs, sandboxes, users, org shapes and snapshots.


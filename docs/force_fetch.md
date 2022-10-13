## force fetch

Export specified artifact(s) to a local directory

### Synopsis


Export specified artifact(s) to a local directory. Use "package" type to retrieve an unmanaged package.


```
force fetch -t ApexClass [flags]
```

### Examples

```

  force fetch -t=CustomObject -n=Book__c -n=Author__c
  force fetch -t Aura -n MyComponent -d /Users/me/Documents/Project/home
  force fetch -t AuraDefinitionBundle -t ApexClass
  force fetch -x myproj/metadata/package.xml

```

### Options

```
  -d, --directory string   Use to specify the root directory of your project
  -h, --help               help for fetch
  -n, --name strings       names of metadata
  -p, --preserve           keep zip file on disk
  -t, --type strings       Type of metadata to fetch
  -u, --unpack             Unpack any static resources
  -x, --xml string         Package.xml file to use for fetch.
```

### Options inherited from parent commands

```
  -a, --account username    account username to use
  -V, --apiversion string   API version to use
```

### SEE ALSO

* [force](force.md)	 - force CLI


## force aura

force aura push -f <filepath>

### Synopsis


	The aura command needs context to work. If you execute "aura get"
	it will create a folder structure that provides the context for
	aura components on disk.

	The aura components will be created in "metadata/aurabundles/<componentname>"
	relative to the current working directory and a .manifest file will be
	created that associates components and their artifacts with their ids in
	the database.

	To create a new component (application, evt or component), create a new
	folder under "aura". Then create a new file in your new folder. You
	must follow a naming convention for your files to enable proper definition
	of the component type.

	Naming convention <compnentName><artifact type>.<file type extension>
	Examples: 	metadata
					aura
						MyApp
							MyAppApplication.app
							MyAppStyle.css
						MyList
							MyComponent.cmp
							MyComponentHelper.js
							MyComponentStyle.css

	force aura push -f <fullFilePath>

	force aura create -t=<entity type> <entityName>

	force aura delete -f=<fullFilePath>

	force aura list

	

```
force aura [flags]
```

### Options

```
  -n, --entityname string   fully qualified file name for entity
  -f, --filepath strings    Path to resource(s)
  -h, --help                help for aura
  -t, --type string         Metatdata type
```

### Options inherited from parent commands

```
  -a, --account string   account to use
```

### SEE ALSO

* [force](force.md)	 - force CLI


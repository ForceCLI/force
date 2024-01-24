## force pubsub publish

Publish event to a pub/sub channel

### Synopsis

Publish an event to a pub/sub channel

```
force pubsub publish <channel> <values> [flags]
```

### Examples

```

	force pubsub publish /event/My_Event__e '{My_Field__c: "My Value", CreatedDate: 946706400}'
	
```

### Options

```
  -h, --help    help for publish
  -q, --quiet   disable status messages to stderr
```

### Options inherited from parent commands

```
  -a, --account username    account username to use
  -V, --apiversion string   API version to use
      --config string       config directory to use (default: .force)
```

### SEE ALSO

* [force pubsub](force_pubsub.md)	 - Subscribe to a pub/sub channel


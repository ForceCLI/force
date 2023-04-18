## force pubsub subscribe

Subscribe to a pub/sub channel

### Synopsis

Subscribe to a pub/sub channel to stream Change Data Capture or custom Platform Events

```
force pubsub subscribe [channel] [flags]
```

### Examples

```

	force pubsub subscribe /data/ChangeEvents | jq .
	force pubsub subscribe /data/AccountChangeEvent
	force pubsub subscribe /data/My_Object__ChangeEvent

	force pubsub subscribe /event/My_Event__e
	force pubsub subscribe /event/My_Channel__chn
	
```

### Options

```
  -c, --changes           show only changed fields (for Change Data Capture events)
  -e, --earliest          start at earliest events (default is latest)
  -h, --help              help for subscribe
  -q, --quiet             disable status messages to stderr
  -r, --replayid string   replay id to start after
```

### Options inherited from parent commands

```
  -a, --account username    account username to use
  -V, --apiversion string   API version to use
```

### SEE ALSO

* [force pubsub](force_pubsub.md)	 - Subscribe to a pub/sub channel


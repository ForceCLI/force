## force trace start

Set trace flag

### Synopsis

Set trace flag

Without a user id, the trace flag is for the current user. The trace flag
uses the Force_CLI debug level, which logs Apex at FINE. With --replay, it uses
the Force_CLI_Replay debug level, which logs Apex at FINEST, the level that
force log debug needs to step through a log.


```
force trace start [user id] [flags]
```

### Options

```
  -h, --help     help for start
      --replay   log Apex at FINEST, so that logs can be opened with force log debug
```

### Options inherited from parent commands

```
  -a, --account username    account username to use
  -V, --apiversion string   API version to use
      --config string       config directory to use (default: .force)
```

### SEE ALSO

* [force trace](force_trace.md)	 - Manage trace flags


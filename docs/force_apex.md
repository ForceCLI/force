## force apex

Execute anonymous Apex code

### Synopsis

Execute anonymous Apex code

With --debug, the code runs with APEX_CODE logging at FINEST and the debug log
returned by Salesforce opens in the replay debugger described in
"force log debug". While the code runs, your DEVELOPER_LOG trace flag uses the
Force_CLI_Replay debug level, because an active trace flag overrides the
levels requested for the returned log. The trace flag is restored afterwards.


```
force apex [file] [flags]
```

### Examples

```

  force apex ~/test.apex

  force apex
  >> Start typing Apex code; press CTRL-D(for Mac/Linux) / Ctrl-Z (for Windows) when finished

  force apex --debug ~/test.apex
  force apex --debug --checkpoint MyClass:42 ~/test.apex
  
```

### Options

```
      --checkpoint stringArray   with --debug, save a heap dump when execution first reaches a line, given as Class:line; repeat for up to 5 lines
      --debug                    replay the execution in the debugger (see force log debug)
  -h, --help                     help for apex
  -t, --test                     run in test context
      --web                      with --debug, open the debugger in a browser instead of the terminal
```

### Options inherited from parent commands

```
  -a, --account username    account username to use
  -V, --apiversion string   API version to use
      --config string       config directory to use (default: .force)
```

### SEE ALSO

* [force](force.md)	 - force CLI


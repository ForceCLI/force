## force log debug

Replay a debug log in a terminal debugger

### Synopsis

Replay a debug log in a terminal debugger.

The log must have been captured with APEX_CODE at FINEST; at lower levels
the log does not record each statement or variable assignment.
"force apex --debug" and "force test --debug" set your trace flag to this
level while they run, then open the log of the run in the debugger.

The source of each class and trigger is fetched from the org by the ID
recorded in the log. If a class has been changed since the log was
captured, the lines shown will not match the code that ran. The source of
anonymous Apex is read from the log.

Keys:
  n          step over
  s          step in
  o          step out
  c          continue to the next breakpoint
  b          set or remove a breakpoint on the line under the cursor
  f          open a class or trigger from the log
  r          restart from the beginning of the log
  tab        move between the source, call stack, and variables panes
  up/down    move within the focused pane
  enter      expand or collapse a variable
  q          quit


```
force log debug [logId] [flags]
```

### Examples

```

  force log debug 07L5g00000AbCdEFG
  force log debug --file apex.log

```

### Options

```
  -f, --file string   read the log from a file instead of fetching it from the org
  -h, --help          help for debug
```

### Options inherited from parent commands

```
  -a, --account username    account username to use
  -V, --apiversion string   API version to use
      --config string       config directory to use (default: .force)
```

### SEE ALSO

* [force log](force_log.md)	 - Fetch debug logs


## force record

Create, modify, or view records

### Synopsis


Create, modify, or view records

Usage:

  force record get <object> <id>
  force record get <object> <extid>:<value>
  force record create <object> [<fields>]
  force record update <object> <id> [<fields>]
  force record update <object> <extid>:<value> [<fields>]
  force record delete <object> <id>
  force record merge <object> <masterId> <duplicateId>
  force record undelete <id>


### Examples

```

  force record get User 00Ei0000000000
  force record get User username:user@name.org
  force record create User Name:"David Dollar" Phone:0000000000
  force record update User 00Ei0000000000 State:GA
  force record update User username:user@name.org State:GA
  force record delete User 00Ei0000000000
  force record merge Contact 0033c00002YDNNWAA5 0033c00002YDPqkAAH
  force record undelete 0033c00002YDNNWAA5

```

### Options

```
  -h, --help   help for record
```

### Options inherited from parent commands

```
  -a, --account username    account username to use
  -V, --apiversion string   API version to use
```

### SEE ALSO

* [force](force.md)	 - force CLI
* [force record create](force_record_create.md)	 - Create new record
* [force record delete](force_record_delete.md)	 - Delete record
* [force record get](force_record_get.md)	 - Get record details
* [force record merge](force_record_merge.md)	 - Merge records
* [force record undelete](force_record_undelete.md)	 - Undelete records
* [force record update](force_record_update.md)	 - Update record


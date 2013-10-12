## force

A command-line interface to force.com

### Installation

##### Precompiled Binaries

* [Linux 32bit](https://godist.herokuapp.com/projects/heroku/force/releases/current/linux-386/force)
* [Linux 64bit](https://godist.herokuapp.com/projects/heroku/force/releases/current/linux-amd64/force)
* [OS X 32bit](https://godist.herokuapp.com/projects/heroku/force/releases/current/darwin-386/force)
* [OS X 64bit](https://godist.herokuapp.com/projects/heroku/force/releases/current/darwin-amd64/force)
* [Windows 32bit](https://godist.herokuapp.com/projects/heroku/force/releases/current/windows-386/force.exe)
* [Windows 64bit](https://godist.herokuapp.com/projects/heroku/force/releases/current/windows-amd64/force.exe)

##### Compile from Source

    $ go get -u github.com/heroku/force

### Usage

	Usage: force <command> [<args>]
	
	Available commands:
	   login       Log in to force.com
	   logout      Log out from force.com
	   apiversion  Manage Force.com API versions
	   accounts    List force.com accounts
	   active      Show or set the active force.com account
	   whoami      Show information about the active account
	   sobject     Manage custom objects
	   field       Manage custom fields
	   record      Create, modify, or view records
	   select      Execute a SOQL select
	   apex        Execute anonymous Apex code
	   version     Display current version
	   update      Update to the latest version
	   help        Show this help
	
	Run 'force help [command]' for details.

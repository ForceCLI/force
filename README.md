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
	   login     Log in to force.com
	   logout    Log out from force.com
	   whoami    Show information about the active account
	   sobject   Manage sobjects
	   field     Manage sobject fields
	   record    Create, modify, or view records
	   export    Export metadata to a local directory
	   import    Import metadata from a local directory
	   query     Execute a SOQL query
	   apex      Execute anonymous Apex code
	   version   Display current version
	   update    Update to the latest version
	   help      Show this help
	
	Run 'force help [command]' for details.

### Hacking

    # set these environment variables in your startup scripts
    export GOPATH=~/go
    export PATH="$GOPATH/bin:$PATH"

    # download the source and all dependencies
    $ go get -u github.com/heroku/force
    $ cd $GOPATH/github.com/heroku/force

    # to compile and test modifications
    $ go get .
    $ force

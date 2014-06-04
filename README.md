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
   	   logins    List force.com logins used
   	   active    Show or set the active force.com account
	   whoami    Show information about the active account
	   sobject   Manage standard & custom objects
	   field     Manage sobject fields
	   record    Create, modify, or view records
	   bulk      Load csv file use Bulk API
	   fetch     Export specified artifact(s) to a local directory
   	   export    Export metadata to a local directory
	   fetch     Export specified artifact(s) to a local directory
	   import    Import metadata from a local directory
	   query     Execute a SOQL statement
	   apex      Execute anonymous Apex code
	   oauth     Manage ConnectedApp credentials
	   version   Display current version
	   update    Update to the latest version
	   push      Deploy single artifact from a local directory
	   password  See password status or reset password
	   help      Show this help
	
	Run 'force help [command]' for details.

### login
      force login     		 	# log in to production or developer org
      force login test 		 	# log in to sandbox org
      force login pre  		 	# log in to prerelease org
      force login un pw 	 	# log in using SOAP
      force login test un pw     	# log in using SOAP to sandbox org
      force login <instance> un pw 	# internal only

### Hacking

    # set these environment variables in your startup scripts
    export GOPATH=~/go
    export PATH="$GOPATH/bin:$PATH"

    # download the source and all dependencies
    $ go get -u github.com/heroku/force
    $ cd $GOPATH/src/github.com/heroku/force

    # to compile and test modifications
    $ go get .
    $ force

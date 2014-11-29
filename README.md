## force

A command-line interface to force.com

### Installation

##### Precompiled Binaries
If the download does not work, download instead from the [binaries](https://github.com/heroku/force/tree/master/binaries) folder in the repo.

* [Linux 32bit](https://godist-new.herokuapp.com/projects/devangel/force/releases/current/linux-386/force)
* [Linux 64bit](https://godist-new.herokuapp.com/projects/devangel/force/releases/current/linux-amd64/force)
* [Linux Arm](https://godist-new.herokuapp.com/projects/devangel/force/releases/current/linux-arm/force)
* [OS X 32bit](https://godist-new.herokuapp.com/projects/devangel/force/releases/current/darwin-386/force)
* [OS X 64bit](https://godist-new.herokuapp.com/projects/devangel/force/releases/current/darwin-amd64/force)
* [Windows 32bit](https://godist-new.herokuapp.com/projects/devangel/force/releases/current/windows-386/force.exe)
* [Windows 64bit](https://godist-new.herokuapp.com/projects/devangel/force/releases/current/windows-amd64/force.exe)

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
When you login using the CLI a record of the login is saved. Eventually your token will expire requiring re-authentication. The default login is for all production instances of salesforce.com. Two predefined non-production instances are available using the test and pre aliases.  You can set an arbitrary instance to log in to by specifying the instance url in the form of subdomain.domain. For example login-blitz.soma.salesforce.com.

      force login               # log in to production or developer org
      force login -i=test           # log in to sandbox org
      force login -i=pre            # log in to prerelease org
      force login -u=un -p=pw       # log in using SOAP
      force login -i=test -u=un -p=pw       # log in using SOAP to sandbox org
      force login -i=<instance> -u=un -p=pw     # internal only

### logout
Logout will delete your authentication token and remove the saved record of that login.

      force logout -u=user@example.org

### logins
Logins will list all the user names that you have used to authenticate with the instance URL associated with each one.  The active login will be indicated behind the login name in red.

      force logins

![](https://raw.githubusercontent.com/dcarroll/dcarroll.github.io/master/images/force/screenshot-191.png)

### active
Active without any arguments will display the currently acctive login that you are using. You can also supply a username argument that will set the active login to the one corresponding to the username argument. Note, just because you set a login as active, does not mean that the token is necessarily valid.

      force active
      force active dave@demo.1

### whoami
Whoami will display detailed user information about the currently active logged in user.  This is Force.com specific information.

      force whomai

![](https://raw.githubusercontent.com/dcarroll/dcarroll.github.io/master/images/force/screenshot-191%20copy.png)

### sobject
Sobject command gives you access to creating and deleting schema objects. The list argumenet will list ALL of the objects, both standard and custom, in your org.

      force sobject list
      force sobject create <object> [<field>:<type>]...
      force sobject delete <object>

![](https://raw.githubusercontent.com/dcarroll/dcarroll.github.io/master/images/force/screenshot-192.png)

### field
Field gives you the ability to create, list and delete the fields on an object. Fields need to be created one at a time. You can also set required and optional attributes for the type of field. All defaultable field attributes will be defaulted based on the defaults in the web UI.

      force field list Todo__c
      force field create Todo__c Due:DateTime required:true
      force field delete Todo__c Due

### push
Push gives you the ability to push specified resources to force.com.  The resource will be pulled from ./src/{type}/

      force -t(ype) StaticReource -n(ame) MyResource.resource
	  force -type ApexClass -name MyClass.cls
	  force -t ApexPage -n MyPage.page

You can also push all of a specific type of resource from a given folder.

      force -t StaticResource -p(ath) src/staticresources/
      force -t ApexClass -path src/classes/
      force -t ApexPage -p src/pages/

### notifications
Includes notification library, [gotifier](https://github.com/ViViDboarder/gotifier), that will display notifications for using either Using [terminal-notifier](https://github.com/alloy/terminal-notifier) on OSX or [notify-send](http://manpages.ubuntu.com/manpages/saucy/man1/notify-send.1.html) on Ubuntu. Currently, only the `push` and `test` methods are displaying notifications.

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

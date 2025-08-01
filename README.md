## force CLI v1.1.0

A command-line interface to force.com
![](https://travis-ci.org/ForceCLI/force.svg?branch=master)

### Installation

##### Precompiled Binaries

Can be downloaded from the [Current Release Page](https://github.com/ForceCLI/force/releases/latest) or by visiting the [CLI Home Page](http://force-cli.herokuapp.com/).

##### Compile from Source

    $ go install github.com/ForceCLI/force@latest

### Usage

See [docs/force.md](docs/force.md) for all supported commands.

Tab completion simplifies use of the force CLI.
Enable bash completion or see `force completion --help` for other options.

```
$ source <(force completion bash)
```

    Usage:
      force [command]

    Available Commands:
      active       Show or set the active force.com account
      apex         Execute anonymous Apex code
      apiversion   Display/Set current API version
      bigobject    Manage big objects
      bulk         Load csv file or query data using Bulk API
      completion   Generate the autocompletion script for the specified shell
      create       Creates a new, empty Apex Class, Trigger, Visualforce page, or Component.
      datapipe     Manage DataPipes
      describe     Describe the object or list of available objects
      eventlogfile List and fetch event log file
      export       Export metadata to a local directory
      fetch        Export specified artifact(s) to a local directory
      field        Manage SObject fields
      help         Help about any command
      import       Import metadata from a local directory
      limits       Display current limits
      log          Fetch debug logs
      login        force login [-i=<instance>] [<-u=username> <-p=password>] [-scratch] [-s]
      logins       List force.com logins used
      logout       Log out from Force.com
      notify       Should notifications be used
      oauth        Manage ConnectedApp credentials
      open         Open a browser window, logged into an authenticated Salesforce org
      package      Manage installed packages
      password     See password status or reset password
      push         Deploy metadata from a local directory
      query        Execute a SOQL statement
      quickdeploy  Quick deploy validation id
      record       Create, modify, or view records
      rest         Execute a REST request
      security     Displays the OLS and FLS for a given SObject
      sobject      Manage standard & custom objects
      test         Run apex tests
      trace        Manage trace flags
      usedxauth    Authenticate with SFDX Scratch Org User
      version      Display current version
      whoami       Show information about the active account

    Flags:
      -a, --account username   account username to use
      -h, --help               help for force

    Use "force [command] --help" for more information about a command.

### login
When you login using the CLI a record of the login is saved. Eventually your token will expire requiring re-authentication. The default login is for all production instances of salesforce.com. Two predefined non-production instances are available using the test and pre aliases.  You can set an arbitrary instance to log in to by specifying the instance url in the form of subdomain.domain. For example login-blitz.soma.salesforce.com.

      force login                           # log in to last environment
      force login -i=login                  # log in to production or developer org
      force login -i=test                   # log in to sandbox org
      force login -i=pre                    # log in to prerelease org
      force login -u=un [-p=pw]             # log in using SOAP. Password is optional
      force login -i=test -u=un -p=pw       # log in using SOAP to sandbox org. Password is optional
      force login -i=<instance> -u=un -p=pw # internal only

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
      force active -a dave@demo.1

### whoami
Whoami will display detailed user information about the currently active logged in user.  This is Force.com specific information.

      force whoami

![](https://raw.githubusercontent.com/dcarroll/dcarroll.github.io/master/images/force/screenshot-191%20copy.png)

### sobject
Sobject command gives you access to creating and deleting schema objects. The list argument will list ALL of the objects, both standard and custom, in your org.

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
Push gives you the ability to push specified resources to force.com.  The resource will be pulled from ./metatdata/{type}/

      force push -t(ype) StaticResource -n(ame) MyResource.resource
      force -type ApexClass -name MyClass.cls
      force -t ApexPage -n MyPage.page

You can also push all of a specific type of resource from a given folder.

      force push -t StaticResource -f metadata/staticresources/
      force push -t ApexClass -f metadata/classes/
      force push -t ApexPage -f metadata/pages/


### import
Import allows you to import code from local directory. This makes a lot of senses when you want to import code from local directory to a brand new org. This import method import codes from `metadata` folder not from your `src` folder


    Make sure you have the metadata folder, in case you only have src folder, simply replicate it

    force import

    Sample folder structure


    |-metadata
    |  |-aura
    |  |-corsWhitelistOrigins
    |  |-remoteSiteSettings
    |  |-staticresources
    |  |package.xml


### export
Export allows you to fetch all codes from your org to local machine. This command will export all the codes to a local folder called `src`

    force export

    |-src
    |  |-aura
    |  |-corsWhitelistOrigins
    |  |-remoteSiteSettings
    |  |-staticresources
    |  |package.xml


### notify
Includes notification library, [gotifier](https://github.com/ViViDboarder/gotifier), that will display notifications for using either Using [terminal-notifier](https://github.com/julienXX/terminal-notifier) on OSX or [notify-send](http://manpages.ubuntu.com/manpages/saucy/man1/notify-send.1.html) on Ubuntu. Currently, only the `push` and `test` methods are displaying notifications.


### limits
Limits will display limits information for your organization.
- Max is the limit total for the organization
- Remaining is the total number of calls or events left for the organization

The list is limited to those exposed by the REST API.

      force limits      

       
### apiversion
Set the API version to be used when interacting with the Salesforce org.
        
      force apiversion nn.0


### Hacking

    # set these environment variables in your startup scripts
    export GOPATH=~/go
    export PATH="$GOPATH/bin:$PATH"

    # download the source and all dependencies
    $ go get -u github.com/ForceCLI/force
    $ cd $GOPATH/src/github.com/ForceCLI/force

    # to compile and test modifications
    $ go get .
    $ force



### Windows Subsystem Linux (aka Bash on Windows)
Starting from Windows 10 Creator Update (version 1703), you now can use force cli within Windows Bash. To access force cli from WSL, you can call `force.exe`

For ease of use you can add the following simple alias

      alias force=force.exe

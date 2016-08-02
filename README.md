## force CLI v0.22.39

A command-line interface to force.com
![](https://travis-ci.org/heroku/force.svg?branch=master)

### Installation

##### Precompiled Binaries

Can be downloaded from the [Current Release Page](https://github.com/heroku/force/releases/latest) or by visiting the [CLI Home Page](http://force-cli.herokuapp.com/).

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
       describe  Describe the object or list of available objects
       sobject   Manage standard & custom objects
       field     Manage sobject fields
       record    Create, modify, or view records
       bulk      Load csv file use Bulk API
       fetch     Export specified artifact(s) to a local directory
       import    Import metadata from a local directory
       export    Export metadata to a local directory
       query     Execute a SOQL statement
       apex      Execute anonymous Apex code
       log       Fetch debug logs
       oauth     Manage ConnectedApp credentials
       test      Run apex tests
       security  Displays the OLS and FLS for a given SObject
       version   Display current version
       update    Update to the latest version
       push      Deploy single artifact from a local directory
       aura      Retrieve or deploy Aura components
       password  See password status or reset password
       notify    Should notifications be used
       limits    Display current limits
       help      Show this help

    Run 'force help [command]' for details.

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

      force whomai

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

### Importing and Exporting Metadata

You can use these commands to copy metadata in and out of Salesforce.  This can be particularly useful for implementing external change management using a VCS such as a git.  However, remember that Salesforce metadata can not be always be simply replaced on a target environment, and so you'll have to be mindful of deleted or renamed metadata objects, in addition to dependencies and undeployable metadata (some retrieved metadata is not valid for being redeployed, particularly things that involve builtins).

The two relevant commands are `force import` and `force export`.

#### Project Structure

When you use `force export` for the first time, you can pass it a path to write the contents to.  By default it retrieves all known types of metadata.  As defined by the Salesforce Metadata API itself, the resulting file structure will have a directory for each type of metadata object, in addition to a `package.xml` manifest.  When run, `force import` will check for the existence of `package.xml` before creating a changeset.

#### Project-level Configuration

Force supports per-project config on your filesystem/source code repository in an `environments.json` config file as a sibling file with your `package.xml`.  Currently this only supports one feature, simple pre-processing of your metadata with variable interpolation when using the `import` command to deploy metadata.

The format of that file is a JSON object hash, with keys of environment names (arbitrary strings) containing hashes which the individual environments.  The object is a hash of environment names (the names are currently used for informative purposes only) mapped to objects which respectively describe the environment, which in turn contain `match` objects of regexes that match by the login name (`login`) and/or by the target instance (`instance`).

A suggested approach is to leverage the typical Salesforce convention (described in the Users and Contacts section of SF's [Sandbox Setup Considerations](https://help.salesforce.com/HTViewHelpDoc?id=data_sandbox_implementation_tips.htm) documentation) of appending Sandbox names to usernames in order to match environments to logins.  This method allows for discrimating between prod and the multiple sandboxes in a given org/project, while still allowing for multiple users to collaborate on the project without them having to meddle with their project configuration on an individual basis.

Example `environments.json`:

```json
{
    "environments": {
        "dev": {
            "match": {
                "login": "@myapp.com.dev$"
            }
        },
        "production": {
            "match": {
                "login": "@myapp.com$"
            }
        },
        "demo": {
            "match": {
                "instance": "https://cs00.salesforce.com/"
            }
        },
    }
}
```

##### Variable interpolation

If required, you can use the string interpolation feature of `force` to dynamically modify your metadata when running `force import` according to a set of variables you can specify.  This can be handy for working around restrictions of Salesforce metadata that by design must refer to a given instance-specific piece of data (say, a user ID), or perhaps a remote endpoint.  Which interpolation to apply is discriminated by the hostname of the instance.

Example `environments.json` where a Salesforce project is integrating with a hypothetical app running on Heroku:

```json
{
    "environments": {
        "dev": {
            "match": {
                "login": "@myapp.com.dev$"
            },
            "vars": {
                "INTEGRATION_HOST": "https://dave-super-staging.herokuapp.com",
                "INTEGRATION_TOKEN": "derp",
                "INTEGRATION_USER": "api@example.com.dev"
            }
        },
        "production": {
            "match": {
                "login": "@myapp.com$"
            },
            "vars": {
                "INTEGRATION_HOST": "https://dave-super.herokuapp.com",
                "INTEGRATION_TOKEN": "sekrit",
                "INTEGRATION_USER": "api@example.com"
            }
        }
    }
}
```

### notify
Includes notification library, [gotifier](https://github.com/ViViDboarder/gotifier), that will display notifications for using either Using [terminal-notifier](https://github.com/julienXX/terminal-notifier) on OSX or [notify-send](http://manpages.ubuntu.com/manpages/saucy/man1/notify-send.1.html) on Ubuntu. Currently, only the `push` and `test` methods are displaying notifications.

### limits
Limits will display limits information for your organization.
- Max is the limit total for the organization
- Remaining is the total number of calls or events left for the organization

The list is limited to those exposed by the REST API.

      force limits

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

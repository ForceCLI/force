package project

import (
	"encoding/json"
	"errors"
	"fmt"
	"strings"
)

// EnvironmentConfigJSON is the struct within your environment.json that
// describes a single environment (staging, prod, sandbox, etc.)
type EnvironmentConfigJSON struct {
	// Domain is the domain host fragment of the username addresses that are used to logged into the
	// SF instance (prod or a sandbox).  This allows for mapping a login to an SF instance to the
	// environment it represents in a way that will be consistent across multiple developers'
	// accounts and machines.
	Domain string `json:"domain"`

	// Variables is a map of placeholders and values that will be interpolated into the metadata,
	// wherever the token is found with a $ prefixed.
	Variables map[string]string `json:"vars"`

	// Human-readable name for this instance.  This does not come from the contents of the JSON
	// object, but rather the name of the key in the top-level EnvironmentsConfigJSON object that
	// contained it.
	Name string
}

// EnvironmentsConfigJSON is the root struct for JSON unmarshalling that an `environment.json` file
// in your source tree root.  It can describe your SF environments and other settings, particularly
// parameters that can be templated into your Salesforce metadata files.
type EnvironmentsConfigJSON struct {
	Environments map[string]EnvironmentConfigJSON `json:"environments"`
}

// GetEnvironmentConfigForActiveUser retrieves the a user-specified environment configuration for
// the active project, looked up by the active username.  Returns nil if there's no per-project
// environment config set up.
func (project *project) GetEnvironmentConfigForActiveUser(activeUsername string) (foundEnvironment *EnvironmentConfigJSON, err error) {
	if environmentJSON, present := project.EnumerateContents()["environments.json"]; present {
		// SF usernames are in RFC 22-style email addresses (not to be confused with the account's email
		// address, however).  Split the thing up to get the domain, which we care about for matching:
		usernameFragments := strings.Split(activeUsername, "@")
		if len(usernameFragments) != 2 {
			return nil, errors.New("Your username appears invalid.")
		}
		activeUserDomain := usernameFragments[1]

		// now, we want to implement our interpolation regime!
		environmentConfig := EnvironmentsConfigJSON{}
		json.Unmarshal(environmentJSON, &environmentConfig)

		// now, to determine the current environment.
		for name, env := range environmentConfig.Environments {
			envCopy := env
			if activeUserDomain == env.Domain {
				foundEnvironment = &envCopy
				foundEnvironment.Name = name
				return
			}
		}
		if foundEnvironment == nil {
			err = fmt.Errorf("None of the specified environments in your project config matched your active login (consider checking that the domain name of your active login matches an entry in environments.json): '%s'\n", activeUserDomain)
		}
	}
	return
}

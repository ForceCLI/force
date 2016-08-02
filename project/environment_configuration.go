package project

import (
	"encoding/json"
	"fmt"
	"regexp"
)

// EnvironmentMatch can be specified as the `match` value in an environment stanza in
// environments.json.  The goal is to allow the author of a project to specify how to match
// a given target environment's configuration with the current active force user.
type EnvironmentMatch struct {
	// LoginRegex can be specified to match against the active login's username.
	LoginRegex *string `json:"login"`

	// InstanceRegex can be specified to match against the active login's sf instance hostname (eg.,
	// `https://na00.salesforce.com`)
	InstanceRegex *string `json:"instance"`
}

// EnvironmentConfigJSON is the struct within your environment.json that
// describes a single environment (staging, prod, sandbox, etc.)
type EnvironmentConfigJSON struct {
	// MatchCriteria This allows for mapping a login to an SF instance to the
	// environment it represents in a way that will be consistent across multiple developers'
	// accounts and machines.
	MatchCriteria *EnvironmentMatch `json:"match"`

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
// the active project, looked up by comparing the given username and instance URI with matchers
// specified in environments.json for each environment.  Returns nil if there's no per-project
// environment config set up.
func (project *project) GetEnvironmentConfigForActiveEnvironment(activeUsername string, activeInstanceURI string) (foundEnvironment *EnvironmentConfigJSON, err error) {
	if environmentJSON, present := project.EnumerateContents()["environments.json"]; present {
		// now, we want to implement our interpolation regime!
		environmentConfig := EnvironmentsConfigJSON{}
		if err = json.Unmarshal(environmentJSON, &environmentConfig); err != nil {
			err = fmt.Errorf("Problem parsing environments.json at offset %v: %s", err.(*json.SyntaxError).Offset, err.Error())
			return
		}

		// now, to determine the current environment.
		for name, env := range environmentConfig.Environments {
			if env.MatchCriteria == nil {
				fmt.Printf("WARN: No matchers specified for environment '%s' in your environments.json.  See README.\n", name)
				continue
			}
			instanceMatched := true
			loginMatched := true

			if env.MatchCriteria.InstanceRegex != nil {
				instanceMatched, err = regexp.MatchString(*env.MatchCriteria.InstanceRegex, activeInstanceURI)
				if err != nil {
					return
				}
			}

			if env.MatchCriteria.LoginRegex != nil {
				loginMatched, err = regexp.MatchString(*env.MatchCriteria.LoginRegex, activeUsername)
				if err != nil {
					return
				}
			}

			if loginMatched && instanceMatched {
				envCopy := env

				foundEnvironment = &envCopy
				foundEnvironment.Name = name
				return
			}
		}
		if foundEnvironment == nil {
			err = fmt.Errorf("None of the environments specified in your project config matched your active login: '%s'\n", activeUsername)
		}
	}
	return
}

package project

import (
	"encoding/json"

	"github.com/heroku/force/util"
)

// EnvironmentConfigJSON is the struct within your environment.json that
// describes a single environment (staging, prod, sandbox, etc.)
type EnvironmentConfigJSON struct {
	// InstanceHost is in the fully qualified HTTPS URI format:
	// eg. 'https://na30.salesforce.com'
	InstanceHost string            `json:"instance"`
	Variables    map[string]string `json:"vars"`

	// Human-readable name for this instance.
	Name string
}

// EnvironmentsConfigJSON is the root struct for JSON unmarshalling that an `environment.json` file
// in your source tree root.  It can describe your SF environments and
// other settings, particularly parameters that can be templated into your
// Salesforce metadata files.
type EnvironmentsConfigJSON struct {
	Environments map[string]EnvironmentConfigJSON `json:"environments"`
}

// GetEnvironmentConfigForActiveEnvironment retrieves the a user-specified environment
// configuration for the active project, looked up by the active SF intance URI.
func (project *project) GetEnvironmentConfigForActiveEnvironment(activeInstanceURL string) *EnvironmentConfigJSON {
	var foundEnvironment *EnvironmentConfigJSON
	if environmentJSON, present := project.EnumerateContents()["environments.json"]; present {
		// now, we want to implement our interpolation regime!
		environmentConfig := EnvironmentsConfigJSON{}
		json.Unmarshal(environmentJSON, &environmentConfig)

		// now, to determine the current environment.

		var foundEnvironment *EnvironmentConfigJSON
		for name, env := range environmentConfig.Environments {
			envCopy := env
			if activeInstanceURL == env.InstanceHost {
				foundEnvironment = &envCopy
				foundEnvironment.Name = name
			}
		}
		if foundEnvironment == nil {
			util.ErrorAndExit("No project specified environment config matched your active SF instance: '%s'\n", activeInstanceURL)
		}
	}
	return foundEnvironment
}

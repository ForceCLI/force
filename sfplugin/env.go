package sfplugin

import (
	"os"
	"strconv"
)

// Environment variables the CLI sets for a plugin process.
const (
	// EnvProtocol is the protocol version of the CLI that started the
	// plugin.
	EnvProtocol = "SF_PLUGIN_PROTOCOL"
	// EnvBinName is the command line that starts the CLI, such as "sf" or
	// "force --sf".  A plugin uses it in the commands it suggests.
	EnvBinName = "SF_PLUGIN_BIN_NAME"
	// EnvName and EnvVersion are the name and version in the manifest the
	// CLI has recorded for the plugin.
	EnvName    = "SF_PLUGIN_NAME"
	EnvVersion = "SF_PLUGIN_VERSION"

	// The org variables are set for a command whose manifest entry has
	// the org requirement optional or required, when an org was resolved.
	EnvInstanceURL = "SF_PLUGIN_INSTANCE_URL"
	EnvAccessToken = "SF_PLUGIN_ACCESS_TOKEN"
	EnvAPIVersion  = "SF_PLUGIN_API_VERSION"
	EnvUsername    = "SF_PLUGIN_USERNAME"
	EnvOrgID       = "SF_PLUGIN_ORG_ID"
)

// Org is the org the CLI resolved for a command.  AccessToken is a bearer
// token for requests to InstanceURL.  The CLI passes no refresh token, so a
// plugin that receives an expired-session response reports it and exits.
type Org struct {
	InstanceURL string
	AccessToken string
	APIVersion  string
	Username    string
	OrgID       string
}

// OrgFromEnv returns the org the CLI passed to this process.  The second
// result is false when the CLI passed none.
func OrgFromEnv() (Org, bool) {
	org := Org{
		InstanceURL: os.Getenv(EnvInstanceURL),
		AccessToken: os.Getenv(EnvAccessToken),
		APIVersion:  os.Getenv(EnvAPIVersion),
		Username:    os.Getenv(EnvUsername),
		OrgID:       os.Getenv(EnvOrgID),
	}
	return org, org.InstanceURL != "" && org.AccessToken != ""
}

// BinName returns the command line that starts the CLI, or "sf" when the
// plugin was started directly.
func BinName() string {
	if name := os.Getenv(EnvBinName); name != "" {
		return name
	}
	return "sf"
}

// HostProtocol returns the protocol version of the CLI that started this
// process, or 0 when the plugin was started directly.
func HostProtocol() int {
	version, _ := strconv.Atoi(os.Getenv(EnvProtocol))
	return version
}

package lib

import (
	"fmt"
	"os"
)

var apiVersionNumber = "40.0"
var apiVersion = fmt.Sprintf("v%s", apiVersionNumber)

func ApiVersion() string {
	return apiVersion
}

func ApiVersionNumber() string {
	return apiVersionNumber
}

func UpdateApiVersion(version string) {
	apiVersion = "v" + version
	apiVersionNumber = version
	force, _ := ActiveForce()
	force.Credentials.ApiVersion = apiVersionNumber
	ForceSaveLogin(*force.Credentials, os.Stdout)
}

func SetApiVersion(version string) {
	apiVersion = "v" + version
	apiVersionNumber = version
}

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

func (f *Force) UpdateApiVersion(version string) (err error) {
	SetApiVersion(version)
	f.Credentials.SessionOptions.ApiVersion = version
	_, err = ForceSaveLogin(*f.Credentials, os.Stdout)
	return
}

func SetApiVersion(version string) {
	apiVersion = "v" + version
	apiVersionNumber = version
}

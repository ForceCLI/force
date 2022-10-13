package lib

import (
	"fmt"
	"os"
	"regexp"
)

var (
	DefaultApiVersionNumber = "55.0"
	apiVersionNumber        = DefaultApiVersionNumber
	apiVersion              = fmt.Sprintf("v%s", apiVersionNumber)
)

func ApiVersion() string {
	return apiVersion
}

func ApiVersionNumber() string {
	return apiVersionNumber
}

func (f *Force) UpdateApiVersion(version string) error {
	err := SetApiVersion(version)
	if err != nil {
		return err
	}
	f.Credentials.SessionOptions.ApiVersion = version
	_, err = ForceSaveLogin(*f.Credentials, os.Stdout)
	return err
}

func SetApiVersion(version string) error {
	matched, err := regexp.MatchString("^\\d{2}\\.0$", version)
	if err != nil || !matched {
		return fmt.Errorf("apiversion must be in the form of nn.0.")
	}
	apiVersion = "v" + version
	apiVersionNumber = version
	return nil
}

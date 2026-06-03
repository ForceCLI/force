package lib

import (
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"regexp"
	"strconv"
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

// LatestApiVersion queries the org for its supported API versions and returns
// the highest one (e.g. "67.0").
func (f *Force) LatestApiVersion() (string, error) {
	data, err := f.GetAbsolute("/services/data")
	if err != nil {
		return "", err
	}
	var versions []struct {
		Version string `json:"version"`
	}
	if err := json.Unmarshal([]byte(data), &versions); err != nil {
		return "", err
	}
	latest := ""
	var latestNum float64
	for _, v := range versions {
		n, err := strconv.ParseFloat(v.Version, 64)
		if err != nil {
			continue
		}
		if n > latestNum {
			latestNum = n
			latest = v.Version
		}
	}
	if latest == "" {
		return "", errors.New("no API versions returned by org")
	}
	return latest, nil
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

package salesforce

import (
	"net/url"
	"strings"
)

func ParseArgumentAttrs(pairs []string) (parsed map[string]string) {
	parsed = make(map[string]string)
	for _, pair := range pairs {
		split := strings.SplitN(pair, ":", 2)
		parsed[split[0]] = split[1]
	}
	return
}

func PairsToUrlValues(pairs map[string]string) (values url.Values) {
	values = url.Values{}
	for key, value := range pairs {
		values.Set(key, value)
	}
	return
}

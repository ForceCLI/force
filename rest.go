package main

import (
	"bytes"
	"fmt"
	"io/ioutil"
	"strings"
)

var cmdRest = &Command{
	Usage: "rest",
	Short: "force rest -s=<service> [-m=<method>] [-b=<body>]",
	Long: `
Usage: force rest -s=<service> [-m=<method>] [-b=<body>]

Call the RestApi Service of the <service> using the <action> and sending the define <body>

<service> Name of the service in case is part of a managed package will be <namespace>/<serviceName>
<method> Choose an HTTP method to perform on the REST API service (GET, POST, PUT, PATCH, DELETE, HEAD). By default use GET
<body> You can define a file (starting with @) or a string to send in the body of the request

Examples:

  force rest -s=RestTest

  force rest -s=RestTest -m=GET

  force rest -s=PackagePrefix/RestTest -m=POST -b=Test

  force rest -s=RestTest -m=POST -b=@./body.txt`,
}

func init() {
	cmdRest.Run = runRest
}

var (
	service = cmdRest.Flag.String("s", "", "Name of the service in case is part of a managed package will be <namespace>/<serviceName>")
	method  = cmdRest.Flag.String("m", "GET", "Choose an HTTP method to perform on the REST API service (GET, POST, PUT, PATCH, DELETE, HEAD). By default use GET")
	body    = cmdRest.Flag.String("b", "", "You can define a file or a string to send in the body")
)

func runRest(cmd *Command, args []string) {
	force, _ := ActiveForce()
	if len(args) >= 1 && *service == "" {
		*service = args[0]
	}
	url := fmt.Sprintf("%s/services/apexrest/%s", force.Credentials.InstanceUrl, *service)
	format := "application/json"

	if strings.Index(*service, ".xml") != -1 {
		format = "application/xml"
	}

	bodyContent := []byte(*body)
	if len(bodyContent) > 0 && bodyContent[0] == '@' {
		var err error
		bodyContent, err = ioutil.ReadFile(*body)
		if err != nil {
			ErrorAndExit(err.Error())
		}
	}

	req, err := httpRequest(*method, url, bytes.NewReader(bodyContent))
	if err != nil {
		return
	}
	req.Header.Add("Authorization", fmt.Sprintf("Bearer %s", force.Credentials.AccessToken))
	req.Header.Add("Content-Type", format)
	res, err := httpClient().Do(req)
	if err != nil {
		ErrorAndExit(err.Error())
	}
	if res.StatusCode/100 == 2 {
		bodyRes, _ := ioutil.ReadAll(res.Body)
		fmt.Println(string(bodyRes))
	} else {
		ErrorAndExit("Status Code: %v", res.StatusCode)
	}
}

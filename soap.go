package main

import (
	"encoding/xml"
	"errors"
	"fmt"
	"io/ioutil"
	"strings"
)

type SoapError struct {
	FaultCode   string `xml:"Body>Fault>faultcode"`
	FaultString string `xml:"Body>Fault>faultstring"`
}

type Soap struct {
	AccessToken string
	Endpoint    string
	Header      string
	Namespace   string
}

func NewSoap(endpoint, namespace, accessToken string) (s *Soap) {
	s = new(Soap)
	s.AccessToken = accessToken
	s.Namespace = namespace
	s.Endpoint = endpoint
	return
}

func (s *Soap) Execute(action, query string) (response []byte, err error) {
	soap := `
		<env:Envelope xmlns:xsd="http://www.w3.org/2001/XMLSchema" xmlns:xsi="http://www.w3.org/2001/XMLSchema-instance" xmlns:env="http://schemas.xmlsoap.org/soap/envelope/" xmlns:cmd="%s" xmlns:apex="http://soap.sforce.com/2006/08/apex">
			<env:Header>
				<cmd:SessionHeader>
					<cmd:sessionId>%s</cmd:sessionId>
				</cmd:SessionHeader>
				%s
			</env:Header>
			<env:Body>
				<%s xmlns="%s">
					%s
				</%s>
			</env:Body>
		</env:Envelope>
	`
	rbody := fmt.Sprintf(soap, s.Namespace, s.AccessToken, s.Header, action, s.Namespace, query, action)
	req, err := httpRequest("POST", s.Endpoint, strings.NewReader(rbody))
	if err != nil {
		return
	}
	req.Header.Add("Content-Type", "text/xml")
	req.Header.Add("SOAPACtion", action)
	res, err := httpClient().Do(req)
	if err != nil {
		return
	}
	defer res.Body.Close()
	if res.StatusCode == 401 {
		err = errors.New("authorization expired, please run `force login`")
		return
	}
	response, err = ioutil.ReadAll(res.Body)
	if err != nil {
		return
	}
	err = processError(response)
	return
}

func processError(body []byte) (err error) {
	var soapError SoapError
	xml.Unmarshal(body, &soapError)
	if soapError.FaultCode != "" {
		return errors.New(soapError.FaultString)
	}
	return
}

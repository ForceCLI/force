package main

import (
	"bytes"
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
func (s *Soap) ExecuteLogin(username, password string) (response []byte, err error) {
	type SoapLogin struct {
		XMLName  xml.Name `xml:"soapenv:Envelope"`
		SoapNS   string   `xml:"xmlns:soapenv,attr"`
		UrnNS    string   `xml:"xmlns:urn,attr"`
		Username string   `xml:"soapenv:Body>urn:login>urn:username"`
		Password string   `xml:"soapenv:Body>urn:login>urn:password"`
	}

	v := &SoapLogin{SoapNS: "http://schemas.xmlsoap.org/soap/envelope/", UrnNS: "urn:partner.soap.sforce.com", Username: username, Password: password}
	rbody := new(bytes.Buffer)
	enc := xml.NewEncoder(rbody)
	err = enc.Encode(v)
	if err != nil {
		return
	}

	req, err := httpRequest("POST", s.Endpoint, rbody)
	if err != nil {
		return
	}
	req.Header.Add("Content-Type", "text/xml")
	req.Header.Add("SOAPACtion", "login")

	res, err := doRequest(req)
	if err != nil {
		fmt.Println(err)
		return
	}
	defer res.Body.Close()
	if res.StatusCode == 401 {
		err = errors.New("authorization expired, please run `force login`")
		return
	}
	if res.StatusCode == 405 {
		err = errors.New("Getting a 405 error. If you are using the my domain feature and the instance flag, please check that your url matches the url on the My Domain setup page in your org.")
		return
	}
	response, err = ioutil.ReadAll(res.Body)
	if err != nil {
		return
	}
	err = processError(response)
	return

}

// Execute soap
/*func (s *Soap) ExecuteLogin(username, password string) (response []byte, err error) {
	soap := `
		<soapenv:Envelope xmlns:soapenv="http://schemas.xmlsoap.org/soap/envelope/"
  				xmlns:urn="urn:partner.soap.sforce.com">
  			<soapenv:Body>
    			<urn:login>
      				<urn:username>%s</urn:username>
      				<urn:password>%s</urn:password>
    			</urn:login>
  			</soapenv:Body>
		</soapenv:Envelope>
		`
	rbody := fmt.Sprintf(soap, username, password)

	req, err := httpRequest("POST", s.Endpoint, strings.NewReader(rbody))
	if err != nil {
		return
	}
	req.Header.Add("Content-Type", "text/xml")
	req.Header.Add("SOAPACtion", "login")

	res, err := httpClient().Do(req)
	if err != nil {
		fmt.Println(err)
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

}*/

func (s *Soap) Execute(action, query string) (response []byte, err error) {
	soap := `
		<env:Envelope xmlns:xsd="http://www.w3.org/2001/XMLSchema" 
		xmlns:xsi="http://www.w3.org/2001/XMLSchema-instance" 
		xmlns:env="http://schemas.xmlsoap.org/soap/envelope/" 
		xmlns:cmd="%s" 
		xmlns:apex="http://soap.sforce.com/2006/08/apex">
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
	rbody := fmt.Sprintf(soap, s.Namespace,
		s.AccessToken, s.Header, action, s.Namespace, query, action)
	//fmt.Println(rbody)
	req, err := httpRequest("POST", s.Endpoint, strings.NewReader(rbody))
	if err != nil {
		return
	}
	req.Header.Add("Content-Type", "text/xml")
	req.Header.Add("SOAPACtion", action)
	res, err := doRequest(req)
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

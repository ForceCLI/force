package lib

import (
	"bytes"
	"encoding/xml"
	"errors"
	"fmt"
	"io"
	"io/ioutil"
	"net/http"
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

func (s *Soap) Execute(action, query string) (response []byte, err error) {
	res, err := s.execute(action, query)
	if err != nil {
		return
	}
	defer res.Body.Close()
	response, err = ioutil.ReadAll(res.Body)
	if err != nil {
		return
	}
	if isSoapInvalidSessionError(response) {
		err = SessionExpiredError
		return
	}
	err = processError(response)
	return
}

// ExecuteExtract performs the SOAP request like Execute, but streams the text
// content of the named element to w instead of including it in the returned
// response. Use for responses with very large payloads, like the
// base64-encoded zip file in a checkRetrieveStatus response.
func (s *Soap) ExecuteExtract(action, query, element string, w io.Writer) (response []byte, err error) {
	res, err := s.execute(action, query)
	if err != nil {
		return
	}
	defer res.Body.Close()
	response, err = extractElementText(res.Body, element, w)
	if err != nil {
		return
	}
	if isSoapInvalidSessionError(response) {
		err = SessionExpiredError
		return
	}
	err = processError(response)
	return
}

func (s *Soap) execute(action, query string) (res *http.Response, err error) {
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
	req, err := httpRequest("POST", s.Endpoint, strings.NewReader(rbody))
	if err != nil {
		return
	}
	req.Header.Add("Content-Type", "text/xml")
	req.Header.Add("SOAPACtion", action)
	res, err = doRequest(req)
	if err != nil {
		return
	}
	if res.Header.Get(http.CanonicalHeaderKey("x-sfdc-edge-err")) == "true" {
		res.Body.Close()
		res = nil
		err = errors.New("Unexpected error from Salesforce Edge")
		return
	}
	if res.StatusCode == 401 {
		res.Body.Close()
		res = nil
		err = errors.New("authorization expired, please run `force login`")
		return
	}
	return
}

func isSoapInvalidSessionError(body []byte) bool {
	var soapError SoapError
	xml.Unmarshal(body, &soapError)
	return soapError.FaultCode == "sf:INVALID_SESSION_ID"
}

func processError(body []byte) (err error) {
	var soapError SoapError
	xml.Unmarshal(body, &soapError)
	if soapError.FaultCode != "" {
		return errors.New(soapError.FaultString)
	}
	return
}

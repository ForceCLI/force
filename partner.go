package main

import (
	"encoding/xml"
	"errors"
	"fmt"
	"strings"
)

type ForcePartner struct {
	Force *Force
}

func NewForcePartner(force *Force) (partner *ForcePartner) {
	partner = &ForcePartner{Force: force}
	return
}

func (partner *ForcePartner) CheckStatus(id string) (err error) {
	body, err := partner.soapExecute("checkStatus", fmt.Sprintf("<id>%s</id>", id))
	if err != nil {
		return
	}
	var status struct {
		Done    bool   `xml:"Body>checkStatusResponse>result>done"`
		State   string `xml:"Body>checkStatusResponse>result>state"`
		Message string `xml:"Body>checkStatusResponse>result>message"`
	}
	if err = xml.Unmarshal(body, &status); err != nil {
		return
	}
	switch {
	case !status.Done:
		return partner.CheckStatus(id)
	case status.State == "Error":
		return errors.New(status.Message)
	}
	return
}

func (partner *ForcePartner) ExecuteAnonymous(apex string) (output string, err error) {
	soap := "<apexcode><![CDATA[%s]]></apexcode>"
	body, err := partner.soapExecute("executeAnonymous", fmt.Sprintf(soap, apex))
	if err != nil {
		return
	}
	var result struct {
		Compiled         bool   `xml:"Body>executeAnonymousResponse>result>compiled"`
		CompileProblem   string `xml:"Body>executeAnonymousResponse>result>compileProblem"`
		ExceptionMessage string `xml:"Body>executeAnonymousResponse>result>exceptionMessage"`
		ExceptionTrace   string `xml:"Body>executeAnonymousResponse>result>exceptionStackTrace"`
		Log              string `xml:"Header>DebuggingInfo>debugLog"`
		Success          bool   `xml:"Body>executeAnonymousResponse>result>success"`
	}
	if err = xml.Unmarshal(body, &result); err != nil {
		return
	}
	if !result.Compiled {
		message := strings.Replace(result.CompileProblem, "%", "%%", -1)
		err = errors.New(message)
		return
	}
	output = result.Log
	return
}

func (partner *ForcePartner) soapExecute(action, query string) (response []byte, err error) {
	login, err := partner.Force.Get(partner.Force.Credentials.Id)
	if err != nil {
		return
	}
	url := strings.Replace(login["urls"].(map[string]interface{})["partner"].(string), "{version}", "28.0", 1)
	url = strings.Replace(url, "/u/", "/s/", 1) // seems dirty
	soap := NewSoap(url, "http://soap.sforce.com/2006/08/apex", partner.Force.Credentials.AccessToken, partner.Force.Credentials.AllowSelfSignedCertificates)
	soap.Header = "<apex:DebuggingHeader><apex:debugLevel>DEBUGONLY</apex:debugLevel></apex:DebuggingHeader>"
	response, err = soap.Execute(action, query)
	return
}

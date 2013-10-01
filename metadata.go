package main

import (
	"archive/zip"
	"bytes"
	"encoding/base64"
	"encoding/xml"
	"errors"
	"fmt"
	"io/ioutil"
	"strings"
)

type SoapCreateResponse struct {
	Done  bool   `xml:"Body>createResponse>result>done"`
	Id    string `xml:"Body>createResponse>result>id"`
	State string `xml:"Body>createResponse>result>state"`
}

type SoapCheckStatusResponse struct {
	Done    bool   `xml:"Body>checkStatusResponse>result>done"`
	Id      string `xml:"Body>checkStatusResponse>result>id"`
	State   string `xml:"Body>checkStatusResponse>result>state"`
	Message string `xml:"Body>checkStatusResponse>result>message"`
}

type SoapListConnectedAppsResponse struct {
	ConnectedApps []SoapListConnectedAppsResponseConnectedApps `xml:"Body>listMetadataResponse>result"`
}

type SoapListConnectedAppsResponseConnectedApps struct {
	Name string `xml:"fullName"`
	Id string `xml:"id"`
}

type SoapRetrieveResponse struct {
	Done  bool   `xml:"Body>retrieveResponse>result>done"`
	Id    string `xml:"Body>retrieveResponse>result>id"`
	State string `xml:"Body>retrieveResponse>result>state"`
}

type SoapCheckRetrieveStatusResponse struct {
	ZipFile string `xml:"Body>checkRetrieveStatusResponse>result>zipFile"`
}

type ForceConnectedApps []ForceConnectedApp

type ForceConnectedApp struct {
	Id string
	Name string
}

type ForceMetadata struct {
	Force *Force
}

func NewForceMetadata(force *Force) (fm *ForceMetadata) {
	fm = &ForceMetadata{Force:force}
	return
}

func (fm *ForceMetadata) CreateConnectedApp(name, callback string) (app string, err error) {
	soap := `
		<env:Envelope xmlns:xsd="http://www.w3.org/2001/XMLSchema" xmlns:xsi="http://www.w3.org/2001/XMLSchema-instance" xmlns:tns="http://soap.sforce.com/2006/04/metadata" xmlns:env="http://schemas.xmlsoap.org/soap/envelope/" xmlns:cmd="http://soap.sforce.com/2006/04/metadata">
			<env:Header>
				<cmd:SessionHeader>
					<cmd:sessionId>%s</cmd:sessionId>
				</cmd:SessionHeader>
			</env:Header>
			<env:Body>
				<create xmlns="http://soap.sforce.com/2006/04/metadata">
					<metadata xsi:type="cmd:ConnectedApp" xmlns:cmd="http://soap.sforce.com/2006/04/metadata">
						<fullName>%s</fullName>
						<version>29.0</version>
						<label>%s</label>
						<contactEmail>%s</contactEmail>
						<oauthConfig>
							<callbackUrl>%s</callbackUrl>
							<scopes>Full</scopes>
							<scopes>RefreshToken</scopes>
							<consumerSecrett>Zm9sdfjsdkfjsdfkjv</consumerSecrett>
						</oauthConfig>
					</metadata>
				</create>
			</env:Body>
		</env:Envelope>
	`
	login, err := fm.Force.Get(fm.Force.Credentials.Id)
	if err != nil {
		return
	}
	url := strings.Replace(login["urls"].(map[string]interface{})["metadata"].(string), "{version}", "29.0", 1)
	rbody := fmt.Sprintf(soap, fm.Force.Credentials.AccessToken, name, name, "foo@bar.org", callback)
	fmt.Println("rbody", rbody)
	req, err := httpRequest("POST", url, strings.NewReader(rbody))
	if err != nil {
		return
	}
	req.Header.Add("Content-Type", "text/xml")
	req.Header.Add("SOAPACtion", "create")
	res, err := httpClient().Do(req)
	defer res.Body.Close()
	if err != nil {
		return
	}
	if res.StatusCode == 401 {
		err = errors.New("authorization expired, please run `force login`")
		return
	}
	body, err := ioutil.ReadAll(res.Body)
	var create SoapCreateResponse
	err = xml.Unmarshal(body, &create)
	if err != nil {
		return
	}
	err = fm.CheckStatus(create.Id)
	if err != nil {
		return
	}
	return
}

func (fm *ForceMetadata) CheckStatus(id string) (err error) {
	soap := `
		<env:Envelope xmlns:xsd="http://www.w3.org/2001/XMLSchema" xmlns:xsi="http://www.w3.org/2001/XMLSchema-instance" xmlns:tns="http://soap.sforce.com/2006/04/metadata" xmlns:env="http://schemas.xmlsoap.org/soap/envelope/" xmlns:cmd="http://soap.sforce.com/2006/04/metadata">
			<env:Header>
				<cmd:SessionHeader>
					<cmd:sessionId>%s</cmd:sessionId>
				</cmd:SessionHeader>
			</env:Header>
			<env:Body>
				<checkStatus xmlns="http://soap.sforce.com/2006/04/metadata">
					<id>%s</id>
				</checkStatus>
			</env:Body>
		</env:Envelope>
	`
	login, err := fm.Force.Get(fm.Force.Credentials.Id)
	if err != nil {
		return
	}
	url := strings.Replace(login["urls"].(map[string]interface{})["metadata"].(string), "{version}", "29.0", 1)
	rbody := fmt.Sprintf(soap, fm.Force.Credentials.AccessToken, id)
	req, err := httpRequest("POST", url, strings.NewReader(rbody))
	if err != nil {
		return
	}
	req.Header.Add("Content-Type", "text/xml")
	req.Header.Add("SOAPACtion", "checkStatus")
	res, err := httpClient().Do(req)
	defer res.Body.Close()
	if err != nil {
		return
	}
	if res.StatusCode == 401 {
		err = errors.New("authorization expired, please run `force login`")
		return
	}
	body, err := ioutil.ReadAll(res.Body)
	fmt.Println("body", string(body))
	var status struct {
		Done    bool   `xml:"Body>checkStatusResponse>result>done"`
		Id      string `xml:"Body>checkStatusResponse>result>id"`
		State   string `xml:"Body>checkStatusResponse>result>state"`
		Message string `xml:"Body>checkStatusResponse>result>message"`
	}
	err = xml.Unmarshal(body, &status)
	if err != nil {
		return
	}
	switch {
	case !status.Done:
		return fm.CheckStatus(id)
	case status.State == "Error":
		return errors.New(status.Message)
	}
	return
}

func (fm *ForceMetadata) ListConnectedApps() (apps ForceConnectedApps, err error) {
	soap := `
		<env:Envelope xmlns:xsd="http://www.w3.org/2001/XMLSchema" xmlns:xsi="http://www.w3.org/2001/XMLSchema-instance" xmlns:tns="http://soap.sforce.com/2006/04/metadata" xmlns:env="http://schemas.xmlsoap.org/soap/envelope/" xmlns:cmd="http://soap.sforce.com/2006/04/metadata">
			<env:Header>
				<cmd:SessionHeader>
					<cmd:sessionId>%s</cmd:sessionId>
				</cmd:SessionHeader>
			</env:Header>
			<env:Body>
				<listMetadata xmlns="http://soap.sforce.com/2006/04/metadata">
					<queries>
						<type>ConnectedApp</type>
					</queries>
				</listMetadata>
			</env:Body>
		</env:Envelope>
	`
	login, err := fm.Force.Get(fm.Force.Credentials.Id)
	if err != nil {
		return
	}
	url := strings.Replace(login["urls"].(map[string]interface{})["metadata"].(string), "{version}", "29.0", 1)
	rbody := fmt.Sprintf(soap, fm.Force.Credentials.AccessToken)
	req, err := httpRequest("POST", url, strings.NewReader(rbody))
	if err != nil {
		return
	}
	req.Header.Add("Content-Type", "text/xml")
	req.Header.Add("SOAPACtion", "listMetadata")
	res, err := httpClient().Do(req)
	defer res.Body.Close()
	if err != nil {
		return
	}
	if res.StatusCode == 401 {
		err = errors.New("authorization expired, please run `force login`")
		return
	}
	body, err := ioutil.ReadAll(res.Body)
	var resp SoapListConnectedAppsResponse
	xml.Unmarshal(body, &resp)
	for _, app := range resp.ConnectedApps {
		apps = append(apps, ForceConnectedApp{Name:app.Name, Id:app.Id})
	}
	return
}

func (fm *ForceMetadata) GetConnectedApp(name string) (app ForceConnectedApp, err error) {
	soap := `
		<env:Envelope xmlns:xsd="http://www.w3.org/2001/XMLSchema" xmlns:xsi="http://www.w3.org/2001/XMLSchema-instance" xmlns:tns="http://soap.sforce.com/2006/04/metadata" xmlns:env="http://schemas.xmlsoap.org/soap/envelope/" xmlns:cmd="http://soap.sforce.com/2006/04/metadata">
			<env:Header>
				<cmd:SessionHeader>
					<cmd:sessionId>%s</cmd:sessionId>
				</cmd:SessionHeader>
			</env:Header>
			<env:Body>
				<retrieve xmlns="http://soap.sforce.com/2006/04/metadata">
					<retrieveRequest>
						<apiVersion>29.0</apiVersion>
						<unpackaged>
							<types>
								<members>%s</members>
								<name>ConnectedApp</name>
							</types>
						</unpackaged>
					</retrieveRequest>
				</retrieve>
			</env:Body>
		</env:Envelope>
	`
	login, err := fm.Force.Get(fm.Force.Credentials.Id)
	if err != nil {
		return
	}
	url := strings.Replace(login["urls"].(map[string]interface{})["metadata"].(string), "{version}", "29.0", 1)
	rbody := fmt.Sprintf(soap, fm.Force.Credentials.AccessToken, name)
	fmt.Println("rbody", rbody)
	req, err := httpRequest("POST", url, strings.NewReader(rbody))
	if err != nil {
		return
	}
	req.Header.Add("Content-Type", "text/xml")
	req.Header.Add("SOAPACtion", "retrieve")
	res, err := httpClient().Do(req)
	defer res.Body.Close()
	if err != nil {
		return
	}
	if res.StatusCode == 401 {
		err = errors.New("authorization expired, please run `force login`")
		return
	}
	body, err := ioutil.ReadAll(res.Body)
	var retrieve SoapRetrieveResponse
	xml.Unmarshal(body, &retrieve)
	err = fm.CheckStatus(retrieve.Id)
	if err != nil {
		return
	}
	err = fm.CheckRetrieveStatus(retrieve.Id)
	fmt.Println("err", err)
	/* var resp SoapListConnectedAppsResponse*/
	/* xml.Unmarshal(body, &resp)*/
	/* for _, app := range resp.ConnectedApps {*/
	/*   apps = append(apps, ForceConnectedApp{Name:app.Name, Id:app.Id})*/
	/* }*/
	return
}

func (fm *ForceMetadata) CheckRetrieveStatus(id string) (err error) {
	soap := `
		<env:Envelope xmlns:xsd="http://www.w3.org/2001/XMLSchema" xmlns:xsi="http://www.w3.org/2001/XMLSchema-instance" xmlns:tns="http://soap.sforce.com/2006/04/metadata" xmlns:env="http://schemas.xmlsoap.org/soap/envelope/" xmlns:cmd="http://soap.sforce.com/2006/04/metadata">
			<env:Header>
				<cmd:SessionHeader>
					<cmd:sessionId>%s</cmd:sessionId>
				</cmd:SessionHeader>
			</env:Header>
			<env:Body>
				<checkRetrieveStatus xmlns="http://soap.sforce.com/2006/04/metadata">
					<id>%s</id>
				</checkRetrieveStatus>
			</env:Body>
		</env:Envelope>
	`
	login, err := fm.Force.Get(fm.Force.Credentials.Id)
	if err != nil {
		return
	}
	url := strings.Replace(login["urls"].(map[string]interface{})["metadata"].(string), "{version}", "29.0", 1)
	rbody := fmt.Sprintf(soap, fm.Force.Credentials.AccessToken, id)
	req, err := httpRequest("POST", url, strings.NewReader(rbody))
	if err != nil {
		return
	}
	req.Header.Add("Content-Type", "text/xml")
	req.Header.Add("SOAPACtion", "checkRetrieveStatus")
	res, err := httpClient().Do(req)
	defer res.Body.Close()
	if err != nil {
		return
	}
	if res.StatusCode == 401 {
		err = errors.New("authorization expired, please run `force login`")
		return
	}
	body, err := ioutil.ReadAll(res.Body)
	var status SoapCheckRetrieveStatusResponse
	err = xml.Unmarshal(body, &status)
	if err != nil {
		return
	}
	zipfile, err := base64.StdEncoding.DecodeString(status.ZipFile)
	if err != nil {
		return
	}
	files, err := zip.NewReader(bytes.NewReader(zipfile), int64(len(zipfile)))
	if err != nil {
		return
	}
	for _, file := range files.File {
		if strings.Contains(file.Name, "connectedApp") {
			rc, _ := file.Open()
			defer rc.Close()
			bytes, _ := ioutil.ReadAll(rc)
			fmt.Println("bytes", string(bytes))
		}
	}
	return
}

func (apps ForceConnectedApps) Len() (int) {
	return len(apps)
}

func (apps ForceConnectedApps) Less(i, j int) (bool) {
	return apps[i].Name < apps[j].Name
}

func (apps ForceConnectedApps) Swap(i, j int) {
	apps[i], apps[j] = apps[j], apps[i]
}


func (fm *ForceMetadata) GetSobject(name string) (app ForceConnectedApp, err error) {
	soap := `
		<env:Envelope xmlns:xsd="http://www.w3.org/2001/XMLSchema" xmlns:xsi="http://www.w3.org/2001/XMLSchema-instance" xmlns:tns="http://soap.sforce.com/2006/04/metadata" xmlns:env="http://schemas.xmlsoap.org/soap/envelope/" xmlns:cmd="http://soap.sforce.com/2006/04/metadata">
			<env:Header>
				<cmd:SessionHeader>
					<cmd:sessionId>%s</cmd:sessionId>
				</cmd:SessionHeader>
			</env:Header>
			<env:Body>
				<retrieve xmlns="http://soap.sforce.com/2006/04/metadata">
					<retrieveRequest>
						<apiVersion>29.0</apiVersion>
						<unpackaged>
							<types>
								<members>%s</members>
								<name>CustomObject</name>
							</types>
						</unpackaged>
					</retrieveRequest>
				</retrieve>
			</env:Body>
		</env:Envelope>
	`
	login, err := fm.Force.Get(fm.Force.Credentials.Id)
	if err != nil {
		return
	}
	url := strings.Replace(login["urls"].(map[string]interface{})["metadata"].(string), "{version}", "29.0", 1)
	rbody := fmt.Sprintf(soap, fm.Force.Credentials.AccessToken, name)
	fmt.Println("rbody", rbody)
	req, err := httpRequest("POST", url, strings.NewReader(rbody))
	if err != nil {
		return
	}
	req.Header.Add("Content-Type", "text/xml")
	req.Header.Add("SOAPACtion", "retrieve")
	res, err := httpClient().Do(req)
	defer res.Body.Close()
	if err != nil {
		return
	}
	if res.StatusCode == 401 {
		err = errors.New("authorization expired, please run `force login`")
		return
	}
	body, err := ioutil.ReadAll(res.Body)
	fmt.Println("body", string(body))
	/* var retrieve SoapRetrieveResponse*/
	/* xml.Unmarshal(body, &retrieve)*/
	/* err = fm.CheckStatus(retrieve.Id)*/
	/* if err != nil {*/
	/*   return*/
	/* }*/
	/* err = fm.CheckRetrieveStatus(retrieve.Id)*/
	/* fmt.Println("err", err)*/
	/* var resp SoapListConnectedAppsResponse*/
	/* xml.Unmarshal(body, &resp)*/
	/* for _, app := range resp.ConnectedApps {*/
	/*   apps = append(apps, ForceConnectedApp{Name:app.Name, Id:app.Id})*/
	/* }*/
	return
}

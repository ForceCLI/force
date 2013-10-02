package main

import (
	"bitbucket.org/pkg/inflect"
	"encoding/xml"
	"errors"
	"fmt"
	"io/ioutil"
	"strings"
)

type ForceConnectedApps []ForceConnectedApp

type ForceConnectedApp struct {
	Name string `xml:"fullName"`
	Id   string `xml:"id"`
}

type SoapError struct {
	FaultCode   string `xml:"Body>Fault>faultcode"`
	FaultString string `xml:"Body>Fault>faultstring"`
}

type ForceMetadata struct {
	Force *Force
}

func NewForceMetadata(force *Force) (fm *ForceMetadata) {
	fm = &ForceMetadata{Force: force}
	return
}

func (fm *ForceMetadata) CheckStatus(id string) (err error) {
	body, err := fm.soapExecute("checkStatus", fmt.Sprintf("<id>%s</id>", id))
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
		return fm.CheckStatus(id)
	case status.State == "Error":
		return errors.New(status.Message)
	}
	return
}

func (fm *ForceMetadata) CreateCustomField(object, field, typ string) (err error) {
	soap := `
		<metadata xsi:type="CustomField" xmlns:cmd="http://soap.sforce.com/2006/04/metadata">
			<fullName>%s.%s__c</fullName>
			<label>%s</label>
			%s
		</metadata>
	`
	soapField := ""
	switch strings.ToLower(typ) {
	case "text":
		soapField = "<type>Text</type><length>255</length>"
	case "datetime":
		soapField = "<type>DateTime</type>"
	case "number", "int":
		soapField = "<type>Number</type><precision>10</precision><scale>0</scale>"
	case "float":
		soapField = "<type>Number</type><precision>10</precision><scale>2</scale>"
	default:
		ErrorAndExit("unable to create field type: %s", typ)
	}
	body, err := fm.soapExecute("create", fmt.Sprintf(soap, object, field, field, soapField))
	if err != nil {
		return err
	}
	var status struct {
		Id string `xml:"Body>createResponse>result>id"`
	}
	if err = xml.Unmarshal(body, &status); err != nil {
		return
	}
	if err = fm.CheckStatus(status.Id); err != nil {
		return
	}
	return
}

func (fm *ForceMetadata) DeleteCustomField(object, field string) (err error) {
	soap := `
		<metadata xsi:type="CustomField" xmlns:cmd="http://soap.sforce.com/2006/04/metadata">
			<fullName>%s.%s</fullName>
		</metadata>
	`
	body, err := fm.soapExecute("delete", fmt.Sprintf(soap, object, field))
	if err != nil {
		return err
	}
	var status struct {
		Id string `xml:"Body>deleteResponse>result>id"`
	}
	if err = xml.Unmarshal(body, &status); err != nil {
		return
	}
	if err = fm.CheckStatus(status.Id); err != nil {
		return
	}
	return
}

func (fm *ForceMetadata) CreateCustomObject(object string) (err error) {
	soap := `
		<metadata xsi:type="CustomObject" xmlns:cmd="http://soap.sforce.com/2006/04/metadata">
			<fullName>%s__c</fullName>
			<label>%s</label>
			<pluralLabel>%s</pluralLabel>
			<deploymentStatus>Deployed</deploymentStatus>
			<sharingModel>ReadWrite</sharingModel>
			<nameField>
				<label>ID</label>
				<type>AutoNumber</type>
			</nameField>
		</metadata>
	`
	body, err := fm.soapExecute("create", fmt.Sprintf(soap, object, object, inflect.Pluralize(object)))
	if err != nil {
		return err
	}
	var status struct {
		Id string `xml:"Body>createResponse>result>id"`
	}
	if err = xml.Unmarshal(body, &status); err != nil {
		return
	}
	if err = fm.CheckStatus(status.Id); err != nil {
		return
	}
	return
}

func (fm *ForceMetadata) DeleteCustomObject(object string) (err error) {
	soap := `
		<metadata xsi:type="CustomObject" xmlns:cmd="http://soap.sforce.com/2006/04/metadata">
			<fullName>%s</fullName>
		</metadata>
	`
	body, err := fm.soapExecute("delete", fmt.Sprintf(soap, object))
	if err != nil {
		return err
	}
	var status struct {
		Id string `xml:"Body>deleteResponse>result>id"`
	}
	if err = xml.Unmarshal(body, &status); err != nil {
		return
	}
	if err = fm.CheckStatus(status.Id); err != nil {
		return
	}
	return
}

func (fm *ForceMetadata) ListMetadata(query string) (res []byte, err error) {
	return fm.soapExecute("listMetadata", fmt.Sprintf("<queries><type>%s</type></queries>", query))
}

func (fm *ForceMetadata) ListConnectedApps() (apps ForceConnectedApps, err error) {
	body, err := fm.ListMetadata("ConnectedApp")
	if err != nil {
		return
	}
	var res struct {
		ConnectedApps []ForceConnectedApp `xml:"Body>listMetadataResponse>result"`
	}
	if err = xml.Unmarshal(body, &res); err != nil {
		return
	}
	apps = res.ConnectedApps
	return
}

func (fm *ForceMetadata) soapExecute(action, query string) (response []byte, err error) {
	soap := `
		<env:Envelope xmlns:xsd="http://www.w3.org/2001/XMLSchema" xmlns:xsi="http://www.w3.org/2001/XMLSchema-instance" xmlns:tns="http://soap.sforce.com/2006/04/metadata" xmlns:env="http://schemas.xmlsoap.org/soap/envelope/" xmlns:cmd="http://soap.sforce.com/2006/04/metadata">
			<env:Header>
				<cmd:SessionHeader>
					<cmd:sessionId>%s</cmd:sessionId>
				</cmd:SessionHeader>
			</env:Header>
			<env:Body>
				<%s xmlns="http://soap.sforce.com/2006/04/metadata">
					%s
				</%s>
			</env:Body>
		</env:Envelope>
	`
	login, err := fm.Force.Get(fm.Force.Credentials.Id)
	if err != nil {
		return
	}
	url := strings.Replace(login["urls"].(map[string]interface{})["metadata"].(string), "{version}", "28.0", 1)
	rbody := fmt.Sprintf(soap, fm.Force.Credentials.AccessToken, action, query, action)
	/* fmt.Println("rbody", rbody)*/
	req, err := httpRequest("POST", url, strings.NewReader(rbody))
	if err != nil {
		return
	}
	req.Header.Add("Content-Type", "text/xml")
	req.Header.Add("SOAPACtion", action)
	res, err := httpClient().Do(req)
	defer res.Body.Close()
	if err != nil {
		return
	}
	if res.StatusCode == 401 {
		err = errors.New("authorization expired, please run `force login`")
		return
	}
	response, err = ioutil.ReadAll(res.Body)
	if err != nil {
		return
	}
	err = fm.soapError(response)
	return
}

func (fm *ForceMetadata) soapError(body []byte) (err error) {
	var soapError SoapError
	xml.Unmarshal(body, &soapError)
	if soapError.FaultCode != "" {
		return errors.New(soapError.FaultString)
	}
	return
}

/* func (fm *ForceMetadata) CreateConnectedApp(name, callback string) (app string, err error) {*/
/*   soap := `*/
/*     <env:Envelope xmlns:xsd="http://www.w3.org/2001/XMLSchema" xmlns:xsi="http://www.w3.org/2001/XMLSchema-instance" xmlns:tns="http://soap.sforce.com/2006/04/metadata" xmlns:env="http://schemas.xmlsoap.org/soap/envelope/" xmlns:cmd="http://soap.sforce.com/2006/04/metadata">*/
/*       <env:Header>*/
/*         <cmd:SessionHeader>*/
/*           <cmd:sessionId>%s</cmd:sessionId>*/
/*         </cmd:SessionHeader>*/
/*       </env:Header>*/
/*       <env:Body>*/
/*         <create xmlns="http://soap.sforce.com/2006/04/metadata">*/
/*           <metadata xsi:type="cmd:ConnectedApp" xmlns:cmd="http://soap.sforce.com/2006/04/metadata">*/
/*             <fullName>%s</fullName>*/
/*             <version>29.0</version>*/
/*             <label>%s</label>*/
/*             <contactEmail>%s</contactEmail>*/
/*             <oauthConfig>*/
/*               <callbackUrl>%s</callbackUrl>*/
/*               <scopes>Full</scopes>*/
/*               <scopes>RefreshToken</scopes>*/
/*               <consumerSecrett>Zm9sdfjsdkfjsdfkjv</consumerSecrett>*/
/*             </oauthConfig>*/
/*           </metadata>*/
/*         </create>*/
/*       </env:Body>*/
/*     </env:Envelope>*/
/*   `*/
/*   login, err := fm.Force.Get(fm.Force.Credentials.Id)*/
/*   if err != nil {*/
/*     return*/
/*   }*/
/*   url := strings.Replace(login["urls"].(map[string]interface{})["metadata"].(string), "{version}", "29.0", 1)*/
/*   rbody := fmt.Sprintf(soap, fm.Force.Credentials.AccessToken, name, name, "foo@bar.org", callback)*/
/*   fmt.Println("rbody", rbody)*/
/*   req, err := httpRequest("POST", url, strings.NewReader(rbody))*/
/*   if err != nil {*/
/*     return*/
/*   }*/
/*   req.Header.Add("Content-Type", "text/xml")*/
/*   req.Header.Add("SOAPACtion", "create")*/
/*   res, err := httpClient().Do(req)*/
/*   defer res.Body.Close()*/
/*   if err != nil {*/
/*     return*/
/*   }*/
/*   if res.StatusCode == 401 {*/
/*     err = errors.New("authorization expired, please run `force login`")*/
/*     return*/
/*   }*/
/*   body, err := ioutil.ReadAll(res.Body)*/
/*   var create SoapCreateResponse*/
/*   err = xml.Unmarshal(body, &create)*/
/*   if err != nil {*/
/*     return*/
/*   }*/
/*   err = fm.CheckStatus(create.Id)*/
/*   if err != nil {*/
/*     return*/
/*   }*/
/*   return*/
/* }*/

/* func (fm *ForceMetadata) GetConnectedApp(name string) (app ForceConnectedApp, err error) {*/
/*   soap := `*/
/*     <env:Envelope xmlns:xsd="http://www.w3.org/2001/XMLSchema" xmlns:xsi="http://www.w3.org/2001/XMLSchema-instance" xmlns:tns="http://soap.sforce.com/2006/04/metadata" xmlns:env="http://schemas.xmlsoap.org/soap/envelope/" xmlns:cmd="http://soap.sforce.com/2006/04/metadata">*/
/*       <env:Header>*/
/*         <cmd:SessionHeader>*/
/*           <cmd:sessionId>%s</cmd:sessionId>*/
/*         </cmd:SessionHeader>*/
/*       </env:Header>*/
/*       <env:Body>*/
/*         <retrieve xmlns="http://soap.sforce.com/2006/04/metadata">*/
/*         </retrieve>*/
/*       </env:Body>*/
/*     </env:Envelope>*/
/*   `*/
/*   login, err := fm.Force.Get(fm.Force.Credentials.Id)*/
/*   if err != nil {*/
/*     return*/
/*   }*/
/*   url := strings.Replace(login["urls"].(map[string]interface{})["metadata"].(string), "{version}", "29.0", 1)*/
/*   rbody := fmt.Sprintf(soap, fm.Force.Credentials.AccessToken, name)*/
/*   fmt.Println("rbody", rbody)*/
/*   req, err := httpRequest("POST", url, strings.NewReader(rbody))*/
/*   if err != nil {*/
/*     return*/
/*   }*/
/*   req.Header.Add("Content-Type", "text/xml")*/
/*   req.Header.Add("SOAPACtion", "retrieve")*/
/*   res, err := httpClient().Do(req)*/
/*   defer res.Body.Close()*/
/*   if err != nil {*/
/*     return*/
/*   }*/
/*   if res.StatusCode == 401 {*/
/*     err = errors.New("authorization expired, please run `force login`")*/
/*     return*/
/*   }*/
/*   body, err := ioutil.ReadAll(res.Body)*/
/*   var retrieve SoapRetrieveResponse*/
/*   xml.Unmarshal(body, &retrieve)*/
/*   err = fm.CheckStatus(retrieve.Id)*/
/*   if err != nil {*/
/*     return*/
/*   }*/
/*   err = fm.CheckRetrieveStatus(retrieve.Id)*/
/*   fmt.Println("err", err)*/
/*   [> var resp SoapListConnectedAppsResponse<]*/
/*   [> xml.Unmarshal(body, &resp)<]*/
/*   [> for _, app := range resp.ConnectedApps {<]*/
/*   [>   apps = append(apps, ForceConnectedApp{Name:app.Name, Id:app.Id})<]*/
/*   [> }<]*/
/*   return*/
/* }*/

/* func (fm *ForceMetadata) CheckRetrieveStatus(id string) (err error) {*/
/*   soap := `*/
/*     <env:Envelope xmlns:xsd="http://www.w3.org/2001/XMLSchema" xmlns:xsi="http://www.w3.org/2001/XMLSchema-instance" xmlns:tns="http://soap.sforce.com/2006/04/metadata" xmlns:env="http://schemas.xmlsoap.org/soap/envelope/" xmlns:cmd="http://soap.sforce.com/2006/04/metadata">*/
/*       <env:Header>*/
/*         <cmd:SessionHeader>*/
/*           <cmd:sessionId>%s</cmd:sessionId>*/
/*         </cmd:SessionHeader>*/
/*       </env:Header>*/
/*       <env:Body>*/
/*         <checkRetrieveStatus xmlns="http://soap.sforce.com/2006/04/metadata">*/
/*           <id>%s</id>*/
/*         </checkRetrieveStatus>*/
/*       </env:Body>*/
/*     </env:Envelope>*/
/*   `*/
/*   login, err := fm.Force.Get(fm.Force.Credentials.Id)*/
/*   if err != nil {*/
/*     return*/
/*   }*/
/*   url := strings.Replace(login["urls"].(map[string]interface{})["metadata"].(string), "{version}", "29.0", 1)*/
/*   rbody := fmt.Sprintf(soap, fm.Force.Credentials.AccessToken, id)*/
/*   req, err := httpRequest("POST", url, strings.NewReader(rbody))*/
/*   if err != nil {*/
/*     return*/
/*   }*/
/*   req.Header.Add("Content-Type", "text/xml")*/
/*   req.Header.Add("SOAPACtion", "checkRetrieveStatus")*/
/*   res, err := httpClient().Do(req)*/
/*   defer res.Body.Close()*/
/*   if err != nil {*/
/*     return*/
/*   }*/
/*   if res.StatusCode == 401 {*/
/*     err = errors.New("authorization expired, please run `force login`")*/
/*     return*/
/*   }*/
/*   body, err := ioutil.ReadAll(res.Body)*/
/*   var status SoapCheckRetrieveStatusResponse*/
/*   err = xml.Unmarshal(body, &status)*/
/*   if err != nil {*/
/*     return*/
/*   }*/
/*   zipfile, err := base64.StdEncoding.DecodeString(status.ZipFile)*/
/*   if err != nil {*/
/*     return*/
/*   }*/
/*   files, err := zip.NewReader(bytes.NewReader(zipfile), int64(len(zipfile)))*/
/*   if err != nil {*/
/*     return*/
/*   }*/
/*   for _, file := range files.File {*/
/*     if strings.Contains(file.Name, "connectedApp") {*/
/*       rc, _ := file.Open()*/
/*       defer rc.Close()*/
/*       bytes, _ := ioutil.ReadAll(rc)*/
/*       fmt.Println("bytes", string(bytes))*/
/*     }*/
/*   }*/
/*   return*/
/* }*/

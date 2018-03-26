package lib

import (
	"bytes"
	"encoding/json"
	"errors"
	"io/ioutil"
	"net/url"
	"os"
	"time"

	"github.com/dgrijalva/jwt-go"
)

func JwtAssertion(endpoint ForceEndpoint, username string, keyfile string, clientId string) (signedToken string, err error) {
	keyData, err := ioutil.ReadFile(keyfile)
	if err != nil {
		return
	}
	key, err := jwt.ParseRSAPrivateKeyFromPEM(keyData)
	if err != nil {
		return
	}

	tokenURL, err := tokenURL(endpoint)
	if err != nil {
		return
	}
	token := jwt.NewWithClaims(jwt.SigningMethodRS256, jwt.MapClaims{
		"iss": clientId,
		"sub": username,
		"aud": tokenURL,
		"exp": time.Now().Add(time.Minute * 3).Unix(),
	})
	signedToken, err = token.SignedString(key)
	return
}

func JWTLogin(endpoint ForceEndpoint, assertion string) (creds ForceSession, err error) {
	if err != nil {
		return
	}
	attrs := url.Values{}
	attrs.Set("grant_type", "urn:ietf:params:oauth:grant-type:jwt-bearer")
	attrs.Set("assertion", assertion)

	postVars := attrs.Encode()
	tokenURL, err := tokenURL(endpoint)
	if err != nil {
		return
	}
	req, err := httpRequest("POST", tokenURL, bytes.NewReader([]byte(postVars)))
	if err != nil {
		return
	}
	req.Header.Add("Content-Type", "application/x-www-form-urlencoded")
	res, err := doRequest(req)
	if err != nil {
		return
	}
	defer res.Body.Close()
	body, err := ioutil.ReadAll(res.Body)
	if err != nil {
		return
	}

	if res.StatusCode != 200 {
		var oauthError *OAuthError
		err = json.Unmarshal(body, &oauthError)
		if err != nil {
			return
		}
		err = errors.New(oauthError.ErrorDescription)
		return
	}

	err = json.Unmarshal(body, &creds)
	creds.SessionOptions = &SessionOptions{}
	creds.ForceEndpoint = endpoint
	creds.ClientId = ClientId
	return
}

func ForceLoginAndSaveJWT(endpoint ForceEndpoint, assertion string, output *os.File) (username string, err error) {
	creds, err := JWTLogin(endpoint, assertion)
	if err != nil {
		return
	}
	username, err = ForceSaveLogin(creds, output)
	return
}

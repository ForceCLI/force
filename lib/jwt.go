package lib

import (
	"bytes"
	"encoding/json"
	"errors"
	"io"
	"net/url"
	"os"
	"time"

	"github.com/golang-jwt/jwt/v4"
)

func JwtAssertion(endpoint ForceEndpoint, username string, keyfile string, clientId string) (signedToken string, err error) {
	Log.Info("Deprecated call to JwtAssertion.  Use JwtAssertionForEndpoint.")
	url := endpointUrl(endpoint)
	return JwtAssertionForEndpoint(url, username, keyfile, clientId)
}

func JwtAssertionForEndpoint(endpoint string, username string, keyfile string, clientId string) (signedToken string, err error) {
	keyData, err := os.ReadFile(keyfile)
	if err != nil {
		return
	}
	key, err := jwt.ParseRSAPrivateKeyFromPEM(keyData)
	if err != nil {
		return
	}

	tokenURL := tokenURL(endpoint)
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
	Log.Info("Deprecated call to JWTLogin.  Use JWTLoginAtEndpoint.")
	url := endpointUrl(endpoint)
	return JWTLoginAtEndpoint(url, assertion)
}

func JWTLoginAtEndpoint(endpoint string, assertion string) (creds ForceSession, err error) {
	attrs := url.Values{}
	attrs.Set("grant_type", "urn:ietf:params:oauth:grant-type:jwt-bearer")
	attrs.Set("assertion", assertion)

	postVars := attrs.Encode()
	tokenURL := tokenURL(endpoint)
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
	body, err := io.ReadAll(res.Body)
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
	creds.EndpointUrl = endpoint
	creds.ClientId = ClientId
	return
}

func ForceLoginAndSaveJWT(endpoint ForceEndpoint, assertion string, output *os.File) (username string, err error) {
	Log.Info("Deprecated call to ForceLoginAndSaveJWT.  Use ForceLoginAtEndpointAndSaveJWT.")
	url := endpointUrl(endpoint)
	return ForceLoginAtEndpointAndSaveJWT(url, assertion, output)
}

func ForceLoginAtEndpointAndSaveJWT(endpoint string, assertion string, output *os.File) (username string, err error) {
	creds, err := JWTLoginAtEndpoint(endpoint, assertion)
	if err != nil {
		return
	}
	username, err = ForceSaveLogin(creds, output)
	return
}

func ClientCredentialsLoginAtEndpoint(endpoint string, clientId string, clientSecret string) (creds ForceSession, err error) {
	attrs := url.Values{}
	attrs.Set("grant_type", "client_credentials")
	attrs.Set("client_id", clientId)
	attrs.Set("client_secret", clientSecret)

	postVars := attrs.Encode()
	tokenURL := tokenURL(endpoint)
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
	body, err := io.ReadAll(res.Body)
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
	creds.EndpointUrl = endpoint
	creds.ClientId = ClientId
	return
}

func ForceLoginAtEndpointAndSaveClientCredentials(endpoint string, clientId string, clientSecret string, output *os.File) (username string, err error) {
	creds, err := ClientCredentialsLoginAtEndpoint(endpoint, clientId, clientSecret)
	if err != nil {
		return
	}
	username, err = ForceSaveLogin(creds, output)
	return
}

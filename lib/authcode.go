package lib

import (
	"bytes"
	"context"
	"crypto/rand"
	"crypto/sha256"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net"
	"net/http"
	"net/url"
	"sync"
	"time"
)

// authorizationCodeResult is what the local OAuth callback server receives
// from Salesforce: either an authorization code or an error.
type authorizationCodeResult struct {
	code string
	err  error
}

type authorizationCodeCallbackServer struct {
	port    int
	results chan authorizationCodeResult
	server  *http.Server
}

// DefaultOAuthCallbackPort is the port the local OAuth callback server listens
// on unless another port is given.
const DefaultOAuthCallbackPort = 3835

// OAuthCallbackURL returns the redirect URI for the local OAuth callback
// server. A connected app or external client app used for browser login must
// list this URL as a callback URL.
func OAuthCallbackURL(port int) string {
	return fmt.Sprintf("http://localhost:%d/oauth/callback", port)
}

// randomURLSafeString returns n random bytes encoded as unpadded base64url.
func randomURLSafeString(n int) (string, error) {
	b := make([]byte, n)
	if _, err := rand.Read(b); err != nil {
		return "", err
	}
	return base64.RawURLEncoding.EncodeToString(b), nil
}

// pkceCodeChallenge returns the S256 code challenge for a PKCE code verifier
// (RFC 7636 section 4.2).
func pkceCodeChallenge(verifier string) string {
	sum := sha256.Sum256([]byte(verifier))
	return base64.RawURLEncoding.EncodeToString(sum[:])
}

func authorizationCodeURL(endpoint, clientId, redirectURI, state, prompt, codeChallenge string) string {
	params := url.Values{}
	params.Set("response_type", "code")
	params.Set("client_id", clientId)
	params.Set("redirect_uri", redirectURI)
	params.Set("state", state)
	params.Set("prompt", prompt)
	params.Set("code_challenge", codeChallenge)
	params.Set("code_challenge_method", "S256")
	return fmt.Sprintf("%s/services/oauth2/authorize?%s", endpoint, params.Encode())
}

// startAuthorizationCodeCallbackServer listens on targetPort (0 picks a free
// port) for the redirect from the Salesforce authorize endpoint. Requests
// whose state does not match are rejected and do not end the login, so only
// the browser session that started the login can complete it.
func startAuthorizationCodeCallbackServer(targetPort int, state string) (*authorizationCodeCallbackServer, error) {
	listener, err := net.Listen("tcp", fmt.Sprintf(":%d", targetPort))
	if err != nil {
		return nil, err
	}
	results := make(chan authorizationCodeResult, 1)
	var once sync.Once
	send := func(r authorizationCodeResult) {
		once.Do(func() { results <- r })
	}
	h := http.NewServeMux()
	h.HandleFunc("/oauth/callback", func(w http.ResponseWriter, r *http.Request) {
		query := r.URL.Query()
		if query.Get("state") != state {
			http.Error(w, "OAuth state does not match", http.StatusBadRequest)
			return
		}
		if query.Get("error") != "" {
			oauthCallbackError(w, query.Get("error"), query.Get("error_description"))
			send(authorizationCodeResult{err: fmt.Errorf("authorization failed: %s: %s", query.Get("error"), query.Get("error_description"))})
			return
		}
		code := query.Get("code")
		if code == "" {
			http.Error(w, "OAuth callback is missing the authorization code", http.StatusBadRequest)
			return
		}
		io.WriteString(w, oauthCallbackHtml())
		send(authorizationCodeResult{code: code})
	})
	server := &http.Server{Handler: h}
	go server.Serve(listener)
	return &authorizationCodeCallbackServer{
		port:    listener.Addr().(*net.TCPAddr).Port,
		results: results,
		server:  server,
	}, nil
}

// close stops the callback server after letting in-flight responses finish.
func (s *authorizationCodeCallbackServer) close() {
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	s.server.Shutdown(ctx)
}

// exchangeAuthorizationCode trades an authorization code and its PKCE code
// verifier for an access token and refresh token.
func exchangeAuthorizationCode(endpoint, clientId, code, codeVerifier, redirectURI string) (creds ForceSession, err error) {
	attrs := url.Values{}
	attrs.Set("grant_type", "authorization_code")
	attrs.Set("client_id", clientId)
	attrs.Set("code", code)
	attrs.Set("code_verifier", codeVerifier)
	attrs.Set("redirect_uri", redirectURI)

	req, err := httpRequest("POST", tokenURL(endpoint), bytes.NewReader([]byte(attrs.Encode())))
	if err != nil {
		return
	}
	req.Header.Add("Content-Type", "application/x-www-form-urlencoded")
	// A Lightning Experience URL (*.lightning.force.com) answers with a 302 to
	// the My Domain URL, so the POST has to be re-sent there.
	res, err := doRequest(req, redirectPostOn302)
	if err != nil {
		return
	}
	defer res.Body.Close()
	body, err := io.ReadAll(res.Body)
	if err != nil {
		return
	}

	if res.StatusCode != 200 {
		var oauthError OAuthError
		if jsonErr := json.Unmarshal(body, &oauthError); jsonErr != nil {
			err = fmt.Errorf("authorization code exchange failed: %s", res.Status)
			return
		}
		err = errors.New("authorization code exchange failed: " + oauthError.ErrorDescription)
		return
	}

	if err = json.Unmarshal(body, &creds); err != nil {
		return
	}
	var tokenResponse struct {
		RefreshToken string `json:"refresh_token"`
	}
	if err = json.Unmarshal(body, &tokenResponse); err != nil {
		return
	}
	creds.RefreshToken = tokenResponse.RefreshToken
	creds.SessionOptions = &SessionOptions{}
	if creds.RefreshToken != "" {
		creds.SessionOptions.RefreshMethod = RefreshOauth
	}
	creds.EndpointUrl = endpoint
	creds.ClientId = clientId
	return
}

func oauthCallbackHtml() string {
	return `
<!doctype html>
<html>
  <head>
	  <title>Force CLI OAuth Callback</title>
  </head>
  <body>
	  <h1>OAuth Callback</h1>
	  <p id="status">Complete! You may now close this window</p>
  </body>
</html>`
}

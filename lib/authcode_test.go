package lib

import (
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"testing"
	"time"
)

func TestPkceCodeChallenge_MatchesRFC7636Example(t *testing.T) {
	// Example from RFC 7636 Appendix B
	verifier := "dBjftJeZ4CVP-mB92K27uhbUJU1p1r_wW1gFWFOEjXk"
	expected := "E9Melhoa2OwvFrEMTJguCHaoeK1t8URWbuGJSstw-cM"
	if got := pkceCodeChallenge(verifier); got != expected {
		t.Errorf("Expected code challenge %s, got %s", expected, got)
	}
}

func TestRandomURLSafeString_ProducesVerifierOfValidLength(t *testing.T) {
	verifier, err := randomURLSafeString(32)
	if err != nil {
		t.Fatalf("randomURLSafeString returned error: %v", err)
	}
	// RFC 7636 requires a code verifier of 43 to 128 characters
	if len(verifier) < 43 || len(verifier) > 128 {
		t.Errorf("Expected verifier length between 43 and 128, got %d", len(verifier))
	}
	other, _ := randomURLSafeString(32)
	if verifier == other {
		t.Errorf("Expected two random strings to differ")
	}
}

func TestAuthorizationCodeURL_RequestsCodeWithPKCE(t *testing.T) {
	got := authorizationCodeURL("https://example.my.salesforce.com", "client", "http://localhost:3835/oauth/callback", "the-state", "login", "the-challenge")
	u, err := url.Parse(got)
	if err != nil {
		t.Fatalf("Failed to parse URL: %v", err)
	}
	if u.Path != "/services/oauth2/authorize" {
		t.Errorf("Expected authorize path, got %s", u.Path)
	}
	expected := map[string]string{
		"response_type":         "code",
		"client_id":             "client",
		"redirect_uri":          "http://localhost:3835/oauth/callback",
		"state":                 "the-state",
		"prompt":                "login",
		"code_challenge":        "the-challenge",
		"code_challenge_method": "S256",
	}
	for k, v := range expected {
		if u.Query().Get(k) != v {
			t.Errorf("Expected %s=%s, got %s", k, v, u.Query().Get(k))
		}
	}
}

func startTestCallbackServer(t *testing.T, state string) *authorizationCodeCallbackServer {
	t.Helper()
	callback, err := startAuthorizationCodeCallbackServer(0, state)
	if err != nil {
		t.Fatalf("Failed to start callback server: %v", err)
	}
	t.Cleanup(callback.close)
	return callback
}

func callbackGet(t *testing.T, callback *authorizationCodeCallbackServer, query url.Values) *http.Response {
	t.Helper()
	res, err := http.Get(fmt.Sprintf("http://localhost:%d/oauth/callback?%s", callback.port, query.Encode()))
	if err != nil {
		t.Fatalf("Callback request failed: %v", err)
	}
	res.Body.Close()
	return res
}

func receiveResult(t *testing.T, callback *authorizationCodeCallbackServer) authorizationCodeResult {
	t.Helper()
	select {
	case result := <-callback.results:
		return result
	case <-time.After(5 * time.Second):
		t.Fatal("Timed out waiting for callback result")
	}
	return authorizationCodeResult{}
}

func TestAuthorizationCodeCallbackServer_ReturnsCodeWhenStateMatches(t *testing.T) {
	callback := startTestCallbackServer(t, "expected-state")

	res := callbackGet(t, callback, url.Values{"state": {"expected-state"}, "code": {"the-code"}})
	if res.StatusCode != http.StatusOK {
		t.Errorf("Expected status 200, got %d", res.StatusCode)
	}
	result := receiveResult(t, callback)
	if result.err != nil {
		t.Fatalf("Expected no error, got %v", result.err)
	}
	if result.code != "the-code" {
		t.Errorf("Expected code the-code, got %s", result.code)
	}
}

func TestAuthorizationCodeCallbackServer_IgnoresRequestWithWrongState(t *testing.T) {
	callback := startTestCallbackServer(t, "expected-state")

	res := callbackGet(t, callback, url.Values{"state": {"other-state"}, "code": {"forged-code"}})
	if res.StatusCode != http.StatusBadRequest {
		t.Errorf("Expected status 400, got %d", res.StatusCode)
	}

	callbackGet(t, callback, url.Values{"state": {"expected-state"}, "code": {"the-code"}})
	result := receiveResult(t, callback)
	if result.code != "the-code" {
		t.Errorf("Expected code the-code, got %q", result.code)
	}
}

func TestAuthorizationCodeCallbackServer_ReturnsErrorFromSalesforce(t *testing.T) {
	callback := startTestCallbackServer(t, "expected-state")

	callbackGet(t, callback, url.Values{
		"state":             {"expected-state"},
		"error":             {"access_denied"},
		"error_description": {"end-user denied authorization"},
	})
	result := receiveResult(t, callback)
	if result.err == nil {
		t.Fatal("Expected an error")
	}
	if !strings.Contains(result.err.Error(), "end-user denied authorization") {
		t.Errorf("Expected error to include description, got %v", result.err)
	}
}

func TestExchangeAuthorizationCode_Success(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/services/oauth2/token" {
			t.Errorf("Expected token path, got %s", r.URL.Path)
		}
		if err := r.ParseForm(); err != nil {
			t.Fatalf("Failed to parse form: %v", err)
		}
		expected := map[string]string{
			"grant_type":    "authorization_code",
			"client_id":     "test-client-id",
			"code":          "the-code",
			"code_verifier": "the-verifier",
			"redirect_uri":  "http://localhost:3835/oauth/callback",
		}
		for k, v := range expected {
			if r.Form.Get(k) != v {
				t.Errorf("Expected %s=%s, got %s", k, v, r.Form.Get(k))
			}
		}
		json.NewEncoder(w).Encode(map[string]interface{}{
			"access_token":  "test-access-token",
			"refresh_token": "test-refresh-token",
			"instance_url":  "https://na1.salesforce.com",
			"issued_at":     "1234567890",
			"scope":         "full refresh_token",
		})
	}))
	defer server.Close()

	creds, err := exchangeAuthorizationCode(server.URL, "test-client-id", "the-code", "the-verifier", "http://localhost:3835/oauth/callback")
	if err != nil {
		t.Fatalf("exchangeAuthorizationCode returned error: %v", err)
	}
	if creds.AccessToken != "test-access-token" {
		t.Errorf("Expected access token test-access-token, got %s", creds.AccessToken)
	}
	if creds.RefreshToken != "test-refresh-token" {
		t.Errorf("Expected refresh token test-refresh-token, got %s", creds.RefreshToken)
	}
	if creds.InstanceUrl != "https://na1.salesforce.com" {
		t.Errorf("Expected instance URL https://na1.salesforce.com, got %s", creds.InstanceUrl)
	}
	if creds.SessionOptions == nil || creds.SessionOptions.RefreshMethod != RefreshOauth {
		t.Errorf("Expected OAuth refresh method")
	}
	if creds.EndpointUrl != server.URL {
		t.Errorf("Expected endpoint URL %s, got %s", server.URL, creds.EndpointUrl)
	}
	if creds.ClientId != "test-client-id" {
		t.Errorf("Expected client id test-client-id, got %s", creds.ClientId)
	}
}

func TestExchangeAuthorizationCode_ResendsPostAfterRedirect(t *testing.T) {
	myDomain := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			t.Errorf("Expected POST after redirect, got %s", r.Method)
		}
		if err := r.ParseForm(); err != nil {
			t.Fatalf("Failed to parse form: %v", err)
		}
		if r.Form.Get("grant_type") != "authorization_code" {
			t.Errorf("Expected grant_type=authorization_code after redirect, got %q", r.Form.Get("grant_type"))
		}
		if r.Form.Get("code_verifier") != "the-verifier" {
			t.Errorf("Expected code_verifier=the-verifier after redirect, got %q", r.Form.Get("code_verifier"))
		}
		json.NewEncoder(w).Encode(map[string]interface{}{
			"access_token": "test-access-token",
			"instance_url": "https://example.my.salesforce.com",
		})
	}))
	defer myDomain.Close()
	lightning := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		http.Redirect(w, r, myDomain.URL+"/services/oauth2/token", http.StatusFound)
	}))
	defer lightning.Close()

	creds, err := exchangeAuthorizationCode(lightning.URL, "test-client-id", "the-code", "the-verifier", "http://localhost:3835/oauth/callback")
	if err != nil {
		t.Fatalf("exchangeAuthorizationCode returned error: %v", err)
	}
	if creds.AccessToken != "test-access-token" {
		t.Errorf("Expected access token test-access-token, got %s", creds.AccessToken)
	}
}

func TestExchangeAuthorizationCode_ReturnsOAuthErrorDescription(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusBadRequest)
		json.NewEncoder(w).Encode(map[string]string{
			"error":             "invalid_grant",
			"error_description": "invalid code verifier",
		})
	}))
	defer server.Close()

	_, err := exchangeAuthorizationCode(server.URL, "test-client-id", "the-code", "wrong-verifier", "http://localhost:3835/oauth/callback")
	if err == nil {
		t.Fatal("Expected an error")
	}
	if !strings.Contains(err.Error(), "invalid code verifier") {
		t.Errorf("Expected error to include description, got %v", err)
	}
}

func TestOAuthCallbackURL_UsesPort(t *testing.T) {
	if got := OAuthCallbackURL(DefaultOAuthCallbackPort); got != "http://localhost:3835/oauth/callback" {
		t.Errorf("Expected http://localhost:3835/oauth/callback, got %s", got)
	}
}

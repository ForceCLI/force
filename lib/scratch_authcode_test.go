package lib

import (
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestSessionFromAuthCodeExchangesTheCodeAtTheLoginURL(t *testing.T) {
	var form map[string][]string
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/services/oauth2/token" {
			t.Errorf("path = %s", r.URL.Path)
		}
		r.ParseForm()
		form = r.PostForm
		if r.PostForm.Get("code") == "bad" {
			w.WriteHeader(http.StatusBadRequest)
			w.Write([]byte(`{"error":"invalid_grant","error_description":"expired authorization code"}`))
			return
		}
		w.Write([]byte(`{"access_token":"token","instance_url":"https://sandbox.example.com","refresh_token":"refresh"}`))
	}))
	defer server.Close()

	session, err := SessionFromAuthCode(server.URL+"/", "code1", "PlatformCLI", "http://localhost:1717/OauthRedirect")
	if err != nil {
		t.Fatal(err)
	}
	if session.AccessToken != "token" || session.RefreshToken != "refresh" || session.InstanceUrl != "https://sandbox.example.com" || session.ClientId != "PlatformCLI" {
		t.Errorf("session = %+v", session)
	}
	if form["grant_type"][0] != "authorization_code" || form["code"][0] != "code1" || form["client_id"][0] != "PlatformCLI" || form["redirect_uri"][0] != "http://localhost:1717/OauthRedirect" {
		t.Errorf("form = %v", form)
	}
	if _, err := SessionFromAuthCode(server.URL, "bad", "PlatformCLI", "http://localhost:1717/OauthRedirect"); err == nil || err.Error() != "Unable to exchange the authorization code: expired authorization code" {
		t.Errorf("error = %v", err)
	}
}

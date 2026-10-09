package lib

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
)

// newRedirectingServer answers every request with a 302 to the same path on
// target, as a *.lightning.force.com URL does for the My Domain URL.
func newRedirectingServer(target *httptest.Server) *httptest.Server {
	return httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		http.Redirect(w, r, target.URL+r.URL.Path, http.StatusFound)
	}))
}

func TestForceLoginAtEndpointDeviceFlow_ResendsPostsAfterRedirect(t *testing.T) {
	myDomain := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			t.Errorf("Expected POST after redirect, got %s", r.Method)
		}
		if err := r.ParseForm(); err != nil {
			t.Fatalf("Failed to parse form: %v", err)
		}
		switch {
		case r.Form.Get("response_type") == "device_code":
			json.NewEncoder(w).Encode(map[string]interface{}{
				"device_code":      "the-device-code",
				"user_code":        "ABCD-1234",
				"verification_uri": "https://example.my.salesforce.com/setup/connect",
				"interval":         1,
				"expires_in":       60,
			})
		case r.Form.Get("grant_type") == "device":
			if r.Form.Get("code") != "the-device-code" {
				t.Errorf("Expected code=the-device-code, got %q", r.Form.Get("code"))
			}
			json.NewEncoder(w).Encode(map[string]interface{}{
				"access_token":  "test-access-token",
				"refresh_token": "test-refresh-token",
				"instance_url":  "https://example.my.salesforce.com",
			})
		default:
			t.Errorf("Unexpected token request form: %v", r.Form)
			w.WriteHeader(http.StatusBadRequest)
			json.NewEncoder(w).Encode(map[string]string{"error": "unsupported_grant_type", "error_description": "grant type not supported"})
		}
	}))
	defer myDomain.Close()
	lightning := newRedirectingServer(myDomain)
	defer lightning.Close()

	creds, err := ForceLoginAtEndpointDeviceFlow(lightning.URL, "login")
	if err != nil {
		t.Fatalf("ForceLoginAtEndpointDeviceFlow returned error: %v", err)
	}
	if creds.AccessToken != "test-access-token" {
		t.Errorf("Expected access token test-access-token, got %s", creds.AccessToken)
	}
	if creds.RefreshToken != "test-refresh-token" {
		t.Errorf("Expected refresh token test-refresh-token, got %s", creds.RefreshToken)
	}
}

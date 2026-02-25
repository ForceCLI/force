package lib

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestPasswordFlowLoginAtEndpoint_Success(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != "POST" {
			t.Errorf("Expected POST request, got %s", r.Method)
		}

		expectedPath := "/services/oauth2/token"
		if r.URL.Path != expectedPath {
			t.Errorf("Expected path %s, got %s", expectedPath, r.URL.Path)
		}

		if r.Header.Get("Content-Type") != "application/x-www-form-urlencoded" {
			t.Errorf("Expected Content-Type application/x-www-form-urlencoded, got %s", r.Header.Get("Content-Type"))
		}

		err := r.ParseForm()
		if err != nil {
			t.Fatalf("Failed to parse form: %v", err)
		}

		if r.Form.Get("grant_type") != "password" {
			t.Errorf("Expected grant_type=password, got %s", r.Form.Get("grant_type"))
		}
		if r.Form.Get("client_id") != "test-client-id" {
			t.Errorf("Expected client_id=test-client-id, got %s", r.Form.Get("client_id"))
		}
		if r.Form.Get("username") != "user@example.com" {
			t.Errorf("Expected username=user@example.com, got %s", r.Form.Get("username"))
		}
		if r.Form.Get("password") != "secretpassword" {
			t.Errorf("Expected password=secretpassword, got %s", r.Form.Get("password"))
		}
		if _, ok := r.Form["client_secret"]; ok {
			t.Errorf("Expected no client_secret in form, got %s", r.Form.Get("client_secret"))
		}

		w.WriteHeader(http.StatusOK)
		json.NewEncoder(w).Encode(map[string]interface{}{
			"access_token":  "test-access-token",
			"refresh_token": "test-refresh-token",
			"instance_url":  "https://na1.salesforce.com",
			"issued_at":     "1234567890",
			"scope":         "full",
		})
	}))
	defer server.Close()

	creds, err := PasswordFlowLoginAtEndpoint(server.URL, "test-client-id", "", "user@example.com", "secretpassword")

	if err != nil {
		t.Fatalf("PasswordFlowLoginAtEndpoint returned error: %v", err)
	}

	if creds.AccessToken != "test-access-token" {
		t.Errorf("Expected AccessToken test-access-token, got %s", creds.AccessToken)
	}

	if creds.RefreshToken != "test-refresh-token" {
		t.Errorf("Expected RefreshToken test-refresh-token, got %s", creds.RefreshToken)
	}

	if creds.InstanceUrl != "https://na1.salesforce.com" {
		t.Errorf("Expected InstanceUrl https://na1.salesforce.com, got %s", creds.InstanceUrl)
	}

	if creds.SessionOptions == nil {
		t.Fatal("Expected SessionOptions to be set")
	}

	if creds.SessionOptions.RefreshMethod != RefreshOauth {
		t.Errorf("Expected RefreshMethod RefreshOauth, got %d", creds.SessionOptions.RefreshMethod)
	}

	if creds.EndpointUrl != server.URL {
		t.Errorf("Expected EndpointUrl %s, got %s", server.URL, creds.EndpointUrl)
	}

	if creds.ClientId != "test-client-id" {
		t.Errorf("Expected ClientId test-client-id, got %s", creds.ClientId)
	}
}

func TestPasswordFlowLoginAtEndpoint_NoRefreshToken(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		json.NewEncoder(w).Encode(map[string]interface{}{
			"access_token": "test-access-token",
			"instance_url": "https://na1.salesforce.com",
			"issued_at":    "1234567890",
		})
	}))
	defer server.Close()

	creds, err := PasswordFlowLoginAtEndpoint(server.URL, "test-client-id", "", "user@example.com", "secretpassword")

	if err != nil {
		t.Fatalf("PasswordFlowLoginAtEndpoint returned error: %v", err)
	}

	if creds.SessionOptions.RefreshMethod != RefreshUnavailable {
		t.Errorf("Expected RefreshMethod RefreshUnavailable when no refresh token, got %d", creds.SessionOptions.RefreshMethod)
	}
}

func TestPasswordFlowLoginAtEndpoint_Error(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusBadRequest)
		json.NewEncoder(w).Encode(map[string]interface{}{
			"error":             "invalid_client_id",
			"error_description": "client identifier invalid",
		})
	}))
	defer server.Close()

	_, err := PasswordFlowLoginAtEndpoint(server.URL, "test-client-id", "", "user@example.com", "wrongpassword")

	if err == nil {
		t.Fatal("Expected error for invalid credentials, got nil")
	}

	if err.Error() != "client identifier invalid" {
		t.Errorf("Expected error message 'client identifier invalid', got %q", err.Error())
	}
}

func TestPasswordFlowLoginAtEndpoint_AuthenticationFailureMentionsExternalClientApp(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusBadRequest)
		json.NewEncoder(w).Encode(map[string]interface{}{
			"error":             "invalid_grant",
			"error_description": "authentication failure",
		})
	}))
	defer server.Close()

	_, err := PasswordFlowLoginAtEndpoint(server.URL, "test-client-id", "test-client-secret", "user@example.com", "wrongpassword")

	if err == nil {
		t.Fatal("Expected error for invalid credentials, got nil")
	}

	expected := "authentication failure: Note that only Connected Apps support the username/password flow; External Client Apps do not"
	if err.Error() != expected {
		t.Errorf("Expected error message %q, got %q", expected, err.Error())
	}
}

func TestPasswordFlowLoginAtEndpoint_WithClientSecret(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		err := r.ParseForm()
		if err != nil {
			t.Fatalf("Failed to parse form: %v", err)
		}

		if r.Form.Get("grant_type") != "password" {
			t.Errorf("Expected grant_type=password, got %s", r.Form.Get("grant_type"))
		}
		if r.Form.Get("client_id") != "test-client-id" {
			t.Errorf("Expected client_id=test-client-id, got %s", r.Form.Get("client_id"))
		}
		if r.Form.Get("client_secret") != "test-client-secret" {
			t.Errorf("Expected client_secret=test-client-secret, got %s", r.Form.Get("client_secret"))
		}
		if r.Form.Get("username") != "user@example.com" {
			t.Errorf("Expected username=user@example.com, got %s", r.Form.Get("username"))
		}
		if r.Form.Get("password") != "secretpassword" {
			t.Errorf("Expected password=secretpassword, got %s", r.Form.Get("password"))
		}

		w.WriteHeader(http.StatusOK)
		json.NewEncoder(w).Encode(map[string]interface{}{
			"access_token": "test-access-token",
			"instance_url": "https://na1.salesforce.com",
			"issued_at":    "1234567890",
		})
	}))
	defer server.Close()

	_, err := PasswordFlowLoginAtEndpoint(server.URL, "test-client-id", "test-client-secret", "user@example.com", "secretpassword")
	if err != nil {
		t.Fatalf("PasswordFlowLoginAtEndpoint returned error: %v", err)
	}
}

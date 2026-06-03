package lib

import (
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestLatestApiVersion(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/services/data" {
			t.Errorf("Expected path /services/data, got %s", r.URL.Path)
		}
		w.WriteHeader(http.StatusOK)
		w.Write([]byte(`[
			{"label":"Winter '26","url":"/services/data/v65.0","version":"65.0"},
			{"label":"Spring '26","url":"/services/data/v66.0","version":"66.0"},
			{"label":"Summer '26","url":"/services/data/v67.0","version":"67.0"},
			{"label":"Old","url":"/services/data/v55.0","version":"55.0"}
		]`))
	}))
	defer server.Close()

	force := &Force{
		Credentials: &ForceSession{
			InstanceUrl: server.URL,
			AccessToken: "test-token",
		},
	}

	latest, err := force.LatestApiVersion()
	if err != nil {
		t.Fatalf("LatestApiVersion returned error: %v", err)
	}
	if latest != "67.0" {
		t.Errorf("Expected 67.0, got %s", latest)
	}
}

func TestLatestApiVersion_Empty(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		w.Write([]byte(`[]`))
	}))
	defer server.Close()

	force := &Force{
		Credentials: &ForceSession{
			InstanceUrl: server.URL,
			AccessToken: "test-token",
		},
	}

	_, err := force.LatestApiVersion()
	if err == nil {
		t.Error("Expected error when no versions returned, got nil")
	}
}

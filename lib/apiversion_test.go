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

func restoreApiVersion(t *testing.T) {
	t.Helper()
	original := ApiVersionNumber()
	t.Cleanup(func() {
		if err := SetApiVersion(original); err != nil {
			t.Fatal(err)
		}
	})
}

func TestUseLatestApiVersion(t *testing.T) {
	restoreApiVersion(t)
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Write([]byte(`[{"version":"67.0"},{"version":"69.0"},{"version":"68.0"}]`))
	}))
	defer server.Close()

	force := NewForce(&ForceSession{InstanceUrl: server.URL, AccessToken: "test-token"})

	if err := force.UseLatestApiVersion(); err != nil {
		t.Fatalf("UseLatestApiVersion returned error: %v", err)
	}
	if ApiVersionNumber() != "69.0" {
		t.Errorf("ApiVersionNumber() = %s, want 69.0", ApiVersionNumber())
	}
	if ApiVersion() != "v69.0" {
		t.Errorf("ApiVersion() = %s, want v69.0", ApiVersion())
	}
	if force.Metadata.ApiVersion != "69.0" {
		t.Errorf("Metadata.ApiVersion = %s, want 69.0", force.Metadata.ApiVersion)
	}
	if force.Credentials.SessionOptions.ApiVersion != "69.0" {
		t.Errorf("SessionOptions.ApiVersion = %s, want 69.0", force.Credentials.SessionOptions.ApiVersion)
	}
}

func TestUseLatestApiVersion_NoVersionsLeavesCurrentVersion(t *testing.T) {
	restoreApiVersion(t)
	if err := SetApiVersion("60.0"); err != nil {
		t.Fatal(err)
	}
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Write([]byte(`[]`))
	}))
	defer server.Close()

	force := NewForce(&ForceSession{InstanceUrl: server.URL, AccessToken: "test-token"})

	if err := force.UseLatestApiVersion(); err == nil {
		t.Fatal("expected an error when no versions are returned")
	}
	if ApiVersionNumber() != "60.0" {
		t.Errorf("ApiVersionNumber() = %s, want 60.0", ApiVersionNumber())
	}
}

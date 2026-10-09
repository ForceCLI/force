package lib

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
)

func newTestHttpRetrier(retryOnErrors ...func(res *http.Response, err error) bool) *HttpRetrier {
	return NewHttpRetrier(2, time.Duration(0), retryOnErrors...)

}

var _ = Describe("HttpRetrier", func() {
	It("retries at most max attempts", func() {
		calls := 0
		retrier := newTestHttpRetrier(
			func(res *http.Response, err error) bool {
				return err == nil
			},
			func(res *http.Response, err error) bool {
				calls += 1
				return err == err
			},
		)
		request := NewRequest("i am fundamentally wrong").RootUrl("wrong.com")
		force := (&Force{Credentials: &ForceSession{InstanceUrl: "instance.com"}}).WithRetrier(retrier)

		force.ExecuteRequest(request)

		Expect(calls).To(Equal(retrier.maxAttempts))
	})

	It("retries only on errors", func() {
		calls := 0
		retrier := newTestHttpRetrier(func(res *http.Response, err error) bool {
			calls += 1
			return err == nil
		})
		request := NewRequest("GET").RootUrl("/")
		force := (&Force{Credentials: &ForceSession{InstanceUrl: "https://google.com"}}).WithRetrier(retrier)

		force.ExecuteRequest(request)

		Expect(calls).To(Equal(0))
	})

	It("retries only on specified errors", func() {
		calls := 0
		retrier := newTestHttpRetrier(func(res *http.Response, err error) bool {
			calls += 1
			return err == nil
		})
		request := NewRequest("i am fundamentally wrong").RootUrl("wrong.com")
		force := (&Force{Credentials: &ForceSession{InstanceUrl: "instance.com"}}).WithRetrier(retrier)

		force.ExecuteRequest(request)

		Expect(calls).To(Equal(1))
	})

	It("retrier is not mutated", func() {
		retrier := newTestHttpRetrier()
		request := NewRequest("i am fundamentally wrong").RootUrl("wrong.com")
		force := (&Force{Credentials: &ForceSession{InstanceUrl: "instance.com"}}).WithRetrier(retrier)

		force.ExecuteRequest(request)

		Expect(force.retrier.attempt).To(Equal(0))
		Expect(retrier.attempt).To(Equal(0))
	})
})

func TestRedirectPostOn302_ResendsFormBodyAndContentType(t *testing.T) {
	target := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			t.Errorf("Expected POST after redirect, got %s", r.Method)
		}
		if ct := r.Header.Get("Content-Type"); ct != "application/x-www-form-urlencoded" {
			t.Errorf("Expected Content-Type application/x-www-form-urlencoded after redirect, got %q", ct)
		}
		if err := r.ParseForm(); err != nil {
			t.Fatalf("Failed to parse form: %v", err)
		}
		if r.Form.Get("grant_type") != "refresh_token" {
			t.Errorf("Expected grant_type=refresh_token after redirect, got %q", r.Form.Get("grant_type"))
		}
	}))
	defer target.Close()
	origin := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		http.Redirect(w, r, target.URL+"/services/oauth2/token", http.StatusFound)
	}))
	defer origin.Close()

	req, err := httpRequest("POST", origin.URL+"/services/oauth2/token", strings.NewReader("grant_type=refresh_token"))
	if err != nil {
		t.Fatalf("Failed to create request: %v", err)
	}
	req.Header.Add("Content-Type", "application/x-www-form-urlencoded")
	res, err := doRequest(req, redirectPostOn302)
	if err != nil {
		t.Fatalf("Request failed: %v", err)
	}
	res.Body.Close()
}

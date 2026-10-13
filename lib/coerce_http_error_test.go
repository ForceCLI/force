package lib

import (
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func coerceTestError(t *testing.T, status int, contentType, body string) error {
	t.Helper()
	recorder := httptest.NewRecorder()
	recorder.Header().Set("Content-Type", contentType)
	recorder.WriteHeader(status)
	return (&Force{})._coerceHttpError(recorder.Result(), []byte(body))
}

func TestCoerceHttpErrorReturnsTheErrorOfA403ThatIsNotAboutTheSession(t *testing.T) {
	err := coerceTestError(t, http.StatusForbidden, "application/json",
		`[{"message":"This feature is not currently enabled for this user.","errorCode":"FUNCTIONALITY_NOT_ENABLED"}]`)

	if errors.Is(err, SessionExpiredError) {
		t.Fatalf("expected the response's error, got %v", err)
	}
	var forceErrors ForceErrors
	if !errors.As(err, &forceErrors) || len(forceErrors) != 1 || forceErrors[0].ErrorCode != "FUNCTIONALITY_NOT_ENABLED" {
		t.Fatalf("expected FUNCTIONALITY_NOT_ENABLED, got %v", err)
	}
	if !strings.Contains(err.Error(), "This feature is not currently enabled for this user.") {
		t.Errorf("expected the response's message, got %q", err.Error())
	}
}

func TestCoerceHttpErrorTreatsA403InvalidSessionIdAsAnExpiredSession(t *testing.T) {
	err := coerceTestError(t, http.StatusForbidden, "application/json",
		`[{"message":"Session expired or invalid","errorCode":"INVALID_SESSION_ID"}]`)

	if err != SessionExpiredError {
		t.Fatalf("expected SessionExpiredError, got %v", err)
	}
}

func TestCoerceHttpErrorTreatsA403WithoutAnErrorCodeAsAnExpiredSession(t *testing.T) {
	err := coerceTestError(t, http.StatusForbidden, "text/html", `<html>Forbidden</html>`)

	if err != SessionExpiredError {
		t.Fatalf("expected SessionExpiredError, got %v", err)
	}
}

func TestCoerceHttpErrorTreatsA401AsAnExpiredSession(t *testing.T) {
	err := coerceTestError(t, http.StatusUnauthorized, "application/json",
		`[{"message":"Session expired or invalid","errorCode":"INVALID_SESSION_ID"}]`)

	if err != SessionExpiredError {
		t.Fatalf("expected SessionExpiredError, got %v", err)
	}
}

func TestCoerceHttpErrorKeepsTheRequestLimitErrorOfA403(t *testing.T) {
	err := coerceTestError(t, http.StatusForbidden, "application/json",
		`[{"message":"TotalRequests Limit exceeded.","errorCode":"REQUEST_LIMIT_EXCEEDED"}]`)

	if err != APILimitExceededError {
		t.Fatalf("expected APILimitExceededError, got %v", err)
	}
}

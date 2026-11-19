package command

import (
	"testing"

	. "github.com/ForceCLI/force/lib"
)

func TestFrontdoorUrl_uses_the_ui_sid_of_a_dpop_session(t *testing.T) {
	creds := &ForceSession{
		AccessToken: "bound-token",
		InstanceUrl: "https://example.my.salesforce.com",
		TokenType:   TokenTypeDPoP,
		DPoPKey:     "key",
		UiSid:       "unbound-ui-sid",
	}
	got, err := frontdoorUrl(creds)
	if err != nil {
		t.Fatal(err)
	}
	if want := "https://example.my.salesforce.com/secur/frontdoor.jsp?sid=unbound-ui-sid"; got != want {
		t.Errorf("got %q, want %q", got, want)
	}
}

func TestFrontdoorUrl_fails_for_a_dpop_session_without_a_ui_sid(t *testing.T) {
	creds := &ForceSession{
		AccessToken: "bound-token",
		InstanceUrl: "https://example.my.salesforce.com",
		TokenType:   TokenTypeDPoP,
		DPoPKey:     "key",
	}
	if _, err := frontdoorUrl(creds); err == nil {
		t.Error("expected an error")
	}
}

func TestFrontdoorUrl_uses_the_access_token_of_a_bearer_session(t *testing.T) {
	creds := &ForceSession{AccessToken: "plain-token", InstanceUrl: "https://example.my.salesforce.com"}
	got, err := frontdoorUrl(creds)
	if err != nil {
		t.Fatal(err)
	}
	if want := "https://example.my.salesforce.com/secur/frontdoor.jsp?sid=plain-token"; got != want {
		t.Errorf("got %q, want %q", got, want)
	}
}

func TestValidateDPoPLogin(t *testing.T) {
	tests := []struct {
		name              string
		dpop              bool
		username, keyFile string
		wantErr           bool
	}{
		{name: "browser login", dpop: true},
		{name: "client credentials", dpop: true},
		{name: "jwt bearer", dpop: true, username: "user@example.com", keyFile: "server.key"},
		{name: "oauth username-password", dpop: true, username: "user@example.com", wantErr: true},
		{name: "username login without dpop", username: "user@example.com"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := validateDPoPLogin(tt.dpop, tt.username, tt.keyFile)
			if (err != nil) != tt.wantErr {
				t.Errorf("err = %v, wantErr %t", err, tt.wantErr)
			}
		})
	}
}

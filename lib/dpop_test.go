package lib

import (
	"crypto/ecdsa"
	"crypto/elliptic"
	"crypto/sha256"
	"encoding/base64"
	"encoding/json"
	"encoding/xml"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"net/url"
	"testing"

	"github.com/golang-jwt/jwt/v5"
)

// verifiedProof is a DPoP proof whose signature has been checked against the
// public key in its header.
type verifiedProof struct {
	claims jwt.MapClaims
	jwk    map[string]interface{}
}

func (p verifiedProof) claim(name string) string {
	v, _ := p.claims[name].(string)
	return v
}

// verifyDPoPProof checks a proof the way a server does (RFC 9449 section
// 4.3): typ is dpop+jwt, alg is ES256, and the signature verifies with the
// public key in the jwk header.
func verifyDPoPProof(t *testing.T, proof string) verifiedProof {
	t.Helper()
	var jwk map[string]interface{}
	token, err := jwt.Parse(proof, func(token *jwt.Token) (interface{}, error) {
		if typ, _ := token.Header["typ"].(string); typ != "dpop+jwt" {
			return nil, fmt.Errorf("typ is %q", typ)
		}
		var ok bool
		jwk, ok = token.Header["jwk"].(map[string]interface{})
		if !ok {
			return nil, fmt.Errorf("missing jwk header")
		}
		if _, hasPrivate := jwk["d"]; hasPrivate {
			return nil, fmt.Errorf("jwk header includes the private key")
		}
		x, err := base64.RawURLEncoding.DecodeString(jwk["x"].(string))
		if err != nil {
			return nil, err
		}
		y, err := base64.RawURLEncoding.DecodeString(jwk["y"].(string))
		if err != nil {
			return nil, err
		}
		point := append([]byte{4}, append(x, y...)...)
		return ecdsa.ParseUncompressedPublicKey(elliptic.P256(), point)
	}, jwt.WithValidMethods([]string{"ES256"}))
	if err != nil {
		t.Fatalf("invalid DPoP proof: %v", err)
	}
	return verifiedProof{claims: token.Claims.(jwt.MapClaims), jwk: jwk}
}

func enableDPoP(t *testing.T) {
	t.Helper()
	original := UseDPoP
	UseDPoP = true
	t.Cleanup(func() { UseDPoP = original })
}

func newDPoPSession(t *testing.T, instanceUrl string) *ForceSession {
	t.Helper()
	key, err := newDPoPKey()
	if err != nil {
		t.Fatal(err)
	}
	encoded, err := encodeDPoPKey(key)
	if err != nil {
		t.Fatal(err)
	}
	return &ForceSession{
		AccessToken:    "bound-token",
		InstanceUrl:    instanceUrl,
		TokenType:      TokenTypeDPoP,
		DPoPKey:        encoded,
		UserInfo:       &UserInfo{OrgId: "00D000000000001"},
		SessionOptions: &SessionOptions{ApiVersion: "60.0"},
	}
}

func TestDPoPKey_round_trips_through_its_saved_encoding(t *testing.T) {
	key, err := newDPoPKey()
	if err != nil {
		t.Fatal(err)
	}
	encoded, err := encodeDPoPKey(key)
	if err != nil {
		t.Fatal(err)
	}
	decoded, err := decodeDPoPKey(encoded)
	if err != nil {
		t.Fatal(err)
	}
	if !decoded.Equal(key) {
		t.Error("decoded key differs from the original")
	}
}

func TestNewDPoPProof_signs_method_url_without_query_and_token_hash(t *testing.T) {
	key, err := newDPoPKey()
	if err != nil {
		t.Fatal(err)
	}
	u, _ := url.Parse("https://example.my.salesforce.com/services/data/v60.0/query?q=SELECT+Id+FROM+User#frag")
	proof, err := newDPoPProof(key, "GET", u, "the-token", "the-nonce")
	if err != nil {
		t.Fatal(err)
	}
	p := verifyDPoPProof(t, proof)
	if p.claim("htm") != "GET" {
		t.Errorf("htm = %q", p.claim("htm"))
	}
	if p.claim("htu") != "https://example.my.salesforce.com/services/data/v60.0/query" {
		t.Errorf("htu = %q", p.claim("htu"))
	}
	if p.claim("ath") != dpopAccessTokenHash("the-token") {
		t.Errorf("ath = %q", p.claim("ath"))
	}
	if p.claim("nonce") != "the-nonce" {
		t.Errorf("nonce = %q", p.claim("nonce"))
	}
	if p.claim("jti") == "" {
		t.Error("missing jti")
	}
	if _, ok := p.claims["iat"]; !ok {
		t.Error("missing iat")
	}
	if p.jwk["kty"] != "EC" || p.jwk["crv"] != "P-256" {
		t.Errorf("jwk = %v", p.jwk)
	}
}

func TestDPoPJWKThumbprint_hashes_the_required_members_in_order(t *testing.T) {
	key, _ := newDPoPKey()
	jwk, _ := dpopJWK(&key.PublicKey)
	got, err := dpopJWKThumbprint(&key.PublicKey)
	if err != nil {
		t.Fatal(err)
	}
	canonical := `{"crv":"P-256","kty":"EC","x":"` + jwk["x"] + `","y":"` + jwk["y"] + `"}`
	sum := sha256.Sum256([]byte(canonical))
	if want := base64.RawURLEncoding.EncodeToString(sum[:]); got != want {
		t.Errorf("thumbprint %q, want %q", got, want)
	}
}

func TestAuthorizationCodeURL_includes_dpop_jkt_only_when_given(t *testing.T) {
	withJKT, _ := url.Parse(authorizationCodeURL("https://example.my.salesforce.com", "client", "http://localhost:3835/oauth/callback", "state", "login", "challenge", "the-thumbprint"))
	if got := withJKT.Query().Get("dpop_jkt"); got != "the-thumbprint" {
		t.Errorf("dpop_jkt = %q", got)
	}
	without, _ := url.Parse(authorizationCodeURL("https://example.my.salesforce.com", "client", "http://localhost:3835/oauth/callback", "state", "login", "challenge", ""))
	if _, ok := without.Query()["dpop_jkt"]; ok {
		t.Error("authorize URL has dpop_jkt without DPoP")
	}
}

func TestRefreshOauth_uses_the_refresh_token_grant_for_a_bearer_session(t *testing.T) {
	cleanup := setupTestConfig(t)
	defer cleanup()
	originalGetUserInfo := getUserInfoFn
	getUserInfoFn = stubUserInfo("tester@example.com")
	defer func() { getUserInfoFn = originalGetUserInfo }()

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		r.ParseForm()
		if r.Form.Get("grant_type") != "refresh_token" {
			t.Errorf("grant_type = %q", r.Form.Get("grant_type"))
		}
		if r.Header.Get("DPoP") != "" {
			t.Error("Bearer refresh has a DPoP proof")
		}
		io.WriteString(w, `{"access_token":"new-token","token_type":"Bearer"}`)
	}))
	defer server.Close()

	f := NewForce(&ForceSession{
		AccessToken:    "old-token",
		RefreshToken:   "refresh",
		EndpointUrl:    server.URL,
		InstanceUrl:    server.URL,
		SessionOptions: &SessionOptions{RefreshMethod: RefreshOauth},
	})
	if err := f.RefreshSession(); err != nil {
		t.Fatal(err)
	}
}

func TestNewDPoPProof_omits_ath_for_token_endpoint_requests(t *testing.T) {
	key, _ := newDPoPKey()
	u, _ := url.Parse("https://login.salesforce.com/services/oauth2/token")
	proof, err := newDPoPProof(key, "POST", u, "", "")
	if err != nil {
		t.Fatal(err)
	}
	if _, ok := verifyDPoPProof(t, proof).claims["ath"]; ok {
		t.Error("token endpoint proof has an ath claim")
	}
}

func TestExchangeAuthorizationCode_binds_the_session_to_the_dpop_key_with_the_hybrid_grant(t *testing.T) {
	var proofKeyX string
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		r.ParseForm()
		if r.Form.Get("grant_type") != "hybrid_auth_code" {
			t.Errorf("grant_type = %q", r.Form.Get("grant_type"))
		}
		proof := r.Header.Get("DPoP")
		if proof == "" {
			t.Fatal("token request has no DPoP proof")
		}
		p := verifyDPoPProof(t, proof)
		proofKeyX, _ = p.jwk["x"].(string)
		if p.claim("htm") != "POST" || p.claim("htu") != "http://"+r.Host+"/services/oauth2/token" {
			t.Errorf("proof names %s %s", p.claim("htm"), p.claim("htu"))
		}
		json.NewEncoder(w).Encode(map[string]interface{}{
			"access_token":  "bound-token",
			"refresh_token": "bound-refresh",
			"token_type":    "DPoP",
			"ui_sid":        "unbound-ui-sid",
			"instance_url":  "https://example.my.salesforce.com",
		})
	}))
	defer server.Close()

	loginKey, _ := newDPoPKey()
	creds, err := exchangeAuthorizationCode(server.URL, "client", "code", "verifier", "http://localhost:3835/oauth/callback", loginKey)
	if err != nil {
		t.Fatal(err)
	}
	if !creds.IsDPoP() {
		t.Fatal("session is not a DPoP session")
	}
	if creds.UiSid != "unbound-ui-sid" {
		t.Errorf("UiSid = %q", creds.UiSid)
	}
	key, err := creds.dpopPrivateKey()
	if err != nil {
		t.Fatal(err)
	}
	jwk, _ := dpopJWK(&key.PublicKey)
	if jwk["x"] != proofKeyX {
		t.Error("saved key is not the key that signed the token request")
	}
}

func TestExchangeAuthorizationCode_discards_the_key_when_the_server_issues_a_bearer_token(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		json.NewEncoder(w).Encode(map[string]interface{}{
			"access_token": "plain-token",
			"token_type":   "Bearer",
			"instance_url": "https://example.my.salesforce.com",
		})
	}))
	defer server.Close()

	loginKey, _ := newDPoPKey()
	creds, err := exchangeAuthorizationCode(server.URL, "client", "code", "verifier", "http://localhost:3835/oauth/callback", loginKey)
	if err != nil {
		t.Fatal(err)
	}
	if creds.IsDPoP() || creds.DPoPKey != "" {
		t.Error("Bearer session kept a DPoP key")
	}
}

func TestExchangeAuthorizationCode_sends_no_proof_unless_dpop_is_requested(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Header.Get("DPoP") != "" {
			t.Error("token request has a DPoP proof")
		}
		r.ParseForm()
		if r.Form.Get("grant_type") != "authorization_code" {
			t.Errorf("grant_type = %q", r.Form.Get("grant_type"))
		}
		json.NewEncoder(w).Encode(map[string]interface{}{"access_token": "plain-token", "token_type": "Bearer"})
	}))
	defer server.Close()

	if _, err := exchangeAuthorizationCode(server.URL, "client", "code", "verifier", "http://localhost:3835/oauth/callback", nil); err != nil {
		t.Fatal(err)
	}
}

func TestPostTokenRequest_retries_with_the_nonce_the_server_requires(t *testing.T) {
	key, _ := newDPoPKey()
	requests := 0
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		requests++
		r.ParseForm()
		if r.Form.Get("grant_type") != "refresh_token" {
			t.Errorf("attempt %d has grant_type %q", requests, r.Form.Get("grant_type"))
		}
		p := verifyDPoPProof(t, r.Header.Get("DPoP"))
		if p.claim("nonce") != "server-nonce" {
			w.Header().Set("DPoP-Nonce", "server-nonce")
			w.WriteHeader(http.StatusBadRequest)
			io.WriteString(w, `{"error":"use_dpop_nonce","error_description":"Authorization server requires nonce in DPoP proof"}`)
			return
		}
		io.WriteString(w, `{"access_token":"bound-token","token_type":"DPoP"}`)
	}))
	defer server.Close()

	res, body, nonce, err := postTokenRequest(server.URL+"/services/oauth2/token", url.Values{"grant_type": {"refresh_token"}}, key, "expired-nonce")
	if err != nil {
		t.Fatal(err)
	}
	if nonce != "server-nonce" {
		t.Errorf("returned nonce %q", nonce)
	}
	if res.StatusCode != 200 {
		t.Fatalf("status %d: %s", res.StatusCode, body)
	}
	if requests != 2 {
		t.Errorf("expected 2 requests, got %d", requests)
	}
}

func TestPostTokenRequest_resigns_the_proof_for_a_redirected_post(t *testing.T) {
	key, _ := newDPoPKey()
	myDomain := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		p := verifyDPoPProof(t, r.Header.Get("DPoP"))
		if want := "http://" + r.Host + "/services/oauth2/token"; p.claim("htu") != want {
			t.Errorf("htu = %q, want %q", p.claim("htu"), want)
		}
		if p.claim("htm") != "POST" {
			t.Errorf("htm = %q", p.claim("htm"))
		}
		io.WriteString(w, `{"access_token":"bound-token","token_type":"DPoP"}`)
	}))
	defer myDomain.Close()
	lightning := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		http.Redirect(w, r, myDomain.URL+"/services/oauth2/token", http.StatusFound)
	}))
	defer lightning.Close()

	res, _, _, err := postTokenRequest(lightning.URL+"/services/oauth2/token", url.Values{"grant_type": {"authorization_code"}}, key, "")
	if err != nil {
		t.Fatal(err)
	}
	if res.StatusCode != 200 {
		t.Errorf("status %d", res.StatusCode)
	}
}

func TestDPoPSession_sends_dpop_authorization_and_a_proof_on_rest_requests(t *testing.T) {
	var proofs []verifiedProof
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if got := r.Header.Get("Authorization"); got != "DPoP bound-token" {
			t.Errorf("Authorization = %q", got)
		}
		if r.Header.Get("X-SFDC-Session") != "" {
			t.Error("bound token sent in X-SFDC-Session")
		}
		p := verifyDPoPProof(t, r.Header.Get("DPoP"))
		if want := "http://" + r.Host + r.URL.Path; p.claim("htu") != want {
			t.Errorf("htu = %q, want %q", p.claim("htu"), want)
		}
		if p.claim("htm") != r.Method {
			t.Errorf("htm = %q, want %q", p.claim("htm"), r.Method)
		}
		if p.claim("ath") != dpopAccessTokenHash("bound-token") {
			t.Errorf("ath = %q", p.claim("ath"))
		}
		proofs = append(proofs, p)
		w.Header().Set("Content-Type", "application/json")
		io.WriteString(w, `{}`)
	}))
	defer server.Close()

	f := NewForce(newDPoPSession(t, server.URL))
	if _, err := f.makeHttpRequestSync(NewRequest("GET").AbsoluteUrl(server.URL + "/services/data/v60.0/query?q=SELECT+Id+FROM+User")); err != nil {
		t.Fatal(err)
	}
	if _, err := f.PostAbsolute("/services/data/v60.0/sobjects/Account", `{"Name":"x"}`); err != nil {
		t.Fatal(err)
	}
	if _, err, _ := f.httpPost(server.URL+"/services/data/v60.0/sobjects/Contact", map[string]string{"LastName": "x"}); err != nil {
		t.Fatal(err)
	}
	if _, err := f.httpPatch(server.URL+"/services/data/v60.0/sobjects/Contact/003", map[string]string{"LastName": "y"}); err != nil {
		t.Fatal(err)
	}
	if _, err := f.httpDelete(server.URL + "/services/data/v60.0/sobjects/Contact/003"); err != nil {
		t.Fatal(err)
	}
	if len(proofs) != 5 {
		t.Fatalf("expected 5 proofs, got %d", len(proofs))
	}
	seen := map[string]bool{}
	for _, p := range proofs {
		if seen[p.claim("jti")] {
			t.Errorf("jti %q reused", p.claim("jti"))
		}
		seen[p.claim("jti")] = true
	}
}

func TestDPoPSession_retries_a_rest_request_with_the_nonce_the_server_requires(t *testing.T) {
	var bodies []string
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		body, _ := io.ReadAll(r.Body)
		bodies = append(bodies, string(body))
		p := verifyDPoPProof(t, r.Header.Get("DPoP"))
		if p.claim("nonce") != "resource-nonce" {
			w.Header().Set("DPoP-Nonce", "resource-nonce")
			w.Header().Set("WWW-Authenticate", `DPoP error="use_dpop_nonce", error_description="Resource server requires nonce in DPoP proof"`)
			w.WriteHeader(http.StatusUnauthorized)
			return
		}
		io.WriteString(w, `{"id":"001"}`)
	}))
	defer server.Close()

	f := NewForce(newDPoPSession(t, server.URL))
	f.Credentials.SessionOptions.RefreshFunc = func(*Force) error {
		t.Error("a nonce challenge refreshed the session")
		return nil
	}
	result, err := f.PostAbsolute("/services/data/v60.0/sobjects/Account", `{"Name":"x"}`)
	if err != nil {
		t.Fatal(err)
	}
	if result != `{"id":"001"}` {
		t.Errorf("result = %q", result)
	}
	if len(bodies) != 2 || bodies[1] != `{"Name":"x"}` {
		t.Errorf("request bodies = %q", bodies)
	}
}

func TestBearerSession_keeps_bearer_authorization_and_x_sfdc_session(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if got := r.Header.Get("Authorization"); got != "Bearer plain-token" {
			t.Errorf("Authorization = %q", got)
		}
		if got := r.Header.Get("X-SFDC-Session"); got != "plain-token" {
			t.Errorf("X-SFDC-Session = %q", got)
		}
		if r.Header.Get("DPoP") != "" {
			t.Error("Bearer session sent a DPoP proof")
		}
		io.WriteString(w, `{}`)
	}))
	defer server.Close()

	f := NewForce(&ForceSession{AccessToken: "plain-token", InstanceUrl: server.URL, SessionOptions: &SessionOptions{}})
	if _, err := f.makeHttpRequestSync(NewRequest("GET").AbsoluteUrl(server.URL + "/services/data")); err != nil {
		t.Fatal(err)
	}
	if _, err := f.PostAbsolute("/services/data", `{}`); err != nil {
		t.Fatal(err)
	}
}

func TestBearerSession_with_a_leftover_key_is_not_a_dpop_session(t *testing.T) {
	creds := newDPoPSession(t, "https://example.my.salesforce.com")
	creds.TokenType = "Bearer"
	if creds.IsDPoP() {
		t.Error("session with token_type Bearer is a DPoP session")
	}
}

func TestRefreshOauth_signs_the_refresh_request_with_the_session_key(t *testing.T) {
	cleanup := setupTestConfig(t)
	defer cleanup()
	originalGetUserInfo := getUserInfoFn
	getUserInfoFn = stubUserInfo("tester@example.com")
	defer func() { getUserInfoFn = originalGetUserInfo }()

	creds := newDPoPSession(t, "")
	creds.RefreshToken = "bound-refresh"
	creds.UiSid = "old-ui-sid"
	creds.DPoPNonce = "expired-nonce"
	key, _ := creds.dpopPrivateKey()
	sessionJWK, _ := dpopJWK(&key.PublicKey)

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		r.ParseForm()
		if r.Form.Get("grant_type") != "hybrid_refresh" {
			t.Errorf("grant_type = %q", r.Form.Get("grant_type"))
		}
		p := verifyDPoPProof(t, r.Header.Get("DPoP"))
		if p.jwk["x"] != sessionJWK["x"] || p.jwk["y"] != sessionJWK["y"] {
			t.Error("refresh proof is not signed with the session key")
		}
		if _, ok := p.claims["ath"]; ok {
			t.Error("refresh proof has an ath claim")
		}
		if p.claim("nonce") != "fresh-nonce" {
			w.Header().Set("DPoP-Nonce", "fresh-nonce")
			w.WriteHeader(http.StatusBadRequest)
			io.WriteString(w, `{"error":"use_dpop_nonce","error_description":"authorization server requires nonce in DPoP proof"}`)
			return
		}
		io.WriteString(w, fmt.Sprintf(`{"access_token":"new-bound-token","token_type":"DPoP","ui_sid":"new-ui-sid","instance_url":%q}`, "http://"+r.Host))
	}))
	defer server.Close()
	creds.EndpointUrl = server.URL
	creds.InstanceUrl = server.URL
	creds.SessionOptions.RefreshMethod = RefreshOauth

	f := NewForce(creds)
	if err := f.RefreshSession(); err != nil {
		t.Fatal(err)
	}
	if f.Credentials.AccessToken != "new-bound-token" || !f.Credentials.IsDPoP() {
		t.Errorf("refreshed session: token %q, type %q", f.Credentials.AccessToken, f.Credentials.TokenType)
	}
	if f.Credentials.UiSid != "new-ui-sid" {
		t.Errorf("UiSid = %q", f.Credentials.UiSid)
	}
	if f.Credentials.DPoPNonce != "fresh-nonce" || f.Credentials.dpopNonce() != "fresh-nonce" {
		t.Errorf("saved nonce %q, current nonce %q", f.Credentials.DPoPNonce, f.Credentials.dpopNonce())
	}
	saved, err := GetAccountCredentials("tester@example.com")
	if err != nil {
		t.Fatal(err)
	}
	if saved.DPoPNonce != "fresh-nonce" {
		t.Errorf("nonce in saved session = %q", saved.DPoPNonce)
	}
}

func TestSoap_puts_the_proof_in_a_dpop_proof_header_for_a_dpop_session(t *testing.T) {
	var endpoint string
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Header.Get("Authorization") != "" || r.Header.Get("DPoP") != "" {
			t.Error("SOAP request has DPoP HTTP headers")
		}
		var envelope struct {
			SessionId string `xml:"Header>SessionHeader>sessionId"`
			Proof     string `xml:"Header>DpopProofHeader>proof"`
		}
		body, _ := io.ReadAll(r.Body)
		if err := xml.Unmarshal(body, &envelope); err != nil {
			t.Fatal(err)
		}
		if envelope.SessionId != "bound-token" {
			t.Errorf("sessionId = %q", envelope.SessionId)
		}
		p := verifyDPoPProof(t, envelope.Proof)
		if p.claim("htm") != "POST" || p.claim("htu") != endpoint {
			t.Errorf("proof names %s %s", p.claim("htm"), p.claim("htu"))
		}
		if p.claim("ath") != dpopAccessTokenHash("bound-token") {
			t.Errorf("ath = %q", p.claim("ath"))
		}
		if p.claim("nonce") != "saved-nonce" {
			t.Errorf("nonce = %q", p.claim("nonce"))
		}
		io.WriteString(w, `<?xml version="1.0"?><soapenv:Envelope xmlns:soapenv="http://schemas.xmlsoap.org/soap/envelope/"><soapenv:Body/></soapenv:Envelope>`)
	}))
	defer server.Close()

	creds := newDPoPSession(t, server.URL)
	creds.DPoPNonce = "saved-nonce"
	endpoint = server.URL + "/services/Soap/m/60.0"
	soap := NewSoap(endpoint, "http://soap.sforce.com/2006/04/metadata", creds.AccessToken)
	soap.Session = creds
	if _, err := soap.Execute("describeMetadata", ""); err != nil {
		t.Fatal(err)
	}
}

const invalidAuthHeaderFault = `<?xml version="1.0" encoding="UTF-8"?><soapenv:Envelope xmlns:soapenv="http://schemas.xmlsoap.org/soap/envelope/"><soapenv:Body><soapenv:Fault><faultcode>sf:INVALID_AUTH_HEADER</faultcode><faultstring>INVALID_AUTH_HEADER: invalid DPoP proof</faultstring></soapenv:Fault></soapenv:Body></soapenv:Envelope>`

const invalidSessionFault = `<?xml version="1.0" encoding="UTF-8"?><soapenv:Envelope xmlns:soapenv="http://schemas.xmlsoap.org/soap/envelope/"><soapenv:Body><soapenv:Fault><faultcode>sf:INVALID_SESSION_ID</faultcode><faultstring>INVALID_SESSION_ID: Invalid Session ID found in SessionHeader: Illegal Session</faultstring></soapenv:Fault></soapenv:Body></soapenv:Envelope>`

func TestSoap_treats_invalid_auth_header_as_an_expired_dpop_session(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusInternalServerError)
		io.WriteString(w, invalidAuthHeaderFault)
	}))
	defer server.Close()

	soap := NewSoap(server.URL+"/services/Soap/m/60.0", "http://soap.sforce.com/2006/04/metadata", "bound-token")
	soap.Session = newDPoPSession(t, server.URL)
	if _, err := soap.Execute("describeMetadata", ""); err != SessionExpiredError {
		t.Errorf("err = %v", err)
	}

	soap.Session = &ForceSession{AccessToken: "plain-token"}
	if _, err := soap.Execute("describeMetadata", ""); err == SessionExpiredError {
		t.Error("Bearer session treated INVALID_AUTH_HEADER as an expired session")
	}
}

func TestMetadataSoapExecute_refreshes_a_rejected_session_once(t *testing.T) {
	requests := 0
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		requests++
		w.WriteHeader(http.StatusInternalServerError)
		io.WriteString(w, invalidSessionFault)
	}))
	defer server.Close()

	refreshes := 0
	f := NewForce(&ForceSession{
		AccessToken: "plain-token",
		InstanceUrl: server.URL,
		SessionOptions: &SessionOptions{RefreshFunc: func(*Force) error {
			refreshes++
			return nil
		}},
	})
	if _, err := f.Metadata.soapExecute("describeMetadata", ""); err != SessionExpiredError {
		t.Errorf("err = %v", err)
	}
	if refreshes != 1 || requests != 2 {
		t.Errorf("%d refreshes and %d requests, want 1 and 2", refreshes, requests)
	}
}

func TestDPoPSession_includes_the_saved_nonce_in_rest_proofs(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if got := verifyDPoPProof(t, r.Header.Get("DPoP")).claim("nonce"); got != "saved-nonce" {
			t.Errorf("nonce = %q", got)
		}
		io.WriteString(w, `{}`)
	}))
	defer server.Close()

	creds := newDPoPSession(t, server.URL)
	creds.DPoPNonce = "saved-nonce"
	if _, err := NewForce(creds).makeHttpRequestSync(NewRequest("GET").AbsoluteUrl(server.URL + "/services/data/v60.0/limits")); err != nil {
		t.Fatal(err)
	}
}

func TestDPoPSession_refreshes_for_a_new_nonce_when_salesforce_rejects_the_proof(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if verifyDPoPProof(t, r.Header.Get("DPoP")).claim("nonce") != "fresh-nonce" {
			w.Header().Set("WWW-Authenticate", "Token")
			w.WriteHeader(http.StatusUnauthorized)
			io.WriteString(w, `[{"message":"INVALID_AUTH_HEADER","errorCode":"INVALID_AUTH_HEADER"}]`)
			return
		}
		io.WriteString(w, `{}`)
	}))
	defer server.Close()

	creds := newDPoPSession(t, server.URL)
	creds.DPoPNonce = "expired-nonce"
	creds.SessionOptions.RefreshFunc = func(f *Force) error {
		f.Credentials.rememberDPoPNonce("fresh-nonce")
		return nil
	}
	if _, err := NewForce(creds).makeHttpRequestSync(NewRequest("GET").AbsoluteUrl(server.URL + "/services/data/v60.0/limits")); err != nil {
		t.Fatal(err)
	}
}

func TestSoap_sends_no_authorization_header_for_a_bearer_session(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Header.Get("Authorization") != "" || r.Header.Get("DPoP") != "" {
			t.Error("Bearer SOAP request has auth headers")
		}
		io.WriteString(w, `<?xml version="1.0"?><soapenv:Envelope xmlns:soapenv="http://schemas.xmlsoap.org/soap/envelope/"><soapenv:Body/></soapenv:Envelope>`)
	}))
	defer server.Close()

	soap := NewSoap(server.URL, "urn:partner.soap.sforce.com", "plain-token")
	soap.Session = &ForceSession{AccessToken: "plain-token"}
	if _, err := soap.Execute("getUserInfo", ""); err != nil {
		t.Fatal(err)
	}
}

func TestUISessionId_uses_ui_sid_for_dpop_sessions_and_the_access_token_otherwise(t *testing.T) {
	dpop := newDPoPSession(t, "https://example.my.salesforce.com")
	dpop.UiSid = "unbound-ui-sid"
	if got := dpop.UISessionId(); got != "unbound-ui-sid" {
		t.Errorf("DPoP session UISessionId = %q", got)
	}
	bearer := &ForceSession{AccessToken: "plain-token", UiSid: "ignored"}
	if got := bearer.UISessionId(); got != "plain-token" {
		t.Errorf("Bearer session UISessionId = %q", got)
	}
}

func TestForceSession_saves_dpop_fields(t *testing.T) {
	creds := newDPoPSession(t, "https://example.my.salesforce.com")
	creds.UiSid = "unbound-ui-sid"
	data, err := json.Marshal(creds)
	if err != nil {
		t.Fatal(err)
	}
	var loaded ForceSession
	if err := json.Unmarshal(data, &loaded); err != nil {
		t.Fatal(err)
	}
	if !loaded.IsDPoP() || loaded.UiSid != "unbound-ui-sid" || loaded.DPoPKey != creds.DPoPKey {
		t.Errorf("loaded session lost DPoP fields: %+v", loaded)
	}
}

// dpopTokenServer answers token requests that carry a valid DPoP proof with a
// DPoP-bound token, and rejects requests without one.
func dpopTokenServer(t *testing.T) *httptest.Server {
	t.Helper()
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		r.ParseForm()
		if r.Form.Get("response_type") == "device_code" {
			json.NewEncoder(w).Encode(map[string]interface{}{
				"device_code":      "the-device-code",
				"user_code":        "ABCD-1234",
				"verification_uri": "https://example.my.salesforce.com/setup/connect",
				"interval":         1,
				"expires_in":       60,
			})
			return
		}
		if r.Header.Get("DPoP") == "" {
			w.WriteHeader(http.StatusBadRequest)
			io.WriteString(w, `{"error":"invalid_dpop_proof","error_description":"missing proof"}`)
			return
		}
		if verifyDPoPProof(t, r.Header.Get("DPoP")).claim("nonce") != "login-nonce" {
			w.Header().Set("DPoP-Nonce", "login-nonce")
			w.WriteHeader(http.StatusBadRequest)
			io.WriteString(w, `{"error":"use_dpop_nonce","error_description":"authorization server requires nonce in DPoP proof"}`)
			return
		}
		json.NewEncoder(w).Encode(map[string]interface{}{
			"access_token":  "bound-token",
			"refresh_token": "bound-refresh",
			"token_type":    "DPoP",
			"ui_sid":        "unbound-ui-sid",
			"instance_url":  "https://example.my.salesforce.com",
		})
	}))
	t.Cleanup(server.Close)
	return server
}

func TestOAuthLoginFlows_bind_the_session_to_a_dpop_key(t *testing.T) {
	enableDPoP(t)
	server := dpopTokenServer(t)
	flows := map[string]func() (ForceSession, error){
		"device": func() (ForceSession, error) {
			return ForceLoginAtEndpointDeviceFlow(server.URL, "login")
		},
		"jwt bearer": func() (ForceSession, error) {
			return JWTLoginAtEndpoint(server.URL, "assertion")
		},
		"client credentials": func() (ForceSession, error) {
			return ClientCredentialsLoginAtEndpoint(server.URL, "client", "secret")
		},
		"password": func() (ForceSession, error) {
			return PasswordFlowLoginAtEndpoint(server.URL, "client", "secret", "user@example.com", "password")
		},
	}
	for name, login := range flows {
		t.Run(name, func(t *testing.T) {
			creds, err := login()
			if err != nil {
				t.Fatal(err)
			}
			if !creds.IsDPoP() {
				t.Errorf("token type %q, key saved %t", creds.TokenType, creds.DPoPKey != "")
			}
			if creds.UiSid != "unbound-ui-sid" {
				t.Errorf("UiSid = %q", creds.UiSid)
			}
			if creds.DPoPNonce != "login-nonce" {
				t.Errorf("DPoPNonce = %q", creds.DPoPNonce)
			}
		})
	}
}

func TestDeviceFlow_reuses_the_dpop_nonce_across_polls(t *testing.T) {
	enableDPoP(t)
	tokenRequests, polls := 0, 0
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		r.ParseForm()
		if r.Form.Get("response_type") == "device_code" {
			json.NewEncoder(w).Encode(map[string]interface{}{
				"device_code":      "the-device-code",
				"user_code":        "ABCD-1234",
				"verification_uri": "https://example.my.salesforce.com/setup/connect",
				"interval":         1,
				"expires_in":       60,
			})
			return
		}
		tokenRequests++
		if verifyDPoPProof(t, r.Header.Get("DPoP")).claim("nonce") != "device-nonce" {
			w.Header().Set("DPoP-Nonce", "device-nonce")
			w.WriteHeader(http.StatusBadRequest)
			io.WriteString(w, `{"error":"use_dpop_nonce","error_description":"authorization server requires nonce in DPoP proof"}`)
			return
		}
		polls++
		if polls < 3 {
			w.WriteHeader(http.StatusBadRequest)
			io.WriteString(w, `{"error":"authorization_pending","error_description":"authorization pending"}`)
			return
		}
		io.WriteString(w, `{"access_token":"bound-token","token_type":"DPoP","instance_url":"https://example.my.salesforce.com"}`)
	}))
	defer server.Close()

	creds, err := ForceLoginAtEndpointDeviceFlow(server.URL, "login")
	if err != nil {
		t.Fatal(err)
	}
	if !creds.IsDPoP() {
		t.Error("session is not a DPoP session")
	}
	// One use_dpop_nonce error for the first poll, then one request per poll.
	if tokenRequests != 4 {
		t.Errorf("%d token requests for 3 polls, want 4", tokenRequests)
	}
}

func TestDPoPSession_sends_bulk_api_1_requests_with_a_proof_for_the_request_url(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if got := r.Header.Get("Authorization"); got != "DPoP bound-token" {
			t.Errorf("Authorization = %q", got)
		}
		if r.Header.Get("X-SFDC-Session") != "" {
			t.Error("bound token sent in X-SFDC-Session")
		}
		p := verifyDPoPProof(t, r.Header.Get("DPoP"))
		if want := "http://" + r.Host + r.URL.Path; p.claim("htu") != want {
			t.Errorf("htu = %q, want %q", p.claim("htu"), want)
		}
		if p.claim("ath") != dpopAccessTokenHash("bound-token") {
			t.Errorf("ath = %q", p.claim("ath"))
		}
		w.Header().Set("Content-Type", "application/xml")
		io.WriteString(w, `<jobInfo/>`)
	}))
	defer server.Close()

	f := NewForce(newDPoPSession(t, server.URL))
	if _, err := f.makeHttpRequestSync(NewRequest("GET").AbsoluteUrl(server.URL + "/services/async/60.0/job/750000000000000AAA")); err != nil {
		t.Fatal(err)
	}
	if _, err := f.httpPostPatchWithRetry(server.URL+"/services/async/60.0/job", `<jobInfo/>`, ContentTypeXml, HttpMethodPost); err != nil {
		t.Fatal(err)
	}
}

func TestProofURL_uses_the_http_scheme_only_for_bulk_api_1_requests(t *testing.T) {
	tests := []struct{ in, want string }{
		{"https://example.my.salesforce.com/services/async/68.0/job", "http://example.my.salesforce.com/services/async/68.0/job"},
		{"https://example.my.salesforce.com/services/data/v68.0/jobs/query", "https://example.my.salesforce.com/services/data/v68.0/jobs/query"},
		{"https://example.my.salesforce.com/services/Soap/m/68.0", "https://example.my.salesforce.com/services/Soap/m/68.0"},
	}
	for _, tt := range tests {
		u, _ := url.Parse(tt.in)
		if got := proofURL(u).String(); got != tt.want {
			t.Errorf("proofURL(%s) = %s, want %s", tt.in, got, tt.want)
		}
		if u.String() != tt.in {
			t.Errorf("proofURL modified its argument: %s", u)
		}
	}
}

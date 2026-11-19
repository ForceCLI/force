package lib

import (
	"crypto/ecdsa"
	"crypto/elliptic"
	"crypto/rand"
	"crypto/sha256"
	"crypto/x509"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"
	"sync"
	"time"

	"github.com/golang-jwt/jwt/v5"
)

// UseDPoP makes new OAuth logins request DPoP-bound tokens (RFC 9449). The
// token request carries a DPoP proof signed with a new P-256 key. If
// Salesforce answers with token_type DPoP, the key is saved with the session
// and every later request made with the session carries a proof signed with
// it. If Salesforce answers with token_type Bearer, the key is discarded.
var UseDPoP = false

// TokenTypeDPoP is the token_type Salesforce returns for a DPoP-bound token.
const TokenTypeDPoP = "DPoP"

const dpopHeader = "DPoP"
const dpopNonceHeader = "DPoP-Nonce"

// sessionNonces holds the latest DPoP nonce received for each session, by
// the session's encoded key. Salesforce requires the nonce in every proof,
// for token requests and resource requests, and issues it only from the
// token endpoint, in the DPoP-Nonce header of a use_dpop_nonce error. A nonce
// is valid for 24 hours. ForceSession.DPoPNonce saves the nonce with the
// session so that a new process can use it; this map holds nonces received
// since the session was loaded, since requests can run concurrently.
var sessionNonces = struct {
	sync.Mutex
	byKey map[string]string
}{byKey: map[string]string{}}

func (creds *ForceSession) dpopNonce() string {
	sessionNonces.Lock()
	defer sessionNonces.Unlock()
	if nonce, ok := sessionNonces.byKey[creds.DPoPKey]; ok {
		return nonce
	}
	return creds.DPoPNonce
}

func (creds *ForceSession) rememberDPoPNonce(nonce string) {
	if nonce == "" || creds.DPoPKey == "" {
		return
	}
	sessionNonces.Lock()
	defer sessionNonces.Unlock()
	sessionNonces.byKey[creds.DPoPKey] = nonce
}

// newDPoPKey generates the key pair a session's tokens are bound to.
func newDPoPKey() (*ecdsa.PrivateKey, error) {
	return ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
}

// newLoginDPoPKey returns a new key when UseDPoP is set, and nil otherwise.
func newLoginDPoPKey() (*ecdsa.PrivateKey, error) {
	if !UseDPoP {
		return nil, nil
	}
	return newDPoPKey()
}

func encodeDPoPKey(key *ecdsa.PrivateKey) (string, error) {
	der, err := x509.MarshalPKCS8PrivateKey(key)
	if err != nil {
		return "", err
	}
	return base64.StdEncoding.EncodeToString(der), nil
}

func decodeDPoPKey(encoded string) (*ecdsa.PrivateKey, error) {
	der, err := base64.StdEncoding.DecodeString(encoded)
	if err != nil {
		return nil, fmt.Errorf("invalid DPoP key: %w", err)
	}
	parsed, err := x509.ParsePKCS8PrivateKey(der)
	if err != nil {
		return nil, fmt.Errorf("invalid DPoP key: %w", err)
	}
	key, ok := parsed.(*ecdsa.PrivateKey)
	if !ok {
		return nil, errors.New("invalid DPoP key: not an ECDSA key")
	}
	return key, nil
}

// dpopJWK returns the public key as a JWK (RFC 7517) for the proof header.
func dpopJWK(pub *ecdsa.PublicKey) (map[string]string, error) {
	ecdhKey, err := pub.ECDH()
	if err != nil {
		return nil, err
	}
	// The uncompressed point encoding is 0x04 || X || Y.
	point := ecdhKey.Bytes()
	size := (len(point) - 1) / 2
	return map[string]string{
		"kty": "EC",
		"crv": "P-256",
		"x":   base64.RawURLEncoding.EncodeToString(point[1 : 1+size]),
		"y":   base64.RawURLEncoding.EncodeToString(point[1+size:]),
	}, nil
}

// dpopJWKThumbprint returns the JWK thumbprint of the public key (RFC 7638):
// the base64url-encoded SHA-256 hash of the JWK's required members, crv, kty,
// x, and y, in that order with no whitespace.
func dpopJWKThumbprint(pub *ecdsa.PublicKey) (string, error) {
	jwk, err := dpopJWK(pub)
	if err != nil {
		return "", err
	}
	canonical := fmt.Sprintf(`{"crv":%q,"kty":%q,"x":%q,"y":%q}`, jwk["crv"], jwk["kty"], jwk["x"], jwk["y"])
	sum := sha256.Sum256([]byte(canonical))
	return base64.RawURLEncoding.EncodeToString(sum[:]), nil
}

// dpopHTU returns the htu claim for a request URL: the URL without its query
// and fragment (RFC 9449 section 4.2).
func dpopHTU(u *url.URL) string {
	return u.Scheme + "://" + u.Host + u.EscapedPath()
}

// dpopAccessTokenHash returns the ath claim: the base64url-encoded SHA-256
// hash of the access token.
func dpopAccessTokenHash(accessToken string) string {
	sum := sha256.Sum256([]byte(accessToken))
	return base64.RawURLEncoding.EncodeToString(sum[:])
}

// newDPoPProof returns a DPoP proof JWT for a request. accessToken is empty
// for token endpoint requests, and the proof then has no ath claim. nonce is
// omitted from the proof when empty.
func newDPoPProof(key *ecdsa.PrivateKey, method string, u *url.URL, accessToken, nonce string) (string, error) {
	jwk, err := dpopJWK(&key.PublicKey)
	if err != nil {
		return "", err
	}
	jti, err := randomURLSafeString(16)
	if err != nil {
		return "", err
	}
	claims := jwt.MapClaims{
		"jti": jti,
		"htm": method,
		"htu": dpopHTU(u),
		"iat": time.Now().Unix(),
	}
	if accessToken != "" {
		claims["ath"] = dpopAccessTokenHash(accessToken)
	}
	if nonce != "" {
		claims["nonce"] = nonce
	}
	token := jwt.NewWithClaims(jwt.SigningMethodES256, claims)
	token.Header["typ"] = "dpop+jwt"
	token.Header["jwk"] = jwk
	return token.SignedString(key)
}

// IsDPoP reports whether the session's access token is DPoP-bound, so that
// requests must carry a DPoP proof signed with the session's key.
func (creds *ForceSession) IsDPoP() bool {
	return strings.EqualFold(creds.TokenType, TokenTypeDPoP) && creds.DPoPKey != ""
}

func (creds *ForceSession) dpopPrivateKey() (*ecdsa.PrivateKey, error) {
	if creds.DPoPKey == "" {
		return nil, nil
	}
	return decodeDPoPKey(creds.DPoPKey)
}

// DPoPProof returns a new DPoP proof for a request with the session's access
// token, signed with the session's key and including the session's latest
// nonce. A proof is valid for one request only.
func (creds *ForceSession) DPoPProof(method string, u *url.URL) (string, error) {
	key, err := creds.dpopPrivateKey()
	if err != nil {
		return "", err
	}
	if key == nil {
		return "", errors.New("session has no DPoP key")
	}
	return newDPoPProof(key, method, u, creds.AccessToken, creds.dpopNonce())
}

// ApplyAuthHeaders sets the Authorization header on req. For a Bearer
// session, it is "Bearer <token>". For a DPoP session, it is "DPoP <token>",
// and the DPoP header is set to a new proof for req's method and URL. Call
// it again for each request, since a proof is valid for one request only.
func (creds *ForceSession) ApplyAuthHeaders(req *http.Request) error {
	if !creds.IsDPoP() {
		req.Header.Set("Authorization", "Bearer "+creds.AccessToken)
		return nil
	}
	proof, err := creds.DPoPProof(req.Method, proofURL(req.URL))
	if err != nil {
		return err
	}
	req.Header.Set("Authorization", "DPoP "+creds.AccessToken)
	req.Header.Set(dpopHeader, proof)
	return nil
}

// proofURL returns the URL to put in a proof's htu claim for a request to u.
// Bulk API 1.0 compares htu with the request URL using the http scheme, even
// though requests are sent over https, and answers InvalidSessionId to a
// proof whose htu uses https. For Bulk API 1.0 requests, proofURL returns u
// with the http scheme; for all others, u.
func proofURL(u *url.URL) *url.URL {
	if !isBulkV1(u) || u.Scheme != "https" {
		return u
	}
	httpURL := *u
	httpURL.Scheme = "http"
	return &httpURL
}

// isBulkV1 reports whether u is a Bulk API 1.0 endpoint.
func isBulkV1(u *url.URL) bool {
	return strings.HasPrefix(strings.ToLower(u.Path), "/services/async/")
}

// UISessionId returns the session ID to use for the Salesforce web UI, such
// as with frontdoor.jsp. A DPoP-bound access token can't be used there,
// because browser requests don't carry DPoP proofs, so a DPoP session uses
// the unbound ui_sid that Salesforce can return with the token. It returns
// an empty string for a DPoP session without a ui_sid.
func (creds *ForceSession) UISessionId() string {
	if creds.IsDPoP() {
		return creds.UiSid
	}
	return creds.AccessToken
}

// doAuthorizedRequest sends req with the session's auth headers. For a DPoP
// session, a 401 response asking for a DPoP nonce is retried once with the
// nonce the server sent, when req's body can be sent again.
func (creds *ForceSession) doAuthorizedRequest(req *http.Request, clientOptions ...clientOption) (*http.Response, error) {
	if err := creds.ApplyAuthHeaders(req); err != nil {
		return nil, err
	}
	if !creds.IsDPoP() {
		return doRequest(req, clientOptions...)
	}
	clientOptions = append(clientOptions, resignDPoPOnRedirect(func(r *http.Request) error {
		// net/http drops the Authorization header on a redirect to another
		// domain. Drop the proof too rather than add the token back.
		if r.Header.Get("Authorization") == "" {
			r.Header.Del(dpopHeader)
			return nil
		}
		return creds.ApplyAuthHeaders(r)
	}))
	res, err := doRequest(req, clientOptions...)
	if err != nil {
		return res, err
	}
	creds.rememberDPoPNonce(res.Header.Get(dpopNonceHeader))
	if !isResourceNonceChallenge(res) {
		return res, nil
	}
	retry, err := rewindRequest(req)
	if err != nil {
		return res, nil
	}
	res.Body.Close()
	if err = creds.ApplyAuthHeaders(retry); err != nil {
		return nil, err
	}
	res, err = doRequest(retry, clientOptions...)
	if err == nil {
		creds.rememberDPoPNonce(res.Header.Get(dpopNonceHeader))
	}
	return res, err
}

// isResourceNonceChallenge reports whether a resource server rejected a
// request because its proof lacked the current nonce, and sent a new one
// (RFC 9449 section 9).
func isResourceNonceChallenge(res *http.Response) bool {
	if res.StatusCode != http.StatusUnauthorized || res.Header.Get(dpopNonceHeader) == "" {
		return false
	}
	for _, v := range res.Header.Values("WWW-Authenticate") {
		if strings.Contains(v, "use_dpop_nonce") {
			return true
		}
	}
	return false
}

// rewindRequest returns a copy of req that can be sent again, with a new
// copy of its body.
func rewindRequest(req *http.Request) (*http.Request, error) {
	retry := req.Clone(req.Context())
	if req.Body == nil || req.Body == http.NoBody {
		return retry, nil
	}
	if req.GetBody == nil {
		return nil, errors.New("request body can't be sent again")
	}
	body, err := req.GetBody()
	if err != nil {
		return nil, err
	}
	retry.Body = body
	return retry, nil
}

// resignDPoPOnRedirect replaces the DPoP proof on each redirected request,
// since a proof names the method and URL of a single request. net/http copies
// the original request's headers to the redirected request before calling
// CheckRedirect.
func resignDPoPOnRedirect(sign func(*http.Request) error) clientOption {
	return func(c *http.Client) {
		next := c.CheckRedirect
		c.CheckRedirect = func(req *http.Request, via []*http.Request) error {
			if next != nil {
				if err := next(req, via); err != nil {
					return err
				}
			} else if len(via) >= 10 {
				return errors.New("stopped after 10 redirects")
			}
			if req.Header.Get(dpopHeader) == "" {
				return nil
			}
			return sign(req)
		}
	}
}

// postTokenRequest POSTs form to a token endpoint and returns the response
// and its body. When key is not nil, the request carries a DPoP proof signed
// with key and including nonce, and a use_dpop_nonce error is retried once
// with the nonce the server sent. It also returns the latest nonce: the one
// the server sent, or nonce if the server sent none. A 302 response is
// followed with another POST.
func postTokenRequest(tokenURL string, form url.Values, key *ecdsa.PrivateKey, nonce string) (res *http.Response, body []byte, latestNonce string, err error) {
	latestNonce = nonce
	for attempt := 0; ; attempt++ {
		res, body, err = postTokenRequestOnce(tokenURL, form, key, latestNonce)
		if err != nil {
			return
		}
		if sent := res.Header.Get(dpopNonceHeader); sent != "" {
			latestNonce = sent
		}
		if key == nil || attempt > 0 || !isTokenNonceChallenge(res, body) {
			return
		}
	}
}

func postTokenRequestOnce(tokenURL string, form url.Values, key *ecdsa.PrivateKey, nonce string) (res *http.Response, body []byte, err error) {
	req, err := httpRequest("POST", tokenURL, strings.NewReader(form.Encode()))
	if err != nil {
		return
	}
	req.Header.Add("Content-Type", "application/x-www-form-urlencoded")
	options := []clientOption{redirectPostOn302}
	if key != nil {
		sign := func(r *http.Request) error {
			proof, err := newDPoPProof(key, r.Method, r.URL, "", nonce)
			if err != nil {
				return err
			}
			r.Header.Set(dpopHeader, proof)
			return nil
		}
		if err = sign(req); err != nil {
			return
		}
		options = append(options, resignDPoPOnRedirect(sign))
	}
	res, err = doRequest(req, options...)
	if err != nil {
		return
	}
	defer res.Body.Close()
	body, err = io.ReadAll(res.Body)
	return
}

func isTokenNonceChallenge(res *http.Response, body []byte) bool {
	if res.StatusCode != http.StatusBadRequest && res.StatusCode != http.StatusUnauthorized {
		return false
	}
	if res.Header.Get(dpopNonceHeader) == "" {
		return false
	}
	var oauthError OAuthError
	if json.Unmarshal(body, &oauthError) != nil {
		return false
	}
	return oauthError.Error == "use_dpop_nonce"
}

// bindDPoPKey saves key and nonce in creds when the token endpoint issued a
// DPoP-bound token. A Bearer token means the server did not bind the token
// to the key, so neither is saved.
func bindDPoPKey(creds *ForceSession, key *ecdsa.PrivateKey, nonce string) error {
	creds.DPoPKey = ""
	creds.DPoPNonce = ""
	if key == nil || !strings.EqualFold(creds.TokenType, TokenTypeDPoP) {
		return nil
	}
	encoded, err := encodeDPoPKey(key)
	if err != nil {
		return err
	}
	creds.DPoPKey = encoded
	creds.DPoPNonce = nonce
	return nil
}

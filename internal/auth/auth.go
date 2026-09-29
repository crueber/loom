// Package auth implements UI-configured OIDC login with stdlib only
// (OSS-50): provider discovery, Authorization Code + PKCE, token
// exchange over net/http, and JWKS ID-token verification with
// crypto/rsa. No new Go dependencies.
//
// Sessions are server-side rows; the browser holds only an opaque
// token in an HttpOnly SameSite cookie. The client secret never
// leaves the server.
package auth

import (
	"crypto"
	"crypto/rand"
	"crypto/rsa"
	"crypto/sha256"
	"encoding/base64"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"math/big"
	"net/http"
	"net/url"
	"strings"
	"sync"
	"time"
)

// SessionCookie is the browser cookie holding the raw session token.
const SessionCookie = "loom_session"

// SessionTTL is how long a login session lasts.
const SessionTTL = 30 * 24 * time.Hour

var httpClient = &http.Client{Timeout: 10 * time.Second}

// Discovery is the subset of the OIDC discovery document we need.
type Discovery struct {
	Issuer                string `json:"issuer"`
	AuthorizationEndpoint string `json:"authorization_endpoint"`
	TokenEndpoint         string `json:"token_endpoint"`
	JwksURI               string `json:"jwks_uri"`
}

// Discover fetches {issuer}/.well-known/openid-configuration.
func Discover(issuer string) (*Discovery, error) {
	issuer = strings.TrimRight(strings.TrimSpace(issuer), "/")
	if issuer == "" {
		return nil, fmt.Errorf("issuer is required")
	}
	u, err := url.Parse(issuer)
	if err != nil || (u.Scheme != "http" && u.Scheme != "https") {
		return nil, fmt.Errorf("issuer must be an http(s) URL")
	}
	resp, err := httpClient.Get(issuer + "/.well-known/openid-configuration")
	if err != nil {
		return nil, fmt.Errorf("discovery failed: %w", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != 200 {
		return nil, fmt.Errorf("discovery failed: HTTP %d", resp.StatusCode)
	}
	body, err := io.ReadAll(io.LimitReader(resp.Body, 1<<20))
	if err != nil {
		return nil, err
	}
	var d Discovery
	if err := json.Unmarshal(body, &d); err != nil {
		return nil, fmt.Errorf("bad discovery document: %w", err)
	}
	if d.AuthorizationEndpoint == "" || d.TokenEndpoint == "" || d.JwksURI == "" {
		return nil, fmt.Errorf("discovery document missing endpoints")
	}
	return &d, nil
}

func randB64(n int) (string, error) {
	b := make([]byte, n)
	if _, err := rand.Read(b); err != nil {
		return "", err
	}
	return base64.RawURLEncoding.EncodeToString(b), nil
}

// NewState returns a random OAuth state value.
func NewState() (string, error) { return randB64(24) }

// NewVerifier returns a random PKCE code verifier.
func NewVerifier() (string, error) { return randB64(32) }

// ChallengeS256 derives the S256 code challenge for a verifier.
func ChallengeS256(verifier string) string {
	sum := sha256.Sum256([]byte(verifier))
	return base64.RawURLEncoding.EncodeToString(sum[:])
}

// Pending is one in-flight login (PKCE verifier + nonce by state).
type Pending struct {
	Verifier    string
	Nonce       string
	RedirectURI string
	Expires     time.Time
}

// Logins tracks in-flight OIDC logins (single-binary memory is fine:
// a restart only invalidates pending, not established, logins).
type Logins struct {
	mu sync.Mutex
	m  map[string]Pending
}

// NewLogins returns an empty pending-login tracker.
func NewLogins() *Logins { return &Logins{m: map[string]Pending{}} }

// Create stores a pending login for state.
func (l *Logins) Create(state, verifier, nonce, redirectURI string) {
	l.mu.Lock()
	defer l.mu.Unlock()
	l.m[state] = Pending{Verifier: verifier, Nonce: nonce, RedirectURI: redirectURI, Expires: time.Now().Add(10 * time.Minute)}
}

// Consume returns and removes the pending login for state.
func (l *Logins) Consume(state string) (Pending, bool) {
	l.mu.Lock()
	defer l.mu.Unlock()
	p, ok := l.m[state]
	delete(l.m, state)
	if !ok || time.Now().After(p.Expires) {
		return Pending{}, false
	}
	return p, true
}

// LoginURL builds the provider authorization URL (code + PKCE S256).
func LoginURL(d *Discovery, clientID, redirectURI, state, nonce, challenge string) string {
	q := url.Values{}
	q.Set("response_type", "code")
	q.Set("client_id", clientID)
	q.Set("redirect_uri", redirectURI)
	q.Set("scope", "openid email profile")
	q.Set("state", state)
	q.Set("nonce", nonce)
	q.Set("code_challenge", challenge)
	q.Set("code_challenge_method", "S256")
	return d.AuthorizationEndpoint + "?" + q.Encode()
}

// Exchange trades an authorization code for an ID token.
func Exchange(tokenEndpoint, clientID, secret, code, verifier, redirectURI string) (string, error) {
	form := url.Values{}
	form.Set("grant_type", "authorization_code")
	form.Set("code", code)
	form.Set("redirect_uri", redirectURI)
	form.Set("client_id", clientID)
	form.Set("code_verifier", verifier)
	if secret != "" {
		form.Set("client_secret", secret)
	}
	resp, err := httpClient.PostForm(tokenEndpoint, form)
	if err != nil {
		return "", fmt.Errorf("token exchange failed: %w", err)
	}
	defer resp.Body.Close()
	body, err := io.ReadAll(io.LimitReader(resp.Body, 1<<20))
	if err != nil {
		return "", err
	}
	if resp.StatusCode != 200 {
		return "", fmt.Errorf("token exchange failed: HTTP %d", resp.StatusCode)
	}
	var out struct {
		IDToken string `json:"id_token"`
	}
	if err := json.Unmarshal(body, &out); err != nil || out.IDToken == "" {
		return "", fmt.Errorf("token response has no id_token")
	}
	return out.IDToken, nil
}

// Identity is the verified subject of an ID token.
type Identity struct {
	Subject string
	Email   string
	Name    string
}

// VerifyIDToken validates an RS256 ID token against the provider JWKS
// (issuer, audience, expiry, signature; nonce when non-empty).
func VerifyIDToken(jwksURI, issuer, clientID, idToken, wantNonce string) (Identity, error) {
	var id Identity
	parts := strings.Split(idToken, ".")
	if len(parts) != 3 {
		return id, fmt.Errorf("malformed id_token")
	}
	var header struct {
		Alg string `json:"alg"`
		Kid string `json:"kid"`
	}
	if err := decodePart(parts[0], &header); err != nil {
		return id, fmt.Errorf("bad token header: %w", err)
	}
	if header.Alg != "RS256" {
		return id, fmt.Errorf("unsupported token alg %q (RS256 only)", header.Alg)
	}
	var claims struct {
		Iss   string  `json:"iss"`
		Sub   string  `json:"sub"`
		Aud   any     `json:"aud"`
		Exp   float64 `json:"exp"`
		Nonce string  `json:"nonce"`
		Email string  `json:"email"`
		Name  string  `json:"name"`
	}
	if err := decodePart(parts[1], &claims); err != nil {
		return id, fmt.Errorf("bad token claims: %w", err)
	}
	if claims.Iss != issuer {
		return id, fmt.Errorf("bad issuer")
	}
	if !audMatches(claims.Aud, clientID) {
		return id, fmt.Errorf("bad audience")
	}
	if claims.Exp == 0 || time.Now().Unix() > int64(claims.Exp)+60 {
		return id, fmt.Errorf("token expired")
	}
	if claims.Sub == "" {
		return id, fmt.Errorf("token has no subject")
	}
	if wantNonce != "" && claims.Nonce != wantNonce {
		return id, fmt.Errorf("bad nonce")
	}
	pub, err := fetchKey(jwksURI, header.Kid)
	if err != nil {
		return id, err
	}
	sig, err := base64.RawURLEncoding.DecodeString(parts[2])
	if err != nil {
		return id, fmt.Errorf("bad signature encoding")
	}
	sum := sha256.Sum256([]byte(parts[0] + "." + parts[1]))
	if err := rsa.VerifyPKCS1v15(pub, crypto.SHA256, sum[:], sig); err != nil {
		return id, fmt.Errorf("bad signature")
	}
	id.Subject, id.Email, id.Name = claims.Sub, claims.Email, claims.Name
	return id, nil
}

func audMatches(aud any, clientID string) bool {
	switch a := aud.(type) {
	case string:
		return a == clientID
	case []any:
		for _, v := range a {
			if s, ok := v.(string); ok && s == clientID {
				return true
			}
		}
	}
	return false
}

func decodePart(part string, v any) error {
	raw, err := base64.RawURLEncoding.DecodeString(part)
	if err != nil {
		return err
	}
	return json.Unmarshal(raw, v)
}

func fetchKey(jwksURI, kid string) (*rsa.PublicKey, error) {
	resp, err := httpClient.Get(jwksURI)
	if err != nil {
		return nil, fmt.Errorf("jwks fetch failed: %w", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != 200 {
		return nil, fmt.Errorf("jwks fetch failed: HTTP %d", resp.StatusCode)
	}
	body, err := io.ReadAll(io.LimitReader(resp.Body, 1<<20))
	if err != nil {
		return nil, err
	}
	var set struct {
		Keys []struct {
			Kty string `json:"kty"`
			Kid string `json:"kid"`
			N   string `json:"n"`
			E   string `json:"e"`
		} `json:"keys"`
	}
	if err := json.Unmarshal(body, &set); err != nil {
		return nil, fmt.Errorf("bad jwks: %w", err)
	}
	for _, k := range set.Keys {
		if k.Kty != "RSA" || (kid != "" && k.Kid != kid) {
			continue
		}
		nb, err := base64.RawURLEncoding.DecodeString(k.N)
		if err != nil {
			continue
		}
		eb, err := base64.RawURLEncoding.DecodeString(k.E)
		if err != nil {
			continue
		}
		e := 0
		for _, b := range eb {
			e = e<<8 + int(b)
		}
		if e == 0 {
			continue
		}
		return &rsa.PublicKey{N: new(big.Int).SetBytes(nb), E: e}, nil
	}
	return nil, fmt.Errorf("no matching RSA key in jwks")
}

// MintToken returns a random session token and its stored hash.
func MintToken() (raw, hash string, err error) {
	b := make([]byte, 32)
	if _, err := rand.Read(b); err != nil {
		return "", "", err
	}
	raw = hex.EncodeToString(b)
	sum := sha256.Sum256([]byte(raw))
	return raw, hex.EncodeToString(sum[:]), nil
}

// HashToken hashes a presented token for store lookup.
func HashToken(raw string) string {
	sum := sha256.Sum256([]byte(strings.TrimSpace(raw)))
	return hex.EncodeToString(sum[:])
}

// SetSessionCookie stores the raw token in an HttpOnly cookie.
func SetSessionCookie(w http.ResponseWriter, raw string, exp time.Time) {
	c := &http.Cookie{
		Name:     SessionCookie,
		Value:    raw,
		Path:     "/",
		HttpOnly: true,
		SameSite: http.SameSiteLaxMode,
		Expires:  exp,
	}
	http.SetCookie(w, c)
}

// ClearSessionCookie removes the session cookie.
func ClearSessionCookie(w http.ResponseWriter) {
	http.SetCookie(w, &http.Cookie{Name: SessionCookie, Path: "/", HttpOnly: true, SameSite: http.SameSiteLaxMode, MaxAge: -1})
}

// TokenFromRequest reads the session token from the request cookie.
func TokenFromRequest(r *http.Request) string {
	c, err := r.Cookie(SessionCookie)
	if err != nil {
		return ""
	}
	return strings.TrimSpace(c.Value)
}

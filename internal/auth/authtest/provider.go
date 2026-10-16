// Package authtest is a fake OIDC provider for tests (docs/adr/0004): a real HTTP server with discovery,
// a signing key, an authorize endpoint that signs whoever the test says in at once, and a token endpoint
// that checks PKCE. Tests drive the app's real sign-in against it.
package authtest

import (
	"crypto/rand"
	"crypto/rsa"
	"crypto/sha256"
	"encoding/base64"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"net/url"
	"sync"
	"testing"
	"time"

	"github.com/go-jose/go-jose/v4"
	"github.com/stretchr/testify/require"
)

const (
	ClientID     = "business-crm-test"
	ClientSecret = "test-secret"
)

// Claims go into the next ID token. Zero values are left out, except EmailVerified.
type Claims struct {
	Subject       string
	Email         string
	EmailVerified bool
	Name          string
	HostedDomain  string
	// Audience overrides the client ID, to test a token issued for another app.
	Audience string
	// Nonce overrides the one the app sent, to test a token from another sign-in.
	Nonce string
}

type Provider struct {
	*httptest.Server
	t   *testing.T
	key *rsa.PrivateKey

	mu    sync.Mutex
	next  Claims
	codes map[string]grant
}

type grant struct {
	claims    Claims
	nonce     string
	challenge string
}

func NewProvider(t *testing.T) *Provider {
	t.Helper()
	key, err := rsa.GenerateKey(rand.Reader, 2048)
	require.NoError(t, err)
	p := &Provider{t: t, key: key, codes: map[string]grant{}}
	mux := http.NewServeMux()
	mux.HandleFunc("GET /.well-known/openid-configuration", p.discovery)
	mux.HandleFunc("GET /jwks", p.jwks)
	mux.HandleFunc("GET /authorize", p.authorize)
	mux.HandleFunc("POST /token", p.token)
	p.Server = httptest.NewServer(mux)
	t.Cleanup(p.Close)
	return p
}

// SignsIn makes the next sign-in at this provider be this person.
func (p *Provider) SignsIn(c Claims) {
	p.mu.Lock()
	defer p.mu.Unlock()
	p.next = c
}

func (p *Provider) discovery(w http.ResponseWriter, _ *http.Request) {
	writeJSON(w, map[string]any{
		"issuer":                                p.URL,
		"authorization_endpoint":                p.URL + "/authorize",
		"token_endpoint":                        p.URL + "/token",
		"jwks_uri":                              p.URL + "/jwks",
		"response_types_supported":              []string{"code"},
		"subject_types_supported":               []string{"public"},
		"id_token_signing_alg_values_supported": []string{"RS256"},
	})
}

func (p *Provider) jwks(w http.ResponseWriter, _ *http.Request) {
	writeJSON(w, jose.JSONWebKeySet{Keys: []jose.JSONWebKey{{Key: &p.key.PublicKey, KeyID: "test", Algorithm: "RS256", Use: "sig"}}})
}

// authorize signs the next person in without a page, and sends the browser back with a code.
func (p *Provider) authorize(w http.ResponseWriter, r *http.Request) {
	q := r.URL.Query()
	if q.Get("client_id") != ClientID || q.Get("code_challenge_method") != "S256" {
		http.Error(w, "bad authorize request", http.StatusBadRequest)
		return
	}
	code := rand.Text()
	p.mu.Lock()
	p.codes[code] = grant{claims: p.next, nonce: q.Get("nonce"), challenge: q.Get("code_challenge")}
	p.mu.Unlock()
	back, _ := url.Parse(q.Get("redirect_uri"))
	back.RawQuery = url.Values{"code": {code}, "state": {q.Get("state")}}.Encode()
	http.Redirect(w, r, back.String(), http.StatusFound)
}

func (p *Provider) token(w http.ResponseWriter, r *http.Request) {
	id, secret, ok := r.BasicAuth()
	if !ok {
		id, secret = r.FormValue("client_id"), r.FormValue("client_secret")
	}
	p.mu.Lock()
	g, found := p.codes[r.FormValue("code")]
	delete(p.codes, r.FormValue("code"))
	p.mu.Unlock()
	sum := sha256.Sum256([]byte(r.FormValue("code_verifier")))
	if id != ClientID || secret != ClientSecret || !found || base64.RawURLEncoding.EncodeToString(sum[:]) != g.challenge {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusBadRequest)
		_, _ = w.Write([]byte(`{"error":"invalid_grant"}`))
		return
	}
	writeJSON(w, map[string]any{"access_token": "test", "token_type": "Bearer", "expires_in": 3600, "id_token": p.idToken(g)})
}

func (p *Provider) idToken(g grant) string {
	aud := g.claims.Audience
	if aud == "" {
		aud = ClientID
	}
	nonce := g.nonce
	if g.claims.Nonce != "" {
		nonce = g.claims.Nonce
	}
	now := time.Now()
	claims := map[string]any{
		"iss": p.URL, "aud": aud, "sub": g.claims.Subject, "iat": now.Unix(), "exp": now.Add(time.Hour).Unix(),
		"nonce": nonce, "email_verified": g.claims.EmailVerified,
	}
	for k, v := range map[string]string{"email": g.claims.Email, "name": g.claims.Name, "hd": g.claims.HostedDomain} {
		if v != "" {
			claims[k] = v
		}
	}
	signer, err := jose.NewSigner(jose.SigningKey{Algorithm: jose.RS256, Key: jose.JSONWebKey{Key: p.key, KeyID: "test"}},
		(&jose.SignerOptions{}).WithType("JWT"))
	require.NoError(p.t, err)
	payload, _ := json.Marshal(claims)
	signed, err := signer.Sign(payload)
	require.NoError(p.t, err)
	raw, err := signed.CompactSerialize()
	require.NoError(p.t, err)
	return raw
}

func writeJSON(w http.ResponseWriter, v any) {
	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(v)
}

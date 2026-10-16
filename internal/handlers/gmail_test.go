package handlers

import (
	"context"
	"net/http"
	"net/http/httptest"
	"sync"
	"testing"

	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/kthaisociety/business-crm/internal/auth"
	"github.com/kthaisociety/business-crm/internal/auth/authtest"
	"github.com/kthaisociety/business-crm/internal/database/databasetest"
	"github.com/kthaisociety/business-crm/internal/deals"
	"github.com/kthaisociety/business-crm/internal/gmail"
	"github.com/kthaisociety/business-crm/internal/secretbox"
)

// Connect Gmail (docs/adr/0006), against the fake provider standing in for Google.

// revoked records the tokens sent to the fake revoke endpoint.
type revoked struct {
	mu     sync.Mutex
	tokens []string
}

func newGmailApp(t *testing.T) (*signInApp, *revoked) {
	gin.SetMode(gin.TestMode)
	provider := authtest.NewProvider(t)
	config := func(callback string, extra ...string) auth.OIDCConfig {
		return auth.OIDCConfig{
			Issuer: provider.URL, ClientID: authtest.ClientID, ClientSecret: authtest.ClientSecret,
			RedirectURL: "http://127.0.0.1:5174" + callback, Domain: "example.com", Google: true,
			ExtraScopes: extra, Offline: len(extra) > 0,
		}
	}
	signIn, err := auth.NewOIDC(context.Background(), config("/auth/callback"))
	require.NoError(t, err)
	google, err := auth.NewOIDC(context.Background(), config("/auth/gmail/callback", gmail.Scope))
	require.NoError(t, err)
	box, err := secretbox.New("BwcHBwcHBwcHBwcHBwcHBwcHBwcHBwcHBwcHBwcHBwc=")
	require.NoError(t, err)

	rec := &revoked{}
	revoke := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		rec.mu.Lock()
		rec.tokens = append(rec.tokens, r.FormValue("token"))
		rec.mu.Unlock()
	}))
	t.Cleanup(revoke.Close)
	before := gmail.RevokeURL
	gmail.RevokeURL = revoke.URL
	t.Cleanup(func() { gmail.RevokeURL = before })

	db := databasetest.Open(t)
	store := deals.NewStore(db).OnlyTeamMembers()
	router := NewRouter(Deps{
		PingDB:    func(context.Context) error { return nil },
		Frontend:  builtUI,
		OIDC:      signIn,
		Sessions:  auth.NewSessions(db, "http://127.0.0.1:5174"),
		Deals:     store,
		GmailOIDC: google,
		Gmail:     gmail.NewConnections(db, box),
	})
	return &signInApp{router: router, provider: provider, store: store, db: db}, rec
}

func signedInAsMaja(t *testing.T, app *signInApp) *browser {
	t.Helper()
	addToTeam(t, app.store, "maja@example.com")
	b := app.browser(t)
	require.Equal(t, "/", b.signIn(maja()))
	return b
}

func gmailState(t *testing.T, b *browser) string {
	t.Helper()
	return decode[map[string]any](t, b.do(http.MethodGet, "/api/me"))["gmail"].(string)
}

func TestAMemberConnectsTheirGmail(t *testing.T) {
	app, _ := newGmailApp(t)
	b := signedInAsMaja(t, app)
	assert.Equal(t, "not-connected", gmailState(t, b))

	assert.Equal(t, "/?gmail=connected", b.through("/auth/gmail/connect", "/auth/gmail/callback", maja()))

	asked := app.provider.LastAuthorize
	assert.Contains(t, asked.Get("scope"), gmail.Scope)
	assert.Equal(t, "offline", asked.Get("access_type"), "a refresh token, to sync later")
	assert.Equal(t, "consent", asked.Get("prompt"))
	assert.Equal(t, "maja@example.com", asked.Get("login_hint"))

	assert.Equal(t, "connected", gmailState(t, b))
	status := decode[gmail.Status](t, b.do(http.MethodGet, "/api/gmail"))
	assert.Equal(t, "maja@example.com", status.Email)

	var stored struct{ RefreshToken []byte }
	require.NoError(t, app.db.Raw("SELECT refresh_token FROM gmail_connections").Scan(&stored).Error)
	require.NotEmpty(t, stored.RefreshToken)
	assert.NotContains(t, string(stored.RefreshToken), "refresh-maja", "the token is stored sealed")

	for _, m := range decode[[]deals.MemberView](t, b.do(http.MethodGet, "/api/members")) {
		require.NotNil(t, m.GmailConnected)
		assert.True(t, *m.GmailConnected)
	}
}

func TestOnlyYourOwnMailboxConnects(t *testing.T) {
	app, _ := newGmailApp(t)
	b := signedInAsMaja(t, app)
	someoneElse := maja()
	someoneElse.Subject, someoneElse.Email = "oskar", "oskar@example.com"

	assert.Equal(t, "/?gmail=wrong-account", b.through("/auth/gmail/connect", "/auth/gmail/callback", someoneElse))

	assert.Equal(t, "not-connected", gmailState(t, b))
}

func TestUntickingGmailConnectsNothing(t *testing.T) {
	app, _ := newGmailApp(t)
	b := signedInAsMaja(t, app)
	who := maja()
	who.Withheld = []string{gmail.Scope}

	assert.Equal(t, "/?gmail=no-access", b.through("/auth/gmail/connect", "/auth/gmail/callback", who))

	assert.Equal(t, "not-connected", gmailState(t, b))
}

func TestConnectingNeedsASession(t *testing.T) {
	app, _ := newGmailApp(t)

	w := app.browser(t).do(http.MethodGet, "/auth/gmail/connect")

	assert.Equal(t, http.StatusFound, w.Code)
	assert.Equal(t, "/", w.Header().Get("Location"))
}

func TestDisconnectingForgetsAndRevokesTheGrant(t *testing.T) {
	app, rec := newGmailApp(t)
	b := signedInAsMaja(t, app)
	b.through("/auth/gmail/connect", "/auth/gmail/callback", maja())

	assert.Equal(t, http.StatusNoContent, b.do(http.MethodDelete, "/api/gmail").Code)

	assert.Equal(t, "not-connected", gmailState(t, b))
	assert.Equal(t, []string{"refresh-maja"}, rec.tokens)
	assert.Equal(t, http.StatusNoContent, b.do(http.MethodDelete, "/api/gmail").Code, "disconnecting twice is fine")
}

func TestRemovingAMemberRemovesTheirMailbox(t *testing.T) {
	app, _ := newGmailApp(t)
	b := signedInAsMaja(t, app)
	b.through("/auth/gmail/connect", "/auth/gmail/callback", maja())
	team, err := app.store.ListMembers(context.Background())
	require.NoError(t, err)

	require.NoError(t, app.store.RemoveMember(context.Background(), team[0].ID))

	var left int64
	require.NoError(t, app.db.Raw("SELECT count(*) FROM gmail_connections").Scan(&left).Error)
	assert.Zero(t, left)
}

func TestWithoutGmailTheAppSaysItsOff(t *testing.T) {
	assert.Equal(t, "off", decode[map[string]any](t, get(newTeamRouter(t), "/api/me"))["gmail"])
}

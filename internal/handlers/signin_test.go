package handlers

import (
	"context"
	"net/http"
	"net/http/httptest"
	"net/url"
	"testing"

	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"gorm.io/gorm"

	"github.com/kthaisociety/business-crm/internal/auth"
	"github.com/kthaisociety/business-crm/internal/auth/authtest"
	"github.com/kthaisociety/business-crm/internal/database/databasetest"
	"github.com/kthaisociety/business-crm/internal/deals"
)

// Real sign-in (docs/adr/0004) against a fake provider. Test accounts are made up, so the domain that may
// sign in is example.com here.

type signInApp struct {
	router   *gin.Engine
	provider *authtest.Provider
	store    *deals.Store
	db       *gorm.DB
}

func newSignInApp(t *testing.T, adminEmails ...string) *signInApp {
	gin.SetMode(gin.TestMode)
	provider := authtest.NewProvider(t)
	oidc, err := auth.NewOIDC(context.Background(), auth.OIDCConfig{
		Issuer:       provider.URL,
		ClientID:     authtest.ClientID,
		ClientSecret: authtest.ClientSecret,
		RedirectURL:  "http://127.0.0.1:5174/auth/callback",
		Domain:       "example.com",
		Google:       true,
	})
	require.NoError(t, err)
	db := databasetest.Open(t)
	store := deals.NewStore(db, adminEmails...).OnlyTeamMembers()
	router := NewRouter(Deps{
		PingDB:   func(context.Context) error { return nil },
		Frontend: builtUI,
		OIDC:     oidc,
		Sessions: auth.NewSessions(db, "http://127.0.0.1:5174"),
		Deals:    store,
	})
	return &signInApp{router: router, provider: provider, store: store, db: db}
}

// browser keeps cookies between requests to the app, like a browser would.
type browser struct {
	t       *testing.T
	app     *signInApp
	cookies map[string]*http.Cookie
}

func (a *signInApp) browser(t *testing.T) *browser {
	return &browser{t: t, app: a, cookies: map[string]*http.Cookie{}}
}

func (b *browser) do(method, target string) *httptest.ResponseRecorder {
	req := httptest.NewRequest(method, target, nil)
	for _, c := range b.cookies {
		req.AddCookie(c)
	}
	w := httptest.NewRecorder()
	b.app.router.ServeHTTP(w, req)
	for _, c := range w.Result().Cookies() {
		if c.MaxAge < 0 {
			delete(b.cookies, c.Name)
		} else {
			b.cookies[c.Name] = c
		}
	}
	return w
}

// signIn goes through the whole sign-in as this person, and returns where the app sends the browser
// at the end.
func (b *browser) signIn(who authtest.Claims) string {
	b.t.Helper()
	b.app.provider.SignsIn(who)
	w := b.do(http.MethodGet, "/auth/login")
	require.Equal(b.t, http.StatusFound, w.Code)

	noFollow := &http.Client{CheckRedirect: func(*http.Request, []*http.Request) error { return http.ErrUseLastResponse }}
	resp, err := noFollow.Get(w.Header().Get("Location"))
	require.NoError(b.t, err)
	resp.Body.Close()
	require.Equal(b.t, http.StatusFound, resp.StatusCode)
	back, err := url.Parse(resp.Header.Get("Location"))
	require.NoError(b.t, err)
	require.Equal(b.t, "/auth/callback", back.Path)

	w = b.do(http.MethodGet, back.RequestURI())
	require.Equal(b.t, http.StatusFound, w.Code)
	return w.Header().Get("Location")
}

func maja() authtest.Claims {
	return authtest.Claims{Subject: "maja", Email: "Maja@example.com", EmailVerified: true, Name: "Maja Holm", HostedDomain: "example.com"}
}

func addToTeam(t *testing.T, store *deals.Store, email string) deals.MemberView {
	m, err := store.AddMember(context.Background(), deals.MemberInput{Email: email, Name: "Added By An Admin"})
	require.NoError(t, err)
	return m
}

func TestTeamMembersSignInAndOut(t *testing.T) {
	app := newSignInApp(t)
	added := addToTeam(t, app.store, "maja@example.com")
	b := app.browser(t)
	assert.Equal(t, http.StatusUnauthorized, b.do(http.MethodGet, "/api/me").Code)

	assert.Equal(t, "/", b.signIn(maja()))

	w := b.do(http.MethodGet, "/api/me")
	require.Equal(t, http.StatusOK, w.Code, w.Body.String())
	me := decode[map[string]any](t, w)
	assert.Equal(t, "maja@example.com", me["email"])
	assert.Equal(t, "Maja Holm", me["name"], "the name comes from the provider")
	assert.Equal(t, added.ID, me["memberId"])
	assert.Equal(t, true, me["canSignOut"])
	session := b.cookies["crm_session"]
	require.NotNil(t, session)
	assert.True(t, session.HttpOnly)
	assert.Equal(t, http.SameSiteLaxMode, session.SameSite)

	assert.Equal(t, http.StatusNoContent, b.do(http.MethodPost, "/auth/logout").Code)
	assert.Equal(t, http.StatusUnauthorized, b.do(http.MethodGet, "/api/me").Code)

	// The old cookie is dead too, not just forgotten by this browser.
	replay := app.browser(t)
	replay.cookies["crm_session"] = session
	assert.Equal(t, http.StatusUnauthorized, replay.do(http.MethodGet, "/api/me").Code)
}

func TestSomeoneNotOnTheTeamIsRefused(t *testing.T) {
	app := newSignInApp(t)
	b := app.browser(t)

	assert.Equal(t, "/?sign-in=not-on-team", b.signIn(maja()))

	assert.Equal(t, http.StatusUnauthorized, b.do(http.MethodGet, "/api/me").Code)
	team, err := app.store.ListMembers(context.Background())
	require.NoError(t, err)
	assert.Empty(t, team, "a refused sign-in doesn't join the team")
}

func TestAdminEmailsSignInAndBecomeAdmins(t *testing.T) {
	app := newSignInApp(t, "maja@example.com")
	b := app.browser(t)

	assert.Equal(t, "/", b.signIn(maja()))

	assert.Equal(t, "admin", decode[map[string]any](t, b.do(http.MethodGet, "/api/me"))["role"])
}

func TestOnlyVerifiedAccountsInTheDomainSignIn(t *testing.T) {
	app := newSignInApp(t, "maja@example.com", "maja@elsewhere.example")
	for name, who := range map[string]authtest.Claims{
		"another domain":      {Subject: "x", Email: "maja@elsewhere.example", EmailVerified: true, HostedDomain: "example.com"},
		"an unverified email": {Subject: "x", Email: "maja@example.com", EmailVerified: false, HostedDomain: "example.com"},
		"not a Workspace one": {Subject: "x", Email: "maja@example.com", EmailVerified: true},
		"a lookalike domain":  {Subject: "x", Email: "maja@notexample.com", EmailVerified: true, HostedDomain: "example.com"},
	} {
		t.Run(name, func(t *testing.T) {
			b := app.browser(t)
			assert.Equal(t, "/?sign-in=domain", b.signIn(who))
			assert.Equal(t, http.StatusUnauthorized, b.do(http.MethodGet, "/api/me").Code)
		})
	}
}

func TestATokenForAnotherAppIsRefused(t *testing.T) {
	app := newSignInApp(t, "maja@example.com")
	who := maja()
	who.Audience = "another-app"
	b := app.browser(t)

	assert.Equal(t, "/?sign-in=failed", b.signIn(who))
	assert.Equal(t, http.StatusUnauthorized, b.do(http.MethodGet, "/api/me").Code)
}

func TestATokenFromAnotherSignInIsRefused(t *testing.T) {
	app := newSignInApp(t, "maja@example.com")
	who := maja()
	who.Nonce = "from-another-sign-in"
	b := app.browser(t)

	assert.Equal(t, "/?sign-in=failed", b.signIn(who))
	assert.Equal(t, http.StatusUnauthorized, b.do(http.MethodGet, "/api/me").Code)
}

func TestSignInsInTwoTabsBothWork(t *testing.T) {
	app := newSignInApp(t, "maja@example.com")
	app.provider.SignsIn(maja())
	b := app.browser(t)
	noFollow := &http.Client{CheckRedirect: func(*http.Request, []*http.Request) error { return http.ErrUseLastResponse }}
	backFrom := func() string {
		w := b.do(http.MethodGet, "/auth/login")
		resp, err := noFollow.Get(w.Header().Get("Location"))
		require.NoError(t, err)
		resp.Body.Close()
		back, _ := url.Parse(resp.Header.Get("Location"))
		return back.RequestURI()
	}
	first, second := backFrom(), backFrom()

	assert.Equal(t, "/", b.do(http.MethodGet, first).Header().Get("Location"))
	assert.Equal(t, "/", b.do(http.MethodGet, second).Header().Get("Location"))
}

// A callback must finish a sign-in this browser started: otherwise someone could sign a victim's browser
// in as themselves with their own code.
func TestACallbackFinishesOnlyThisBrowsersSignIn(t *testing.T) {
	app := newSignInApp(t, "maja@example.com")
	app.provider.SignsIn(maja())
	attacker := app.browser(t)
	start := attacker.do(http.MethodGet, "/auth/login")
	noFollow := &http.Client{CheckRedirect: func(*http.Request, []*http.Request) error { return http.ErrUseLastResponse }}
	resp, err := noFollow.Get(start.Header().Get("Location"))
	require.NoError(t, err)
	resp.Body.Close()
	back, _ := url.Parse(resp.Header.Get("Location"))

	victim := app.browser(t)
	w := victim.do(http.MethodGet, back.RequestURI())
	assert.Equal(t, "/?sign-in=failed", w.Header().Get("Location"))
	assert.Empty(t, victim.cookies)

	// The right browser with the wrong state fails too.
	q := back.Query()
	q.Set("state", "forged")
	w = attacker.do(http.MethodGet, "/auth/callback?"+q.Encode())
	assert.Equal(t, "/?sign-in=failed", w.Header().Get("Location"))
	assert.Equal(t, http.StatusUnauthorized, attacker.do(http.MethodGet, "/api/me").Code)
}

func TestRemovingAMemberSignsThemOut(t *testing.T) {
	app := newSignInApp(t)
	added := addToTeam(t, app.store, "maja@example.com")
	b := app.browser(t)
	b.signIn(maja())
	require.Equal(t, http.StatusOK, b.do(http.MethodGet, "/api/deals").Code)

	require.NoError(t, app.store.RemoveMember(context.Background(), added.ID))

	assert.Equal(t, http.StatusUnauthorized, b.do(http.MethodGet, "/api/deals").Code)
}

func TestSessionsExpire(t *testing.T) {
	app := newSignInApp(t)
	addToTeam(t, app.store, "maja@example.com")
	b := app.browser(t)
	b.signIn(maja())

	require.NoError(t, app.db.Exec("UPDATE sessions SET expires_at = now() - interval '1 second'").Error)

	assert.Equal(t, http.StatusUnauthorized, b.do(http.MethodGet, "/api/me").Code)
}

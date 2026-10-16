package auth

import (
	"context"
	"net/http"
	"net/http/httptest"
	"net/url"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/kthaisociety/lucid/internal/auth/authtest"
)

func TestASignInGivesUpOnAProviderThatStopsAnswering(t *testing.T) {
	provider := authtest.NewProvider(t)
	o, err := NewOIDC(context.Background(), OIDCConfig{
		Issuer: provider.URL, ClientID: authtest.ClientID, ClientSecret: authtest.ClientSecret,
		RedirectURL: "http://127.0.0.1:5174/auth/callback", Domain: "example.com",
	})
	require.NoError(t, err)
	before := providerTimeout
	providerTimeout = 100 * time.Millisecond
	t.Cleanup(func() { providerTimeout = before })

	begun := httptest.NewRecorder()
	noFollow := &http.Client{CheckRedirect: func(*http.Request, []*http.Request) error { return http.ErrUseLastResponse }}
	resp, err := noFollow.Get(o.Begin(begun, ""))
	require.NoError(t, err)
	resp.Body.Close()
	back, err := url.Parse(resp.Header.Get("Location"))
	require.NoError(t, err)
	callback := httptest.NewRequest(http.MethodGet, back.RequestURI(), nil)
	for _, c := range begun.Result().Cookies() {
		callback.AddCookie(c)
	}
	provider.Stalls()

	start := time.Now()
	_, err = o.Finish(httptest.NewRecorder(), callback)

	var refused *SignInError
	require.ErrorAs(t, err, &refused)
	assert.Equal(t, ReasonFailed, refused.Reason)
	assert.Less(t, time.Since(start), 5*time.Second)
}

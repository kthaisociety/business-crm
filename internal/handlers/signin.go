package handlers

import (
	"errors"
	"net/http"
	"net/url"

	"github.com/gin-gonic/gin"

	"github.com/kthaisociety/lucid/internal/auth"
	"github.com/kthaisociety/lucid/internal/deals"
)

// registerSignIn adds real sign-in (docs/adr/0004). The callback always ends on the app: signed in, or
// with ?sign-in=<reason> for the sign-in page to explain.
func registerSignIn(r *gin.Engine, oidc *auth.OIDC, sessions *auth.Sessions, store *deals.Store) {
	r.GET("/auth/login", func(c *gin.Context) {
		c.Redirect(http.StatusFound, oidc.Begin(c.Writer, ""))
	})

	r.GET("/auth/callback", func(c *gin.Context) {
		refuse := func(reason string, err error) {
			_ = c.Error(err)
			c.Redirect(http.StatusFound, "/?"+url.Values{"sign-in": {reason}}.Encode())
		}
		user, err := oidc.Finish(c.Writer, c.Request)
		var refused *auth.SignInError
		if errors.As(err, &refused) {
			refuse(refused.Reason, err)
			return
		}
		if err != nil {
			refuse(auth.ReasonFailed, err)
			return
		}
		member, err := store.Admit(c.Request.Context(), user)
		if errors.Is(err, deals.ErrNotOnTeam) {
			refuse(auth.ReasonNotOnTeam, err)
			return
		}
		if err == nil {
			err = sessions.Start(c, member.ID)
		}
		if err != nil {
			refuse(auth.ReasonFailed, err)
			return
		}
		c.Redirect(http.StatusFound, "/")
	})

	r.POST("/auth/logout", func(c *gin.Context) {
		if err := sessions.End(c); err != nil {
			fail(c, err)
			return
		}
		c.Status(http.StatusNoContent)
	})
}

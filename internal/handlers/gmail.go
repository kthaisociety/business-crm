package handlers

import (
	"net/http"
	"net/url"
	"strings"

	"github.com/gin-gonic/gin"

	"github.com/kthaisociety/lucid/internal/auth"
	"github.com/kthaisociety/lucid/internal/gmail"
)

// registerGmail adds Connect Gmail (docs/adr/0006): a second Google consent, for the signed-in member's
// own mailbox only. The callback always ends on the app, with ?gmail=<outcome> for it to show.
func registerGmail(r *gin.Engine, api *gin.RouterGroup, google *auth.OIDC, sessions *auth.Sessions, conns *gmail.Connections, sync *gmail.Syncer) {
	back := func(c *gin.Context, outcome string) {
		c.Redirect(http.StatusFound, "/?"+url.Values{"gmail": {outcome}}.Encode())
	}
	// Pages the browser opens directly: without a session, back to the app to sign in.
	signedIn := func(c *gin.Context) (auth.User, bool) {
		u, ok, err := sessions.UserOf(c.Request)
		if err != nil {
			_ = c.Error(err)
		}
		if !ok {
			c.Redirect(http.StatusFound, "/")
		}
		return u, ok
	}

	r.GET("/auth/gmail/connect", func(c *gin.Context) {
		if u, ok := signedIn(c); ok {
			c.Redirect(http.StatusFound, google.Begin(c.Writer, u.Email))
		}
	})

	r.GET("/auth/gmail/callback", func(c *gin.Context) {
		u, ok := signedIn(c)
		if !ok {
			return
		}
		mailbox, token, err := google.FinishWithToken(c.Writer, c.Request)
		if err != nil {
			_ = c.Error(err)
			back(c, "failed")
			return
		}
		if !strings.EqualFold(mailbox.Email, u.Email) {
			// Someone signed in as one member can't connect another account's mail to the CRM.
			back(c, "wrong-account")
			return
		}
		scopes, _ := token.Extra("scope").(string)
		if !gmail.Granted(scopes) {
			back(c, "no-access")
			return
		}
		if token.RefreshToken == "" {
			back(c, "failed")
			return
		}
		if err := conns.Save(c.Request.Context(), u.Subject, u.Email, token.RefreshToken); err != nil {
			_ = c.Error(err)
			back(c, "failed")
			return
		}
		if sync != nil {
			sync.Kick()
		}
		back(c, "connected")
	})

	api.GET("/gmail", func(c *gin.Context) {
		status, err := conns.Status(c.Request.Context(), auth.Current(c).Subject)
		if err != nil {
			fail(c, err)
			return
		}
		c.JSON(http.StatusOK, status)
	})
	api.DELETE("/gmail", func(c *gin.Context) {
		if err := conns.Disconnect(c.Request.Context(), auth.Current(c).Subject); err != nil {
			fail(c, err)
			return
		}
		c.Status(http.StatusNoContent)
	})
}

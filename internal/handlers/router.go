// Package handlers is the HTTP layer: the JSON API under /api, and the frontend everywhere else
// (docs/adr/0001).
package handlers

import (
	"context"
	"io/fs"
	"net/http"
	"path"
	"strings"

	"github.com/gin-gonic/gin"

	"github.com/kthaisociety/business-crm/internal/auth"
	"github.com/kthaisociety/business-crm/internal/deals"
)

type Deps struct {
	PingDB func(ctx context.Context) error
	// Frontend is the built frontend (frontend.Dist()).
	Frontend fs.FS
	// Real sign-in (docs/adr/0004): with OIDC and Sessions set, requests are signed in from their session
	// cookie. Otherwise DevUser signs every request in, for local work (docs/pcr/0007); with neither, every
	// request but health is refused.
	OIDC     *auth.OIDC
	Sessions *auth.Sessions
	DevUser  *auth.User
	Deals    *deals.Store
}

func NewRouter(d Deps) *gin.Engine {
	r := gin.New()
	r.Use(gin.Logger(), gin.Recovery())

	api := r.Group("/api", limitBody(maxBodyBytes))
	api.GET("/health", health(d.PingDB))

	signIn := auth.Middleware(d.DevUser)
	if d.Sessions != nil {
		signIn = d.Sessions.Middleware()
		registerSignIn(r, d.OIDC, d.Sessions, d.Deals)
	}
	signedIn := api.Group("", signIn)
	signedIn.GET("/me", me(d.Deals, d.Sessions != nil))
	signedIn.GET("/deals", listDeals(d.Deals))
	signedIn.POST("/deals", createDeal(d.Deals))
	signedIn.GET("/deals/:id", getDeal(d.Deals))
	signedIn.PATCH("/deals/:id", updateDeal(d.Deals))
	signedIn.POST("/deals/:id/notes", addNote(d.Deals))
	registerCompanies(signedIn, d.Deals)
	registerTeam(signedIn, d.Deals)

	r.NoRoute(frontend(d.Frontend))
	return r
}

func health(pingDB func(ctx context.Context) error) gin.HandlerFunc {
	return func(c *gin.Context) {
		if err := pingDB(c.Request.Context()); err != nil {
			c.JSON(http.StatusServiceUnavailable, gin.H{"service": "business-crm", "status": "unhealthy", "database": err.Error()})
			return
		}
		c.JSON(http.StatusOK, gin.H{"service": "business-crm", "status": "healthy"})
	}
}

// me is the signed-in user and, with a store, who they are on the team; canSignOut is false for the dev
// user, who is always signed in.
func me(store *deals.Store, canSignOut bool) gin.HandlerFunc {
	return func(c *gin.Context) {
		user := auth.Current(c)
		if store == nil {
			c.JSON(http.StatusOK, gin.H{"sub": user.Subject, "email": user.Email, "name": user.Name, "canSignOut": canSignOut})
			return
		}
		member, err := store.SignIn(c.Request.Context(), user)
		if err != nil {
			fail(c, err)
			return
		}
		c.JSON(http.StatusOK, gin.H{"sub": user.Subject, "email": user.Email, "name": user.Name, "memberId": member.ID, "role": member.Role, "canSignOut": canSignOut})
	}
}

// frontend serves the single-page app: a file if one exists at the path, otherwise index.html so
// client-side routes work on reload. Unknown /api paths stay JSON 404s.
func frontend(dist fs.FS) gin.HandlerFunc {
	files := http.FileServerFS(dist)
	return func(c *gin.Context) {
		p := c.Request.URL.Path
		if p == "/api" || strings.HasPrefix(p, "/api/") {
			c.JSON(http.StatusNotFound, gin.H{"error": "not found"})
			return
		}
		if c.Request.Method != http.MethodGet && c.Request.Method != http.MethodHead {
			c.Status(http.StatusMethodNotAllowed)
			return
		}

		name := strings.TrimPrefix(path.Clean(p), "/")
		if name != "" && name != "index.html" {
			if info, err := fs.Stat(dist, name); err == nil && !info.IsDir() {
				if strings.HasPrefix(name, "assets/") {
					// Vite puts a content hash in every asset's name.
					c.Header("Cache-Control", "public, max-age=31536000, immutable")
				}
				files.ServeHTTP(c.Writer, c.Request)
				return
			}
		}

		index, err := fs.ReadFile(dist, "index.html")
		if err != nil {
			c.String(http.StatusServiceUnavailable, "frontend not built: run `npm run build` in frontend/")
			return
		}
		c.Header("Cache-Control", "no-cache")
		c.Data(http.StatusOK, "text/html; charset=utf-8", index)
	}
}

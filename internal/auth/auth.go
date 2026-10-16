// Package auth tells handlers who is signed in: from a session after a real OIDC sign-in (docs/adr/0004),
// or, for local work without a client, the dev user from config (docs/pcr/0007). With neither, every
// request is refused. Either way, handlers call Current.
package auth

import (
	"net/http"

	"github.com/gin-gonic/gin"
)

// User is the signed-in person. identity-service's tokens will add to it.
type User struct {
	Subject string `json:"sub"`
	Email   string `json:"email"`
	Name    string `json:"name"`
}

const userKey = "auth.user"

// Middleware signs every request in as the dev user, or answers 401 when dev is nil.
func Middleware(dev *User) gin.HandlerFunc {
	return func(c *gin.Context) {
		if dev == nil {
			c.AbortWithStatusJSON(http.StatusUnauthorized, gin.H{"error": "sign-in required"})
			return
		}
		c.Set(userKey, *dev)
		c.Next()
	}
}

// Current is the signed-in user. Only call it in handlers behind Middleware.
func Current(c *gin.Context) User {
	return c.MustGet(userKey).(User)
}

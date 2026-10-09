// Package auth tells handlers who is signed in (docs/pcr/0005). Until identity-service is live, the user is
// simulated: the dev user from config; with none configured, every request is refused. identity-service's
// sign-in replaces Middleware; handlers keep calling Current.
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

// Middleware signs every request in as dev, or answers 401 when dev is nil.
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

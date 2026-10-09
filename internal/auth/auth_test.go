package auth

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/assert"
)

func newRouter(dev *User) *gin.Engine {
	gin.SetMode(gin.TestMode)
	r := gin.New()
	r.GET("/me", Middleware(dev), func(c *gin.Context) { c.JSON(http.StatusOK, Current(c)) })
	return r
}

func get(r http.Handler) *httptest.ResponseRecorder {
	w := httptest.NewRecorder()
	r.ServeHTTP(w, httptest.NewRequest(http.MethodGet, "/me", nil))
	return w
}

func TestNoDevUserRefusesEveryRequest(t *testing.T) {
	w := get(newRouter(nil))

	assert.Equal(t, http.StatusUnauthorized, w.Code)
}

func TestDevUserIsSignedIn(t *testing.T) {
	w := get(newRouter(&User{Subject: "dev", Email: "dev@kthais.com"}))

	assert.Equal(t, http.StatusOK, w.Code)
	assert.Contains(t, w.Body.String(), `"email":"dev@kthais.com"`)
}

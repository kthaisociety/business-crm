package handlers

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"testing/fstest"

	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/kthaisociety/lucid/internal/auth"
)

var builtUI = fstest.MapFS{
	"index.html":      {Data: []byte("<!doctype html><title>Business CRM</title>")},
	"assets/app-1.js": {Data: []byte("console.log(1)")},
	"favicon.svg":     {Data: []byte("<svg/>")},
	".gitkeep":        {Data: nil},
}

var devUser = &auth.User{Subject: "dev", Email: "dev@kthais.com", Name: "Dev User"}

func newTestRouter(dist fstest.MapFS) *gin.Engine {
	gin.SetMode(gin.TestMode)
	return NewRouter(Deps{PingDB: func(context.Context) error { return nil }, Frontend: dist, DevUser: devUser})
}

func get(r http.Handler, path string) *httptest.ResponseRecorder {
	w := httptest.NewRecorder()
	r.ServeHTTP(w, httptest.NewRequest(http.MethodGet, path, nil))
	return w
}

func TestMeIsTheSignedInUser(t *testing.T) {
	w := get(newTestRouter(builtUI), "/api/me")

	require.Equal(t, http.StatusOK, w.Code)
	var body auth.User
	require.NoError(t, json.Unmarshal(w.Body.Bytes(), &body))
	assert.Equal(t, *devUser, body)
}

func TestWithoutADevUserOnlyHealthAnswers(t *testing.T) {
	gin.SetMode(gin.TestMode)
	r := NewRouter(Deps{PingDB: func(context.Context) error { return nil }, Frontend: builtUI})

	assert.Equal(t, http.StatusOK, get(r, "/api/health").Code)
	assert.Equal(t, http.StatusUnauthorized, get(r, "/api/me").Code)
	assert.Equal(t, http.StatusOK, get(r, "/").Code)
}

func TestFrontendRouting(t *testing.T) {
	r := newTestRouter(builtUI)

	tests := []struct {
		path     string
		status   int
		contains string
	}{
		{"/", http.StatusOK, "Business CRM"},
		{"/companies/42", http.StatusOK, "Business CRM"}, // client-side route
		{"/assets/app-1.js", http.StatusOK, "console.log"},
		{"/favicon.svg", http.StatusOK, "<svg/>"},
		{"/api/nope", http.StatusNotFound, "not found"},
		{"/api/health", http.StatusOK, "healthy"},
	}
	for _, tt := range tests {
		t.Run(tt.path, func(t *testing.T) {
			w := get(r, tt.path)
			assert.Equal(t, tt.status, w.Code)
			assert.Contains(t, w.Body.String(), tt.contains)
		})
	}
}

func TestAssetsAreCachedAndIndexIsNot(t *testing.T) {
	r := newTestRouter(builtUI)

	assert.Contains(t, get(r, "/assets/app-1.js").Header().Get("Cache-Control"), "immutable")
	assert.Equal(t, "no-cache", get(r, "/").Header().Get("Cache-Control"))
}

func TestFrontendNotBuilt(t *testing.T) {
	r := newTestRouter(fstest.MapFS{".gitkeep": {}})

	w := get(r, "/")

	assert.Equal(t, http.StatusServiceUnavailable, w.Code)
	assert.Contains(t, w.Body.String(), "frontend not built")
}

package handlers

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/kthaisociety/business-crm/internal/database/databasetest"
	"github.com/kthaisociety/business-crm/internal/deals"
)

func newDealsRouter(t *testing.T) *gin.Engine {
	gin.SetMode(gin.TestMode)
	return NewRouter(Deps{
		PingDB:   func(context.Context) error { return nil },
		Frontend: builtUI,
		DevUser:  devUser,
		Deals:    deals.NewStore(databasetest.Open(t)),
	})
}

func send(r http.Handler, method, path, body string) *httptest.ResponseRecorder {
	w := httptest.NewRecorder()
	req := httptest.NewRequest(method, path, strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	r.ServeHTTP(w, req)
	return w
}

func decode[T any](t *testing.T, w *httptest.ResponseRecorder) T {
	t.Helper()
	var v T
	require.NoError(t, json.Unmarshal(w.Body.Bytes(), &v), w.Body.String())
	return v
}

func TestDealLifecycle(t *testing.T) {
	r := newDealsRouter(t)

	// An empty log is an empty list, not null.
	w := get(r, "/api/deals")
	require.Equal(t, http.StatusOK, w.Code)
	assert.JSONEq(t, `[]`, w.Body.String())

	w = send(r, http.MethodPost, "/api/deals", `{"company":"Nordljus AB","dealType":"Event","semester":"HT 2026","nextStep":"Send intro email"}`)
	require.Equal(t, http.StatusCreated, w.Code, w.Body.String())
	created := decode[deals.View](t, w)
	assert.Equal(t, devUser.Email, created.Owner.Email)
	assert.Nil(t, created.Contact)

	w = send(r, http.MethodPatch, "/api/deals/"+created.ID, `{"status":"discussing","feeSEK":20000}`)
	require.Equal(t, http.StatusOK, w.Code, w.Body.String())
	updated := decode[deals.View](t, w)
	assert.Equal(t, "discussing", updated.Status)
	assert.Equal(t, 20000, *updated.FeeSEK)

	w = send(r, http.MethodPost, "/api/deals/"+created.ID+"/notes", `{"body":"Intro call booked"}`)
	require.Equal(t, http.StatusCreated, w.Code, w.Body.String())
	assert.Equal(t, "Intro call booked", decode[deals.ActivityView](t, w).Body)

	list := decode[[]deals.View](t, get(r, "/api/deals"))
	require.Len(t, list, 1)
	assert.Equal(t, "discussing", list[0].Status)
	assert.Len(t, list[0].Activities, 2)
}

func TestDealErrors(t *testing.T) {
	r := newDealsRouter(t)
	created := decode[deals.View](t, send(r, http.MethodPost, "/api/deals", `{"company":"Nordljus AB","dealType":"Event","semester":"HT 2026"}`))
	unknown := "/api/deals/00000000-0000-0000-0000-000000000000"

	tests := []struct {
		name, method, path, body string
		status                   int
	}{
		{"lead without company", http.MethodPost, "/api/deals", `{"dealType":"Event","semester":"HT 2026"}`, http.StatusBadRequest},
		{"lead that isn't JSON", http.MethodPost, "/api/deals", `nope`, http.StatusBadRequest},
		{"negative fee", http.MethodPatch, "/api/deals/" + created.ID, `{"feeSEK":-1}`, http.StatusBadRequest},
		{"field that can't change", http.MethodPatch, "/api/deals/" + created.ID, `{"company":"Other AB"}`, http.StatusBadRequest},
		{"empty note", http.MethodPost, "/api/deals/" + created.ID + "/notes", `{"body":" "}`, http.StatusBadRequest},
		{"update unknown deal", http.MethodPatch, unknown, `{"status":"signed"}`, http.StatusNotFound},
		{"note on unknown deal", http.MethodPost, unknown + "/notes", `{"body":"hi"}`, http.StatusNotFound},
		{"malformed id", http.MethodPatch, "/api/deals/42", `{"status":"signed"}`, http.StatusNotFound},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			w := send(r, tt.method, tt.path, tt.body)
			assert.Equal(t, tt.status, w.Code, w.Body.String())
			assert.NotEmpty(t, decode[map[string]string](t, w)["error"])
		})
	}
}

func TestDealsNeedSignIn(t *testing.T) {
	gin.SetMode(gin.TestMode)
	r := NewRouter(Deps{PingDB: func(context.Context) error { return nil }, Frontend: builtUI})

	assert.Equal(t, http.StatusUnauthorized, get(r, "/api/deals").Code)
	assert.Equal(t, http.StatusUnauthorized, send(r, http.MethodPost, "/api/deals", `{}`).Code)
}

package handlers

import (
	"context"
	"net/http"
	"testing"

	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/kthaisociety/business-crm/internal/database/databasetest"
	"github.com/kthaisociety/business-crm/internal/deals"
)

func newTeamRouter(t *testing.T, adminEmails ...string) *gin.Engine {
	gin.SetMode(gin.TestMode)
	return NewRouter(Deps{
		PingDB:   func(context.Context) error { return nil },
		Frontend: builtUI,
		DevUser:  devUser,
		Deals:    deals.NewStore(databasetest.Open(t), adminEmails...),
	})
}

func TestMeSaysWhoIAmOnTheTeam(t *testing.T) {
	r := newTeamRouter(t, devUser.Email)

	me := decode[map[string]string](t, get(r, "/api/me"))

	assert.Equal(t, devUser.Email, me["email"])
	assert.Equal(t, "admin", me["role"])
	assert.NotEmpty(t, me["memberId"])
	assert.Len(t, decode[[]deals.MemberView](t, get(r, "/api/members")), 1)
}

func TestOnlyAdminsChangeTheTeam(t *testing.T) {
	r := newTeamRouter(t) // the dev user is a plain member

	assert.Equal(t, http.StatusOK, get(r, "/api/members").Code)
	w := send(r, http.MethodPost, "/api/members", `{"email":"maja@example.com","name":"Maja Holm"}`)
	assert.Equal(t, http.StatusForbidden, w.Code, w.Body.String())
	assert.Equal(t, "only admins can change the team", decode[map[string]string](t, w)["error"])
}

func TestAdminsManageTheTeam(t *testing.T) {
	r := newTeamRouter(t, devUser.Email)

	w := send(r, http.MethodPost, "/api/members", `{"email":"maja@example.com","name":"Maja Holm","role":"board"}`)
	require.Equal(t, http.StatusCreated, w.Code, w.Body.String())
	maja := decode[deals.MemberView](t, w)
	assert.Equal(t, "board", maja.Role)

	w = send(r, http.MethodPatch, "/api/members/"+maja.ID, `{"role":"member"}`)
	require.Equal(t, http.StatusOK, w.Code, w.Body.String())
	assert.Equal(t, "member", decode[deals.MemberView](t, w).Role)

	// A lead assigned to Maja before she has ever signed in.
	w = send(r, http.MethodPost, "/api/deals", `{"ownerId":"`+maja.ID+`","company":"Vinga Voice","dealType":"Event","semester":"HT 2026"}`)
	require.Equal(t, http.StatusCreated, w.Code, w.Body.String())
	deal := decode[deals.View](t, w)
	assert.Equal(t, "Maja Holm", deal.Owner.Name)

	assert.Equal(t, http.StatusConflict, send(r, http.MethodDelete, "/api/members/"+maja.ID, ``).Code)
	send(r, http.MethodPatch, "/api/deals/"+deal.ID, `{"ownerId":null}`)
	assert.Equal(t, http.StatusNoContent, send(r, http.MethodDelete, "/api/members/"+maja.ID, ``).Code)

	me := decode[map[string]string](t, get(r, "/api/me"))
	assert.Equal(t, http.StatusConflict, send(r, http.MethodPatch, "/api/members/"+me["memberId"], `{"role":"member"}`).Code)
}

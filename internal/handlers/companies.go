package handlers

import (
	"context"
	"encoding/json"
	"io"
	"net/http"

	"github.com/gin-gonic/gin"

	"github.com/kthaisociety/lucid/internal/auth"
	"github.com/kthaisociety/lucid/internal/deals"
	"github.com/kthaisociety/lucid/internal/gmail"
)

// Companies and contacts. Deals point at them, so a company or contact with deals can't be deleted (409).

func list[T any](load func(context.Context) ([]T, error)) gin.HandlerFunc {
	return func(c *gin.Context) {
		items, err := load(c.Request.Context())
		if err != nil {
			fail(c, err)
			return
		}
		c.JSON(http.StatusOK, items)
	}
}

func create[In, Out any](save func(context.Context, In) (Out, error)) gin.HandlerFunc {
	return func(c *gin.Context) {
		var in In
		if err := c.ShouldBindJSON(&in); err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"error": "body must be a JSON object"})
			return
		}
		out, err := save(c.Request.Context(), in)
		if err != nil {
			fail(c, err)
			return
		}
		c.JSON(http.StatusCreated, out)
	}
}

func patch[Out any](parse func([]byte) (map[string]any, error), save func(context.Context, string, map[string]any) (Out, error)) gin.HandlerFunc {
	return func(c *gin.Context) {
		body, err := io.ReadAll(c.Request.Body)
		if err != nil {
			fail(c, err)
			return
		}
		cols, err := parse(body)
		if err != nil {
			fail(c, err)
			return
		}
		out, err := save(c.Request.Context(), c.Param("id"), cols)
		if err != nil {
			fail(c, err)
			return
		}
		c.JSON(http.StatusOK, out)
	}
}

func remove(del func(context.Context, string) error) gin.HandlerFunc {
	return func(c *gin.Context) {
		if err := del(c.Request.Context(), c.Param("id")); err != nil {
			fail(c, err)
			return
		}
		c.Status(http.StatusNoContent)
	}
}

func registerCompanies(r *gin.RouterGroup, store *deals.Store) {
	r.GET("/companies", list(store.ListCompanies))
	r.POST("/companies", create(store.CreateCompany))
	r.PATCH("/companies/:id", patch(deals.ParseCompanyPatch, store.UpdateCompany))
	r.DELETE("/companies/:id", remove(store.DeleteCompany))

	r.GET("/contacts", list(store.ListContacts))
	r.POST("/contacts", create(store.CreateContact))
	r.PATCH("/contacts/:id", patch(deals.ParseContactPatch, store.UpdateContact))
	r.DELETE("/contacts/:id", remove(store.DeleteContact))
}

// adminOnly lets only admins through (docs/adr/0003).
func adminOnly(store *deals.Store) gin.HandlerFunc {
	return func(c *gin.Context) {
		if err := store.RequireAdmin(c.Request.Context(), auth.Current(c)); err != nil {
			fail(c, err)
			c.Abort()
			return
		}
		c.Next()
	}
}

// The team: everyone sees it; admins change it.
func registerTeam(r *gin.RouterGroup, store *deals.Store, conns *gmail.Connections) {
	r.GET("/members", list(func(ctx context.Context) ([]deals.MemberView, error) {
		members, err := store.ListMembers(ctx)
		if err != nil || conns == nil {
			return members, err
		}
		connected, err := conns.ConnectedMembers(ctx)
		for i := range members {
			c := connected[members[i].ID]
			members[i].GmailConnected = &c
		}
		return members, err
	}))
	admin := r.Group("", adminOnly(store))
	admin.POST("/members", create(store.AddMember))
	admin.PATCH("/members/:id", patch(deals.ParseMemberPatch, store.UpdateMember))
	admin.DELETE("/members/:id", remove(store.RemoveMember))
	admin.POST("/members/:id/hand-over-admin", handOverAdmin(store))
}

// handOverAdmin makes the member an admin, and the signed-in admin `{stepDownTo}`: board or member
// (docs/adr/0005). It answers with who the signed-in admin is now.
func handOverAdmin(store *deals.Store) gin.HandlerFunc {
	return func(c *gin.Context) {
		var body struct {
			StepDownTo string `json:"stepDownTo"`
		}
		raw, ok := readBody(c)
		if !ok {
			return
		}
		if err := json.Unmarshal(raw, &body); err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"error": "the body must be JSON: {stepDownTo}"})
			return
		}
		me, err := store.HandOverAdmin(c.Request.Context(), auth.Current(c), c.Param("id"), body.StepDownTo)
		if err != nil {
			fail(c, err)
			return
		}
		c.JSON(http.StatusOK, me)
	}
}

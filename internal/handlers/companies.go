package handlers

import (
	"context"
	"io"
	"net/http"

	"github.com/gin-gonic/gin"

	"github.com/kthaisociety/business-crm/internal/auth"
	"github.com/kthaisociety/business-crm/internal/deals"
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
func registerTeam(r *gin.RouterGroup, store *deals.Store) {
	r.GET("/members", list(store.ListMembers))
	admin := r.Group("", adminOnly(store))
	admin.POST("/members", create(store.AddMember))
	admin.PATCH("/members/:id", patch(deals.ParseMemberPatch, store.UpdateMember))
	admin.DELETE("/members/:id", remove(store.RemoveMember))
}

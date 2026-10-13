package handlers

import (
	"errors"
	"io"
	"log"
	"net/http"

	"github.com/gin-gonic/gin"

	"github.com/kthaisociety/business-crm/internal/auth"
	"github.com/kthaisociety/business-crm/internal/deals"
)

// The deal log API. Every route is behind auth.Middleware: creating and changing deals records who did it.

func listDeals(store *deals.Store) gin.HandlerFunc {
	return func(c *gin.Context) {
		list, err := store.List(c.Request.Context())
		if err != nil {
			fail(c, err)
			return
		}
		c.JSON(http.StatusOK, list)
	}
}

func createDeal(store *deals.Store) gin.HandlerFunc {
	return func(c *gin.Context) {
		var lead deals.NewLead
		if err := c.ShouldBindJSON(&lead); err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"error": "body must be a JSON object"})
			return
		}
		deal, err := store.Create(c.Request.Context(), lead, auth.Current(c))
		if err != nil {
			fail(c, err)
			return
		}
		c.JSON(http.StatusCreated, deal)
	}
}

func updateDeal(store *deals.Store) gin.HandlerFunc {
	return func(c *gin.Context) {
		body, err := io.ReadAll(c.Request.Body)
		if err != nil {
			fail(c, err)
			return
		}
		cols, err := deals.ParsePatch(body)
		if err != nil {
			fail(c, err)
			return
		}
		deal, err := store.Update(c.Request.Context(), c.Param("id"), cols)
		if err != nil {
			fail(c, err)
			return
		}
		c.JSON(http.StatusOK, deal)
	}
}

func addNote(store *deals.Store) gin.HandlerFunc {
	return func(c *gin.Context) {
		body, err := io.ReadAll(c.Request.Body)
		if err != nil {
			fail(c, err)
			return
		}
		text, err := deals.ParseNote(body)
		if err != nil {
			fail(c, err)
			return
		}
		note, err := store.AddNote(c.Request.Context(), c.Param("id"), text, auth.Current(c))
		if err != nil {
			fail(c, err)
			return
		}
		c.JSON(http.StatusCreated, note)
	}
}

// fail answers 400 with the reason for a bad request, 404 for an unknown deal, and 500 for anything else,
// which is logged rather than shown.
func fail(c *gin.Context, err error) {
	var invalid *deals.ValidationError
	switch {
	case errors.As(err, &invalid):
		c.JSON(http.StatusBadRequest, gin.H{"error": invalid.Error()})
	case errors.Is(err, deals.ErrNotFound):
		c.JSON(http.StatusNotFound, gin.H{"error": "deal not found"})
	default:
		log.Printf("%s %s: %v", c.Request.Method, c.Request.URL.Path, err)
		c.JSON(http.StatusInternalServerError, gin.H{"error": "something went wrong"})
	}
}

package handlers

import (
	"log"
	"net/http"
	"time"

	"github.com/gin-gonic/gin"

	"github.com/kthaisociety/business-crm/internal/deals"
	"github.com/kthaisociety/business-crm/internal/fathom"
)

// A meeting's summary and action items are larger than anything the app's own forms send.
const maxWebhookBytes = 1 << 20

// fathomWebhook takes a meeting from the team's Fathom webhook (docs/adr/0006). It's outside sign-in:
// the signature is what proves it's Fathom. A meeting with no contact on it is acknowledged and dropped.
func fathomWebhook(key []byte, store *deals.Store) gin.HandlerFunc {
	return func(c *gin.Context) {
		body, ok := readBody(c)
		if !ok {
			return
		}
		if err := fathom.Verify(key, c.Request.Header, body, time.Now()); err != nil {
			log.Printf("fathom webhook refused: %v", err)
			c.JSON(http.StatusUnauthorized, gin.H{"error": "invalid signature"})
			return
		}
		meeting, err := fathom.Parse(body)
		if err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
			return
		}
		added, err := store.Record(c.Request.Context(), deals.Touchpoint{
			Type:         "meeting",
			Source:       "fathom",
			ExternalID:   meeting.RecordingID,
			Title:        meeting.Title,
			Body:         meeting.Summary,
			BulletPoints: meeting.ActionItems,
			URL:          meeting.URL,
			OccurredAt:   meeting.StartedAt,
			Emails:       meeting.InviteeEmails,
			By:           meeting.RecordedBy,
		})
		if err != nil {
			fail(c, err)
			return
		}
		if added > 0 {
			log.Printf("fathom meeting %s added to %d deals", meeting.RecordingID, added)
		}
		c.Status(http.StatusNoContent)
	}
}


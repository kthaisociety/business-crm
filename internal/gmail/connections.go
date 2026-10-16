// Package gmail connects members' own Gmail mailboxes to the CRM (docs/adr/0006): it keeps each
// member's refresh token sealed, and lets them disconnect. Syncing their mail comes on top of this.
package gmail

import (
	"context"
	"errors"
	"log"
	"net/http"
	"net/url"
	"strings"
	"time"

	"gorm.io/gorm"
	"gorm.io/gorm/clause"

	"github.com/kthaisociety/business-crm/internal/secretbox"
)

// Scope reads a mailbox: headers, snippets and bodies, never sending or changing anything.
const Scope = "https://www.googleapis.com/auth/gmail.readonly"

// RevokeURL is Google's endpoint for withdrawing a grant; tests point it at a fake.
var RevokeURL = "https://oauth2.googleapis.com/revoke"

// Granted says whether a token response's space-separated scopes include Gmail's: Google lets someone
// untick it on the consent screen.
func Granted(scopes string) bool {
	for _, s := range strings.Fields(scopes) {
		if s == Scope {
			return true
		}
	}
	return false
}

type Connections struct {
	db     *gorm.DB
	box    *secretbox.Box
	client *http.Client
}

func NewConnections(db *gorm.DB, box *secretbox.Box) *Connections {
	return &Connections{db: db, box: box, client: &http.Client{Timeout: 10 * time.Second}}
}

type connection struct {
	MemberID     string `gorm:"primaryKey"`
	Email        string
	RefreshToken []byte
	HistoryID    *string
	ConnectedAt  time.Time `gorm:"default:now()"`
	SyncedAt     *time.Time
	SyncError    string
	LeaseUntil   *time.Time
	// The first sync's progress: the history id it started at, and its next page.
	BackfillHistory *string
	BackfillPage    *string
}

func (connection) TableName() string { return "gmail_connections" }

// Status is what a member sees of their connection.
type Status struct {
	Connected   bool       `json:"connected"`
	Email       string     `json:"email,omitempty"`
	ConnectedAt *time.Time `json:"connectedAt,omitempty"`
	SyncedAt    *time.Time `json:"syncedAt,omitempty"`
	SyncError   string     `json:"syncError,omitempty"`
}

// Save connects the member's mailbox, replacing an earlier connection: sync starts over from now.
func (c *Connections) Save(ctx context.Context, memberID, email, refreshToken string) error {
	row := connection{MemberID: memberID, Email: strings.ToLower(email), RefreshToken: c.box.Seal([]byte(refreshToken)), ConnectedAt: time.Now()}
	return c.db.WithContext(ctx).Clauses(clause.OnConflict{
		Columns:   []clause.Column{{Name: "member_id"}},
		DoUpdates: clause.Assignments(map[string]any{"email": row.Email, "refresh_token": row.RefreshToken, "history_id": nil, "connected_at": row.ConnectedAt, "synced_at": nil, "sync_error": "", "lease_until": nil, "backfill_history": nil, "backfill_page": nil}),
	}).Create(&row).Error
}

func (c *Connections) Status(ctx context.Context, memberID string) (Status, error) {
	var row connection
	err := c.db.WithContext(ctx).Where("member_id = ?", memberID).Take(&row).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return Status{}, nil
	}
	if err != nil {
		return Status{}, err
	}
	return Status{Connected: true, Email: row.Email, ConnectedAt: &row.ConnectedAt, SyncedAt: row.SyncedAt, SyncError: row.SyncError}, nil
}

// Disconnect forgets the member's mailbox, then asks Google to withdraw the grant. The token is gone
// from the CRM even if Google can't be reached; the grant then lapses when the member removes the app
// from their Google account.
func (c *Connections) Disconnect(ctx context.Context, memberID string) error {
	var row connection
	err := c.db.WithContext(ctx).Clauses(clause.Returning{}).Where("member_id = ?", memberID).Delete(&row).Error
	if err != nil || len(row.RefreshToken) == 0 {
		return err
	}
	token, err := c.box.Open(row.RefreshToken)
	if err != nil {
		log.Printf("gmail: can't open %s's token to revoke it: %v", row.Email, err)
		return nil
	}
	if err := c.revoke(ctx, string(token)); err != nil {
		log.Printf("gmail: revoking %s's grant: %v", row.Email, err)
	}
	return nil
}

func (c *Connections) revoke(ctx context.Context, token string) error {
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, RevokeURL, strings.NewReader(url.Values{"token": {token}}.Encode()))
	if err != nil {
		return err
	}
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	resp, err := c.client.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return errors.New("google answered " + resp.Status)
	}
	return nil
}

// ConnectedMembers is the ids of members with a connected mailbox, for the Team page.
func (c *Connections) ConnectedMembers(ctx context.Context) (map[string]bool, error) {
	var ids []string
	if err := c.db.WithContext(ctx).Model(&connection{}).Pluck("member_id", &ids).Error; err != nil {
		return nil, err
	}
	connected := make(map[string]bool, len(ids))
	for _, id := range ids {
		connected[id] = true
	}
	return connected, nil
}

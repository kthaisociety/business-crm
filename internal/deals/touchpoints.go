package deals

import (
	"context"
	"strings"
	"time"

	"gorm.io/gorm"
	"gorm.io/gorm/clause"
)

// Touchpoint is an email or a meeting from email sync or Fathom (docs/adr/0006), before it's matched to
// deals.
type Touchpoint struct {
	Type         string // email or meeting
	Source       string // email_sync or fathom
	ExternalID   string // the same email or meeting always has the same one
	Title        string
	Body         string
	BulletPoints []string
	URL          string
	OccurredAt   time.Time
	// Emails of everyone on it, matched to contacts.
	Emails []string
	// The team member whose mailbox or recording it is, by email: its author, if they're on the team.
	By string
	// An email's sender. Sent by someone on the team, it's "sent" and they're its author; otherwise
	// "received". So the same email reads the same way whichever member's mailbox it was found in.
	From string
	// Guard, if set, runs first in the transaction that records the touchpoint, and stops it with its
	// error: e.g. a Gmail sync checking the mailbox is still connected, so nothing lands after the
	// member is removed or disconnects.
	Guard func(tx *gorm.DB) error
}

// Record puts a touchpoint on the deals of every company with a contact on it: the company's open deals,
// or its latest deal when none is open. A team member's address never counts as a contact, even if
// someone added it as one: they're on every email in their own mailbox. It returns how many deals it was added to or updated on: 0 with
// no contact on it, when nothing is stored. The same touchpoint recorded again updates it in place, so a
// Fathom summary written again replaces the old one, and it's never there twice.
func (s *Store) Record(ctx context.Context, t Touchpoint) (int, error) {
	emails := make([]string, 0, len(t.Emails))
	for _, e := range t.Emails {
		if e = strings.ToLower(strings.TrimSpace(e)); e != "" {
			emails = append(emails, e)
		}
	}
	if len(emails) == 0 {
		return 0, nil
	}
	recorded := 0
	err := s.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		if t.Guard != nil {
			if err := t.Guard(tx); err != nil {
				return err
			}
		}
		var dealIDs []string
		err := tx.Raw(`
			WITH matched AS (
				SELECT DISTINCT company_id FROM contacts
				WHERE lower(email) IN ? AND lower(email) NOT IN (SELECT email FROM members)
			)
			SELECT d.id FROM deals d JOIN matched m ON m.company_id = d.company_id
			WHERE d.status NOT IN ('finished', 'canceled')
			UNION
			SELECT latest.id FROM matched m CROSS JOIN LATERAL (
				SELECT id FROM deals WHERE company_id = m.company_id ORDER BY created_at DESC, id DESC LIMIT 1
			) latest
			WHERE NOT EXISTS (
				SELECT 1 FROM deals WHERE company_id = m.company_id AND status NOT IN ('finished', 'canceled')
			)`, emails).Scan(&dealIDs).Error
		if err != nil || len(dealIDs) == 0 {
			return err
		}
		var author *string
		direction := ""
		if t.Type == "email" {
			direction = "received"
			sender, err := memberByEmail(tx, t.From)
			if err != nil {
				return err
			}
			if sender != nil {
				direction, author = "sent", sender
			}
		}
		if author == nil {
			by, err := memberByEmail(tx, t.By)
			if err != nil {
				return err
			}
			author = by
		}
		points := t.BulletPoints
		if points == nil {
			points = []string{}
		}
		for _, dealID := range dealIDs {
			res := tx.Clauses(clause.OnConflict{
				Columns:   []clause.Column{{Name: "deal_id"}, {Name: "source"}, {Name: "external_id"}},
				DoUpdates: clause.AssignmentColumns([]string{"title", "body", "bullet_points", "url", "occurred_at"}),
			}).Create(&Activity{
				DealID: dealID, Type: t.Type, Source: t.Source, ExternalID: &t.ExternalID, AuthorID: author,
				Title: t.Title, Body: t.Body, BulletPoints: points, URL: t.URL, OccurredAt: t.OccurredAt,
				Direction: direction,
			})
			if res.Error != nil {
				return res.Error
			}
			recorded += int(res.RowsAffected)
		}
		return nil
	})
	return recorded, err
}

// memberByEmail is the id of the team member with this email, or nil.
func memberByEmail(tx *gorm.DB, email string) (*string, error) {
	email = strings.ToLower(strings.TrimSpace(email))
	if email == "" {
		return nil, nil
	}
	var m Member
	if err := tx.Where("email = ?", email).Limit(1).Find(&m).Error; err != nil {
		return nil, err
	}
	if m.ID == "" {
		return nil, nil
	}
	return &m.ID, nil
}

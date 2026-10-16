package gmail

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"html"
	"log"
	"net/http"
	"net/mail"
	"net/url"
	"regexp"
	"slices"
	"strconv"
	"strings"
	"time"

	"golang.org/x/oauth2"
	"gorm.io/gorm"
	"gorm.io/gorm/clause"

	"github.com/kthaisociety/business-crm/internal/deals"
)

// API is the Gmail REST API for the mailbox a client acts for; tests point it at a fake.
var API = "https://gmail.googleapis.com/gmail/v1/users/me"

// Backfill is how far back a mailbox's first sync looks, so conversations already under way show up.
const Backfill = 30 * 24 * time.Hour

// Lease is how long a replica has a mailbox to itself; SyncOne gives up before then.
const Lease = 15 * time.Minute

// syncTimeout bounds one mailbox's sync, so a slow mailbox can't hold up the others for long.
var syncTimeout = 10 * time.Minute

// NeedsReconnect is the sync error of a mailbox whose grant Google no longer honours: revoked, or its
// password changed. Only connecting again fixes it.
const NeedsReconnect = "Google no longer lets Lucid read this mailbox: connect Gmail again"

// Syncer brings new mail with contacts on it into the CRM (docs/adr/0006). Each mailbox carries on from
// its Gmail history id; replicas take different mailboxes at a time.
type Syncer struct {
	conns  *Connections
	client func(ctx context.Context, refreshToken string) *http.Client
	record func(ctx context.Context, t deals.Touchpoint) (int, error)
	kick   chan struct{}
}

// NewSyncer syncs conns' mailboxes with client, recording what it finds in store.
func NewSyncer(conns *Connections, client func(ctx context.Context, refreshToken string) *http.Client, store *deals.Store) *Syncer {
	return &Syncer{conns: conns, client: client, record: store.Record, kick: make(chan struct{}, 1)}
}

// Run syncs every mailbox every interval, and right away when kicked, until ctx ends.
func (s *Syncer) Run(ctx context.Context, every time.Duration) {
	tick := time.NewTicker(every)
	defer tick.Stop()
	for {
		if err := s.SyncAll(ctx); err != nil && ctx.Err() == nil {
			log.Printf("gmail sync: %v", err)
		}
		select {
		case <-ctx.Done():
			return
		case <-tick.C:
		case <-s.kick:
		}
	}
}

// Kick asks for a sync now, e.g. right after someone connects, rather than at the next tick.
func (s *Syncer) Kick() {
	select {
	case s.kick <- struct{}{}:
	default:
	}
}

// SyncAll syncs each connected mailbox once. One mailbox failing doesn't stop the others: its error is
// kept on its connection, for its member to see.
func (s *Syncer) SyncAll(ctx context.Context) error {
	var ids []string
	if err := s.conns.db.WithContext(ctx).Model(&connection{}).Pluck("member_id", &ids).Error; err != nil {
		return err
	}
	for _, id := range ids {
		if err := s.SyncOne(ctx, id); err != nil {
			return err
		}
	}
	return nil
}

// SyncOne syncs one mailbox, unless another replica has it. It holds no lock while it talks to Gmail:
// it takes a lease, so disconnecting, reconnecting or removing the member never waits on the sync. Only
// database errors are returned; Gmail's are kept on the connection.
func (s *Syncer) SyncOne(ctx context.Context, memberID string) error {
	db := s.conns.db.WithContext(ctx)
	var row connection
	res := db.Clauses(clause.Returning{}).Model(&row).
		Where("member_id = ? AND (lease_until IS NULL OR lease_until < now())", memberID).
		Update("lease_until", gorm.Expr("now() + make_interval(secs => ?)", Lease.Seconds()))
	if res.Error != nil || res.RowsAffected == 0 {
		return res.Error // disconnected meanwhile, or another replica has it
	}
	// The connection as it was when leased: if it's been replaced since (reconnected), what's learnt here
	// is about the old one and isn't written back.
	same := db.Model(&connection{}).Where("member_id = ? AND connected_at = ?", row.MemberID, row.ConnectedAt)
	token, err := s.conns.box.Open(row.RefreshToken)
	if err != nil {
		return same.Updates(map[string]any{"sync_error": "the stored token can't be read: connect Gmail again", "lease_until": nil}).Error
	}
	syncCtx, cancel := context.WithTimeout(ctx, syncTimeout)
	defer cancel()
	mailbox := &mailbox{http: s.client(syncCtx, string(token)), email: row.Email}
	history, err := s.sync(syncCtx, mailbox, row.HistoryID)
	if err != nil {
		log.Printf("gmail sync of %s: %v", row.Email, err)
		return same.Updates(map[string]any{"sync_error": syncError(err), "lease_until": nil}).Error
	}
	return same.Updates(map[string]any{"history_id": history, "synced_at": time.Now(), "sync_error": "", "lease_until": nil}).Error
}

// sync records the mailbox's mail since history, or its last Backfill when there's no usable history,
// and returns the history id to carry on from.
func (s *Syncer) sync(ctx context.Context, m *mailbox, history *string) (string, error) {
	var ids []string
	var next string
	var err error
	if history != nil {
		ids, next, err = m.since(ctx, *history)
	}
	if history == nil || errors.Is(err, errHistoryGone) {
		// Taken before listing, so nothing that arrives meanwhile is missed.
		if next, err = m.historyID(ctx); err != nil {
			return "", err
		}
		ids, err = m.recent(ctx, Backfill)
	}
	if err != nil {
		return "", err
	}
	for _, id := range ids {
		msg, err := m.message(ctx, id)
		if errors.Is(err, errGone) {
			continue // deleted since it was listed
		}
		if err != nil {
			return "", err
		}
		if t, ok := touchpointOf(msg, m.email); ok {
			if _, err := s.record(ctx, t); err != nil {
				return "", err
			}
		}
	}
	return next, nil
}

func syncError(err error) string {
	var retrieve *oauth2.RetrieveError
	if errors.As(err, &retrieve) && retrieve.ErrorCode == "invalid_grant" {
		return NeedsReconnect
	}
	return "Gmail sync failed; it will try again in a few minutes"
}

// A message as Gmail's format=metadata gives it: headers and a snippet, no body.
type message struct {
	ID           string   `json:"id"`
	ThreadID     string   `json:"threadId"`
	LabelIDs     []string `json:"labelIds"`
	Snippet      string   `json:"snippet"`
	InternalDate string   `json:"internalDate"`
	Payload      struct {
		Headers []struct {
			Name  string `json:"name"`
			Value string `json:"value"`
		} `json:"headers"`
	} `json:"payload"`
}

func (m message) header(name string) string {
	for _, h := range m.Payload.Headers {
		if strings.EqualFold(h.Name, name) {
			return strings.TrimSpace(h.Value)
		}
	}
	return ""
}

// touchpointOf is the touchpoint an email makes, or false for mail that never counts: drafts, spam,
// trash and chats. Whether it has a contact on it is the deals store's call.
func touchpointOf(m message, mailboxEmail string) (deals.Touchpoint, bool) {
	if slices.ContainsFunc(m.LabelIDs, func(l string) bool { return l == "DRAFT" || l == "SPAM" || l == "TRASH" || l == "CHAT" }) {
		return deals.Touchpoint{}, false
	}
	from := addresses(m.header("From"))
	var emails []string
	for _, h := range []string{"From", "To", "Cc"} {
		emails = append(emails, addresses(m.header(h))...)
	}
	if len(from) == 0 || len(emails) == 0 {
		return deals.Touchpoint{}, false
	}
	id := strings.Trim(m.header("Message-ID"), "<>")
	if id == "" {
		id = "gmail:" + m.ID
	}
	title := m.header("Subject")
	if title == "" {
		title = "(no subject)"
	}
	ms, _ := strconv.ParseInt(m.InternalDate, 10, 64)
	return deals.Touchpoint{
		Type:       "email",
		Source:     "email_sync",
		ExternalID: id,
		Title:      title,
		Body:       html.UnescapeString(m.Snippet),
		URL:        "https://mail.google.com/mail/?authuser=" + url.QueryEscape(mailboxEmail) + "#all/" + m.ThreadID,
		OccurredAt: time.UnixMilli(ms).UTC(),
		Emails:     emails,
		By:         mailboxEmail,
		From:       from[0],
	}, true
}

var looseAddress = regexp.MustCompile(`[A-Za-z0-9._%+'-]+@[A-Za-z0-9.-]+\.[A-Za-z]{2,}`)

// addresses reads the email addresses in a From, To or Cc header, lower-cased. Headers that don't parse
// strictly still give the addresses that can be picked out of them.
func addresses(header string) []string {
	if header == "" {
		return nil
	}
	var out []string
	if list, err := mail.ParseAddressList(header); err == nil {
		for _, a := range list {
			out = append(out, strings.ToLower(a.Address))
		}
		return out
	}
	for _, a := range looseAddress.FindAllString(header, -1) {
		out = append(out, strings.ToLower(a))
	}
	return out
}

// mailbox calls Gmail's API as one member.
type mailbox struct {
	http  *http.Client
	email string
}

var (
	errHistoryGone = errors.New("gmail no longer has history that old")
	errGone        = errors.New("not found")
)

func (m *mailbox) get(ctx context.Context, path string, query url.Values, into any) error {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, API+path+"?"+query.Encode(), nil)
	if err != nil {
		return err
	}
	resp, err := m.http.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	if resp.StatusCode == http.StatusNotFound {
		return errGone
	}
	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("gmail %s: %s", path, resp.Status)
	}
	return json.NewDecoder(resp.Body).Decode(into)
}

func (m *mailbox) historyID(ctx context.Context) (string, error) {
	var profile struct {
		HistoryID string `json:"historyId"`
	}
	err := m.get(ctx, "/profile", nil, &profile)
	return profile.HistoryID, err
}

// recent lists the ids of messages from the last `back`.
func (m *mailbox) recent(ctx context.Context, back time.Duration) ([]string, error) {
	var ids []string
	q := url.Values{"q": {fmt.Sprintf("newer_than:%dd", int(back.Hours()/24))}, "maxResults": {"500"}}
	for {
		var page struct {
			Messages []struct {
				ID string `json:"id"`
			} `json:"messages"`
			NextPageToken string `json:"nextPageToken"`
		}
		if err := m.get(ctx, "/messages", q, &page); err != nil {
			return nil, err
		}
		for _, msg := range page.Messages {
			ids = append(ids, msg.ID)
		}
		if page.NextPageToken == "" {
			return ids, nil
		}
		q.Set("pageToken", page.NextPageToken)
	}
}

// since lists the ids of messages added after history, or taken out of spam or trash since (they were
// skipped while there), and the history id to carry on from.
func (m *mailbox) since(ctx context.Context, history string) ([]string, string, error) {
	var ids []string
	next := history
	q := url.Values{"startHistoryId": {history}, "historyTypes": {"messageAdded", "labelRemoved"}, "maxResults": {"500"}}
	add := func(id string) {
		if !slices.Contains(ids, id) {
			ids = append(ids, id)
		}
	}
	for {
		var page struct {
			History []struct {
				MessagesAdded []struct {
					Message struct {
						ID string `json:"id"`
					} `json:"message"`
				} `json:"messagesAdded"`
				LabelsRemoved []struct {
					Message struct {
						ID string `json:"id"`
					} `json:"message"`
					LabelIDs []string `json:"labelIds"`
				} `json:"labelsRemoved"`
			} `json:"history"`
			HistoryID     string `json:"historyId"`
			NextPageToken string `json:"nextPageToken"`
		}
		err := m.get(ctx, "/history", q, &page)
		if errors.Is(err, errGone) {
			return nil, "", errHistoryGone
		}
		if err != nil {
			return nil, "", err
		}
		for _, h := range page.History {
			for _, added := range h.MessagesAdded {
				add(added.Message.ID)
			}
			for _, removed := range h.LabelsRemoved {
				if slices.Contains(removed.LabelIDs, "SPAM") || slices.Contains(removed.LabelIDs, "TRASH") {
					add(removed.Message.ID)
				}
			}
		}
		if page.HistoryID != "" {
			next = page.HistoryID
		}
		if page.NextPageToken == "" {
			return ids, next, nil
		}
		q.Set("pageToken", page.NextPageToken)
	}
}

func (m *mailbox) message(ctx context.Context, id string) (message, error) {
	var msg message
	q := url.Values{"format": {"metadata"}, "metadataHeaders": {"From", "To", "Cc", "Subject", "Message-ID"}}
	err := m.get(ctx, "/messages/"+url.PathEscape(id), q, &msg)
	return msg, err
}

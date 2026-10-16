package gmail_test

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"golang.org/x/oauth2"
	"gorm.io/gorm"

	"github.com/kthaisociety/business-crm/internal/auth"
	"github.com/kthaisociety/business-crm/internal/database/databasetest"
	"github.com/kthaisociety/business-crm/internal/deals"
	"github.com/kthaisociety/business-crm/internal/gmail"
	"github.com/kthaisociety/business-crm/internal/secretbox"
)

// fakeGmail is Gmail's API for test mailboxes, keyed by the refresh token each mailbox was connected with.
type fakeGmail struct {
	mu       sync.Mutex
	messages map[string]map[string]any // token → message id → message
	recent   map[string][]string       // token → ids the backfill lists
	added    map[string][]string       // token → ids added since the last history id
	history  string                    // the history id every mailbox is at
	gone     bool                      // history too old to answer
}

func newFakeGmail(t *testing.T) *fakeGmail {
	f := &fakeGmail{messages: map[string]map[string]any{}, recent: map[string][]string{}, added: map[string][]string{}, history: "100"}
	srv := httptest.NewServer(http.HandlerFunc(f.serve))
	t.Cleanup(srv.Close)
	before := gmail.API
	gmail.API = srv.URL
	t.Cleanup(func() { gmail.API = before })
	return f
}

// mail adds a message to a mailbox, listed by the backfill or, with added, as new since the last sync.
func (f *fakeGmail) mail(token, id, from, to, subject string, labels []string, added bool) {
	f.mu.Lock()
	defer f.mu.Unlock()
	if f.messages[token] == nil {
		f.messages[token] = map[string]any{}
	}
	f.messages[token][id] = map[string]any{
		"id": id, "threadId": "thread-" + id, "labelIds": labels, "snippet": "It&#39;s about " + subject,
		"internalDate": "1792000000000",
		"payload": map[string]any{"headers": []map[string]string{
			{"name": "From", "value": from}, {"name": "To", "value": to},
			{"name": "Subject", "value": subject}, {"name": "Message-ID", "value": "<" + id + "@mail.example>"},
		}},
	}
	if added {
		f.added[token] = append(f.added[token], id)
	} else {
		f.recent[token] = append(f.recent[token], id)
	}
}

func (f *fakeGmail) serve(w http.ResponseWriter, r *http.Request) {
	f.mu.Lock()
	defer f.mu.Unlock()
	token := r.Header.Get("X-Test-Token")
	write := func(v any) { _ = json.NewEncoder(w).Encode(v) }
	ids := func(list []string) []map[string]string {
		out := []map[string]string{}
		for _, id := range list {
			out = append(out, map[string]string{"id": id})
		}
		return out
	}
	switch {
	case r.URL.Path == "/profile":
		write(map[string]string{"historyId": f.history})
	case r.URL.Path == "/messages":
		write(map[string]any{"messages": ids(f.recent[token])})
	case r.URL.Path == "/history":
		if f.gone {
			http.Error(w, "too old", http.StatusNotFound)
			return
		}
		var added []map[string]any
		for _, id := range f.added[token] {
			added = append(added, map[string]any{"message": map[string]string{"id": id}})
		}
		write(map[string]any{"history": []map[string]any{{"messagesAdded": added}}, "historyId": f.history})
	case strings.HasPrefix(r.URL.Path, "/messages/"):
		msg, ok := f.messages[token][strings.TrimPrefix(r.URL.Path, "/messages/")]
		if !ok {
			http.NotFound(w, r)
			return
		}
		write(msg)
	default:
		http.NotFound(w, r)
	}
}

// tokenTransport sends a mailbox's refresh token where the fake can see which mailbox it is, or fails
// like a withdrawn grant.
type tokenTransport struct {
	token   string
	revoked bool
}

func (t tokenTransport) RoundTrip(r *http.Request) (*http.Response, error) {
	if t.revoked {
		return nil, &oauth2.RetrieveError{ErrorCode: "invalid_grant"}
	}
	r = r.Clone(r.Context())
	r.Header.Set("X-Test-Token", t.token)
	return http.DefaultTransport.RoundTrip(r)
}

type world struct {
	db      *gorm.DB
	store   *deals.Store
	conns   *gmail.Connections
	syncer  *gmail.Syncer
	revoked map[string]bool
	members map[string]string // email → id
	granit  deals.View
}

var dev = auth.User{Subject: "dev", Email: "dev@example.com", Name: "Dev"}

func newWorld(t *testing.T) *world {
	db := databasetest.Open(t)
	box, err := secretbox.New("BwcHBwcHBwcHBwcHBwcHBwcHBwcHBwcHBwcHBwcHBwc=")
	require.NoError(t, err)
	w := &world{db: db, store: deals.NewStore(db), conns: gmail.NewConnections(db, box), revoked: map[string]bool{}, members: map[string]string{}}
	w.syncer = gmail.NewSyncer(w.conns, func(_ context.Context, token string) *http.Client {
		return &http.Client{Transport: tokenTransport{token: token, revoked: w.revoked[token]}}
	}, w.store)
	ctx := context.Background()
	for _, email := range []string{"maja@example.com", "noah@example.com"} {
		m, err := w.store.AddMember(ctx, deals.MemberInput{Email: email, Name: strings.Split(email, "@")[0]})
		require.NoError(t, err)
		w.members[email] = m.ID
	}
	w.granit, err = w.store.Create(ctx, deals.NewLead{
		Company: "Granit Consulting", DealType: "Event", Semester: "HT 2026",
		Contact: &deals.ContactInput{Name: "Lina Dahl", Email: "lina.dahl@granit.example"},
	}, dev)
	require.NoError(t, err)
	return w
}

func (w *world) connect(t *testing.T, email string) {
	t.Helper()
	require.NoError(t, w.conns.Save(context.Background(), w.members[email], email, "token-"+email))
}

func (w *world) emails(t *testing.T) []deals.ActivityView {
	t.Helper()
	v, err := w.store.Get(context.Background(), w.granit.ID)
	require.NoError(t, err)
	var out []deals.ActivityView
	for _, a := range v.Activities {
		if a.Type == "email" {
			out = append(out, a)
		}
	}
	return out
}

func (w *world) status(t *testing.T, email string) gmail.Status {
	t.Helper()
	s, err := w.conns.Status(context.Background(), w.members[email])
	require.NoError(t, err)
	return s
}

func TestTheFirstSyncBringsInRecentMailWithContacts(t *testing.T) {
	f := newFakeGmail(t)
	w := newWorld(t)
	w.connect(t, "maja@example.com")
	tok := "token-maja@example.com"
	f.mail(tok, "m1", "Lina Dahl <Lina.Dahl@granit.example>", "maja@example.com", "Lunch lecture", []string{"INBOX"}, false)
	f.mail(tok, "m2", "Maja <maja@example.com>", "lina.dahl@granit.example", "Re: Lunch lecture", []string{"SENT"}, false)
	f.mail(tok, "m3", "news@shop.example", "maja@example.com", "Sale!", []string{"INBOX"}, false)
	f.mail(tok, "m4", "maja@example.com", "lina.dahl@granit.example", "Unsent", []string{"DRAFT"}, false)

	require.NoError(t, w.syncer.SyncAll(context.Background()))

	emails := w.emails(t)
	require.Len(t, emails, 2, "the newsletter has no contact; the draft was never sent")
	byTitle := map[string]deals.ActivityView{}
	for _, e := range emails {
		byTitle[e.Title] = e
	}
	assert.Equal(t, "received", byTitle["Lunch lecture"].Direction)
	assert.Equal(t, "sent", byTitle["Re: Lunch lecture"].Direction)
	assert.Equal(t, "email_sync", byTitle["Lunch lecture"].Source)
	assert.Equal(t, "It's about Lunch lecture", byTitle["Lunch lecture"].Body)
	assert.Equal(t, "https://mail.google.com/mail/?authuser=maja%40example.com#all/thread-m1", byTitle["Lunch lecture"].URL)
	assert.Equal(t, "2026-10-14T17:46:40Z", byTitle["Lunch lecture"].Timestamp)

	s := w.status(t, "maja@example.com")
	assert.NotNil(t, s.SyncedAt)
	assert.Empty(t, s.SyncError)
}

func TestLaterSyncsBringOnlyNewMail(t *testing.T) {
	f := newFakeGmail(t)
	w := newWorld(t)
	w.connect(t, "maja@example.com")
	tok := "token-maja@example.com"
	f.mail(tok, "m1", "lina.dahl@granit.example", "maja@example.com", "First", nil, false)
	require.NoError(t, w.syncer.SyncAll(context.Background()))

	f.mail(tok, "m5", "lina.dahl@granit.example", "maja@example.com", "Second", nil, true)
	f.history = "200"
	require.NoError(t, w.syncer.SyncAll(context.Background()))

	assert.Len(t, w.emails(t), 2)
	var history string
	require.NoError(t, w.db.Raw("SELECT history_id FROM gmail_connections").Scan(&history).Error)
	assert.Equal(t, "200", history)
}

func TestWhenGmailForgetsTheHistoryTheSyncStartsOver(t *testing.T) {
	f := newFakeGmail(t)
	w := newWorld(t)
	w.connect(t, "maja@example.com")
	tok := "token-maja@example.com"
	f.mail(tok, "m1", "lina.dahl@granit.example", "maja@example.com", "First", nil, false)
	require.NoError(t, w.syncer.SyncAll(context.Background()))

	f.gone = true
	require.NoError(t, w.syncer.SyncAll(context.Background()))

	assert.Len(t, w.emails(t), 1, "the backfill again, without doubles")
	assert.Empty(t, w.status(t, "maja@example.com").SyncError)
}

func TestAnEmailInTwoMailboxesIsOneTouchpointThatReadsTheSame(t *testing.T) {
	f := newFakeGmail(t)
	w := newWorld(t)
	w.connect(t, "maja@example.com")
	w.connect(t, "noah@example.com")
	// Maja writes to Lina and copies Noah: sent, in both mailboxes.
	for _, tok := range []string{"token-maja@example.com", "token-noah@example.com"} {
		f.mail(tok, "m1", "maja@example.com", "lina.dahl@granit.example, Noah <noah@example.com>", "Proposal", nil, false)
	}

	require.NoError(t, w.syncer.SyncAll(context.Background()))

	emails := w.emails(t)
	require.Len(t, emails, 1)
	assert.Equal(t, "sent", emails[0].Direction)
}

func TestAWithdrawnGrantAsksToReconnect(t *testing.T) {
	newFakeGmail(t)
	w := newWorld(t)
	w.connect(t, "maja@example.com")
	w.revoked["token-maja@example.com"] = true

	require.NoError(t, w.syncer.SyncAll(context.Background()))

	assert.Equal(t, gmail.NeedsReconnect, w.status(t, "maja@example.com").SyncError)
}

func TestAMailboxBeingSyncedElsewhereIsSkipped(t *testing.T) {
	f := newFakeGmail(t)
	w := newWorld(t)
	w.connect(t, "maja@example.com")
	f.mail("token-maja@example.com", "m1", "lina.dahl@granit.example", "maja@example.com", "First", nil, false)
	other := w.db.Begin()
	defer other.Rollback()
	require.NoError(t, other.Exec("SELECT 1 FROM gmail_connections FOR UPDATE").Error)

	done := make(chan error, 1)
	go func() { done <- w.syncer.SyncOne(context.Background(), w.members["maja@example.com"]) }()
	select {
	case err := <-done:
		require.NoError(t, err)
	case <-time.After(5 * time.Second):
		t.Fatal("the sync waited for the other replica instead of skipping")
	}
	assert.Empty(t, w.emails(t))
}


// A member who is also on the contact list, by mistake or to test, would otherwise match all their mail.
func TestATeamMembersAddressNeverCountsAsAContact(t *testing.T) {
	f := newFakeGmail(t)
	w := newWorld(t)
	w.connect(t, "maja@example.com")
	_, err := w.store.CreateContact(context.Background(), deals.ContactInput{CompanyID: w.granit.CompanyID, Name: "Maja", Email: "maja@example.com"})
	require.NoError(t, err)
	f.mail("token-maja@example.com", "m1", "friend@elsewhere.example", "maja@example.com", "Weekend plans", nil, false)

	require.NoError(t, w.syncer.SyncAll(context.Background()))

	assert.Empty(t, w.emails(t))
}

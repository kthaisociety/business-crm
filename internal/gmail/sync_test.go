package gmail_test

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"slices"
	"strconv"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"golang.org/x/oauth2"
	"gorm.io/gorm"

	"github.com/kthaisociety/lucid/internal/auth"
	"github.com/kthaisociety/lucid/internal/database/databasetest"
	"github.com/kthaisociety/lucid/internal/deals"
	"github.com/kthaisociety/lucid/internal/gmail"
	"github.com/kthaisociety/lucid/internal/secretbox"
)

// fakeGmail is Gmail's API for test mailboxes, keyed by the refresh token each mailbox was connected
// with. Like Gmail it numbers changes, answers history only from the id it's asked for, pages its lists,
// and checks what it's asked.
type fakeGmail struct {
	t        *testing.T
	mu       sync.Mutex
	messages map[string]map[string]map[string]any // token → message id → message
	recent   map[string][]string                  // token → ids the backfill lists, newest first
	received map[string]time.Time                 // message id → when it came in
	changes  map[string][]change                  // token → changes, in order
	seq      int                                  // the latest change's history id
	oldest   int                                  // history before this is gone
	pageSize int                                  // 0: everything on one page
	failPage string                               // a pageToken to answer 500 once
	asked    []string                             // startHistoryIds asked for
	pages    []string                             // backfill pageTokens asked for
	fetched  map[string]int                       // message id → times fetched
	stalled  chan struct{}                        // while open, requests hang until it's closed or they're cancelled
	waiting  chan struct{}                        // gets a value when a request starts hanging
	stallOn  string                               // only requests for this path hang; "" all of them
}

type change struct {
	seq      int
	id       string
	restored bool // taken out of spam or trash, rather than new
}

func newFakeGmail(t *testing.T) *fakeGmail {
	f := &fakeGmail{t: t, messages: map[string]map[string]map[string]any{}, recent: map[string][]string{}, changes: map[string][]change{}, seq: 100, fetched: map[string]int{}, received: map[string]time.Time{}}
	srv := httptest.NewServer(http.HandlerFunc(f.serve))
	t.Cleanup(srv.Close)
	before := gmail.API
	gmail.API = srv.URL
	t.Cleanup(func() { gmail.API = before })
	return f
}

// mail adds a message to a mailbox: listed by the backfill or, with added, as a new change.
func (f *fakeGmail) mail(token, id, from, to, subject string, labels []string, added bool) {
	f.mu.Lock()
	defer f.mu.Unlock()
	if f.messages[token] == nil {
		f.messages[token] = map[string]map[string]any{}
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
		f.seq++
		f.changes[token] = append(f.changes[token], change{seq: f.seq, id: id})
		f.received[id] = time.Now()
	} else {
		f.recent[token] = append(f.recent[token], id)
		f.received[id] = time.Now().Add(-time.Hour)
	}
}

// arrive is new mail coming in now: first in the mailbox's list, and a new change.
func (f *fakeGmail) arrive(token, id, from, to, subject string) {
	f.mail(token, id, from, to, subject, nil, true)
	f.mu.Lock()
	defer f.mu.Unlock()
	f.recent[token] = append([]string{id}, f.recent[token]...)
	f.received[id] = time.Now().Add(time.Minute)
}

// remove deletes a message from the mailbox.
func (f *fakeGmail) remove(token, id string) {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.recent[token] = slices.DeleteFunc(f.recent[token], func(m string) bool { return m == id })
	delete(f.messages[token], id)
}

// restore takes a message out of spam or trash, into the inbox.
func (f *fakeGmail) restore(token, id string) {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.messages[token][id]["labelIds"] = []string{"INBOX"}
	f.seq++
	f.changes[token] = append(f.changes[token], change{seq: f.seq, id: id, restored: true})
}

// page cuts a list of n at pageToken, a plain offset here, and gives the next page's token.
func (f *fakeGmail) page(n int, pageToken string) (from, to int, next string) {
	from, _ = strconv.Atoi(pageToken)
	to = n
	if f.pageSize > 0 && from+f.pageSize < n {
		to = from + f.pageSize
		next = strconv.Itoa(to)
	}
	return from, to, next
}

func (f *fakeGmail) serve(w http.ResponseWriter, r *http.Request) {
	f.mu.Lock()
	stalled, waiting := f.stalled, f.waiting
	if f.stallOn != "" && r.URL.Path != f.stallOn {
		stalled = nil
	}
	f.mu.Unlock()
	if stalled != nil {
		waiting <- struct{}{}
		select {
		case <-stalled:
		case <-r.Context().Done():
			return
		}
	}
	f.mu.Lock()
	defer f.mu.Unlock()
	token, q := r.Header.Get("X-Test-Token"), r.URL.Query()
	write := func(v any) { _ = json.NewEncoder(w).Encode(v) }
	if pt := q.Get("pageToken"); pt != "" && pt == f.failPage {
		f.failPage = ""
		http.Error(w, "backend error", http.StatusInternalServerError)
		return
	}
	switch {
	case r.URL.Path == "/profile":
		write(map[string]string{"historyId": strconv.Itoa(f.seq)})
	case r.URL.Path == "/messages":
		var after, before int64
		if _, err := fmt.Sscanf(q.Get("q"), "after:%d before:%d", &after, &before); err != nil || before-after != 30*24*3600 {
			f.t.Errorf("backfill asked q=%q, not 30 days up to its start", q.Get("q"))
		}
		var list []string
		for _, id := range f.recent[token] {
			if at := f.received[id].Unix(); at >= after && at < before {
				list = append(list, id)
			}
		}
		f.pages = append(f.pages, q.Get("pageToken"))
		from, to, next := f.page(len(list), q.Get("pageToken"))
		page := []map[string]string{}
		for _, id := range list[from:to] {
			page = append(page, map[string]string{"id": id})
		}
		write(map[string]any{"messages": page, "nextPageToken": next})
	case r.URL.Path == "/history":
		start, err := strconv.Atoi(q.Get("startHistoryId"))
		if err != nil || !slices.Contains(q["historyTypes"], "messageAdded") || !slices.Contains(q["historyTypes"], "labelRemoved") {
			http.Error(w, "bad history request", http.StatusBadRequest)
			return
		}
		if q.Get("pageToken") == "" {
			f.asked = append(f.asked, q.Get("startHistoryId"))
		}
		if start < f.oldest {
			http.Error(w, "too old", http.StatusNotFound)
			return
		}
		var since []change
		for _, c := range f.changes[token] {
			if c.seq > start {
				since = append(since, c)
			}
		}
		from, to, next := f.page(len(since), q.Get("pageToken"))
		history := []map[string]any{}
		for _, c := range since[from:to] {
			msg, id := map[string]string{"id": c.id}, strconv.Itoa(c.seq)
			if c.restored {
				history = append(history, map[string]any{"id": id, "labelsRemoved": []map[string]any{{"message": msg, "labelIds": []string{"SPAM"}}}})
			} else {
				history = append(history, map[string]any{"id": id, "messagesAdded": []map[string]any{{"message": msg}}})
			}
		}
		write(map[string]any{"history": history, "historyId": strconv.Itoa(f.seq), "nextPageToken": next})
	case strings.HasPrefix(r.URL.Path, "/messages/"):
		f.fetched[strings.TrimPrefix(r.URL.Path, "/messages/")]++
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

// stall makes requests hang; the returned func lets them go.
func (f *fakeGmail) stall() func() {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.stalled, f.waiting = make(chan struct{}), make(chan struct{}, 10)
	stalled := f.stalled
	var once sync.Once
	return func() { once.Do(func() { close(stalled) }) }
}

func (f *fakeGmail) waitForStalled(t *testing.T) {
	t.Helper()
	select {
	case <-f.waiting:
	case <-time.After(5 * time.Second):
		t.Fatal("no request reached Gmail")
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

func (w *world) historyID(t *testing.T) string {
	t.Helper()
	var row struct{ HistoryID string }
	require.NoError(t, w.db.Raw("SELECT history_id FROM gmail_connections").Scan(&row).Error)
	return row.HistoryID
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
	require.NoError(t, w.syncer.SyncAll(context.Background()))

	assert.Len(t, w.emails(t), 2)
	assert.Equal(t, []string{"100"}, f.asked, "carrying on from where the first sync left off")
	assert.Equal(t, "101", w.historyID(t))
}

func TestWhenGmailForgetsTheHistoryTheSyncStartsOver(t *testing.T) {
	f := newFakeGmail(t)
	w := newWorld(t)
	w.connect(t, "maja@example.com")
	tok := "token-maja@example.com"
	f.mail(tok, "m1", "lina.dahl@granit.example", "maja@example.com", "First", nil, false)
	require.NoError(t, w.syncer.SyncAll(context.Background()))

	f.oldest = f.seq + 1
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

func TestAMailboxAnotherReplicaHasIsSkipped(t *testing.T) {
	f := newFakeGmail(t)
	w := newWorld(t)
	w.connect(t, "maja@example.com")
	f.mail("token-maja@example.com", "m1", "lina.dahl@granit.example", "maja@example.com", "First", nil, false)
	require.NoError(t, w.db.Exec("UPDATE gmail_connections SET lease_until = now() + interval '1 minute'").Error)

	require.NoError(t, w.syncer.SyncAll(context.Background()))

	assert.Empty(t, w.emails(t))
	require.NoError(t, w.db.Exec("UPDATE gmail_connections SET lease_until = now() - interval '1 second'").Error)
	require.NoError(t, w.syncer.SyncAll(context.Background()))
	assert.Len(t, w.emails(t), 1, "an expired lease, from a replica that died, is taken over")
}

// A sync stuck on Gmail holds no lock: removing the member, disconnecting or reconnecting goes ahead.
func TestASyncWaitingOnGmailBlocksNothing(t *testing.T) {
	f := newFakeGmail(t)
	w := newWorld(t)
	w.connect(t, "maja@example.com")
	f.mail("token-maja@example.com", "m1", "lina.dahl@granit.example", "maja@example.com", "First", nil, false)
	release := f.stall()
	defer release()
	syncing := make(chan error, 1)
	go func() { syncing <- w.syncer.SyncOne(context.Background(), w.members["maja@example.com"]) }()
	f.waitForStalled(t)

	removed := make(chan error, 1)
	go func() { removed <- w.store.RemoveMember(context.Background(), w.members["maja@example.com"]) }()
	select {
	case err := <-removed:
		require.NoError(t, err)
	case <-time.After(5 * time.Second):
		t.Fatal("removing the member waited on the sync")
	}
	release()
	require.NoError(t, <-syncing)
	var left int64
	require.NoError(t, w.db.Raw("SELECT count(*) FROM gmail_connections").Scan(&left).Error)
	assert.Zero(t, left, "the finished sync doesn't bring the removed mailbox back")
}

func TestAGmailThatStopsAnsweringDoesntHoldUpTheSync(t *testing.T) {
	f := newFakeGmail(t)
	w := newWorld(t)
	w.connect(t, "maja@example.com")
	defer gmail.SetSyncTimeout(200 * time.Millisecond)()
	release := f.stall()
	defer release()

	start := time.Now()
	require.NoError(t, w.syncer.SyncAll(context.Background()))

	assert.Less(t, time.Since(start), 5*time.Second)
	s := w.status(t, "maja@example.com")
	assert.NotEmpty(t, s.SyncError)
	var row struct{ LeaseUntil *time.Time }
	require.NoError(t, w.db.Raw("SELECT lease_until FROM gmail_connections").Scan(&row).Error)
	assert.Nil(t, row.LeaseUntil, "the lease is let go, so the next run tries again")
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

func TestTheBackfillReadsEveryPage(t *testing.T) {
	f := newFakeGmail(t)
	w := newWorld(t)
	w.connect(t, "maja@example.com")
	f.pageSize = 2
	for _, id := range []string{"m1", "m2", "m3", "m4", "m5"} {
		f.mail("token-maja@example.com", id, "lina.dahl@granit.example", "maja@example.com", "Mail "+id, nil, false)
	}

	require.NoError(t, w.syncer.SyncAll(context.Background()))

	assert.Len(t, w.emails(t), 5)
}

func TestHistoryIsReadPageByPageFromTheCheckpoint(t *testing.T) {
	f := newFakeGmail(t)
	w := newWorld(t)
	w.connect(t, "maja@example.com")
	tok := "token-maja@example.com"
	require.NoError(t, w.syncer.SyncAll(context.Background()))
	f.pageSize = 2
	for _, id := range []string{"n1", "n2", "n3", "n4", "n5"} {
		f.mail(tok, id, "lina.dahl@granit.example", "maja@example.com", "New "+id, nil, true)
	}

	require.NoError(t, w.syncer.SyncAll(context.Background()))

	assert.Len(t, w.emails(t), 5)
	assert.Equal(t, "105", w.historyID(t))
}

// A page failing partway leaves the checkpoint after the last page that was recorded, so the next run
// carries on from there: it neither skips what it missed nor reads the rest again.
func TestAFailedPageKeepsTheCheckpointOfTheLastGoodPage(t *testing.T) {
	f := newFakeGmail(t)
	w := newWorld(t)
	w.connect(t, "maja@example.com")
	tok := "token-maja@example.com"
	require.NoError(t, w.syncer.SyncAll(context.Background()))
	f.pageSize, f.failPage = 2, "4"
	for _, id := range []string{"n1", "n2", "n3", "n4", "n5"} {
		f.mail(tok, id, "lina.dahl@granit.example", "maja@example.com", "New "+id, nil, true)
	}

	require.NoError(t, w.syncer.SyncAll(context.Background()))
	assert.Equal(t, "104", w.historyID(t), "two pages of two were recorded before the third failed")
	assert.NotEmpty(t, w.status(t, "maja@example.com").SyncError)
	assert.Len(t, w.emails(t), 4)

	require.NoError(t, w.syncer.SyncAll(context.Background()))
	assert.Len(t, w.emails(t), 5)
	assert.Equal(t, "105", w.historyID(t))
	assert.Equal(t, []string{"100", "104"}, f.asked)
}

func TestMailTakenOutOfSpamIsBroughtIn(t *testing.T) {
	f := newFakeGmail(t)
	w := newWorld(t)
	w.connect(t, "maja@example.com")
	tok := "token-maja@example.com"
	f.mail(tok, "m1", "lina.dahl@granit.example", "maja@example.com", "Caught as spam", []string{"SPAM"}, false)
	require.NoError(t, w.syncer.SyncAll(context.Background()))
	require.Empty(t, w.emails(t))

	f.restore(tok, "m1")
	require.NoError(t, w.syncer.SyncAll(context.Background()))

	assert.Len(t, w.emails(t), 1)
}

// Removing a member while their mailbox syncs stops the sync: nothing more is read or recorded.
func TestRemovingAMemberStopsTheirSyncMidway(t *testing.T) {
	f := newFakeGmail(t)
	w := newWorld(t)
	w.connect(t, "maja@example.com")
	tok := "token-maja@example.com"
	for _, id := range []string{"m1", "m2", "m3"} {
		f.mail(tok, id, "lina.dahl@granit.example", "maja@example.com", "Mail "+id, nil, false)
	}
	f.stallOn = "/messages/m2"
	release := f.stall()
	defer release()
	syncing := make(chan error, 1)
	go func() { syncing <- w.syncer.SyncOne(context.Background(), w.members["maja@example.com"]) }()
	f.waitForStalled(t)

	require.NoError(t, w.store.RemoveMember(context.Background(), w.members["maja@example.com"]))
	release()
	require.NoError(t, <-syncing)

	titles := []string{}
	for _, e := range w.emails(t) {
		titles = append(titles, e.Title)
	}
	assert.Equal(t, []string{"Mail m1"}, titles, "m2 was fetched as the member was removed, and isn't recorded")
	assert.Zero(t, f.fetched["m3"], "nothing more is read")
}

// A backfill cut off partway carries on from the page it reached, without fetching what it had again.
func TestAnUnfinishedBackfillCarriesOn(t *testing.T) {
	f := newFakeGmail(t)
	w := newWorld(t)
	w.connect(t, "maja@example.com")
	f.pageSize, f.failPage = 2, "4"
	for _, id := range []string{"m1", "m2", "m3", "m4", "m5"} {
		f.mail("token-maja@example.com", id, "lina.dahl@granit.example", "maja@example.com", "Mail "+id, nil, false)
	}

	require.NoError(t, w.syncer.SyncAll(context.Background()))
	assert.Len(t, w.emails(t), 4)
	assert.Empty(t, w.historyID(t), "no checkpoint until the backfill is done")

	require.NoError(t, w.syncer.SyncAll(context.Background()))
	assert.Len(t, w.emails(t), 5)
	assert.Equal(t, "100", w.historyID(t))
	// The failed request for page 4 isn't counted: the second run asks for it first.
	assert.Equal(t, []string{"", "2", "4"}, f.pages, "the second run starts at the page the first didn't finish")
	for _, id := range []string{"m1", "m2", "m3", "m4"} {
		assert.Equal(t, 1, f.fetched[id], id)
	}
}

// A page that takes longer than a sync's deadline still gets done: each run carries on from the message
// after the last one it recorded, never starting the page over.
func TestABackfillPageLongerThanTheDeadlineStillGetsDone(t *testing.T) {
	f := newFakeGmail(t)
	w := newWorld(t)
	w.connect(t, "maja@example.com")
	for _, id := range []string{"m1", "m2", "m3", "m4"} {
		f.mail("token-maja@example.com", id, "lina.dahl@granit.example", "maja@example.com", "Mail "+id, nil, false)
	}
	defer gmail.SetSyncTimeout(300 * time.Millisecond)()
	f.stallOn = "/messages/m3"
	release := f.stall()

	require.NoError(t, w.syncer.SyncAll(context.Background()))
	assert.Len(t, w.emails(t), 2, "m1 and m2 before the deadline")

	release()
	f.stallOn = ""
	require.NoError(t, w.syncer.SyncAll(context.Background()))
	assert.Len(t, w.emails(t), 4)
	assert.Equal(t, "100", w.historyID(t))
	assert.Equal(t, 1, f.fetched["m1"], "the page carried on, not started over")
	assert.Equal(t, 1, f.fetched["m2"])
}

func TestAHistoryPageLongerThanTheDeadlineStillGetsDone(t *testing.T) {
	f := newFakeGmail(t)
	w := newWorld(t)
	w.connect(t, "maja@example.com")
	tok := "token-maja@example.com"
	require.NoError(t, w.syncer.SyncAll(context.Background()))
	for _, id := range []string{"n1", "n2", "n3", "n4"} {
		f.mail(tok, id, "lina.dahl@granit.example", "maja@example.com", "New "+id, nil, true)
	}
	defer gmail.SetSyncTimeout(300 * time.Millisecond)()
	f.stallOn = "/messages/n3"
	release := f.stall()

	require.NoError(t, w.syncer.SyncAll(context.Background()))
	assert.Equal(t, "102", w.historyID(t), "past n1 and n2")

	release()
	f.stallOn = ""
	require.NoError(t, w.syncer.SyncAll(context.Background()))
	assert.Len(t, w.emails(t), 4)
	assert.Equal(t, "104", w.historyID(t))
	assert.Equal(t, 1, f.fetched["n1"])
}

// Mail deleted, or arriving, while a backfill is cut off doesn't make the resumed backfill skip anything:
// it carries on after the last message it recorded, and new mail is history's to bring.
func TestAResumedBackfillMissesNothingWhenTheMailboxChanges(t *testing.T) {
	f := newFakeGmail(t)
	w := newWorld(t)
	w.connect(t, "maja@example.com")
	tok := "token-maja@example.com"
	for _, id := range []string{"m1", "m2", "m3", "m4"} {
		f.mail(tok, id, "lina.dahl@granit.example", "maja@example.com", "Mail "+id, nil, false)
	}
	defer gmail.SetSyncTimeout(300 * time.Millisecond)()
	f.stallOn = "/messages/m3"
	release := f.stall()
	require.NoError(t, w.syncer.SyncAll(context.Background()))
	require.Len(t, w.emails(t), 2)

	release()
	f.stallOn = ""
	f.remove(tok, "m1")
	f.arrive(tok, "m0", "lina.dahl@granit.example", "maja@example.com", "Mail m0")
	require.NoError(t, w.syncer.SyncAll(context.Background()))
	require.NoError(t, w.syncer.SyncAll(context.Background()))

	titles := []string{}
	for _, e := range w.emails(t) {
		titles = append(titles, e.Title)
	}
	assert.ElementsMatch(t, []string{"Mail m0", "Mail m1", "Mail m2", "Mail m3", "Mail m4"}, titles)
	assert.Equal(t, 1, f.fetched["m2"], "carried on after m2, without reading it again")
}

package handlers

import (
	"bytes"
	"context"
	"net/http"
	"net/http/httptest"
	"os"
	"strconv"
	"testing"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/kthaisociety/business-crm/internal/database/databasetest"
	"github.com/kthaisociety/business-crm/internal/deals"
	"github.com/kthaisociety/business-crm/internal/fathom"
)

// Fathom posts finished meetings (docs/adr/0006). The payload names Lina Dahl at granit.example and
// Maja Holm from the team.

var fathomKey = []byte("a made-up webhook key")

func newFathomApp(t *testing.T) (*gin.Engine, *deals.Store) {
	gin.SetMode(gin.TestMode)
	store := deals.NewStore(databasetest.Open(t))
	return NewRouter(Deps{
		PingDB:    func(context.Context) error { return nil },
		Frontend:  builtUI,
		DevUser:   devUser,
		Deals:     store,
		FathomKey: fathomKey,
	}), store
}

func postMeeting(r http.Handler, body []byte, key []byte) *httptest.ResponseRecorder {
	stamp := strconv.FormatInt(time.Now().Unix(), 10)
	req := httptest.NewRequest(http.MethodPost, "/api/integrations/fathom", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("webhook-id", "msg_test")
	req.Header.Set("webhook-timestamp", stamp)
	req.Header.Set("webhook-signature", "v1,"+fathom.Sign(key, "msg_test", stamp, body))
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)
	return w
}

func meetingPayload(t *testing.T) []byte {
	t.Helper()
	body, err := os.ReadFile("../fathom/testdata/meeting.json")
	require.NoError(t, err)
	return body
}

// lead adds a deal at the company, with Lina as its contact when the company is Granit.
func lead(t *testing.T, store *deals.Store, company, semester string) deals.View {
	t.Helper()
	in := deals.NewLead{Company: company, DealType: "Event", Semester: semester}
	if company == "Granit Consulting" {
		in.Contact = &deals.ContactInput{Name: "Lina Dahl", Email: "lina.dahl@granit.example"}
	}
	v, err := store.Create(context.Background(), in, *devUser)
	require.NoError(t, err)
	return v
}

func setStatus(t *testing.T, store *deals.Store, id, status string) {
	t.Helper()
	_, err := store.Update(context.Background(), id, map[string]any{"status": status})
	require.NoError(t, err)
}

func meetingsOn(t *testing.T, store *deals.Store, id string) []deals.ActivityView {
	t.Helper()
	v, err := store.Get(context.Background(), id)
	require.NoError(t, err)
	var meetings []deals.ActivityView
	for _, a := range v.Activities {
		if a.Type == "meeting" {
			meetings = append(meetings, a)
		}
	}
	return meetings
}

func TestAFathomMeetingLandsOnTheContactsOpenDeals(t *testing.T) {
	r, store := newFathomApp(t)
	first := lead(t, store, "Granit Consulting", "HT 2026")
	second := lead(t, store, "Granit Consulting", "VT 2027")
	done := lead(t, store, "Granit Consulting", "HT 2025")
	setStatus(t, store, done.ID, "finished")
	other := lead(t, store, "Polar Systems", "HT 2026")

	w := postMeeting(r, meetingPayload(t), fathomKey)

	require.Equal(t, http.StatusNoContent, w.Code, w.Body.String())
	for _, id := range []string{first.ID, second.ID} {
		meetings := meetingsOn(t, store, id)
		require.Len(t, meetings, 1)
		m := meetings[0]
		assert.Equal(t, "fathom", m.Source)
		assert.Equal(t, "Lunch lecture planning", m.Title)
		assert.Equal(t, "2026-10-14T14:01:12Z", m.Timestamp)
		assert.Contains(t, m.Body, "We agreed on a lunch lecture on 4 Dec")
		assert.Equal(t, []string{"Send the invoice with 30-day terms"}, m.BulletPoints)
		assert.Equal(t, "https://fathom.video/share/abc123", m.URL)
	}
	assert.Empty(t, meetingsOn(t, store, done.ID), "a finished deal doesn't get it while others are open")
	assert.Empty(t, meetingsOn(t, store, other.ID), "nor does another company's")

	// Fathom resends a webhook it thinks failed: still one meeting.
	require.Equal(t, http.StatusNoContent, postMeeting(r, meetingPayload(t), fathomKey).Code)
	assert.Len(t, meetingsOn(t, store, first.ID), 1)
}

func TestWithNoOpenDealTheMeetingLandsOnTheLatest(t *testing.T) {
	r, store := newFathomApp(t)
	older := lead(t, store, "Granit Consulting", "HT 2025")
	newer := lead(t, store, "Granit Consulting", "VT 2026")
	setStatus(t, store, older.ID, "finished")
	setStatus(t, store, newer.ID, "canceled")

	require.Equal(t, http.StatusNoContent, postMeeting(r, meetingPayload(t), fathomKey).Code)

	assert.Len(t, meetingsOn(t, store, newer.ID), 1)
	assert.Empty(t, meetingsOn(t, store, older.ID))
}

func TestAMeetingWithNoContactIsDropped(t *testing.T) {
	r, store := newFathomApp(t)
	other := lead(t, store, "Polar Systems", "HT 2026")

	require.Equal(t, http.StatusNoContent, postMeeting(r, meetingPayload(t), fathomKey).Code)

	assert.Empty(t, meetingsOn(t, store, other.ID))
}

func TestOnlyFathomCanPostMeetings(t *testing.T) {
	r, store := newFathomApp(t)
	open := lead(t, store, "Granit Consulting", "HT 2026")

	w := postMeeting(r, meetingPayload(t), []byte("someone else's key"))

	assert.Equal(t, http.StatusUnauthorized, w.Code)
	assert.Empty(t, meetingsOn(t, store, open.ID))
	assert.Equal(t, http.StatusBadRequest, postMeeting(r, []byte(`{"title":"no id"}`), fathomKey).Code)
}

func TestWithoutASecretTheWebhookIsOff(t *testing.T) {
	r := newTestRouter(builtUI)
	assert.Equal(t, http.StatusNotFound, postMeeting(r, meetingPayload(t), fathomKey).Code)
}


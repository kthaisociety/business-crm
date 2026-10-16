package fathom

import (
	"encoding/base64"
	"net/http"
	"os"
	"strconv"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

var (
	secret = "whsec_" + base64.StdEncoding.EncodeToString([]byte("a made-up test key"))
	now    = time.Date(2026, 10, 14, 15, 2, 0, 0, time.UTC)
)

func signed(t *testing.T, body []byte, at time.Time) http.Header {
	t.Helper()
	key, err := Key(secret)
	require.NoError(t, err)
	stamp := strconv.FormatInt(at.Unix(), 10)
	h := http.Header{}
	h.Set("webhook-id", "msg_1")
	h.Set("webhook-timestamp", stamp)
	h.Set("webhook-signature", "v1,"+Sign(key, "msg_1", stamp, body))
	return h
}

func TestKeyNeedsFathomsFormat(t *testing.T) {
	_, err := Key(secret)
	assert.NoError(t, err)
	for _, bad := range []string{"", "whsec_", "whsec_!!!", base64.StdEncoding.EncodeToString([]byte("no prefix"))} {
		_, err := Key(bad)
		assert.Error(t, err, bad)
	}
}

// The Standard Webhooks spec's example, the scheme Fathom signs with (https://www.standardwebhooks.com,
// https://developers.fathom.ai/webhooks).
func TestVerifyMatchesFathomsAlgorithm(t *testing.T) {
	key, err := Key("whsec_MfKQ9r8GKYqrTwjUPD8ILPZIo2LaLaSw")
	require.NoError(t, err)
	body := []byte(`{"test": 2432232314}`)
	h := http.Header{}
	h.Set("webhook-id", "msg_p5jXN8AQM9LWM0D4loKWxJek")
	h.Set("webhook-timestamp", "1614265330")
	h.Set("webhook-signature", "v1,g0hM9SsE+OTPJTGt/tmIKtSyZlE3uFJELVlNIOLJ1OE=")

	assert.NoError(t, Verify(key, h, body, time.Unix(1614265330, 0)))
}

func TestVerifyRefusesAnythingElse(t *testing.T) {
	key, _ := Key(secret)
	body := []byte(`{"recording_id": 1}`)

	assert.NoError(t, Verify(key, signed(t, body, now), body, now))

	tampered := []byte(`{"recording_id": 2}`)
	assert.ErrorIs(t, Verify(key, signed(t, body, now), tampered, now), ErrSignature)

	other, _ := Key("whsec_" + base64.StdEncoding.EncodeToString([]byte("someone else's key")))
	assert.ErrorIs(t, Verify(other, signed(t, body, now), body, now), ErrSignature)

	assert.ErrorIs(t, Verify(key, signed(t, body, now.Add(-6*time.Minute)), body, now), ErrSignature, "too old: a replay")
	assert.ErrorIs(t, Verify(key, signed(t, body, now.Add(6*time.Minute)), body, now), ErrSignature)

	h := signed(t, body, now)
	h.Del("webhook-id")
	assert.ErrorIs(t, Verify(key, h, body, now), ErrSignature)

	h = signed(t, body, now)
	h.Set("webhook-signature", "v1,bm90IGl0 "+h.Get("webhook-signature"))
	assert.NoError(t, Verify(key, h, body, now), "any one of several signatures may match")
}

func TestParseReadsWhatTheCRMKeeps(t *testing.T) {
	body, err := os.ReadFile("testdata/meeting.json")
	require.NoError(t, err)

	m, err := Parse(body)

	require.NoError(t, err)
	assert.Equal(t, "123456789", m.RecordingID)
	assert.Equal(t, "Lunch lecture planning", m.Title)
	assert.Equal(t, "https://fathom.video/share/abc123", m.URL)
	assert.Equal(t, time.Date(2026, 10, 14, 14, 1, 12, 0, time.UTC), m.StartedAt)
	assert.Equal(t, []string{"maja.holm@example.com", "lina.dahl@granit.example"}, m.InviteeEmails)
	assert.Equal(t, "maja.holm@example.com", m.RecordedBy)
	assert.Equal(t, "Summary\nWe agreed on a lunch lecture on 4 Dec, see the brief.\n\nNext steps\n- Send the invoice", m.Summary)
	assert.Equal(t, []string{"Send the invoice with 30-day terms"}, m.ActionItems)
}

// Fathom's real summaries link most lines to their moment in the recording, with bold inside the link,
// and escape some characters.
func TestPlainTextReadsFathomsLinkedLines(t *testing.T) {
	markdown := "## Key Takeaways\n\n  - [**Instant review:** Recordings are ready in \\~5 seconds.](https://fathom.video/share/x?timestamp=337.0)\n  - [Plain line](https://fathom.video/share/x)"

	assert.Equal(t, "Key Takeaways\n\n- Instant review: Recordings are ready in ~5 seconds.\n- Plain line", plainText(markdown))
}

func TestParseFallsBackToTheCalendarTitle(t *testing.T) {
	m, err := Parse([]byte(`{"recording_id": 7, "title": " ", "meeting_title": "Intro call"}`))
	require.NoError(t, err)
	assert.Equal(t, "Intro call", m.Title)

	_, err = Parse([]byte(`{"title": "no id"}`))
	assert.Error(t, err)
	_, err = Parse([]byte(`not json`))
	assert.Error(t, err)
}

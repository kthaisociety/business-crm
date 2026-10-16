// Package fathom reads Fathom's "new meeting content ready" webhook (docs/adr/0006): it checks the
// signature and turns the payload into a meeting the deals store can match to contacts. Fathom signs
// webhooks the Standard Webhooks way: https://developers.fathom.ai/webhooks.
package fathom

import (
	"crypto/hmac"
	"crypto/sha256"
	"crypto/subtle"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"regexp"
	"strconv"
	"strings"
	"time"
)

// Tolerance is how far a webhook's timestamp may be from now, so an old, captured call can't be replayed.
const Tolerance = 5 * time.Minute

// ErrSignature is a webhook that isn't from Fathom, or is too old: the API answers 401.
var ErrSignature = errors.New("the webhook signature doesn't match")

// Key reads a webhook secret as Fathom shows it, `whsec_` and base64.
func Key(secret string) ([]byte, error) {
	key, err := base64.StdEncoding.DecodeString(strings.TrimPrefix(secret, "whsec_"))
	if err != nil || len(key) == 0 || !strings.HasPrefix(secret, "whsec_") {
		return nil, errors.New("a Fathom webhook secret looks like whsec_ followed by base64")
	}
	return key, nil
}

// Verify checks that body came from Fathom: one of the webhook-signature header's signatures must be
// HMAC-SHA256 of "id.timestamp.body" with key, and the timestamp within Tolerance of now.
func Verify(key []byte, header http.Header, body []byte, now time.Time) error {
	id, stamp, signatures := header.Get("webhook-id"), header.Get("webhook-timestamp"), header.Get("webhook-signature")
	if id == "" || stamp == "" || signatures == "" {
		return fmt.Errorf("%w: missing webhook headers", ErrSignature)
	}
	seconds, err := strconv.ParseInt(stamp, 10, 64)
	if err != nil {
		return fmt.Errorf("%w: bad timestamp", ErrSignature)
	}
	if at := time.Unix(seconds, 0); at.Before(now.Add(-Tolerance)) || at.After(now.Add(Tolerance)) {
		return fmt.Errorf("%w: timestamp out of tolerance", ErrSignature)
	}
	want := []byte(Sign(key, id, stamp, body))
	for _, s := range strings.Fields(signatures) {
		_, sig, ok := strings.Cut(s, ",")
		if ok && subtle.ConstantTimeCompare([]byte(sig), want) == 1 {
			return nil
		}
	}
	return ErrSignature
}

// Sign is Fathom's signature of a webhook, without the version prefix.
func Sign(key []byte, id, timestamp string, body []byte) string {
	mac := hmac.New(sha256.New, key)
	mac.Write([]byte(id + "." + timestamp + "."))
	mac.Write(body)
	return base64.StdEncoding.EncodeToString(mac.Sum(nil))
}

// Meeting is what the CRM keeps of a Fathom meeting.
type Meeting struct {
	RecordingID   string
	Title         string
	URL           string // the recording's share link
	StartedAt     time.Time
	InviteeEmails []string
	RecordedBy    string // email
	Summary       string // plain text
	ActionItems   []string
}

// payload is the part of Fathom's Meeting schema the CRM reads.
type payload struct {
	Title              string    `json:"title"`
	MeetingTitle       *string   `json:"meeting_title"`
	RecordingID        int64     `json:"recording_id"`
	ShareURL           string    `json:"share_url"`
	RecordingStartTime time.Time `json:"recording_start_time"`
	CalendarInvitees   []struct {
		Email string `json:"email"`
	} `json:"calendar_invitees"`
	RecordedBy struct {
		Email string `json:"email"`
	} `json:"recorded_by"`
	DefaultSummary *struct {
		MarkdownFormatted string `json:"markdown_formatted"`
	} `json:"default_summary"`
	ActionItems []struct {
		Description string `json:"description"`
	} `json:"action_items"`
}

// Parse reads a webhook's body.
func Parse(body []byte) (Meeting, error) {
	var p payload
	if err := json.Unmarshal(body, &p); err != nil {
		return Meeting{}, fmt.Errorf("not a Fathom meeting: %w", err)
	}
	if p.RecordingID == 0 {
		return Meeting{}, errors.New("not a Fathom meeting: no recording_id")
	}
	m := Meeting{
		RecordingID: strconv.FormatInt(p.RecordingID, 10),
		Title:       strings.TrimSpace(p.Title),
		URL:         p.ShareURL,
		StartedAt:   p.RecordingStartTime,
		RecordedBy:  strings.ToLower(strings.TrimSpace(p.RecordedBy.Email)),
	}
	if m.Title == "" && p.MeetingTitle != nil {
		m.Title = strings.TrimSpace(*p.MeetingTitle)
	}
	if m.Title == "" {
		m.Title = "Meeting"
	}
	for _, i := range p.CalendarInvitees {
		if email := strings.ToLower(strings.TrimSpace(i.Email)); email != "" {
			m.InviteeEmails = append(m.InviteeEmails, email)
		}
	}
	if p.DefaultSummary != nil {
		m.Summary = plainText(p.DefaultSummary.MarkdownFormatted)
	}
	for _, a := range p.ActionItems {
		if d := strings.TrimSpace(a.Description); d != "" {
			m.ActionItems = append(m.ActionItems, d)
		}
	}
	return m, nil
}

var (
	markdownLink = regexp.MustCompile(`\[([^\]]*)\]\([^)]*\)`)
	blankLines   = regexp.MustCompile(`\n{3,}`)
)

// plainText turns Fathom's markdown summary into the timeline's plain text: no headings' #, no bold **,
// links as their text. Bullets and line breaks stay.
func plainText(markdown string) string {
	lines := strings.Split(strings.ReplaceAll(markdown, "\r\n", "\n"), "\n")
	for i, line := range lines {
		line = strings.TrimLeft(line, "#")
		line = strings.ReplaceAll(line, "**", "")
		lines[i] = strings.TrimSpace(markdownLink.ReplaceAllString(line, "$1"))
	}
	return strings.TrimSpace(blankLines.ReplaceAllString(strings.Join(lines, "\n"), "\n\n"))
}

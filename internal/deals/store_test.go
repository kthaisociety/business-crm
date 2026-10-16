package deals_test

import (
	"context"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/kthaisociety/lucid/internal/auth"
	"github.com/kthaisociety/lucid/internal/database/databasetest"
	"github.com/kthaisociety/lucid/internal/deals"
)

var alva = auth.User{Subject: "dev", Email: "alva@example.com", Name: "Alva Lindqvist"}

func newStore(t *testing.T) *deals.Store {
	return deals.NewStore(databasetest.Open(t))
}

func addLead(t *testing.T, s *deals.Store, company string) deals.View {
	t.Helper()
	owner, err := s.SignIn(context.Background(), alva)
	require.NoError(t, err)
	d, err := s.Create(context.Background(), deals.NewLead{
		OwnerID: &owner.ID, Company: company, DealType: "Event", Semester: "HT 2026", NextStep: "Send intro email",
		Contact: &deals.ContactInput{Name: "Kim Ek", Email: "kim." + strings.ToLower(strings.Fields(company)[0]) + "@example.com", Role: "Head of Talent"},
	}, alva)
	require.NoError(t, err)
	return d
}

func TestCreateAddsALeadOwnedByTheMemberWhoAddedIt(t *testing.T) {
	s := newStore(t)

	d := addLead(t, s, "Nordljus AB")

	assert.NotEmpty(t, d.ID)
	assert.Equal(t, "Nordljus AB", d.Company)
	assert.Equal(t, "lead", d.Status)
	assert.Equal(t, "TBA", d.EventType)
	assert.Equal(t, "Unclear", d.Audience)
	require.NotNil(t, d.Owner)
	assert.Equal(t, "alva@example.com", d.Owner.Email)
	require.NotNil(t, d.Contact)
	assert.Equal(t, "Kim Ek", d.Contact.Name)
	assert.Equal(t, "kim.nordljus@example.com", d.Contact.Email)
	assert.Equal(t, "Head of Talent", d.Contact.Role)
	assert.Equal(t, d.CompanyID, d.Contact.CompanyID)
	require.Len(t, d.Activities, 1)
	assert.Equal(t, "Added as a lead", d.Activities[0].Title)
	assert.Empty(t, d.Support)
}

func TestTheSameUserIsOneMember(t *testing.T) {
	s := newStore(t)

	first := addLead(t, s, "Nordljus AB")
	second := addLead(t, s, "Vinga Voice")

	assert.Equal(t, first.Owner.ID, second.Owner.ID)
}

func TestListReturnsEveryDealOldestFirst(t *testing.T) {
	s := newStore(t)
	addLead(t, s, "Nordljus AB")
	addLead(t, s, "Vinga Voice")

	list, err := s.List(context.Background())

	require.NoError(t, err)
	require.Len(t, list, 2)
	assert.Equal(t, "Nordljus AB", list[0].Company)
	assert.Equal(t, "Vinga Voice", list[1].Company)
}

func TestUpdateSetsAndClearsFields(t *testing.T) {
	s := newStore(t)
	d := addLead(t, s, "Nordljus AB")
	ctx := context.Background()

	cols, err := deals.ParsePatch([]byte(`{"status":"signed","nextStepDue":"2026-11-01","feeSEK":40000,"invoiceStatus":"not_sent"}`))
	require.NoError(t, err)
	updated, err := s.Update(ctx, d.ID, cols)
	require.NoError(t, err)
	assert.Equal(t, "signed", updated.Status)
	assert.Equal(t, deals.Date("2026-11-01"), *updated.NextStepDue)
	assert.Equal(t, 40000, *updated.FeeSEK)
	assert.Equal(t, "not_sent", *updated.InvoiceStatus)

	cols, err = deals.ParsePatch([]byte(`{"nextStepDue":null,"feeSEK":0,"invoiceStatus":null}`))
	require.NoError(t, err)
	updated, err = s.Update(ctx, d.ID, cols)
	require.NoError(t, err)
	assert.Nil(t, updated.NextStepDue)
	assert.Equal(t, 0, *updated.FeeSEK)
	assert.Nil(t, updated.InvoiceStatus)

	got, err := s.Get(ctx, d.ID)
	require.NoError(t, err)
	assert.Equal(t, updated, got)
}

func TestAddNotePutsTheNoteOnTheTimeline(t *testing.T) {
	s := newStore(t)
	d := addLead(t, s, "Nordljus AB")
	ctx := context.Background()

	note, err := s.AddNote(ctx, d.ID, "Called Kim, demo on Friday", alva)
	require.NoError(t, err)
	assert.Equal(t, "Quick note", note.Title)
	assert.Equal(t, "manual", note.Source)

	got, err := s.Get(ctx, d.ID)
	require.NoError(t, err)
	require.Len(t, got.Activities, 2)
	assert.Equal(t, note, got.Activities[1])
}

func TestUnknownDealsAreNotFound(t *testing.T) {
	s := newStore(t)
	ctx := context.Background()
	cols := map[string]any{"status": "signed"}

	for _, id := range []string{"00000000-0000-0000-0000-000000000000", "not-a-uuid"} {
		_, err := s.Get(ctx, id)
		assert.ErrorIs(t, err, deals.ErrNotFound)
		_, err = s.Update(ctx, id, cols)
		assert.ErrorIs(t, err, deals.ErrNotFound)
		_, err = s.AddNote(ctx, id, "hello", alva)
		assert.ErrorIs(t, err, deals.ErrNotFound)
	}
}

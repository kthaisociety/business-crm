package deals_test

import (
	"context"
	"slices"
	"sync"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/kthaisociety/business-crm/internal/auth"
	"github.com/kthaisociety/business-crm/internal/database/databasetest"
	"github.com/kthaisociety/business-crm/internal/deals"
)

var oskar = auth.User{Subject: "oskar", Email: "oskar@example.com", Name: "Oskar Berg"}

func TestSigningInJoinsTheTeamAsAMember(t *testing.T) {
	s := newStore(t)
	ctx := context.Background()

	me, err := s.SignIn(ctx, alva)
	require.NoError(t, err)
	assert.Equal(t, "member", me.Role)

	again, err := s.SignIn(ctx, alva)
	require.NoError(t, err)
	assert.Equal(t, me.ID, again.ID)
	var forbidden *deals.ForbiddenError
	assert.ErrorAs(t, s.RequireAdmin(ctx, alva), &forbidden)
}

func TestAdminEmailsBecomeAdminsOnSignIn(t *testing.T) {
	s := deals.NewStore(databasetest.Open(t), "ALVA@example.com")
	ctx := context.Background()

	me, err := s.SignIn(ctx, alva)
	require.NoError(t, err)
	assert.Equal(t, "admin", me.Role)
	assert.NoError(t, s.RequireAdmin(ctx, alva))

	other, err := s.SignIn(ctx, oskar)
	require.NoError(t, err)
	assert.Equal(t, "member", other.Role)
}

func TestAdminsAddAndChangeTeamMembers(t *testing.T) {
	s := deals.NewStore(databasetest.Open(t), alva.Email)
	ctx := context.Background()
	_, err := s.SignIn(ctx, alva)
	require.NoError(t, err)

	maja, err := s.AddMember(ctx, deals.MemberInput{Email: "maja@example.com", Name: "Maja Holm"})
	require.NoError(t, err)
	assert.Equal(t, "member", maja.Role)
	_, err = s.AddMember(ctx, deals.MemberInput{Email: "MAJA@example.com", Name: "Maja again"})
	conflictError(t, err)
	_, err = s.AddMember(ctx, deals.MemberInput{Email: "noah@example.com", Name: "Noah", Role: "owner"})
	validationError(t, err)

	cols, err := deals.ParseMemberPatch([]byte(`{"role":"board"}`))
	require.NoError(t, err)
	board, err := s.UpdateMember(ctx, maja.ID, cols)
	require.NoError(t, err)
	assert.Equal(t, "board", board.Role)

	team, err := s.ListMembers(ctx)
	require.NoError(t, err)
	assert.Len(t, team, 2)
}

func TestTheTeamKeepsAnAdmin(t *testing.T) {
	s := deals.NewStore(databasetest.Open(t), alva.Email)
	ctx := context.Background()
	me, err := s.SignIn(ctx, alva)
	require.NoError(t, err)

	_, err = s.UpdateMember(ctx, me.ID, map[string]any{"role": "member"})
	conflictError(t, err)
	conflictError(t, s.RemoveMember(ctx, me.ID))

	second, err := s.AddMember(ctx, deals.MemberInput{Email: "maja@example.com", Name: "Maja Holm", Role: "admin"})
	require.NoError(t, err)
	demoted, err := s.UpdateMember(ctx, me.ID, map[string]any{"role": "member"})
	require.NoError(t, err)
	assert.Equal(t, "member", demoted.Role)
	conflictError(t, s.RemoveMember(ctx, second.ID))
}

func TestAMemberWithDealsStaysUntilTheyAreReassigned(t *testing.T) {
	s := newStore(t)
	ctx := context.Background()
	d := addLead(t, s, "Nordljus AB")
	_, err := s.AddNote(ctx, d.ID, "Called them", alva)
	require.NoError(t, err)

	conflictError(t, s.RemoveMember(ctx, d.Owner.ID))

	cols, err := deals.ParsePatch([]byte(`{"ownerId":null}`))
	require.NoError(t, err)
	_, err = s.Update(ctx, d.ID, cols)
	require.NoError(t, err)
	require.NoError(t, s.RemoveMember(ctx, d.Owner.ID))

	got, err := s.Get(ctx, d.ID)
	require.NoError(t, err)
	assert.Len(t, got.Activities, 2, "their notes stay")
	assert.ErrorIs(t, s.RemoveMember(ctx, d.Owner.ID), deals.ErrMemberNotFound)
}

func TestLeadsCanStartUnassignedAndBeClaimedOrAssigned(t *testing.T) {
	s := newStore(t)
	ctx := context.Background()

	lead, err := s.Create(ctx, deals.NewLead{Company: "Vinga Voice", DealType: "Job Post", Semester: "HT 2026"}, alva)
	require.NoError(t, err)
	assert.Nil(t, lead.Owner)

	me, err := s.SignIn(ctx, oskar)
	require.NoError(t, err)
	cols, err := deals.ParsePatch([]byte(`{"ownerId":"` + me.ID + `"}`))
	require.NoError(t, err)
	claimed, err := s.Update(ctx, lead.ID, cols)
	require.NoError(t, err)
	assert.Equal(t, "Oskar Berg", claimed.Owner.Name)

	cols, err = deals.ParsePatch([]byte(`{"ownerId":"00000000-0000-0000-0000-000000000001"}`))
	require.NoError(t, err)
	_, err = s.Update(ctx, lead.ID, cols)
	validationError(t, err)

	unknown := "00000000-0000-0000-0000-000000000001"
	_, err = s.Create(ctx, deals.NewLead{OwnerID: &unknown, Company: "Vinga Voice", DealType: "Event", Semester: "HT 2026"}, alva)
	validationError(t, err)
}

// Demoting or removing different admins at once, the team still keeps one.
func TestConcurrentChangesKeepAnAdmin(t *testing.T) {
	for _, removeSecond := range []bool{false, true} {
		s := deals.NewStore(databasetest.Open(t))
		ctx := context.Background()
		var ids []string
		for _, email := range []string{"a@example.com", "b@example.com"} {
			m, err := s.AddMember(ctx, deals.MemberInput{Email: email, Name: email, Role: "admin"})
			require.NoError(t, err)
			ids = append(ids, m.ID)
		}

		var wg sync.WaitGroup
		results := make([]error, 2)
		wg.Add(2)
		go func() {
			defer wg.Done()
			_, results[0] = s.UpdateMember(ctx, ids[0], map[string]any{"role": "member"})
		}()
		go func() {
			defer wg.Done()
			if removeSecond {
				results[1] = s.RemoveMember(ctx, ids[1])
			} else {
				_, results[1] = s.UpdateMember(ctx, ids[1], map[string]any{"role": "board"})
			}
		}()
		wg.Wait()

		failed := 0
		for _, err := range results {
			if err != nil {
				conflictError(t, err)
				failed++
			}
		}
		assert.Equal(t, 1, failed, "exactly one change is refused")
		team, err := s.ListMembers(ctx)
		require.NoError(t, err)
		admins := 0
		for _, m := range team {
			if m.Role == "admin" {
				admins++
			}
		}
		assert.Equal(t, 1, admins)
	}
}

// A lead's transaction locks its author's row on sign-in, then its owner's key when it inserts the deal. A
// team change in between, which locks the admins, must not deadlock with it.
func TestTeamChangesDoNotDeadlockWithNewLeads(t *testing.T) {
	db := databasetest.Open(t)
	s := deals.NewStore(db)
	ctx := context.Background()
	var ids []string
	for _, email := range []string{"a@example.com", "b@example.com"} {
		m, err := s.AddMember(ctx, deals.MemberInput{Email: email, Name: email, Role: "admin"})
		require.NoError(t, err)
		ids = append(ids, m.ID)
	}
	slices.Sort(ids)
	owner, author := ids[0], ids[1]
	other, err := s.AddMember(ctx, deals.MemberInput{Email: "c@example.com", Name: "C", Role: "member"})
	require.NoError(t, err)

	// What Create does: the author's upsert, then the deal's foreign-key check on its owner.
	lead := db.Begin()
	defer lead.Rollback()
	require.NoError(t, lead.Exec("UPDATE members SET name = name WHERE id = ?", author).Error)

	changed := make(chan error, 1)
	go func() {
		_, err := s.UpdateMember(ctx, other.ID, map[string]any{"role": "board"})
		changed <- err
	}()
	require.Eventually(t, func() bool {
		var waiting int64
		db.Raw("SELECT count(*) FROM pg_stat_activity WHERE wait_event_type = 'Lock' AND datname = current_database()").Scan(&waiting)
		return waiting > 0
	}, 5*time.Second, 10*time.Millisecond, "the team change waits for the author's row")

	require.NoError(t, lead.Exec("SELECT 1 FROM members WHERE id = ? FOR KEY SHARE", owner).Error)
	require.NoError(t, lead.Commit().Error)
	assert.NoError(t, <-changed)
}

// With real sign-in (docs/adr/0004), someone removed from the team while signing in, or while a request
// of theirs is under way, stays removed: nothing but an admin adds them back.
func TestARemovedMemberStaysRemoved(t *testing.T) {
	db := databasetest.Open(t)
	s := deals.NewStore(db).OnlyTeamMembers()
	ctx := context.Background()
	maja := auth.User{Subject: "maja", Email: "maja@example.com", Name: "Maja Holm"}
	m, err := s.AddMember(ctx, deals.MemberInput{Email: maja.Email, Name: maja.Name})
	require.NoError(t, err)

	// An admin's removal is under way when Maja's sign-in checks the team.
	removal := db.Begin()
	defer removal.Rollback()
	require.NoError(t, removal.Exec("DELETE FROM members WHERE id = ?", m.ID).Error)
	admitted := make(chan error, 1)
	go func() {
		_, err := s.Admit(ctx, maja)
		admitted <- err
	}()
	require.Eventually(t, func() bool {
		var waiting int64
		db.Raw("SELECT count(*) FROM pg_stat_activity WHERE wait_event_type = 'Lock' AND datname = current_database()").Scan(&waiting)
		return waiting > 0
	}, 5*time.Second, 10*time.Millisecond, "the sign-in waits for the removal")
	require.NoError(t, removal.Commit().Error)

	assert.ErrorIs(t, <-admitted, deals.ErrNotOnTeam)
	_, err = s.SignIn(ctx, maja)
	assert.ErrorIs(t, err, deals.ErrNotOnTeam, "a request already under way doesn't add her back either")
	team, err := s.ListMembers(ctx)
	require.NoError(t, err)
	assert.Empty(t, team)
}

func TestOnlyTeamMembersStillLetsAdminEmailsIn(t *testing.T) {
	s := deals.NewStore(databasetest.Open(t), "maja@example.com").OnlyTeamMembers()

	me, err := s.Admit(context.Background(), auth.User{Email: "Maja@example.com", Name: "Maja Holm"})

	require.NoError(t, err)
	assert.Equal(t, "admin", me.Role)
}

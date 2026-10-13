package deals_test

import (
	"context"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/kthaisociety/business-crm/internal/deals"
)

func conflictError(t *testing.T, err error) {
	t.Helper()
	var clash *deals.ConflictError
	assert.ErrorAs(t, err, &clash)
}

func validationError(t *testing.T, err error) {
	t.Helper()
	var invalid *deals.ValidationError
	assert.ErrorAs(t, err, &invalid)
}

func TestANewLeadReusesACompanyOfTheSameName(t *testing.T) {
	s := newStore(t)
	ctx := context.Background()
	first := addLead(t, s, "Nordljus AB")

	second, err := s.Create(ctx, deals.NewLead{Company: "nordljus ab", DealType: "Job Post", Semester: "VT 2027"}, alva)

	require.NoError(t, err)
	assert.Equal(t, first.CompanyID, second.CompanyID)
	assert.Equal(t, "Nordljus AB", second.Company)
	assert.Nil(t, second.Contact)
	companies, err := s.ListCompanies(ctx)
	require.NoError(t, err)
	assert.Len(t, companies, 1)
}

func TestANewLeadCanUseAnExistingContact(t *testing.T) {
	s := newStore(t)
	ctx := context.Background()
	first := addLead(t, s, "Nordljus AB")

	byID, err := s.Create(ctx, deals.NewLead{CompanyID: first.CompanyID, ContactID: first.Contact.ID, DealType: "Event", Semester: "VT 2027"}, alva)
	require.NoError(t, err)
	assert.Equal(t, first.Contact.ID, byID.Contact.ID)

	// The same email at the same company is the same person.
	byEmail, err := s.Create(ctx, deals.NewLead{CompanyID: first.CompanyID, DealType: "Event", Semester: "VT 2027",
		Contact: &deals.ContactInput{Name: "Kim", Email: "KIM.NORDLJUS@example.com"}}, alva)
	require.NoError(t, err)
	assert.Equal(t, first.Contact.ID, byEmail.Contact.ID)
}

func TestALeadsContactMustWorkAtItsCompany(t *testing.T) {
	s := newStore(t)
	ctx := context.Background()
	nordljus := addLead(t, s, "Nordljus AB")
	vinga := addLead(t, s, "Vinga Voice")

	_, err := s.Create(ctx, deals.NewLead{CompanyID: vinga.CompanyID, ContactID: nordljus.Contact.ID, DealType: "Event", Semester: "HT 2026"}, alva)
	validationError(t, err)

	_, err = s.Create(ctx, deals.NewLead{CompanyID: vinga.CompanyID, DealType: "Event", Semester: "HT 2026",
		Contact: &deals.ContactInput{Email: "kim.nordljus@example.com"}}, alva)
	conflictError(t, err)

	cols, err := deals.ParsePatch([]byte(`{"contactId":"` + nordljus.Contact.ID + `"}`))
	require.NoError(t, err)
	_, err = s.Update(ctx, vinga.ID, cols)
	validationError(t, err)
}

func TestChangingAndClearingADealsContact(t *testing.T) {
	s := newStore(t)
	ctx := context.Background()
	d := addLead(t, s, "Nordljus AB")
	other, err := s.CreateContact(ctx, deals.ContactInput{CompanyID: d.CompanyID, Name: "Sam Berg"})
	require.NoError(t, err)

	cols, _ := deals.ParsePatch([]byte(`{"contactId":"` + other.ID + `"}`))
	updated, err := s.Update(ctx, d.ID, cols)
	require.NoError(t, err)
	assert.Equal(t, "Sam Berg", updated.Contact.Name)

	cols, _ = deals.ParsePatch([]byte(`{"contactId":null}`))
	updated, err = s.Update(ctx, d.ID, cols)
	require.NoError(t, err)
	assert.Nil(t, updated.Contact)
}

func TestCompanyNamesAreUniqueIgnoringCase(t *testing.T) {
	s := newStore(t)
	ctx := context.Background()
	granit, err := s.CreateCompany(ctx, deals.CompanyInput{Name: "Granit Consulting", Website: "https://example.com"})
	require.NoError(t, err)
	_, err = s.CreateCompany(ctx, deals.CompanyInput{Name: "GRANIT consulting"})
	conflictError(t, err)

	vinga, err := s.CreateCompany(ctx, deals.CompanyInput{Name: "Vinga Voice"})
	require.NoError(t, err)
	_, err = s.UpdateCompany(ctx, vinga.ID, map[string]any{"name": "granit consulting"})
	conflictError(t, err)

	renamed, err := s.UpdateCompany(ctx, granit.ID, map[string]any{"name": "Granit AB", "notes": "Met at D-Dagen"})
	require.NoError(t, err)
	assert.Equal(t, deals.CompanyView{ID: granit.ID, Name: "Granit AB", Website: "https://example.com", Notes: "Met at D-Dagen"}, renamed)
}

func TestACompanyWithDealsCantBeDeleted(t *testing.T) {
	s := newStore(t)
	ctx := context.Background()
	d := addLead(t, s, "Nordljus AB")
	conflictError(t, s.DeleteCompany(ctx, d.CompanyID))

	empty, err := s.CreateCompany(ctx, deals.CompanyInput{Name: "Vinga Voice"})
	require.NoError(t, err)
	_, err = s.CreateContact(ctx, deals.ContactInput{CompanyID: empty.ID, Name: "Adam Roos"})
	require.NoError(t, err)
	require.NoError(t, s.DeleteCompany(ctx, empty.ID))

	contacts, err := s.ListContacts(ctx)
	require.NoError(t, err)
	assert.Len(t, contacts, 1, "the deleted company's contact goes with it")
	assert.ErrorIs(t, s.DeleteCompany(ctx, empty.ID), deals.ErrCompanyNotFound)
}

func TestContactEmailsAreUnique(t *testing.T) {
	s := newStore(t)
	ctx := context.Background()
	d := addLead(t, s, "Nordljus AB")

	_, err := s.CreateContact(ctx, deals.ContactInput{CompanyID: d.CompanyID, Name: "Kim again", Email: "Kim.Nordljus@example.com"})
	conflictError(t, err)

	sam, err := s.CreateContact(ctx, deals.ContactInput{CompanyID: d.CompanyID, Name: "Sam Berg", Email: "sam@example.com"})
	require.NoError(t, err)
	_, err = s.UpdateContact(ctx, sam.ID, map[string]any{"email": "kim.nordljus@example.com"})
	conflictError(t, err)
}

func TestEditingAContactShowsOnItsDeals(t *testing.T) {
	s := newStore(t)
	ctx := context.Background()
	d := addLead(t, s, "Nordljus AB")

	_, err := s.UpdateContact(ctx, d.Contact.ID, map[string]any{"role": "CTO"})
	require.NoError(t, err)

	got, err := s.Get(ctx, d.ID)
	require.NoError(t, err)
	assert.Equal(t, "CTO", got.Contact.Role)
}

func TestAContactOnDealsStaysWithTheirCompany(t *testing.T) {
	s := newStore(t)
	ctx := context.Background()
	d := addLead(t, s, "Nordljus AB")
	vinga, err := s.CreateCompany(ctx, deals.CompanyInput{Name: "Vinga Voice"})
	require.NoError(t, err)

	_, err = s.UpdateContact(ctx, d.Contact.ID, map[string]any{"company_id": vinga.ID})
	conflictError(t, err)
	conflictError(t, s.DeleteContact(ctx, d.Contact.ID))

	// Without deals, a contact can move and be deleted.
	sam, err := s.CreateContact(ctx, deals.ContactInput{CompanyID: d.CompanyID, Name: "Sam Berg"})
	require.NoError(t, err)
	moved, err := s.UpdateContact(ctx, sam.ID, map[string]any{"company_id": vinga.ID})
	require.NoError(t, err)
	assert.Equal(t, vinga.ID, moved.CompanyID)
	require.NoError(t, s.DeleteContact(ctx, sam.ID))
	assert.ErrorIs(t, s.DeleteContact(ctx, sam.ID), deals.ErrContactNotFound)

	_, err = s.UpdateContact(ctx, d.Contact.ID, map[string]any{"name": "", "email": ""})
	validationError(t, err)
}

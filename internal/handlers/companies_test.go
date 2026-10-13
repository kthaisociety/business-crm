package handlers

import (
	"net/http"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/kthaisociety/business-crm/internal/deals"
)

func TestCompaniesAndContactsLifecycle(t *testing.T) {
	r := newDealsRouter(t)

	w := send(r, http.MethodPost, "/api/companies", `{"name":"Granit Consulting","website":"https://example.com"}`)
	require.Equal(t, http.StatusCreated, w.Code, w.Body.String())
	granit := decode[deals.CompanyView](t, w)

	w = send(r, http.MethodPost, "/api/contacts", `{"companyId":"`+granit.ID+`","name":"Lina Dahl","email":"lina.dahl@example.com","role":"Recruiter"}`)
	require.Equal(t, http.StatusCreated, w.Code, w.Body.String())
	lina := decode[deals.ContactView](t, w)
	assert.Equal(t, granit.ID, lina.CompanyID)

	w = send(r, http.MethodPost, "/api/deals", `{"companyId":"`+granit.ID+`","contactId":"`+lina.ID+`","dealType":"Event","semester":"HT 2026"}`)
	require.Equal(t, http.StatusCreated, w.Code, w.Body.String())
	deal := decode[deals.View](t, w)
	assert.Equal(t, "Granit Consulting", deal.Company)
	assert.Equal(t, "Lina Dahl", deal.Contact.Name)

	w = send(r, http.MethodPatch, "/api/contacts/"+lina.ID, `{"role":"Recruitment Manager"}`)
	require.Equal(t, http.StatusOK, w.Code, w.Body.String())
	list := decode[[]deals.View](t, get(r, "/api/deals"))
	assert.Equal(t, "Recruitment Manager", list[0].Contact.Role)

	assert.Len(t, decode[[]deals.CompanyView](t, get(r, "/api/companies")), 1)
	assert.Len(t, decode[[]deals.ContactView](t, get(r, "/api/contacts")), 1)

	w = send(r, http.MethodPatch, "/api/companies/"+granit.ID, `{"name":"Granit AB"}`)
	require.Equal(t, http.StatusOK, w.Code, w.Body.String())
	assert.Equal(t, "Granit AB", decode[[]deals.View](t, get(r, "/api/deals"))[0].Company)
}

func TestCompanyAndContactErrors(t *testing.T) {
	r := newDealsRouter(t)
	deal := decode[deals.View](t, send(r, http.MethodPost, "/api/deals",
		`{"company":"Granit Consulting","contact":{"name":"Lina Dahl","email":"lina@example.com"},"dealType":"Event","semester":"HT 2026"}`))
	empty := decode[deals.CompanyView](t, send(r, http.MethodPost, "/api/companies", `{"name":"Vinga Voice"}`))
	unknown := "00000000-0000-0000-0000-000000000000"

	tests := []struct {
		name, method, path, body string
		status                   int
	}{
		{"company without a name", http.MethodPost, "/api/companies", `{"name":" "}`, http.StatusBadRequest},
		{"company name taken", http.MethodPost, "/api/companies", `{"name":"granit consulting"}`, http.StatusConflict},
		{"contact email taken", http.MethodPost, "/api/contacts", `{"companyId":"` + empty.ID + `","email":"LINA@example.com"}`, http.StatusConflict},
		{"contact at unknown company", http.MethodPost, "/api/contacts", `{"companyId":"` + unknown + `","name":"Kim"}`, http.StatusBadRequest},
		{"delete company with deals", http.MethodDelete, "/api/companies/" + deal.CompanyID, ``, http.StatusConflict},
		{"delete contact on a deal", http.MethodDelete, "/api/contacts/" + deal.Contact.ID, ``, http.StatusConflict},
		{"unknown company", http.MethodPatch, "/api/companies/" + unknown, `{"name":"X"}`, http.StatusNotFound},
		{"unknown contact", http.MethodDelete, "/api/contacts/" + unknown, ``, http.StatusNotFound},
		{"deal contact from another company", http.MethodPost, "/api/deals",
			`{"companyId":"` + empty.ID + `","contactId":"` + deal.Contact.ID + `","dealType":"Event","semester":"HT 2026"}`, http.StatusBadRequest},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			w := send(r, tt.method, tt.path, tt.body)
			assert.Equal(t, tt.status, w.Code, w.Body.String())
			assert.NotEmpty(t, decode[map[string]string](t, w)["error"])
		})
	}

	w := send(r, http.MethodDelete, "/api/companies/"+empty.ID, ``)
	assert.Equal(t, http.StatusNoContent, w.Code, w.Body.String())
}

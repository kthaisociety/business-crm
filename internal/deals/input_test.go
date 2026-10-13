package deals

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestParsePatchMapsFieldsToColumns(t *testing.T) {
	cols, err := ParsePatch([]byte(`{"status":"signed","nextStep":"Send invoice","nextStepDue":"2026-11-01","feeSEK":40000,"invoiceStatus":"not_sent"}`))

	require.NoError(t, err)
	assert.Equal(t, map[string]any{
		"status":         "signed",
		"next_step":      "Send invoice",
		"next_step_due":  Date("2026-11-01"),
		"fee_sek":        40000,
		"invoice_status": "not_sent",
	}, cols)
}

func TestParsePatchClearsNullableFields(t *testing.T) {
	cols, err := ParsePatch([]byte(`{"nextStepDue":null,"feeSEK":null,"invoiceStatus":null}`))

	require.NoError(t, err)
	assert.Equal(t, map[string]any{"next_step_due": nil, "fee_sek": nil, "invoice_status": nil}, cols)
}

func TestParsePatchRejectsBadInput(t *testing.T) {
	for _, body := range []string{
		`not json`,
		`{}`,
		`{"status":"won"}`,
		`{"status":null}`,
		`{"feeSEK":-1000}`,
		`{"feeSEK":2147483648}`,
		`{"nextStep":null}`,
		`{"feeSEK":"40000"}`,
		`{"nextStepDue":"13/10/2026"}`,
		`{"invoiceStatus":"overdue"}`,
		`{"company":"Renamed AB"}`,
	} {
		t.Run(body, func(t *testing.T) {
			_, err := ParsePatch([]byte(body))
			var invalid *ValidationError
			assert.ErrorAs(t, err, &invalid)
		})
	}
}

func TestNewLeadValidation(t *testing.T) {
	valid := NewLead{Company: "  Nordljus AB ", DealType: "Event", Semester: "HT 2026", ContactEmail: "kim@example.com"}
	require.NoError(t, valid.validate())
	assert.Equal(t, "Nordljus AB", valid.Company)

	for name, lead := range map[string]NewLead{
		"no company":    {DealType: "Event", Semester: "HT 2026"},
		"bad deal type": {Company: "X", DealType: "Sponsorship", Semester: "HT 2026"},
		"bad semester":  {Company: "X", DealType: "Event", Semester: "Autumn 2026"},
		"bad email":     {Company: "X", DealType: "Event", Semester: "HT 2026", ContactEmail: "kim"},
	} {
		t.Run(name, func(t *testing.T) {
			var invalid *ValidationError
			assert.ErrorAs(t, lead.validate(), &invalid)
		})
	}
}

func TestParseNote(t *testing.T) {
	text, err := ParseNote([]byte(`{"body":"  Called them  "}`))
	require.NoError(t, err)
	assert.Equal(t, "Called them", text)

	_, err = ParseNote([]byte(`{"body":"   "}`))
	var invalid *ValidationError
	assert.ErrorAs(t, err, &invalid)
}

func TestTheLargestFeeFitsTheColumn(t *testing.T) {
	cols, err := ParsePatch([]byte(`{"feeSEK":2147483647}`))
	require.NoError(t, err)
	assert.Equal(t, map[string]any{"fee_sek": 2147483647}, cols)
}

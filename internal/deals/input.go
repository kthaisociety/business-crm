package deals

import (
	"encoding/json"
	"fmt"
	"regexp"
	"slices"
	"strings"
)

// The values the database's CHECK constraints allow. Validating them here too turns a bad request into
// a 400 with a reason, instead of a constraint error.
var (
	dealTypes       = []string{"Event", "Partnership", "Job Post"}
	statuses        = []string{"lead", "discussing", "agreement", "signed", "finished", "canceled"}
	invoiceStatuses = []string{"not_sent", "waiting", "paid", "cancelled"}
	semesterPattern = regexp.MustCompile(`^(HT|VT) [0-9]{4}$`)
)

const maxText = 2000

// ValidationError is a request the API rejects with 400; its message says why.
type ValidationError struct{ msg string }

func (e *ValidationError) Error() string { return e.msg }

func invalid(format string, args ...any) error {
	return &ValidationError{msg: fmt.Sprintf(format, args...)}
}

// NewLead is what a member fills in to add a lead.
type NewLead struct {
	Company      string `json:"company"`
	DealType     string `json:"dealType"`
	Semester     string `json:"semester"`
	ContactName  string `json:"contactName"`
	ContactEmail string `json:"contactEmail"`
	NextStep     string `json:"nextStep"`
}

func (n *NewLead) validate() error {
	n.Company = strings.TrimSpace(n.Company)
	n.ContactName = strings.TrimSpace(n.ContactName)
	n.ContactEmail = strings.TrimSpace(n.ContactEmail)
	n.NextStep = strings.TrimSpace(n.NextStep)
	switch {
	case n.Company == "":
		return invalid("company is required")
	case !slices.Contains(dealTypes, n.DealType):
		return invalid("dealType must be one of %s", strings.Join(dealTypes, ", "))
	case !semesterPattern.MatchString(n.Semester):
		return invalid(`semester must look like "HT 2026" or "VT 2026"`)
	case n.ContactEmail != "" && !strings.Contains(n.ContactEmail, "@"):
		return invalid("contactEmail is not an email address")
	}
	for _, s := range []string{n.Company, n.ContactName, n.ContactEmail, n.NextStep} {
		if len(s) > maxText {
			return invalid("fields are limited to %d characters", maxText)
		}
	}
	return nil
}

// ParsePatch turns a PATCH body into the columns to update. Only the fields the deal panel edits are
// accepted; null clears a nullable one.
func ParsePatch(body []byte) (map[string]any, error) {
	var raw map[string]json.RawMessage
	if err := json.Unmarshal(body, &raw); err != nil {
		return nil, invalid("body must be a JSON object")
	}
	if len(raw) == 0 {
		return nil, invalid("nothing to update")
	}
	cols := map[string]any{}
	for field, value := range raw {
		isNull := string(value) == "null"
		switch field {
		case "status":
			var s string
			if json.Unmarshal(value, &s) != nil || !slices.Contains(statuses, s) {
				return nil, invalid("status must be one of %s", strings.Join(statuses, ", "))
			}
			cols["status"] = s
		case "nextStep":
			var s string
			if json.Unmarshal(value, &s) != nil || len(s) > maxText {
				return nil, invalid("nextStep must be text of at most %d characters", maxText)
			}
			cols["next_step"] = s
		case "nextStepDue":
			if isNull {
				cols["next_step_due"] = nil
				continue
			}
			var s string
			if json.Unmarshal(value, &s) != nil {
				return nil, invalid("nextStepDue must be a date (YYYY-MM-DD) or null")
			}
			d, err := ParseDate(s)
			if err != nil {
				return nil, invalid("nextStepDue: %v", err)
			}
			cols["next_step_due"] = d
		case "feeSEK":
			if isNull {
				cols["fee_sek"] = nil
				continue
			}
			var n int
			if json.Unmarshal(value, &n) != nil || n < 0 {
				return nil, invalid("feeSEK must be a whole number of 0 or more, or null")
			}
			cols["fee_sek"] = n
		case "invoiceStatus":
			if isNull {
				cols["invoice_status"] = nil
				continue
			}
			var s string
			if json.Unmarshal(value, &s) != nil || !slices.Contains(invoiceStatuses, s) {
				return nil, invalid("invoiceStatus must be one of %s, or null", strings.Join(invoiceStatuses, ", "))
			}
			cols["invoice_status"] = s
		default:
			return nil, invalid("%s can't be changed", field)
		}
	}
	return cols, nil
}

// ParseNote reads the body of a quick note.
func ParseNote(body []byte) (string, error) {
	var in struct {
		Body string `json:"body"`
	}
	if err := json.Unmarshal(body, &in); err != nil {
		return "", invalid("body must be a JSON object")
	}
	text := strings.TrimSpace(in.Body)
	if text == "" {
		return "", invalid("a note needs text")
	}
	if len(text) > maxText {
		return "", invalid("a note is limited to %d characters", maxText)
	}
	return text, nil
}

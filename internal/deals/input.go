package deals

import (
	"encoding/json"
	"fmt"
	"math"
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

// ConflictError is a request that clashes with what's stored (a name or email already taken, a delete
// that would orphan deals); the API answers 409 with its message.
type ConflictError struct{ msg string }

func (e *ConflictError) Error() string { return e.msg }

func conflict(format string, args ...any) error {
	return &ConflictError{msg: fmt.Sprintf(format, args...)}
}

func tooLong(fields ...string) error {
	for _, s := range fields {
		if len(s) > maxText {
			return invalid("fields are limited to %d characters", maxText)
		}
	}
	return nil
}

// ContactInput is a contact's details, for a new contact or a new lead's contact.
type ContactInput struct {
	CompanyID string `json:"companyId"`
	Name      string `json:"name"`
	Email     string `json:"email"`
	Role      string `json:"role"`
}

func (c *ContactInput) validate() error {
	c.Name = strings.TrimSpace(c.Name)
	c.Email = strings.TrimSpace(c.Email)
	c.Role = strings.TrimSpace(c.Role)
	switch {
	case c.Name == "" && c.Email == "":
		return invalid("a contact needs a name or an email")
	case c.Email != "" && !strings.Contains(c.Email, "@"):
		return invalid("%q is not an email address", c.Email)
	}
	return tooLong(c.Name, c.Email, c.Role)
}

// NewLead is what a member fills in to add a lead. The company is an existing one (CompanyID) or a new one
// by name (Company), which reuses a company of that name if there is one. The contact is likewise an
// existing one (ContactID), a new one (Contact), or none.
type NewLead struct {
	OwnerID   *string       `json:"ownerId"` // absent or null: no owner yet
	CompanyID string        `json:"companyId"`
	Company   string        `json:"company"`
	ContactID string        `json:"contactId"`
	Contact   *ContactInput `json:"contact"`
	DealType  string        `json:"dealType"`
	Semester  string        `json:"semester"`
	NextStep  string        `json:"nextStep"`
}

func (n *NewLead) validate() error {
	n.Company = strings.TrimSpace(n.Company)
	n.NextStep = strings.TrimSpace(n.NextStep)
	switch {
	case n.CompanyID == "" && n.Company == "":
		return invalid("company is required")
	case n.CompanyID != "" && !uuidPattern.MatchString(n.CompanyID):
		return invalid("companyId is not a company")
	case n.ContactID != "" && !uuidPattern.MatchString(n.ContactID):
		return invalid("contactId is not a contact")
	case n.ContactID != "" && n.Contact != nil:
		return invalid("give contactId or contact, not both")
	case !slices.Contains(dealTypes, n.DealType):
		return invalid("dealType must be one of %s", strings.Join(dealTypes, ", "))
	case !semesterPattern.MatchString(n.Semester):
		return invalid(`semester must look like "HT 2026" or "VT 2026"`)
	}
	if n.Contact != nil {
		if err := n.Contact.validate(); err != nil {
			return err
		}
	}
	return tooLong(n.Company, n.NextStep)
}

// CompanyInput is a company's details, for a new company.
type CompanyInput struct {
	Name    string `json:"name"`
	Website string `json:"website"`
	Notes   string `json:"notes"`
}

func (c *CompanyInput) validate() error {
	c.Name = strings.TrimSpace(c.Name)
	c.Website = strings.TrimSpace(c.Website)
	if c.Name == "" {
		return invalid("a company needs a name")
	}
	return tooLong(c.Name, c.Website, c.Notes)
}

// textPatch reads a PATCH body whose fields are all text, mapping each allowed field to its column.
func textPatch(body []byte, fields map[string]string) (map[string]any, error) {
	var raw map[string]json.RawMessage
	if err := json.Unmarshal(body, &raw); err != nil {
		return nil, invalid("body must be a JSON object")
	}
	if len(raw) == 0 {
		return nil, invalid("nothing to update")
	}
	cols := map[string]any{}
	for field, value := range raw {
		column, ok := fields[field]
		if !ok {
			return nil, invalid("%s can't be changed", field)
		}
		// null would decode to "" and silently erase the field; clearing one takes "".
		var s string
		if string(value) == "null" || json.Unmarshal(value, &s) != nil {
			return nil, invalid("%s must be text", field)
		}
		s = strings.TrimSpace(s)
		if err := tooLong(s); err != nil {
			return nil, err
		}
		cols[column] = s
	}
	return cols, nil
}

// ParseCompanyPatch reads a PATCH to a company: name, website or notes.
func ParseCompanyPatch(body []byte) (map[string]any, error) {
	cols, err := textPatch(body, map[string]string{"name": "name", "website": "website", "notes": "notes"})
	if err == nil && cols["name"] == "" {
		return nil, invalid("a company needs a name")
	}
	return cols, err
}

// ParseContactPatch reads a PATCH to a contact: companyId, name, email or role.
func ParseContactPatch(body []byte) (map[string]any, error) {
	cols, err := textPatch(body, map[string]string{"companyId": "company_id", "name": "name", "email": "email", "role": "role"})
	if err != nil {
		return nil, err
	}
	if id, ok := cols["company_id"].(string); ok && !uuidPattern.MatchString(id) {
		return nil, invalid("companyId is not a company")
	}
	if email, ok := cols["email"].(string); ok && email != "" && !strings.Contains(email, "@") {
		return nil, invalid("%q is not an email address", email)
	}
	return cols, nil
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
			// Not nullable: null would decode to "" and silently erase the next step.
			var s string
			if isNull || json.Unmarshal(value, &s) != nil || len(s) > maxText {
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
			// fee_sek is a Postgres integer: anything larger fails in the driver, as a 500.
			var n int
			if json.Unmarshal(value, &n) != nil || n < 0 || n > math.MaxInt32 {
				return nil, invalid("feeSEK must be a whole number from 0 to %d, or null", math.MaxInt32)
			}
			cols["fee_sek"] = n
		case "ownerId":
			if isNull {
				cols["owner_id"] = nil
				continue
			}
			var id string
			if json.Unmarshal(value, &id) != nil || !uuidPattern.MatchString(id) {
				return nil, invalid("ownerId must be a team member's id, or null")
			}
			cols["owner_id"] = id
		case "contactId":
			if isNull {
				cols["contact_id"] = nil
				continue
			}
			var id string
			if json.Unmarshal(value, &id) != nil || !uuidPattern.MatchString(id) {
				return nil, invalid("contactId must be a contact's id, or null")
			}
			cols["contact_id"] = id
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

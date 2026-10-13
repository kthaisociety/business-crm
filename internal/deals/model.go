// Package deals stores the deal log (CONTEXT.md): deals, the members who own and support them, and each
// deal's timeline of activities.
package deals

import (
	"database/sql/driver"
	"fmt"
	"time"
)

// GORM models, one per table in migrations/00001_create_deals.sql (docs/pcr/0006).

type Member struct {
	ID        string `gorm:"type:uuid;default:gen_random_uuid()"`
	Email     string
	Name      string
	CreatedAt time.Time
}

type Deal struct {
	ID            string `gorm:"type:uuid;default:gen_random_uuid()"`
	Company       string
	DealType      string
	EventType     string
	Audience      string
	Semester      string
	Status        string
	StartDate     *Date
	EndDate       *Date
	Location      string
	OwnerID       *string
	Owner         *Member  `gorm:"foreignKey:OwnerID"`
	Support       []Member `gorm:"many2many:deal_support;joinForeignKey:DealID;joinReferences:MemberID"`
	ContactName   string
	ContactEmail  string
	ContactRole   string
	NextStep      string
	NextStepDue   *Date
	FeeSEK        *int `gorm:"column:fee_sek"`
	InvoiceStatus *string
	InvoiceDue    *Date
	DriveURL      string
	ContractURL   string
	WriteUpURL    string
	Rating        *int
	Notes         string
	Activities    []Activity
	CreatedAt     time.Time
	UpdatedAt     time.Time
}

type Activity struct {
	ID           string `gorm:"type:uuid;default:gen_random_uuid()"`
	DealID       string
	Type         string
	Source       string
	ExternalID   *string
	AuthorID     *string
	Title        string
	Body         string
	BulletPoints []string `gorm:"serializer:json"`
	OccurredAt   time.Time
	CreatedAt    time.Time
}

// Date is a calendar date without a time or zone, YYYY-MM-DD, stored in a Postgres date column.
type Date string

func ParseDate(s string) (Date, error) {
	if _, err := time.Parse(time.DateOnly, s); err != nil {
		return "", fmt.Errorf("%q is not a date (YYYY-MM-DD)", s)
	}
	return Date(s), nil
}

func (d Date) Value() (driver.Value, error) { return string(d), nil }

func (d *Date) Scan(v any) error {
	switch t := v.(type) {
	case time.Time:
		*d = Date(t.Format(time.DateOnly))
	case string:
		*d = Date(t)
	case []byte:
		*d = Date(t)
	default:
		return fmt.Errorf("scanning %T into Date", v)
	}
	return nil
}

// The JSON the API returns, shaped like the frontend's Deal type (frontend/src/data/mockDeals.ts).

type MemberView struct {
	ID    string `json:"id"`
	Name  string `json:"name"`
	Email string `json:"email"`
}

type ContactView struct {
	Name  string `json:"name"`
	Email string `json:"email"`
	Role  string `json:"role"`
}

type LinksView struct {
	Drive    string `json:"drive,omitempty"`
	Contract string `json:"contract,omitempty"`
	WriteUp  string `json:"writeUp,omitempty"`
}

type ActivityView struct {
	ID           string   `json:"id"`
	Type         string   `json:"type"`
	Source       string   `json:"source"`
	Title        string   `json:"title"`
	Timestamp    string   `json:"timestamp"`
	Body         string   `json:"body"`
	BulletPoints []string `json:"bulletPoints,omitempty"`
}

type View struct {
	ID            string         `json:"id"`
	Company       string         `json:"company"`
	DealType      string         `json:"dealType"`
	EventType     string         `json:"eventType"`
	Audience      string         `json:"audience"`
	Semester      string         `json:"semester"`
	Status        string         `json:"status"`
	StartDate     *Date          `json:"startDate"`
	EndDate       *Date          `json:"endDate"`
	Location      string         `json:"location"`
	Owner         *MemberView    `json:"owner"`
	Support       []MemberView   `json:"support"`
	Contact       *ContactView   `json:"contact"`
	NextStep      string         `json:"nextStep"`
	NextStepDue   *Date          `json:"nextStepDue"`
	FeeSEK        *int           `json:"feeSEK"`
	InvoiceStatus *string        `json:"invoiceStatus"`
	InvoiceDue    *Date          `json:"invoiceDue"`
	Links         LinksView      `json:"links"`
	Rating        *int           `json:"rating"`
	Notes         string         `json:"notes"`
	Activities    []ActivityView `json:"activities"`
}

func viewOfMember(m Member) MemberView {
	return MemberView{ID: m.ID, Name: m.Name, Email: m.Email}
}

func viewOfActivity(a Activity) ActivityView {
	return ActivityView{
		ID:           a.ID,
		Type:         a.Type,
		Source:       a.Source,
		Title:        a.Title,
		Timestamp:    a.OccurredAt.UTC().Format(time.RFC3339),
		Body:         a.Body,
		BulletPoints: a.BulletPoints,
	}
}

func viewOf(d Deal) View {
	v := View{
		ID:            d.ID,
		Company:       d.Company,
		DealType:      d.DealType,
		EventType:     d.EventType,
		Audience:      d.Audience,
		Semester:      d.Semester,
		Status:        d.Status,
		StartDate:     d.StartDate,
		EndDate:       d.EndDate,
		Location:      d.Location,
		Support:       []MemberView{},
		NextStep:      d.NextStep,
		NextStepDue:   d.NextStepDue,
		FeeSEK:        d.FeeSEK,
		InvoiceStatus: d.InvoiceStatus,
		InvoiceDue:    d.InvoiceDue,
		Links:         LinksView{Drive: d.DriveURL, Contract: d.ContractURL, WriteUp: d.WriteUpURL},
		Rating:        d.Rating,
		Notes:         d.Notes,
		Activities:    []ActivityView{},
	}
	if d.Owner != nil {
		owner := viewOfMember(*d.Owner)
		v.Owner = &owner
	}
	for _, m := range d.Support {
		v.Support = append(v.Support, viewOfMember(m))
	}
	if d.ContactName != "" || d.ContactEmail != "" {
		v.Contact = &ContactView{Name: d.ContactName, Email: d.ContactEmail, Role: d.ContactRole}
	}
	for _, a := range d.Activities {
		v.Activities = append(v.Activities, viewOfActivity(a))
	}
	return v
}

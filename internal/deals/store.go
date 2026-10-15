package deals

import (
	"context"
	"errors"
	"regexp"
	"time"

	"gorm.io/gorm"
	"gorm.io/gorm/clause"

	"github.com/kthaisociety/business-crm/internal/auth"
)

var ErrNotFound = errors.New("deal not found")

var uuidPattern = regexp.MustCompile(`^[0-9a-fA-F]{8}-[0-9a-fA-F]{4}-[0-9a-fA-F]{4}-[0-9a-fA-F]{4}-[0-9a-fA-F]{12}$`)

type Store struct {
	db *gorm.DB
}

func NewStore(db *gorm.DB) *Store {
	return &Store{db: db}
}

// withEverything loads a deal's company, contact, owner, support and timeline (oldest first) along with it.
func withEverything(db *gorm.DB) *gorm.DB {
	return db.Preload("Company").Preload("Contact").Preload("Owner").Preload("Support").
		Preload("Activities", func(db *gorm.DB) *gorm.DB { return db.Order("occurred_at") })
}

// List returns every deal, oldest first. The team has a few hundred deals at most, so the app loads them
// all and filters by semester itself.
func (s *Store) List(ctx context.Context) ([]View, error) {
	var deals []Deal
	if err := withEverything(s.db.WithContext(ctx)).Order("created_at, id").Find(&deals).Error; err != nil {
		return nil, err
	}
	views := make([]View, 0, len(deals))
	for _, d := range deals {
		views = append(views, viewOf(d))
	}
	return views, nil
}

func (s *Store) Get(ctx context.Context, id string) (View, error) {
	return get(s.db.WithContext(ctx), id)
}

func get(db *gorm.DB, id string) (View, error) {
	if !uuidPattern.MatchString(id) {
		return View{}, ErrNotFound
	}
	var d Deal
	err := withEverything(db).Where("id = ?", id).Take(&d).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return View{}, ErrNotFound
	}
	if err != nil {
		return View{}, err
	}
	return viewOf(d), nil
}

// Create adds a lead owned by whoever adds it, with an "Added as a lead" entry on its timeline. A new
// company or contact given by name is added along with it.
func (s *Store) Create(ctx context.Context, lead NewLead, by auth.User) (View, error) {
	if err := lead.validate(); err != nil {
		return View{}, err
	}
	eventType := "TBA"
	if lead.DealType == "Job Post" {
		eventType = "Job post"
	}
	var view View
	err := s.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		owner, err := memberFor(tx, by)
		if err != nil {
			return err
		}
		var company Company
		if lead.CompanyID != "" {
			company, err = companyByID(tx, lead.CompanyID)
		} else {
			company, err = companyNamed(tx, lead.Company)
		}
		if err != nil {
			return err
		}
		var contactID *string
		switch {
		case lead.ContactID != "":
			c, err := contactAt(tx, lead.ContactID, company)
			if err != nil {
				return err
			}
			contactID = &c.ID
		case lead.Contact != nil:
			c, err := createContact(tx, company.ID, *lead.Contact)
			if err != nil {
				return err
			}
			contactID = &c.ID
		}
		d := Deal{
			CompanyID: company.ID,
			ContactID: contactID,
			DealType:  lead.DealType,
			EventType: eventType,
			Audience:  "Unclear",
			Semester:  lead.Semester,
			Status:    "lead",
			OwnerID:   &owner.ID,
			NextStep:  lead.NextStep,
		}
		if err := tx.Omit(clause.Associations).Create(&d).Error; isForeignKeyViolation(err) {
			return invalid("that contact isn't at %s", company.Name) // moved there after the check above
		} else if err != nil {
			return err
		}
		if err := tx.Create(&Activity{
			DealID:       d.ID,
			Type:         "note",
			Source:       "manual",
			AuthorID:     &owner.ID,
			Title:        "Added as a lead",
			BulletPoints: []string{},
			OccurredAt:   time.Now(),
		}).Error; err != nil {
			return err
		}
		view, err = get(tx, d.ID)
		return err
	})
	return view, err
}

// Update sets the columns ParsePatch returned.
func (s *Store) Update(ctx context.Context, id string, cols map[string]any) (View, error) {
	if !uuidPattern.MatchString(id) {
		return View{}, ErrNotFound
	}
	var view View
	err := s.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		if contactID, ok := cols["contact_id"].(string); ok {
			var d Deal
			if err := tx.Preload("Company").Where("id = ?", id).Take(&d).Error; errors.Is(err, gorm.ErrRecordNotFound) {
				return ErrNotFound
			} else if err != nil {
				return err
			}
			if _, err := contactAt(tx, contactID, *d.Company); err != nil {
				return err
			}
		}
		cols["updated_at"] = time.Now()
		res := tx.Model(&Deal{}).Where("id = ?", id).Updates(cols)
		if isForeignKeyViolation(res.Error) {
			return invalid("that contact isn't at the deal's company")
		}
		if res.Error != nil {
			return res.Error
		}
		if res.RowsAffected == 0 {
			return ErrNotFound
		}
		var err error
		view, err = get(tx, id)
		return err
	})
	return view, err
}

// AddNote puts a quick note from a member on a deal's timeline. A touchpoint resets the follow-up nudge.
func (s *Store) AddNote(ctx context.Context, dealID, text string, by auth.User) (ActivityView, error) {
	if !uuidPattern.MatchString(dealID) {
		return ActivityView{}, ErrNotFound
	}
	var view ActivityView
	err := s.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		var exists int64
		if err := tx.Model(&Deal{}).Where("id = ?", dealID).Count(&exists).Error; err != nil {
			return err
		}
		if exists == 0 {
			return ErrNotFound
		}
		author, err := memberFor(tx, by)
		if err != nil {
			return err
		}
		a := Activity{
			DealID:       dealID,
			Type:         "note",
			Source:       "manual",
			AuthorID:     &author.ID,
			Title:        "Quick note",
			Body:         text,
			BulletPoints: []string{},
			OccurredAt:   time.Now(),
		}
		if err := tx.Create(&a).Error; err != nil {
			return err
		}
		view = viewOfActivity(a)
		return nil
	})
	return view, err
}

// contactAt finds a contact who works at the company: a deal's contact must.
func contactAt(tx *gorm.DB, id string, company Company) (Contact, error) {
	var c Contact
	err := tx.Where("id = ?", id).Take(&c).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return c, invalid("contactId is not a contact")
	}
	if err == nil && c.CompanyID != company.ID {
		return c, invalid("that contact isn't at %s", company.Name)
	}
	return c, err
}

// memberFor finds the signed-in user's member row, adding it the first time they change anything.
func memberFor(tx *gorm.DB, u auth.User) (Member, error) {
	name := u.Name
	if name == "" {
		name = u.Email
	}
	m := Member{Email: u.Email, Name: name}
	err := tx.Clauses(clause.OnConflict{
		Columns:   []clause.Column{{Name: "email"}},
		DoUpdates: clause.AssignmentColumns([]string{"name"}),
	}).Create(&m).Error
	return m, err
}

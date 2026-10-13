package deals

import (
	"context"
	"errors"
	"time"

	"github.com/jackc/pgx/v5/pgconn"
	"gorm.io/gorm"
)

var (
	ErrCompanyNotFound = errors.New("company not found")
	ErrContactNotFound = errors.New("contact not found")
)

// isUniqueViolation reports whether err is Postgres refusing a duplicate (a company name or contact
// email that's taken).
func isUniqueViolation(err error) bool {
	var pgErr *pgconn.PgError
	return errors.As(err, &pgErr) && pgErr.Code == "23505"
}

func (s *Store) ListCompanies(ctx context.Context) ([]CompanyView, error) {
	var companies []Company
	if err := s.db.WithContext(ctx).Order("lower(name)").Find(&companies).Error; err != nil {
		return nil, err
	}
	views := make([]CompanyView, 0, len(companies))
	for _, c := range companies {
		views = append(views, viewOfCompany(c))
	}
	return views, nil
}

func (s *Store) CreateCompany(ctx context.Context, in CompanyInput) (CompanyView, error) {
	if err := in.validate(); err != nil {
		return CompanyView{}, err
	}
	c := Company{Name: in.Name, Website: in.Website, Notes: in.Notes}
	if err := s.db.WithContext(ctx).Create(&c).Error; err != nil {
		if isUniqueViolation(err) {
			return CompanyView{}, conflict("there is already a company called %s", in.Name)
		}
		return CompanyView{}, err
	}
	return viewOfCompany(c), nil
}

func (s *Store) UpdateCompany(ctx context.Context, id string, cols map[string]any) (CompanyView, error) {
	if !uuidPattern.MatchString(id) {
		return CompanyView{}, ErrCompanyNotFound
	}
	cols["updated_at"] = time.Now()
	res := s.db.WithContext(ctx).Model(&Company{}).Where("id = ?", id).Updates(cols)
	if isUniqueViolation(res.Error) {
		return CompanyView{}, conflict("there is already a company called %s", cols["name"])
	}
	if res.Error != nil {
		return CompanyView{}, res.Error
	}
	if res.RowsAffected == 0 {
		return CompanyView{}, ErrCompanyNotFound
	}
	var c Company
	err := s.db.WithContext(ctx).Where("id = ?", id).Take(&c).Error
	return viewOfCompany(c), err
}

// DeleteCompany removes a company and its contacts. A company with deals stays: deleting it would lose
// their history.
func (s *Store) DeleteCompany(ctx context.Context, id string) error {
	if !uuidPattern.MatchString(id) {
		return ErrCompanyNotFound
	}
	return s.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		var deals int64
		if err := tx.Model(&Deal{}).Where("company_id = ?", id).Count(&deals).Error; err != nil {
			return err
		}
		if deals > 0 {
			return conflict("the company has %d %s; it can only be deleted without any", deals, plural(deals, "deal", "deals"))
		}
		res := tx.Where("id = ?", id).Delete(&Company{})
		if res.Error == nil && res.RowsAffected == 0 {
			return ErrCompanyNotFound
		}
		return res.Error
	})
}

func (s *Store) ListContacts(ctx context.Context) ([]ContactView, error) {
	var contacts []Contact
	if err := s.db.WithContext(ctx).Order("lower(name), lower(email)").Find(&contacts).Error; err != nil {
		return nil, err
	}
	views := make([]ContactView, 0, len(contacts))
	for _, c := range contacts {
		views = append(views, viewOfContact(c))
	}
	return views, nil
}

func (s *Store) CreateContact(ctx context.Context, in ContactInput) (ContactView, error) {
	if err := in.validate(); err != nil {
		return ContactView{}, err
	}
	if !uuidPattern.MatchString(in.CompanyID) {
		return ContactView{}, invalid("companyId is not a company")
	}
	var view ContactView
	err := s.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		if _, err := companyByID(tx, in.CompanyID); err != nil {
			return err
		}
		// Unlike a new lead's contact, a contact added on its own is never quietly the existing one.
		if in.Email != "" {
			var taken int64
			if err := tx.Model(&Contact{}).Where("lower(email) = lower(?)", in.Email).Count(&taken).Error; err != nil {
				return err
			}
			if taken > 0 {
				return conflict("%s is already a contact", in.Email)
			}
		}
		c, err := createContact(tx, in.CompanyID, in)
		view = viewOfContact(c)
		return err
	})
	return view, err
}

// UpdateContact changes a contact. Moving one with deals to another company is refused: each deal's
// contact must work at the deal's company.
func (s *Store) UpdateContact(ctx context.Context, id string, cols map[string]any) (ContactView, error) {
	if !uuidPattern.MatchString(id) {
		return ContactView{}, ErrContactNotFound
	}
	var view ContactView
	err := s.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		var c Contact
		if err := tx.Where("id = ?", id).Take(&c).Error; errors.Is(err, gorm.ErrRecordNotFound) {
			return ErrContactNotFound
		} else if err != nil {
			return err
		}
		if companyID, ok := cols["company_id"].(string); ok && companyID != c.CompanyID {
			if _, err := companyByID(tx, companyID); err != nil {
				return err
			}
			var deals int64
			if err := tx.Model(&Deal{}).Where("contact_id = ?", id).Count(&deals).Error; err != nil {
				return err
			}
			if deals > 0 {
				return conflict("the contact is on %d %s with their current company; add them again at the new one", deals, plural(deals, "deal", "deals"))
			}
		}
		name, email := c.Name, c.Email
		if v, ok := cols["name"].(string); ok {
			name = v
		}
		if v, ok := cols["email"].(string); ok {
			email = v
		}
		if name == "" && email == "" {
			return invalid("a contact needs a name or an email")
		}
		cols["updated_at"] = time.Now()
		if err := tx.Model(&Contact{}).Where("id = ?", id).Updates(cols).Error; err != nil {
			if isUniqueViolation(err) {
				return conflict("%s is already a contact", email)
			}
			return err
		}
		if err := tx.Where("id = ?", id).Take(&c).Error; err != nil {
			return err
		}
		view = viewOfContact(c)
		return nil
	})
	return view, err
}

// DeleteContact removes a contact who isn't on any deal. One who is stays, so the deals keep who they
// were with; change those deals' contact first.
func (s *Store) DeleteContact(ctx context.Context, id string) error {
	if !uuidPattern.MatchString(id) {
		return ErrContactNotFound
	}
	return s.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		var deals int64
		if err := tx.Model(&Deal{}).Where("contact_id = ?", id).Count(&deals).Error; err != nil {
			return err
		}
		if deals > 0 {
			return conflict("the contact is on %d %s; change %s contact first", deals, plural(deals, "deal", "deals"), plural(deals, "its", "their"))
		}
		res := tx.Where("id = ?", id).Delete(&Contact{})
		if res.Error == nil && res.RowsAffected == 0 {
			return ErrContactNotFound
		}
		return res.Error
	})
}

func companyByID(tx *gorm.DB, id string) (Company, error) {
	var c Company
	err := tx.Where("id = ?", id).Take(&c).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return c, invalid("companyId is not a company")
	}
	return c, err
}

// companyNamed finds the company with this name, ignoring case, or adds it.
func companyNamed(tx *gorm.DB, name string) (Company, error) {
	var c Company
	err := tx.Where("lower(name) = lower(?)", name).Take(&c).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		c = Company{Name: name}
		err = tx.Create(&c).Error
	}
	return c, err
}

// createContact adds a contact at a company. An email that's already a contact there is that contact;
// at another company, it's a conflict.
func createContact(tx *gorm.DB, companyID string, in ContactInput) (Contact, error) {
	if in.Email != "" {
		var existing Contact
		err := tx.Where("lower(email) = lower(?)", in.Email).Take(&existing).Error
		if err == nil {
			if existing.CompanyID != companyID {
				var other Company
				_ = tx.Where("id = ?", existing.CompanyID).Take(&other).Error
				return existing, conflict("%s is already a contact at %s", in.Email, other.Name)
			}
			return existing, nil
		}
		if !errors.Is(err, gorm.ErrRecordNotFound) {
			return existing, err
		}
	}
	c := Contact{CompanyID: companyID, Name: in.Name, Email: in.Email, Role: in.Role}
	return c, tx.Create(&c).Error
}

func plural(n int64, one, many string) string {
	if n == 1 {
		return one
	}
	return many
}

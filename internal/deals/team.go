package deals

import (
	"context"
	"errors"
	"slices"
	"strings"

	"gorm.io/gorm"

	"github.com/kthaisociety/business-crm/internal/auth"
)

// The business team (docs/adr/0003): who can own and support deals, and what role they have here.

var (
	ErrMemberNotFound = errors.New("team member not found")
	Roles             = []string{"admin", "board", "member"}
)

// ForbiddenError is a request the signed-in member's role doesn't allow; the API answers 403.
type ForbiddenError struct{ msg string }

func (e *ForbiddenError) Error() string { return e.msg }

// SignIn records the signed-in user as a team member, the first time adding them as a member, and
// returns who they are on the team.
func (s *Store) SignIn(ctx context.Context, u auth.User) (MemberView, error) {
	var m Member
	err := s.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		var err error
		m, err = s.memberFor(tx, u)
		return err
	})
	return viewOfMember(m), err
}

// RequireAdmin is nil if the signed-in user is an admin, and a ForbiddenError otherwise.
func (s *Store) RequireAdmin(ctx context.Context, u auth.User) error {
	me, err := s.SignIn(ctx, u)
	if err != nil {
		return err
	}
	if me.Role != "admin" {
		return &ForbiddenError{msg: "only admins can change the team"}
	}
	return nil
}

func (s *Store) ListMembers(ctx context.Context) ([]MemberView, error) {
	var members []Member
	if err := s.db.WithContext(ctx).Order("lower(name)").Find(&members).Error; err != nil {
		return nil, err
	}
	views := make([]MemberView, 0, len(members))
	for _, m := range members {
		views = append(views, viewOfMember(m))
	}
	return views, nil
}

// MemberInput adds someone to the team before they've signed in, so deals can be assigned to them.
type MemberInput struct {
	Email string `json:"email"`
	Name  string `json:"name"`
	Role  string `json:"role"`
}

func (in *MemberInput) validate() error {
	in.Email = strings.ToLower(strings.TrimSpace(in.Email))
	in.Name = strings.TrimSpace(in.Name)
	if in.Role == "" {
		in.Role = "member"
	}
	switch {
	case !strings.Contains(in.Email, "@"):
		return invalid("a team member needs an email address")
	case in.Name == "":
		return invalid("a team member needs a name")
	case !slices.Contains(Roles, in.Role):
		return invalid("role must be one of %s", strings.Join(Roles, ", "))
	}
	return tooLong(in.Email, in.Name)
}

func (s *Store) AddMember(ctx context.Context, in MemberInput) (MemberView, error) {
	if err := in.validate(); err != nil {
		return MemberView{}, err
	}
	m := Member{Email: in.Email, Name: in.Name, Role: in.Role}
	err := s.db.WithContext(ctx).Create(&m).Error
	if isUniqueViolation(err) {
		return MemberView{}, conflict("%s is already on the team", in.Email)
	}
	return viewOfMember(m), err
}

// ParseMemberPatch reads a PATCH to a team member: name or role.
func ParseMemberPatch(body []byte) (map[string]any, error) {
	cols, err := textPatch(body, map[string]string{"name": "name", "role": "role"})
	if err != nil {
		return nil, err
	}
	if cols["name"] == "" {
		return nil, invalid("a team member needs a name")
	}
	if role, ok := cols["role"].(string); ok && !slices.Contains(Roles, role) {
		return nil, invalid("role must be one of %s", strings.Join(Roles, ", "))
	}
	return cols, nil
}

func (s *Store) UpdateMember(ctx context.Context, id string, cols map[string]any) (MemberView, error) {
	if !uuidPattern.MatchString(id) {
		return MemberView{}, ErrMemberNotFound
	}
	var m Member
	err := s.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		if err := tx.Where("id = ?", id).Take(&m).Error; errors.Is(err, gorm.ErrRecordNotFound) {
			return ErrMemberNotFound
		} else if err != nil {
			return err
		}
		if role, ok := cols["role"].(string); ok && m.Role == "admin" && role != "admin" {
			if err := keepAnAdmin(tx); err != nil {
				return err
			}
		}
		if err := tx.Model(&Member{}).Where("id = ?", id).Updates(cols).Error; err != nil {
			return err
		}
		return tx.Where("id = ?", id).Take(&m).Error
	})
	return viewOfMember(m), err
}

// RemoveMember takes someone off the team. Someone who owns or supports deals stays until those deals
// are reassigned, so the deals' history keeps a name.
func (s *Store) RemoveMember(ctx context.Context, id string) error {
	if !uuidPattern.MatchString(id) {
		return ErrMemberNotFound
	}
	return s.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		var m Member
		if err := tx.Where("id = ?", id).Take(&m).Error; errors.Is(err, gorm.ErrRecordNotFound) {
			return ErrMemberNotFound
		} else if err != nil {
			return err
		}
		if m.Role == "admin" {
			if err := keepAnAdmin(tx); err != nil {
				return err
			}
		}
		var owned, supported, authored int64
		if err := tx.Model(&Deal{}).Where("owner_id = ?", id).Count(&owned).Error; err != nil {
			return err
		}
		if err := tx.Table("deal_support").Where("member_id = ?", id).Count(&supported).Error; err != nil {
			return err
		}
		if owned+supported > 0 {
			return conflict("%s owns or supports %d %s; reassign them first", m.Name, owned+supported, plural(owned+supported, "deal", "deals"))
		}
		if err := tx.Model(&Activity{}).Where("author_id = ?", id).Count(&authored).Error; err != nil {
			return err
		}
		if authored > 0 {
			// Their notes stay on the timeline, just without an author.
			if err := tx.Model(&Activity{}).Where("author_id = ?", id).Update("author_id", nil).Error; err != nil {
				return err
			}
		}
		return tx.Delete(&m).Error
	})
}

// keepAnAdmin refuses a change that would leave the team without an admin.
func keepAnAdmin(tx *gorm.DB) error {
	var admins int64
	if err := tx.Model(&Member{}).Where("role = 'admin'").Count(&admins).Error; err != nil {
		return err
	}
	if admins <= 1 {
		return conflict("the team needs at least one admin")
	}
	return nil
}

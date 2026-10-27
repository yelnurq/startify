// Package seed creates universities and the demonstration data of a fresh
// installation: the structure of KazUTB, demo accounts of every role and
// requests on the teammates board. Every step is idempotent.
package seed

import (
	"context"
	"encoding/json"
	"fmt"
	"time"

	"github.com/google/uuid"

	"github.com/yelnurq/startify/backend/internal/apiconv"
	"github.com/yelnurq/startify/backend/internal/auth"
	"github.com/yelnurq/startify/backend/internal/auth/authdb"
	"github.com/yelnurq/startify/backend/internal/org/orgdb"
	"github.com/yelnurq/startify/backend/internal/platform/db"
	"github.com/yelnurq/startify/backend/internal/team/teamdb"
	"github.com/yelnurq/startify/backend/internal/tenant/tenantdb"
	"github.com/yelnurq/startify/backend/internal/user"
	"github.com/yelnurq/startify/backend/internal/user/userdb"
)

// TenantSpec describes a university.
type TenantSpec struct {
	Slug           string
	Name           apiconv.Text
	ShortName      apiconv.Text
	Locale         string
	Color          string
	Domains        []string
	SelfSignup     bool
	EmailDomains   []string
	ConsentVersion string
	ConsentText    apiconv.Text
	// The intellectual property policy teams accept with a new project.
	IPPolicyVersion string
	IPPolicyText    apiconv.Text
}

// Tenant creates or updates a university with its settings.
func Tenant(ctx context.Context, database *db.DB, spec TenantSpec) (tenantdb.Tenant, error) {
	var t tenantdb.Tenant
	err := database.Tx(db.WithSystem(ctx), func(ctx context.Context) error {
		q := tenantdb.New(database.Conn(ctx))
		var err error
		t, err = q.UpsertTenant(ctx, tenantdb.UpsertTenantParams{
			Slug: spec.Slug, Name: spec.Name.JSON(), ShortName: spec.ShortName.JSON(),
			DefaultLocale: spec.Locale, PrimaryColor: spec.Color,
		})
		if err != nil {
			return err
		}
		for i, d := range spec.Domains {
			if err := q.UpsertTenantDomain(ctx, tenantdb.UpsertTenantDomainParams{Domain: d, TenantID: t.ID, IsPrimary: i == 0}); err != nil {
				return err
			}
		}
		domains := spec.EmailDomains
		if domains == nil {
			domains = []string{}
		}
		return q.UpsertTenantSettings(ctx, tenantdb.UpsertTenantSettingsParams{
			TenantID: t.ID, SelfSignup: spec.SelfSignup, EmailDomains: domains,
			ConsentVersion: spec.ConsentVersion, ConsentText: spec.ConsentText.JSON(),
			IpPolicyVersion: spec.IPPolicyVersion, IpPolicyText: ipPolicy(spec.IPPolicyText),
		})
	})
	if err != nil {
		return t, fmt.Errorf("seed: tenant %s: %w", spec.Slug, err)
	}
	return t, nil
}

func ipPolicy(t apiconv.Text) json.RawMessage {
	if len(t) == 0 {
		return json.RawMessage("{}")
	}
	return t.JSON()
}

// Member describes a demo account.
type Member struct {
	Email    string
	FullName string
	Locale   string
	Password string
	// PasswordHash, when set, is used instead of hashing Password again.
	PasswordHash string
	Roles        []string
	Group        string
	Course       int16
	Headline     string
	Bio          string
	Skills       []string
	Interests    []string
	Slug         string
	Public       bool
	LookTeam     bool
}

// Account creates or updates a member of the tenant with a verified e-mail,
// a password, a finished onboarding and a profile. It returns the user id.
func Account(ctx context.Context, database *db.DB, tenantID uuid.UUID, m Member) (uuid.UUID, error) {
	hash := m.PasswordHash
	if hash == "" {
		var err error
		if hash, err = auth.HashPassword(m.Password); err != nil {
			return uuid.Nil, err
		}
	}
	var userID uuid.UUID
	err := database.Tx(db.WithTenant(ctx, tenantID), func(ctx context.Context) error {
		aq := authdb.New(database.Conn(ctx))
		uq := userdb.New(database.Conn(ctx))
		u, err := aq.GetUserByEmail(ctx, m.Email)
		if db.IsNoRows(err) {
			now := time.Now()
			u, err = aq.CreateUser(ctx, authdb.CreateUserParams{Email: m.Email, FullName: m.FullName, Locale: m.Locale, EmailVerifiedAt: &now})
		}
		if err != nil {
			return err
		}
		userID = u.ID
		if err := aq.UpsertPasswordIdentity(ctx, authdb.UpsertPasswordIdentityParams{UserID: u.ID, Subject: u.ID.String(), PasswordHash: &hash}); err != nil {
			return err
		}
		if err := aq.MarkEmailVerified(ctx, u.ID); err != nil {
			return err
		}
		version := "seed"
		now := time.Now()
		if _, err := uq.CreateMembership(ctx, userdb.CreateMembershipParams{
			TenantID: tenantID, UserID: u.ID, Roles: m.Roles, Status: user.StatusActive, ConsentVersion: &version, ConsentAt: &now,
		}); err != nil {
			return err
		}
		if _, err := uq.UpdateMembershipAccess(ctx, userdb.UpdateMembershipAccessParams{Roles: m.Roles, UserID: u.ID}); err != nil {
			return err
		}
		study := userdb.UpdateMembershipStudyParams{UserID: u.ID, OnboardedAt: &now}
		if m.Group != "" {
			g, err := orgdb.New(database.Conn(ctx)).FindStudyGroupByCode(ctx, m.Group)
			if err != nil {
				return fmt.Errorf("group %s: %w", m.Group, err)
			}
			study.GroupID, study.FacultyID, study.DepartmentID, study.StudyLevel = &g.ID, &g.FacultyID, g.DepartmentID, &g.StudyLevel
			if m.Course > 0 {
				study.Course = &m.Course
			}
		}
		if _, err := uq.UpdateMembershipStudy(ctx, study); err != nil {
			return err
		}
		if _, err := uq.CreateProfile(ctx, userdb.CreateProfileParams{UserID: u.ID, PublicSlug: m.Slug}); err != nil {
			return err
		}
		var headline, bio *string
		if m.Headline != "" {
			headline = &m.Headline
		}
		if m.Bio != "" {
			bio = &m.Bio
		}
		_, err = uq.UpdateProfile(ctx, userdb.UpdateProfileParams{
			Headline: headline, Bio: bio, Skills: orEmpty(m.Skills), Interests: orEmpty(m.Interests),
			Links: json.RawMessage("{}"), PublicSlug: m.Slug, IsPublic: m.Public, LookingForTeam: m.LookTeam, UserID: u.ID,
		})
		return err
	})
	if err != nil {
		return uuid.Nil, fmt.Errorf("seed: account %s: %w", m.Email, err)
	}
	return userID, nil
}

func orEmpty(v []string) []string {
	if v == nil {
		return []string{}
	}
	return v
}

// Faculty is a faculty with its departments and groups.
type Faculty struct {
	Code        string
	Name        apiconv.Text
	Departments []Department
}

// Department is a department with its groups.
type Department struct {
	Code   string
	Name   apiconv.Text
	Groups []Group
}

// Group is a study group.
type Group struct {
	Code  string
	Level string
	Year  int16
}

// Structure creates the faculties, departments and groups of a tenant.
func Structure(ctx context.Context, database *db.DB, tenantID uuid.UUID, faculties []Faculty) error {
	err := database.Tx(db.WithTenant(ctx, tenantID), func(ctx context.Context) error {
		q := orgdb.New(database.Conn(ctx))
		for _, f := range faculties {
			fac, err := q.UpsertFaculty(ctx, orgdb.UpsertFacultyParams{Code: f.Code, Name: f.Name.JSON()})
			if err != nil {
				return err
			}
			for _, d := range f.Departments {
				dep, err := q.UpsertDepartment(ctx, orgdb.UpsertDepartmentParams{FacultyID: fac.ID, Code: d.Code, Name: d.Name.JSON()})
				if err != nil {
					return err
				}
				for _, g := range d.Groups {
					year := g.Year
					if _, err := q.UpsertStudyGroup(ctx, orgdb.UpsertStudyGroupParams{
						FacultyID: fac.ID, DepartmentID: &dep.ID, Code: g.Code, StudyLevel: g.Level, AdmissionYear: &year,
					}); err != nil {
						return err
					}
				}
			}
		}
		return nil
	})
	if err != nil {
		return fmt.Errorf("seed: structure: %w", err)
	}
	return nil
}

// TeamRequest is a demo request for teammates.
type TeamRequest struct {
	Author      string
	Title       string
	Description string
	Roles       []string
	Skills      []string
}

// TeamRequests publishes requests unless the author already has one with the title.
func TeamRequests(ctx context.Context, database *db.DB, tenantID uuid.UUID, authors map[string]uuid.UUID, reqs []TeamRequest) error {
	err := database.Tx(db.WithTenant(ctx, tenantID), func(ctx context.Context) error {
		q := teamdb.New(database.Conn(ctx))
		for i := range reqs {
			r := &reqs[i]
			author := authors[r.Author]
			existing, err := q.ListTeamRequests(ctx, teamdb.ListTeamRequestsParams{Mine: true, ViewerID: author, RowLimit: 100})
			if err != nil {
				return err
			}
			dup := false
			for j := range existing {
				dup = dup || existing[j].Title == r.Title
			}
			if dup {
				continue
			}
			if _, err := q.CreateTeamRequest(ctx, teamdb.CreateTeamRequestParams{
				AuthorID: author, Title: r.Title, Description: r.Description, RolesNeeded: r.Roles, Skills: r.Skills,
			}); err != nil {
				return err
			}
		}
		return nil
	})
	if err != nil {
		return fmt.Errorf("seed: team requests: %w", err)
	}
	return nil
}

// Package user owns accounts as members see them: the signed-in user, their
// membership in the university, the profile, and the member directory of
// administrators. Users and profiles are global; everything else is read
// through the membership in the current university (D-007).
package user

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"mime/multipart"
	"regexp"
	"slices"
	"strings"
	"time"

	"github.com/google/uuid"
	openapi_types "github.com/oapi-codegen/runtime/types"

	"github.com/yelnurq/startify/backend/internal/apiconv"
	"github.com/yelnurq/startify/backend/internal/apigen"
	"github.com/yelnurq/startify/backend/internal/audit"
	"github.com/yelnurq/startify/backend/internal/files"
	"github.com/yelnurq/startify/backend/internal/org"
	"github.com/yelnurq/startify/backend/internal/platform/apperr"
	"github.com/yelnurq/startify/backend/internal/platform/authz"
	"github.com/yelnurq/startify/backend/internal/platform/db"
	"github.com/yelnurq/startify/backend/internal/platform/pagination"
	"github.com/yelnurq/startify/backend/internal/platform/tenancy"
	"github.com/yelnurq/startify/backend/internal/platform/validate"
	"github.com/yelnurq/startify/backend/internal/tenant"
	"github.com/yelnurq/startify/backend/internal/user/userdb"
)

// Membership statuses.
const (
	StatusActive    = "active"
	StatusInvited   = "invited"
	StatusSuspended = "suspended"
)

const maxAvatarBytes = 10 << 20

var (
	errMemberNotFound  = apperr.NotFound("member.not_found", "the member does not exist")
	errProfileNotFound = apperr.NotFound("profile.not_found", "the profile does not exist or is hidden")
	telegramPattern    = regexp.MustCompile(`^[A-Za-z][A-Za-z0-9_]{4,31}$`)
	likeEscaper        = strings.NewReplacer(`\`, `\\`, `%`, `\%`, `_`, `\_`)
)

// AccessRevoker ends access granted by issued tokens (implemented by the auth module).
type AccessRevoker interface {
	// RolesChanged invalidates access tokens carrying the previous roles.
	RolesChanged(ctx context.Context, tenantID, userID uuid.UUID) error
	// RevokeAll ends every session of the user in the tenant.
	RevokeAll(ctx context.Context, tenantID, userID uuid.UUID, reason string) error
}

// Service manages members and profiles.
type Service struct {
	db      *db.DB
	org     *org.Service
	files   *files.Service
	tenants *tenant.Service
	audit   *audit.Logger
	revoker AccessRevoker
}

// NewService returns the user service.
func NewService(database *db.DB, orgs *org.Service, fs *files.Service, tenants *tenant.Service,
	auditLog *audit.Logger, revoker AccessRevoker,
) *Service {
	return &Service{db: database, org: orgs, files: fs, tenants: tenants, audit: auditLog, revoker: revoker}
}

func (s *Service) q(ctx context.Context) *userdb.Queries { return userdb.New(s.db.Conn(ctx)) }

// userAPI converts a stored user.
func userAPI(u userdb.User) apigen.User {
	return apigen.User{
		AvatarUrl:     apiconv.FileURL(u.AvatarFileID),
		CreatedAt:     u.CreatedAt,
		Email:         openapi_types.Email(u.Email),
		EmailVerified: u.EmailVerifiedAt != nil,
		FullName:      u.FullName,
		Id:            u.ID,
		IsSuperAdmin:  u.IsSuperAdmin,
		Locale:        apigen.Locale(u.Locale),
	}
}

func rolesAPI(roles []string) []apigen.Role {
	out := make([]apigen.Role, len(roles))
	for i, r := range roles {
		out[i] = apigen.Role(r)
	}
	return out
}

func courseAPI(c *int16) *int {
	if c == nil {
		return nil
	}
	v := int(*c)
	return &v
}

func membershipAPI(m userdb.Membership, refs org.Refs) apigen.Membership {
	out := apigen.Membership{
		Course:    courseAPI(m.Course),
		Onboarded: m.OnboardedAt != nil,
		Roles:     rolesAPI(m.Roles),
		Status:    apigen.MembershipStatus(m.Status),
		TenantId:  m.TenantID,
	}
	if m.StudyLevel != nil {
		l := apigen.StudyLevel(*m.StudyLevel)
		out.StudyLevel = &l
	}
	if m.FacultyID != nil {
		if f, ok := refs.Faculties[*m.FacultyID]; ok {
			out.Faculty = &f
		}
	}
	if m.DepartmentID != nil {
		if d, ok := refs.Departments[*m.DepartmentID]; ok {
			out.Department = &d
		}
	}
	if m.GroupID != nil {
		if g, ok := refs.Groups[*m.GroupID]; ok {
			out.Group = &g
		}
	}
	return out
}

func linksAPI(raw json.RawMessage) apigen.ProfileLinks {
	var l apigen.ProfileLinks
	_ = json.Unmarshal(raw, &l)
	return l
}

func orEmpty(v []string) []string {
	if v == nil {
		return []string{}
	}
	return v
}

func profileAPI(p userdb.UserProfile) apigen.Profile {
	return apigen.Profile{
		Bio:            p.Bio,
		Headline:       p.Headline,
		Interests:      orEmpty(p.Interests),
		IsPublic:       p.IsPublic,
		Links:          linksAPI(p.Links),
		LookingForTeam: p.LookingForTeam,
		PublicSlug:     p.PublicSlug,
		Skills:         orEmpty(p.Skills),
	}
}

func ids(v ...*uuid.UUID) []uuid.UUID {
	out := make([]uuid.UUID, 0, len(v))
	for _, id := range v {
		if id != nil {
			out = append(out, *id)
		}
	}
	return out
}

func (s *Service) membershipRefs(ctx context.Context, m userdb.Membership) (org.Refs, error) {
	return s.org.LoadRefs(ctx, ids(m.FacultyID), ids(m.DepartmentID), ids(m.GroupID))
}

// Session describes the signed-in user in the current university.
func (s *Service) Session(ctx context.Context, userID uuid.UUID) (apigen.Session, error) {
	t, err := tenancy.Require(ctx)
	if err != nil {
		return apigen.Session{}, err
	}
	var out apigen.Session
	err = s.db.ReadTx(ctx, func(ctx context.Context) error {
		q := s.q(ctx)
		u, err := q.GetUser(ctx, userID)
		if err != nil {
			return err
		}
		m, err := q.GetMembership(ctx, userID)
		if err != nil {
			return err
		}
		refs, err := s.membershipRefs(ctx, m)
		if err != nil {
			return err
		}
		brief, err := s.tenants.Brief(ctx, t.ID)
		if err != nil {
			return err
		}
		out = apigen.Session{User: userAPI(u), Membership: membershipAPI(m, refs), Tenant: brief}
		return nil
	})
	if err != nil {
		return out, fmt.Errorf("user: session: %w", err)
	}
	return out, nil
}

// Me describes the signed-in user with membership and profile.
func (s *Service) Me(ctx context.Context) (apigen.Me, error) {
	p, err := authz.Require(ctx)
	if err != nil {
		return apigen.Me{}, err
	}
	var out apigen.Me
	missing := false
	err = s.db.ReadTx(ctx, func(ctx context.Context) error {
		q := s.q(ctx)
		u, err := q.GetUser(ctx, p.UserID)
		if err != nil {
			return err
		}
		m, err := q.GetMembership(ctx, p.UserID)
		if err != nil {
			return err
		}
		prof, err := q.GetProfile(ctx, p.UserID)
		if db.IsNoRows(err) {
			missing = true
			return nil
		}
		if err != nil {
			return err
		}
		refs, err := s.membershipRefs(ctx, m)
		if err != nil {
			return err
		}
		out = apigen.Me{User: userAPI(u), Membership: membershipAPI(m, refs), Profile: profileAPI(prof)}
		return nil
	})
	if db.IsNoRows(err) {
		return out, authz.ErrUnauthenticated
	}
	if err != nil {
		return out, fmt.Errorf("user: me: %w", err)
	}
	if missing {
		err := s.db.Tx(ctx, func(ctx context.Context) error {
			u, err := s.q(ctx).GetUser(ctx, p.UserID)
			if err != nil {
				return err
			}
			return s.EnsureProfile(ctx, u.ID, u.FullName)
		})
		if err != nil {
			return out, fmt.Errorf("user: create profile: %w", err)
		}
		return s.Me(ctx)
	}
	return out, nil
}

// UpdateMe changes the name or the interface language.
func (s *Service) UpdateMe(ctx context.Context, body apigen.MeUpdate) (apigen.Me, error) {
	p, err := authz.Require(ctx)
	if err != nil {
		return apigen.Me{}, err
	}
	var v validate.Errors
	params := userdb.UpdateUserParams{ID: p.UserID}
	if body.FullName != nil {
		name := v.Text("full_name", *body.FullName, 2, 120)
		params.FullName = &name
	}
	if body.Locale != nil {
		if !body.Locale.Valid() {
			v.Add("locale", validate.CodeInvalid, nil)
		}
		l := string(*body.Locale)
		params.Locale = &l
	}
	if err := v.Err(); err != nil {
		return apigen.Me{}, err
	}
	err = s.db.Tx(ctx, func(ctx context.Context) error {
		_, err := s.q(ctx).UpdateUser(ctx, params)
		return err
	})
	if err != nil {
		return apigen.Me{}, fmt.Errorf("user: update: %w", err)
	}
	return s.Me(ctx)
}

// CompleteOnboarding stores the place of study, skills and interests chosen
// in the first-run wizard.
func (s *Service) CompleteOnboarding(ctx context.Context, body apigen.OnboardingRequest) (apigen.Me, error) {
	p, err := authz.Require(ctx)
	if err != nil {
		return apigen.Me{}, err
	}
	var v validate.Errors
	if !body.Locale.Valid() {
		v.Add("locale", validate.CodeInvalid, nil)
	}
	skills := v.Tags("skills", body.Skills, 20, 40)
	interests := v.Tags("interests", body.Interests, 12, 40)
	idea := v.Optional("idea", body.Idea, 500, false)
	if !body.HasIdea {
		idea = nil
	}
	v.IntRange("course", body.Course, 1, 7)
	if body.StudyLevel != nil && !body.StudyLevel.Valid() {
		v.Add("study_level", validate.CodeInvalid, nil)
	}

	err = s.db.Tx(ctx, func(ctx context.Context) error {
		q := s.q(ctx)
		m, err := q.GetMembership(ctx, p.UserID)
		if err != nil {
			return err
		}
		place := org.Placement{FacultyID: body.FacultyId, DepartmentID: body.DepartmentId, GroupID: body.GroupId}
		if body.StudyLevel != nil {
			l := string(*body.StudyLevel)
			place.StudyLevel = &l
		}
		place, err = s.org.ValidatePlacement(ctx, place, &v)
		if err != nil {
			return err
		}
		if slices.Contains(m.Roles, string(authz.RoleStudent)) && place.FacultyID == nil && !v.Has("faculty_id") {
			v.Add("faculty_id", validate.CodeRequired, nil)
		}
		if err := v.Err(); err != nil {
			return err
		}
		locale := string(body.Locale)
		if _, err := q.UpdateUser(ctx, userdb.UpdateUserParams{ID: p.UserID, Locale: &locale}); err != nil {
			return err
		}
		var course *int16
		if body.Course != nil {
			c := int16(*body.Course) //nolint:gosec // validated range 1-7
			course = &c
		}
		now := time.Now()
		_, err = q.UpdateMembershipStudy(ctx, userdb.UpdateMembershipStudyParams{
			FacultyID: place.FacultyID, DepartmentID: place.DepartmentID, GroupID: place.GroupID,
			StudyLevel: place.StudyLevel, Course: course, OnboardedAt: &now, UserID: p.UserID,
		})
		if err != nil {
			return err
		}
		err = q.UpdateProfileOnboarding(ctx, userdb.UpdateProfileOnboardingParams{
			Skills: skills, Interests: interests, HasIdea: body.HasIdea, Idea: idea, UserID: p.UserID,
		})
		if err != nil {
			return err
		}
		return s.audit.Record(ctx, audit.Event{Action: "member.onboarded", TargetType: "user", TargetID: p.UserID.String()})
	})
	if err != nil {
		if _, ok := apperr.As(err); ok {
			return apigen.Me{}, err
		}
		return apigen.Me{}, fmt.Errorf("user: onboarding: %w", err)
	}
	return s.Me(ctx)
}

func telegramHandle(v *validate.Errors, value *string) *string {
	h := v.Optional("links.telegram", value, 64, false)
	if h == nil {
		return nil
	}
	s := *h
	for _, prefix := range []string{"https://", "http://", "t.me/", "telegram.me/", "@"} {
		s = strings.TrimPrefix(s, prefix)
	}
	if !telegramPattern.MatchString(s) {
		v.Add("links.telegram", validate.CodeFormat, nil)
		return h
	}
	return &s
}

// UpdateProfile changes the profile shown to other members and, when public,
// to everyone.
func (s *Service) UpdateProfile(ctx context.Context, body apigen.ProfileUpdate) (apigen.Me, error) {
	p, err := authz.Require(ctx)
	if err != nil {
		return apigen.Me{}, err
	}
	var v validate.Errors
	headline := v.Optional("headline", body.Headline, 140, false)
	bio := v.Optional("bio", body.Bio, 2000, true)
	skills := v.Tags("skills", body.Skills, 20, 40)
	interests := v.Tags("interests", body.Interests, 12, 40)
	links := apigen.ProfileLinks{
		Website:  v.URL("links.website", body.Links.Website, 300),
		Github:   v.URL("links.github", body.Links.Github, 300, "github.com"),
		Linkedin: v.URL("links.linkedin", body.Links.Linkedin, 300, "linkedin.com"),
		Telegram: telegramHandle(&v, body.Links.Telegram),
	}
	slug := strings.ToLower(strings.TrimSpace(body.PublicSlug))
	if !slugAllowed(slug) {
		v.Add("public_slug", validate.CodeFormat, nil)
	}
	rawLinks, err := json.Marshal(links)
	if err != nil {
		return apigen.Me{}, fmt.Errorf("user: links: %w", err)
	}

	err = s.db.Tx(ctx, func(ctx context.Context) error {
		q := s.q(ctx)
		if !v.Has("public_slug") {
			taken, err := q.SlugTaken(ctx, userdb.SlugTakenParams{PublicSlug: slug, UserID: p.UserID})
			if err != nil {
				return err
			}
			if taken {
				v.Add("public_slug", validate.CodeTaken, nil)
			}
		}
		if err := v.Err(); err != nil {
			return err
		}
		_, err := q.UpdateProfile(ctx, userdb.UpdateProfileParams{
			Headline: headline, Bio: bio, Skills: skills, Interests: interests, Links: rawLinks,
			PublicSlug: slug, IsPublic: body.IsPublic, LookingForTeam: body.LookingForTeam, UserID: p.UserID,
		})
		if db.IsUniqueViolation(err, "") {
			return apperr.Validation(apperr.FieldError{Field: "public_slug", Code: validate.CodeTaken})
		}
		return err
	})
	if err != nil {
		if _, ok := apperr.As(err); ok {
			return apigen.Me{}, err
		}
		return apigen.Me{}, fmt.Errorf("user: profile: %w", err)
	}
	return s.Me(ctx)
}

// SetAvatar replaces the profile photo.
func (s *Service) SetAvatar(ctx context.Context, form *multipart.Reader) (apigen.Me, error) {
	p, err := authz.Require(ctx)
	if err != nil {
		return apigen.Me{}, err
	}
	_, part, err := files.ReadForm(form, "file", maxAvatarBytes)
	if err != nil {
		return apigen.Me{}, err
	}
	image, err := files.ProcessAvatar(part.Data)
	if err != nil {
		return apigen.Me{}, err
	}
	f, err := s.files.Store(ctx, files.Upload{
		OwnerID: &p.UserID, Purpose: files.PurposeAvatar, Visibility: files.VisibilityPublic,
		FileName: "avatar.jpg", ContentType: "image/jpeg", Data: image,
	})
	if err != nil {
		return apigen.Me{}, err
	}
	err = s.db.Tx(ctx, func(ctx context.Context) error {
		q := s.q(ctx)
		u, err := q.GetUser(ctx, p.UserID)
		if err != nil {
			return err
		}
		if err := q.SetUserAvatar(ctx, userdb.SetUserAvatarParams{AvatarFileID: &f.ID, ID: p.UserID}); err != nil {
			return err
		}
		if u.AvatarFileID != nil {
			return s.files.Delete(ctx, *u.AvatarFileID)
		}
		return nil
	})
	if err != nil {
		_ = s.db.Tx(context.WithoutCancel(ctx), func(ctx context.Context) error { return s.files.Delete(ctx, f.ID) })
		return apigen.Me{}, fmt.Errorf("user: avatar: %w", err)
	}
	return s.Me(ctx)
}

// DeleteAvatar removes the profile photo.
func (s *Service) DeleteAvatar(ctx context.Context) error {
	p, err := authz.Require(ctx)
	if err != nil {
		return err
	}
	err = s.db.Tx(ctx, func(ctx context.Context) error {
		q := s.q(ctx)
		u, err := q.GetUser(ctx, p.UserID)
		if err != nil {
			return err
		}
		if u.AvatarFileID == nil {
			return nil
		}
		if err := q.SetUserAvatar(ctx, userdb.SetUserAvatarParams{ID: p.UserID}); err != nil {
			return err
		}
		return s.files.Delete(ctx, *u.AvatarFileID)
	})
	if err != nil {
		return fmt.Errorf("user: delete avatar: %w", err)
	}
	return nil
}

// PublicProfile returns a profile its owner made public, if the owner is an
// active member of the current university.
func (s *Service) PublicProfile(ctx context.Context, slug string) (apigen.PublicProfile, error) {
	t, err := tenancy.Require(ctx)
	if err != nil {
		return apigen.PublicProfile{}, err
	}
	var out apigen.PublicProfile
	err = s.db.ReadTx(ctx, func(ctx context.Context) error {
		q := s.q(ctx)
		row, err := q.GetPublicProfile(ctx, strings.ToLower(slug))
		if db.IsNoRows(err) {
			return errProfileNotFound
		}
		if err != nil {
			return err
		}
		m, err := q.GetMembership(ctx, row.UserID)
		if db.IsNoRows(err) || (err == nil && m.Status != StatusActive) {
			return errProfileNotFound
		}
		if err != nil {
			return err
		}
		refs, err := s.org.LoadRefs(ctx, ids(m.FacultyID), nil, nil)
		if err != nil {
			return err
		}
		brief, err := s.tenants.Brief(ctx, t.ID)
		if err != nil {
			return err
		}
		out = apigen.PublicProfile{
			AvatarUrl:      apiconv.FileURL(row.AvatarFileID),
			Bio:            row.Bio,
			FullName:       row.FullName,
			Headline:       row.Headline,
			Interests:      orEmpty(row.Interests),
			Links:          linksAPI(row.Links),
			LookingForTeam: row.LookingForTeam,
			Skills:         orEmpty(row.Skills),
			Tenant:         brief,
		}
		if m.FacultyID != nil {
			if f, ok := refs.Faculties[*m.FacultyID]; ok {
				out.Faculty = &f
			}
		}
		return nil
	})
	if err != nil {
		if _, ok := apperr.As(err); ok {
			return out, err
		}
		return out, fmt.Errorf("user: public profile: %w", err)
	}
	return out, nil
}

type memberCursor struct {
	Name string    `json:"n"`
	ID   uuid.UUID `json:"i"`
}

// Members lists the members of the university for administrators.
func (s *Service) Members(ctx context.Context, params apigen.ListMembersParams) (apigen.MemberPage, error) {
	if _, err := authz.RequireRole(ctx, authz.RoleTenantAdmin, authz.RoleInnovationManager); err != nil {
		return apigen.MemberPage{}, err
	}
	limit := pagination.Limit(params.Limit)
	args := userdb.ListMembersParams{FacultyID: params.FacultyId, RowLimit: int32(limit + 1)} //nolint:gosec // bounded by MaxLimit
	if params.Q != nil {
		if q := strings.TrimSpace(*params.Q); q != "" {
			q = likeEscaper.Replace(q)
			args.Q = &q
		}
	}
	if params.Role != nil {
		if !params.Role.Valid() {
			return apigen.MemberPage{}, apperr.BadRequest("request.invalid_parameter", "unknown role")
		}
		r := string(*params.Role)
		args.Role = &r
	}
	if params.Cursor != nil {
		var c memberCursor
		if err := pagination.Decode(*params.Cursor, &c); err != nil {
			return apigen.MemberPage{}, err
		}
		args.AfterName, args.AfterID = &c.Name, &c.ID
	}

	page := apigen.MemberPage{Items: []apigen.Member{}}
	err := s.db.ReadTx(ctx, func(ctx context.Context) error {
		rows, err := s.q(ctx).ListMembers(ctx, args)
		if err != nil {
			return err
		}
		if len(rows) > limit {
			rows = rows[:limit]
			last := rows[len(rows)-1]
			next := pagination.Encode(memberCursor{Name: last.FullName, ID: last.ID})
			page.NextCursor = &next
		}
		var faculties, groups []uuid.UUID
		for i := range rows {
			faculties = append(faculties, ids(rows[i].FacultyID)...)
			groups = append(groups, ids(rows[i].GroupID)...)
		}
		refs, err := s.org.LoadRefs(ctx, faculties, nil, groups)
		if err != nil {
			return err
		}
		for i := range rows {
			page.Items = append(page.Items, memberAPI(&rows[i], refs))
		}
		return nil
	})
	if err != nil {
		return page, fmt.Errorf("user: members: %w", err)
	}
	return page, nil
}

func memberAPI(r *userdb.ListMembersRow, refs org.Refs) apigen.Member {
	m := apigen.Member{
		Course:       courseAPI(r.Course),
		Email:        openapi_types.Email(r.Email),
		JoinedAt:     r.JoinedAt,
		LastActiveAt: r.LastActiveAt,
		Roles:        rolesAPI(r.Roles),
		Status:       apigen.MembershipStatus(r.Status),
		User:         apigen.MemberBrief{AvatarUrl: apiconv.FileURL(r.AvatarFileID), FullName: r.FullName, Id: r.ID},
	}
	if r.GroupID != nil {
		if g, ok := refs.Groups[*r.GroupID]; ok {
			m.Group = &g
		}
	}
	if r.FacultyID != nil {
		if f, ok := refs.Faculties[*r.FacultyID]; ok {
			m.User.Faculty = &f
		}
	}
	return m
}

// UpdateMember changes the roles or the status of a member.
func (s *Service) UpdateMember(ctx context.Context, userID uuid.UUID, body apigen.MemberUpdate) (apigen.Member, error) {
	actor, err := authz.RequireRole(ctx, authz.RoleTenantAdmin)
	if err != nil {
		return apigen.Member{}, err
	}
	var v validate.Errors
	var roles []string
	if body.Roles != nil {
		for _, r := range *body.Roles {
			if !authz.Role(r).Valid() {
				v.Add("roles", validate.CodeInvalid, nil)
				break
			}
			if !slices.Contains(roles, string(r)) {
				roles = append(roles, string(r))
			}
		}
		if len(roles) == 0 && !v.Has("roles") {
			v.Add("roles", validate.CodeMinItems, map[string]any{"count": 1})
		}
	}
	var status *string
	if body.Status != nil {
		if !body.Status.Valid() {
			v.Add("status", validate.CodeInvalid, nil)
		}
		st := string(*body.Status)
		status = &st
	}
	if err := v.Err(); err != nil {
		return apigen.Member{}, err
	}

	var before, after userdb.Membership
	err = s.db.Tx(ctx, func(ctx context.Context) error {
		q := s.q(ctx)
		var err error
		before, err = q.GetMembership(ctx, userID)
		if db.IsNoRows(err) {
			return errMemberNotFound
		}
		if err != nil {
			return err
		}
		wasAdmin := slices.Contains(before.Roles, string(authz.RoleTenantAdmin)) && before.Status == StatusActive
		losesAdmin := wasAdmin && ((roles != nil && !slices.Contains(roles, string(authz.RoleTenantAdmin))) ||
			(status != nil && *status == StatusSuspended))
		if userID == actor.UserID && losesAdmin {
			return apperr.Conflict("member.cannot_demote_self", "administrators cannot remove their own access")
		}
		if losesAdmin {
			admins, err := q.CountAdmins(ctx)
			if err != nil {
				return err
			}
			if admins <= 1 {
				return apperr.Conflict("member.last_admin", "the university needs at least one administrator")
			}
		}
		after, err = q.UpdateMembershipAccess(ctx, userdb.UpdateMembershipAccessParams{Roles: roles, Status: status, UserID: userID})
		if err != nil {
			return err
		}
		return s.audit.Record(ctx, audit.Event{
			Action: "member.updated", TargetType: "user", TargetID: userID.String(),
			Metadata: map[string]any{
				"roles_before": before.Roles, "roles_after": after.Roles,
				"status_before": before.Status, "status_after": after.Status,
			},
		})
	})
	if err != nil {
		if _, ok := apperr.As(err); ok {
			return apigen.Member{}, err
		}
		return apigen.Member{}, fmt.Errorf("user: update member: %w", err)
	}

	if !slices.Equal(before.Roles, after.Roles) {
		if err := s.revoker.RolesChanged(ctx, after.TenantID, userID); err != nil {
			return apigen.Member{}, err
		}
	}
	if after.Status == StatusSuspended && before.Status != StatusSuspended {
		if err := s.revoker.RevokeAll(ctx, after.TenantID, userID, "suspended"); err != nil {
			return apigen.Member{}, err
		}
	}
	return s.member(ctx, userID)
}

func (s *Service) member(ctx context.Context, userID uuid.UUID) (apigen.Member, error) {
	var out apigen.Member
	err := s.db.ReadTx(ctx, func(ctx context.Context) error {
		q := s.q(ctx)
		u, err := q.GetUser(ctx, userID)
		if err != nil {
			return err
		}
		m, err := q.GetMembership(ctx, userID)
		if err != nil {
			return err
		}
		refs, err := s.org.LoadRefs(ctx, ids(m.FacultyID), nil, ids(m.GroupID))
		if err != nil {
			return err
		}
		out = memberAPI(&userdb.ListMembersRow{
			ID: u.ID, Email: u.Email, FullName: u.FullName, AvatarFileID: u.AvatarFileID, Roles: m.Roles,
			Status: m.Status, FacultyID: m.FacultyID, GroupID: m.GroupID, Course: m.Course,
			JoinedAt: m.JoinedAt, LastActiveAt: m.LastActiveAt,
		}, refs)
		return nil
	})
	if err != nil {
		return out, fmt.Errorf("user: member: %w", err)
	}
	return out, nil
}

// People finds active members of the university by name, for adding to a
// team or mentioning in a comment.
func (s *Service) People(ctx context.Context, params apigen.SearchPeopleParams) ([]apigen.Person, error) {
	if _, err := authz.Require(ctx); err != nil {
		return nil, err
	}
	limit := 20
	if params.Limit != nil {
		limit = min(max(*params.Limit, 1), 50)
	}
	args := userdb.SearchPeopleParams{RowLimit: int32(limit)}
	if params.Q != nil {
		if q := strings.TrimSpace(*params.Q); q != "" {
			q = likeEscaper.Replace(q)
			args.Q = &q
		}
	}
	if params.Role != nil {
		if !params.Role.Valid() {
			return nil, apperr.BadRequest("request.invalid_parameter", "unknown role")
		}
		r := string(*params.Role)
		args.Role = &r
	}
	out := []apigen.Person{}
	err := s.db.ReadTx(ctx, func(ctx context.Context) error {
		rows, err := s.q(ctx).SearchPeople(ctx, args)
		if err != nil {
			return err
		}
		var faculties []uuid.UUID
		for i := range rows {
			faculties = append(faculties, ids(rows[i].FacultyID)...)
		}
		refs, err := s.org.LoadRefs(ctx, faculties, nil, nil)
		if err != nil {
			return err
		}
		for _, r := range rows {
			p := apigen.Person{Id: r.ID, FullName: r.FullName, AvatarUrl: apiconv.FileURL(r.AvatarFileID), Roles: rolesAPI(r.Roles)}
			if r.FacultyID != nil {
				if f, ok := refs.Faculties[*r.FacultyID]; ok {
					p.Faculty = &f
				}
			}
			if r.IsPublic && r.PublicSlug != nil {
				p.Slug = r.PublicSlug
			}
			out = append(out, p)
		}
		return nil
	})
	if err != nil {
		return nil, fmt.Errorf("user: people: %w", err)
	}
	return out, nil
}

// Briefs describes users for lists (authors of requests, replies).
func (s *Service) Briefs(ctx context.Context, userIDs []uuid.UUID) (map[uuid.UUID]apigen.MemberBrief, error) {
	out := map[uuid.UUID]apigen.MemberBrief{}
	if len(userIDs) == 0 {
		return out, nil
	}
	err := s.db.ReadTx(ctx, func(ctx context.Context) error {
		rows, err := s.q(ctx).GetUsersByIDs(ctx, userIDs)
		if err != nil {
			return err
		}
		var faculties []uuid.UUID
		for _, r := range rows {
			faculties = append(faculties, ids(r.FacultyID)...)
		}
		refs, err := s.org.LoadRefs(ctx, faculties, nil, nil)
		if err != nil {
			return err
		}
		for _, r := range rows {
			b := apigen.MemberBrief{AvatarUrl: apiconv.FileURL(r.AvatarFileID), FullName: r.FullName, Id: r.ID}
			if r.FacultyID != nil {
				if f, ok := refs.Faculties[*r.FacultyID]; ok {
					b.Faculty = &f
				}
			}
			out[r.ID] = b
		}
		return nil
	})
	if err != nil {
		return out, fmt.Errorf("user: briefs: %w", err)
	}
	return out, nil
}

// Recipient is where letters to a member go.
type Recipient struct {
	ID       uuid.UUID
	Email    string
	FullName string
	Locale   string
}

// Recipients returns the active members among the users (inside the
// transaction of ctx, if any).
func (s *Service) Recipients(ctx context.Context, userIDs []uuid.UUID) ([]Recipient, error) {
	rows, err := s.q(ctx).ListRecipients(ctx, userIDs)
	if err != nil {
		return nil, fmt.Errorf("user: recipients: %w", err)
	}
	out := make([]Recipient, len(rows))
	for i := range rows {
		out[i] = Recipient{ID: rows[i].ID, Email: rows[i].Email, FullName: rows[i].FullName, Locale: rows[i].Locale}
	}
	return out, nil
}

// CountStudents counts the active students of the university (a public
// figure of the landing page).
func (s *Service) CountStudents(ctx context.Context) (int, error) {
	var n int64
	err := s.db.ReadTx(ctx, func(ctx context.Context) error {
		var err error
		n, err = s.q(ctx).CountStudents(ctx)
		return err
	})
	if err != nil {
		return 0, fmt.Errorf("user: count students: %w", err)
	}
	return int(n), nil
}

// EnsureProfile creates the profile of a user, with an address derived from
// the name (inside the transaction of ctx).
func (s *Service) EnsureProfile(ctx context.Context, userID uuid.UUID, fullName string) error {
	if _, err := s.q(ctx).GetProfile(ctx, userID); err == nil {
		return nil
	} else if !db.IsNoRows(err) {
		return fmt.Errorf("user: profile: %w", err)
	}
	base := Slugify(fullName)
	for attempt := range 6 {
		slug := base
		if attempt > 0 || reservedSlugs[slug] {
			var b [2]byte
			_, _ = rand.Read(b[:])
			slug = base + "-" + hex.EncodeToString(b[:])
		}
		err := s.db.Tx(ctx, func(ctx context.Context) error {
			_, err := s.q(ctx).CreateProfile(ctx, userdb.CreateProfileParams{UserID: userID, PublicSlug: slug})
			return err
		})
		if db.IsUniqueViolation(err, "user_profiles_public_slug_key") {
			continue
		}
		if err != nil {
			return fmt.Errorf("user: create profile: %w", err)
		}
		return nil
	}
	return fmt.Errorf("user: no free profile address for %q", base)
}

// Join makes the user a member of the current university (inside the
// transaction of ctx). An existing membership keeps its roles and status.
func (s *Service) Join(ctx context.Context, userID uuid.UUID, roles []string, status string, consentVersion *string) (userdb.Membership, error) {
	t, err := tenancy.Require(ctx)
	if err != nil {
		return userdb.Membership{}, err
	}
	var consentAt *time.Time
	if consentVersion != nil {
		now := time.Now()
		consentAt = &now
	}
	m, err := s.q(ctx).CreateMembership(ctx, userdb.CreateMembershipParams{
		TenantID: t.ID, UserID: userID, Roles: roles, Status: status,
		ConsentVersion: consentVersion, ConsentAt: consentAt,
	})
	if err != nil {
		return m, fmt.Errorf("user: join: %w", err)
	}
	return m, nil
}

// Study is the place of study of a member.
type Study struct {
	FacultyID    *uuid.UUID
	DepartmentID *uuid.UUID
	GroupID      *uuid.UUID
	StudyLevel   *string
	Course       *int16
}

// SetStudy replaces the place of study of a member (inside the transaction of ctx).
func (s *Service) SetStudy(ctx context.Context, userID uuid.UUID, st Study) error {
	_, err := s.q(ctx).UpdateMembershipStudy(ctx, userdb.UpdateMembershipStudyParams{
		FacultyID: st.FacultyID, DepartmentID: st.DepartmentID, GroupID: st.GroupID,
		StudyLevel: st.StudyLevel, Course: st.Course, UserID: userID,
	})
	if err != nil {
		return fmt.Errorf("user: set study: %w", err)
	}
	return nil
}

// Membership returns the membership of a user in the current university
// (inside the transaction of ctx); ok is false for non-members.
func (s *Service) Membership(ctx context.Context, userID uuid.UUID) (userdb.Membership, bool, error) {
	m, err := s.q(ctx).GetMembership(ctx, userID)
	if db.IsNoRows(err) {
		return m, false, nil
	}
	if err != nil {
		return m, false, fmt.Errorf("user: membership: %w", err)
	}
	return m, true, nil
}

// Activate turns an invitation into an active membership (inside the
// transaction of ctx).
func (s *Service) Activate(ctx context.Context, userID uuid.UUID) error {
	if err := s.q(ctx).ActivateMembership(ctx, userID); err != nil {
		return fmt.Errorf("user: activate: %w", err)
	}
	return nil
}

// Touch records member activity, at most every few minutes.
func (s *Service) Touch(ctx context.Context, userID uuid.UUID) error {
	if err := s.q(ctx).TouchMembership(ctx, userID); err != nil {
		return fmt.Errorf("user: touch: %w", err)
	}
	return nil
}

// Handler serves the profile and member operations of the API.
type Handler struct {
	svc *Service
}

// NewHandler returns the user handler.
func NewHandler(svc *Service) *Handler { return &Handler{svc: svc} }

// GetMe implements apigen.StrictServerInterface.
func (h *Handler) GetMe(ctx context.Context, _ apigen.GetMeRequestObject) (apigen.GetMeResponseObject, error) {
	me, err := h.svc.Me(ctx)
	if err != nil {
		return nil, err
	}
	return apigen.GetMe200JSONResponse(me), nil
}

// UpdateMe implements apigen.StrictServerInterface.
func (h *Handler) UpdateMe(ctx context.Context, req apigen.UpdateMeRequestObject) (apigen.UpdateMeResponseObject, error) {
	me, err := h.svc.UpdateMe(ctx, *req.Body)
	if err != nil {
		return nil, err
	}
	return apigen.UpdateMe200JSONResponse(me), nil
}

// CompleteOnboarding implements apigen.StrictServerInterface.
func (h *Handler) CompleteOnboarding(ctx context.Context, req apigen.CompleteOnboardingRequestObject) (apigen.CompleteOnboardingResponseObject, error) {
	me, err := h.svc.CompleteOnboarding(ctx, *req.Body)
	if err != nil {
		return nil, err
	}
	return apigen.CompleteOnboarding200JSONResponse(me), nil
}

// UpdateProfile implements apigen.StrictServerInterface.
func (h *Handler) UpdateProfile(ctx context.Context, req apigen.UpdateProfileRequestObject) (apigen.UpdateProfileResponseObject, error) {
	me, err := h.svc.UpdateProfile(ctx, *req.Body)
	if err != nil {
		return nil, err
	}
	return apigen.UpdateProfile200JSONResponse(me), nil
}

// UploadAvatar implements apigen.StrictServerInterface.
func (h *Handler) UploadAvatar(ctx context.Context, req apigen.UploadAvatarRequestObject) (apigen.UploadAvatarResponseObject, error) {
	me, err := h.svc.SetAvatar(ctx, req.Body)
	if err != nil {
		return nil, err
	}
	return apigen.UploadAvatar200JSONResponse(me), nil
}

// DeleteAvatar implements apigen.StrictServerInterface.
func (h *Handler) DeleteAvatar(ctx context.Context, _ apigen.DeleteAvatarRequestObject) (apigen.DeleteAvatarResponseObject, error) {
	if err := h.svc.DeleteAvatar(ctx); err != nil {
		return nil, err
	}
	return apigen.DeleteAvatar204Response{}, nil
}

// GetPublicProfile implements apigen.StrictServerInterface.
func (h *Handler) GetPublicProfile(ctx context.Context, req apigen.GetPublicProfileRequestObject) (apigen.GetPublicProfileResponseObject, error) {
	p, err := h.svc.PublicProfile(ctx, req.Slug)
	if err != nil {
		return nil, err
	}
	return apigen.GetPublicProfile200JSONResponse(p), nil
}

// SearchPeople implements apigen.StrictServerInterface.
func (h *Handler) SearchPeople(ctx context.Context, req apigen.SearchPeopleRequestObject) (apigen.SearchPeopleResponseObject, error) {
	people, err := h.svc.People(ctx, req.Params)
	if err != nil {
		return nil, err
	}
	return apigen.SearchPeople200JSONResponse{Items: people}, nil
}

// ListMembers implements apigen.StrictServerInterface.
func (h *Handler) ListMembers(ctx context.Context, req apigen.ListMembersRequestObject) (apigen.ListMembersResponseObject, error) {
	page, err := h.svc.Members(ctx, req.Params)
	if err != nil {
		return nil, err
	}
	return apigen.ListMembers200JSONResponse(page), nil
}

// UpdateMember implements apigen.StrictServerInterface.
func (h *Handler) UpdateMember(ctx context.Context, req apigen.UpdateMemberRequestObject) (apigen.UpdateMemberResponseObject, error) {
	m, err := h.svc.UpdateMember(ctx, req.UserId, *req.Body)
	if err != nil {
		return nil, err
	}
	return apigen.UpdateMember200JSONResponse(m), nil
}

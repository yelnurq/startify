// Package project holds student projects and their teams: who may see,
// edit, approve and manage a project. The incubator module stores the
// answers of the steps and reports progress back.
package project

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"strconv"
	"strings"
	"time"

	"github.com/google/uuid"

	"github.com/yelnurq/startify/backend/internal/apigen"
	"github.com/yelnurq/startify/backend/internal/audit"
	"github.com/yelnurq/startify/backend/internal/org"
	"github.com/yelnurq/startify/backend/internal/platform/apperr"
	"github.com/yelnurq/startify/backend/internal/platform/authz"
	"github.com/yelnurq/startify/backend/internal/platform/db"
	"github.com/yelnurq/startify/backend/internal/platform/pagination"
	"github.com/yelnurq/startify/backend/internal/platform/tenancy"
	"github.com/yelnurq/startify/backend/internal/platform/validate"
	"github.com/yelnurq/startify/backend/internal/project/projectdb"
	"github.com/yelnurq/startify/backend/internal/tenant"
	"github.com/yelnurq/startify/backend/internal/user"
)

// Roles in a project; "manager" is an innovation manager, administrator or
// leadership member who sees every project of the university.
const (
	RoleOwner      = "owner"
	RoleMember     = "member"
	RoleSupervisor = "supervisor"
	RoleManager    = "manager"
)

// MaxTeam bounds the number of people in a project.
const MaxTeam = 12

var (
	errNotFound     = apperr.NotFound("project.not_found", "the project does not exist")
	errArchived     = apperr.Conflict("project.archived", "the project is archived")
	errForbidden    = apperr.Forbidden("rbac.forbidden", "not allowed for your role in the project")
	errIPPolicy     = apperr.New(apperr.KindValidation, "project.ip_policy_required", "accept the intellectual property policy")
	errNotMember    = apperr.NotFound("project.member_not_found", "the person is not in the project")
	errOwnerLeaving = apperr.Conflict("project.owner_cannot_leave", "hand the project over before leaving")
)

// managers see every project of the university; moderators also manage them.
var (
	managers   = []authz.Role{authz.RoleInnovationManager, authz.RoleTenantAdmin, authz.RoleLeadership}
	moderators = []authz.Role{authz.RoleInnovationManager, authz.RoleTenantAdmin}
)

// Access is how the signed-in member relates to a project.
type Access struct {
	Project   projectdb.Project
	Role      string
	Principal authz.Principal
}

// Team reports whether the member is the owner or a member of the team.
func (a Access) Team() bool { return a.Role == RoleOwner || a.Role == RoleMember }

// Archived reports whether the project is read-only.
func (a Access) Archived() bool { return a.Project.ArchivedAt != nil }

// CanEdit reports whether the member may change the project's answers.
func (a Access) CanEdit() bool { return a.Team() && !a.Archived() }

// CanApprove reports whether the member may approve steps.
func (a Access) CanApprove() bool {
	return !a.Archived() && (a.Role == RoleSupervisor || a.Principal.Has(moderators...))
}

// CanModerate reports whether the member manages projects of the university.
func (a Access) CanModerate() bool { return a.Principal.Has(moderators...) }

// RequireEdit fails unless the member may edit.
func (a Access) RequireEdit() error {
	switch {
	case a.Archived():
		return errArchived
	case !a.Team():
		return errForbidden
	}
	return nil
}

// ProgressSource computes the progress of projects (the incubator).
type ProgressSource interface {
	Progress(ctx context.Context, projectID uuid.UUID) (apigen.ProjectProgress, error)
	TotalSteps() int
	FirstStep() string
}

// Service manages projects and teams.
type Service struct {
	db       *db.DB
	users    *user.Service
	org      *org.Service
	tenants  *tenant.Service
	audit    *audit.Logger
	progress ProgressSource
}

// NewService returns the project service; UseProgress must be called before use.
func NewService(database *db.DB, users *user.Service, orgs *org.Service, tenants *tenant.Service, auditLog *audit.Logger) *Service {
	return &Service{db: database, users: users, org: orgs, tenants: tenants, audit: auditLog}
}

// UseProgress connects the incubator.
func (s *Service) UseProgress(p ProgressSource) { s.progress = p }

func (s *Service) q(ctx context.Context) *projectdb.Queries { return projectdb.New(s.db.Conn(ctx)) }

// Access loads a project the member may see. Projects the member may not see
// do not exist for them.
func (s *Service) Access(ctx context.Context, projectID uuid.UUID) (Access, error) {
	p, err := authz.Require(ctx)
	if err != nil {
		return Access{}, err
	}
	a := Access{Principal: p}
	err = s.db.ReadTx(ctx, func(ctx context.Context) error {
		q := s.q(ctx)
		var err error
		if a.Project, err = q.GetProject(ctx, projectID); err != nil {
			return err
		}
		m, err := q.GetMember(ctx, projectdb.GetMemberParams{ProjectID: projectID, UserID: p.UserID})
		switch {
		case err == nil:
			a.Role = m.Role
		case db.IsNoRows(err) && p.Has(managers...):
			a.Role = RoleManager
		default:
			return err
		}
		return nil
	})
	if db.IsNoRows(err) {
		return Access{}, errNotFound
	}
	if err != nil {
		return Access{}, fmt.Errorf("project: access: %w", err)
	}
	return a, nil
}

// ETag versions the editable card of a project; progress changes do not
// touch it.
func ETag(p projectdb.Project) string {
	h := sha256.Sum256([]byte(strings.Join([]string{p.Title, p.Tagline, p.Industry, strconv.FormatBool(p.HideSensitive)}, "\x00")))
	return `"` + hex.EncodeToString(h[:8]) + `"`
}

// Create starts a project with the signed-in member as the owner.
func (s *Service) Create(ctx context.Context, body apigen.ProjectCreate) (apigen.Project, error) {
	p, err := authz.Require(ctx)
	if err != nil {
		return apigen.Project{}, err
	}
	if _, err := tenancy.Require(ctx); err != nil {
		return apigen.Project{}, err
	}
	var v validate.Errors
	title := v.Text("title", body.Title, 2, 120)
	tagline := ""
	if body.Tagline != nil {
		tagline = v.Text("tagline", *body.Tagline, 0, 160)
	}
	if !body.Industry.Valid() {
		v.Add("industry", validate.CodeInvalid, nil)
	}
	if err := v.Err(); err != nil {
		return apigen.Project{}, err
	}
	settings, err := s.tenants.CurrentSettings(ctx)
	if err != nil {
		return apigen.Project{}, err
	}
	var accepted *time.Time
	if settings.IPPolicyVersion != "" {
		if body.IpPolicyVersion == nil || *body.IpPolicyVersion != settings.IPPolicyVersion {
			return apigen.Project{}, errIPPolicy.WithParams(map[string]any{"version": settings.IPPolicyVersion})
		}
		now := time.Now()
		accepted = &now
	}
	var id uuid.UUID
	err = s.db.Tx(ctx, func(ctx context.Context) error {
		m, _, err := s.users.Membership(ctx, p.UserID)
		if err != nil {
			return err
		}
		row, err := s.q(ctx).CreateProject(ctx, projectdb.CreateProjectParams{
			Title: title, Tagline: tagline, Industry: string(body.Industry), FacultyID: m.FacultyID,
			CurrentStep: s.progress.FirstStep(), IpPolicyVersion: settings.IPPolicyVersion, IpAcceptedAt: accepted,
			CreatedBy: &p.UserID,
		})
		if err != nil {
			return err
		}
		id = row.ID
		if _, err := s.q(ctx).AddMember(ctx, projectdb.AddMemberParams{ProjectID: id, UserID: p.UserID, Role: RoleOwner}); err != nil {
			return err
		}
		return s.audit.Record(ctx, audit.Event{Action: "project.created", TargetType: "project", TargetID: id.String()})
	})
	if err != nil {
		return apigen.Project{}, fmt.Errorf("project: create: %w", err)
	}
	return s.Get(ctx, id)
}

// Get returns a project with its team and progress.
func (s *Service) Get(ctx context.Context, projectID uuid.UUID) (apigen.Project, error) {
	a, err := s.Access(ctx, projectID)
	if err != nil {
		return apigen.Project{}, err
	}
	return s.view(ctx, a)
}

func (s *Service) view(ctx context.Context, a Access) (apigen.Project, error) {
	members, err := s.Members(ctx, a.Project.ID)
	if err != nil {
		return apigen.Project{}, err
	}
	progress, err := s.progress.Progress(ctx, a.Project.ID)
	if err != nil {
		return apigen.Project{}, err
	}
	out := apigen.Project{
		Id: a.Project.ID, Title: a.Project.Title, Tagline: a.Project.Tagline, Industry: apigen.Industry(a.Project.Industry),
		Visibility: apigen.ProjectVisibility(a.Project.Visibility), HideSensitive: a.Project.HideSensitive,
		Members: members, Access: apigen.ProjectAccess(a.Role), Archived: a.Archived(), Progress: progress,
		CreatedAt: a.Project.CreatedAt, UpdatedAt: a.Project.UpdatedAt,
	}
	if a.Project.FacultyID != nil {
		err := s.db.ReadTx(ctx, func(ctx context.Context) error {
			refs, err := s.org.LoadRefs(ctx, []uuid.UUID{*a.Project.FacultyID}, nil, nil)
			if f, ok := refs.Faculties[*a.Project.FacultyID]; ok {
				out.Faculty = &f
			}
			return err
		})
		if err != nil {
			return out, fmt.Errorf("project: faculty: %w", err)
		}
	}
	return out, nil
}

// Members lists the people of a project: owner, members, supervisors.
func (s *Service) Members(ctx context.Context, projectID uuid.UUID) ([]apigen.ProjectMember, error) {
	var rows []projectdb.ProjectMember
	err := s.db.ReadTx(ctx, func(ctx context.Context) error {
		var err error
		rows, err = s.q(ctx).ListMembers(ctx, []uuid.UUID{projectID})
		return err
	})
	if err != nil {
		return nil, fmt.Errorf("project: members: %w", err)
	}
	ids := make([]uuid.UUID, len(rows))
	for i := range rows {
		ids[i] = rows[i].UserID
	}
	briefs, err := s.users.Briefs(ctx, ids)
	if err != nil {
		return nil, err
	}
	out := make([]apigen.ProjectMember, len(rows))
	for i := range rows {
		m := &rows[i]
		out[i] = apigen.ProjectMember{User: briefs[m.UserID], Role: apigen.ProjectRole(m.Role), Title: m.Title, JoinedAt: m.CreatedAt}
	}
	return out, nil
}

// MemberIDs returns the user ids of everyone in a project by role.
func (s *Service) MemberIDs(ctx context.Context, projectID uuid.UUID) (map[uuid.UUID]string, error) {
	out := map[uuid.UUID]string{}
	err := s.db.ReadTx(ctx, func(ctx context.Context) error {
		rows, err := s.q(ctx).ListMembers(ctx, []uuid.UUID{projectID})
		for i := range rows {
			out[rows[i].UserID] = rows[i].Role
		}
		return err
	})
	if err != nil {
		return nil, fmt.Errorf("project: member ids: %w", err)
	}
	return out, nil
}

// SetProgress stores the denormalised progress (called by the incubator in
// its transaction).
func (s *Service) SetProgress(ctx context.Context, projectID uuid.UUID, completed int, current string) error {
	return s.q(ctx).SetProgress(ctx, projectdb.SetProgressParams{
		ID: projectID, CompletedSteps: int32(completed), CurrentStep: current, //nolint:gosec // at most the number of steps
	})
}

// Update edits the card of a project.
func (s *Service) Update(ctx context.Context, projectID uuid.UUID, body apigen.ProjectUpdate, ifMatch *string) (apigen.Project, error) {
	a, err := s.Access(ctx, projectID)
	if err != nil {
		return apigen.Project{}, err
	}
	if err := a.RequireEdit(); err != nil {
		return apigen.Project{}, err
	}
	var v validate.Errors
	args := projectdb.UpdateProjectParams{ID: projectID, HideSensitive: body.HideSensitive}
	if body.Title != nil {
		t := v.Text("title", *body.Title, 2, 120)
		args.Title = &t
	}
	if body.Tagline != nil {
		t := v.Text("tagline", *body.Tagline, 0, 160)
		args.Tagline = &t
	}
	if body.Industry != nil {
		if !body.Industry.Valid() {
			v.Add("industry", validate.CodeInvalid, nil)
		}
		i := string(*body.Industry)
		args.Industry = &i
	}
	if err := v.Err(); err != nil {
		return apigen.Project{}, err
	}
	err = s.db.Tx(ctx, func(ctx context.Context) error {
		q := s.q(ctx)
		current, err := q.LockProject(ctx, projectID)
		if err != nil {
			return err
		}
		if ifMatch != nil && strings.TrimPrefix(*ifMatch, "W/") != ETag(current) {
			return apperr.PreconditionFailed("precondition.failed", "the project was changed since it was loaded")
		}
		if a.Project, err = q.UpdateProject(ctx, args); err != nil {
			return err
		}
		return s.audit.Record(ctx, audit.Event{Action: "project.updated", TargetType: "project", TargetID: projectID.String()})
	})
	if err != nil {
		return apigen.Project{}, fmt.Errorf("project: update: %w", err)
	}
	return s.view(ctx, a)
}

// Archive makes a project read-only.
func (s *Service) Archive(ctx context.Context, projectID uuid.UUID) error {
	a, err := s.Access(ctx, projectID)
	if err != nil {
		return err
	}
	if a.Role != RoleOwner && !a.CanModerate() {
		return errForbidden
	}
	err = s.db.Tx(ctx, func(ctx context.Context) error {
		if err := s.q(ctx).ArchiveProject(ctx, projectID); err != nil {
			return err
		}
		return s.audit.Record(ctx, audit.Event{Action: "project.archived", TargetType: "project", TargetID: projectID.String()})
	})
	if err != nil {
		return fmt.Errorf("project: archive: %w", err)
	}
	return nil
}

type listCursor struct {
	Updated time.Time `json:"u"`
	ID      uuid.UUID `json:"i"`
}

// List returns the member's projects, or every project for managers.
func (s *Service) List(ctx context.Context, params apigen.ListProjectsParams) (apigen.ProjectPage, error) {
	p, err := authz.Require(ctx)
	if err != nil {
		return apigen.ProjectPage{}, err
	}
	all := params.Scope != nil && *params.Scope == apigen.ListProjectsParamsScopeAll
	if all && !p.Has(managers...) {
		return apigen.ProjectPage{}, errForbidden
	}
	limit := pagination.Limit(params.Limit)
	var q *string
	if params.Q != nil {
		if t := strings.TrimSpace(*params.Q); t != "" {
			t = strings.NewReplacer(`\`, `\\`, `%`, `\%`, `_`, `\_`).Replace(t)
			q = &t
		}
	}
	var before *listCursor
	if params.Cursor != nil {
		before = &listCursor{}
		if err := pagination.Decode(*params.Cursor, before); err != nil {
			return apigen.ProjectPage{}, err
		}
	}
	page := apigen.ProjectPage{Items: []apigen.ProjectSummary{}}
	var rows []projectdb.Project
	var members []projectdb.ProjectMember
	err = s.db.ReadTx(ctx, func(ctx context.Context) error {
		var bu *time.Time
		var bi *uuid.UUID
		if before != nil {
			bu, bi = &before.Updated, &before.ID
		}
		var err error
		if all {
			rows, err = s.q(ctx).ListAllProjects(ctx, projectdb.ListAllProjectsParams{
				Q: q, BeforeUpdated: bu, BeforeID: bi, RowLimit: int32(limit + 1), //nolint:gosec // bounded by MaxLimit
			})
		} else {
			rows, err = s.q(ctx).ListMyProjects(ctx, projectdb.ListMyProjectsParams{
				UserID: p.UserID, Q: q, BeforeUpdated: bu, BeforeID: bi, RowLimit: int32(limit + 1), //nolint:gosec // bounded by MaxLimit
			})
		}
		if err != nil {
			return err
		}
		ids := make([]uuid.UUID, len(rows))
		for i := range rows {
			ids[i] = rows[i].ID
		}
		members, err = s.q(ctx).ListMembers(ctx, ids)
		return err
	})
	if err != nil {
		return page, fmt.Errorf("project: list: %w", err)
	}
	if len(rows) > limit {
		rows = rows[:limit]
		last := rows[len(rows)-1]
		next := pagination.Encode(listCursor{Updated: last.UpdatedAt, ID: last.ID})
		page.NextCursor = &next
	}
	var people []uuid.UUID
	byProject := map[uuid.UUID][]projectdb.ProjectMember{}
	for i := range members {
		byProject[members[i].ProjectID] = append(byProject[members[i].ProjectID], members[i])
		people = append(people, members[i].UserID)
	}
	briefs, err := s.users.Briefs(ctx, people)
	if err != nil {
		return page, err
	}
	for i := range rows {
		r := &rows[i]
		access := RoleManager
		team := make([]apigen.MemberBrief, 0, len(byProject[r.ID]))
		for j := range byProject[r.ID] {
			m := &byProject[r.ID][j]
			if m.UserID == p.UserID {
				access = m.Role
			}
			if m.Role != RoleSupervisor {
				team = append(team, briefs[m.UserID])
			}
		}
		page.Items = append(page.Items, apigen.ProjectSummary{
			Id: r.ID, Title: r.Title, Tagline: r.Tagline, Industry: apigen.Industry(r.Industry), Team: team,
			Access: apigen.ProjectAccess(access), Archived: r.ArchivedAt != nil, CompletedSteps: int(r.CompletedSteps),
			TotalSteps: s.progress.TotalSteps(), CurrentStep: r.CurrentStep, UpdatedAt: r.UpdatedAt,
		})
	}
	return page, nil
}

// AddMember adds a member of the university to the team or as supervisor.
func (s *Service) AddMember(ctx context.Context, projectID uuid.UUID, body apigen.ProjectMemberAdd) (apigen.ProjectMember, error) {
	a, err := s.Access(ctx, projectID)
	if err != nil {
		return apigen.ProjectMember{}, err
	}
	switch {
	case a.Archived():
		return apigen.ProjectMember{}, errArchived
	case body.Role == apigen.ProjectMemberAddRoleSupervisor && a.Role != RoleOwner && !a.CanModerate():
		return apigen.ProjectMember{}, errForbidden
	case body.Role == apigen.ProjectMemberAddRoleMember && a.Role != RoleOwner:
		return apigen.ProjectMember{}, errForbidden
	case body.Role != apigen.ProjectMemberAddRoleSupervisor && body.Role != apigen.ProjectMemberAddRoleMember:
		return apigen.ProjectMember{}, apperr.Validation(apperr.FieldError{Field: "role", Code: validate.CodeInvalid})
	}
	var v validate.Errors
	title := ""
	if body.Title != nil {
		title = v.Text("title", *body.Title, 0, 60)
	}
	if err := v.Err(); err != nil {
		return apigen.ProjectMember{}, err
	}
	err = s.db.Tx(ctx, func(ctx context.Context) error {
		m, ok, err := s.users.Membership(ctx, body.UserId)
		if err != nil {
			return err
		}
		if !ok || m.Status != "active" {
			return apperr.Validation(apperr.FieldError{Field: "user_id", Code: validate.CodeNotFound})
		}
		if body.Role == apigen.ProjectMemberAddRoleSupervisor && !hasRole(m.Roles, authz.RoleSupervisor) {
			return apperr.New(apperr.KindValidation, "project.supervisor_role_required", "the person is not a supervisor")
		}
		q := s.q(ctx)
		team, err := q.ListMembers(ctx, []uuid.UUID{projectID})
		if err != nil {
			return err
		}
		if len(team) >= MaxTeam {
			return apperr.Conflict("project.team_full", "the team is full").WithParams(map[string]any{"count": MaxTeam})
		}
		if _, err := q.AddMember(ctx, projectdb.AddMemberParams{ProjectID: projectID, UserID: body.UserId, Role: string(body.Role), Title: title}); err != nil {
			if db.IsUniqueViolation(err, "") {
				return apperr.Conflict("project.already_member", "the person is already in the project")
			}
			return err
		}
		return s.audit.Record(ctx, audit.Event{
			Action: "project.member_added", TargetType: "project", TargetID: projectID.String(),
			Metadata: map[string]any{"user_id": body.UserId, "role": body.Role},
		})
	})
	if err != nil {
		return apigen.ProjectMember{}, fmt.Errorf("project: add member: %w", err)
	}
	return s.member(ctx, projectID, body.UserId)
}

func hasRole(roles []string, r authz.Role) bool {
	for _, x := range roles {
		if x == string(r) {
			return true
		}
	}
	return false
}

func (s *Service) member(ctx context.Context, projectID, userID uuid.UUID) (apigen.ProjectMember, error) {
	members, err := s.Members(ctx, projectID)
	if err != nil {
		return apigen.ProjectMember{}, err
	}
	for _, m := range members {
		if m.User.Id == userID {
			return m, nil
		}
	}
	return apigen.ProjectMember{}, errNotMember
}

// UpdateMember changes a member's title or hands the project over.
func (s *Service) UpdateMember(ctx context.Context, projectID, userID uuid.UUID, body apigen.ProjectMemberUpdate) (apigen.ProjectMember, error) {
	a, err := s.Access(ctx, projectID)
	if err != nil {
		return apigen.ProjectMember{}, err
	}
	if a.Archived() {
		return apigen.ProjectMember{}, errArchived
	}
	self := userID == a.Principal.UserID
	if body.Title != nil && !self && a.Role != RoleOwner {
		return apigen.ProjectMember{}, errForbidden
	}
	if body.Role != nil && (a.Role != RoleOwner || self) {
		return apigen.ProjectMember{}, errForbidden
	}
	var v validate.Errors
	var title string
	if body.Title != nil {
		title = v.Text("title", *body.Title, 0, 60)
	}
	if err := v.Err(); err != nil {
		return apigen.ProjectMember{}, err
	}
	err = s.db.Tx(ctx, func(ctx context.Context) error {
		q := s.q(ctx)
		target, err := q.GetMember(ctx, projectdb.GetMemberParams{ProjectID: projectID, UserID: userID})
		if db.IsNoRows(err) {
			return errNotMember
		}
		if err != nil {
			return err
		}
		if body.Title != nil {
			if _, err := q.UpdateMemberTitle(ctx, projectdb.UpdateMemberTitleParams{ProjectID: projectID, UserID: userID, Title: title}); err != nil {
				return err
			}
		}
		if body.Role != nil {
			if target.Role != RoleMember {
				return apperr.Validation(apperr.FieldError{Field: "role", Code: validate.CodeInvalid})
			}
			// Demote first: a project has exactly one owner.
			if _, err := q.SetMemberRole(ctx, projectdb.SetMemberRoleParams{ProjectID: projectID, UserID: a.Principal.UserID, Role: RoleMember}); err != nil {
				return err
			}
			if _, err := q.SetMemberRole(ctx, projectdb.SetMemberRoleParams{ProjectID: projectID, UserID: userID, Role: RoleOwner}); err != nil {
				return err
			}
			return s.audit.Record(ctx, audit.Event{
				Action: "project.owner_changed", TargetType: "project", TargetID: projectID.String(),
				Metadata: map[string]any{"owner_id": userID},
			})
		}
		return nil
	})
	if err != nil {
		return apigen.ProjectMember{}, fmt.Errorf("project: update member: %w", err)
	}
	return s.member(ctx, projectID, userID)
}

// RemoveMember removes someone from the project, or lets a member leave.
func (s *Service) RemoveMember(ctx context.Context, projectID, userID uuid.UUID) error {
	a, err := s.Access(ctx, projectID)
	if err != nil {
		return err
	}
	self := userID == a.Principal.UserID
	if !self && a.Role != RoleOwner && !a.CanModerate() {
		return errForbidden
	}
	err = s.db.Tx(ctx, func(ctx context.Context) error {
		q := s.q(ctx)
		target, err := q.GetMember(ctx, projectdb.GetMemberParams{ProjectID: projectID, UserID: userID})
		if db.IsNoRows(err) {
			return errNotMember
		}
		if err != nil {
			return err
		}
		if target.Role == RoleOwner {
			return errOwnerLeaving
		}
		if err := q.RemoveMember(ctx, projectdb.RemoveMemberParams{ProjectID: projectID, UserID: userID}); err != nil {
			return err
		}
		return s.audit.Record(ctx, audit.Event{
			Action: "project.member_removed", TargetType: "project", TargetID: projectID.String(),
			Metadata: map[string]any{"user_id": userID},
		})
	})
	if err != nil {
		return fmt.Errorf("project: remove member: %w", err)
	}
	return nil
}

// Handler serves the project operations of the API.
type Handler struct {
	svc *Service
}

// NewHandler returns the project handler.
func NewHandler(svc *Service) *Handler { return &Handler{svc: svc} }

// ListProjects implements apigen.StrictServerInterface.
func (h *Handler) ListProjects(ctx context.Context, req apigen.ListProjectsRequestObject) (apigen.ListProjectsResponseObject, error) {
	page, err := h.svc.List(ctx, req.Params)
	if err != nil {
		return nil, err
	}
	return apigen.ListProjects200JSONResponse(page), nil
}

// CreateProject implements apigen.StrictServerInterface.
func (h *Handler) CreateProject(ctx context.Context, req apigen.CreateProjectRequestObject) (apigen.CreateProjectResponseObject, error) {
	p, err := h.svc.Create(ctx, *req.Body)
	if err != nil {
		return nil, err
	}
	tag, err := h.etag(ctx, p.Id)
	if err != nil {
		return nil, err
	}
	return apigen.CreateProject201JSONResponse{Body: p, Headers: apigen.CreateProject201ResponseHeaders{ETag: &tag}}, nil
}

func (h *Handler) etag(ctx context.Context, id uuid.UUID) (string, error) {
	a, err := h.svc.Access(ctx, id)
	if err != nil {
		return "", err
	}
	return ETag(a.Project), nil
}

// GetProject implements apigen.StrictServerInterface.
func (h *Handler) GetProject(ctx context.Context, req apigen.GetProjectRequestObject) (apigen.GetProjectResponseObject, error) {
	a, err := h.svc.Access(ctx, req.ProjectId)
	if err != nil {
		return nil, err
	}
	p, err := h.svc.view(ctx, a)
	if err != nil {
		return nil, err
	}
	tag := ETag(a.Project)
	return apigen.GetProject200JSONResponse{Body: p, Headers: apigen.GetProject200ResponseHeaders{ETag: &tag}}, nil
}

// UpdateProject implements apigen.StrictServerInterface.
func (h *Handler) UpdateProject(ctx context.Context, req apigen.UpdateProjectRequestObject) (apigen.UpdateProjectResponseObject, error) {
	p, err := h.svc.Update(ctx, req.ProjectId, *req.Body, req.Params.IfMatch)
	if err != nil {
		return nil, err
	}
	tag, err := h.etag(ctx, p.Id)
	if err != nil {
		return nil, err
	}
	return apigen.UpdateProject200JSONResponse{Body: p, Headers: apigen.UpdateProject200ResponseHeaders{ETag: &tag}}, nil
}

// ArchiveProject implements apigen.StrictServerInterface.
func (h *Handler) ArchiveProject(ctx context.Context, req apigen.ArchiveProjectRequestObject) (apigen.ArchiveProjectResponseObject, error) {
	if err := h.svc.Archive(ctx, req.ProjectId); err != nil {
		return nil, err
	}
	return apigen.ArchiveProject204Response{}, nil
}

// AddProjectMember implements apigen.StrictServerInterface.
func (h *Handler) AddProjectMember(ctx context.Context, req apigen.AddProjectMemberRequestObject) (apigen.AddProjectMemberResponseObject, error) {
	m, err := h.svc.AddMember(ctx, req.ProjectId, *req.Body)
	if err != nil {
		return nil, err
	}
	return apigen.AddProjectMember201JSONResponse(m), nil
}

// UpdateProjectMember implements apigen.StrictServerInterface.
func (h *Handler) UpdateProjectMember(ctx context.Context, req apigen.UpdateProjectMemberRequestObject) (apigen.UpdateProjectMemberResponseObject, error) {
	m, err := h.svc.UpdateMember(ctx, req.ProjectId, req.UserId, *req.Body)
	if err != nil {
		return nil, err
	}
	return apigen.UpdateProjectMember200JSONResponse(m), nil
}

// RemoveProjectMember implements apigen.StrictServerInterface.
func (h *Handler) RemoveProjectMember(ctx context.Context, req apigen.RemoveProjectMemberRequestObject) (apigen.RemoveProjectMemberResponseObject, error) {
	if err := h.svc.RemoveMember(ctx, req.ProjectId, req.UserId); err != nil {
		return nil, err
	}
	return apigen.RemoveProjectMember204Response{}, nil
}

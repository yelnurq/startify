// Package team is the teammates board: students publish what kind of people
// their project needs, others reply, and the author is notified by e-mail.
package team

import (
	"context"
	"fmt"
	"slices"
	"strconv"
	"strings"
	"time"

	"github.com/google/uuid"

	"github.com/yelnurq/startify/backend/internal/apigen"
	"github.com/yelnurq/startify/backend/internal/audit"
	"github.com/yelnurq/startify/backend/internal/mail"
	"github.com/yelnurq/startify/backend/internal/platform/apperr"
	"github.com/yelnurq/startify/backend/internal/platform/authz"
	"github.com/yelnurq/startify/backend/internal/platform/db"
	"github.com/yelnurq/startify/backend/internal/platform/pagination"
	"github.com/yelnurq/startify/backend/internal/platform/ratelimit"
	"github.com/yelnurq/startify/backend/internal/platform/tenancy"
	"github.com/yelnurq/startify/backend/internal/platform/validate"
	"github.com/yelnurq/startify/backend/internal/team/teamdb"
	"github.com/yelnurq/startify/backend/internal/user"
	"github.com/yelnurq/startify/backend/internal/user/userdb"
)

const (
	statusOpen   = "open"
	statusClosed = "closed"
)

var (
	errNotFound  = apperr.NotFound("team.not_found", "the request does not exist")
	errForbidden = apperr.Forbidden("team.forbidden", "only the author can change the request")
	likeEscaper  = strings.NewReplacer(`\`, `\\`, `%`, `\%`, `_`, `\_`)

	limitCreate  = ratelimit.PerHour(10)
	limitRespond = ratelimit.PerHour(30)
)

// Service manages requests for teammates.
type Service struct {
	db      *db.DB
	users   *user.Service
	mailer  *mail.Mailer
	limiter *ratelimit.Limiter
	audit   *audit.Logger
}

// NewService returns the team service.
func NewService(database *db.DB, users *user.Service, mailer *mail.Mailer, limiter *ratelimit.Limiter, auditLog *audit.Logger) *Service {
	return &Service{db: database, users: users, mailer: mailer, limiter: limiter, audit: auditLog}
}

func (s *Service) q(ctx context.Context) *teamdb.Queries { return teamdb.New(s.db.Conn(ctx)) }

func canModerate(p authz.Principal) bool {
	return p.SuperAdmin || p.Has(authz.RoleTenantAdmin, authz.RoleInnovationManager)
}

// view converts requests, loading reply counts, the viewer's replies and authors.
func (s *Service) view(ctx context.Context, viewer uuid.UUID, rows []teamdb.TeamRequest) ([]apigen.TeamRequest, error) {
	out := make([]apigen.TeamRequest, 0, len(rows))
	if len(rows) == 0 {
		return out, nil
	}
	ids := make([]uuid.UUID, len(rows))
	authors := make([]uuid.UUID, 0, len(rows))
	for i := range rows {
		ids[i] = rows[i].ID
		if !slices.Contains(authors, rows[i].AuthorID) {
			authors = append(authors, rows[i].AuthorID)
		}
	}
	q := s.q(ctx)
	counts, err := q.CountTeamResponses(ctx, ids)
	if err != nil {
		return nil, err
	}
	byRequest := map[uuid.UUID]int{}
	for _, c := range counts {
		byRequest[c.RequestID] = int(c.Responses)
	}
	responded, err := q.RespondedRequests(ctx, teamdb.RespondedRequestsParams{Ids: ids, AuthorID: viewer})
	if err != nil {
		return nil, err
	}
	briefs, err := s.users.Briefs(ctx, authors)
	if err != nil {
		return nil, err
	}
	for i := range rows {
		r := &rows[i]
		roles := make([]apigen.TeamRole, len(r.RolesNeeded))
		for i, role := range r.RolesNeeded {
			roles[i] = apigen.TeamRole(role)
		}
		skills := r.Skills
		if skills == nil {
			skills = []string{}
		}
		out = append(out, apigen.TeamRequest{
			Author:         briefs[r.AuthorID],
			CreatedAt:      r.CreatedAt,
			Description:    r.Description,
			HasResponded:   slices.Contains(responded, r.ID),
			Id:             r.ID,
			IsMine:         r.AuthorID == viewer,
			ResponsesCount: byRequest[r.ID],
			RolesNeeded:    roles,
			Skills:         skills,
			Status:         apigen.TeamRequestStatus(r.Status),
			Title:          r.Title,
			UpdatedAt:      r.UpdatedAt,
		})
	}
	return out, nil
}

// List returns open requests, or the viewer's own with mine=true.
func (s *Service) List(ctx context.Context, params apigen.ListTeamRequestsParams) (apigen.TeamRequestPage, error) {
	p, err := authz.Require(ctx)
	if err != nil {
		return apigen.TeamRequestPage{}, err
	}
	limit := pagination.Limit(params.Limit)
	args := teamdb.ListTeamRequestsParams{ViewerID: p.UserID, RowLimit: int32(limit + 1)} //nolint:gosec // bounded by MaxLimit
	if params.Mine != nil {
		args.Mine = *params.Mine
	}
	if params.Role != nil {
		if !params.Role.Valid() {
			return apigen.TeamRequestPage{}, apperr.BadRequest("request.invalid_parameter", "unknown role")
		}
		r := string(*params.Role)
		args.Role = &r
	}
	if params.Skill != nil && strings.TrimSpace(*params.Skill) != "" {
		v := strings.TrimSpace(*params.Skill)
		args.Skill = &v
	}
	if params.Q != nil && strings.TrimSpace(*params.Q) != "" {
		v := likeEscaper.Replace(strings.TrimSpace(*params.Q))
		args.Q = &v
	}
	if params.Cursor != nil {
		var before uuid.UUID
		if err := pagination.Decode(*params.Cursor, &before); err != nil {
			return apigen.TeamRequestPage{}, err
		}
		args.BeforeID = &before
	}
	page := apigen.TeamRequestPage{Items: []apigen.TeamRequest{}}
	err = s.db.ReadTx(ctx, func(ctx context.Context) error {
		rows, err := s.q(ctx).ListTeamRequests(ctx, args)
		if err != nil {
			return err
		}
		if len(rows) > limit {
			rows = rows[:limit]
			next := pagination.Encode(rows[len(rows)-1].ID)
			page.NextCursor = &next
		}
		page.Items, err = s.view(ctx, p.UserID, rows)
		return err
	})
	if err != nil {
		return page, fmt.Errorf("team: list: %w", err)
	}
	return page, nil
}

type fields struct {
	title       *string
	description *string
	roles       []string
	skills      []string
}

func check(v *validate.Errors, title, description *string, roles *[]apigen.TeamRole, skills *[]string) fields {
	var f fields
	if title != nil {
		t := v.Text("title", *title, 5, 120)
		f.title = &t
	}
	if description != nil {
		d := v.Paragraph("description", *description, 20, 2000)
		f.description = &d
	}
	if roles != nil {
		for _, r := range *roles {
			if !r.Valid() {
				v.Add("roles_needed", validate.CodeInvalid, nil)
				break
			}
			if !slices.Contains(f.roles, string(r)) {
				f.roles = append(f.roles, string(r))
			}
		}
		switch {
		case len(f.roles) == 0 && !v.Has("roles_needed"):
			v.Add("roles_needed", validate.CodeMinItems, map[string]any{"count": 1})
		case len(f.roles) > 5:
			v.Add("roles_needed", validate.CodeMaxItems, map[string]any{"count": 5})
		}
	}
	if skills != nil {
		f.skills = v.Tags("skills", *skills, 15, 40)
	}
	return f
}

// Create publishes a request.
func (s *Service) Create(ctx context.Context, body apigen.TeamRequestCreate) (apigen.TeamRequest, error) {
	p, err := authz.Require(ctx)
	if err != nil {
		return apigen.TeamRequest{}, err
	}
	var v validate.Errors
	f := check(&v, &body.Title, &body.Description, &body.RolesNeeded, &body.Skills)
	if err := v.Err(); err != nil {
		return apigen.TeamRequest{}, err
	}
	if err := s.limiter.Allow(ctx, "team-create:"+p.UserID.String(), limitCreate, "rate_limited"); err != nil {
		return apigen.TeamRequest{}, err
	}
	var out apigen.TeamRequest
	err = s.db.Tx(ctx, func(ctx context.Context) error {
		r, err := s.q(ctx).CreateTeamRequest(ctx, teamdb.CreateTeamRequestParams{
			AuthorID: p.UserID, Title: *f.title, Description: *f.description, RolesNeeded: f.roles, Skills: f.skills,
		})
		if err != nil {
			return err
		}
		if err := s.audit.Record(ctx, audit.Event{Action: "team.request_created", TargetType: "team_request", TargetID: r.ID.String()}); err != nil {
			return err
		}
		views, err := s.view(ctx, p.UserID, []teamdb.TeamRequest{r})
		if err != nil {
			return err
		}
		out = views[0]
		return nil
	})
	if err != nil {
		return out, fmt.Errorf("team: create: %w", err)
	}
	return out, nil
}

// Get returns one request.
func (s *Service) Get(ctx context.Context, id uuid.UUID) (apigen.TeamRequest, error) {
	p, err := authz.Require(ctx)
	if err != nil {
		return apigen.TeamRequest{}, err
	}
	var out apigen.TeamRequest
	err = s.db.ReadTx(ctx, func(ctx context.Context) error {
		r, err := s.q(ctx).GetTeamRequest(ctx, id)
		if db.IsNoRows(err) {
			return errNotFound
		}
		if err != nil {
			return err
		}
		views, err := s.view(ctx, p.UserID, []teamdb.TeamRequest{r})
		if err != nil {
			return err
		}
		out = views[0]
		return nil
	})
	if err != nil {
		if _, ok := apperr.As(err); ok {
			return out, err
		}
		return out, fmt.Errorf("team: get: %w", err)
	}
	return out, nil
}

// ETag is the version of a request for optimistic locking.
func ETag(r apigen.TeamRequest) string {
	return `"` + strconv.FormatInt(r.UpdatedAt.UnixNano(), 36) + `"`
}

func etagMatches(ifMatch *string, updatedAt time.Time) bool {
	if ifMatch == nil {
		return true
	}
	current := ETag(apigen.TeamRequest{UpdatedAt: updatedAt})
	for candidate := range strings.SplitSeq(*ifMatch, ",") {
		candidate = strings.TrimPrefix(strings.TrimSpace(candidate), "W/")
		if candidate == current || candidate == "*" {
			return true
		}
	}
	return false
}

// Update edits, closes or reopens a request; only its author (or a moderator)
// may. With ifMatch, a request changed since that version is not overwritten.
func (s *Service) Update(ctx context.Context, id uuid.UUID, body apigen.TeamRequestUpdate, ifMatch *string) (apigen.TeamRequest, error) {
	p, err := authz.Require(ctx)
	if err != nil {
		return apigen.TeamRequest{}, err
	}
	var v validate.Errors
	f := check(&v, body.Title, body.Description, body.RolesNeeded, body.Skills)
	var status *string
	if body.Status != nil {
		if !body.Status.Valid() {
			v.Add("status", validate.CodeInvalid, nil)
		}
		st := string(*body.Status)
		status = &st
	}
	if err := v.Err(); err != nil {
		return apigen.TeamRequest{}, err
	}
	var out apigen.TeamRequest
	err = s.db.Tx(ctx, func(ctx context.Context) error {
		q := s.q(ctx)
		r, err := q.LockTeamRequest(ctx, id)
		if db.IsNoRows(err) {
			return errNotFound
		}
		if err != nil {
			return err
		}
		if r.AuthorID != p.UserID && !canModerate(p) {
			return errForbidden
		}
		if !etagMatches(ifMatch, r.UpdatedAt) {
			return apperr.PreconditionFailed("precondition.failed", "the request was changed since it was loaded")
		}
		r, err = q.UpdateTeamRequest(ctx, teamdb.UpdateTeamRequestParams{
			Title: f.title, Description: f.description, RolesNeeded: f.roles, Skills: f.skills, Status: status, ID: id,
		})
		if err != nil {
			return err
		}
		views, err := s.view(ctx, p.UserID, []teamdb.TeamRequest{r})
		if err != nil {
			return err
		}
		out = views[0]
		return nil
	})
	if err != nil {
		if _, ok := apperr.As(err); ok {
			return out, err
		}
		return out, fmt.Errorf("team: update: %w", err)
	}
	return out, nil
}

func responseAPI(r teamdb.TeamResponse, author apigen.MemberBrief, email *string) apigen.TeamResponse {
	return apigen.TeamResponse{Author: author, CreatedAt: r.CreatedAt, Email: email, Id: r.ID, Message: r.Message}
}

// Responses lists replies: all of them to the author, only their own to others.
// The e-mail of a responder is shared with the author only.
func (s *Service) Responses(ctx context.Context, id uuid.UUID) ([]apigen.TeamResponse, error) {
	p, err := authz.Require(ctx)
	if err != nil {
		return nil, err
	}
	out := []apigen.TeamResponse{}
	err = s.db.ReadTx(ctx, func(ctx context.Context) error {
		q := s.q(ctx)
		r, err := q.GetTeamRequest(ctx, id)
		if db.IsNoRows(err) {
			return errNotFound
		}
		if err != nil {
			return err
		}
		owner := r.AuthorID == p.UserID || canModerate(p)
		rows, err := q.ListTeamResponses(ctx, id)
		if err != nil {
			return err
		}
		var visible []teamdb.TeamResponse
		var authors []uuid.UUID
		for _, resp := range rows {
			if owner || resp.AuthorID == p.UserID {
				visible = append(visible, resp)
				authors = append(authors, resp.AuthorID)
			}
		}
		briefs, err := s.users.Briefs(ctx, authors)
		if err != nil {
			return err
		}
		emails := map[uuid.UUID]string{}
		if r.AuthorID == p.UserID {
			for _, a := range authors {
				u, err := userdb.New(s.db.Conn(ctx)).GetUser(ctx, a)
				if err != nil {
					return err
				}
				emails[a] = u.Email
			}
		}
		for _, resp := range visible {
			var email *string
			if e, ok := emails[resp.AuthorID]; ok {
				email = &e
			}
			out = append(out, responseAPI(resp, briefs[resp.AuthorID], email))
		}
		return nil
	})
	if err != nil {
		if _, ok := apperr.As(err); ok {
			return nil, err
		}
		return nil, fmt.Errorf("team: responses: %w", err)
	}
	return out, nil
}

// Respond replies to an open request of someone else and notifies its author.
func (s *Service) Respond(ctx context.Context, id uuid.UUID, body apigen.TeamResponseCreate) (apigen.TeamResponse, error) {
	p, err := authz.Require(ctx)
	if err != nil {
		return apigen.TeamResponse{}, err
	}
	var v validate.Errors
	message := v.Paragraph("message", body.Message, 10, 1000)
	if err := v.Err(); err != nil {
		return apigen.TeamResponse{}, err
	}
	if err := s.limiter.Allow(ctx, "team-respond:"+p.UserID.String(), limitRespond, "rate_limited"); err != nil {
		return apigen.TeamResponse{}, err
	}
	t, err := tenancy.Require(ctx)
	if err != nil {
		return apigen.TeamResponse{}, err
	}
	var out apigen.TeamResponse
	err = s.db.Tx(ctx, func(ctx context.Context) error {
		q := s.q(ctx)
		r, err := q.GetTeamRequest(ctx, id)
		if db.IsNoRows(err) {
			return errNotFound
		}
		if err != nil {
			return err
		}
		switch {
		case r.AuthorID == p.UserID:
			return apperr.Conflict("team.own_request", "you cannot reply to your own request")
		case r.Status != statusOpen:
			return apperr.Conflict("team.closed", "the request is closed")
		}
		resp, err := q.CreateTeamResponse(ctx, teamdb.CreateTeamResponseParams{RequestID: id, AuthorID: p.UserID, Message: message})
		if db.IsUniqueViolation(err, "") {
			return apperr.Conflict("team.already_responded", "you have already replied")
		}
		if err != nil {
			return err
		}
		briefs, err := s.users.Briefs(ctx, []uuid.UUID{p.UserID})
		if err != nil {
			return err
		}
		out = responseAPI(resp, briefs[p.UserID], nil)

		author, err := userdb.New(s.db.Conn(ctx)).GetUser(ctx, r.AuthorID)
		if err != nil {
			return err
		}
		if err := s.mailer.Send(ctx, author.Email, author.FullName, mail.TemplateTeamResponse, author.Locale, map[string]string{
			"name": author.FullName, "responder": briefs[p.UserID].FullName, "request": r.Title,
			"message": message, "link": t.Origin + "/team/" + id.String(),
		}); err != nil {
			return err
		}
		return s.audit.Record(ctx, audit.Event{Action: "team.responded", TargetType: "team_request", TargetID: id.String()})
	})
	if err != nil {
		if _, ok := apperr.As(err); ok {
			return out, err
		}
		return out, fmt.Errorf("team: respond: %w", err)
	}
	return out, nil
}

// Handler serves the team operations of the API.
type Handler struct {
	svc *Service
}

// NewHandler returns the team handler.
func NewHandler(svc *Service) *Handler { return &Handler{svc: svc} }

// ListTeamRequests implements apigen.StrictServerInterface.
func (h *Handler) ListTeamRequests(ctx context.Context, req apigen.ListTeamRequestsRequestObject) (apigen.ListTeamRequestsResponseObject, error) {
	page, err := h.svc.List(ctx, req.Params)
	if err != nil {
		return nil, err
	}
	return apigen.ListTeamRequests200JSONResponse(page), nil
}

// CreateTeamRequest implements apigen.StrictServerInterface.
func (h *Handler) CreateTeamRequest(ctx context.Context, req apigen.CreateTeamRequestRequestObject) (apigen.CreateTeamRequestResponseObject, error) {
	r, err := h.svc.Create(ctx, *req.Body)
	if err != nil {
		return nil, err
	}
	return apigen.CreateTeamRequest201JSONResponse(r), nil
}

// GetTeamRequest implements apigen.StrictServerInterface.
func (h *Handler) GetTeamRequest(ctx context.Context, req apigen.GetTeamRequestRequestObject) (apigen.GetTeamRequestResponseObject, error) {
	r, err := h.svc.Get(ctx, req.RequestId)
	if err != nil {
		return nil, err
	}
	tag := ETag(r)
	return apigen.GetTeamRequest200JSONResponse{Body: r, Headers: apigen.GetTeamRequest200ResponseHeaders{ETag: &tag}}, nil
}

// UpdateTeamRequest implements apigen.StrictServerInterface.
func (h *Handler) UpdateTeamRequest(ctx context.Context, req apigen.UpdateTeamRequestRequestObject) (apigen.UpdateTeamRequestResponseObject, error) {
	r, err := h.svc.Update(ctx, req.RequestId, *req.Body, req.Params.IfMatch)
	if err != nil {
		return nil, err
	}
	tag := ETag(r)
	return apigen.UpdateTeamRequest200JSONResponse{Body: r, Headers: apigen.UpdateTeamRequest200ResponseHeaders{ETag: &tag}}, nil
}

// ListTeamResponses implements apigen.StrictServerInterface.
func (h *Handler) ListTeamResponses(ctx context.Context, req apigen.ListTeamResponsesRequestObject) (apigen.ListTeamResponsesResponseObject, error) {
	items, err := h.svc.Responses(ctx, req.RequestId)
	if err != nil {
		return nil, err
	}
	return apigen.ListTeamResponses200JSONResponse{Items: items}, nil
}

// RespondToTeamRequest implements apigen.StrictServerInterface.
func (h *Handler) RespondToTeamRequest(ctx context.Context, req apigen.RespondToTeamRequestRequestObject) (apigen.RespondToTeamRequestResponseObject, error) {
	r, err := h.svc.Respond(ctx, req.RequestId, *req.Body)
	if err != nil {
		return nil, err
	}
	return apigen.RespondToTeamRequest201JSONResponse(r), nil
}

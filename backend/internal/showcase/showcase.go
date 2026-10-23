// Package showcase publishes projects: the page a team prepares from its
// incubator answers, its moderation, the public catalog of the university and
// the national catalog of every university, link previews and search engine
// pages, and the contact requests partners send to teams.
//
// A page is live inside its university once published (published_at is set)
// and outside it only while it is published with public visibility, which an
// innovation manager approves (D-043).
package showcase

import (
	"context"
	"encoding/json"
	"fmt"
	"mime/multipart"
	"regexp"
	"slices"
	"strconv"
	"strings"
	"time"
	"unicode"

	"github.com/google/uuid"

	"github.com/yelnurq/startify/backend/internal/apiconv"
	"github.com/yelnurq/startify/backend/internal/apigen"
	"github.com/yelnurq/startify/backend/internal/audit"
	"github.com/yelnurq/startify/backend/internal/files"
	"github.com/yelnurq/startify/backend/internal/incubator"
	"github.com/yelnurq/startify/backend/internal/mail"
	"github.com/yelnurq/startify/backend/internal/org"
	"github.com/yelnurq/startify/backend/internal/platform/apperr"
	"github.com/yelnurq/startify/backend/internal/platform/authz"
	"github.com/yelnurq/startify/backend/internal/platform/db"
	"github.com/yelnurq/startify/backend/internal/platform/gotenberg"
	"github.com/yelnurq/startify/backend/internal/platform/ratelimit"
	"github.com/yelnurq/startify/backend/internal/platform/tenancy"
	"github.com/yelnurq/startify/backend/internal/platform/validate"
	"github.com/yelnurq/startify/backend/internal/project"
	"github.com/yelnurq/startify/backend/internal/showcase/showcasedb"
	"github.com/yelnurq/startify/backend/internal/tenant"
	"github.com/yelnurq/startify/backend/internal/user"
)

// Page statuses.
const (
	StatusDraft     = "draft"
	StatusInReview  = "in_review"
	StatusPublished = "published"
	StatusRejected  = "rejected"
)

// Visibilities of a page.
const (
	VisibilityUniversity = "university"
	VisibilityPublic     = "public"
)

// Limits of the page.
const (
	maxTitle    = 120
	maxTagline  = 160
	maxVideoURL = 500
	maxNeeds    = 5
	maxTags     = 8
	maxTag      = 40
	maxComment  = 1000
	maxCover    = 10 << 20
)

var sectionLimits = map[string]int{"problem": 1500, "solution": 1500, "market": 1000, "traction": 1000, "ask": 800}

var (
	errNotFound    = apperr.NotFound("showcase.not_found", "the page does not exist")
	errIncomplete  = apperr.New(apperr.KindValidation, "showcase.incomplete", "save the page with a problem and a solution first")
	errNotInReview = apperr.Conflict("showcase.not_in_review", "the page is not waiting for review")
	errForbidden   = apperr.Forbidden("rbac.forbidden", "not allowed for your role in the project")
	errOwnPage     = apperr.Forbidden("showcase.own_page", "a moderator does not approve the page of their own project")
	errSlugTaken   = apperr.Validation(apperr.FieldError{Field: "slug", Code: validate.CodeTaken})
	errPageStale   = apperr.PreconditionFailed("precondition.failed", "the page was changed since it was loaded")

	slugPattern = regexp.MustCompile(`^[a-z0-9]+(-[a-z0-9]+)*$`)

	// Video pitches are embedded from these hosts (the CSP allows their players).
	videoHosts = []string{"youtube.com", "youtu.be", "vimeo.com"}

	needs       = []string{"investment", "team", "partner", "pilot", "mentor"}
	reviewRoles = []authz.Role{authz.RoleInnovationManager, authz.RoleTenantAdmin}
)

// Counters give the public figures of the landing page.
type Counters interface {
	Count(ctx context.Context) (int, error)
}

// Deps are the collaborators of the service.
type Deps struct {
	DB          *db.DB
	Projects    *project.Service
	Users       *user.Service
	Org         *org.Service
	Tenants     *tenant.Service
	Files       *files.Service
	Evaluations project.EvaluationSource
	// ReadyEvaluations counts finished evaluations for the landing page.
	ReadyEvaluations func(ctx context.Context) (int, error)
	Mailer           *mail.Mailer
	Audit            *audit.Logger
	Limiter          *ratelimit.Limiter
	Gotenberg        *gotenberg.Client
	// ShellURL is the address of the web client's index.html that search
	// engine pages are built on; empty serves a minimal page.
	ShellURL string
}

// Service manages showcase pages.
type Service struct {
	Deps
	now   func() time.Time
	shell *shell
}

// NewService returns the showcase service.
func NewService(d Deps) *Service {
	return &Service{Deps: d, now: time.Now, shell: newShell(d.ShellURL)}
}

func (s *Service) q(ctx context.Context) *showcasedb.Queries { return showcasedb.New(s.DB.Conn(ctx)) }

// Sections are the texts of a page.
type Sections struct {
	Problem  string `json:"problem,omitempty"`
	Solution string `json:"solution,omitempty"`
	Market   string `json:"market,omitempty"`
	Traction string `json:"traction,omitempty"`
	Ask      string `json:"ask,omitempty"`
}

func (x Sections) api() apigen.ShowcaseSections {
	opt := func(s string) *string {
		if s == "" {
			return nil
		}
		return &s
	}
	return apigen.ShowcaseSections{
		Problem: opt(x.Problem), Solution: opt(x.Solution), Market: opt(x.Market), Traction: opt(x.Traction), Ask: opt(x.Ask),
	}
}

// Member is a person of the team as the page shows them (avatars are public
// files, readable from any university).
type Member struct {
	Name      string  `json:"name"`
	Role      string  `json:"role"`
	AvatarURL *string `json:"avatar_url,omitempty"`
}

func teamAPI(raw json.RawMessage) []apigen.ShowcaseTeamMember {
	var team []Member
	_ = json.Unmarshal(raw, &team)
	out := make([]apigen.ShowcaseTeamMember, len(team))
	for i, m := range team {
		out[i] = apigen.ShowcaseTeamMember{Name: m.Name, Role: m.Role, AvatarUrl: m.AvatarURL}
	}
	return out
}

func sectionsOf(raw json.RawMessage) Sections {
	var x Sections
	_ = json.Unmarshal(raw, &x)
	return x
}

func needsAPI(v []string) []apigen.ShowcaseNeed {
	out := make([]apigen.ShowcaseNeed, len(v))
	for i, n := range v {
		out[i] = apigen.ShowcaseNeed(n)
	}
	return out
}

func intPtr(v *int32) *int {
	if v == nil {
		return nil
	}
	n := int(*v)
	return &n
}

func levelPtr(v *string) *apigen.ReadinessLevel {
	if v == nil {
		return nil
	}
	l := apigen.ReadinessLevel(*v)
	return &l
}

// ETag versions a page; a page that was never saved is version 0.
func ETag(version int32) string { return `"` + strconv.Itoa(int(version)) + `"` }

// live reports whether members of the university see the page.
func live(e *showcasedb.ShowcaseEntry) bool { return e.PublishedAt != nil }

// public reports whether everyone sees the page.
func public(status, visibility string) bool {
	return status == StatusPublished && visibility == VisibilityPublic
}

// pageURL is the address of a page on the site of the current university.
func pageURL(ctx context.Context, slug string) string {
	t, _ := tenancy.FromContext(ctx)
	return t.Origin + "/showcase/" + slug
}

// entryAPI is the page as the team and moderators see it.
func entryAPI(ctx context.Context, e *showcasedb.ShowcaseEntry) apigen.ShowcaseEntry {
	out := apigen.ShowcaseEntry{
		ProjectId: e.ProjectID, Saved: true, Version: int(e.Version), Held: e.Held, Slug: e.Slug, Status: apigen.ShowcaseStatus(e.Status),
		Visibility: apigen.ShowcaseVisibility(e.Visibility), Title: e.Title, Tagline: e.Tagline,
		Industry: apigen.Industry(e.Industry), Stage: apigen.ShowcaseStage(e.Stage), CoverUrl: apiconv.FileURL(e.CoverFileID),
		VideoUrl: e.VideoUrl, Sections: sectionsOf(e.Sections).api(), Team: teamAPI(e.Team), Needs: needsAPI(e.Needs),
		Tags: e.Tags, ShowScore: e.ShowScore, Score: intPtr(e.Score), Level: levelPtr(e.Level),
		ContactEnabled: e.ContactEnabled, ReviewComment: e.ReviewComment, SubmittedAt: e.SubmittedAt, PublishedAt: e.PublishedAt,
	}
	if live(e) {
		u := pageURL(ctx, e.Slug)
		out.Url = &u
	}
	return out
}

// load returns the saved page of a project, or nil.
func (s *Service) load(ctx context.Context, projectID uuid.UUID) (*showcasedb.ShowcaseEntry, error) {
	var row *showcasedb.ShowcaseEntry
	err := s.DB.ReadTx(ctx, func(ctx context.Context) error {
		e, err := s.q(ctx).GetEntry(ctx, projectID)
		if db.IsNoRows(err) {
			return nil
		}
		row = &e
		return err
	})
	if err != nil {
		return nil, fmt.Errorf("showcase: load: %w", err)
	}
	return row, nil
}

// PublicURL returns the address of the project's page when everyone can see
// it, or "".
func (s *Service) PublicURL(ctx context.Context, projectID uuid.UUID) (string, error) {
	row, err := s.load(ctx, projectID)
	if err != nil || row == nil || !public(row.Status, row.Visibility) {
		return "", err
	}
	return pageURL(ctx, row.Slug), nil
}

// derived is what a page takes from the project at every save.
type derived struct {
	stage     string
	facultyID *uuid.UUID
	faculty   json.RawMessage
	year      int32
	team      []Member
	score     *int32
	level     *string
}

// teamOf is the team as a page shows it: the owner and the members (not the
// supervisors), with the roles the team step names or their titles. It
// returns the owner's faculty too.
func (s *Service) teamOf(ctx context.Context, projectID uuid.UUID, roles map[string]string) ([]Member, *uuid.UUID, error) {
	members, err := s.Projects.Members(ctx, projectID)
	if err != nil {
		return nil, nil, err
	}
	team := []Member{}
	var faculty *uuid.UUID
	for i := range members {
		m := &members[i]
		if m.Role != apigen.ProjectRoleOwner && m.Role != apigen.ProjectRoleMember {
			continue
		}
		role := roles[strings.ToLower(m.User.FullName)]
		if role == "" {
			role = m.Title
		}
		team = append(team, Member{Name: m.User.FullName, Role: role, AvatarURL: m.User.AvatarUrl})
		if faculty == nil && m.Role == apigen.ProjectRoleOwner && m.User.Faculty != nil {
			id := m.User.Faculty.Id
			faculty = &id
		}
	}
	return team, faculty, nil
}

func (s *Service) derive(ctx context.Context, a project.Access, roles map[string]string) (derived, error) {
	p := a.Project
	d := derived{stage: incubator.StageOf(p.CurrentStep), year: int32(p.CreatedAt.Year())} //nolint:gosec // a calendar year
	var err error
	if d.team, d.facultyID, err = s.teamOf(ctx, p.ID, roles); err != nil {
		return d, err
	}
	if p.FacultyID != nil {
		d.facultyID = p.FacultyID
	}
	if d.facultyID != nil {
		err := s.DB.ReadTx(ctx, func(ctx context.Context) error {
			refs, err := s.Org.LoadRefs(ctx, []uuid.UUID{*d.facultyID}, nil, nil)
			if f, ok := refs.Faculties[*d.facultyID]; ok {
				d.faculty, _ = json.Marshal(f.Name)
			}
			return err
		})
		if err != nil {
			return d, err
		}
	}
	if s.Evaluations != nil {
		latest, err := s.Evaluations.Latest(ctx, []uuid.UUID{p.ID})
		if err != nil {
			return d, err
		}
		if e, ok := latest[p.ID]; ok {
			score, level := int32(e.Total), string(e.Level) //nolint:gosec // 0..100
			d.score, d.level = &score, &level
		}
	}
	return d, nil
}

// suggestion is the unsaved page prefilled from the incubator answers.
func (s *Service) suggestion(ctx context.Context, a project.Access) (apigen.ShowcaseEntry, Sections, derived, error) {
	p := a.Project
	draft, err := incubator.DraftShowcase(ctx, s.DB, p.ID, p.Tagline)
	if err != nil {
		return apigen.ShowcaseEntry{}, Sections{}, derived{}, err
	}
	d, err := s.derive(ctx, a, draft.Roles)
	if err != nil {
		return apigen.ShowcaseEntry{}, Sections{}, derived{}, err
	}
	sections := Sections{Problem: draft.Problem, Solution: draft.Solution, Market: draft.Market, Traction: draft.Traction, Ask: draft.Ask}
	team, _ := json.Marshal(d.team)
	out := apigen.ShowcaseEntry{
		ProjectId: p.ID, Saved: false, Slug: Slugify(p.Title), Status: StatusDraft, Visibility: VisibilityUniversity,
		Title: p.Title, Tagline: draft.Tagline, Industry: apigen.Industry(p.Industry), Stage: apigen.ShowcaseStage(d.stage),
		Sections: sections.api(), Team: teamAPI(team), Needs: []apigen.ShowcaseNeed{}, Tags: []string{},
		Score: intPtr(d.score), Level: levelPtr(d.level), ContactEnabled: true, VideoUrl: videoLink(draft.VideoURL),
	}
	return out, sections, d, nil
}

// videoLink keeps a video link of the incubator answers only when the page
// may embed it (YouTube or Vimeo), the rule a saved page follows: a page
// saved without the editor (a cover uploaded first) must not slip past it.
func videoLink(u string) *string {
	if strings.TrimSpace(u) == "" {
		return nil
	}
	var v validate.Errors
	link := v.URL("video_url", &u, maxVideoURL, videoHosts...)
	if v.Err() != nil {
		return nil
	}
	return link
}

// Entry returns the page of a project with its ETag.
func (s *Service) Entry(ctx context.Context, projectID uuid.UUID) (apigen.ShowcaseEntry, string, error) {
	a, err := s.Projects.Access(ctx, projectID)
	if err != nil {
		return apigen.ShowcaseEntry{}, "", err
	}
	row, err := s.load(ctx, projectID)
	if err != nil {
		return apigen.ShowcaseEntry{}, "", err
	}
	if row != nil {
		return entryAPI(ctx, row), ETag(row.Version), nil
	}
	out, _, _, err := s.suggestion(ctx, a)
	return out, ETag(0), err
}

// input is a checked page from the team.
type input struct {
	slug, title, tagline, visibility string
	videoURL                         *string
	sections                         Sections
	needs, tags                      []string
	showScore, contactEnabled        bool
}

func check(body apigen.ShowcaseEntryUpdate) (input, error) {
	var v validate.Errors
	in := input{
		slug:       strings.ToLower(strings.TrimSpace(body.Slug)),
		title:      v.Text("title", body.Title, 2, maxTitle),
		visibility: string(body.Visibility),
		showScore:  body.ShowScore, contactEnabled: body.ContactEnabled,
	}
	if n := len(in.slug); n < 3 || n > 60 || !slugPattern.MatchString(in.slug) {
		v.Add("slug", validate.CodeFormat, nil)
	}
	tagline := v.Optional("tagline", &body.Tagline, maxTagline, false)
	if tagline != nil {
		in.tagline = *tagline
	}
	in.videoURL = v.URL("video_url", body.VideoUrl, maxVideoURL, videoHosts...)
	if in.visibility != VisibilityUniversity && in.visibility != VisibilityPublic {
		v.Add("visibility", validate.CodeInvalid, nil)
	}
	section := func(key string, value *string) string {
		if out := v.Optional("sections."+key, value, sectionLimits[key], true); out != nil {
			return *out
		}
		return ""
	}
	in.sections = Sections{
		Problem: section("problem", body.Sections.Problem), Solution: section("solution", body.Sections.Solution),
		Market: section("market", body.Sections.Market), Traction: section("traction", body.Sections.Traction),
		Ask: section("ask", body.Sections.Ask),
	}
	in.needs = []string{}
	for _, n := range body.Needs {
		switch {
		case !slices.Contains(needs, string(n)):
			v.Add("needs", validate.CodeInvalid, nil)
		case !slices.Contains(in.needs, string(n)):
			in.needs = append(in.needs, string(n))
		}
	}
	if len(in.needs) > maxNeeds {
		v.Add("needs", validate.CodeMaxItems, map[string]any{"count": maxNeeds})
	}
	in.tags = v.Tags("tags", body.Tags, maxTags, maxTag)
	for i := range in.tags {
		in.tags[i] = strings.ToLower(in.tags[i])
	}
	return in, v.Err()
}

// SearchText is what full-text search looks through.
func SearchText(title, tagline string, x Sections, tags []string) string {
	return strings.Join([]string{title, tagline, x.Problem, x.Solution, x.Market, x.Traction, x.Ask, strings.Join(tags, " ")}, " ")
}

// shownChanged reports whether a save changed what the page shows to the
// public: such a change of an approved public page needs a new approval.
func shownChanged(before, after *showcasedb.ShowcaseEntry) bool {
	return before.Slug != after.Slug || before.Title != after.Title || before.Tagline != after.Tagline ||
		string(before.Sections) != string(after.Sections) || !equalPtr(before.VideoUrl, after.VideoUrl) ||
		!slices.Equal(before.Tags, after.Tags) || !slices.Equal(before.Needs, after.Needs)
}

func equalPtr(a, b *string) bool {
	return (a == nil && b == nil) || (a != nil && b != nil && *a == *b)
}

func equalUUID(a, b *uuid.UUID) bool {
	return (a == nil && b == nil) || (a != nil && b != nil && *a == *b)
}

// statusAfterSave is the status of a live page after the team saved it:
// university pages publish at once; going public, a rejected page and any
// change of what an approved public page shows wait for review (D-043, as
// changed: an approval covers the version that was reviewed).
func statusAfterSave(before, after *showcasedb.ShowcaseEntry) string {
	switch {
	case after.Visibility == VisibilityUniversity:
		return StatusPublished
	case before.Visibility == VisibilityUniversity || before.Status == StatusRejected:
		return StatusInReview
	case before.Status == StatusPublished && shownChanged(before, after):
		return StatusInReview
	}
	return after.Status
}

// Update saves the page. Moving a published page to public visibility, or
// changing what an approved public page shows, sends it to review; it stays
// live inside the university meanwhile.
func (s *Service) Update(ctx context.Context, projectID uuid.UUID, body apigen.ShowcaseEntryUpdate, ifMatch *string) (apigen.ShowcaseEntry, string, error) {
	a, err := s.Projects.Access(ctx, projectID)
	if err != nil {
		return apigen.ShowcaseEntry{}, "", err
	}
	if err := a.RequireEdit(); err != nil {
		return apigen.ShowcaseEntry{}, "", err
	}
	if ifMatch == nil {
		return apigen.ShowcaseEntry{}, "", apperr.PreconditionRequired("precondition.required", "send If-Match with the version you edit")
	}
	in, err := check(body)
	if err != nil {
		return apigen.ShowcaseEntry{}, "", err
	}
	draft, err := incubator.DraftShowcase(ctx, s.DB, projectID, a.Project.Tagline)
	if err != nil {
		return apigen.ShowcaseEntry{}, "", err
	}
	d, err := s.derive(ctx, a, draft.Roles)
	if err != nil {
		return apigen.ShowcaseEntry{}, "", err
	}
	var row showcasedb.ShowcaseEntry
	err = s.DB.Tx(ctx, func(ctx context.Context) error {
		// The project first, then the page: the order every writer keeps.
		if err := s.Projects.Lock(ctx, projectID); err != nil {
			return err
		}
		q := s.q(ctx)
		current, err := q.LockEntry(ctx, projectID)
		var version int32
		switch {
		case err == nil:
			version = current.Version
		case db.IsNoRows(err):
			current = showcasedb.ShowcaseEntry{Status: StatusDraft}
		default:
			return err
		}
		if strings.TrimPrefix(*ifMatch, "W/") != ETag(version) {
			return apperr.PreconditionFailed("precondition.failed", "the page was changed since it was loaded")
		}
		if owner, err := q.SlugOwner(ctx, in.slug); err == nil && owner != projectID {
			return errSlugTaken
		} else if err != nil && !db.IsNoRows(err) {
			return err
		}
		sections, _ := json.Marshal(in.sections)
		team, _ := json.Marshal(d.team)
		if row, err = q.UpsertEntry(ctx, showcasedb.UpsertEntryParams{
			ProjectID: projectID, Slug: in.slug, Visibility: in.visibility, Title: in.title, Tagline: in.tagline,
			Industry: a.Project.Industry, Stage: d.stage, FacultyID: d.facultyID, Faculty: d.faculty, Year: d.year,
			VideoUrl: in.videoURL, Sections: sections, Team: team, Needs: in.needs, Tags: in.tags, ShowScore: in.showScore,
			Score: d.score, Level: d.level, ContactEnabled: in.contactEnabled, SearchText: SearchText(in.title, in.tagline, in.sections, in.tags),
		}); err != nil {
			// Another project took the address after the check above.
			if db.IsUniqueViolation(err, "") {
				return errSlugTaken
			}
			return err
		}
		if live(&row) {
			if row, err = s.setStatus(ctx, &row, statusAfterSave(&current, &row)); err != nil {
				return err
			}
		}
		return s.Audit.Record(ctx, audit.Event{Action: "showcase.updated", TargetType: "project", TargetID: projectID.String()})
	})
	if err != nil {
		if _, ok := apperr.As(err); ok {
			return apigen.ShowcaseEntry{}, "", err
		}
		return apigen.ShowcaseEntry{}, "", fmt.Errorf("showcase: update: %w", err)
	}
	return entryAPI(ctx, &row), ETag(row.Version), nil
}

// setStatus moves a page to status when it changes, stamping a submission
// for review, and keeps the project's visibility in step (inside the
// transaction of ctx).
func (s *Service) setStatus(ctx context.Context, row *showcasedb.ShowcaseEntry, status string) (showcasedb.ShowcaseEntry, error) {
	out := *row
	if status != row.Status {
		submitted := row.SubmittedAt
		if status == StatusInReview {
			now := s.now()
			submitted = &now
		}
		var err error
		if out, err = s.q(ctx).SetStatus(ctx, showcasedb.SetStatusParams{
			ProjectID: row.ProjectID, Status: status, SubmittedAt: submitted, PublishedAt: row.PublishedAt,
			ReviewComment: row.ReviewComment, ReviewedBy: row.ReviewedBy, Held: row.Held,
		}); err != nil {
			return out, err
		}
	}
	err := s.syncProject(ctx, &out)
	return out, err
}

// syncProject keeps the visibility of the project in step with its page.
func (s *Service) syncProject(ctx context.Context, e *showcasedb.ShowcaseEntry) error {
	visibility := "private"
	switch {
	case public(e.Status, e.Visibility):
		visibility = VisibilityPublic
	case live(e):
		visibility = VisibilityUniversity
	}
	return s.Projects.SetVisibility(ctx, e.ProjectID, visibility)
}

// Publish puts the saved page on the showcase: university pages at once,
// public pages after review (live inside the university meanwhile). A page a
// moderator took down waits for review whatever its visibility, and stays
// hidden until a moderator approves it.
func (s *Service) Publish(ctx context.Context, projectID uuid.UUID) (apigen.ShowcaseEntry, error) {
	a, err := s.Projects.Access(ctx, projectID)
	if err != nil {
		return apigen.ShowcaseEntry{}, err
	}
	if err := a.RequireEdit(); err != nil {
		return apigen.ShowcaseEntry{}, err
	}
	var row showcasedb.ShowcaseEntry
	err = s.DB.Tx(ctx, func(ctx context.Context) error {
		if err := s.Projects.Lock(ctx, projectID); err != nil {
			return err
		}
		q := s.q(ctx)
		var err error
		row, err = q.LockEntry(ctx, projectID)
		if db.IsNoRows(err) {
			return errIncomplete
		}
		if err != nil {
			return err
		}
		x := sectionsOf(row.Sections)
		if x.Problem == "" || x.Solution == "" {
			return errIncomplete
		}
		now := s.now()
		published := row.PublishedAt
		if published == nil && !row.Held {
			published = &now
		}
		status, submitted, action := StatusPublished, row.SubmittedAt, "showcase.published"
		if row.Held || (row.Visibility == VisibilityPublic && row.Status != StatusPublished) {
			status, submitted, action = StatusInReview, &now, "showcase.submitted"
		}
		if row, err = q.SetStatus(ctx, showcasedb.SetStatusParams{
			ProjectID: projectID, Status: status, SubmittedAt: submitted, PublishedAt: published,
			ReviewComment: row.ReviewComment, ReviewedBy: row.ReviewedBy, Held: row.Held,
		}); err != nil {
			return err
		}
		if err := s.syncProject(ctx, &row); err != nil {
			return err
		}
		return s.Audit.Record(ctx, audit.Event{Action: action, TargetType: "project", TargetID: projectID.String()})
	})
	if err != nil {
		if _, ok := apperr.As(err); ok {
			return apigen.ShowcaseEntry{}, err
		}
		return apigen.ShowcaseEntry{}, fmt.Errorf("showcase: publish: %w", err)
	}
	return entryAPI(ctx, &row), nil
}

// Unpublish takes the page off the showcase (back to draft). A moderator's
// take-down holds the page: the team publishes it again only through review.
func (s *Service) Unpublish(ctx context.Context, projectID uuid.UUID) (apigen.ShowcaseEntry, error) {
	a, err := s.Projects.Access(ctx, projectID)
	if err != nil {
		return apigen.ShowcaseEntry{}, err
	}
	if !a.Team() && !a.CanModerate() {
		return apigen.ShowcaseEntry{}, errForbidden
	}
	takeDown := !a.Team()
	var row showcasedb.ShowcaseEntry
	err = s.DB.Tx(ctx, func(ctx context.Context) error {
		if err := s.Projects.Lock(ctx, projectID); err != nil {
			return err
		}
		q := s.q(ctx)
		var err error
		if row, err = q.LockEntry(ctx, projectID); err != nil {
			return err
		}
		if row, err = q.SetStatus(ctx, showcasedb.SetStatusParams{
			ProjectID: projectID, Status: StatusDraft, ReviewComment: row.ReviewComment, ReviewedBy: row.ReviewedBy,
			Held: row.Held || takeDown,
		}); err != nil {
			return err
		}
		if err := s.syncProject(ctx, &row); err != nil {
			return err
		}
		action := "showcase.unpublished"
		if takeDown {
			action = "showcase.taken_down"
		}
		return s.Audit.Record(ctx, audit.Event{Action: action, TargetType: "project", TargetID: projectID.String()})
	})
	if db.IsNoRows(err) {
		return apigen.ShowcaseEntry{}, errNotFound
	}
	if err != nil {
		return apigen.ShowcaseEntry{}, fmt.Errorf("showcase: unpublish: %w", err)
	}
	return entryAPI(ctx, &row), nil
}

// TeamChanged implements project.TeamListener: the team a saved page shows
// (on the national showcase and in its JSON-LD too) follows the project's
// members, so someone who left the team leaves the page at once.
func (s *Service) TeamChanged(ctx context.Context, projectID uuid.UUID) error {
	row, err := s.load(ctx, projectID)
	if err != nil || row == nil {
		return err
	}
	var current []Member
	_ = json.Unmarshal(row.Team, &current)
	draft, err := incubator.DraftShowcase(ctx, s.DB, projectID, "")
	if err != nil {
		return err
	}
	team, _, err := s.teamOf(ctx, projectID, draft.Roles)
	if err != nil {
		return err
	}
	if slices.EqualFunc(team, current, func(a, b Member) bool {
		return a.Name == b.Name && a.Role == b.Role && equalPtr(a.AvatarURL, b.AvatarURL)
	}) {
		return nil
	}
	raw, err := json.Marshal(team)
	if err != nil {
		return err
	}
	if _, err := s.q(ctx).SetTeam(ctx, showcasedb.SetTeamParams{ProjectID: projectID, Team: raw}); err != nil && !db.IsNoRows(err) {
		return fmt.Errorf("showcase: team: %w", err)
	}
	return nil
}

// ensure saves the suggested page when the team changes it for the first
// time without the editor (a cover upload), with a free address.
func (s *Service) ensure(ctx context.Context, a project.Access) error {
	if row, err := s.load(ctx, a.Project.ID); err != nil || row != nil {
		return err
	}
	suggested, sections, d, err := s.suggestion(ctx, a)
	if err != nil {
		return err
	}
	return s.DB.Tx(ctx, func(ctx context.Context) error {
		q := s.q(ctx)
		if _, err := q.LockEntry(ctx, a.Project.ID); err == nil || !db.IsNoRows(err) {
			return err
		}
		slug := suggested.Slug
		for i := 2; ; i++ {
			_, err := q.SlugOwner(ctx, slug)
			if db.IsNoRows(err) {
				break
			}
			if err != nil {
				return err
			}
			slug = fmt.Sprintf("%s-%d", strings.TrimSuffix(suggested.Slug[:min(len(suggested.Slug), 55)], "-"), i)
		}
		raw, _ := json.Marshal(sections)
		team, _ := json.Marshal(d.team)
		_, err := q.UpsertEntry(ctx, showcasedb.UpsertEntryParams{
			ProjectID: a.Project.ID, Slug: slug, Visibility: VisibilityUniversity, Title: suggested.Title, Tagline: suggested.Tagline,
			Industry: a.Project.Industry, Stage: d.stage, FacultyID: d.facultyID, Faculty: d.faculty, Year: d.year,
			VideoUrl: suggested.VideoUrl, Sections: raw, Team: team, Needs: []string{}, Tags: []string{},
			Score: d.score, Level: d.level, ContactEnabled: true, SearchText: SearchText(suggested.Title, suggested.Tagline, sections, nil),
		})
		return err
	})
}

// SetCover replaces the cover with an uploaded image (cropped to 16:9).
func (s *Service) SetCover(ctx context.Context, projectID uuid.UUID, form *multipart.Reader) (apigen.ShowcaseEntry, error) {
	a, err := s.Projects.Access(ctx, projectID)
	if err != nil {
		return apigen.ShowcaseEntry{}, err
	}
	if err := a.RequireEdit(); err != nil {
		return apigen.ShowcaseEntry{}, err
	}
	_, part, err := files.ReadForm(form, "file", maxCover)
	if err != nil {
		return apigen.ShowcaseEntry{}, err
	}
	image, err := files.ProcessCover(part.Data)
	if err != nil {
		return apigen.ShowcaseEntry{}, err
	}
	if err := s.ensure(ctx, a); err != nil {
		return apigen.ShowcaseEntry{}, fmt.Errorf("showcase: cover: %w", err)
	}
	f, err := s.Files.Store(ctx, files.Upload{
		OwnerID: &a.Principal.UserID, Purpose: files.PurposeProjectCover, Visibility: files.VisibilityPublic,
		FileName: "cover.jpg", ContentType: "image/jpeg", Data: image,
	})
	if err != nil {
		return apigen.ShowcaseEntry{}, err
	}
	row, err := s.replaceCover(ctx, projectID, &f.ID)
	if err != nil {
		_ = s.DB.Tx(context.WithoutCancel(ctx), func(ctx context.Context) error { return s.Files.Delete(ctx, f.ID) })
		return apigen.ShowcaseEntry{}, err
	}
	return entryAPI(ctx, &row), nil
}

// DeleteCover removes the cover.
func (s *Service) DeleteCover(ctx context.Context, projectID uuid.UUID) (apigen.ShowcaseEntry, error) {
	a, err := s.Projects.Access(ctx, projectID)
	if err != nil {
		return apigen.ShowcaseEntry{}, err
	}
	if err := a.RequireEdit(); err != nil {
		return apigen.ShowcaseEntry{}, err
	}
	row, err := s.replaceCover(ctx, projectID, nil)
	if err != nil {
		return apigen.ShowcaseEntry{}, err
	}
	return entryAPI(ctx, &row), nil
}

func (s *Service) replaceCover(ctx context.Context, projectID uuid.UUID, fileID *uuid.UUID) (showcasedb.ShowcaseEntry, error) {
	var row showcasedb.ShowcaseEntry
	err := s.DB.Tx(ctx, func(ctx context.Context) error {
		if err := s.Projects.Lock(ctx, projectID); err != nil {
			return err
		}
		q := s.q(ctx)
		current, err := q.LockEntry(ctx, projectID)
		if err != nil {
			return err
		}
		if row, err = q.SetCover(ctx, showcasedb.SetCoverParams{ProjectID: projectID, CoverFileID: fileID}); err != nil {
			return err
		}
		// A new picture on an approved public page needs a new approval.
		if public(row.Status, row.Visibility) && !equalUUID(current.CoverFileID, fileID) {
			if row, err = s.setStatus(ctx, &row, StatusInReview); err != nil {
				return err
			}
		}
		if current.CoverFileID != nil {
			if err := s.Files.Delete(ctx, *current.CoverFileID); err != nil {
				return err
			}
		}
		return s.Audit.Record(ctx, audit.Event{Action: "showcase.cover_changed", TargetType: "project", TargetID: projectID.String()})
	})
	if db.IsNoRows(err) {
		return row, errNotFound
	}
	if err != nil {
		return row, fmt.Errorf("showcase: cover: %w", err)
	}
	return row, nil
}

// Reviews lists the pages of the university in a status for moderation
// (waiting for review by default), the oldest submission first.
func (s *Service) Reviews(ctx context.Context, status *apigen.ShowcaseStatus) (apigen.ShowcaseReviewList, error) {
	if _, err := authz.RequireRole(ctx, reviewRoles...); err != nil {
		return apigen.ShowcaseReviewList{}, err
	}
	want := StatusInReview
	if status != nil {
		want = string(*status)
	}
	out := apigen.ShowcaseReviewList{Items: []apigen.ShowcaseEntry{}}
	err := s.DB.ReadTx(ctx, func(ctx context.Context) error {
		rows, err := s.q(ctx).ListForReview(ctx, want)
		for i := range rows {
			out.Items = append(out.Items, entryAPI(ctx, &rows[i]))
		}
		return err
	})
	if err != nil {
		return out, fmt.Errorf("showcase: reviews: %w", err)
	}
	return out, nil
}

// Review approves a page for everyone or rejects it with a comment; a
// rejected page stays live inside the university. Published public pages can
// be rejected later too.
func (s *Service) Review(ctx context.Context, projectID uuid.UUID, body apigen.ShowcaseReview, ifMatch *string) (apigen.ShowcaseEntry, error) {
	p, err := authz.RequireRole(ctx, reviewRoles...)
	if err != nil {
		return apigen.ShowcaseEntry{}, err
	}
	var v validate.Errors
	comment := v.Optional("comment", body.Comment, maxComment, true)
	if !body.Approve && comment == nil && !v.Has("comment") {
		v.Add("comment", validate.CodeRequired, nil)
	}
	if err := v.Err(); err != nil {
		return apigen.ShowcaseEntry{}, err
	}
	// An approval publishes the version the moderator saw.
	if body.Approve && ifMatch == nil {
		return apigen.ShowcaseEntry{}, apperr.PreconditionRequired("precondition.required", "send If-Match with the version you reviewed")
	}
	if body.Approve {
		people, err := s.Projects.MemberIDs(ctx, projectID)
		if err != nil {
			return apigen.ShowcaseEntry{}, err
		}
		if _, own := people[p.UserID]; own {
			return apigen.ShowcaseEntry{}, errOwnPage
		}
	}
	var row showcasedb.ShowcaseEntry
	err = s.DB.Tx(ctx, func(ctx context.Context) error {
		if err := s.Projects.Lock(ctx, projectID); err != nil {
			return err
		}
		q := s.q(ctx)
		var err error
		if row, err = q.LockEntry(ctx, projectID); err != nil {
			return err
		}
		if ifMatch != nil && strings.TrimPrefix(strings.TrimSpace(*ifMatch), "W/") != ETag(row.Version) {
			return errPageStale
		}
		reviewable := row.Status == StatusInReview || (!body.Approve && public(row.Status, row.Visibility))
		if !reviewable {
			return errNotInReview
		}
		status, action, held, published := StatusPublished, "showcase.approved", false, row.PublishedAt
		if !body.Approve {
			status, action, held = StatusRejected, "showcase.rejected", row.Held
		} else if published == nil {
			// A page taken down by a moderator goes live again once approved.
			now := s.now()
			published = &now
		}
		if row, err = q.SetStatus(ctx, showcasedb.SetStatusParams{
			ProjectID: projectID, Status: status, SubmittedAt: row.SubmittedAt, PublishedAt: published,
			ReviewComment: comment, ReviewedBy: &p.UserID, Held: held,
		}); err != nil {
			return err
		}
		if err := s.syncProject(ctx, &row); err != nil {
			return err
		}
		return s.Audit.Record(ctx, audit.Event{
			Action: action, TargetType: "project", TargetID: projectID.String(), Metadata: map[string]any{"version": row.Version},
		})
	})
	if db.IsNoRows(err) {
		return apigen.ShowcaseEntry{}, errNotFound
	}
	if err != nil {
		if _, ok := apperr.As(err); ok {
			return apigen.ShowcaseEntry{}, err
		}
		return apigen.ShowcaseEntry{}, fmt.Errorf("showcase: review: %w", err)
	}
	return entryAPI(ctx, &row), nil
}

// translit spells Kazakh and Russian letters in Latin for page addresses.
var translit = map[rune]string{
	'а': "a", 'ә': "a", 'б': "b", 'в': "v", 'г': "g", 'ғ': "g", 'д': "d", 'е': "e", 'ё': "e", 'ж': "zh", 'з': "z",
	'и': "i", 'й': "i", 'і': "i", 'к': "k", 'қ': "q", 'л': "l", 'м': "m", 'н': "n", 'ң': "n", 'о': "o", 'ө': "o",
	'п': "p", 'р': "r", 'с': "s", 'т': "t", 'у': "u", 'ұ': "u", 'ү': "u", 'ф': "f", 'х': "h", 'һ': "h", 'ц': "ts",
	'ч': "ch", 'ш': "sh", 'щ': "sch", 'ъ': "", 'ы': "y", 'ь': "", 'э': "e", 'ю': "yu", 'я': "ya",
}

// Slugify makes a page address from a title: Latin letters, digits and
// hyphens, 3 to 60 characters.
func Slugify(title string) string {
	var b strings.Builder
	dash := false
	for _, r := range strings.ToLower(title) {
		var part string
		switch {
		case r >= 'a' && r <= 'z' || r >= '0' && r <= '9':
			part = string(r)
		case translit[r] != "" || r == 'ъ' || r == 'ь':
			part = translit[r]
		case unicode.IsLetter(r) || unicode.IsDigit(r):
			continue
		default:
			if b.Len() > 0 && !dash {
				b.WriteByte('-')
				dash = true
			}
			continue
		}
		if part != "" {
			b.WriteString(part)
			dash = false
		}
	}
	slug := strings.Trim(b.String(), "-")
	if len(slug) > 60 {
		slug = strings.TrimRight(slug[:60], "-")
	}
	for len(slug) < 3 {
		slug += "-project"
		slug = strings.Trim(slug, "-")
	}
	return slug
}

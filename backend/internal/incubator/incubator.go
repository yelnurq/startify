// Package incubator guides a team through the 14 steps of a project: the
// answers with the history of every field, completion and approval, the AI
// review of a step, the AI mentor, comments, presence and the generated
// pitch materials.
package incubator

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"slices"
	"strconv"
	"strings"
	"time"

	"github.com/google/uuid"
	"github.com/redis/go-redis/v9"

	"github.com/yelnurq/startify/backend/internal/ai"
	"github.com/yelnurq/startify/backend/internal/apigen"
	"github.com/yelnurq/startify/backend/internal/audit"
	"github.com/yelnurq/startify/backend/internal/files"
	"github.com/yelnurq/startify/backend/internal/incubator/incubatordb"
	"github.com/yelnurq/startify/backend/internal/knowledge"
	"github.com/yelnurq/startify/backend/internal/platform/apperr"
	"github.com/yelnurq/startify/backend/internal/platform/db"
	"github.com/yelnurq/startify/backend/internal/platform/httpx"
	"github.com/yelnurq/startify/backend/internal/platform/jobs"
	"github.com/yelnurq/startify/backend/internal/platform/pagination"
	"github.com/yelnurq/startify/backend/internal/platform/ratelimit"
	"github.com/yelnurq/startify/backend/internal/platform/tenancy"
	"github.com/yelnurq/startify/backend/internal/platform/validate"
	"github.com/yelnurq/startify/backend/internal/project"
	"github.com/yelnurq/startify/backend/internal/tenant"
	"github.com/yelnurq/startify/backend/internal/user"
)

// Step statuses.
const (
	StatusNotStarted = "not_started"
	StatusInProgress = "in_progress"
	StatusCompleted  = "completed"
	StatusApproved   = "approved"
)

// foldWindow joins autosaves of one author into one revision.
const foldWindow = 3 * time.Minute

var (
	errStepNotFound     = apperr.NotFound("incubator.step_not_found", "the step does not exist")
	errRevisionNotFound = apperr.NotFound("incubator.revision_not_found", "the revision does not exist")
	errForbidden        = apperr.Forbidden("rbac.forbidden", "not allowed for your role in the project")
	errStale            = apperr.PreconditionFailed("precondition.failed", "the step was changed since it was loaded")

	limitCheck  = ratelimit.Limit{Rate: 30, Burst: 10, Period: time.Hour}
	limitMentor = ratelimit.Limit{Rate: 60, Burst: 20, Period: time.Hour}
)

// Deps are the collaborators of the service.
type Deps struct {
	DB        *db.DB
	Projects  *project.Service
	Users     *user.Service
	Tenants   *tenant.Service
	Gateway   *ai.Gateway
	Knowledge *knowledge.Service
	Files     *files.Service
	Queue     jobs.Enqueuer
	Audit     *audit.Logger
	Limiter   *ratelimit.Limiter
	Cache     *redis.Client
	// Zone sets the day boundary of activity streaks.
	Zone *time.Location
}

// PageSource gives the public showcase page of a project (the showcase).
type PageSource interface {
	PublicURL(ctx context.Context, projectID uuid.UUID) (string, error)
}

// Service runs the incubator.
type Service struct {
	Deps
	tmpl  *Template
	now   func() time.Time
	pages PageSource
}

// UsePages connects the showcase: passports link to public pages.
func (s *Service) UsePages(p PageSource) { s.pages = p }

// NewService returns the incubator and connects it to the project service.
func NewService(d Deps) *Service {
	if d.Zone == nil {
		d.Zone = time.UTC
	}
	s := &Service{Deps: d, tmpl: Default(), now: time.Now}
	d.Projects.UseProgress(s)
	return s
}

func (s *Service) q(ctx context.Context) *incubatordb.Queries { return incubatordb.New(s.DB.Conn(ctx)) }

// TotalSteps implements project.ProgressSource.
func (s *Service) TotalSteps() int { return len(s.tmpl.Keys()) }

// FirstStep implements project.ProgressSource.
func (s *Service) FirstStep() string { return s.tmpl.Keys()[0] }

func done(status string) bool { return status == StatusCompleted || status == StatusApproved }

// Progress implements project.ProgressSource.
func (s *Service) Progress(ctx context.Context, projectID uuid.UUID) (apigen.ProjectProgress, error) {
	var rows []incubatordb.ProjectStepDatum
	var days []time.Time
	var materials int32
	err := s.DB.ReadTx(ctx, func(ctx context.Context) error {
		q := s.q(ctx)
		var err error
		if rows, err = q.ListSteps(ctx, projectID); err != nil {
			return err
		}
		if days, err = q.ActivityDays(ctx, incubatordb.ActivityDaysParams{ProjectID: projectID, Zone: s.Zone.String()}); err != nil {
			return err
		}
		materials, err = q.CountReadyMaterials(ctx, projectID)
		return err
	})
	if err != nil {
		return apigen.ProjectProgress{}, fmt.Errorf("incubator: progress: %w", err)
	}
	team, err := s.Projects.MemberIDs(ctx, projectID)
	if err != nil {
		return apigen.ProjectProgress{}, err
	}
	status := map[string]string{}
	reviewed := false
	for i := range rows {
		status[rows[i].StepKey] = rows[i].Status
		reviewed = reviewed || rows[i].AiCheck != nil
	}
	out := apigen.ProjectProgress{TotalSteps: s.TotalSteps(), Stages: []apigen.StageProgress{}, Badges: []apigen.Badge{}}
	for _, st := range s.tmpl.Stages {
		sp := apigen.StageProgress{Key: st.Key, Total: len(st.Steps)}
		for _, step := range st.Steps {
			if done(status[step.Key]) {
				sp.Completed++
			} else if out.CurrentStep == "" {
				out.CurrentStep = step.Key
			}
		}
		out.CompletedSteps += sp.Completed
		out.Stages = append(out.Stages, sp)
		if sp.Completed == sp.Total {
			out.Badges = append(out.Badges, apigen.Badge(st.Key+"_stage"))
		}
	}
	if out.CurrentStep == "" {
		keys := s.tmpl.Keys()
		out.CurrentStep = keys[len(keys)-1]
	}
	out.Percent = out.CompletedSteps * 100 / out.TotalSteps
	if reviewed {
		out.Badges = append(out.Badges, apigen.BadgeFirstReview)
	}
	if materials > 0 {
		out.Badges = append(out.Badges, apigen.BadgeMaterialsReady)
	}
	people := 0
	for _, role := range team {
		if role != project.RoleSupervisor {
			people++
		}
	}
	if people >= 3 {
		out.Badges = append(out.Badges, apigen.BadgeTeamOfThree)
	}
	out.StreakDays = streak(days, s.now().In(s.Zone))
	return out, nil
}

// streak counts consecutive days with activity that end today or yesterday.
func streak(days []time.Time, now time.Time) int {
	day := func(t time.Time) string { return t.Format(time.DateOnly) }
	cursor := time.Date(now.Year(), now.Month(), now.Day(), 0, 0, 0, 0, time.UTC)
	set := map[string]bool{}
	for _, d := range days {
		set[day(d)] = true
	}
	if !set[day(cursor)] {
		cursor = cursor.AddDate(0, 0, -1)
	}
	n := 0
	for set[day(cursor)] {
		n++
		cursor = cursor.AddDate(0, 0, -1)
	}
	return n
}

// updateProgress stores the denormalised progress of the project (inside
// the transaction of ctx).
func (s *Service) updateProgress(ctx context.Context, projectID uuid.UUID) error {
	rows, err := s.q(ctx).ListSteps(ctx, projectID)
	if err != nil {
		return err
	}
	status := map[string]string{}
	for i := range rows {
		status[rows[i].StepKey] = rows[i].Status
	}
	completed, current := 0, ""
	for _, key := range s.tmpl.Keys() {
		if done(status[key]) {
			completed++
		} else if current == "" {
			current = key
		}
	}
	if current == "" {
		keys := s.tmpl.Keys()
		current = keys[len(keys)-1]
	}
	return s.Projects.SetProgress(ctx, projectID, completed, current)
}

// locale is the language of AI answers: the interface language of the
// request, else the university's.
func locale(ctx context.Context) string {
	if l := httpx.Locale(ctx); l != "" {
		return l
	}
	if t, ok := tenancy.FromContext(ctx); ok && t.Locale != "" {
		return t.Locale
	}
	return "ru"
}

func (s *Service) step(key string) (*Step, error) {
	st, ok := s.tmpl.Step(key)
	if !ok {
		return nil, errStepNotFound
	}
	return st, nil
}

// ETag is the version of a step.
func ETag(version int32) string { return `"` + strconv.Itoa(int(version)) + `"` }

func matches(ifMatch string, version int32) bool {
	return strings.TrimPrefix(strings.TrimSpace(ifMatch), "W/") == ETag(version)
}

func decodeValues(raw json.RawMessage) Values {
	v := Values{}
	if len(raw) > 0 {
		_ = json.Unmarshal(raw, &v)
	}
	return v
}

// stepAPI converts a stored step (nil for one never saved).
func (s *Service) stepAPI(ctx context.Context, st *Step, row *incubatordb.ProjectStepDatum) (apigen.Step, error) {
	out := apigen.Step{Key: st.Key, Status: StatusNotStarted, Values: apigen.StepValues{}, Missing: []apigen.StepGap{}}
	values := Values{}
	if row != nil {
		values = decodeValues(row.Data)
		out.Status, out.Version, out.CompletedAt, out.ApprovalComment = apigen.StepStatus(row.Status), int(row.Version), row.CompletedAt, row.ApprovalComment
		if !row.UpdatedAt.IsZero() && row.Version > 0 {
			out.UpdatedAt = &row.UpdatedAt
		}
		if row.AiCheck != nil {
			var check apigen.StepCheck
			if json.Unmarshal(row.AiCheck, &check) == nil {
				out.Check = &check
			}
		}
		var people []uuid.UUID
		for _, id := range []*uuid.UUID{row.UpdatedBy, row.ApprovedBy} {
			if id != nil {
				people = append(people, *id)
			}
		}
		briefs, err := s.Users.Briefs(ctx, people)
		if err != nil {
			return out, err
		}
		if row.UpdatedBy != nil && row.Version > 0 {
			b := briefs[*row.UpdatedBy]
			out.UpdatedBy = &b
		}
		if row.ApprovedBy != nil {
			b := briefs[*row.ApprovedBy]
			out.ApprovedBy = &b
		}
	}
	for k, v := range values {
		out.Values[k] = v
	}
	for _, g := range st.Missing(values) {
		gap := apigen.StepGap{Field: g.Field, Code: apigen.StepGapCode(g.Code)}
		if g.Count > 0 {
			c := g.Count
			gap.Count = &c
		}
		out.Missing = append(out.Missing, gap)
	}
	return out, nil
}

// Steps lists the state of every step with the project's progress.
func (s *Service) Steps(ctx context.Context, projectID uuid.UUID) (apigen.StepList, error) {
	if _, err := s.Projects.Access(ctx, projectID); err != nil {
		return apigen.StepList{}, err
	}
	var rows []incubatordb.ProjectStepDatum
	var open []incubatordb.OpenCommentsRow
	err := s.DB.ReadTx(ctx, func(ctx context.Context) error {
		var err error
		if rows, err = s.q(ctx).ListSteps(ctx, projectID); err != nil {
			return err
		}
		open, err = s.q(ctx).OpenComments(ctx, projectID)
		return err
	})
	if err != nil {
		return apigen.StepList{}, fmt.Errorf("incubator: steps: %w", err)
	}
	byKey := map[string]*incubatordb.ProjectStepDatum{}
	var people []uuid.UUID
	for i := range rows {
		byKey[rows[i].StepKey] = &rows[i]
		if rows[i].UpdatedBy != nil {
			people = append(people, *rows[i].UpdatedBy)
		}
	}
	comments := map[string]int{}
	for _, o := range open {
		comments[o.StepKey] = int(o.Open)
	}
	briefs, err := s.Users.Briefs(ctx, people)
	if err != nil {
		return apigen.StepList{}, err
	}
	out := apigen.StepList{Items: []apigen.StepSummary{}}
	for _, key := range s.tmpl.Keys() {
		st, _ := s.tmpl.Step(key)
		sum := apigen.StepSummary{Key: key, Status: StatusNotStarted, OpenComments: comments[key]}
		values := Values{}
		if r := byKey[key]; r != nil {
			values = decodeValues(r.Data)
			sum.Status, sum.Version = apigen.StepStatus(r.Status), int(r.Version)
			if r.Version > 0 {
				sum.UpdatedAt = &r.UpdatedAt
				if r.UpdatedBy != nil {
					b := briefs[*r.UpdatedBy]
					sum.UpdatedBy = &b
				}
			}
		}
		sum.Filled, sum.Total = st.Filled(values)
		out.Items = append(out.Items, sum)
	}
	if out.Progress, err = s.Progress(ctx, projectID); err != nil {
		return out, err
	}
	return out, nil
}

// Step returns the answers of a step.
func (s *Service) Step(ctx context.Context, projectID uuid.UUID, key string) (apigen.Step, error) {
	if _, err := s.Projects.Access(ctx, projectID); err != nil {
		return apigen.Step{}, err
	}
	st, err := s.step(key)
	if err != nil {
		return apigen.Step{}, err
	}
	var row *incubatordb.ProjectStepDatum
	err = s.DB.ReadTx(ctx, func(ctx context.Context) error {
		r, err := s.q(ctx).GetStep(ctx, incubatordb.GetStepParams{ProjectID: projectID, StepKey: key})
		if db.IsNoRows(err) {
			return nil
		}
		row = &r
		return err
	})
	if err != nil {
		return apigen.Step{}, fmt.Errorf("incubator: step: %w", err)
	}
	return s.stepAPI(ctx, st, row)
}

// Save stores answers of a step; ifMatch must name the current version.
func (s *Service) Save(ctx context.Context, projectID uuid.UUID, key string, values map[string]any, source, ifMatch string) (apigen.Step, error) {
	a, err := s.Projects.Access(ctx, projectID)
	if err != nil {
		return apigen.Step{}, err
	}
	if err := a.RequireEdit(); err != nil {
		return apigen.Step{}, err
	}
	st, err := s.step(key)
	if err != nil {
		return apigen.Step{}, err
	}
	raw := make(map[string]json.RawMessage, len(values))
	for k, v := range values {
		b, err := json.Marshal(v)
		if err != nil {
			return apigen.Step{}, apperr.Validation(apperr.FieldError{Field: "fields." + k, Code: validate.CodeInvalid})
		}
		raw[k] = b
	}
	clean, err := st.Sanitize(raw)
	if err != nil {
		return apigen.Step{}, err
	}
	row, err := s.save(ctx, a, st, clean, source, ifMatch)
	if err != nil {
		return apigen.Step{}, err
	}
	return s.stepAPI(ctx, st, &row)
}

func (s *Service) save(ctx context.Context, a project.Access, st *Step, changes Values, source, ifMatch string) (incubatordb.ProjectStepDatum, error) {
	userID := a.Principal.UserID
	var row incubatordb.ProjectStepDatum
	err := s.DB.Tx(ctx, func(ctx context.Context) error {
		q := s.q(ctx)
		if err := q.EnsureStep(ctx, incubatordb.EnsureStepParams{ProjectID: a.Project.ID, StepKey: st.Key}); err != nil {
			return err
		}
		current, err := q.LockStep(ctx, incubatordb.LockStepParams{ProjectID: a.Project.ID, StepKey: st.Key})
		if err != nil {
			return err
		}
		if !matches(ifMatch, current.Version) {
			return errStale
		}
		values := decodeValues(current.Data)
		changed := map[string]json.RawMessage{}
		for k, v := range changes {
			next, err := json.Marshal(v)
			if err != nil {
				return err
			}
			prev, _ := json.Marshal(values[k])
			if _, had := values[k]; !had || !bytes.Equal(prev, next) {
				changed[k] = next
				values[k] = v
			}
		}
		if len(changed) == 0 {
			row = current
			return nil
		}
		status := current.Status
		switch {
		case status == StatusNotStarted:
			status = StatusInProgress
		case status == StatusCompleted && len(st.Missing(values)) > 0:
			status = StatusInProgress
		}
		data, err := json.Marshal(values)
		if err != nil {
			return err
		}
		if row, err = q.SaveStep(ctx, incubatordb.SaveStepParams{
			ProjectID: a.Project.ID, StepKey: st.Key, Data: data, Status: status, UpdatedBy: &userID,
		}); err != nil {
			return err
		}
		keys := make([]string, 0, len(changed))
		for k := range changed {
			keys = append(keys, k)
		}
		slices.Sort(keys)
		for _, k := range keys {
			if err := s.revision(ctx, q, a.Project.ID, st.Key, k, changed[k], row.Version, userID, source); err != nil {
				return err
			}
		}
		return s.updateProgress(ctx, a.Project.ID)
	})
	if err != nil {
		return row, fmt.Errorf("incubator: save: %w", err)
	}
	return row, nil
}

// revision records a field's new value, folding quick autosaves of the same
// author into the previous revision.
func (s *Service) revision(ctx context.Context, q *incubatordb.Queries, projectID uuid.UUID, step, field string, value json.RawMessage, version int32, author uuid.UUID, source string) error {
	last, err := q.LastRevision(ctx, incubatordb.LastRevisionParams{ProjectID: projectID, StepKey: step, FieldKey: field})
	if err != nil && !db.IsNoRows(err) {
		return err
	}
	if err == nil && source == "edit" && last.Source == "edit" && last.AuthorID != nil && *last.AuthorID == author &&
		s.now().Sub(last.UpdatedAt) < foldWindow {
		return q.FoldRevision(ctx, incubatordb.FoldRevisionParams{ID: last.ID, Value: value, Version: version})
	}
	return q.InsertRevision(ctx, incubatordb.InsertRevisionParams{
		ProjectID: projectID, StepKey: step, FieldKey: field, Value: value, Version: version, AuthorID: &author, Source: source,
	})
}

// Complete marks a step done when it meets the minimum.
func (s *Service) Complete(ctx context.Context, projectID uuid.UUID, key string) (apigen.Step, error) {
	a, err := s.Projects.Access(ctx, projectID)
	if err != nil {
		return apigen.Step{}, err
	}
	if err := a.RequireEdit(); err != nil {
		return apigen.Step{}, err
	}
	st, err := s.step(key)
	if err != nil {
		return apigen.Step{}, err
	}
	userID := a.Principal.UserID
	var row incubatordb.ProjectStepDatum
	err = s.DB.Tx(ctx, func(ctx context.Context) error {
		q := s.q(ctx)
		if err := q.EnsureStep(ctx, incubatordb.EnsureStepParams{ProjectID: projectID, StepKey: key}); err != nil {
			return err
		}
		current, err := q.LockStep(ctx, incubatordb.LockStepParams{ProjectID: projectID, StepKey: key})
		if err != nil {
			return err
		}
		if done(current.Status) {
			row = current
			return nil
		}
		if gaps := st.Missing(decodeValues(current.Data)); len(gaps) > 0 {
			fields := make([]apperr.FieldError, len(gaps))
			for i, g := range gaps {
				fields[i] = apperr.FieldError{Field: "fields." + g.Field, Code: g.Code}
				if g.Count > 0 {
					fields[i].Params = map[string]any{"count": g.Count}
				}
			}
			e := apperr.Validation(fields...)
			e.Code, e.Message = "incubator.step_incomplete", "the step does not meet its minimum yet"
			return e
		}
		if row, err = q.CompleteStep(ctx, incubatordb.CompleteStepParams{ProjectID: projectID, StepKey: key, UserID: &userID}); err != nil {
			return err
		}
		if err := s.updateProgress(ctx, projectID); err != nil {
			return err
		}
		return s.Audit.Record(ctx, audit.Event{
			Action: "incubator.step_completed", TargetType: "project", TargetID: projectID.String(), Metadata: map[string]any{"step": key},
		})
	})
	if err != nil {
		return apigen.Step{}, fmt.Errorf("incubator: complete: %w", err)
	}
	return s.stepAPI(ctx, st, &row)
}

// Approve lets a supervisor or an innovation manager accept a step.
func (s *Service) Approve(ctx context.Context, projectID uuid.UUID, key, comment string) (apigen.Step, error) {
	a, err := s.Projects.Access(ctx, projectID)
	if err != nil {
		return apigen.Step{}, err
	}
	if !a.CanApprove() {
		return apigen.Step{}, errForbidden
	}
	st, err := s.step(key)
	if err != nil {
		return apigen.Step{}, err
	}
	var v validate.Errors
	comment = v.Paragraph("comment", comment, 1, 500)
	if err := v.Err(); err != nil {
		return apigen.Step{}, err
	}
	userID := a.Principal.UserID
	var row incubatordb.ProjectStepDatum
	err = s.DB.Tx(ctx, func(ctx context.Context) error {
		q := s.q(ctx)
		if err := q.EnsureStep(ctx, incubatordb.EnsureStepParams{ProjectID: projectID, StepKey: key}); err != nil {
			return err
		}
		var err error
		if row, err = q.ApproveStep(ctx, incubatordb.ApproveStepParams{ProjectID: projectID, StepKey: key, UserID: &userID, Comment: &comment}); err != nil {
			return err
		}
		if err := s.updateProgress(ctx, projectID); err != nil {
			return err
		}
		return s.Audit.Record(ctx, audit.Event{
			Action: "incubator.step_approved", TargetType: "project", TargetID: projectID.String(), Metadata: map[string]any{"step": key},
		})
	})
	if err != nil {
		return apigen.Step{}, fmt.Errorf("incubator: approve: %w", err)
	}
	return s.stepAPI(ctx, st, &row)
}

type revisionCursor struct {
	ID uuid.UUID `json:"i"`
}

// Revisions lists the history of a step, optionally of one field.
func (s *Service) Revisions(ctx context.Context, projectID uuid.UUID, key string, params apigen.ListStepRevisionsParams) (apigen.RevisionPage, error) {
	if _, err := s.Projects.Access(ctx, projectID); err != nil {
		return apigen.RevisionPage{}, err
	}
	st, err := s.step(key)
	if err != nil {
		return apigen.RevisionPage{}, err
	}
	args := incubatordb.ListRevisionsParams{ProjectID: projectID, StepKey: key}
	if params.Field != nil {
		if st.Field(*params.Field) == nil {
			return apigen.RevisionPage{}, apperr.BadRequest("incubator.field_unknown", "the step has no such field")
		}
		args.FieldKey = params.Field
	}
	limit, err := pagination.Limit(params.Limit)
	if err != nil {
		return apigen.RevisionPage{}, err
	}
	args.RowLimit = int32(limit + 1) //nolint:gosec // bounded by MaxLimit
	if params.Cursor != nil {
		var c revisionCursor
		if err := pagination.Decode(*params.Cursor, &c); err != nil {
			return apigen.RevisionPage{}, err
		}
		args.BeforeID = &c.ID
	}
	var rows []incubatordb.ProjectRevision
	err = s.DB.ReadTx(ctx, func(ctx context.Context) error {
		var err error
		rows, err = s.q(ctx).ListRevisions(ctx, args)
		return err
	})
	if err != nil {
		return apigen.RevisionPage{}, fmt.Errorf("incubator: revisions: %w", err)
	}
	page := apigen.RevisionPage{Items: []apigen.Revision{}}
	if len(rows) > limit {
		rows = rows[:limit]
		next := pagination.Encode(revisionCursor{ID: rows[len(rows)-1].ID})
		page.NextCursor = &next
	}
	var people []uuid.UUID
	for i := range rows {
		if rows[i].AuthorID != nil {
			people = append(people, *rows[i].AuthorID)
		}
	}
	briefs, err := s.Users.Briefs(ctx, people)
	if err != nil {
		return page, err
	}
	for i := range rows {
		r := &rows[i]
		var value any
		_ = json.Unmarshal(r.Value, &value)
		rev := apigen.Revision{
			Id: r.ID, Field: r.FieldKey, Value: value, Version: int(r.Version), Source: apigen.RevisionSource(r.Source),
			CreatedAt: r.CreatedAt, UpdatedAt: r.UpdatedAt,
		}
		if r.AuthorID != nil {
			b := briefs[*r.AuthorID]
			rev.Author = &b
		}
		page.Items = append(page.Items, rev)
	}
	return page, nil
}

// Restore puts an earlier value of a field back as a new revision.
func (s *Service) Restore(ctx context.Context, projectID uuid.UUID, key string, revisionID uuid.UUID, ifMatch string) (apigen.Step, error) {
	a, err := s.Projects.Access(ctx, projectID)
	if err != nil {
		return apigen.Step{}, err
	}
	if err := a.RequireEdit(); err != nil {
		return apigen.Step{}, err
	}
	st, err := s.step(key)
	if err != nil {
		return apigen.Step{}, err
	}
	var rev incubatordb.ProjectRevision
	err = s.DB.ReadTx(ctx, func(ctx context.Context) error {
		var err error
		rev, err = s.q(ctx).GetRevision(ctx, incubatordb.GetRevisionParams{ID: revisionID, ProjectID: projectID, StepKey: key})
		return err
	})
	if db.IsNoRows(err) {
		return apigen.Step{}, errRevisionNotFound
	}
	if err != nil {
		return apigen.Step{}, fmt.Errorf("incubator: restore: %w", err)
	}
	clean, err := st.Sanitize(map[string]json.RawMessage{rev.FieldKey: rev.Value})
	if err != nil {
		return apigen.Step{}, err
	}
	row, err := s.save(ctx, a, st, clean, "restore", ifMatch)
	if err != nil {
		return apigen.Step{}, err
	}
	return s.stepAPI(ctx, st, &row)
}

// TemplateAPI is the template as the API returns it.
func (s *Service) TemplateAPI() apigen.IncubatorTemplate {
	out := apigen.IncubatorTemplate{Version: s.tmpl.Version, Stages: []apigen.TemplateStage{}}
	for _, stage := range s.tmpl.Stages {
		ts := apigen.TemplateStage{Key: stage.Key, Title: stage.Title.API(), Steps: []apigen.TemplateStep{}}
		for i := range stage.Steps {
			st := &stage.Steps[i]
			step := apigen.TemplateStep{
				Key: st.Key, Number: st.Number(), Stage: stage.Key, Title: st.Title.API(), Goal: st.Goal.API(),
				Method: st.Method.API(), Criteria: []apigen.TemplateCriterion{}, Fields: fieldsAPI(st.Fields),
			}
			for _, c := range st.Criteria {
				step.Criteria = append(step.Criteria, apigen.TemplateCriterion{Fields: c.Fields, Text: c.Text.API()})
			}
			ts.Steps = append(ts.Steps, step)
		}
		out.Stages = append(out.Stages, ts)
	}
	return out
}

func fieldsAPI(fields []Field) []apigen.TemplateField {
	out := make([]apigen.TemplateField, len(fields))
	for i := range fields {
		f := &fields[i]
		tf := apigen.TemplateField{Key: f.Key, Type: apigen.FieldType(f.Type), Required: f.Required, Label: f.Label.API()}
		if len(f.Hint) > 0 {
			h := f.Hint.API()
			tf.Hint = &h
		}
		if len(f.Placeholder) > 0 {
			p := f.Placeholder.API()
			tf.Placeholder = &p
		}
		if f.Unit != "" {
			u := apigen.FieldUnit(f.Unit)
			tf.Unit = &u
		}
		switch f.Type {
		case TypeText, TypeTextarea, TypeURL:
			maxLen := f.maxLength()
			tf.MaxLength = &maxLen
			if f.MinLength > 0 {
				minLen := f.MinLength
				tf.MinLength = &minLen
			}
		case TypeList, TypeTable:
			maxItems := f.maxItems()
			tf.MaxItems = &maxItems
			if f.Type == TypeList {
				maxLen := f.maxLength()
				tf.MaxLength = &maxLen
			}
			if f.MinItems > 0 {
				minItems := f.MinItems
				tf.MinItems = &minItems
			}
		}
		if len(f.Options) > 0 {
			opts := make([]apigen.TemplateOption, len(f.Options))
			for j, o := range f.Options {
				opts[j] = apigen.TemplateOption{Value: o.Value, Label: o.Label.API()}
			}
			tf.Options = &opts
		}
		if len(f.Columns) > 0 {
			cols := fieldsAPI(f.Columns)
			tf.Columns = &cols
		}
		out[i] = tf
	}
	return out
}

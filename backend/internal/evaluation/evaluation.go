package evaluation

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"log/slog"
	"strconv"
	"strings"
	"time"

	"github.com/google/uuid"
	"github.com/riverqueue/river"

	"github.com/yelnurq/startify/backend/internal/ai"
	"github.com/yelnurq/startify/backend/internal/ai/llm"
	"github.com/yelnurq/startify/backend/internal/ai/prompts"
	"github.com/yelnurq/startify/backend/internal/apigen"
	"github.com/yelnurq/startify/backend/internal/audit"
	"github.com/yelnurq/startify/backend/internal/evaluation/evaluationdb"
	"github.com/yelnurq/startify/backend/internal/incubator"
	"github.com/yelnurq/startify/backend/internal/platform/apperr"
	"github.com/yelnurq/startify/backend/internal/platform/authz"
	"github.com/yelnurq/startify/backend/internal/platform/db"
	"github.com/yelnurq/startify/backend/internal/platform/gotenberg"
	"github.com/yelnurq/startify/backend/internal/platform/httpx"
	"github.com/yelnurq/startify/backend/internal/platform/jobs"
	"github.com/yelnurq/startify/backend/internal/platform/ratelimit"
	"github.com/yelnurq/startify/backend/internal/platform/tenancy"
	"github.com/yelnurq/startify/backend/internal/platform/validate"
	"github.com/yelnurq/startify/backend/internal/project"
	"github.com/yelnurq/startify/backend/internal/tenant"
	"github.com/yelnurq/startify/backend/internal/user"
)

// Evaluation statuses.
const (
	StatusPending = "pending"
	StatusRunning = "running"
	StatusReady   = "ready"
	StatusFailed  = "failed"
)

var (
	errNotFound  = apperr.NotFound("evaluation.not_found", "the evaluation does not exist")
	errNotReady  = apperr.Conflict("evaluation.not_ready", "the evaluation is not ready")
	errForbidden = apperr.Forbidden("rbac.forbidden", "not allowed for your role in the project")

	limitRequests = ratelimit.Limit{Rate: 5, Burst: 5, Period: 24 * time.Hour}

	rubricEditors = []authz.Role{authz.RoleTenantAdmin, authz.RoleInnovationManager}
)

// Deps are the collaborators of the service.
type Deps struct {
	DB        *db.DB
	Projects  *project.Service
	Users     *user.Service
	Tenants   *tenant.Service
	Queue     jobs.Enqueuer
	Audit     *audit.Logger
	Limiter   *ratelimit.Limiter
	Gotenberg *gotenberg.Client
}

// Service evaluates projects.
type Service struct {
	Deps
	now func() time.Time
}

// NewService returns the evaluation service and shows the latest scores on
// project cards.
func NewService(d Deps) *Service {
	s := &Service{Deps: d, now: time.Now}
	d.Projects.UseEvaluations(s)
	return s
}

func (s *Service) q(ctx context.Context) *evaluationdb.Queries {
	return evaluationdb.New(s.DB.Conn(ctx))
}

// weighted is a rubric line stored with an evaluation.
type weighted struct {
	Key    string `json:"key"`
	Weight int    `json:"weight"`
}

func snapshot(r Rubric) json.RawMessage {
	out := make([]weighted, len(r))
	for i, c := range r {
		out[i] = weighted{c.Key, c.Weight}
	}
	raw, _ := json.Marshal(out)
	return raw
}

// fromSnapshot restores the rubric an evaluation used.
func fromSnapshot(raw json.RawMessage) Rubric {
	var lines []weighted
	_ = json.Unmarshal(raw, &lines)
	weights := map[string]int{}
	for _, l := range lines {
		weights[l.Key] = l.Weight
	}
	return WithWeights(weights)
}

// rubric loads the university's rubric.
func (s *Service) rubric(ctx context.Context) (Rubric, *evaluationdb.EvaluationRubric, error) {
	var row *evaluationdb.EvaluationRubric
	err := s.DB.ReadTx(ctx, func(ctx context.Context) error {
		r, err := s.q(ctx).GetRubric(ctx)
		if db.IsNoRows(err) {
			return nil
		}
		row = &r
		return err
	})
	if err != nil {
		return nil, nil, fmt.Errorf("evaluation: rubric: %w", err)
	}
	if row == nil {
		return DefaultRubric, nil, nil
	}
	var weights map[string]int
	_ = json.Unmarshal(row.Weights, &weights)
	// Stored weights were checked on save; a rubric that no longer fits the
	// criteria falls back to the default.
	if r := WithWeights(weights); r.Validate() == nil {
		return r, row, nil
	}
	return DefaultRubric, row, nil
}

func rubricETag(row *evaluationdb.EvaluationRubric) string {
	if row == nil {
		return `"0"`
	}
	return `"` + strconv.FormatInt(row.UpdatedAt.UnixNano(), 36) + `"`
}

func rubricAPI(r Rubric, customized bool) apigen.Rubric {
	out := apigen.Rubric{Customized: customized, Criteria: make([]apigen.RubricCriterion, len(r))}
	for i, c := range r {
		out.Criteria[i] = apigen.RubricCriterion{Key: c.Key, Title: c.Title.API(), Weight: c.Weight}
	}
	return out
}

// Rubric returns the university's rubric with its ETag.
func (s *Service) Rubric(ctx context.Context) (apigen.Rubric, string, error) {
	if _, err := authz.RequireRole(ctx, rubricEditors...); err != nil {
		return apigen.Rubric{}, "", err
	}
	r, row, err := s.rubric(ctx)
	if err != nil {
		return apigen.Rubric{}, "", err
	}
	return rubricAPI(r, r.customized()), rubricETag(row), nil
}

// UpdateRubric changes the weights; they must cover every criterion and add
// up to 100.
func (s *Service) UpdateRubric(ctx context.Context, weights map[string]int, ifMatch *string) (apigen.Rubric, string, error) {
	p, err := authz.RequireRole(ctx, rubricEditors...)
	if err != nil {
		return apigen.Rubric{}, "", err
	}
	var v validate.Errors
	sum := 0
	for key, w := range weights {
		known := false
		for _, c := range DefaultRubric {
			known = known || c.Key == key
		}
		if !known || w < 0 || w > 100 {
			v.Add("weights."+key, validate.CodeInvalid, nil)
		}
		sum += w
	}
	for _, c := range DefaultRubric {
		if _, ok := weights[c.Key]; !ok {
			v.Add("weights."+c.Key, validate.CodeRequired, nil)
		}
	}
	if sum != 100 {
		v.Add("weights", "sum", map[string]any{"expected": 100, "actual": sum})
	}
	if err := v.Err(); err != nil {
		return apigen.Rubric{}, "", err
	}
	raw, err := json.Marshal(weights)
	if err != nil {
		return apigen.Rubric{}, "", err
	}
	var row evaluationdb.EvaluationRubric
	err = s.DB.Tx(ctx, func(ctx context.Context) error {
		q := s.q(ctx)
		if ifMatch != nil {
			current, err := q.GetRubric(ctx)
			var row *evaluationdb.EvaluationRubric
			switch {
			case err == nil:
				row = &current
			case !db.IsNoRows(err):
				return err
			}
			if strings.TrimPrefix(*ifMatch, "W/") != rubricETag(row) {
				return apperr.PreconditionFailed("precondition.failed", "the rubric was changed since it was loaded")
			}
		}
		var err error
		if row, err = q.UpsertRubric(ctx, evaluationdb.UpsertRubricParams{Weights: raw, UpdatedBy: &p.UserID}); err != nil {
			return err
		}
		return s.Audit.Record(ctx, audit.Event{Action: "evaluation.rubric_changed", TargetType: "rubric", Metadata: map[string]any{"weights": weights}})
	})
	if err != nil {
		return apigen.Rubric{}, "", fmt.Errorf("evaluation: update rubric: %w", err)
	}
	saved := WithWeights(weights)
	return rubricAPI(saved, saved.customized()), rubricETag(&row), nil
}

// RunArgs schedules an evaluation.
type RunArgs struct {
	TenantID     uuid.UUID             `json:"tenant_id"`
	ProjectID    uuid.UUID             `json:"project_id"`
	EvaluationID uuid.UUID             `json:"evaluation_id"`
	RequestedBy  uuid.UUID             `json:"requested_by"`
	Card         incubator.ProjectCard `json:"card"`
}

// Kind implements river.JobArgs.
func (RunArgs) Kind() string { return "evaluation.run" }

// InsertOpts retries while the model is unavailable.
func (RunArgs) InsertOpts() river.InsertOpts { return river.InsertOpts{MaxAttempts: 3} }

func locale(ctx context.Context) string {
	if l := httpx.Locale(ctx); l != "" {
		return l
	}
	if t, ok := tenancy.FromContext(ctx); ok && t.Locale != "" {
		return t.Locale
	}
	return "ru"
}

// Request schedules an evaluation of the project with the current rubric.
func (s *Service) Request(ctx context.Context, projectID uuid.UUID) (apigen.Evaluation, error) {
	a, err := s.Projects.Access(ctx, projectID)
	if err != nil {
		return apigen.Evaluation{}, err
	}
	if a.Archived() {
		return apigen.Evaluation{}, apperr.Conflict("project.archived", "the project is archived")
	}
	if !a.Team() && !a.CanApprove() {
		return apigen.Evaluation{}, errForbidden
	}
	t, err := tenancy.Require(ctx)
	if err != nil {
		return apigen.Evaluation{}, err
	}
	r, _, err := s.rubric(ctx)
	if err != nil {
		return apigen.Evaluation{}, err
	}
	card := incubator.ProjectCard{Title: a.Project.Title, Tagline: a.Project.Tagline, Industry: a.Project.Industry}
	_, versions, err := incubator.ProjectContext(ctx, s.DB, projectID, card)
	if err != nil {
		return apigen.Evaluation{}, err
	}
	if len(versions) == 0 {
		return apigen.Evaluation{}, apperr.New(apperr.KindValidation, "evaluation.project_empty", "answer at least one step before the evaluation")
	}
	requester := a.Principal.UserID
	var row evaluationdb.Evaluation
	err = s.DB.Tx(ctx, func(ctx context.Context) error {
		// One evaluation at a time per project.
		if _, err := s.DB.Conn(ctx).Exec(ctx, "SELECT pg_advisory_xact_lock(hashtextextended($1, 0))", "evaluation:"+projectID.String()); err != nil {
			return err
		}
		q := s.q(ctx)
		existing, err := q.ListEvaluations(ctx, projectID)
		if err != nil {
			return err
		}
		for i := range existing {
			if existing[i].Status == StatusPending || existing[i].Status == StatusRunning {
				return apperr.Conflict("evaluation.in_progress", "an evaluation is already running").
					WithParams(map[string]any{"evaluation_id": existing[i].ID})
			}
		}
		if err := s.Limiter.Allow(ctx, "evaluation:"+projectID.String(), limitRequests, "evaluation.too_many_requests"); err != nil {
			return err
		}
		stepVersions, _ := json.Marshal(versions)
		if row, err = q.CreateEvaluation(ctx, evaluationdb.CreateEvaluationParams{
			ProjectID: projectID, Locale: locale(ctx), Rubric: snapshot(r), StepVersions: stepVersions, RequestedBy: &requester,
		}); err != nil {
			return err
		}
		if err := s.Queue.Enqueue(ctx, RunArgs{TenantID: t.ID, ProjectID: projectID, EvaluationID: row.ID, RequestedBy: requester, Card: card}, nil); err != nil {
			return err
		}
		return s.Audit.Record(ctx, audit.Event{Action: "evaluation.requested", TargetType: "project", TargetID: projectID.String()})
	})
	if err != nil {
		return apigen.Evaluation{}, fmt.Errorf("evaluation: request: %w", err)
	}
	return s.view(ctx, a, &row)
}

// List returns the evaluations of a project, newest first.
func (s *Service) List(ctx context.Context, projectID uuid.UUID) (apigen.EvaluationList, error) {
	if _, err := s.Projects.Access(ctx, projectID); err != nil {
		return apigen.EvaluationList{}, err
	}
	out := apigen.EvaluationList{Items: []apigen.EvaluationSummary{}}
	err := s.DB.ReadTx(ctx, func(ctx context.Context) error {
		q := s.q(ctx)
		rows, err := q.ListEvaluations(ctx, projectID)
		if err != nil {
			return err
		}
		ids := make([]uuid.UUID, len(rows))
		for i := range rows {
			ids[i] = rows[i].ID
		}
		counts, err := q.CountOverrides(ctx, ids)
		if err != nil {
			return err
		}
		overridden := map[uuid.UUID]bool{}
		for _, c := range counts {
			overridden[c.EvaluationID] = c.N > 0
		}
		for i := range rows {
			r := &rows[i]
			out.Items = append(out.Items, apigen.EvaluationSummary{
				Id: r.ID, Status: apigen.EvaluationStatus(r.Status), Total: intPtr(r.Total), Level: levelPtr(r.Level),
				Overridden: overridden[r.ID], CreatedAt: r.CreatedAt, CompletedAt: r.CompletedAt,
			})
		}
		return nil
	})
	if err != nil {
		return out, fmt.Errorf("evaluation: list: %w", err)
	}
	return out, nil
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

// Get returns an evaluation with the comparison to the previous one.
func (s *Service) Get(ctx context.Context, projectID, evaluationID uuid.UUID) (apigen.Evaluation, error) {
	a, err := s.Projects.Access(ctx, projectID)
	if err != nil {
		return apigen.Evaluation{}, err
	}
	row, err := s.load(ctx, projectID, evaluationID)
	if err != nil {
		return apigen.Evaluation{}, err
	}
	return s.view(ctx, a, &row)
}

func (s *Service) load(ctx context.Context, projectID, evaluationID uuid.UUID) (evaluationdb.Evaluation, error) {
	var row evaluationdb.Evaluation
	err := s.DB.ReadTx(ctx, func(ctx context.Context) error {
		var err error
		row, err = s.q(ctx).GetEvaluation(ctx, evaluationdb.GetEvaluationParams{ID: evaluationID, ProjectID: projectID})
		return err
	})
	if db.IsNoRows(err) {
		return row, errNotFound
	}
	if err != nil {
		return row, fmt.Errorf("evaluation: get: %w", err)
	}
	return row, nil
}

func decodeList[T any](raw json.RawMessage) []T {
	out := []T{}
	_ = json.Unmarshal(raw, &out)
	return out
}

// view builds the API form of an evaluation.
func (s *Service) view(ctx context.Context, a project.Access, row *evaluationdb.Evaluation) (apigen.Evaluation, error) {
	r := fromSnapshot(row.Rubric)
	out := apigen.Evaluation{
		Id: row.ID, Status: apigen.EvaluationStatus(row.Status), Total: intPtr(row.Total), AiTotal: intPtr(row.AiTotal),
		Level: levelPtr(row.Level), Summary: row.Summary, Hypotheses: decodeList[string](row.Hypotheses),
		Recommendations: decodeList[string](row.Recommendations), RedFlags: []apigen.RedFlag{}, Experiments: []apigen.Experiment{},
		Criteria: []apigen.CriterionScore{}, Overrides: []apigen.ScoreOverrideRecord{}, Provider: row.Provider, Model: row.Model,
		PromptVersion: row.PromptVersion, Locale: apigen.Locale(row.Locale), Error: row.Error, CreatedAt: row.CreatedAt,
		CompletedAt: row.CompletedAt, CanOverride: a.CanApprove() && row.Status == StatusReady,
	}
	for _, f := range decodeList[RedFlag](row.RedFlags) {
		out.RedFlags = append(out.RedFlags, apigen.RedFlag{Title: f.Title, Detail: f.Detail, Severity: apigen.RedFlagSeverity(f.Severity)})
	}
	for _, e := range decodeList[Experiment](row.Experiments) {
		out.Experiments = append(out.Experiments, apigen.Experiment{Title: e.Title, How: e.How, Metric: e.Metric, Success: e.Success, Day: e.Day})
	}
	var scores []evaluationdb.EvaluationCriteriaScore
	var overrides []evaluationdb.EvaluationOverride
	previous := map[string]int{}
	err := s.DB.ReadTx(ctx, func(ctx context.Context) error {
		q := s.q(ctx)
		prev, err := q.PreviousReady(ctx, evaluationdb.PreviousReadyParams{ProjectID: row.ProjectID, BeforeID: row.ID})
		ids := []uuid.UUID{row.ID}
		switch {
		case err == nil:
			out.PreviousTotal = intPtr(prev.Total)
			ids = append(ids, prev.ID)
		case !db.IsNoRows(err):
			return err
		}
		all, err := q.ListScores(ctx, ids)
		if err != nil {
			return err
		}
		for i := range all {
			if all[i].EvaluationID == row.ID {
				scores = append(scores, all[i])
			} else {
				previous[all[i].CriterionKey] = int(all[i].Score)
			}
		}
		overrides, err = q.ListOverrides(ctx, row.ID)
		return err
	})
	if err != nil {
		return out, fmt.Errorf("evaluation: view: %w", err)
	}
	byKey := map[string]*evaluationdb.EvaluationCriteriaScore{}
	for i := range scores {
		byKey[scores[i].CriterionKey] = &scores[i]
	}
	for _, c := range r {
		sc, ok := byKey[c.Key]
		if !ok {
			continue
		}
		cs := apigen.CriterionScore{
			Key: c.Key, Title: c.Title.API(), Weight: c.Weight, Score: int(sc.Score), AiScore: int(sc.AiScore),
			Justification: sc.Justification, Overridden: sc.Score != sc.AiScore,
		}
		if p, ok := previous[c.Key]; ok {
			cs.Previous = &p
		}
		out.Criteria = append(out.Criteria, cs)
	}
	people := []uuid.UUID{}
	if row.RequestedBy != nil {
		people = append(people, *row.RequestedBy)
	}
	for i := range overrides {
		if overrides[i].AuthorID != nil {
			people = append(people, *overrides[i].AuthorID)
		}
	}
	briefs, err := s.Users.Briefs(ctx, people)
	if err != nil {
		return out, err
	}
	if row.RequestedBy != nil {
		b := briefs[*row.RequestedBy]
		out.RequestedBy = &b
	}
	for i := range overrides {
		o := &overrides[i]
		rec := apigen.ScoreOverrideRecord{
			Criterion: o.CriterionKey, From: int(o.OldScore), To: int(o.NewScore), Comment: o.Comment, CreatedAt: o.CreatedAt,
		}
		if o.AuthorID != nil {
			b := briefs[*o.AuthorID]
			rec.Author = &b
		}
		out.Overrides = append(out.Overrides, rec)
	}
	return out, nil
}

// Override changes a criterion score as an expert, with a comment.
func (s *Service) Override(ctx context.Context, projectID, evaluationID uuid.UUID, body apigen.ScoreOverride) (apigen.Evaluation, error) {
	a, err := s.Projects.Access(ctx, projectID)
	if err != nil {
		return apigen.Evaluation{}, err
	}
	if !a.CanApprove() {
		return apigen.Evaluation{}, errForbidden
	}
	var v validate.Errors
	if body.Score < 0 || body.Score > 10 {
		v.Add("score", validate.CodeInvalid, nil)
	}
	comment := v.Paragraph("comment", body.Comment, 10, 1000)
	if err := v.Err(); err != nil {
		return apigen.Evaluation{}, err
	}
	var row evaluationdb.Evaluation
	err = s.DB.Tx(ctx, func(ctx context.Context) error {
		q := s.q(ctx)
		var err error
		row, err = q.LockEvaluation(ctx, evaluationdb.LockEvaluationParams{ID: evaluationID, ProjectID: projectID})
		if db.IsNoRows(err) {
			return errNotFound
		}
		if err != nil {
			return err
		}
		if row.Status != StatusReady {
			return errNotReady
		}
		scores, err := q.ListScores(ctx, []uuid.UUID{evaluationID})
		if err != nil {
			return err
		}
		current := map[string]int{}
		old, found := 0, false
		for i := range scores {
			current[scores[i].CriterionKey] = int(scores[i].Score)
			if scores[i].CriterionKey == body.Criterion {
				old, found = int(scores[i].Score), true
			}
		}
		if !found {
			return apperr.Validation(apperr.FieldError{Field: "criterion", Code: validate.CodeInvalid})
		}
		if old == body.Score {
			return apperr.Validation(apperr.FieldError{Field: "score", Code: validate.CodeInvalid})
		}
		if _, err := q.SetScore(ctx, evaluationdb.SetScoreParams{EvaluationID: evaluationID, CriterionKey: body.Criterion, Score: int32(body.Score)}); err != nil { //nolint:gosec // 0–10
			return err
		}
		author := a.Principal.UserID
		if err := q.InsertOverride(ctx, evaluationdb.InsertOverrideParams{
			EvaluationID: evaluationID, CriterionKey: body.Criterion, OldScore: int32(old), NewScore: int32(body.Score), //nolint:gosec // 0–10
			Comment: comment, AuthorID: &author,
		}); err != nil {
			return err
		}
		current[body.Criterion] = body.Score
		total := fromSnapshot(row.Rubric).Total(current)
		level := Level(total)
		t := int32(total) //nolint:gosec // 0–100
		row.Total, row.Level = &t, &level
		if err := q.SetTotal(ctx, evaluationdb.SetTotalParams{ID: evaluationID, Total: &t, Level: &level}); err != nil {
			return err
		}
		return s.Audit.Record(ctx, audit.Event{
			Action: "evaluation.score_overridden", TargetType: "evaluation", TargetID: evaluationID.String(),
			Metadata: map[string]any{"criterion": body.Criterion, "from": old, "to": body.Score},
		})
	})
	if err != nil {
		return apigen.Evaluation{}, fmt.Errorf("evaluation: override: %w", err)
	}
	return s.view(ctx, a, &row)
}

// Latest implements project.EvaluationSource.
func (s *Service) Latest(ctx context.Context, projectIDs []uuid.UUID) (map[uuid.UUID]apigen.EvaluationBrief, error) {
	out := map[uuid.UUID]apigen.EvaluationBrief{}
	if len(projectIDs) == 0 {
		return out, nil
	}
	err := s.DB.ReadTx(ctx, func(ctx context.Context) error {
		rows, err := s.q(ctx).LatestReady(ctx, projectIDs)
		for i := range rows {
			r := &rows[i]
			if r.Total == nil || r.Level == nil || r.CompletedAt == nil {
				continue
			}
			out[r.ProjectID] = apigen.EvaluationBrief{
				Id: r.ID, Total: int(*r.Total), Level: apigen.ReadinessLevel(*r.Level), CompletedAt: *r.CompletedAt,
			}
		}
		return err
	})
	if err != nil {
		return nil, fmt.Errorf("evaluation: latest: %w", err)
	}
	return out, nil
}

// RunWorker asks the model to evaluate the project.
type RunWorker struct {
	river.WorkerDefaults[RunArgs]
	DB      *db.DB
	Gateway *ai.Gateway
}

// Timeout leaves room for a thorough model.
func (w *RunWorker) Timeout(*river.Job[RunArgs]) time.Duration { return 10 * time.Minute }

// Work implements river.Worker.
func (w *RunWorker) Work(ctx context.Context, job *river.Job[RunArgs]) error {
	args := job.Args
	ctx = db.WithTenant(tenancy.With(ctx, tenancy.Tenant{ID: args.TenantID}), args.TenantID)
	err := w.run(ctx, args)
	if err == nil {
		return nil
	}
	if job.Attempt < job.MaxAttempts {
		return err
	}
	code := "evaluation.failed"
	if e, ok := apperr.As(err); ok {
		code = e.Code
	}
	slog.ErrorContext(ctx, "evaluation failed", slog.String("evaluation_id", args.EvaluationID.String()), slog.Any("error", err))
	return w.DB.Tx(ctx, func(ctx context.Context) error {
		return evaluationdb.New(w.DB.Conn(ctx)).FailEvaluation(ctx, evaluationdb.FailEvaluationParams{ID: args.EvaluationID, Error: &code})
	})
}

var errInvalidResult = apperr.New(apperr.KindUnavailable, "ai.invalid_output", "the evaluation does not cover the rubric")

// Prompt renders the evaluation request of a project.
func Prompt(r Rubric, projectData, locale string) (system, version string, schema llm.Schema, err error) {
	criteria := make([]prompts.Criterion, len(r))
	for i, c := range r {
		criteria[i] = prompts.Criterion{Key: c.Key, Title: c.Title.Best("en"), Weight: c.Weight, Guide: c.Guide}
	}
	system, version, err = prompts.Render("evaluation", prompts.Evaluation{Locale: locale, Rubric: criteria, ProjectData: projectData})
	return system, version, resultSchema(r), err
}

// Parse reads the model's answer and checks that it scores every criterion once.
func Parse(r Rubric, text string) (Result, map[string]int, error) {
	var res Result
	if err := json.Unmarshal([]byte(text), &res); err != nil {
		return res, nil, fmt.Errorf("%w: %w", errInvalidResult, err)
	}
	scores := map[string]int{}
	for _, c := range res.Criteria {
		if _, dup := scores[c.Key]; dup {
			return res, nil, errInvalidResult
		}
		scores[c.Key] = min(max(c.Score, 0), 10)
	}
	for _, c := range r {
		if _, ok := scores[c.Key]; !ok {
			return res, nil, errInvalidResult
		}
	}
	return res, scores, nil
}

func (w *RunWorker) run(ctx context.Context, args RunArgs) error {
	var row evaluationdb.Evaluation
	err := w.DB.Tx(ctx, func(ctx context.Context) error {
		q := evaluationdb.New(w.DB.Conn(ctx))
		var err error
		if row, err = q.GetEvaluation(ctx, evaluationdb.GetEvaluationParams{ID: args.EvaluationID, ProjectID: args.ProjectID}); err != nil {
			return err
		}
		if row.Status == StatusReady || row.Status == StatusFailed {
			return nil
		}
		return q.SetRunning(ctx, args.EvaluationID)
	})
	if db.IsNoRows(err) {
		return nil // the project is gone
	}
	if err != nil {
		return fmt.Errorf("evaluation: start: %w", err)
	}
	if row.Status == StatusReady || row.Status == StatusFailed {
		return nil
	}
	r := fromSnapshot(row.Rubric)
	data, _, err := incubator.ProjectContext(ctx, w.DB, args.ProjectID, args.Card)
	if err != nil {
		return err
	}
	system, version, schema, err := Prompt(r, data, row.Locale)
	if err != nil {
		return err
	}
	res, err := w.Gateway.Complete(ctx, ai.Call{
		Feature: llm.FeatureEvaluation, PromptVersion: version, System: system, Schema: &schema, Locale: row.Locale,
		MaxTokens: 8000, UserID: args.RequestedBy,
		Messages: []llm.Message{{Role: llm.RoleUser, Content: "Evaluate the project."}},
	})
	if err != nil {
		return err
	}
	result, scores, err := Parse(r, res.Text)
	if err != nil {
		return err
	}
	total := r.Total(scores)
	level := Level(total)
	t := int32(total) //nolint:gosec // 0–100
	marshal := func(v any) json.RawMessage {
		raw, err := json.Marshal(v)
		if err != nil {
			return json.RawMessage("[]")
		}
		return raw
	}
	return w.DB.Tx(ctx, func(ctx context.Context) error {
		q := evaluationdb.New(w.DB.Conn(ctx))
		for _, c := range r {
			justification := ""
			for _, rc := range result.Criteria {
				if rc.Key == c.Key {
					justification = rc.Justification
				}
			}
			if err := q.InsertScore(ctx, evaluationdb.InsertScoreParams{
				EvaluationID: args.EvaluationID, CriterionKey: c.Key, Score: int32(scores[c.Key]), Justification: justification, //nolint:gosec // 0–10
			}); err != nil {
				return err
			}
		}
		provider, model := res.Provider, res.Model
		return q.CompleteEvaluation(ctx, evaluationdb.CompleteEvaluationParams{
			ID: args.EvaluationID, Total: &t, Level: &level, Summary: result.Summary,
			RedFlags: marshal(nonNil(result.RedFlags)), Hypotheses: marshal(nonNil(result.Hypotheses)),
			Experiments: marshal(nonNil(result.Experiments)), Recommendations: marshal(nonNil(result.Recommendations)),
			Provider: &provider, Model: &model, PromptVersion: &version,
		})
	})
}

func nonNil[T any](s []T) []T {
	if s == nil {
		return []T{}
	}
	return s
}

var errNoConverter = apperr.Unavailable("documents.unavailable", "documents cannot be generated right now")

// asConverterError maps a missing converter to a clear error.
func asConverterError(err error) error {
	if errors.Is(err, gotenberg.ErrNotConfigured) {
		return errNoConverter
	}
	return err
}

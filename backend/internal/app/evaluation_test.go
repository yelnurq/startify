package app

import (
	"context"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/riverqueue/river"
	"github.com/riverqueue/river/rivertype"

	"github.com/yelnurq/startify/backend/internal/apigen"
	"github.com/yelnurq/startify/backend/internal/evaluation"
	"github.com/yelnurq/startify/backend/internal/platform/gotenberg"
)

// runEvaluation runs the last evaluation job the API enqueued.
func runEvaluation(t *testing.T, h *harness) {
	t.Helper()
	var args evaluation.RunArgs
	found := false
	for _, j := range h.queue.jobs {
		if a, ok := j.(evaluation.RunArgs); ok {
			args, found = a, true
		}
	}
	if !found {
		t.Fatal("no evaluation job")
	}
	w := &evaluation.RunWorker{DB: h.db.App, Gateway: h.services.AI}
	if err := w.Work(context.Background(), &river.Job[evaluation.RunArgs]{Args: args, JobRow: &rivertype.JobRow{Attempt: 1, MaxAttempts: 3}}); err != nil {
		t.Fatal(err)
	}
	h.queue.jobs = nil
}

func TestEvaluations(t *testing.T) {
	h := newHarness(t)
	ts := seedTenants(t, h)
	owner, _ := account(t, h, ts, "owner@qadam.test", "Нұрлан Ахметов", "student")
	teacher, teacherID := account(t, h, ts, "teacher@qadam.test", "Гүлнар Әбдікерімова", "supervisor")
	manager, _ := account(t, h, ts, "manager@qadam.test", "Асхат Нұрғали", "innovation_manager")
	p := newProject(t, h, owner, "AgroScan")
	base := "/api/v1/projects/" + p.Id.String()
	expect(t, owner.do(http.MethodPost, base+"/members", apigen.ProjectMemberAdd{UserId: teacherID, Role: apigen.ProjectMemberAddRoleSupervisor}), http.StatusCreated)
	ru := func(r *http.Request) { r.Header.Set("Accept-Language", "ru") }

	expectProblem(t, owner.do(http.MethodPost, base+"/evaluations", nil), http.StatusUnprocessableEntity, "evaluation.project_empty")
	expect(t, owner.do(http.MethodPatch, base+"/steps/problem", apigen.StepUpdate{Values: map[string]any{
		"statement":    "Фермеры Северного Казахстана теряют до 20% урожая пшеницы, потому что замечают болезни слишком поздно, когда обработка уже не помогает.",
		"who":          "Хозяйства Костанайской и Акмолинской областей с полями от 100 до 500 гектаров.",
		"alternatives": "Агроном объезжает поля раз в неделю; дроны и спутники дороги для небольших хозяйств.",
	}}, ifMatchHeader(`"0"`)), http.StatusOK)

	rec := owner.do(http.MethodPost, base+"/evaluations", nil, ru)
	expect(t, rec, http.StatusAccepted)
	h.validate(t, http.MethodPost, base+"/evaluations", rec)
	first := decode[apigen.Evaluation](t, rec)
	if first.Status != apigen.EvaluationStatusPending || first.Total != nil || first.Locale != apigen.LocaleRu {
		t.Fatalf("requested %+v", first)
	}
	problem := expectProblem(t, teacher.do(http.MethodPost, base+"/evaluations", nil), http.StatusConflict, "evaluation.in_progress")
	if problem.Params["evaluation_id"] != first.Id.String() {
		t.Fatalf("params %+v", problem.Params)
	}
	runEvaluation(t, h)

	path := base + "/evaluations/" + first.Id.String()
	rec = owner.do(http.MethodGet, path, nil)
	expect(t, rec, http.StatusOK)
	h.validate(t, http.MethodGet, path, rec)
	e := decode[apigen.Evaluation](t, rec)
	if e.Status != apigen.EvaluationStatusReady || e.Total == nil || *e.Total != *e.AiTotal || len(e.Criteria) != 8 ||
		e.Level == nil || *e.Level != apigen.ReadinessLevelIdea || len(e.Hypotheses) < 3 || len(e.Experiments) == 0 || e.CanOverride ||
		*e.PromptVersion != "evaluation@2" || e.PreviousTotal != nil {
		t.Fatalf("ready %+v", e)
	}
	if !strings.Contains(e.Criteria[0].Justification, "Проблема") || e.Criteria[0].Previous != nil {
		t.Fatalf("criterion %+v", e.Criteria[0])
	}
	if card := decode[apigen.Project](t, owner.do(http.MethodGet, base, nil)); card.Evaluation == nil || card.Evaluation.Total != *e.Total {
		t.Fatalf("project card %+v", card.Evaluation)
	}

	// Experts correct a score with a comment; the AI score stays visible.
	override := apigen.ScoreOverride{Criterion: "problem", Score: 9, Comment: "Проблема подтверждена 14 интервью, которые команда показала на защите."}
	expectProblem(t, owner.do(http.MethodPost, path+"/overrides", override), http.StatusForbidden, "rbac.forbidden")
	expectProblem(t, teacher.do(http.MethodPost, path+"/overrides", apigen.ScoreOverride{Criterion: "problem", Score: 9, Comment: "коротко"}),
		http.StatusUnprocessableEntity, "validation.failed")
	expectProblem(t, teacher.do(http.MethodPost, path+"/overrides", apigen.ScoreOverride{
		Criterion: "problem", Score: e.Criteria[0].Score, Comment: "Оставляем балл без изменений, но фиксируем мнение.",
	}), http.StatusUnprocessableEntity, "validation.failed")
	rec = teacher.do(http.MethodPost, path+"/overrides", override)
	expect(t, rec, http.StatusOK)
	h.validate(t, http.MethodPost, path+"/overrides", rec)
	corrected := decode[apigen.Evaluation](t, rec)
	if *corrected.Total <= *e.Total || *corrected.AiTotal != *e.Total || !corrected.Criteria[0].Overridden ||
		corrected.Criteria[0].AiScore != e.Criteria[0].Score || len(corrected.Overrides) != 1 || corrected.Overrides[0].Author.Id != teacherID {
		t.Fatalf("corrected %+v", corrected)
	}

	// A second evaluation compares itself with the first.
	expect(t, owner.do(http.MethodPatch, base+"/steps/audience", apigen.StepUpdate{Values: map[string]any{
		"segments":       "Средние зерновые хозяйства без своего агронома и агрохолдинги со службой агрономов.",
		"early_adopters": "Фермеры, которые уже присылают фото листьев агрономам в WhatsApp-группах.",
		"personas":       []any{map[string]any{"name": "Серик, 46 лет, фермер", "pains": "Узнаёт о болезни слишком поздно"}},
	}}, ifMatchHeader(`"0"`)), http.StatusOK)
	second := decode[apigen.Evaluation](t, manager.do(http.MethodPost, base+"/evaluations", nil))
	runEvaluation(t, h)
	second = decode[apigen.Evaluation](t, manager.do(http.MethodGet, base+"/evaluations/"+second.Id.String(), nil))
	if second.PreviousTotal == nil || *second.PreviousTotal != *corrected.Total || second.Criteria[0].Previous == nil || !second.CanOverride {
		t.Fatalf("second %+v", second)
	}
	rec = owner.do(http.MethodGet, base+"/evaluations", nil)
	expect(t, rec, http.StatusOK)
	h.validate(t, http.MethodGet, base+"/evaluations", rec)
	if list := decode[apigen.EvaluationList](t, rec).Items; len(list) != 2 || list[0].Id != second.Id || !list[1].Overridden {
		t.Fatalf("history %+v", list)
	}

	// The PDF report needs the converter.
	expectProblem(t, owner.do(http.MethodGet, path+"/report", nil), http.StatusServiceUnavailable, "documents.unavailable")
	var html string
	conv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_ = r.ParseMultipartForm(10 << 20)
		f, _, err := r.FormFile("files")
		if err != nil {
			http.Error(w, err.Error(), http.StatusBadRequest)
			return
		}
		raw, _ := io.ReadAll(f)
		html = string(raw)
		_, _ = w.Write([]byte("%PDF-1.7 report"))
	}))
	defer conv.Close()
	h.services.Evaluations.Gotenberg = gotenberg.New(conv.URL, time.Second)
	rec = owner.do(http.MethodGet, path+"/report?locale=kk", nil)
	expect(t, rec, http.StatusOK)
	if rec.Body.String() != "%PDF-1.7 report" || !strings.Contains(rec.Header().Get("Content-Disposition"), "agroscan-evaluation-") ||
		!strings.Contains(html, "Жобаны AI бағалау") || !strings.Contains(html, "Сарапшы түзеткен") {
		t.Fatalf("report %q %q", rec.Header().Get("Content-Disposition"), html)
	}
}

func TestRubric(t *testing.T) {
	h := newHarness(t)
	ts := seedTenants(t, h)
	admin := seedAdmin(t, h, ts)
	student, _ := account(t, h, ts, "owner@qadam.test", "Нұрлан Ахметов", "student")

	expectProblem(t, student.do(http.MethodGet, "/api/v1/admin/rubric", nil), http.StatusForbidden, "rbac.forbidden")
	rec := admin.do(http.MethodGet, "/api/v1/admin/rubric", nil)
	expect(t, rec, http.StatusOK)
	h.validate(t, http.MethodGet, "/api/v1/admin/rubric", rec)
	r := decode[apigen.Rubric](t, rec)
	if r.Customized || len(r.Criteria) != 8 || rec.Header().Get("ETag") != `"0"` {
		t.Fatalf("rubric %+v", r)
	}
	weights := map[string]int{}
	for _, c := range r.Criteria {
		weights[c.Key] = c.Weight
	}
	weights["problem"] = 25
	problem := expectProblem(t, admin.do(http.MethodPut, "/api/v1/admin/rubric", apigen.RubricUpdate{Weights: weights}),
		http.StatusUnprocessableEntity, "validation.failed")
	if len(problem.Errors) != 1 || problem.Errors[0].Field != "weights" || problem.Errors[0].Code != "sum" {
		t.Fatalf("errors %+v", problem.Errors)
	}
	weights["impact"] = 0
	rec = admin.do(http.MethodPut, "/api/v1/admin/rubric", apigen.RubricUpdate{Weights: weights}, ifMatchHeader(`"0"`))
	expect(t, rec, http.StatusOK)
	h.validate(t, http.MethodPut, "/api/v1/admin/rubric", rec)
	if r := decode[apigen.Rubric](t, rec); !r.Customized || r.Criteria[0].Weight != 25 {
		t.Fatalf("saved %+v", r)
	}
	expectProblem(t, admin.do(http.MethodPut, "/api/v1/admin/rubric", apigen.RubricUpdate{Weights: weights}, ifMatchHeader(`"0"`)),
		http.StatusPreconditionFailed, "precondition.failed")
	// Weights put back to the defaults are not a customisation.
	defaults := map[string]int{}
	for _, c := range r.Criteria {
		defaults[c.Key] = c.Weight
	}
	if back := decode[apigen.Rubric](t, admin.do(http.MethodPut, "/api/v1/admin/rubric", apigen.RubricUpdate{Weights: defaults})); back.Customized {
		t.Fatal("default weights reported as customized")
	}
	if got := decode[apigen.Rubric](t, admin.do(http.MethodGet, "/api/v1/admin/rubric", nil)); got.Customized {
		t.Fatal("default weights reported as customized")
	}
	expect(t, admin.do(http.MethodPut, "/api/v1/admin/rubric", apigen.RubricUpdate{Weights: weights}), http.StatusOK)

	// New evaluations use the new weights.
	p := newProject(t, h, student, "AgroScan")
	base := "/api/v1/projects/" + p.Id.String()
	expect(t, student.do(http.MethodPatch, base+"/steps/problem", apigen.StepUpdate{Values: map[string]any{"statement": "Фермеры поздно замечают болезни"}},
		ifMatchHeader(`"0"`)), http.StatusOK)
	e := decode[apigen.Evaluation](t, student.do(http.MethodPost, base+"/evaluations", nil))
	runEvaluation(t, h)
	e = decode[apigen.Evaluation](t, student.do(http.MethodGet, base+"/evaluations/"+e.Id.String(), nil))
	if e.Criteria[0].Weight != 25 || e.Criteria[7].Weight != 0 {
		t.Fatalf("weights %+v", e.Criteria)
	}
}

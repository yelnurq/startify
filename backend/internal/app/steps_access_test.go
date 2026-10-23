package app

import (
	"bytes"
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/riverqueue/river"
	"github.com/riverqueue/river/rivertype"

	"github.com/yelnurq/startify/backend/internal/apigen"
	"github.com/yelnurq/startify/backend/internal/incubator"
	"github.com/yelnurq/startify/backend/internal/platform/gotenberg"
)

// auditActions lists the actions recorded about a target, oldest first.
func auditActions(t *testing.T, h *harness, targetID string) []string {
	t.Helper()
	ctx := context.Background()
	conn, err := pgx.Connect(ctx, h.db.OwnerURL)
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = conn.Close(ctx) }()
	rows, err := conn.Query(ctx, "SELECT action FROM audit_log WHERE target_id = $1 ORDER BY created_at, id", targetID)
	if err != nil {
		t.Fatal(err)
	}
	actions, err := pgx.CollectRows(rows, pgx.RowTo[string])
	if err != nil {
		t.Fatal(err)
	}
	return actions
}

func count(actions []string, action string) int {
	n := 0
	for _, a := range actions {
		if a == action {
			n++
		}
	}
	return n
}

// fakeConverter stands in for Gotenberg: every document becomes a tiny PDF.
func fakeConverter(t *testing.T) *gotenberg.Client {
	t.Helper()
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_ = r.ParseMultipartForm(10 << 20)
		_, _ = w.Write([]byte("%PDF-1.7 test"))
	}))
	t.Cleanup(srv.Close)
	return gotenberg.New(srv.URL, time.Second)
}

const (
	secretNovelty = "Собственная нейросеть различает 14 болезней пшеницы по одному фото листа"
	secretAssume  = "Цена из интервью с 14 фермерами, CAC — по стоимости выездной демонстрации в районе."
)

func TestSensitiveDetailsStayInTheTeam(t *testing.T) {
	h := newHarness(t)
	ts := seedTenants(t, h)
	owner, _ := account(t, h, ts, "owner@qadam.test", "Нұрлан Ахметов", "student")
	teacher, teacherID := account(t, h, ts, "teacher@qadam.test", "Гүлнар Әбдікерімова", "supervisor")
	manager, _ := account(t, h, ts, "manager@qadam.test", "Асхат Нұрғали", "innovation_manager")
	rector, _ := account(t, h, ts, "rector@qadam.test", "Ректор Университета", "leadership")
	p := newProject(t, h, owner, "AgroScan")
	base := "/api/v1/projects/" + p.Id.String()
	expect(t, owner.do(http.MethodPost, base+"/members", apigen.ProjectMemberAdd{UserId: teacherID, Role: apigen.ProjectMemberAddRoleSupervisor}), http.StatusCreated)

	// The template marks what is sensitive.
	tmpl := decode[apigen.IncubatorTemplate](t, owner.do(http.MethodGet, "/api/v1/incubator/template", nil))
	sensitive := map[string][]string{}
	for _, stage := range tmpl.Stages {
		for _, step := range stage.Steps {
			for _, f := range step.Fields {
				if f.Sensitive != nil && *f.Sensitive {
					sensitive[step.Key] = append(sensitive[step.Key], f.Key)
				}
			}
		}
	}
	if strings.Join(sensitive["unit_economics"], ",") != "unit,price,variable_cost,cac,purchases,assumptions" ||
		strings.Join(sensitive["solution"], ",") != "novelty,unfair_advantage" || len(sensitive) != 2 {
		t.Fatalf("sensitive fields %v", sensitive)
	}

	expect(t, owner.do(http.MethodPatch, base+"/steps/unit_economics", apigen.StepUpdate{Values: map[string]any{
		"unit": "Хозяйство в сезон", "price": 60000, "variable_cost": 8000, "cac": 15000, "purchases": 4, "assumptions": secretAssume,
	}}, ifMatchHeader(`"0"`)), http.StatusOK)
	expect(t, owner.do(http.MethodPatch, base+"/steps/solution", apigen.StepUpdate{Values: map[string]any{
		"solution": "Фермер фотографирует лист, приложение называет болезнь и подсказывает обработку прямо в поле.",
		"uvp":      "Диагноз болезни пшеницы за 10 секунд по фото",
		"novelty":  secretNovelty,
	}}, ifMatchHeader(`"0"`)), http.StatusOK)
	expect(t, owner.do(http.MethodPost, base+"/steps/unit_economics/check", nil), http.StatusOK)
	rec := owner.do(http.MethodPost, base+"/steps/unit_economics/mentor", apigen.MentorMessageCreate{Content: "Наш CAC 15000 ₸ — это нормально?"},
		func(r *http.Request) { r.Header.Set("Accept", "text/event-stream") })
	expect(t, rec, http.StatusOK)

	// A material written from these answers.
	worker := &incubator.MaterialWorker{DB: h.db.App, Gateway: h.services.AI, Files: h.services.Files, Gotenberg: fakeConverter(t)}
	h.queue.jobs = nil
	rec = owner.do(http.MethodPost, base+"/materials", apigen.MaterialCreate{Kind: apigen.MaterialKindOnePager, Format: apigen.MaterialFormatPdf})
	expect(t, rec, http.StatusAccepted)
	material := decode[apigen.Material](t, rec)
	args := h.queue.jobs[len(h.queue.jobs)-1].(incubator.MaterialArgs)
	if err := worker.Work(context.Background(), &river.Job[incubator.MaterialArgs]{Args: args, JobRow: &rivertype.JobRow{Attempt: 1, MaxAttempts: 3}}); err != nil {
		t.Fatal(err)
	}
	content := base + "/materials/" + material.Id.String() + "/content"

	// While the project shows its details, managers read them.
	step := decode[apigen.Step](t, manager.do(http.MethodGet, base+"/steps/unit_economics", nil))
	if step.Values["price"] == nil || step.Withheld == nil || len(*step.Withheld) != 0 || step.Check == nil {
		t.Fatalf("shown step %+v", step)
	}
	expect(t, manager.do(http.MethodGet, content, nil), http.StatusOK)

	rec = owner.do(http.MethodGet, base, nil)
	expect(t, owner.do(http.MethodPatch, base, apigen.ProjectUpdate{HideSensitive: ptr(true)}, ifMatchHeader(rec.Header().Get("ETag"))), http.StatusOK)

	// Outside the team the sensitive answers, their history, the review of
	// their step, the mentor's conversations and the materials are gone.
	for _, outsider := range []*browser{manager, rector} {
		rec = outsider.do(http.MethodGet, base+"/steps/unit_economics", nil)
		expect(t, rec, http.StatusOK)
		h.validate(t, http.MethodGet, base+"/steps/unit_economics", rec)
		step = decode[apigen.Step](t, rec)
		if len(step.Values) != 0 || step.Check != nil || step.Withheld == nil || len(*step.Withheld) != 6 || step.Status != apigen.StepStatusInProgress {
			t.Fatalf("withheld step %+v", step)
		}
		if strings.Contains(rec.Body.String(), "60000") || strings.Contains(rec.Body.String(), "интервью") {
			t.Fatalf("sensitive answers leak: %s", rec.Body)
		}
		solution := decode[apigen.Step](t, outsider.do(http.MethodGet, base+"/steps/solution", nil))
		if solution.Values["novelty"] != nil || solution.Values["uvp"] == nil || strings.Join(*solution.Withheld, ",") != "novelty,unfair_advantage" {
			t.Fatalf("solution %+v", solution)
		}
		for _, q := range []string{"", "?field=price", "?field=assumptions"} {
			rec = outsider.do(http.MethodGet, base+"/steps/unit_economics/revisions"+q, nil)
			expect(t, rec, http.StatusOK)
			if revs := decode[apigen.RevisionPage](t, rec).Items; len(revs) != 0 {
				t.Fatalf("history %s: %+v", q, revs)
			}
		}
		if revs := decode[apigen.RevisionPage](t, outsider.do(http.MethodGet, base+"/steps/solution/revisions", nil)).Items; len(revs) != 2 {
			t.Fatalf("solution history %+v", revs)
		}
		rec = outsider.do(http.MethodGet, base+"/steps/unit_economics/mentor", nil)
		expect(t, rec, http.StatusOK)
		h.validate(t, http.MethodGet, base+"/steps/unit_economics/mentor", rec)
		if conv := decode[apigen.MentorConversation](t, rec); len(conv.Items) != 0 || conv.Withheld == nil || !*conv.Withheld || strings.Contains(rec.Body.String(), "15000") {
			t.Fatalf("conversation %s", rec.Body)
		}
		if items := decode[apigen.MaterialList](t, outsider.do(http.MethodGet, base+"/materials", nil)).Items; len(items) != 0 {
			t.Fatalf("materials %+v", items)
		}
		expectProblem(t, outsider.do(http.MethodGet, content, nil), http.StatusNotFound, "incubator.material_not_found")
	}

	// The team and its supervisor see everything.
	for _, insider := range []*browser{owner, teacher} {
		step = decode[apigen.Step](t, insider.do(http.MethodGet, base+"/steps/unit_economics", nil))
		if step.Values["assumptions"] != secretAssume || len(*step.Withheld) != 0 || step.Check == nil {
			t.Fatalf("insider step %+v", step)
		}
		if revs := decode[apigen.RevisionPage](t, insider.do(http.MethodGet, base+"/steps/unit_economics/revisions", nil)).Items; len(revs) != 6 {
			t.Fatalf("insider history %+v", revs)
		}
		if conv := decode[apigen.MentorConversation](t, insider.do(http.MethodGet, base+"/steps/unit_economics/mentor", nil)); len(conv.Items) != 2 || *conv.Withheld {
			t.Fatalf("insider conversation %+v", conv)
		}
		if items := decode[apigen.MaterialList](t, insider.do(http.MethodGet, base+"/materials", nil)).Items; len(items) != 1 {
			t.Fatalf("insider materials %+v", items)
		}
		expect(t, insider.do(http.MethodGet, content, nil), http.StatusOK)
	}

	// A manager may still approve the step, and gets it back withheld.
	rec = manager.do(http.MethodPost, base+"/steps/unit_economics/approve", apigen.StepApprove{Comment: "Обсудили на защите"}, ifMatchHeader(`"1"`))
	expect(t, rec, http.StatusOK)
	if approved := decode[apigen.Step](t, rec); approved.Status != apigen.StepStatusApproved || len(approved.Values) != 0 {
		t.Fatalf("approved %+v", approved)
	}

	// Materials written before any sensitive answer stay visible.
	other := newProject(t, h, owner, "Kiiz Lab")
	obase := "/api/v1/projects/" + other.Id.String()
	rec = owner.do(http.MethodGet, obase, nil)
	expect(t, owner.do(http.MethodPatch, obase, apigen.ProjectUpdate{HideSensitive: ptr(true)}, ifMatchHeader(rec.Header().Get("ETag"))), http.StatusOK)
	expect(t, owner.do(http.MethodPatch, obase+"/steps/problem", apigen.StepUpdate{Values: map[string]any{"statement": "Войлок делают вручную неделями."}}, ifMatchHeader(`"0"`)), http.StatusOK)
	h.queue.jobs = nil
	expect(t, owner.do(http.MethodPost, obase+"/materials", apigen.MaterialCreate{Kind: apigen.MaterialKindOnePager, Format: apigen.MaterialFormatPdf}), http.StatusAccepted)
	args = h.queue.jobs[len(h.queue.jobs)-1].(incubator.MaterialArgs)
	if err := worker.Work(context.Background(), &river.Job[incubator.MaterialArgs]{Args: args, JobRow: &rivertype.JobRow{Attempt: 1, MaxAttempts: 3}}); err != nil {
		t.Fatal(err)
	}
	if items := decode[apigen.MaterialList](t, manager.do(http.MethodGet, obase+"/materials", nil)).Items; len(items) != 1 {
		t.Fatalf("plain materials %+v", items)
	}

	// The showcase prefill never takes sensitive answers.
	rec = owner.do(http.MethodGet, base+"/showcase", nil)
	expect(t, rec, http.StatusOK)
	if body := rec.Body.String(); strings.Contains(body, "нейросеть") || strings.Contains(body, "интервью с 14") ||
		!strings.Contains(body, "Диагноз болезни пшеницы") {
		t.Fatalf("showcase prefill: %s", body)
	}
}

// parallel sends a request as b without touching its cookie jar, so that
// several can run at once.
func (b *browser) parallel(method, path string, body any, mods ...func(*http.Request)) int {
	raw, err := json.Marshal(body)
	if err != nil {
		panic(err)
	}
	req := httptest.NewRequest(method, "http://"+b.host+path, bytes.NewReader(raw))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Origin", "http://"+b.host)
	req.Header.Set("Sec-Fetch-Site", "same-origin")
	for _, c := range b.jar {
		req.AddCookie(&http.Cookie{Name: c.Name, Value: c.Value})
	}
	if c, ok := b.jar["qadam_csrf"]; ok {
		req.Header.Set("X-CSRF-Token", c.Value)
	}
	for _, m := range mods {
		m(req)
	}
	rec := httptest.NewRecorder()
	b.h.handler.ServeHTTP(rec, req)
	return rec.Code
}

func TestApprovalHoldsForTheApprovedAnswers(t *testing.T) {
	h := newHarness(t)
	ts := seedTenants(t, h)
	owner, _ := account(t, h, ts, "owner@qadam.test", "Нұрлан Ахметов", "student")
	teacher, teacherID := account(t, h, ts, "teacher@qadam.test", "Гүлнар Әбдікерімова", "supervisor")
	p := newProject(t, h, owner, "AgroScan")
	base := "/api/v1/projects/" + p.Id.String()
	expect(t, owner.do(http.MethodPost, base+"/members", apigen.ProjectMemberAdd{UserId: teacherID, Role: apigen.ProjectMemberAddRoleSupervisor}), http.StatusCreated)
	step := base + "/steps/problem"
	expect(t, owner.do(http.MethodPatch, step, apigen.StepUpdate{Values: map[string]any{
		"statement":    "Фермеры Костанайской области теряют до 20% урожая пшеницы, потому что замечают болезни слишком поздно.",
		"who":          "Фермеры Северного Казахстана с полями от 100 до 500 гектаров пшеницы.",
		"alternatives": "Агроном объезжает поля раз в неделю; дроны дороги для небольших хозяйств.",
	}}, ifMatchHeader(`"0"`)), http.StatusOK)
	approve := func(etag string) *httptest.ResponseRecorder {
		if etag == "" {
			return teacher.do(http.MethodPost, step+"/approve", apigen.StepApprove{Comment: "Проблема описана честно"})
		}
		return teacher.do(http.MethodPost, step+"/approve", apigen.StepApprove{Comment: "Проблема описана честно"}, ifMatchHeader(etag))
	}

	// The approval names the version the supervisor reviewed.
	expectProblem(t, approve(""), http.StatusPreconditionRequired, "precondition.required")
	expectProblem(t, approve(`"0"`), http.StatusPreconditionFailed, "precondition.failed")
	rec := approve(`"1"`)
	expect(t, rec, http.StatusOK)
	h.validate(t, http.MethodPost, step+"/approve", rec)
	if s := decode[apigen.Step](t, rec); s.Status != apigen.StepStatusApproved || rec.Header().Get("ETag") != `"1"` {
		t.Fatalf("approved %+v", s)
	}
	completed := func() int {
		page := decode[apigen.ProjectPage](t, owner.do(http.MethodGet, "/api/v1/projects", nil))
		return page.Items[0].CompletedSteps
	}
	if completed() != 1 {
		t.Fatal("approval not counted")
	}

	// New answers drop the approval: completed while the minimum holds…
	rec = owner.do(http.MethodPatch, step, apigen.StepUpdate{Values: map[string]any{
		"alternatives": "Ничего не делают и теряют урожай; иногда зовут агронома из района за свой счёт.",
	}}, ifMatchHeader(`"1"`))
	expect(t, rec, http.StatusOK)
	if s := decode[apigen.Step](t, rec); s.Status != apigen.StepStatusCompleted || s.ApprovedBy != nil || s.ApprovalComment != nil {
		t.Fatalf("after edit %+v", s)
	}
	if completed() != 1 {
		t.Fatal("a completed step still counts")
	}
	// …in progress below it.
	expect(t, approve(`"2"`), http.StatusOK)
	rec = owner.do(http.MethodPatch, step, apigen.StepUpdate{Values: map[string]any{"who": ""}}, ifMatchHeader(`"2"`))
	if s := decode[apigen.Step](t, rec); s.Status != apigen.StepStatusInProgress || s.ApprovedBy != nil {
		t.Fatalf("after clearing %+v", s)
	}
	if completed() != 0 {
		t.Fatal("a broken step still counts")
	}
	if actions := auditActions(t, h, p.Id.String()); count(actions, "incubator.step_approval_dropped") != 2 || count(actions, "incubator.step_approved") != 2 {
		t.Fatalf("audit %v", actions)
	}
}

func TestTeamDoesNotApproveItsOwnSteps(t *testing.T) {
	h := newHarness(t)
	ts := seedTenants(t, h)
	// An innovation manager who is also a student with a project.
	owner, _ := account(t, h, ts, "owner@qadam.test", "Нұрлан Ахметов", "student", "innovation_manager")
	p := newProject(t, h, owner, "AgroScan")
	expectProblem(t, owner.do(http.MethodPost, "/api/v1/projects/"+p.Id.String()+"/steps/problem/approve",
		apigen.StepApprove{Comment: "Сам себе одобрил"}, ifMatchHeader(`"0"`)), http.StatusForbidden, "rbac.forbidden")
}

func TestProgressCountsStepsApprovedAtOnce(t *testing.T) {
	h := newHarness(t)
	ts := seedTenants(t, h)
	owner, _ := account(t, h, ts, "owner@qadam.test", "Нұрлан Ахметов", "student")
	teacher, teacherID := account(t, h, ts, "teacher@qadam.test", "Гүлнар Әбдікерімова", "supervisor")
	p := newProject(t, h, owner, "AgroScan")
	base := "/api/v1/projects/" + p.Id.String()
	expect(t, owner.do(http.MethodPost, base+"/members", apigen.ProjectMemberAdd{UserId: teacherID, Role: apigen.ProjectMemberAddRoleSupervisor}), http.StatusCreated)
	tmpl := decode[apigen.IncubatorTemplate](t, owner.do(http.MethodGet, "/api/v1/incubator/template", nil))
	var keys []string
	for _, stage := range tmpl.Stages {
		for _, s := range stage.Steps {
			keys = append(keys, s.Key)
		}
	}
	// Every step approved at the same moment: each transaction counts the
	// steps after the previous one committed, so none is lost.
	var wg sync.WaitGroup
	codes := make([]int, len(keys))
	for i, key := range keys {
		wg.Go(func() {
			codes[i] = teacher.parallel(http.MethodPost, base+"/steps/"+key+"/approve", apigen.StepApprove{Comment: "Принято"}, ifMatchHeader(`"0"`))
		})
	}
	wg.Wait()
	for i, code := range codes {
		if code != http.StatusOK {
			t.Fatalf("%s: %d", keys[i], code)
		}
	}
	page := decode[apigen.ProjectPage](t, owner.do(http.MethodGet, "/api/v1/projects", nil))
	if got := page.Items[0].CompletedSteps; got != len(keys) {
		t.Fatalf("completed_steps = %d, want %d", got, len(keys))
	}
}

func TestCommentsFollowTheProjectRules(t *testing.T) {
	h := newHarness(t)
	ts := seedTenants(t, h)
	owner, _ := account(t, h, ts, "owner@qadam.test", "Нұрлан Ахметов", "student")
	manager, _ := account(t, h, ts, "manager@qadam.test", "Асхат Нұрғали", "innovation_manager")
	admin, _ := account(t, h, ts, "admin@qadam.test", "Администратор Вуза", "tenant_admin")
	rector, _ := account(t, h, ts, "rector@qadam.test", "Ректор Университета", "leadership")
	p := newProject(t, h, owner, "AgroScan")
	base := "/api/v1/projects/" + p.Id.String()
	comment := func(b *browser, body string) *httptest.ResponseRecorder {
		return b.do(http.MethodPost, base+"/comments", apigen.CommentCreate{StepKey: "problem", Body: body})
	}

	// Leadership and administrators read; the team and innovation managers write.
	expectProblem(t, comment(rector, "Ректор одобряет"), http.StatusForbidden, "rbac.forbidden")
	expectProblem(t, comment(admin, "Администратор пишет"), http.StatusForbidden, "rbac.forbidden")
	byManager := decode[apigen.Comment](t, comment(manager, "Покажите источник цифры"))
	own := decode[apigen.Comment](t, comment(owner, "Добавим отчёт МСХ"))
	expectProblem(t, rector.do(http.MethodPatch, base+"/comments/"+byManager.Id.String(), apigen.CommentUpdate{Resolved: ptr(true)}),
		http.StatusForbidden, "rbac.forbidden")
	if items := decode[apigen.CommentPage](t, rector.do(http.MethodGet, base+"/comments", nil)).Items; len(items) != 2 {
		t.Fatalf("leadership reads %+v", items)
	}

	// Edits, resolutions and deletions are recorded, a moderator's with the author.
	expect(t, owner.do(http.MethodPatch, base+"/comments/"+own.Id.String(), apigen.CommentUpdate{Body: ptr("Добавим отчёт МСХ РК за 2025 год")}), http.StatusOK)
	expect(t, owner.do(http.MethodPatch, base+"/comments/"+byManager.Id.String(), apigen.CommentUpdate{Resolved: ptr(true)}), http.StatusOK)
	expect(t, owner.do(http.MethodPatch, base+"/comments/"+byManager.Id.String(), apigen.CommentUpdate{Resolved: ptr(false)}), http.StatusOK)
	expect(t, admin.do(http.MethodDelete, base+"/comments/"+byManager.Id.String(), nil), http.StatusNoContent)
	actions := auditActions(t, h, p.Id.String())
	for _, a := range []string{"incubator.comment_edited", "incubator.comment_resolved", "incubator.comment_reopened", "incubator.comment_deleted"} {
		if count(actions, a) != 1 {
			t.Fatalf("%s not recorded once: %v", a, actions)
		}
	}

	// An archived project keeps its comments.
	expect(t, owner.do(http.MethodDelete, base, nil), http.StatusNoContent)
	expectProblem(t, owner.do(http.MethodDelete, base+"/comments/"+own.Id.String(), nil), http.StatusConflict, "project.archived")
	expectProblem(t, comment(owner, "Ещё одна мысль"), http.StatusConflict, "project.archived")

	// A member writes a bounded number of comments.
	other := newProject(t, h, owner, "Kiiz Lab")
	limited := 0
	for i := range 40 {
		rec := owner.do(http.MethodPost, "/api/v1/projects/"+other.Id.String()+"/comments", apigen.CommentCreate{StepKey: "problem", Body: "Мысль " + uuid.NewString()[:8]})
		if rec.Code == http.StatusTooManyRequests {
			expectProblem(t, rec, http.StatusTooManyRequests, "rate_limited")
			limited = i
			break
		}
		expect(t, rec, http.StatusCreated)
	}
	if limited == 0 {
		t.Fatal("comments are not rate-limited")
	}
}

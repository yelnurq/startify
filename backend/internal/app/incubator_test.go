package app

import (
	"context"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/riverqueue/river"
	"github.com/riverqueue/river/rivertype"

	"github.com/yelnurq/startify/backend/internal/apigen"
	"github.com/yelnurq/startify/backend/internal/incubator"
	"github.com/yelnurq/startify/backend/internal/platform/gotenberg"
	"github.com/yelnurq/startify/backend/internal/seed"
)

// account creates a member of KazUTB with roles and signs them in.
func account(t *testing.T, h *harness, ts tenants, email, name string, roles ...string) (*browser, uuid.UUID) {
	t.Helper()
	id, err := seed.Account(context.Background(), h.db.App, ts.kazutb, seed.Member{
		Email: email, FullName: name, Locale: "ru", Password: password, Roles: roles,
		Slug: strings.Split(email, "@")[0],
	})
	if err != nil {
		t.Fatal(err)
	}
	b := h.browser(t, kazutbHost)
	login(t, b, email, password)
	return b, id
}

func ifMatchHeader(v string) func(*http.Request) {
	return func(r *http.Request) { r.Header.Set("If-Match", v) }
}

func newProject(t *testing.T, h *harness, b *browser, title string) apigen.Project {
	t.Helper()
	rec := b.do(http.MethodPost, "/api/v1/projects", apigen.ProjectCreate{
		Title: title, Industry: apigen.IndustryAgritech, IpPolicyVersion: ptr("2026-09"),
	})
	expect(t, rec, http.StatusCreated)
	h.validate(t, http.MethodPost, "/api/v1/projects", rec)
	return decode[apigen.Project](t, rec)
}

func TestProjectsAndTeams(t *testing.T) {
	h := newHarness(t)
	ts := seedTenants(t, h)
	owner, ownerID := account(t, h, ts, "owner@qadam.test", "Нұрлан Ахметов", "student")
	mate, mateID := account(t, h, ts, "mate@qadam.test", "Дана Сейітқали", "student")
	outsider, _ := account(t, h, ts, "outsider@qadam.test", "Тимур Ли", "student")
	teacher, teacherID := account(t, h, ts, "teacher@qadam.test", "Гүлнар Әбдікерімова", "supervisor")
	manager, _ := account(t, h, ts, "manager@qadam.test", "Асхат Нұрғали", "innovation_manager")

	// The university's intellectual property policy must be accepted.
	expectProblem(t, owner.do(http.MethodPost, "/api/v1/projects", apigen.ProjectCreate{Title: "AgroScan", Industry: apigen.IndustryAgritech}),
		http.StatusUnprocessableEntity, "project.ip_policy_required")
	rec := owner.do(http.MethodGet, "/api/v1/tenant", nil)
	if tn := decode[apigen.Tenant](t, rec); tn.IpPolicy == nil || tn.IpPolicy.Version != "2026-09" {
		t.Fatalf("tenant ip policy %+v", tn.IpPolicy)
	}

	p := newProject(t, h, owner, "AgroScan")
	if p.Access != apigen.ProjectAccessOwner || len(p.Members) != 1 || p.Progress.TotalSteps != 14 || p.Progress.CurrentStep != "problem" {
		t.Fatalf("created %+v", p)
	}
	base := "/api/v1/projects/" + p.Id.String()

	// Projects are visible to their people and to managers only.
	expectProblem(t, outsider.do(http.MethodGet, base, nil), http.StatusNotFound, "project.not_found")
	expectProblem(t, mate.do(http.MethodGet, "/api/v1/projects?scope=all", nil), http.StatusForbidden, "rbac.forbidden")
	rec = manager.do(http.MethodGet, "/api/v1/projects?scope=all", nil)
	expect(t, rec, http.StatusOK)
	h.validate(t, http.MethodGet, "/api/v1/projects", rec)
	if page := decode[apigen.ProjectPage](t, rec); len(page.Items) != 1 || page.Items[0].Access != apigen.ProjectAccessManager {
		t.Fatalf("manager list %+v", page)
	}

	// People search finds members to add.
	rec = owner.do(http.MethodGet, "/api/v1/people?q="+"Дана", nil)
	expect(t, rec, http.StatusOK)
	h.validate(t, http.MethodGet, "/api/v1/people", rec)
	if people := decode[apigen.PersonList](t, rec); len(people.Items) != 1 || people.Items[0].Id != mateID {
		t.Fatalf("people %+v", people)
	}

	// Only the owner adds members; supervisors need the supervisor role.
	add := apigen.ProjectMemberAdd{UserId: mateID, Role: apigen.ProjectMemberAddRoleMember, Title: ptr("CTO")}
	expectProblem(t, outsider.do(http.MethodPost, base+"/members", add), http.StatusNotFound, "project.not_found")
	rec = owner.do(http.MethodPost, base+"/members", add)
	expect(t, rec, http.StatusCreated)
	h.validate(t, http.MethodPost, base+"/members", rec)
	expectProblem(t, owner.do(http.MethodPost, base+"/members", add), http.StatusConflict, "project.already_member")
	expectProblem(t, owner.do(http.MethodPost, base+"/members", apigen.ProjectMemberAdd{UserId: ownerID, Role: apigen.ProjectMemberAddRoleSupervisor}),
		http.StatusUnprocessableEntity, "project.supervisor_role_required")
	expect(t, manager.do(http.MethodPost, base+"/members", apigen.ProjectMemberAdd{UserId: teacherID, Role: apigen.ProjectMemberAddRoleSupervisor}), http.StatusCreated)
	if p := decode[apigen.Project](t, teacher.do(http.MethodGet, base, nil)); p.Access != apigen.ProjectAccessSupervisor || len(p.Members) != 3 {
		t.Fatalf("supervisor view %+v", p)
	}

	// The card is edited with If-Match; progress changes do not touch its ETag.
	rec = mate.do(http.MethodGet, base, nil)
	etag := rec.Header().Get("ETag")
	expectProblem(t, teacher.do(http.MethodPatch, base, apigen.ProjectUpdate{Title: ptr("X")}), http.StatusForbidden, "rbac.forbidden")
	rec = mate.do(http.MethodPatch, base, apigen.ProjectUpdate{Tagline: ptr("Болезни пшеницы по фото листа")}, ifMatchHeader(etag))
	expect(t, rec, http.StatusOK)
	h.validate(t, http.MethodPatch, base, rec)
	expectProblem(t, owner.do(http.MethodPatch, base, apigen.ProjectUpdate{Title: ptr("Agro")}, ifMatchHeader(etag)),
		http.StatusPreconditionFailed, "precondition.failed")

	// Ownership moves; the owner cannot leave; members can.
	role := apigen.ProjectMemberUpdateRoleOwner
	expect(t, owner.do(http.MethodPatch, base+"/members/"+mateID.String(), apigen.ProjectMemberUpdate{Role: &role}), http.StatusOK)
	expectProblem(t, mate.do(http.MethodDelete, base+"/members/"+mateID.String(), nil), http.StatusConflict, "project.owner_cannot_leave")
	expect(t, owner.do(http.MethodDelete, base+"/members/"+ownerID.String(), nil), http.StatusNoContent)
	expectProblem(t, owner.do(http.MethodGet, base, nil), http.StatusNotFound, "project.not_found")

	// Archived projects are read-only.
	expect(t, mate.do(http.MethodDelete, base, nil), http.StatusNoContent)
	expectProblem(t, mate.do(http.MethodPatch, base, apigen.ProjectUpdate{Title: ptr("Agro")}), http.StatusConflict, "project.archived")
	if p := decode[apigen.Project](t, mate.do(http.MethodGet, base, nil)); !p.Archived {
		t.Fatal("not archived")
	}

	// Another university does not see the project at all.
	sandbox := h.browser(t, sandboxHost)
	register(t, sandbox, "sandbox-student@qadam.test", "Sandbox Student")
	expectProblem(t, sandbox.do(http.MethodGet, base, nil), http.StatusNotFound, "project.not_found")
}

func TestStepsHistoryAndCompletion(t *testing.T) {
	h := newHarness(t)
	ts := seedTenants(t, h)
	owner, _ := account(t, h, ts, "owner@qadam.test", "Нұрлан Ахметов", "student")
	mate, mateID := account(t, h, ts, "mate@qadam.test", "Дана Сейітқали", "student")
	teacher, teacherID := account(t, h, ts, "teacher@qadam.test", "Гүлнар Әбдікерімова", "supervisor")
	p := newProject(t, h, owner, "AgroScan")
	base := "/api/v1/projects/" + p.Id.String()
	expect(t, owner.do(http.MethodPost, base+"/members", apigen.ProjectMemberAdd{UserId: mateID, Role: apigen.ProjectMemberAddRoleMember}), http.StatusCreated)
	expect(t, owner.do(http.MethodPost, base+"/members", apigen.ProjectMemberAdd{UserId: teacherID, Role: apigen.ProjectMemberAddRoleSupervisor}), http.StatusCreated)

	rec := owner.do(http.MethodGet, "/api/v1/incubator/template", nil)
	expect(t, rec, http.StatusOK)
	h.validate(t, http.MethodGet, "/api/v1/incubator/template", rec)

	step := base + "/steps/problem"
	rec = owner.do(http.MethodGet, step, nil)
	expect(t, rec, http.StatusOK)
	h.validate(t, http.MethodGet, step, rec)
	if s := decode[apigen.Step](t, rec); s.Status != apigen.StepStatusNotStarted || len(s.Missing) != 3 || rec.Header().Get("ETag") != `"0"` {
		t.Fatalf("empty step %+v %s", s, rec.Header().Get("ETag"))
	}
	expectProblem(t, owner.do(http.MethodGet, base+"/steps/nope", nil), http.StatusNotFound, "incubator.step_not_found")

	save := func(b *browser, etag string, values map[string]any) *httptest.ResponseRecorder {
		return b.do(http.MethodPatch, step, apigen.StepUpdate{Values: values}, ifMatchHeader(etag))
	}
	expectProblem(t, owner.do(http.MethodPatch, step, apigen.StepUpdate{Values: map[string]any{"who": "x"}}), http.StatusPreconditionRequired, "precondition.required")
	expectProblem(t, save(owner, `"0"`, map[string]any{"color": "x"}), http.StatusUnprocessableEntity, "validation.failed")
	expectProblem(t, save(teacher, `"0"`, map[string]any{"who": "x"}), http.StatusForbidden, "rbac.forbidden")

	rec = save(owner, `"0"`, map[string]any{"statement": "Фермеры поздно замечают болезни"})
	expect(t, rec, http.StatusOK)
	h.validate(t, http.MethodPatch, step, rec)
	if s := decode[apigen.Step](t, rec); s.Version != 1 || s.Status != apigen.StepStatusInProgress || rec.Header().Get("ETag") != `"1"` {
		t.Fatalf("saved %+v", s)
	}
	// Autosaves of one author fold into one revision; another author starts a new one.
	expect(t, save(owner, `"1"`, map[string]any{"statement": "Фермеры поздно замечают болезни пшеницы"}), http.StatusOK)
	expectProblem(t, save(mate, `"1"`, map[string]any{"who": "Фермеры"}), http.StatusPreconditionFailed, "precondition.failed")
	statement := "Фермеры Костанайской области теряют до 20% урожая пшеницы, потому что замечают болезни слишком поздно, когда обработка уже не помогает."
	expect(t, save(mate, `"2"`, map[string]any{"statement": statement}), http.StatusOK)

	rec = owner.do(http.MethodGet, step+"/revisions?field=statement", nil)
	expect(t, rec, http.StatusOK)
	h.validate(t, http.MethodGet, step+"/revisions", rec)
	revs := decode[apigen.RevisionPage](t, rec).Items
	if len(revs) != 2 || revs[0].Value != statement || revs[1].Version != 2 || revs[1].Author.Id == mateID {
		t.Fatalf("revisions %+v", revs)
	}

	// Completion needs the minimum; the gaps come back as field errors.
	problem := expectProblem(t, owner.do(http.MethodPost, step+"/complete", nil), http.StatusUnprocessableEntity, "incubator.step_incomplete")
	if len(problem.Errors) != 2 || problem.Errors[0].Field != "fields.who" {
		t.Fatalf("gaps %+v", problem.Errors)
	}
	expect(t, save(owner, `"3"`, map[string]any{
		"who":          "Фермеры Северного Казахстана с полями от 100 до 500 гектаров пшеницы.",
		"alternatives": "Агроном объезжает поля раз в неделю; дроны дороги для небольших хозяйств.",
	}), http.StatusOK)
	rec = owner.do(http.MethodPost, step+"/complete", nil)
	expect(t, rec, http.StatusOK)
	h.validate(t, http.MethodPost, step+"/complete", rec)
	if s := decode[apigen.Step](t, rec); s.Status != apigen.StepStatusCompleted || s.CompletedAt == nil {
		t.Fatalf("completed %+v", s)
	}
	rec = owner.do(http.MethodGet, base+"/steps", nil)
	expect(t, rec, http.StatusOK)
	h.validate(t, http.MethodGet, base+"/steps", rec)
	list := decode[apigen.StepList](t, rec)
	if list.Progress.CompletedSteps != 1 || list.Progress.CurrentStep != "audience" || list.Progress.StreakDays != 1 || list.Items[0].Filled != 3 {
		t.Fatalf("progress %+v", list.Progress)
	}
	// Breaking the minimum reopens the step.
	expect(t, save(owner, `"4"`, map[string]any{"who": ""}), http.StatusOK)
	if s := decode[apigen.Step](t, owner.do(http.MethodGet, step, nil)); s.Status != apigen.StepStatusInProgress {
		t.Fatalf("reopened %+v", s)
	}

	// Restoring a revision brings the value back as a new revision.
	revs = decode[apigen.RevisionPage](t, owner.do(http.MethodGet, step+"/revisions?field=statement", nil)).Items
	rec = owner.do(http.MethodPost, step+"/revisions/"+revs[1].Id.String()+"/restore", nil, ifMatchHeader(`"5"`))
	expect(t, rec, http.StatusOK)
	h.validate(t, http.MethodPost, step+"/revisions/"+revs[1].Id.String()+"/restore", rec)
	if s := decode[apigen.Step](t, rec); s.Values["statement"] != revs[1].Value || s.Version != 6 {
		t.Fatalf("restored %+v", s)
	}
	if revs = decode[apigen.RevisionPage](t, owner.do(http.MethodGet, step+"/revisions?field=statement", nil)).Items; len(revs) != 3 ||
		revs[0].Source != apigen.RevisionSourceRestore {
		t.Fatalf("restore is a new revision: %+v", revs)
	}

	// The supervisor approves; the AI reviews.
	expectProblem(t, mate.do(http.MethodPost, base+"/steps/audience/approve", apigen.StepApprove{Comment: "ok"}, ifMatchHeader(`"0"`)),
		http.StatusForbidden, "rbac.forbidden")
	rec = teacher.do(http.MethodPost, base+"/steps/audience/approve", apigen.StepApprove{Comment: "Аудиторию обсудили на консультации"}, ifMatchHeader(`"0"`))
	expect(t, rec, http.StatusOK)
	if s := decode[apigen.Step](t, rec); s.Status != apigen.StepStatusApproved || s.ApprovedBy == nil || s.ApprovedBy.Id != teacherID {
		t.Fatalf("approved %+v", s)
	}
	rec = owner.do(http.MethodPost, step+"/check", nil, func(r *http.Request) { r.Header.Set("Accept-Language", "kk") })
	expect(t, rec, http.StatusOK)
	h.validate(t, http.MethodPost, step+"/check", rec)
	check := decode[apigen.Step](t, rec).Check
	if check == nil || len(check.Items) != 4 || check.PromptVersion != "step_check@1" || !strings.Contains(check.Items[1].Advice, "толтырылмаған") {
		t.Fatalf("check %+v", check)
	}
	if p := decode[apigen.Project](t, owner.do(http.MethodGet, base, nil)); !hasBadge(p.Progress.Badges, apigen.BadgeFirstReview) {
		t.Fatalf("badges %+v", p.Progress.Badges)
	}

	// Comments with mentions of the project's people.
	rec = mate.do(http.MethodPost, base+"/comments", apigen.CommentCreate{
		StepKey: "problem", FieldKey: ptr("statement"), Body: "@Гүлнар проверьте цифру 20%", Mentions: &[]uuid.UUID{teacherID},
	})
	expect(t, rec, http.StatusCreated)
	h.validate(t, http.MethodPost, base+"/comments", rec)
	comment := decode[apigen.Comment](t, rec)
	expectProblem(t, mate.do(http.MethodPost, base+"/comments", apigen.CommentCreate{
		StepKey: "problem", Body: "x", Mentions: &[]uuid.UUID{uuid.New()},
	}), http.StatusUnprocessableEntity, "validation.failed")
	reply := decode[apigen.Comment](t, teacher.do(http.MethodPost, base+"/comments", apigen.CommentCreate{
		StepKey: "problem", ParentId: &comment.Id, Body: "Источник — отчёт МСХ РК, проверьте год",
	}))
	expectProblem(t, mate.do(http.MethodPatch, base+"/comments/"+reply.Id.String(), apigen.CommentUpdate{Body: ptr("x")}), http.StatusForbidden, "rbac.forbidden")
	expect(t, mate.do(http.MethodPatch, base+"/comments/"+comment.Id.String(), apigen.CommentUpdate{Resolved: ptr(true)}), http.StatusOK)
	rec = owner.do(http.MethodGet, base+"/comments?step_key=problem", nil)
	expect(t, rec, http.StatusOK)
	h.validate(t, http.MethodGet, base+"/comments", rec)
	if c := decode[apigen.CommentPage](t, rec).Items; len(c) != 2 || !c[0].Resolved || c[0].Mentions[0].Id != teacherID {
		t.Fatalf("comments %+v", c)
	}

	// Presence shows who is here and the step versions.
	expect(t, mate.do(http.MethodPost, base+"/presence", apigen.PresenceUpdate{StepKey: ptr("audience")}), http.StatusOK)
	rec = owner.do(http.MethodPost, base+"/presence", apigen.PresenceUpdate{StepKey: ptr("problem")})
	expect(t, rec, http.StatusOK)
	h.validate(t, http.MethodPost, base+"/presence", rec)
	if pr := decode[apigen.Presence](t, rec); len(pr.Viewers) != 2 || pr.Versions["problem"] != 6 {
		t.Fatalf("presence %+v", pr)
	}
	// The presence of a quiet project expires.
	h.redis.FastForward(2 * time.Minute)
	if pr := decode[apigen.Presence](t, owner.do(http.MethodPost, base+"/presence", apigen.PresenceUpdate{})); len(pr.Viewers) != 1 {
		t.Fatalf("stale viewers stay: %+v", pr.Viewers)
	}

	rec = owner.do(http.MethodGet, base+"/activity?limit=50", nil)
	expect(t, rec, http.StatusOK)
	h.validate(t, http.MethodGet, base+"/activity", rec)
	kinds := map[apigen.ActivityKind]int{}
	for _, a := range decode[apigen.ActivityList](t, rec).Items {
		kinds[a.Kind]++
	}
	if kinds[apigen.ActivityKindEdit] == 0 || kinds[apigen.ActivityKindComment] != 2 || kinds[apigen.ActivityKindStepApproved] != 1 || kinds[apigen.ActivityKindMemberJoined] != 3 {
		t.Fatalf("activity %+v", kinds)
	}
}

func hasBadge(badges []apigen.Badge, b apigen.Badge) bool {
	for _, x := range badges {
		if x == b {
			return true
		}
	}
	return false
}

func TestMentorConversation(t *testing.T) {
	h := newHarness(t)
	ts := seedTenants(t, h)
	owner, _ := account(t, h, ts, "owner@qadam.test", "Нұрлан Ахметов", "student")
	teacher, teacherID := account(t, h, ts, "teacher@qadam.test", "Гүлнар Әбдікерімова", "supervisor")
	p := newProject(t, h, owner, "AgroScan")
	base := "/api/v1/projects/" + p.Id.String()
	expect(t, owner.do(http.MethodPost, base+"/members", apigen.ProjectMemberAdd{UserId: teacherID, Role: apigen.ProjectMemberAddRoleSupervisor}), http.StatusCreated)
	mentor := base + "/steps/problem/mentor"

	ask := func(b *browser, question string) []sseEvent {
		rec := b.do(http.MethodPost, mentor, apigen.MentorMessageCreate{Content: question}, func(r *http.Request) {
			r.Header.Set("Accept", "text/event-stream")
			r.Header.Set("Accept-Language", "ru-RU,ru;q=0.9")
		})
		expect(t, rec, http.StatusOK)
		return readEvents(t, rec.Body.String())
	}
	expectProblem(t, teacher.do(http.MethodPost, mentor, apigen.MentorMessageCreate{Content: "?"}), http.StatusForbidden, "incubator.mentor_forbidden")

	expect(t, owner.do(http.MethodPatch, base+"/steps/problem", apigen.StepUpdate{Values: map[string]any{
		"statement": "Фермеры поздно замечают болезни пшеницы и теряют урожай.",
	}}, ifMatchHeader(`"0"`)), http.StatusOK)
	events := ask(owner, "С чего начать?")
	if len(events) < 3 || events[0].name != "message" || events[len(events)-1].name != "done" {
		t.Fatalf("events %+v", events)
	}
	var streamed strings.Builder
	for _, e := range events {
		if e.name == "delta" {
			var d struct {
				Text string `json:"text"`
			}
			_ = json.Unmarshal([]byte(e.data), &d)
			streamed.WriteString(d.Text)
		}
	}
	var answer apigen.MentorMessage
	if err := json.Unmarshal([]byte(events[len(events)-1].data), &answer); err != nil {
		t.Fatal(err)
	}
	// The mentor sees the step's answers and points at what is missing.
	if answer.Content != streamed.String() || !strings.Contains(answer.Content, "осталось заполнить поле «Формулировка проблемы»") {
		t.Fatalf("answer %q vs %q", answer.Content, streamed.String())
	}

	events = ask(owner, "Помоги сформулировать проблему")
	if !strings.Contains(events[len(events)-1].data, `suggestion field=\"statement\"`) {
		t.Fatalf("no suggestion: %s", events[len(events)-1].data)
	}

	rec := teacher.do(http.MethodGet, mentor, nil)
	expect(t, rec, http.StatusOK)
	h.validate(t, http.MethodGet, mentor, rec)
	conv := decode[apigen.MentorConversation](t, rec)
	if len(conv.Items) != 4 || conv.Items[0].Role != apigen.MentorMessageRoleUser || conv.CanAsk || conv.External || conv.Modes[0] != apigen.MentorModeCoach {
		t.Fatalf("conversation %+v", conv)
	}
	if page := decode[apigen.MentorConversation](t, owner.do(http.MethodGet, mentor+"?limit=3", nil)); len(page.Items) != 3 || page.NextCursor == nil || !page.CanAsk {
		t.Fatalf("page %+v", page)
	}
}

func TestPitchMaterials(t *testing.T) {
	h := newHarness(t)
	ts := seedTenants(t, h)
	owner, _ := account(t, h, ts, "owner@qadam.test", "Нұрлан Ахметов", "student")
	p := newProject(t, h, owner, "AgroScan")
	base := "/api/v1/projects/" + p.Id.String()
	expect(t, owner.do(http.MethodPatch, base+"/steps/pitch", apigen.StepUpdate{Values: map[string]any{
		"one_liner": "Диагностика болезней пшеницы по фото листа для фермеров",
		"ask":       "Ищем пилот в двух хозяйствах Костанайской области.",
		"contact":   "agroscan@qadam.test",
	}}, ifMatchHeader(`"0"`)), http.StatusOK)

	expectProblem(t, owner.do(http.MethodPost, base+"/materials", apigen.MaterialCreate{Kind: apigen.MaterialKindPassport, Format: apigen.MaterialFormatPptx}),
		http.StatusUnprocessableEntity, "validation.failed")

	var converted []string
	conv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_ = r.ParseMultipartForm(10 << 20)
		f, _, err := r.FormFile("files")
		if err != nil {
			http.Error(w, err.Error(), http.StatusBadRequest)
			return
		}
		html, _ := io.ReadAll(f)
		converted = append(converted, string(html))
		_, _ = w.Write([]byte("%PDF-1.7 test"))
	}))
	defer conv.Close()

	worker := &incubator.MaterialWorker{DB: h.db.App, Gateway: h.services.AI, Files: h.services.Files, Gotenberg: gotenberg.New(conv.URL, time.Second)}
	request := func(kind apigen.MaterialKind, format apigen.MaterialFormat) apigen.Material {
		t.Helper()
		h.queue.jobs = nil
		rec := owner.do(http.MethodPost, base+"/materials", apigen.MaterialCreate{Kind: kind, Format: format, Locale: ptr(apigen.LocaleKk)})
		expect(t, rec, http.StatusAccepted)
		h.validate(t, http.MethodPost, base+"/materials", rec)
		m := decode[apigen.Material](t, rec)
		args := h.queue.jobs[len(h.queue.jobs)-1].(incubator.MaterialArgs)
		if err := worker.Work(context.Background(), &river.Job[incubator.MaterialArgs]{Args: args, JobRow: &rivertype.JobRow{Attempt: 1, MaxAttempts: 3}}); err != nil {
			t.Fatal(err)
		}
		return m
	}
	onePager := request(apigen.MaterialKindOnePager, apigen.MaterialFormatPdf)
	passport := request(apigen.MaterialKindPassport, apigen.MaterialFormatPdf)
	deck := request(apigen.MaterialKindPitchDeck, apigen.MaterialFormatPptx)

	rec := owner.do(http.MethodGet, base+"/materials", nil)
	expect(t, rec, http.StatusOK)
	h.validate(t, http.MethodGet, base+"/materials", rec)
	for _, m := range decode[apigen.MaterialList](t, rec).Items {
		if m.Status != apigen.MaterialStatusReady || m.DownloadUrl == nil {
			t.Fatalf("material %+v", m)
		}
	}
	if len(converted) != 2 || !strings.Contains(converted[0], "Жоба бір бетте") || !strings.Contains(converted[1], "<svg") ||
		!strings.Contains(converted[1], "agroscan@qadam.test") {
		t.Fatalf("documents sent for conversion: %d", len(converted))
	}

	rec = owner.do(http.MethodGet, base+"/materials/"+onePager.Id.String()+"/content", nil)
	expect(t, rec, http.StatusOK)
	if rec.Body.String() != "%PDF-1.7 test" || !strings.Contains(rec.Header().Get("Content-Disposition"), "agroscan-one-pager.pdf") {
		t.Fatalf("download %q %q", rec.Body.String(), rec.Header().Get("Content-Disposition"))
	}
	rec = owner.do(http.MethodGet, base+"/materials/"+deck.Id.String()+"/content", nil)
	expect(t, rec, http.StatusOK)
	if !strings.HasPrefix(rec.Body.String(), "PK") || !strings.Contains(rec.Header().Get("Content-Type"), "presentationml") {
		t.Fatalf("deck %q", rec.Header().Get("Content-Type"))
	}
	if p := decode[apigen.Project](t, owner.do(http.MethodGet, base, nil)); !hasBadge(p.Progress.Badges, apigen.BadgeMaterialsReady) {
		t.Fatalf("badges %+v", p.Progress.Badges)
	}
	_ = passport

	// Without a converter the last attempt marks the material as failed.
	worker.Gotenberg = gotenberg.New("", time.Second)
	h.queue.jobs = nil
	expect(t, owner.do(http.MethodPost, base+"/materials", apigen.MaterialCreate{Kind: apigen.MaterialKindOnePager, Format: apigen.MaterialFormatPdf}), http.StatusAccepted)
	args := h.queue.jobs[0].(incubator.MaterialArgs)
	if err := worker.Work(context.Background(), &river.Job[incubator.MaterialArgs]{Args: args, JobRow: &rivertype.JobRow{Attempt: 1, MaxAttempts: 3}}); err != nil {
		t.Fatal(err)
	}
	failed := decode[apigen.MaterialList](t, owner.do(http.MethodGet, base+"/materials", nil)).Items[0]
	if failed.Status != apigen.MaterialStatusFailed || failed.Error == nil || *failed.Error != "incubator.documents_unavailable" {
		t.Fatalf("failed %+v", failed)
	}
	expectProblem(t, owner.do(http.MethodGet, base+"/materials/"+failed.Id.String()+"/content", nil), http.StatusConflict, "incubator.material_not_ready")
}

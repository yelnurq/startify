package app

import (
	"context"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"testing"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/riverqueue/river"
	"github.com/riverqueue/river/rivertype"

	"github.com/yelnurq/startify/backend/internal/apigen"
	"github.com/yelnurq/startify/backend/internal/files"
	"github.com/yelnurq/startify/backend/internal/incubator"
	"github.com/yelnurq/startify/backend/internal/platform/db"
	"github.com/yelnurq/startify/backend/internal/seed"
)

// fingerprint hashes every row of a university in every table with
// tenant_id (from the catalog), as the worker's system scope sees them.
func fingerprint(t *testing.T, h *harness, tenantID uuid.UUID) map[string]string {
	t.Helper()
	ctx := context.Background()
	out := map[string]string{}
	err := h.db.Worker.ReadTx(db.WithSystem(ctx), func(ctx context.Context) error {
		conn := h.db.Worker.Conn(ctx)
		rows, err := conn.Query(ctx, `
SELECT c.relname::text FROM pg_class c
  JOIN pg_namespace n ON n.oid = c.relnamespace
  JOIN pg_attribute a ON a.attrelid = c.oid AND a.attname = 'tenant_id' AND NOT a.attisdropped
 WHERE n.nspname = 'public' AND c.relkind IN ('r', 'p')`)
		if err != nil {
			return err
		}
		tables, err := pgx.CollectRows(rows, pgx.RowTo[string])
		if err != nil {
			return err
		}
		for _, table := range tables {
			var sum string
			err := conn.QueryRow(ctx, "SELECT md5(coalesce(string_agg(t::text, '|' ORDER BY t::text), '')) FROM "+
				pgx.Identifier{table}.Sanitize()+" t WHERE tenant_id = $1", tenantID).Scan(&sum)
			if err != nil {
				return err
			}
			out[table] = sum
		}
		return nil
	})
	if err != nil {
		t.Fatal(err)
	}
	return out
}

// denied expects a foreign resource to be invisible (404) or forbidden (403).
func denied(t *testing.T, rec *httptest.ResponseRecorder, what string) {
	t.Helper()
	if rec.Code != http.StatusNotFound && rec.Code != http.StatusForbidden {
		t.Errorf("%s: status %d, want 404 or 403: %s", what, rec.Code, rec.Body)
	}
}

// Everything KazUTB has — projects with evaluations, materials, mentor
// conversations and comments, its rubric, knowledge base, contact requests,
// members, imports, private files and AI usage — stays out of reach of
// another university: its administrator (with every staff role) and a member
// of both universities on the other university's host get 404 or 403 for
// reads and writes by KazUTB's ids, and nothing of KazUTB changes (С-22).
func TestForeignUniversitiesReachNothing(t *testing.T) {
	h := newHarness(t)
	ts := seedTenants(t, h)
	ctx := context.Background()
	owner, _ := account(t, h, ts, "owner@qadam.test", "Нұрлан Ахметов", "student")
	manager, _ := account(t, h, ts, "manager@qadam.test", "Асхат Нұрғали", "innovation_manager")
	admin := seedAdmin(t, h, ts)
	_, studentID := account(t, h, ts, "student@qadam.test", "Дана Сейітқали", "student")

	// KazUTB's data: a public page with a contact request, steps, a mentor
	// conversation, a comment, a material, an evaluation, a custom rubric, a
	// knowledge document and an import in preview.
	p, base := publicPage(t, h, owner, manager)
	rec := h.browser(t, kazutbHost).do(http.MethodPost, "/api/v1/showcase/entries/agroscan/contact", apigen.ContactCreate{
		Name: "Ержан Байжанов", Email: "erzhan@agroinvest.kz", Kind: apigen.ContactKindInvestment,
		Message: "Хотим обсудить пилот на трёх хозяйствах в Костанайской области этой весной.",
	})
	expect(t, rec, http.StatusAccepted)
	contacts := decode[apigen.ContactRequestList](t, manager.do(http.MethodGet, "/api/v1/admin/contacts", nil)).Items
	if len(contacts) != 1 {
		t.Fatalf("contacts %+v", contacts)
	}
	mentor := base + "/steps/problem/mentor"
	expect(t, owner.do(http.MethodPost, mentor, apigen.MentorMessageCreate{Content: "С чего начать?"},
		func(r *http.Request) { r.Header.Set("Accept", "text/event-stream") }), http.StatusOK)
	comment := decode[apigen.Comment](t, owner.do(http.MethodPost, base+"/comments", apigen.CommentCreate{StepKey: "problem", Body: "Добавим отчёт МСХ"}))
	h.queue.jobs = nil
	material := decode[apigen.Material](t, owner.do(http.MethodPost, base+"/materials", apigen.MaterialCreate{Kind: apigen.MaterialKindOnePager, Format: apigen.MaterialFormatPdf}))
	worker := &incubator.MaterialWorker{DB: h.db.Worker, Gateway: h.services.AI, Files: h.services.Files, Gotenberg: fakeConverter(t)}
	if err := worker.Work(ctx, &river.Job[incubator.MaterialArgs]{Args: lastJob[incubator.MaterialArgs](t, h), JobRow: &rivertype.JobRow{Attempt: 1, MaxAttempts: 3}}); err != nil {
		t.Fatal(err)
	}
	generated := materialFile(t, h, material.Id)
	evaluation := decode[apigen.Evaluation](t, owner.do(http.MethodPost, base+"/evaluations", nil))
	runEvaluation(t, h)
	rubric := decode[apigen.Rubric](t, admin.do(http.MethodGet, "/api/v1/admin/rubric", nil))
	weights := map[string]int{}
	for _, c := range rubric.Criteria {
		weights[c.Key] = c.Weight
	}
	weights["problem"], weights["impact"] = weights["problem"]+weights["impact"], 0
	expect(t, admin.do(http.MethodPut, "/api/v1/admin/rubric", apigen.RubricUpdate{Weights: weights}), http.StatusOK)
	rec = manager.do(http.MethodPost, "/api/v1/admin/knowledge", multipartForm(t, map[string]string{"title": "Положение о конкурсе"},
		"file", "polozhenie.txt", []byte("Раздел 1. Сроки\n\nЗаявки на конкурс принимаются до 1 декабря.")))
	expect(t, rec, http.StatusCreated)
	document := decode[apigen.KnowledgeDocument](t, rec)
	rec = admin.do(http.MethodPost, "/api/v1/admin/imports", multipartForm(t, map[string]string{"kind": "students"},
		"file", "students.csv", []byte("почта,фио\ninvited@qadam.test,Шақырылған Студент\n")))
	expect(t, rec, http.StatusCreated)
	job := decode[apigen.ImportJob](t, rec)

	// The other university: its administrator holds every staff role, and
	// KazUTB's project owner is a member there too.
	if _, err := seed.Account(ctx, h.db.Worker, ts.sandbox, seed.Member{
		Email: "sandbox-staff@qadam.test", FullName: "Sandbox Staff", Locale: "ru", Password: password,
		Roles: []string{"tenant_admin", "innovation_manager", "supervisor", "jury", "leadership"}, Slug: "sandbox-staff",
	}); err != nil {
		t.Fatal(err)
	}
	staff := h.browser(t, sandboxHost)
	login(t, staff, "sandbox-staff@qadam.test", password)
	dual := h.browser(t, sandboxHost)
	if joined := register(t, dual, "owner@qadam.test", "Нұрлан Ахметов"); joined.Tenant.Slug != "sandbox" {
		t.Fatalf("joined %+v", joined.Tenant)
	}

	before := fingerprint(t, h, ts.kazutb)
	project, eval := p.Id.String(), evaluation.Id.String()
	type attempt struct {
		method, path string
		body         any
		mods         []func(*http.Request)
	}
	ifMatch := ifMatchHeader(`"1"`)
	foreign := []attempt{
		// Projects and their parts.
		{method: http.MethodGet, path: base},
		{method: http.MethodPatch, path: base, body: apigen.ProjectUpdate{Title: ptr("Чужой проект")}},
		{method: http.MethodDelete, path: base},
		{method: http.MethodGet, path: base + "/steps/problem"},
		{method: http.MethodPatch, path: base + "/steps/problem", body: apigen.StepUpdate{Values: map[string]any{"statement": "Перезаписано чужим вузом"}}, mods: []func(*http.Request){ifMatch}},
		{method: http.MethodPost, path: base + "/steps/problem/approve", body: apigen.StepApprove{}, mods: []func(*http.Request){ifMatch}},
		// Evaluations and the rubric.
		{method: http.MethodGet, path: base + "/evaluations"},
		{method: http.MethodPost, path: base + "/evaluations"},
		{method: http.MethodGet, path: base + "/evaluations/" + eval},
		{method: http.MethodGet, path: base + "/evaluations/" + eval + "/report"},
		{method: http.MethodPost, path: base + "/evaluations/" + eval + "/overrides", body: apigen.ScoreOverride{
			Criterion: "problem", Score: 9, Comment: "Проблема подтверждена опросом сорока студентов.",
		}},
		// Mentor conversations.
		{method: http.MethodGet, path: mentor},
		{
			method: http.MethodPost, path: mentor, body: apigen.MentorMessageCreate{Content: "Что тут у соседей?"},
			mods: []func(*http.Request){func(r *http.Request) { r.Header.Set("Accept", "text/event-stream") }},
		},
		// Materials and comments.
		{method: http.MethodGet, path: base + "/materials"},
		{method: http.MethodPost, path: base + "/materials", body: apigen.MaterialCreate{Kind: apigen.MaterialKindOnePager, Format: apigen.MaterialFormatPdf}},
		{method: http.MethodGet, path: base + "/materials/" + material.Id.String() + "/content"},
		{method: http.MethodGet, path: base + "/comments"},
		{method: http.MethodPost, path: base + "/comments", body: apigen.CommentCreate{StepKey: "problem", Body: "Комментарий чужого вуза"}},
		{method: http.MethodPatch, path: base + "/comments/" + comment.Id.String(), body: apigen.CommentUpdate{Body: ptr("Изменено чужим вузом")}},
		{method: http.MethodDelete, path: base + "/comments/" + comment.Id.String()},
		// The showcase page and its contact requests.
		{method: http.MethodGet, path: base + "/showcase"},
		{method: http.MethodPut, path: base + "/showcase", body: pageUpdate("agroscan", apigen.ShowcaseVisibilityPublic), mods: []func(*http.Request){ifMatch}},
		{method: http.MethodPost, path: base + "/showcase/unpublish"},
		{method: http.MethodPost, path: "/api/v1/admin/showcase/" + project + "/review", body: apigen.ShowcaseReview{Approve: false, Comment: ptr("Снято чужим вузом")}},
		{method: http.MethodGet, path: base + "/contacts"},
		{method: http.MethodPost, path: "/api/v1/admin/contacts/" + contacts[0].Id.String() + "/moderate", body: apigen.ContactModeration{Forward: true}},
		{method: http.MethodGet, path: "/api/v1/showcase/entries/agroscan"},
		// The knowledge base, members, imports and private files.
		{method: http.MethodGet, path: "/api/v1/admin/knowledge/" + document.Id.String()},
		{method: http.MethodDelete, path: "/api/v1/admin/knowledge/" + document.Id.String()},
		{method: http.MethodPatch, path: "/api/v1/admin/members/" + studentID.String(), body: apigen.MemberUpdate{Roles: &[]apigen.Role{apigen.RoleTenantAdmin}}},
		{method: http.MethodGet, path: "/api/v1/admin/imports/" + job.Id.String()},
		{method: http.MethodPost, path: "/api/v1/admin/imports/" + job.Id.String() + "/commit"},
		{method: http.MethodGet, path: *document.FileUrl},
		{method: http.MethodGet, path: "/api/v1/files/" + generated.String() + "/content"},
	}
	for _, who := range []struct {
		name string
		b    *browser
	}{{"sandbox staff", staff}, {"member of both", dual}} {
		for _, a := range foreign {
			denied(t, who.b.do(a.method, a.path, a.body, a.mods...), who.name+": "+a.method+" "+a.path)
		}
	}

	// Lists of the other university show none of KazUTB's (AI usage first:
	// the knowledge search below embeds the question in sandbox's account).
	if usage := decode[apigen.AiUsageReport](t, staff.do(http.MethodGet, "/api/v1/admin/ai/usage?days=7", nil)); usage.Totals.Requests != 0 {
		t.Errorf("AI usage for sandbox staff: %+v", usage.Totals)
	}
	if items := decode[apigen.ProjectPage](t, dual.do(http.MethodGet, "/api/v1/projects", nil)).Items; len(items) != 0 {
		t.Errorf("projects of the member of both on sandbox: %+v", items)
	}
	if items := decode[apigen.ProjectPage](t, staff.do(http.MethodGet, "/api/v1/projects", nil)).Items; len(items) != 0 {
		t.Errorf("projects for sandbox staff: %+v", items)
	}
	if items := decode[apigen.ContactRequestList](t, staff.do(http.MethodGet, "/api/v1/admin/contacts", nil)).Items; len(items) != 0 {
		t.Errorf("contacts for sandbox staff: %+v", items)
	}
	if items := decode[apigen.ShowcaseReviewList](t, staff.do(http.MethodGet, "/api/v1/admin/showcase?status=published", nil)).Items; len(items) != 0 {
		t.Errorf("pages for sandbox moderation: %+v", items)
	}
	if items := decode[apigen.KnowledgeDocumentPage](t, staff.do(http.MethodGet, "/api/v1/admin/knowledge", nil)).Items; len(items) != 0 {
		t.Errorf("knowledge for sandbox staff: %+v", items)
	}
	rec = staff.do(http.MethodGet, "/api/v1/admin/knowledge/search?q="+url.QueryEscape("до какого числа принимаются заявки"), nil)
	if hits := decode[struct{ Items []apigen.KnowledgeHit }](t, rec).Items; len(hits) != 0 {
		t.Errorf("knowledge search for sandbox staff: %+v", hits)
	}
	if items := decode[apigen.MemberPage](t, staff.do(http.MethodGet, "/api/v1/admin/members?q=student%40qadam.test", nil)).Items; len(items) != 0 {
		t.Errorf("members for sandbox staff: %+v", items)
	}
	if r := decode[apigen.Rubric](t, staff.do(http.MethodGet, "/api/v1/admin/rubric", nil)); r.Customized {
		t.Error("sandbox reads KazUTB's rubric")
	}
	// Its own settings are its own: changing them leaves KazUTB's alone.
	expect(t, staff.do(http.MethodPut, "/api/v1/admin/rubric", apigen.RubricUpdate{Weights: weights}), http.StatusOK)
	expect(t, staff.do(http.MethodPut, "/api/v1/admin/ai/settings", apigen.AiSettingsUpdate{
		Routes: apigen.AiRoutes{}, LocalOnly: true, DailyUserTokens: 1000, DailyTenantTokens: 100000, MentorMode: apigen.MentorModeCoach,
	}), http.StatusOK)

	after := fingerprint(t, h, ts.kazutb)
	for table, sum := range before {
		if after[table] != sum {
			t.Errorf("%s of KazUTB changed from another university", table)
		}
	}
	// The data is still there for KazUTB itself.
	if got := decode[apigen.Project](t, owner.do(http.MethodGet, base, nil)); got.Title != "AgroScan" {
		t.Errorf("project %+v", got)
	}
	if !strings.HasPrefix(*document.FileUrl, "/api/v1/files/") {
		t.Fatalf("file url %q", *document.FileUrl)
	}
	expect(t, manager.do(http.MethodGet, *document.FileUrl, nil), http.StatusOK)
}

// A member of two universities changes and removes the avatar uploaded in
// the other one: the API may read that file but not change it, so the
// worker deletes it (audit Н-37: this was a 500).
func TestAvatarOfAMemberOfTwoUniversities(t *testing.T) {
	h := newHarness(t)
	ts := seedTenants(t, h)
	k, userID := account(t, h, ts, "owner@qadam.test", "Нұрлан Ахметов", "student")
	s := h.browser(t, sandboxHost)
	register(t, s, "owner@qadam.test", "Нұрлан Ахметов")

	upload := func(b *browser) uuid.UUID {
		t.Helper()
		rec := b.do(http.MethodPut, "/api/v1/me/avatar", multipartForm(t, nil, "file", "photo.png", pngImage(t, 64, 64)))
		expect(t, rec, http.StatusOK)
		u := decode[apigen.Me](t, rec).User.AvatarUrl
		if u == nil {
			t.Fatal("no avatar")
		}
		id, err := uuid.Parse(strings.TrimSuffix(strings.TrimPrefix(*u, "/api/v1/files/"), "/content"))
		if err != nil {
			t.Fatal(err)
		}
		return id
	}
	deleted := func(id uuid.UUID) bool {
		return scalar[bool](t, h, "SELECT deleted_at IS NOT NULL FROM files WHERE id = $1", id)
	}
	release := func(want uuid.UUID) {
		t.Helper()
		args := lastJob[files.ReleaseArgs](t, h)
		if args.FileID != want || args.OwnerID != userID {
			t.Fatalf("release %+v, want %s of %s", args, want, userID)
		}
		w := &files.ReleaseWorker{DB: h.db.Worker}
		// Only an avatar of the account that asked is deleted.
		other := river.Job[files.ReleaseArgs]{Args: files.ReleaseArgs{FileID: args.FileID, OwnerID: uuid.Must(uuid.NewV7())}, JobRow: jobRow()}
		if err := w.Work(context.Background(), &other); err != nil || deleted(want) {
			t.Fatalf("released for another account: %v", err)
		}
		if err := w.Work(context.Background(), &river.Job[files.ReleaseArgs]{Args: args, JobRow: jobRow()}); err != nil {
			t.Fatal(err)
		}
		h.queue.jobs = nil
	}

	first := upload(k)
	h.queue.jobs = nil
	second := upload(s)
	if deleted(first) {
		t.Fatal("the other university's file was changed by the API")
	}
	release(first)
	if !deleted(first) || deleted(second) {
		t.Fatalf("after release: first deleted %v, second deleted %v", deleted(first), deleted(second))
	}
	expectProblem(t, k.do(http.MethodGet, "/api/v1/files/"+first.String()+"/content", nil), http.StatusNotFound, "files.not_found")

	// Removing it from the first university, where it was not uploaded.
	expect(t, k.do(http.MethodDelete, "/api/v1/me/avatar", nil), http.StatusNoContent)
	release(second)
	if !deleted(second) {
		t.Fatal("the removed avatar stays")
	}
	if me := decode[apigen.Me](t, s.do(http.MethodGet, "/api/v1/me", nil)); me.User.AvatarUrl != nil {
		t.Fatalf("avatar %v", *me.User.AvatarUrl)
	}
	// In its own university the API deletes the file itself.
	third := upload(s)
	h.queue.jobs = nil
	expect(t, s.do(http.MethodDelete, "/api/v1/me/avatar", nil), http.StatusNoContent)
	if !deleted(third) || len(h.queue.jobs) != 0 {
		t.Fatalf("own avatar: deleted %v, jobs %d", deleted(third), len(h.queue.jobs))
	}
}

package app

import (
	"bufio"
	"context"
	"encoding/json"
	"net/http"
	"net/url"
	"strings"
	"testing"

	"github.com/google/uuid"
	"github.com/riverqueue/river"
	"github.com/riverqueue/river/rivertype"

	"github.com/yelnurq/startify/backend/internal/apigen"
	"github.com/yelnurq/startify/backend/internal/knowledge"
	"github.com/yelnurq/startify/backend/internal/platform/db"
	"github.com/yelnurq/startify/backend/internal/platform/tenancy"
)

type sseEvent struct {
	name string
	data string
}

func readEvents(t *testing.T, body string) []sseEvent {
	t.Helper()
	var out []sseEvent
	var cur sseEvent
	sc := bufio.NewScanner(strings.NewReader(body))
	for sc.Scan() {
		line := sc.Text()
		switch {
		case strings.HasPrefix(line, "event: "):
			cur.name = strings.TrimPrefix(line, "event: ")
		case strings.HasPrefix(line, "data: "):
			cur.data = strings.TrimPrefix(line, "data: ")
		case line == "" && cur.name != "":
			out = append(out, cur)
			cur = sseEvent{}
		}
	}
	return out
}

func TestAISettingsCheckAndUsage(t *testing.T) {
	h := newHarness(t)
	ts := seedTenants(t, h)
	admin := seedAdmin(t, h, ts)
	student := h.browser(t, kazutbHost)
	register(t, student, "ai-student@qadam.test", "Әлия Нұрболқызы")

	rec := student.do(http.MethodGet, "/api/v1/ai/status", nil)
	expect(t, rec, http.StatusOK)
	h.validate(t, http.MethodGet, "/api/v1/ai/status", rec)
	if s := decode[apigen.AiStatus](t, rec); s.External || s.DailyLimit != 200000 || s.MentorMode != apigen.MentorModeCoach {
		t.Fatalf("status %+v", s)
	}

	expectProblem(t, student.do(http.MethodGet, "/api/v1/admin/ai/settings", nil), http.StatusForbidden, "rbac.forbidden")
	rec = admin.do(http.MethodGet, "/api/v1/admin/ai/settings", nil)
	expect(t, rec, http.StatusOK)
	h.validate(t, http.MethodGet, "/api/v1/admin/ai/settings", rec)
	settings := decode[apigen.AiSettings](t, rec)
	etag := rec.Header().Get("ETag")
	if len(settings.Providers) == 0 || settings.Providers[0].Name != "mock" || settings.DefaultRoutes["mentor"][0].Provider != "mock" || etag == "" {
		t.Fatalf("settings %+v %q", settings, etag)
	}

	update := apigen.AiSettingsUpdate{
		Routes:          apigen.AiRoutes{"mentor": {{Provider: "mock"}}},
		LocalOnly:       true,
		DailyUserTokens: 50000, DailyTenantTokens: 5_000_000, MentorMode: apigen.MentorModeCoauthor,
	}
	ifMatch := func(v string) func(*http.Request) { return func(r *http.Request) { r.Header.Set("If-Match", v) } }
	rec = admin.do(http.MethodPut, "/api/v1/admin/ai/settings", update, ifMatch(etag))
	expect(t, rec, http.StatusOK)
	h.validate(t, http.MethodPut, "/api/v1/admin/ai/settings", rec)
	if s := decode[apigen.AiSettings](t, rec); !s.LocalOnly || s.MentorMode != apigen.MentorModeCoauthor || s.DailyUserTokens != 50000 {
		t.Fatalf("saved %+v", s)
	}
	expectProblem(t, admin.do(http.MethodPut, "/api/v1/admin/ai/settings", update, ifMatch(etag)), http.StatusPreconditionFailed, "precondition.failed")
	bad := update
	bad.Routes = apigen.AiRoutes{"mentor": {{Provider: "skynet"}}}
	expectProblem(t, admin.do(http.MethodPut, "/api/v1/admin/ai/settings", bad), http.StatusUnprocessableEntity, "validation.failed")

	// The connection check streams the answer as server-sent events.
	rec = admin.do(http.MethodPost, "/api/v1/admin/ai/check", apigen.AiCheckRequest{Feature: apigen.AiFeatureMentor},
		func(r *http.Request) { r.Header.Set("Accept", "text/event-stream") })
	expect(t, rec, http.StatusOK)
	if !strings.HasPrefix(rec.Header().Get("Content-Type"), "text/event-stream") {
		t.Fatalf("content type %q", rec.Header().Get("Content-Type"))
	}
	events := readEvents(t, rec.Body.String())
	var text strings.Builder
	for _, ev := range events[:len(events)-1] {
		var d struct {
			Text string `json:"text"`
		}
		if ev.name != "delta" || json.Unmarshal([]byte(ev.data), &d) != nil {
			t.Fatalf("event %+v", ev)
		}
		text.WriteString(d.Text)
	}
	last := events[len(events)-1]
	var done struct {
		Provider      string `json:"provider"`
		PromptVersion string `json:"prompt_version"`
	}
	if last.name != "done" || json.Unmarshal([]byte(last.data), &done) != nil || done.Provider != "mock" ||
		done.PromptVersion != "connection_check@1" || text.Len() == 0 {
		t.Fatalf("done %+v text %q", last, text.String())
	}
	rec = admin.do(http.MethodPost, "/api/v1/admin/ai/check", apigen.AiCheckRequest{Feature: apigen.AiFeatureEmbedding})
	if events := readEvents(t, rec.Body.String()); len(events) != 2 || !strings.Contains(events[0].data, "mock-hash-256") {
		t.Fatalf("embedding check %+v", events)
	}

	rec = admin.do(http.MethodGet, "/api/v1/admin/ai/usage?days=7", nil)
	expect(t, rec, http.StatusOK)
	h.validate(t, http.MethodGet, "/api/v1/admin/ai/usage", rec)
	report := decode[apigen.AiUsageReport](t, rec)
	if report.Totals.Requests != 2 || len(report.ByFeature) != 2 || len(report.ByDay) != 1 || report.ByModel[0].Provider != "mock" {
		t.Fatalf("usage %+v", report)
	}
}

func TestKnowledgeBase(t *testing.T) {
	h := newHarness(t)
	ts := seedTenants(t, h)
	admin := seedAdmin(t, h, ts)
	student := h.browser(t, kazutbHost)
	register(t, student, "kb-student@qadam.test", "Бекжан Оразов")

	body := "Раздел 1. Сроки\n\nЗаявки на конкурс стартап-проектов КазУТБ принимаются на платформе Qadam до 1 декабря.\n\n" +
		"Раздел 2. Команда\n\nКоманда должна состоять не менее чем из двух студентов; руководитель — преподаватель вуза."
	expectProblem(t, student.do(http.MethodPost, "/api/v1/admin/knowledge",
		multipartForm(t, map[string]string{"title": "x", "text": body}, "file", "x.txt", []byte(body))), http.StatusForbidden, "rbac.forbidden")
	expectProblem(t, admin.do(http.MethodPost, "/api/v1/admin/knowledge",
		multipartForm(t, map[string]string{"title": "Фото"}, "file", "photo.png", []byte{0x89, 'P', 'N', 'G', 1, 2, 3})),
		http.StatusUnprocessableEntity, "knowledge.unsupported_file")

	rec := admin.do(http.MethodPost, "/api/v1/admin/knowledge",
		multipartForm(t, map[string]string{"title": "Положение о конкурсе стартап-проектов"}, "file", "polozhenie.txt", []byte(body)))
	expect(t, rec, http.StatusCreated)
	h.validate(t, http.MethodPost, "/api/v1/admin/knowledge", rec)
	doc := decode[apigen.KnowledgeDocument](t, rec)
	if doc.Status != apigen.KnowledgeStatusPending || doc.FileUrl == nil || doc.Characters == 0 {
		t.Fatalf("created %+v", doc)
	}

	// Run the indexing job the API enqueued.
	var args knowledge.IndexArgs
	for _, j := range h.queue.jobs {
		if a, ok := j.(knowledge.IndexArgs); ok {
			args = a
		}
	}
	worker := &knowledge.IndexWorker{DB: h.db.App, Gateway: h.services.AI}
	if err := worker.Work(context.Background(), &river.Job[knowledge.IndexArgs]{Args: args, JobRow: jobRow()}); err != nil {
		t.Fatal(err)
	}
	rec = admin.do(http.MethodGet, "/api/v1/admin/knowledge/"+doc.Id.String(), nil)
	expect(t, rec, http.StatusOK)
	h.validate(t, http.MethodGet, "/api/v1/admin/knowledge/"+doc.Id.String(), rec)
	if got := decode[apigen.KnowledgeDocument](t, rec); got.Status != apigen.KnowledgeStatusReady || got.ChunkCount != 2 {
		t.Fatalf("indexed %+v", got)
	}

	rec = admin.do(http.MethodGet, "/api/v1/admin/knowledge/search?q="+url.QueryEscape("до какого числа принимаются заявки"), nil)
	expect(t, rec, http.StatusOK)
	h.validate(t, http.MethodGet, "/api/v1/admin/knowledge/search", rec)
	hits := decode[struct{ Items []apigen.KnowledgeHit }](t, rec).Items
	if len(hits) == 0 || !strings.Contains(hits[0].Excerpt, "1 декабря") || *hits[0].Heading != "Раздел 1. Сроки" {
		t.Fatalf("hits %+v", hits)
	}

	// The mentor's search is scoped to the university.
	other := tenancy.Tenant{ID: ts.sandbox}
	ctx := db.WithTenant(tenancy.With(context.Background(), other), ts.sandbox)
	if found, err := h.services.Knowledge.Search(ctx, "заявки на конкурс", 5); err != nil || len(found) != 0 {
		t.Fatalf("sandbox found %+v %v", found, err)
	}

	list := decode[apigen.KnowledgeDocumentPage](t, admin.do(http.MethodGet, "/api/v1/admin/knowledge", nil))
	if len(list.Items) != 1 {
		t.Fatalf("list %+v", list)
	}
	expect(t, admin.do(http.MethodDelete, "/api/v1/admin/knowledge/"+doc.Id.String(), nil), http.StatusNoContent)
	expectProblem(t, admin.do(http.MethodGet, "/api/v1/admin/knowledge/"+doc.Id.String(), nil), http.StatusNotFound, "knowledge.not_found")
}

func TestKnowledgeReindex(t *testing.T) {
	h := newHarness(t)
	ts := seedTenants(t, h)
	admin := seedAdmin(t, h, ts)
	text := []byte("Раздел 1. Гранты\n\nГрант выдаётся на 12 месяцев и расходуется по смете.")
	rec := admin.do(http.MethodPost, "/api/v1/admin/knowledge",
		multipartForm(t, map[string]string{"title": "Правила грантов"}, "file", "granty.txt", text))
	expect(t, rec, http.StatusCreated)
	doc := decode[apigen.KnowledgeDocument](t, rec)

	reindex := func() []uuid.UUID {
		t.Helper()
		queue := &captureQueue{}
		w := &knowledge.ReindexWorker{DB: h.db.App, Gateway: h.services.AI, Queue: queue}
		if err := w.Work(context.Background(), &river.Job[knowledge.ReindexArgs]{JobRow: jobRow()}); err != nil {
			t.Fatal(err)
		}
		var ids []uuid.UUID
		for _, j := range queue.jobs {
			ids = append(ids, j.(knowledge.IndexArgs).DocumentID)
		}
		return ids
	}
	// A pending document is scheduled again (the queue keeps one job per document).
	if ids := reindex(); len(ids) != 1 || ids[0] != doc.Id {
		t.Fatalf("pending: %v", ids)
	}
	worker := &knowledge.IndexWorker{DB: h.db.App, Gateway: h.services.AI}
	job := &river.Job[knowledge.IndexArgs]{Args: knowledge.IndexArgs{TenantID: ts.kazutb, DocumentID: doc.Id}, JobRow: jobRow()}
	if err := worker.Work(context.Background(), job); err != nil {
		t.Fatal(err)
	}
	if ids := reindex(); len(ids) != 0 {
		t.Fatalf("indexed with the current model: %v", ids)
	}

	// Passages of another model are indexed again.
	h.db.Exec(t, "UPDATE knowledge_chunks SET embedding_model = 'retired-model'")
	if ids := reindex(); len(ids) != 1 || ids[0] != doc.Id {
		t.Fatalf("after a model change: %v", ids)
	}
	// A failed document waits before the next attempt.
	h.db.Exec(t, "UPDATE knowledge_documents SET status = 'failed'")
	if ids := reindex(); len(ids) != 0 {
		t.Fatalf("recently failed: %v", ids)
	}
	h.db.Exec(t, "SET session_replication_role = replica",
		"UPDATE knowledge_documents SET updated_at = now() - interval '7 hours'")
	if ids := reindex(); len(ids) != 1 {
		t.Fatalf("failed long ago: %v", ids)
	}
}

func jobRow() *rivertype.JobRow { return &rivertype.JobRow{Attempt: 1, MaxAttempts: 5} }

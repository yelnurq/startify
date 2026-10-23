package app

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/riverqueue/river"
	"github.com/riverqueue/river/rivertype"

	"github.com/yelnurq/startify/backend/internal/ai"
	"github.com/yelnurq/startify/backend/internal/ai/llm"
	"github.com/yelnurq/startify/backend/internal/ai/mock"
	"github.com/yelnurq/startify/backend/internal/apigen"
	"github.com/yelnurq/startify/backend/internal/evaluation"
	"github.com/yelnurq/startify/backend/internal/files"
	"github.com/yelnurq/startify/backend/internal/incubator"
	"github.com/yelnurq/startify/backend/internal/knowledge"
	"github.com/yelnurq/startify/backend/internal/platform/config"
	"github.com/yelnurq/startify/backend/internal/platform/gotenberg"
	"github.com/yelnurq/startify/backend/internal/platform/maintenance"
)

// scalar reads one value as the database owner (RLS does not apply).
func scalar[T any](t *testing.T, h *harness, query string, args ...any) T {
	t.Helper()
	ctx := context.Background()
	conn, err := pgx.Connect(ctx, h.db.OwnerURL)
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = conn.Close(ctx) }()
	var v T
	if err := conn.QueryRow(ctx, query, args...).Scan(&v); err != nil {
		t.Fatalf("%s: %v", query, err)
	}
	return v
}

func lastJob[T river.JobArgs](t *testing.T, h *harness) T {
	t.Helper()
	for i := len(h.queue.jobs) - 1; i >= 0; i-- {
		if a, ok := h.queue.jobs[i].(T); ok {
			return a
		}
	}
	var zero T
	t.Fatalf("no %s job", zero.Kind())
	return zero
}

// The last attempt of a job often fails because its deadline passed; the
// failure is still recorded, so nothing stays "in progress" forever and the
// project can be evaluated again (В-3, Н-17, С-17).
func TestJobFailuresSurviveTheirDeadline(t *testing.T) {
	h := newHarness(t)
	ts := seedTenants(t, h)
	owner, _ := account(t, h, ts, "owner@qadam.test", "Нұрлан Ахметов", "student")
	admin := seedAdmin(t, h, ts)
	p := newProject(t, h, owner, "AgroScan")
	base := "/api/v1/projects/" + p.Id.String()
	expect(t, owner.do(http.MethodPatch, base+"/steps/problem", apigen.StepUpdate{Values: map[string]any{
		"statement": "Фермеры поздно замечают болезни пшеницы и теряют урожай.",
	}}, ifMatchHeader(`"0"`)), http.StatusOK)
	expired, cancel := context.WithDeadline(context.Background(), time.Now().Add(-time.Second))
	defer cancel()
	last := func(n int) *rivertype.JobRow { return &rivertype.JobRow{Attempt: n, MaxAttempts: n} }

	first := decode[apigen.Evaluation](t, owner.do(http.MethodPost, base+"/evaluations", nil))
	w := &evaluation.RunWorker{DB: h.db.App, Gateway: h.services.AI}
	if err := w.Work(expired, &river.Job[evaluation.RunArgs]{Args: lastJob[evaluation.RunArgs](t, h), JobRow: last(3)}); err != nil {
		t.Fatalf("the failure must be recorded: %v", err)
	}
	e := decode[apigen.Evaluation](t, owner.do(http.MethodGet, base+"/evaluations/"+first.Id.String(), nil))
	if e.Status != apigen.EvaluationStatusFailed || e.Error == nil || *e.Error != "evaluation.timed_out" {
		t.Fatalf("evaluation %+v", e)
	}
	expect(t, owner.do(http.MethodPost, base+"/evaluations", nil), http.StatusAccepted)

	h.queue.jobs = nil
	expect(t, owner.do(http.MethodPost, base+"/materials", apigen.MaterialCreate{Kind: apigen.MaterialKindOnePager, Format: apigen.MaterialFormatPdf}), http.StatusAccepted)
	mw := &incubator.MaterialWorker{DB: h.db.App, Gateway: h.services.AI, Files: h.services.Files, Gotenberg: gotenberg.New("http://gotenberg.invalid", time.Second)}
	if err := mw.Work(expired, &river.Job[incubator.MaterialArgs]{Args: lastJob[incubator.MaterialArgs](t, h), JobRow: last(3)}); err != nil {
		t.Fatalf("the failure must be recorded: %v", err)
	}
	m := decode[apigen.MaterialList](t, owner.do(http.MethodGet, base+"/materials", nil)).Items[0]
	if m.Status != apigen.MaterialStatusFailed || m.Error == nil || *m.Error != "incubator.material_timed_out" {
		t.Fatalf("material %+v", m)
	}

	rec := admin.do(http.MethodPost, "/api/v1/admin/knowledge",
		multipartForm(t, map[string]string{"title": "Правила грантов"}, "file", "granty.txt", []byte("Раздел 1. Гранты\n\nГрант выдаётся на 12 месяцев.")))
	expect(t, rec, http.StatusCreated)
	doc := decode[apigen.KnowledgeDocument](t, rec)
	iw := &knowledge.IndexWorker{DB: h.db.App, Gateway: h.services.AI}
	if err := iw.Work(expired, &river.Job[knowledge.IndexArgs]{Args: lastJob[knowledge.IndexArgs](t, h), JobRow: last(5)}); err != nil {
		t.Fatalf("the failure must be recorded: %v", err)
	}
	got := decode[apigen.KnowledgeDocument](t, admin.do(http.MethodGet, "/api/v1/admin/knowledge/"+doc.Id.String(), nil))
	if got.Status != apigen.KnowledgeStatusFailed || got.Error == nil || *got.Error != "knowledge.index_failed" {
		t.Fatalf("document %+v", got)
	}
	if n := scalar[int32](t, h, "SELECT index_failures FROM knowledge_documents WHERE id = $1", doc.Id); n != 1 {
		t.Fatalf("index failures %d", n)
	}
}

// Work whose job is gone (a crashed worker, a discarded job) is given up by
// the periodic maintenance job after its worst case, not before (В-3).
func TestStaleBackgroundWorkIsGivenUp(t *testing.T) {
	h := newHarness(t)
	ts := seedTenants(t, h)
	owner, _ := account(t, h, ts, "owner@qadam.test", "Нұрлан Ахметов", "student")
	admin := seedAdmin(t, h, ts)
	p := newProject(t, h, owner, "AgroScan")
	base := "/api/v1/projects/" + p.Id.String()
	expect(t, owner.do(http.MethodPatch, base+"/steps/problem", apigen.StepUpdate{Values: map[string]any{
		"statement": "Фермеры поздно замечают болезни пшеницы и теряют урожай.",
	}}, ifMatchHeader(`"0"`)), http.StatusOK)
	stuck := decode[apigen.Evaluation](t, owner.do(http.MethodPost, base+"/evaluations", nil))
	expectProblem(t, owner.do(http.MethodPost, base+"/evaluations", nil), http.StatusConflict, "evaluation.in_progress")
	expect(t, owner.do(http.MethodPost, base+"/materials", apigen.MaterialCreate{Kind: apigen.MaterialKindOnePager, Format: apigen.MaterialFormatPdf}), http.StatusAccepted)
	expect(t, admin.do(http.MethodPost, "/api/v1/admin/knowledge",
		multipartForm(t, map[string]string{"title": "Правила"}, "file", "rules.txt", []byte("Раздел 1. Гранты\n\nГрант выдаётся на 12 месяцев."))), http.StatusCreated)
	// The worker died during the evaluation.
	h.db.Exec(t, "UPDATE evaluations SET status = 'running'")

	materials := &incubator.MaterialWorker{DB: h.db.App, Gateway: h.services.AI, Files: h.services.Files}
	stale := &maintenance.StaleWorker{Reapers: []maintenance.Reaper{
		{Name: "evaluations", Fn: (&evaluation.RunWorker{DB: h.db.App, Gateway: h.services.AI}).FailStale},
		{Name: "materials", Fn: materials.FailStale},
		{Name: "knowledge_documents", Fn: (&knowledge.IndexWorker{DB: h.db.App, Gateway: h.services.AI}).FailStale},
		{Name: "ai_reservations", Fn: h.services.AI.PurgeReservations},
	}}
	run := func(at time.Time) {
		t.Helper()
		stale.Now = func() time.Time { return at }
		if err := stale.Work(context.Background(), &river.Job[maintenance.StaleArgs]{JobRow: jobRow()}); err != nil {
			t.Fatal(err)
		}
	}
	// Within the worst case of their jobs nothing is touched.
	run(time.Now().Add(30 * time.Minute))
	if n := scalar[int64](t, h, "SELECT count(*) FROM evaluations WHERE status = 'running'"); n != 1 {
		t.Fatalf("a running evaluation was given up too early: %d", n)
	}
	run(time.Now().Add(6 * time.Hour))
	e := decode[apigen.Evaluation](t, owner.do(http.MethodGet, base+"/evaluations/"+stuck.Id.String(), nil))
	if e.Status != apigen.EvaluationStatusFailed || e.Error == nil || *e.Error != "evaluation.timed_out" {
		t.Fatalf("evaluation %+v", e)
	}
	if m := decode[apigen.MaterialList](t, owner.do(http.MethodGet, base+"/materials", nil)).Items[0]; m.Status != apigen.MaterialStatusFailed {
		t.Fatalf("material %+v", m)
	}
	if n := scalar[int64](t, h, "SELECT count(*) FROM knowledge_documents WHERE status = 'failed' AND error = 'knowledge.index_failed'"); n != 1 {
		t.Fatalf("documents given up: %d", n)
	}
	// The project is no longer blocked.
	expect(t, owner.do(http.MethodPost, base+"/evaluations", nil), http.StatusAccepted)
}

// The evaluation records the versions of the steps it actually read, also
// when the job runs after the team changed the project (Н-25).
func TestEvaluationRecordsTheVersionsItRead(t *testing.T) {
	h := newHarness(t)
	ts := seedTenants(t, h)
	owner, _ := account(t, h, ts, "owner@qadam.test", "Нұрлан Ахметов", "student")
	p := newProject(t, h, owner, "AgroScan")
	base := "/api/v1/projects/" + p.Id.String()
	expect(t, owner.do(http.MethodPatch, base+"/steps/problem", apigen.StepUpdate{Values: map[string]any{"statement": "Фермеры поздно замечают болезни."}},
		ifMatchHeader(`"0"`)), http.StatusOK)
	e := decode[apigen.Evaluation](t, owner.do(http.MethodPost, base+"/evaluations", nil))
	expect(t, owner.do(http.MethodPatch, base+"/steps/problem", apigen.StepUpdate{Values: map[string]any{"who": "Хозяйства Костанайской области"}},
		ifMatchHeader(`"1"`)), http.StatusOK)
	runEvaluation(t, h)
	var versions map[string]int
	if err := json.Unmarshal([]byte(scalar[string](t, h, "SELECT step_versions::text FROM evaluations WHERE id = $1", e.Id)), &versions); err != nil {
		t.Fatal(err)
	}
	if versions["problem"] != 2 {
		t.Fatalf("versions %v: the evaluation read version 2 of the problem", versions)
	}
}

// A project keeps its newest finished materials; older ones go with their
// files instead of piling up in storage (Н-17).
func TestOldMaterialsAreRemoved(t *testing.T) {
	h := newHarness(t)
	ts := seedTenants(t, h)
	owner, _ := account(t, h, ts, "owner@qadam.test", "Нұрлан Ахметов", "student")
	p := newProject(t, h, owner, "AgroScan")
	base := "/api/v1/projects/" + p.Id.String()
	h.db.Exec(t, "INSERT INTO files (id, tenant_id, purpose, visibility, object_key, file_name, content_type, size_bytes, sha256, scan_status) "+
		"SELECT gen_random_uuid(), tenant_id, 'generated', 'private', 'old/' || g, 'old.pdf', 'application/pdf', 1, '', 'clean' "+
		"FROM projects, generate_series(1, 25) g WHERE id = '"+p.Id.String()+"'",
		"INSERT INTO project_materials (tenant_id, project_id, kind, format, locale, status, file_id) "+
			"SELECT f.tenant_id, '"+p.Id.String()+"', 'one_pager', 'pdf', 'ru', 'ready', f.id FROM files f WHERE f.object_key LIKE 'old/%' ORDER BY f.object_key")
	h.queue.jobs = nil
	expect(t, owner.do(http.MethodPost, base+"/materials", apigen.MaterialCreate{Kind: apigen.MaterialKindPitchDeck, Format: apigen.MaterialFormatPptx}), http.StatusAccepted)
	w := &incubator.MaterialWorker{DB: h.db.App, Gateway: h.services.AI, Files: h.services.Files}
	if err := w.Work(context.Background(), &river.Job[incubator.MaterialArgs]{Args: lastJob[incubator.MaterialArgs](t, h), JobRow: jobRow()}); err != nil {
		t.Fatal(err)
	}
	if n := scalar[int64](t, h, "SELECT count(*) FROM project_materials"); n != 20 {
		t.Fatalf("%d materials kept", n)
	}
	if n := scalar[int64](t, h, "SELECT count(*) FROM files WHERE object_key LIKE 'old/%' AND deleted_at IS NOT NULL"); n != 6 {
		t.Fatalf("%d old files deleted", n)
	}
	if m := decode[apigen.MaterialList](t, owner.do(http.MethodGet, base+"/materials", nil)).Items[0]; m.Status != apigen.MaterialStatusReady || m.Kind != apigen.MaterialKindPitchDeck {
		t.Fatalf("newest %+v", m)
	}
}

// flaky embeds the first batch and then goes down.
type flaky struct{ calls int }

func (f *flaky) Name() string                    { return "openai" }
func (f *flaky) Local() bool                     { return true }
func (f *flaky) DefaultModel(llm.Feature) string { return "flaky-embedding" }
func (f *flaky) Complete(context.Context, llm.CompletionRequest) (*llm.CompletionResponse, error) {
	return nil, llm.ErrUnavailable
}

func (f *flaky) Stream(context.Context, llm.CompletionRequest) (<-chan llm.StreamEvent, error) {
	return nil, llm.ErrUnavailable
}

func (f *flaky) Embed(_ context.Context, texts []string) ([][]float32, string, error) {
	f.calls++
	if f.calls > 1 {
		return nil, "", llm.ErrUnavailable
	}
	out := make([][]float32, len(texts))
	for i := range out {
		out[i] = []float32{1, 0, 0}
	}
	return out, "flaky-embedding", nil
}

// When the embedding route falls back in the middle of a document, its
// passages would mix vectors of two models: the indexing is retried
// instead (Н-26).
func TestDocumentPassagesShareOneEmbeddingModel(t *testing.T) {
	h := newHarness(t)
	ts := seedTenants(t, h)
	admin := seedAdmin(t, h, ts)
	var body strings.Builder
	for i := range 40 {
		fmt.Fprintf(&body, "Пункт %d. %s\n\n", i+1, strings.Repeat("Участники конкурса подают заявку с питч-деком и паспортом стартапа. ", 14))
	}
	rec := admin.do(http.MethodPost, "/api/v1/admin/knowledge", multipartForm(t, map[string]string{"title": "Длинное положение", "text": body.String()}, "", "", nil))
	expect(t, rec, http.StatusCreated)
	g, err := ai.NewGateway(h.db.App, []llm.Provider{&mock.Provider{}, &flaky{}},
		config.AI{Default: "mock", Routes: "embedding=openai,mock", Timezone: "Asia/Almaty"}, slog.New(slog.NewTextHandler(io.Discard, nil)))
	if err != nil {
		t.Fatal(err)
	}
	w := &knowledge.IndexWorker{DB: h.db.App, Gateway: g}
	err = w.Work(context.Background(), &river.Job[knowledge.IndexArgs]{Args: lastJob[knowledge.IndexArgs](t, h), JobRow: jobRow()})
	if err == nil || !strings.Contains(err.Error(), "different models") {
		t.Fatalf("mixed models must fail the attempt: %v", err)
	}
	if n := scalar[int64](t, h, "SELECT count(*) FROM knowledge_chunks"); n != 0 {
		t.Fatalf("%d passages stored", n)
	}
}

type infected struct{}

func (infected) Scan(context.Context, []byte) (files.Verdict, error) {
	return files.Verdict{Clean: false, Signature: "Eicar-Test-Signature"}, nil
}

// An uploaded document is checked for malware before a parser reads it; a
// file the parser rejects is not kept; extracted text has a limit (Н-28).
func TestKnowledgeUploadIsScannedBeforeParsing(t *testing.T) {
	h := newHarness(t)
	ts := seedTenants(t, h)
	admin := seedAdmin(t, h, ts)
	upload := func(name string, data []byte) *httptest.ResponseRecorder {
		return admin.do(http.MethodPost, "/api/v1/admin/knowledge", multipartForm(t, map[string]string{"title": "Документ"}, "file", name, data))
	}
	expectProblem(t, upload("scan.pdf", []byte("%PDF-1.4 garbage")), http.StatusUnprocessableEntity, "knowledge.unsupported_file")
	if n := scalar[int64](t, h, "SELECT count(*) FROM files WHERE purpose = 'knowledge' AND deleted_at IS NULL"); n != 0 {
		t.Fatalf("a rejected upload must not be kept: %d files", n)
	}
	expectProblem(t, upload("huge.txt", []byte(strings.Repeat("Длинный регламент. ", 300_000))), http.StatusUnprocessableEntity, "knowledge.text_too_long")

	h.services.Files.WithScanner(infected{})
	expectProblem(t, upload("scan.pdf", []byte("%PDF-1.4 garbage")), http.StatusUnprocessableEntity, "files.infected")
}

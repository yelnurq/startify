package seed

import (
	"context"
	"log/slog"
	"strings"
	"testing"

	"github.com/jackc/pgx/v5"
	"github.com/riverqueue/river"
	"github.com/riverqueue/river/rivertype"

	"github.com/yelnurq/startify/backend/internal/ai"
	"github.com/yelnurq/startify/backend/internal/ai/llm"
	"github.com/yelnurq/startify/backend/internal/ai/mock"
	"github.com/yelnurq/startify/backend/internal/knowledge"
	"github.com/yelnurq/startify/backend/internal/knowledge/knowledgedb"
	"github.com/yelnurq/startify/backend/internal/platform/config"
	"github.com/yelnurq/startify/backend/internal/platform/db"
	"github.com/yelnurq/startify/backend/internal/platform/tenancy"
	"github.com/yelnurq/startify/backend/internal/platform/testpg"
	"github.com/yelnurq/startify/backend/internal/tenant/tenantdb"
)

var pg *testpg.Server

func TestMain(m *testing.M) { testpg.Main(m, &pg) }

// tenantTables are the tables isolated by row-level security.
var tenantTables = []string{
	"tenant_settings", "tenant_oidc_providers", "faculties", "departments", "study_groups", "memberships",
	"sessions", "auth_tokens", "files", "audit_log", "import_jobs", "team_requests", "team_responses",
	"tenant_ai_settings", "ai_usage", "knowledge_documents", "knowledge_chunks",
	"projects", "project_members", "project_step_data", "project_revisions", "project_comments",
	"ai_conversations", "ai_messages", "project_materials",
	"evaluation_rubrics", "evaluations", "evaluation_criteria_scores", "evaluation_overrides",
	"contact_requests",
}

// counter counts the rows a data scope can see.
type counter struct {
	t  *testing.T
	db *db.DB
}

func (c counter) rows(ctx context.Context, table string) int {
	c.t.Helper()
	var n int
	err := c.db.ReadTx(ctx, func(ctx context.Context) error {
		return c.db.Conn(ctx).QueryRow(ctx, "SELECT count(*) FROM "+pgx.Identifier{table}.Sanitize()).Scan(&n)
	})
	if err != nil {
		c.t.Fatalf("count %s: %v", table, err)
	}
	return n
}

func TestDemoIsIdempotentAndIsolated(t *testing.T) {
	database := pg.NewDB(t)
	ctx := context.Background()
	if err := Demo(ctx, database.App); err != nil {
		t.Fatal(err)
	}
	if err := Demo(ctx, database.App); err != nil {
		t.Fatalf("second run: %v", err)
	}

	var kazutb, sandbox tenantdb.Tenant
	q := tenantdb.New(database.App.Conn(ctx))
	kazutb, err := q.GetTenantBySlug(ctx, "kazutb")
	if err != nil {
		t.Fatal(err)
	}
	if sandbox, err = q.GetTenantBySlug(ctx, "sandbox"); err != nil {
		t.Fatal(err)
	}

	c := counter{t: t, db: database.App}
	k := db.WithTenant(ctx, kazutb.ID)
	s := db.WithTenant(ctx, sandbox.ID)
	if n := c.rows(k, "memberships"); n != len(AllKazUTBMembers()) {
		t.Fatalf("kazutb members: %d", n)
	}
	if n := c.rows(k, "team_requests"); n != len(KazUTBRequests) {
		t.Fatalf("team requests after two runs: %d", n)
	}
	if n := c.rows(k, "study_groups"); n == 0 {
		t.Fatal("no groups")
	}
	if n := c.rows(k, "knowledge_documents"); n != len(KazUTBKnowledge) {
		t.Fatalf("knowledge documents after two runs: %d", n)
	}
	var jobs int
	if err := database.App.Pool().QueryRow(ctx, "SELECT count(*) FROM river_job WHERE kind = 'knowledge.index'").Scan(&jobs); err != nil {
		t.Fatal(err)
	}
	if jobs != len(KazUTBKnowledge) {
		t.Fatalf("index jobs: %d", jobs)
	}
	if n := c.rows(k, "projects"); n != len(KazUTBProjects) {
		t.Fatalf("projects after two runs: %d", n)
	}
	done := 0
	for i := range KazUTBProjects {
		done += KazUTBProjects[i].Done
	}
	var completed, completedRows int
	err = database.App.ReadTx(k, func(ctx context.Context) error {
		conn := database.App.Conn(ctx)
		if err := conn.QueryRow(ctx, "SELECT coalesce(sum(completed_steps), 0) FROM projects").Scan(&completed); err != nil {
			return err
		}
		return conn.QueryRow(ctx, "SELECT count(*) FROM project_step_data WHERE status = 'completed'").Scan(&completedRows)
	})
	if err != nil {
		t.Fatal(err)
	}
	if completed != done || completedRows != done {
		t.Fatalf("completed steps: %d in projects, %d step rows, want %d", completed, completedRows, done)
	}
	evaluated := 0
	for i := range KazUTBProjects {
		if KazUTBProjects[i].Done >= evaluatedFrom {
			evaluated++
		}
	}
	if n := c.rows(k, "evaluations"); n != 2*evaluated || c.rows(k, "evaluation_overrides") != 1 {
		t.Fatalf("evaluations after two runs: %d, want %d", n, 2*evaluated)
	}
	var changed int
	err = database.App.ReadTx(k, func(ctx context.Context) error {
		return database.App.Conn(ctx).QueryRow(ctx,
			"SELECT count(*) FROM evaluation_criteria_scores WHERE score <> ai_score").Scan(&changed)
	})
	if err != nil || changed != 1 {
		t.Fatalf("the expert correction must change a score: %d %v", changed, err)
	}

	// Every tenant table shows only the rows of the scope; without a scope, none.
	for _, table := range tenantTables {
		all := c.rows(db.WithSystem(ctx), table)
		mine := c.rows(k, table)
		theirs := c.rows(s, table)
		if mine+theirs != all {
			t.Errorf("%s: kazutb %d + sandbox %d != all %d", table, mine, theirs, all)
		}
		if n := c.rows(ctx, table); n != 0 {
			t.Errorf("%s: %d rows visible without a tenant", table, n)
		}
	}
	// Showcase pages are tenant data too, except published public pages:
	// the national showcase reads them from every university.
	public := 0
	for _, page := range KazUTBShowcase {
		if page.Public && page.Status == "published" {
			public++
		}
	}
	if n := c.rows(k, "showcase_entries"); n != len(KazUTBShowcase) {
		t.Fatalf("kazutb showcase pages after two runs: %d", n)
	}
	if theirs, none := c.rows(s, "showcase_entries"), c.rows(ctx, "showcase_entries"); theirs != public || none != public {
		t.Fatalf("showcase pages outside kazutb: sandbox %d, no tenant %d, want the %d public ones", theirs, none, public)
	}
	if c.rows(s, "memberships") != 1 || c.rows(s, "faculties") != 0 {
		t.Fatal("sandbox sees KazUTB data")
	}

	// Rows cannot be written into another tenant either.
	err = database.App.Tx(s, func(ctx context.Context) error {
		_, err := database.App.Conn(ctx).Exec(ctx,
			"INSERT INTO faculties (tenant_id, code, name) VALUES ($1, 'X', '{}')", kazutb.ID)
		return err
	})
	if err == nil {
		t.Fatal("cross-tenant insert succeeded")
	}
}

func TestDemoKnowledgeAnswersQuestions(t *testing.T) {
	database := pg.NewDB(t)
	ctx := context.Background()
	if err := Demo(ctx, database.App); err != nil {
		t.Fatal(err)
	}
	kazutb, err := tenantdb.New(database.App.Conn(ctx)).GetTenantBySlug(ctx, "kazutb")
	if err != nil {
		t.Fatal(err)
	}
	gateway, err := ai.NewGateway(database.App, []llm.Provider{&mock.Provider{}},
		config.AI{Default: "mock", Timezone: "Asia/Almaty"}, slog.New(slog.DiscardHandler))
	if err != nil {
		t.Fatal(err)
	}
	scoped := db.WithTenant(tenancy.With(ctx, tenancy.Tenant{ID: kazutb.ID}), kazutb.ID)
	var docs []knowledgedb.ListDocumentsRow
	err = database.App.ReadTx(scoped, func(ctx context.Context) error {
		var err error
		docs, err = knowledgedb.New(database.App.Conn(ctx)).ListDocuments(ctx, knowledgedb.ListDocumentsParams{RowLimit: 100})
		return err
	})
	if err != nil {
		t.Fatal(err)
	}
	worker := &knowledge.IndexWorker{DB: database.App, Gateway: gateway}
	for i := range docs {
		job := &river.Job[knowledge.IndexArgs]{
			Args:   knowledge.IndexArgs{TenantID: kazutb.ID, DocumentID: docs[i].ID},
			JobRow: &rivertype.JobRow{Attempt: 1, MaxAttempts: 5},
		}
		if err := worker.Work(ctx, job); err != nil {
			t.Fatal(err)
		}
	}

	// The passages the mentor cites (top three) answer the demo questions,
	// even with the bag-of-words vectors of the mock.
	svc := knowledge.NewService(database.App, gateway, nil, nil, nil)
	for question, answer := range map[string]string{
		"До какого числа принимаются заявки на конкурс стартап-проектов?": "1 декабря 2026",
		"До какого числа принимаются заявки на конкурс?":                  "1 декабря 2026",
		"Какие призы получат победители?":                                 "3 000 000 тенге",
		"Грант мөлшері қанша және ол қалай аударылады?":                   "1 500 000",
		"Who owns the result of a student project?":                       "belong to the authors",
		"Сколько интервью с пользователями нужно провести?":               "пяти",
	} {
		hits, err := svc.Search(scoped, question, 3)
		if err != nil {
			t.Fatal(err)
		}
		found := false
		for _, h := range hits {
			found = found || strings.Contains(h.Text, answer)
		}
		if !found {
			t.Errorf("%s: hits %+v", question, hits)
		}
	}
}

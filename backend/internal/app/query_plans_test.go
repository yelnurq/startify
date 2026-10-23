package app

import (
	"context"
	"encoding/json"
	"strings"
	"testing"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"

	"github.com/yelnurq/startify/backend/internal/knowledge/knowledgedb"
	"github.com/yelnurq/startify/backend/internal/platform/db"
	"github.com/yelnurq/startify/backend/internal/project/projectdb"
	"github.com/yelnurq/startify/backend/internal/team/teamdb"
)

// planRecorder passes queries on to a transaction and records the plan of
// each one, as the application role sees it (row-level security included).
type planRecorder struct {
	t     *testing.T
	tx    db.Querier
	plans []json.RawMessage
}

func (p *planRecorder) explain(ctx context.Context, sql string, args ...any) {
	p.t.Helper()
	var plan json.RawMessage
	if err := p.tx.QueryRow(ctx, "EXPLAIN (FORMAT JSON) "+sql, args...).Scan(&plan); err != nil {
		p.t.Fatalf("explain: %v\n%s", err, sql)
	}
	p.plans = append(p.plans, plan)
}

func (p *planRecorder) Exec(ctx context.Context, sql string, args ...any) (pgconn.CommandTag, error) {
	return p.tx.Exec(ctx, sql, args...)
}

func (p *planRecorder) Query(ctx context.Context, sql string, args ...any) (pgx.Rows, error) {
	p.explain(ctx, sql, args...)
	return p.tx.Query(ctx, sql, args...)
}

func (p *planRecorder) QueryRow(ctx context.Context, sql string, args ...any) pgx.Row {
	p.explain(ctx, sql, args...)
	return p.tx.QueryRow(ctx, sql, args...)
}

// planNode is the part of an EXPLAIN (FORMAT JSON) node the test reads.
type planNode struct {
	NodeType    string     `json:"Node Type"`
	Relation    string     `json:"Relation Name"`
	Index       string     `json:"Index Name"`
	IndexCond   string     `json:"Index Cond"`
	RecheckCond string     `json:"Recheck Cond"`
	Plans       []planNode `json:"Plans"`
}

// scans lists the nodes of a plan that read the table, with what the index
// is searched by (bitmap heap scans take their condition from the index
// scans below them).
func scans(raw json.RawMessage, table string) (seq int, conds []string) {
	var top []struct {
		Plan planNode `json:"Plan"`
	}
	_ = json.Unmarshal(raw, &top)
	var walk func(n planNode)
	walk = func(n planNode) {
		switch {
		case n.NodeType == "Seq Scan" && n.Relation == table:
			seq++
		case n.Relation == table:
			conds = append(conds, n.IndexCond+" "+n.RecheckCond)
		}
		for _, c := range n.Plans {
			walk(c)
		}
	}
	for _, p := range top {
		walk(p.Plan)
	}
	return seq, conds
}

// The lists of a university find its rows through an index on tenant_id,
// not by reading every university and filtering by row-level security,
// whose "… OR app_is_system()" no index can use (audit Н-36).
func TestListsReadOnlyTheirUniversity(t *testing.T) {
	d := pg.NewDB(t)
	ctx := context.Background()
	small, large := uuid.Must(uuid.NewV7()), uuid.Must(uuid.NewV7())
	user := uuid.Must(uuid.NewV7())
	s, l, u := "'"+small.String()+"'", "'"+large.String()+"'", "'"+user.String()+"'"
	// A small university next to a large one; the owner bypasses row security.
	d.Exec(t,
		"INSERT INTO tenants (id, slug, name, short_name) VALUES ("+s+", 'small', '{}', '{}'), ("+l+", 'large', '{}', '{}')",
		"INSERT INTO users (id, email, full_name) VALUES ("+u+", 'member@small.test', 'Member')",
		"INSERT INTO users (email, full_name) SELECT 'u' || i || '@large.test', 'User ' || i FROM generate_series(1, 2000) i",
		"INSERT INTO projects (tenant_id, title, industry, current_step) SELECT "+l+", 'Large ' || i, 'edtech', 'problem' FROM generate_series(1, 4000) i",
		"INSERT INTO projects (tenant_id, title, industry, current_step) SELECT "+s+", 'Small ' || i, 'edtech', 'problem' FROM generate_series(1, 10) i",
		"INSERT INTO project_members (tenant_id, project_id, user_id, role) SELECT tenant_id, id, "+u+", 'owner' FROM projects WHERE tenant_id = "+s,
		`INSERT INTO project_members (tenant_id, project_id, user_id, role)
		 SELECT p.tenant_id, p.id, x.id, 'owner'
		   FROM (SELECT id, tenant_id, row_number() OVER (ORDER BY id) AS n FROM projects WHERE tenant_id = `+l+`) p
		   JOIN (SELECT id, row_number() OVER (ORDER BY id) AS n FROM users WHERE email LIKE '%@large.test') x ON x.n = 1 + p.n % 2000`,
		"INSERT INTO team_requests (tenant_id, author_id, title, description, roles_needed) SELECT "+l+", "+u+", 'Large ' || i, 'Request', '{designer}' FROM generate_series(1, 4000) i",
		"INSERT INTO team_requests (tenant_id, author_id, title, description, roles_needed) SELECT "+s+", "+u+", 'Small ' || i, 'Request', '{designer}' FROM generate_series(1, 5) i",
		"INSERT INTO knowledge_documents (tenant_id, title, body, status) SELECT "+l+", 'Large ' || i, 'Body', 'ready' FROM generate_series(1, 2000) i",
		"INSERT INTO knowledge_documents (tenant_id, title, body, status) SELECT "+s+", 'Small ' || i, 'Body', 'ready' FROM generate_series(1, 3) i",
		`INSERT INTO knowledge_chunks (tenant_id, document_id, ordinal, content, embedding, embedding_model)
		 SELECT d.tenant_id, d.id, o, 'grant rules ' || o, '[1,0,0]', 'mock' FROM knowledge_documents d, generate_series(0, 1) o`,
		"ANALYZE",
	)

	scope := db.WithTenant(ctx, small)
	run := func(name string, query func(q *planRecorder) error) json.RawMessage {
		t.Helper()
		rec := &planRecorder{t: t}
		err := d.App.ReadTx(scope, func(ctx context.Context) error {
			rec.tx = d.App.Conn(ctx)
			return query(rec)
		})
		if err != nil || len(rec.plans) != 1 {
			t.Fatalf("%s: %v (%d plans)", name, err, len(rec.plans))
		}
		return rec.plans[0]
	}
	for _, c := range []struct {
		name, table string
		query       func(q *planRecorder) error
	}{
		{"ListAllProjects", "projects", func(q *planRecorder) error {
			_, err := projectdb.New(q).ListAllProjects(ctx, projectdb.ListAllProjectsParams{RowLimit: 21})
			return err
		}},
		{"ListMyProjects", "project_members", func(q *planRecorder) error {
			_, err := projectdb.New(q).ListMyProjects(ctx, projectdb.ListMyProjectsParams{UserID: user, RowLimit: 21})
			return err
		}},
		{"ListTeamRequests", "team_requests", func(q *planRecorder) error {
			_, err := teamdb.New(q).ListTeamRequests(ctx, teamdb.ListTeamRequestsParams{ViewerID: user, RowLimit: 21})
			return err
		}},
		{"ListDocuments", "knowledge_documents", func(q *planRecorder) error {
			_, err := knowledgedb.New(q).ListDocuments(ctx, knowledgedb.ListDocumentsParams{RowLimit: 21})
			return err
		}},
		{"SearchChunks", "knowledge_chunks", func(q *planRecorder) error {
			_, err := knowledgedb.New(q).SearchChunks(ctx, knowledgedb.SearchChunksParams{Query: "[1,0,0]", EmbeddingModel: "mock", RowLimit: 3})
			return err
		}},
	} {
		plan := run(c.name, c.query)
		seq, conds := scans(plan, c.table)
		byTenant := false
		for _, cond := range conds {
			byTenant = byTenant || strings.Contains(cond, "tenant_id")
		}
		if seq > 0 || !byTenant {
			t.Errorf("%s reads %s without an index on tenant_id (%d sequential scans, conditions %q):\n%s", c.name, c.table, seq, conds, plan)
		}
	}
	// Full-text search goes through its own index; it must not read the
	// whole table either.
	plan := run("SearchChunksText", func(q *planRecorder) error {
		_, err := knowledgedb.New(q).SearchChunksText(ctx, knowledgedb.SearchChunksTextParams{Query: "grant", RowLimit: 3})
		return err
	})
	if seq, _ := scans(plan, "knowledge_chunks"); seq > 0 {
		t.Errorf("SearchChunksText reads every passage:\n%s", plan)
	}
}

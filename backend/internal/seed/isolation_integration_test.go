package seed

import (
	"context"
	"errors"
	"slices"
	"testing"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"

	"github.com/yelnurq/startify/backend/internal/platform/db"
	"github.com/yelnurq/startify/backend/internal/platform/testpg"
	"github.com/yelnurq/startify/backend/internal/tenant/tenantdb"
)

// widenedReads are the tables whose rows other universities may read, with
// the condition under which they may: public files (avatars, covers),
// published public pages (the national showcase) and the registry of domains
// (the university is resolved from the host before any scope exists).
var widenedReads = map[string]string{
	"files":            "visibility = 'public' AND deleted_at IS NULL",
	"showcase_entries": "status = 'published' AND visibility = 'public'",
	"tenant_domains":   "true",
}

// registries are written only in the system scope: from a university's scope
// a write matches no rows instead of failing on the new row.
var registries = []string{"tenant_domains"}

// catalogTenantTables lists the tables with a tenant_id column from the
// catalog, so a new table is checked without anyone adding it to a list.
func catalogTenantTables(t *testing.T, database *testpg.Database) []string {
	t.Helper()
	ctx := context.Background()
	rows, err := database.App.Conn(ctx).Query(ctx, `
SELECT c.relname::text
  FROM pg_class c
  JOIN pg_namespace n ON n.oid = c.relnamespace
  JOIN pg_attribute a ON a.attrelid = c.oid AND a.attname = 'tenant_id' AND NOT a.attisdropped
 WHERE n.nspname = 'public' AND c.relkind IN ('r', 'p')
 ORDER BY 1`)
	if err != nil {
		t.Fatal(err)
	}
	tables, err := pgx.CollectRows(rows, pgx.RowTo[string])
	if err != nil || len(tables) < 30 {
		t.Fatalf("tenant tables %v: %v", tables, err)
	}
	return tables
}

// fixture writes one row into every tenant table of a university, through
// the application role in the university's scope (so WITH CHECK applies),
// and returns the tables it filled. The demo data leaves many tables empty
// (sessions, files, audit, chats, materials, comments, contacts…), and an
// empty table proves nothing about isolation.
func fixture(t *testing.T, database *testpg.Database, tenantID uuid.UUID, slug string) []string {
	t.Helper()
	ctx := context.Background()
	userID, err := Account(ctx, database.Worker, tenantID, Member{
		Email: "fixture@" + slug + ".test", FullName: "Fixture Member", Locale: "ru", Password: "Fixture-Password-2026",
		Roles: []string{"student"}, Slug: "fixture-" + slug,
	})
	if err != nil {
		t.Fatal(err)
	}
	var filled []string
	err = database.App.Tx(db.WithTenant(ctx, tenantID), func(ctx context.Context) error {
		conn := database.App.Conn(ctx)
		one := func(table, query string, args ...any) uuid.UUID {
			t.Helper()
			var id uuid.UUID
			if err := conn.QueryRow(ctx, query+" RETURNING "+returning(table), args...).Scan(&id); err != nil {
				t.Fatalf("fixture %s: %v", table, err)
			}
			filled = append(filled, table)
			return id
		}
		faculty := one("faculties", `INSERT INTO faculties (tenant_id, code, name) VALUES (app_current_tenant(), 'FIXTURE', '{}')`)
		department := one("departments", `INSERT INTO departments (tenant_id, faculty_id, code, name)
			VALUES (app_current_tenant(), $1, 'FIXTURE-D', '{}')`, faculty)
		one("study_groups", `INSERT INTO study_groups (tenant_id, faculty_id, department_id, code)
			VALUES (app_current_tenant(), $1, $2, 'FIXTURE-G')`, faculty, department)
		one("tenant_oidc_providers", `INSERT INTO tenant_oidc_providers (tenant_id, name, issuer, client_id, client_secret_enc)
			VALUES (app_current_tenant(), '{}', 'https://idp.invalid', 'fixture', '\x00')`)
		one("tenant_directory_settings", `INSERT INTO tenant_directory_settings
			(tenant_id, preset, url, bind_dn, user_base_dn, user_filter, login_attr, email_attr, name_attr, id_attr)
			VALUES (app_current_tenant(), 'openldap', 'ldap://ldap.invalid', 'cn=fixture', 'ou=people',
			'(uid={username})', 'uid', 'mail', 'cn', 'entryUUID')`)
		one("sessions", `INSERT INTO sessions (tenant_id, user_id, refresh_hash, expires_at)
			VALUES (app_current_tenant(), $1, gen_random_bytes(32), now() + interval '1 day')`, userID)
		one("auth_tokens", `INSERT INTO auth_tokens (tenant_id, user_id, purpose, token_hash, expires_at)
			VALUES (app_current_tenant(), $1, 'login_code', gen_random_bytes(32), now() + interval '1 hour')`, userID)
		one("files", `INSERT INTO files (tenant_id, owner_id, purpose, visibility, object_key, file_name, content_type, size_bytes, sha256, scan_status)
			VALUES (app_current_tenant(), $1, 'attachment', 'private', 'fixture/' || gen_random_uuid(), 'f.txt', 'text/plain', 1, 'x', 'clean')`, userID)
		// A public file too: other universities read it, and must still not
		// change or delete it (audit Н-37).
		if _, err := conn.Exec(ctx, `INSERT INTO files (tenant_id, owner_id, purpose, visibility, object_key, file_name, content_type, size_bytes, sha256, scan_status)
			VALUES (app_current_tenant(), $1, 'avatar', 'public', 'fixture/' || gen_random_uuid(), 'a.jpg', 'image/jpeg', 1, 'x', 'clean')`, userID); err != nil {
			t.Fatalf("fixture public file: %v", err)
		}
		one("audit_log", `INSERT INTO audit_log (tenant_id, actor_id, action) VALUES (app_current_tenant(), $1, 'fixture.created')`, userID)
		one("import_jobs", `INSERT INTO import_jobs (tenant_id, created_by, kind, file_name, columns, rows, summary)
			VALUES (app_current_tenant(), $1, 'students', 'f.csv', '{}', '[]', '{}')`, userID)
		request := one("team_requests", `INSERT INTO team_requests (tenant_id, author_id, title, description, roles_needed)
			VALUES (app_current_tenant(), $1, 'Fixture', 'Fixture request', '{designer}')`, userID)
		one("team_responses", `INSERT INTO team_responses (tenant_id, request_id, author_id, message)
			VALUES (app_current_tenant(), $1, $2, 'Fixture reply')`, request, userID)
		one("tenant_ai_settings", `INSERT INTO tenant_ai_settings (tenant_id) VALUES (app_current_tenant())
			ON CONFLICT (tenant_id) DO UPDATE SET mentor_mode = tenant_ai_settings.mentor_mode`)
		one("ai_usage", `INSERT INTO ai_usage (tenant_id, user_id, feature, provider, model, status)
			VALUES (app_current_tenant(), $1, 'mentor', 'mock', 'mock', 'ok')`, userID)
		one("ai_reservations", `INSERT INTO ai_reservations (tenant_id, user_id, tokens, expires_at)
			VALUES (app_current_tenant(), $1, 1, now() + interval '1 hour')`, userID)
		document := one("knowledge_documents", `INSERT INTO knowledge_documents (tenant_id, title, body)
			VALUES (app_current_tenant(), 'Fixture', 'Fixture body')`)
		one("knowledge_chunks", `INSERT INTO knowledge_chunks (tenant_id, document_id, ordinal, content, embedding, embedding_model)
			VALUES (app_current_tenant(), $1, 0, 'Fixture', '[1,0,0]', 'fixture')`, document)
		project := one("projects", `INSERT INTO projects (tenant_id, title, industry, current_step)
			VALUES (app_current_tenant(), 'Fixture', 'edtech', 'problem')`)
		one("project_members", `INSERT INTO project_members (tenant_id, project_id, user_id, role)
			VALUES (app_current_tenant(), $1, $2, 'owner')`, project, userID)
		one("project_step_data", `INSERT INTO project_step_data (tenant_id, project_id, step_key)
			VALUES (app_current_tenant(), $1, 'problem')`, project)
		one("project_revisions", `INSERT INTO project_revisions (tenant_id, project_id, step_key, field_key, value, version, source)
			VALUES (app_current_tenant(), $1, 'problem', 'statement', '"x"', 1, 'edit')`, project)
		one("project_comments", `INSERT INTO project_comments (tenant_id, project_id, step_key, author_id, body)
			VALUES (app_current_tenant(), $1, 'problem', $2, 'Fixture')`, project, userID)
		conversation := one("ai_conversations", `INSERT INTO ai_conversations (tenant_id, subject_type, subject_id)
			VALUES (app_current_tenant(), 'project_step', $1)`, project)
		one("ai_messages", `INSERT INTO ai_messages (tenant_id, conversation_id, role, content)
			VALUES (app_current_tenant(), $1, 'user', 'Fixture')`, conversation)
		one("project_materials", `INSERT INTO project_materials (tenant_id, project_id, kind, format, locale)
			VALUES (app_current_tenant(), $1, 'one_pager', 'pdf', 'ru')`, project)
		one("evaluation_rubrics", `INSERT INTO evaluation_rubrics (tenant_id, weights) VALUES (app_current_tenant(), '{}')
			ON CONFLICT (tenant_id) DO UPDATE SET weights = evaluation_rubrics.weights`)
		evaluation := one("evaluations", `INSERT INTO evaluations (tenant_id, project_id, locale, rubric)
			VALUES (app_current_tenant(), $1, 'ru', '[]')`, project)
		one("evaluation_criteria_scores", `INSERT INTO evaluation_criteria_scores (tenant_id, evaluation_id, criterion_key, ai_score, score, justification)
			VALUES (app_current_tenant(), $1, 'problem', 5, 5, 'Fixture')`, evaluation)
		one("evaluation_overrides", `INSERT INTO evaluation_overrides (tenant_id, evaluation_id, criterion_key, old_score, new_score, comment)
			VALUES (app_current_tenant(), $1, 'problem', 5, 6, 'Fixture correction')`, evaluation)
		one("showcase_entries", `INSERT INTO showcase_entries (project_id, tenant_id, slug, title, industry, stage, year)
			VALUES ($1, app_current_tenant(), 'fixture-page', 'Fixture', 'edtech', 'idea', 2026)`, project)
		one("contact_requests", `INSERT INTO contact_requests (tenant_id, project_id, name, email, kind, message, locale)
			VALUES (app_current_tenant(), $1, 'Fixture', 'fixture@partner.test', 'other', 'A fixture message to the team.', 'ru')`, project)
		// Settings and memberships come with the university and the account.
		filled = append(filled, "tenant_settings", "memberships")
		// A university cannot add a domain to the registry, not even its own.
		_, err := conn.Exec(ctx, "SAVEPOINT domain")
		if err != nil {
			return err
		}
		if _, err := conn.Exec(ctx, `INSERT INTO tenant_domains (domain, tenant_id) VALUES ($1, app_current_tenant())`, slug+".invalid"); !isRLSViolation(err) {
			t.Errorf("%s added a domain from its own scope: %v", slug, err)
		}
		_, err = conn.Exec(ctx, "ROLLBACK TO SAVEPOINT domain")
		return err
	})
	if err != nil {
		t.Fatal(err)
	}
	// The registry is written in the system scope, by the worker's role.
	err = database.Worker.Tx(db.WithSystem(ctx), func(ctx context.Context) error {
		_, err := database.Worker.Conn(ctx).Exec(ctx, `INSERT INTO tenant_domains (domain, tenant_id) VALUES ($1, $2)`, slug+".invalid", tenantID)
		return err
	})
	if err != nil {
		t.Fatal(err)
	}
	return append(filled, "tenant_domains")
}

// returning names the key a fixture row is identified by.
func returning(table string) string {
	switch table {
	case "tenant_ai_settings", "evaluation_rubrics", "tenant_directory_settings":
		return "tenant_id"
	case "project_members", "project_step_data", "showcase_entries":
		return "project_id"
	case "evaluation_criteria_scores":
		return "evaluation_id"
	}
	return "id"
}

func isRLSViolation(err error) bool {
	var pgErr *pgconn.PgError
	return errors.As(err, &pgErr) && pgErr.Code == "42501"
}

// TestEveryTenantTableIsIsolated proves, for every table with tenant_id,
// that a university sees its own rows (plus only what the table deliberately
// shares), that no scope or the API's "system scope" sees nothing, and that
// a university neither changes nor deletes rows of another one nor moves its
// rows there (audit С-22). Both universities have rows in every table.
func TestEveryTenantTableIsIsolated(t *testing.T) {
	database := pg.NewDB(t)
	ctx := context.Background()
	if err := Demo(ctx, database.Worker); err != nil {
		t.Fatal(err)
	}
	q := tenantdb.New(database.App.Conn(ctx))
	kazutb, err := q.GetTenantBySlug(ctx, KazUTB.Slug)
	if err != nil {
		t.Fatal(err)
	}
	sandbox, err := q.GetTenantBySlug(ctx, Sandbox.Slug)
	if err != nil {
		t.Fatal(err)
	}
	tables := catalogTenantTables(t, database)
	for _, u := range []tenantdb.Tenant{kazutb, sandbox} {
		filled := fixture(t, database, u.ID, u.Slug)
		slices.Sort(filled)
		if !slices.Equal(filled, tables) {
			t.Fatalf("the fixture fills %v, the catalog has %v: give a new table a fixture row", filled, tables)
		}
	}

	// count runs a count in a scope of a pool.
	count := func(pool *db.DB, scope context.Context, query string, args ...any) int {
		t.Helper()
		var n int
		err := pool.ReadTx(scope, func(ctx context.Context) error {
			return pool.Conn(ctx).QueryRow(ctx, query, args...).Scan(&n)
		})
		if err != nil {
			t.Fatalf("%s: %v", query, err)
		}
		return n
	}
	system := db.WithSystem(ctx)
	for _, table := range tables {
		name := pgx.Identifier{table}.Sanitize()
		shared := "false"
		if cond, ok := widenedReads[table]; ok {
			shared = cond
		}
		all := count(database.Worker, system, "SELECT count(*) FROM "+name)
		for _, u := range []tenantdb.Tenant{kazutb, sandbox} {
			own := count(database.Worker, system, "SELECT count(*) FROM "+name+" WHERE tenant_id = $1", u.ID)
			foreign := count(database.Worker, system, "SELECT count(*) FROM "+name+" WHERE tenant_id <> $1 AND ("+shared+")", u.ID)
			seen := count(database.App, db.WithTenant(ctx, u.ID), "SELECT count(*) FROM "+name)
			if own == 0 {
				t.Errorf("%s: no rows of %s, nothing would be proven", table, u.Slug)
			}
			if seen != own+foreign {
				t.Errorf("%s: %s sees %d rows, want its %d and %d shared", table, u.Slug, seen, own, foreign)
			}
		}
		if own := count(database.Worker, system, "SELECT count(*) FROM "+name+" WHERE tenant_id IN ($1, $2)", kazutb.ID, sandbox.ID); own != all {
			t.Errorf("%s: %d rows of other universities", table, all-own)
		}
		public := count(database.Worker, system, "SELECT count(*) FROM "+name+" WHERE "+shared)
		if n := count(database.App, ctx, "SELECT count(*) FROM "+name); n != public {
			t.Errorf("%s: %d rows visible without a scope, want the %d shared", table, n, public)
		}
		if n := count(database.App, system, "SELECT count(*) FROM "+name); n != public {
			t.Errorf("%s: the API's system scope sees %d rows, want the %d shared", table, n, public)
		}
		writesFromSandbox(t, database, table, kazutb.ID, sandbox.ID)
	}
}

// errRollback undoes a transaction of probing writes.
var errRollback = errors.New("rollback")

// writesFromSandbox tries, in the sandbox university's scope, to change and
// delete KazUTB's rows and to move its own rows to KazUTB. Nothing may happen.
func writesFromSandbox(t *testing.T, database *testpg.Database, table string, kazutb, sandbox uuid.UUID) {
	t.Helper()
	name := pgx.Identifier{table}.Sanitize()
	probe := func(query string, args ...any) (int64, error) {
		var n int64
		var execErr error
		err := database.App.Tx(db.WithTenant(context.Background(), sandbox), func(ctx context.Context) error {
			tag, err := database.App.Conn(ctx).Exec(ctx, query, args...)
			n, execErr = tag.RowsAffected(), err
			return errRollback
		})
		if !errors.Is(err, errRollback) {
			t.Fatalf("%s: %v", query, err)
		}
		return n, execErr
	}
	if table == "audit_log" {
		// The trail is append-only for the application: no UPDATE or DELETE at all.
		for _, query := range []string{"UPDATE audit_log SET action = action", "DELETE FROM audit_log"} {
			if _, err := probe(query); err == nil {
				t.Errorf("%s: allowed", query)
			}
		}
		return
	}
	for _, query := range []string{
		"UPDATE " + name + " SET tenant_id = tenant_id WHERE tenant_id = $1",
		"DELETE FROM " + name + " WHERE tenant_id = $1",
	} {
		if n, err := probe(query, kazutb); n != 0 || err != nil {
			t.Errorf("%s: %d rows of another university (%v)", query, n, err)
		}
	}
	n, err := probe("UPDATE "+name+" SET tenant_id = $1 WHERE tenant_id = $2", kazutb, sandbox)
	switch {
	case slices.Contains(registries, table):
		if n != 0 || err != nil {
			t.Errorf("%s: a university changed %d rows of the registry (%v)", table, n, err)
		}
	case !isRLSViolation(err):
		t.Errorf("%s: rows moved to another university: %d (%v)", table, n, err)
	}
}

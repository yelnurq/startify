package seed

import (
	"context"
	"fmt"

	"github.com/google/uuid"

	"github.com/yelnurq/startify/backend/internal/platform/db"
)

// The marks end-to-end tests put on what they create (frontend/e2e): fresh
// accounts have e2e.<…>@qadam.test addresses, partners writing to a team
// e2e.<…> addresses, and requests for teammates and knowledge documents
// titles starting with "E2E ".
const (
	e2eAccounts = "e2e.%@qadam.test"
	e2eSenders  = "e2e.%"
	e2eTitles   = "E2E %"
	// What runs before these marks (October 2026) left under demo accounts:
	// the board test's requests and the showcase test's contact requests.
	legacyRequests = `^Ищу дизайнера в агротех-стартап [0-9]+$`
	legacySenders  = `^aidos(\.[a-z0-9]+)?@agrofund\.kz$`
)

// Cleaned counts what CleanE2E removed.
type Cleaned struct {
	Accounts, Projects, Requests, Contacts, Documents, Imports, Files int64
}

// CleanE2E removes what end-to-end runs left in a development database: the
// accounts they registered or imported with their memberships, sessions,
// profiles, replies and projects (with everything in them), the requests for
// teammates and knowledge documents they titled "E2E …", the contact
// requests of their senders and the imports of their addresses. Files of
// all that are marked deleted, so the purge job removes their contents.
// The demo data is left alone; cmd/seed refuses to run it in production.
// It needs the worker's pool: it works across universities.
func CleanE2E(ctx context.Context, database *db.DB) (Cleaned, error) {
	var c Cleaned
	err := database.Tx(db.WithSystem(ctx), func(ctx context.Context) error {
		conn := database.Conn(ctx)
		var accounts, projects []uuid.UUID
		if err := conn.QueryRow(ctx, `SELECT coalesce(array_agg(id), '{}') FROM users WHERE email::text LIKE $1`,
			e2eAccounts).Scan(&accounts); err != nil {
			return err
		}
		if err := conn.QueryRow(ctx, `
SELECT coalesce(array_agg(DISTINCT id), '{}') FROM projects
 WHERE created_by = ANY($1)
    OR id IN (SELECT project_id FROM project_members WHERE role = 'owner' AND user_id = ANY($1))`,
			accounts).Scan(&projects); err != nil {
			return err
		}
		steps := []struct {
			count *int64
			query string
			args  []any
		}{
			{&c.Files, `
UPDATE files SET deleted_at = now()
 WHERE deleted_at IS NULL
   AND (owner_id = ANY($1)
        OR id IN (SELECT file_id FROM project_materials WHERE project_id = ANY($2))
        OR id IN (SELECT cover_file_id FROM showcase_entries WHERE project_id = ANY($2))
        OR id IN (SELECT og_file_id FROM showcase_entries WHERE project_id = ANY($2))
        OR id IN (SELECT file_id FROM knowledge_documents WHERE title LIKE $3))`, []any{accounts, projects, e2eTitles}},
			// Conversations name their project without a foreign key.
			{nil, `DELETE FROM ai_conversations WHERE subject_type = 'project_step' AND subject_id = ANY($1)`, []any{projects}},
			{&c.Projects, `DELETE FROM projects WHERE id = ANY($1)`, []any{projects}},
			{&c.Requests, `DELETE FROM team_requests WHERE title LIKE $1 OR author_id = ANY($2) OR title ~ $3`,
				[]any{e2eTitles, accounts, legacyRequests}},
			{&c.Contacts, `DELETE FROM contact_requests WHERE email::text LIKE $1 OR email::text ~ $2`, []any{e2eSenders, legacySenders}},
			{&c.Documents, `DELETE FROM knowledge_documents WHERE title LIKE $1`, []any{e2eTitles}},
			{&c.Imports, `DELETE FROM import_jobs WHERE rows::text LIKE '%e2e.%@qadam.test%'`, nil},
			{&c.Accounts, `DELETE FROM users WHERE id = ANY($1)`, []any{accounts}},
		}
		for _, s := range steps {
			tag, err := conn.Exec(ctx, s.query, s.args...)
			if err != nil {
				return fmt.Errorf("%s: %w", s.query, err)
			}
			if s.count != nil {
				*s.count = tag.RowsAffected()
			}
		}
		return nil
	})
	if err != nil {
		return c, fmt.Errorf("seed: clean e2e data: %w", err)
	}
	return c, nil
}

package seed

import (
	"context"
	"testing"

	"github.com/google/uuid"

	"github.com/yelnurq/startify/backend/internal/platform/db"
	"github.com/yelnurq/startify/backend/internal/platform/testpg"
	"github.com/yelnurq/startify/backend/internal/tenant/tenantdb"
)

// What an end-to-end run leaves goes; the demo data stays (audit Н-59).
func TestCleanE2ERemovesOnlyTestData(t *testing.T) {
	database := pg.NewDB(t)
	ctx := context.Background()
	if err := Demo(ctx, database.Worker); err != nil {
		t.Fatal(err)
	}
	kazutb, err := tenantdb.New(database.App.Conn(ctx)).GetTenantBySlug(ctx, KazUTB.Slug)
	if err != nil {
		t.Fatal(err)
	}
	demo := snapshotCounts(t, database)

	// A run: a registered student with a project, a page and a material; a
	// request for teammates, a contact request and a knowledge document by
	// the demo accounts, marked as the tests mark them.
	student, err := Account(ctx, database.Worker, kazutb.ID, Member{
		Email: "e2e.student.abc123@qadam.test", FullName: "Тест Студентов", Locale: "ru", Password: "Fixture-Password-2026",
		Roles: []string{"student"}, Slug: "e2e-student-abc123",
	})
	if err != nil {
		t.Fatal(err)
	}
	err = database.App.Tx(db.WithTenant(ctx, kazutb.ID), func(ctx context.Context) error {
		conn := database.App.Conn(ctx)
		var project, file uuid.UUID
		if err := conn.QueryRow(ctx, `INSERT INTO files (tenant_id, owner_id, purpose, visibility, object_key, file_name, content_type, size_bytes, sha256, scan_status)
			VALUES (app_current_tenant(), $1, 'avatar', 'public', 'e2e/avatar', 'a.jpg', 'image/jpeg', 1, 'x', 'clean') RETURNING id`, student).Scan(&file); err != nil {
			return err
		}
		if err := conn.QueryRow(ctx, `INSERT INTO projects (tenant_id, title, industry, current_step, created_by)
			VALUES (app_current_tenant(), 'ЗерноСкан', 'agritech', 'problem', $1) RETURNING id`, student).Scan(&project); err != nil {
			return err
		}
		if _, err := conn.Exec(ctx, `INSERT INTO project_members (tenant_id, project_id, user_id, role)
			VALUES (app_current_tenant(), $1, $2, 'owner')`, project, student); err != nil {
			return err
		}
		for _, q := range []string{
			`INSERT INTO ai_conversations (tenant_id, subject_type, subject_id, topic) VALUES (app_current_tenant(), 'project_step', $1, 'problem')`,
			`INSERT INTO showcase_entries (project_id, tenant_id, slug, title, industry, stage, year)
			 VALUES ($1, app_current_tenant(), 'e2e-zernoscan', 'ЗерноСкан', 'agritech', 'idea', 2026)`,
		} {
			if _, err := conn.Exec(ctx, q, project); err != nil {
				return err
			}
		}
		// The board test's requests, with the mark and from before it.
		_, err := conn.Exec(ctx, `
INSERT INTO team_requests (tenant_id, author_id, title, description, roles_needed)
SELECT app_current_tenant(), m.user_id, t, 'Делаем сервис для фермеров.', '{designer}'
  FROM memberships m JOIN users u ON u.id = m.user_id,
       unnest(ARRAY['E2E Ищу дизайнера в агротех-стартап 1729', 'Ищу дизайнера в агротех-стартап 1728']) AS t
 WHERE u.email = 'nurlan@qadam.test'`)
		if err != nil {
			return err
		}
		if _, err := conn.Exec(ctx, `
INSERT INTO contact_requests (tenant_id, project_id, name, email, kind, message, locale)
SELECT app_current_tenant(), project_id, 'Айдос Сеитов', 'e2e.aidos.abc@agrofund.kz', 'pilot', 'Готовы дать поле для пилота весной.', 'ru'
  FROM showcase_entries WHERE slug = 'agroscan'`); err != nil {
			return err
		}
		_, err = conn.Exec(ctx, `INSERT INTO knowledge_documents (tenant_id, title, body) VALUES (app_current_tenant(), 'E2E регламент abc', 'Правила хакатона')`)
		return err
	})
	if err != nil {
		t.Fatal(err)
	}

	c, err := CleanE2E(ctx, database.Worker)
	if err != nil {
		t.Fatal(err)
	}
	if c != (Cleaned{Accounts: 1, Projects: 1, Requests: 2, Contacts: 1, Documents: 1, Files: 1}) {
		t.Fatalf("cleaned %+v", c)
	}
	if after := snapshotCounts(t, database); after != demo {
		t.Fatalf("the demo data changed: %+v, want %+v", after, demo)
	}
	var deleted bool
	err = database.Worker.ReadTx(db.WithSystem(ctx), func(ctx context.Context) error {
		return database.Worker.Conn(ctx).QueryRow(ctx, "SELECT deleted_at IS NOT NULL FROM files WHERE object_key = 'e2e/avatar'").Scan(&deleted)
	})
	if err != nil || !deleted {
		t.Fatalf("the student's file is not marked deleted: %v %v", deleted, err)
	}
	if again, err := CleanE2E(ctx, database.Worker); err != nil || again != (Cleaned{}) {
		t.Fatalf("second run: %+v %v", again, err)
	}
}

type counts struct {
	users, projects, requests, contacts, documents, conversations int
}

func snapshotCounts(t *testing.T, database *testpg.Database) counts {
	t.Helper()
	ctx := context.Background()
	var c counts
	err := database.Worker.ReadTx(db.WithSystem(ctx), func(ctx context.Context) error {
		return database.Worker.Conn(ctx).QueryRow(ctx, `SELECT
  (SELECT count(*) FROM users), (SELECT count(*) FROM projects), (SELECT count(*) FROM team_requests),
  (SELECT count(*) FROM contact_requests), (SELECT count(*) FROM knowledge_documents), (SELECT count(*) FROM ai_conversations)`).
			Scan(&c.users, &c.projects, &c.requests, &c.contacts, &c.documents, &c.conversations)
	})
	if err != nil {
		t.Fatal(err)
	}
	return c
}

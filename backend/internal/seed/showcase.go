package seed

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"
	"time"

	"github.com/google/uuid"

	"github.com/yelnurq/startify/backend/internal/incubator"
	"github.com/yelnurq/startify/backend/internal/platform/db"
	"github.com/yelnurq/startify/backend/internal/showcase"
	"github.com/yelnurq/startify/backend/internal/showcase/showcasedb"
)

// demoPage is how a demo project appears on the showcase.
type demoPage struct {
	// Status is published, in_review or draft; Public asks for everyone.
	Status string
	Public bool
	Needs  []string
	Tags   []string
	Score  bool
}

// KazUTBShowcase are the showcase pages of the demo projects by title.
var KazUTBShowcase = map[string]demoPage{
	"AgroScan":   {Status: showcase.StatusPublished, Public: true, Needs: []string{"investment", "pilot"}, Tags: []string{"computer vision", "mobile", "b2b"}, Score: true},
	"PomaFilm":   {Status: showcase.StatusPublished, Public: true, Needs: []string{"partner", "investment"}, Tags: []string{"biomaterials", "packaging", "b2b"}, Score: true},
	"Kiiz Lab":   {Status: showcase.StatusPublished, Public: true, Needs: []string{"partner"}, Tags: []string{"felt", "design", "export"}, Score: true},
	"EsepBot":    {Status: showcase.StatusPublished, Public: true, Needs: []string{"investment", "team"}, Tags: []string{"telegram", "ai", "accounting"}, Score: true},
	"Oryn":       {Status: showcase.StatusPublished, Public: true, Needs: []string{"pilot"}, Tags: []string{"booking", "campus", "web"}},
	"AuylMed":    {Status: showcase.StatusPublished, Public: true, Needs: []string{"pilot", "mentor"}, Tags: []string{"telemedicine", "rural", "ai"}, Score: true},
	"DalaCargo":  {Status: showcase.StatusPublished, Public: true, Needs: []string{"team", "investment"}, Tags: []string{"marketplace", "trucks"}},
	"Daiyn":      {Status: showcase.StatusPublished, Public: true, Needs: []string{"mentor"}, Tags: []string{"ent", "exam prep", "mobile"}},
	"SunQora":    {Status: showcase.StatusPublished, Needs: []string{"partner"}, Tags: []string{"solar", "iot"}},
	"QalaSignal": {Status: showcase.StatusInReview, Public: true, Needs: []string{"pilot", "partner"}, Tags: []string{"city", "civic tech"}},
	"QabylBot":   {Status: showcase.StatusDraft, Needs: []string{"team"}, Tags: []string{"telegram", "admissions"}},
}

// Showcase prepares the showcase pages of the demo projects from their
// incubator answers; pages that exist are kept.
func Showcase(ctx context.Context, database *db.DB, tenantID uuid.UUID, projects []DemoProject, pages map[string]demoPage, now time.Time) error {
	err := database.Tx(db.WithTenant(ctx, tenantID), func(ctx context.Context) error {
		conn := database.Conn(ctx)
		q := showcasedb.New(conn)
		for i := range projects {
			p := &projects[i]
			page, ok := pages[p.Title]
			if !ok {
				continue
			}
			var id uuid.UUID
			var currentStep string
			var exists bool
			if err := conn.QueryRow(ctx, `SELECT p.id, p.current_step, EXISTS (SELECT 1 FROM showcase_entries e WHERE e.project_id = p.id)
				FROM projects p WHERE p.title = $1`, p.Title).Scan(&id, &currentStep, &exists); err != nil {
				return fmt.Errorf("%s: %w", p.Title, err)
			}
			if exists {
				continue
			}
			if err := demoShowcasePage(ctx, database, q, id, currentStep, p, page, now); err != nil {
				return fmt.Errorf("%s: %w", p.Title, err)
			}
		}
		return nil
	})
	if err != nil {
		return fmt.Errorf("seed: showcase: %w", err)
	}
	return nil
}

func demoShowcasePage(ctx context.Context, database *db.DB, q *showcasedb.Queries, id uuid.UUID, currentStep string, p *DemoProject, page demoPage, now time.Time) error {
	conn := database.Conn(ctx)
	draft, err := incubator.DraftShowcase(ctx, database, id, p.Tagline)
	if err != nil {
		return err
	}
	sections := showcase.Sections{Problem: draft.Problem, Solution: draft.Solution, Market: draft.Market, Traction: draft.Traction, Ask: draft.Ask}

	rows, err := conn.Query(ctx, `SELECT u.full_name, pm.title FROM project_members pm JOIN users u ON u.id = pm.user_id
		WHERE pm.project_id = $1 AND pm.role IN ('owner', 'member') ORDER BY pm.role = 'owner' DESC, pm.created_at`, id)
	if err != nil {
		return err
	}
	var team []showcase.Member
	for rows.Next() {
		var name, title string
		if err := rows.Scan(&name, &title); err != nil {
			rows.Close()
			return err
		}
		role := draft.Roles[strings.ToLower(name)]
		if role == "" {
			role = title
		}
		team = append(team, showcase.Member{Name: name, Role: role})
	}
	rows.Close()
	if err := rows.Err(); err != nil {
		return err
	}

	var facultyID *uuid.UUID
	var faculty json.RawMessage
	err = conn.QueryRow(ctx, `SELECT f.id, f.name FROM projects p
		JOIN memberships m ON m.user_id = p.created_by AND m.tenant_id = p.tenant_id
		JOIN faculties f ON f.id = coalesce(p.faculty_id, m.faculty_id)
		WHERE p.id = $1`, id).Scan(&facultyID, &faculty)
	if err != nil && !db.IsNoRows(err) {
		return err
	}
	var score *int32
	var level *string
	err = conn.QueryRow(ctx, `SELECT total, level FROM evaluations WHERE project_id = $1 AND status = 'ready'
		ORDER BY created_at DESC LIMIT 1`, id).Scan(&score, &level)
	if err != nil && !db.IsNoRows(err) {
		return err
	}
	var video *string
	if draft.VideoURL != "" {
		video = &draft.VideoURL
	}
	visibility := showcase.VisibilityUniversity
	if page.Public {
		visibility = showcase.VisibilityPublic
	}
	rawSections, _ := json.Marshal(sections)
	rawTeam, _ := json.Marshal(team)
	started := now.AddDate(0, 0, -p.Started)
	if _, err := q.UpsertEntry(ctx, showcasedb.UpsertEntryParams{
		ProjectID: id, Slug: showcase.Slugify(p.Title), Visibility: visibility, Title: p.Title, Tagline: draft.Tagline,
		Industry: p.Industry, Stage: incubator.StageOf(currentStep), FacultyID: facultyID, Faculty: faculty,
		Year: int32(started.Year()), VideoUrl: video, Sections: rawSections, Team: rawTeam, Needs: page.Needs, //nolint:gosec // a calendar year
		Tags: page.Tags, ShowScore: page.Score, Score: score, Level: level, ContactEnabled: true,
		SearchText: showcase.SearchText(p.Title, draft.Tagline, sections, page.Tags),
	}); err != nil {
		return err
	}
	if page.Status == showcase.StatusDraft {
		return nil
	}
	// Pages were published after the last work on the project.
	published := now.AddDate(0, 0, -p.LastActive).Add(-3 * time.Hour)
	var submitted *time.Time
	if page.Status == showcase.StatusInReview {
		at := now.Add(-26 * time.Hour)
		submitted = &at
	}
	if _, err := q.SetStatus(ctx, showcasedb.SetStatusParams{
		ProjectID: id, Status: page.Status, SubmittedAt: submitted, PublishedAt: &published,
	}); err != nil {
		return err
	}
	projectVisibility := showcase.VisibilityUniversity
	if page.Status == showcase.StatusPublished && page.Public {
		projectVisibility = showcase.VisibilityPublic
	}
	_, err = conn.Exec(ctx, "UPDATE projects SET visibility = $2 WHERE id = $1", id, projectVisibility)
	return err
}

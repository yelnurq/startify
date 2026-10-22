package seed

import (
	"context"
	"encoding/json"
	"fmt"
	"time"

	"github.com/google/uuid"

	"github.com/yelnurq/startify/backend/internal/incubator"
	"github.com/yelnurq/startify/backend/internal/platform/db"
)

// DemoProject is a project of the demo installation with its answers.
type DemoProject struct {
	Title    string
	Tagline  string
	Industry string
	// Owner, Members and Supervisor are e-mails of demo accounts.
	Owner      string
	Members    []string
	Supervisor string
	// Done is how many leading steps are completed; they must meet the
	// minimum of the template.
	Done int
	// Started and LastActive date the project in days before now.
	Started    int
	LastActive int
	// Steps holds the answers by step and field key.
	Steps map[string]map[string]any
}

// Student is the e-mail of the i-th generated demo student.
func Student(i int) string { return Students(GeneratedStudents)[i].Email }

// Projects creates the demo projects missing by title. The rows are written
// directly so that their dates spread over the past weeks.
func Projects(ctx context.Context, database *db.DB, tenantID uuid.UUID, people map[string]uuid.UUID, projects []DemoProject, now time.Time) error {
	tmpl := incubator.Default()
	err := database.Tx(db.WithTenant(ctx, tenantID), func(ctx context.Context) error {
		conn := database.Conn(ctx)
		for i := range projects {
			p := &projects[i]
			var exists bool
			if err := conn.QueryRow(ctx, "SELECT EXISTS (SELECT 1 FROM projects WHERE title = $1)", p.Title).Scan(&exists); err != nil {
				return err
			}
			if exists {
				continue
			}
			if err := demoProject(ctx, database, tmpl, people, p, now); err != nil {
				return fmt.Errorf("%s: %w", p.Title, err)
			}
		}
		return nil
	})
	if err != nil {
		return fmt.Errorf("seed: projects: %w", err)
	}
	return nil
}

func person(people map[string]uuid.UUID, email string) (uuid.UUID, error) {
	id, ok := people[email]
	if !ok {
		return uuid.Nil, fmt.Errorf("unknown demo account %s", email)
	}
	return id, nil
}

type demoStep struct {
	key    string
	st     *incubator.Step
	values incubator.Values
	status string
}

// demoSteps checks the answers against the template and works out the
// status of every answered step.
func demoSteps(tmpl *incubator.Template, p *DemoProject) (steps []demoStep, completed int, current string, err error) {
	for idx, key := range tmpl.Keys() {
		values := p.Steps[key]
		if len(values) == 0 {
			if idx < p.Done {
				return nil, 0, "", fmt.Errorf("step %s is completed without answers", key)
			}
			if current == "" {
				current = key
			}
			continue
		}
		st, _ := tmpl.Step(key)
		raw := map[string]json.RawMessage{}
		for k, v := range values {
			b, err := json.Marshal(v)
			if err != nil {
				return nil, 0, "", err
			}
			raw[k] = b
		}
		clean, err := st.Sanitize(raw)
		if err != nil {
			return nil, 0, "", fmt.Errorf("step %s: %w", key, err)
		}
		status := "in_progress"
		if idx < p.Done {
			if gaps := st.Missing(clean); len(gaps) > 0 {
				return nil, 0, "", fmt.Errorf("step %s is completed but misses %v", key, gaps)
			}
			status = "completed"
			completed++
		} else if current == "" {
			current = key
		}
		steps = append(steps, demoStep{key: key, st: st, values: clean, status: status})
	}
	if current == "" {
		keys := tmpl.Keys()
		current = keys[len(keys)-1]
	}
	return steps, completed, current, nil
}

func demoProject(ctx context.Context, database *db.DB, tmpl *incubator.Template, people map[string]uuid.UUID, p *DemoProject, now time.Time) error {
	conn := database.Conn(ctx)
	owner, err := person(people, p.Owner)
	if err != nil {
		return err
	}
	team := []uuid.UUID{owner}
	for _, e := range p.Members {
		id, err := person(people, e)
		if err != nil {
			return err
		}
		team = append(team, id)
	}
	steps, completed, current, err := demoSteps(tmpl, p)
	if err != nil {
		return err
	}
	started := now.AddDate(0, 0, -p.Started)
	last := now.AddDate(0, 0, -p.LastActive)
	id, err := uuid.NewV7()
	if err != nil {
		return err
	}
	if _, err := conn.Exec(ctx, `INSERT INTO projects
		(id, tenant_id, title, tagline, industry, current_step, completed_steps, ip_policy_version, ip_accepted_at, created_by, created_at, updated_at)
		VALUES ($1, app_current_tenant(), $2, $3, $4, $5, $6, $7, $8, $9, $8, $10)`,
		id, p.Title, p.Tagline, p.Industry, current, completed, KazUTB.IPPolicyVersion, started, owner, last); err != nil {
		return err
	}
	join := func(user uuid.UUID, role string, at time.Time) error {
		_, err := conn.Exec(ctx, `INSERT INTO project_members (tenant_id, project_id, user_id, role, created_at, updated_at)
			VALUES (app_current_tenant(), $1, $2, $3, $4, $4)`, id, user, role, at)
		return err
	}
	for i, u := range team {
		role := "member"
		if i == 0 {
			role = "owner"
		}
		if err := join(u, role, started.Add(time.Duration(i)*36*time.Hour)); err != nil {
			return err
		}
	}
	if p.Supervisor != "" {
		u, err := person(people, p.Supervisor)
		if err != nil {
			return err
		}
		if err := join(u, "supervisor", started.Add(72*time.Hour)); err != nil {
			return err
		}
	}

	// Answers are spread over the life of the project, step by step.
	span := last.Sub(started)
	for n, s := range steps {
		at := started.Add(span * time.Duration(n+1) / time.Duration(len(steps)))
		data, err := json.Marshal(s.values)
		if err != nil {
			return err
		}
		author := team[n%len(team)]
		var completedAt *time.Time
		if s.status == "completed" {
			completedAt = &at
		}
		if _, err := conn.Exec(ctx, `INSERT INTO project_step_data
			(tenant_id, project_id, step_key, data, version, status, completed_at, completed_by, updated_by, created_at, updated_at)
			VALUES (app_current_tenant(), $1, $2, $3, $4, $5, $6, $7, $7, $8, $8)`,
			id, s.key, data, len(s.values), s.status, completedAt, author, at); err != nil {
			return err
		}
		version := 0
		for i := range s.st.Fields {
			f := &s.st.Fields[i]
			v, ok := s.values[f.Key]
			if !ok {
				continue
			}
			version++
			value, err := json.Marshal(v)
			if err != nil {
				return err
			}
			when := at.Add(-time.Duration(len(s.values)-version) * 7 * time.Minute)
			if _, err := conn.Exec(ctx, `INSERT INTO project_revisions
				(tenant_id, project_id, step_key, field_key, value, version, author_id, source, created_at, updated_at)
				VALUES (app_current_tenant(), $1, $2, $3, $4, $5, $6, 'edit', $7, $7)`,
				id, s.key, f.Key, value, version, team[(n+version)%len(team)], when); err != nil {
				return err
			}
		}
	}
	return nil
}

package seed

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"
	"time"

	"github.com/google/uuid"

	"github.com/yelnurq/startify/backend/internal/evaluation"
	"github.com/yelnurq/startify/backend/internal/incubator"
	"github.com/yelnurq/startify/backend/internal/platform/db"
)

// evaluatedFrom is how far a project must be to have demo evaluations.
const evaluatedFrom = 6

// contentLocale guesses the language a project is written in. Kazakh names
// in a Russian text add a few Kazakh letters, so the share decides.
func contentLocale(data string) string {
	kazakh, cyrillic := 0, 0
	for _, r := range strings.ToLower(data) {
		switch {
		case strings.ContainsRune("әғқңөұүһі", r):
			kazakh++
			cyrillic++
		case r >= 'а' && r <= 'я' || r == 'ё':
			cyrillic++
		}
	}
	switch {
	case cyrillic > 0 && kazakh*100 >= cyrillic*3:
		return "kk"
	case cyrillic > 0:
		return "ru"
	}
	return "en"
}

// Evaluations gives the more advanced demo projects an evaluation history: an
// earlier, weaker evaluation and the current one, made by the mock reviewer.
// The first project also gets an expert correction.
func Evaluations(ctx context.Context, database *db.DB, tenantID uuid.UUID, people map[string]uuid.UUID, projects []DemoProject, now time.Time) error {
	err := database.Tx(db.WithTenant(ctx, tenantID), func(ctx context.Context) error {
		conn := database.Conn(ctx)
		corrected := false
		for i := range projects {
			p := &projects[i]
			if p.Done < evaluatedFrom {
				continue
			}
			var id uuid.UUID
			var exists bool
			if err := conn.QueryRow(ctx, `SELECT p.id, EXISTS (SELECT 1 FROM evaluations e WHERE e.project_id = p.id)
				FROM projects p WHERE p.title = $1`, p.Title).Scan(&id, &exists); err != nil {
				return fmt.Errorf("%s: %w", p.Title, err)
			}
			if exists {
				continue
			}
			card := incubator.ProjectCard{Title: p.Title, Tagline: p.Tagline, Industry: p.Industry}
			data, versions, err := incubator.ProjectContext(ctx, database, id, card)
			if err != nil {
				return err
			}
			locale := contentLocale(data)
			current := evaluation.MockResult(data, locale)
			owner := people[p.Owner]
			last := now.AddDate(0, 0, -p.LastActive)
			// The earlier evaluation saw a weaker project.
			earlier := current
			earlier.Criteria = make([]evaluation.Score, len(current.Criteria))
			copy(earlier.Criteria, current.Criteria)
			for j := range earlier.Criteria {
				if j%2 == 0 {
					earlier.Criteria[j].Score = max(earlier.Criteria[j].Score-2, 0)
				}
			}
			if _, err := insertEvaluation(ctx, database, id, owner, locale, versions, earlier, last.AddDate(0, 0, -21)); err != nil {
				return err
			}
			evalID, err := insertEvaluation(ctx, database, id, owner, locale, versions, current, last.Add(-2*time.Hour))
			if err != nil {
				return err
			}
			if !corrected && p.Supervisor != "" {
				corrected = true
				if err := correct(ctx, database, evalID, people[p.Supervisor], locale, last.Add(-time.Hour)); err != nil {
					return err
				}
			}
		}
		return nil
	})
	if err != nil {
		return fmt.Errorf("seed: evaluations: %w", err)
	}
	return nil
}

func insertEvaluation(ctx context.Context, database *db.DB, projectID, owner uuid.UUID, locale string, versions map[string]int, r evaluation.Result, at time.Time) (uuid.UUID, error) {
	conn := database.Conn(ctx)
	scores := map[string]int{}
	for _, c := range r.Criteria {
		scores[c.Key] = c.Score
	}
	total := evaluation.DefaultRubric.Total(scores)
	rubric := make([]map[string]any, len(evaluation.DefaultRubric))
	for i, c := range evaluation.DefaultRubric {
		rubric[i] = map[string]any{"key": c.Key, "weight": c.Weight}
	}
	js := func(v any) []byte {
		b, _ := json.Marshal(v)
		return b
	}
	id, err := uuid.NewV7()
	if err != nil {
		return id, err
	}
	if _, err := conn.Exec(ctx, `INSERT INTO evaluations
		(id, tenant_id, project_id, status, locale, rubric, step_versions, ai_total, total, level, summary, red_flags,
		 hypotheses, experiments, recommendations, provider, model, prompt_version, requested_by, created_at, updated_at, completed_at)
		VALUES ($1, app_current_tenant(), $2, 'ready', $3, $4, $5, $6, $6, $7, $8, $9, $10, $11, $12, 'mock', 'mock-evaluation',
		 'evaluation@2', $13, $14, $15, $15)`,
		id, projectID, locale, js(rubric), js(versions), total, evaluation.Level(total), r.Summary, js(r.RedFlags),
		js(r.Hypotheses), js(r.Experiments), js(r.Recommendations), owner, at.Add(-3*time.Minute), at); err != nil {
		return id, err
	}
	for _, c := range r.Criteria {
		if _, err := conn.Exec(ctx, `INSERT INTO evaluation_criteria_scores (tenant_id, evaluation_id, criterion_key, ai_score, score, justification)
			VALUES (app_current_tenant(), $1, $2, $3, $3, $4)`, id, c.Key, c.Score, c.Justification); err != nil {
			return id, err
		}
	}
	return id, nil
}

var correctionComments = map[string]string{
	"kk": "Қорғауда команда пилоттың нәтижелері мен 14 сұхбаттың жазбаларын көрсетті — валидация инкубатордағы деректерден күштірек, балл көтерілді.",
	"ru": "На защите команда показала результаты пилота и записи 14 интервью — валидация сильнее, чем видно по данным в инкубаторе, балл повышен.",
	"en": "At the defence the team showed the pilot results and the notes of 14 interviews — validation is stronger than the incubator data shows, so the score is raised.",
}

// correct raises the validation score as the supervisor would after a
// defence (another criterion when validation already has the top score).
func correct(ctx context.Context, database *db.DB, evaluationID, author uuid.UUID, locale string, at time.Time) error {
	conn := database.Conn(ctx)
	var key string
	var old int
	if err := conn.QueryRow(ctx, `SELECT criterion_key, score FROM evaluation_criteria_scores
		WHERE evaluation_id = $1 AND score < 10
		ORDER BY criterion_key = 'validation' DESC, score
		LIMIT 1`, evaluationID).Scan(&key, &old); err != nil {
		return err
	}
	next := min(old+2, 10)
	if _, err := conn.Exec(ctx, `UPDATE evaluation_criteria_scores SET score = $3 WHERE evaluation_id = $1 AND criterion_key = $2`,
		evaluationID, key, next); err != nil {
		return err
	}
	if _, err := conn.Exec(ctx, `INSERT INTO evaluation_overrides (tenant_id, evaluation_id, criterion_key, old_score, new_score, comment, author_id, created_at)
		VALUES (app_current_tenant(), $1, $2, $3, $4, $5, $6, $7)`, evaluationID, key, old, next, correctionComments[locale], author, at); err != nil {
		return err
	}
	rows, err := conn.Query(ctx, `SELECT criterion_key, score FROM evaluation_criteria_scores WHERE evaluation_id = $1`, evaluationID)
	if err != nil {
		return err
	}
	scores := map[string]int{}
	for rows.Next() {
		var key string
		var score int
		if err := rows.Scan(&key, &score); err != nil {
			rows.Close()
			return err
		}
		scores[key] = score
	}
	rows.Close()
	total := evaluation.DefaultRubric.Total(scores)
	_, err = conn.Exec(ctx, `UPDATE evaluations SET total = $2, level = $3 WHERE id = $1`, evaluationID, total, evaluation.Level(total))
	return err
}

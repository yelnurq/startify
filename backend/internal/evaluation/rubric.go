// Package evaluation scores a project against a transparent rubric: an AI
// review with red flags, hypotheses and seven-day experiments, kept with its
// history, the prompt and the model, and adjustable by experts.
package evaluation

import (
	"errors"
	"fmt"
	"slices"

	"github.com/yelnurq/startify/backend/internal/ai/llm"
	"github.com/yelnurq/startify/backend/internal/apiconv"
)

// Criterion is a line of the rubric.
type Criterion struct {
	Key    string       `json:"key"`
	Weight int          `json:"weight"`
	Title  apiconv.Text `json:"title"`
	// Guide tells the model what to look at (English, like the prompts).
	Guide string `json:"-"`
	// Steps are the incubator steps the criterion draws on.
	Steps []string `json:"-"`
}

// Rubric is a set of weighted criteria; weights add up to 100.
type Rubric []Criterion

// DefaultRubric follows the platform specification; universities may change
// the weights.
var DefaultRubric = Rubric{
	{
		Key: "problem", Weight: 15,
		Title: apiconv.Text{"kk": "Мәселе және оның маңыздылығы", "ru": "Проблема и её значимость", "en": "Problem and its significance"},
		Guide: "Is the problem specific (who, situation, cost) and is it confirmed by evidence rather than assumed?",
		Steps: []string{"problem", "audience", "journey"},
	},
	{
		Key: "market", Weight: 15,
		Title: apiconv.Text{"kk": "Нарық және сұраныс", "ru": "Рынок и спрос", "en": "Market and demand"},
		Guide: "Is the market sized bottom-up with sources, are competitors and alternatives known, is there demand?",
		Steps: []string{"market_size", "competitors"},
	},
	{
		Key: "solution", Weight: 15,
		Title: apiconv.Text{"kk": "Шешім және инновациялылық", "ru": "Решение и инновационность", "en": "Solution and innovation"},
		Guide: "Does the solution remove the key pains, is the value proposition clear and is there real novelty?",
		Steps: []string{"solution", "mvp"},
	},
	{
		Key: "business_model", Weight: 15,
		Title: apiconv.Text{"kk": "Бизнес-модель және юнит-экономика", "ru": "Бизнес-модель и юнит-экономика", "en": "Business model and unit economics"},
		Guide: "Who pays, how much and through which channels; do the unit economics work (LTV/CAC) and are assumptions stated?",
		Steps: []string{"business_model", "unit_economics"},
	},
	{
		Key: "team", Weight: 10,
		Title: apiconv.Text{"kk": "Команда", "ru": "Команда", "en": "Team"},
		Guide: "Does the team cover product, technology and sales, with relevant experience and commitment?",
		Steps: []string{"team"},
	},
	{
		Key: "validation", Weight: 10,
		Title: apiconv.Text{"kk": "Валидация және трекшн", "ru": "Валидация и трекшн", "en": "Validation and traction"},
		Guide: "What has been tested with real users: interviews, pilots, confirmed hypotheses, metrics with current values?",
		Steps: []string{"problem", "metrics", "risks", "mvp"},
	},
	{
		Key: "feasibility", Weight: 10,
		Title: apiconv.Text{"kk": "Техникалық іске асырылуы", "ru": "Техническая реализуемость", "en": "Technical feasibility"},
		Guide: "Can this team build the MVP and the roadmap with the stated resources; are technical risks addressed?",
		Steps: []string{"mvp", "roadmap", "risks"},
	},
	{
		Key: "impact", Weight: 10,
		Title: apiconv.Text{
			"kk": "ҚР басымдықтарына сәйкестігі және әлеуметтік әсері",
			"ru": "Соответствие приоритетам РК и социальный эффект",
			"en": "Fit with Kazakhstan's priorities and social impact",
		},
		Guide: "Does the project serve priorities of Kazakhstan (agriculture, education, health, ecology, digital public services, regions) and create social benefit?",
		Steps: []string{"problem", "solution", "roadmap"},
	},
}

// Validate checks keys and weights.
func (r Rubric) Validate() error {
	sum := 0
	seen := map[string]bool{}
	for _, c := range r {
		if seen[c.Key] {
			return fmt.Errorf("evaluation: duplicate criterion %q", c.Key)
		}
		if !slices.ContainsFunc(DefaultRubric, func(d Criterion) bool { return d.Key == c.Key }) {
			return fmt.Errorf("evaluation: unknown criterion %q", c.Key)
		}
		if c.Weight < 0 || c.Weight > 100 {
			return fmt.Errorf("evaluation: weight of %q out of range", c.Key)
		}
		seen[c.Key] = true
		sum += c.Weight
	}
	if len(seen) != len(DefaultRubric) {
		return errors.New("evaluation: every criterion needs a weight")
	}
	if sum != 100 {
		return fmt.Errorf("evaluation: weights add up to %d, not 100", sum)
	}
	return nil
}

// WithWeights returns the default rubric with the university's weights.
func WithWeights(weights map[string]int) Rubric {
	out := make(Rubric, len(DefaultRubric))
	copy(out, DefaultRubric)
	for i := range out {
		if w, ok := weights[out[i].Key]; ok {
			out[i].Weight = w
		}
	}
	return out
}

// customized reports whether the weights differ from the default rubric.
func (r Rubric) customized() bool {
	for i, c := range r {
		if i >= len(DefaultRubric) || DefaultRubric[i].Key != c.Key || DefaultRubric[i].Weight != c.Weight {
			return true
		}
	}
	return len(r) != len(DefaultRubric)
}

// Total is the weighted score 0–100 of criterion scores 0–10.
func (r Rubric) Total(scores map[string]int) int {
	sum := 0
	for _, c := range r {
		sum += c.Weight * scores[c.Key]
	}
	return (sum + 5) / 10
}

// Levels of readiness by total score.
const (
	LevelIdea             = "idea"
	LevelValidatedProblem = "validated_problem"
	LevelMVPReady         = "mvp_ready"
	LevelInvestmentReady  = "investment_ready"
)

// Level maps a total score to the readiness level: the thresholds are fixed
// so that the level always agrees with the score.
func Level(total int) string {
	switch {
	case total >= 75:
		return LevelInvestmentReady
	case total >= 55:
		return LevelMVPReady
	case total >= 35:
		return LevelValidatedProblem
	}
	return LevelIdea
}

func text(maxLength int) map[string]any {
	return map[string]any{"type": "string", "maxLength": maxLength}
}

// resultSchema constrains the model's evaluation.
func resultSchema(r Rubric) llm.Schema {
	keys := make([]any, len(r))
	for i, c := range r {
		keys[i] = c.Key
	}
	return llm.Schema{Name: "project_evaluation", JSON: map[string]any{
		"type":     "object",
		"required": []any{"criteria", "red_flags", "hypotheses", "experiments", "recommendations", "summary"},
		"properties": map[string]any{
			"criteria": map[string]any{
				"type": "array", "minItems": len(r), "maxItems": len(r),
				"items": map[string]any{
					"type":     "object",
					"required": []any{"key", "score", "justification"},
					"properties": map[string]any{
						"key":           map[string]any{"type": "string", "enum": keys},
						"score":         map[string]any{"type": "integer", "minimum": 0, "maximum": 10},
						"justification": text(900),
					},
				},
			},
			"red_flags": map[string]any{
				"type": "array", "maxItems": 6,
				"items": map[string]any{
					"type":     "object",
					"required": []any{"title", "detail", "severity"},
					"properties": map[string]any{
						"title":    text(200),
						"detail":   text(600),
						"severity": map[string]any{"type": "string", "enum": []any{"low", "medium", "high"}},
					},
				},
			},
			"hypotheses": map[string]any{"type": "array", "minItems": 3, "maxItems": 5, "items": text(400)},
			"experiments": map[string]any{
				"type": "array", "minItems": 1, "maxItems": 5,
				"items": map[string]any{
					"type":     "object",
					"required": []any{"title", "how", "metric", "success", "day"},
					"properties": map[string]any{
						"title":   text(200),
						"how":     text(500),
						"metric":  text(300),
						"success": text(300),
						"day":     map[string]any{"type": "integer", "minimum": 1, "maximum": 7},
					},
				},
			},
			"recommendations": map[string]any{"type": "array", "minItems": 2, "maxItems": 5, "items": text(400)},
			"summary":         text(800),
		},
	}}
}

// Score is the score of one criterion.
type Score struct {
	Key           string `json:"key"`
	Score         int    `json:"score"`
	Justification string `json:"justification"`
}

// RedFlag is a serious weakness of the project.
type RedFlag struct {
	Title    string `json:"title"`
	Detail   string `json:"detail"`
	Severity string `json:"severity"`
}

// Experiment is a seven-day test of a hypothesis.
type Experiment struct {
	Title   string `json:"title"`
	How     string `json:"how"`
	Metric  string `json:"metric"`
	Success string `json:"success"`
	Day     int    `json:"day"`
}

// Result is the model's evaluation.
type Result struct {
	Criteria        []Score      `json:"criteria"`
	RedFlags        []RedFlag    `json:"red_flags"`
	Hypotheses      []string     `json:"hypotheses"`
	Experiments     []Experiment `json:"experiments"`
	Recommendations []string     `json:"recommendations"`
	Summary         string       `json:"summary"`
}

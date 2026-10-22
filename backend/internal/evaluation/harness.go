package evaluation

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"slices"
	"strings"

	"github.com/yelnurq/startify/backend/internal/ai/llm"
	"github.com/yelnurq/startify/backend/internal/ai/schema"
	"github.com/yelnurq/startify/backend/internal/incubator"
)

// Case is a golden project of the evaluation harness with what a sound
// reviewer must conclude about it.
type Case struct {
	Name    string `json:"name"`
	About   string `json:"about"`
	Locale  string `json:"locale"`
	Project struct {
		Title    string `json:"title"`
		Industry string `json:"industry"`
	} `json:"project"`
	// Steps holds the answers, or "@seed:<title>" for a demo project.
	Steps  json.RawMessage `json:"steps"`
	Expect struct {
		MinTotal *int     `json:"min_total"`
		MaxTotal *int     `json:"max_total"`
		Levels   []string `json:"levels"`
		Language string   `json:"language"`
	} `json:"expect"`
}

// LoadCases reads the golden cases; seed resolves "@seed:<title>" steps.
func LoadCases(path string, seed func(title string) (map[string]incubator.Values, bool)) ([]Case, error) {
	raw, err := os.ReadFile(path) //nolint:gosec // a path given by the developer
	if err != nil {
		return nil, err
	}
	var cases []Case
	if err := json.Unmarshal(raw, &cases); err != nil {
		return nil, fmt.Errorf("evaluation: cases: %w", err)
	}
	for i := range cases {
		var ref string
		if json.Unmarshal(cases[i].Steps, &ref) != nil {
			continue
		}
		title, ok := strings.CutPrefix(ref, "@seed:")
		steps, found := seed(title)
		if !ok || !found {
			return nil, fmt.Errorf("evaluation: case %s: unknown steps %q", cases[i].Name, ref)
		}
		if cases[i].Steps, err = json.Marshal(steps); err != nil {
			return nil, err
		}
	}
	return cases, nil
}

// Data is the project JSON of the case, as the incubator gives it.
func (c *Case) Data() string {
	raw, _ := json.Marshal(map[string]any{
		"project": map[string]any{"title": c.Project.Title, "tagline": "", "industry": c.Project.Industry},
		"steps":   c.Steps,
	})
	return string(raw)
}

// Outcome is the harness's verdict on one case.
type Outcome struct {
	Case     string
	Total    int
	Level    string
	Problems []string
}

// RunCase evaluates a case with a provider and checks the expectations: the
// answer matches the schema, covers the rubric, and the score, level and
// language are what a sound reviewer would give.
func RunCase(ctx context.Context, p llm.Provider, model string, c *Case) Outcome {
	out := Outcome{Case: c.Name}
	system, version, s, err := Prompt(DefaultRubric, c.Data(), c.Locale)
	if err != nil {
		out.Problems = append(out.Problems, err.Error())
		return out
	}
	res, err := p.Complete(ctx, llm.CompletionRequest{
		Model: model, System: system, JSONSchema: &s, MaxTokens: 8000,
		Messages: []llm.Message{{Role: llm.RoleUser, Content: "Evaluate the project."}},
		Metadata: llm.RequestMeta{Feature: llm.FeatureEvaluation, PromptVersion: version, Locale: c.Locale},
	})
	if err != nil {
		out.Problems = append(out.Problems, "model: "+err.Error())
		return out
	}
	v, err := schema.Compile(s.Name, s.JSON)
	if err != nil {
		out.Problems = append(out.Problems, err.Error())
		return out
	}
	if _, err := v.Validate(res.Text); err != nil {
		out.Problems = append(out.Problems, err.Error())
		return out
	}
	result, scores, err := Parse(DefaultRubric, schema.Extract(res.Text))
	if err != nil {
		out.Problems = append(out.Problems, err.Error())
		return out
	}
	out.Total = DefaultRubric.Total(scores)
	out.Level = Level(out.Total)
	e := c.Expect
	if e.MinTotal != nil && out.Total < *e.MinTotal {
		out.Problems = append(out.Problems, fmt.Sprintf("total %d is below %d", out.Total, *e.MinTotal))
	}
	if e.MaxTotal != nil && out.Total > *e.MaxTotal {
		out.Problems = append(out.Problems, fmt.Sprintf("total %d is above %d", out.Total, *e.MaxTotal))
	}
	if len(e.Levels) > 0 && !slices.Contains(e.Levels, out.Level) {
		out.Problems = append(out.Problems, fmt.Sprintf("level %s, expected one of %v", out.Level, e.Levels))
	}
	all := true
	for _, sc := range scores {
		all = all && sc == 10
	}
	if all {
		out.Problems = append(out.Problems, "every criterion got 10")
	}
	if e.Language == "kk" && !strings.ContainsAny(result.Summary+strings.Join(result.Recommendations, " "), "әғқңөұүһі") {
		out.Problems = append(out.Problems, "the texts are not in Kazakh")
	}
	return out
}

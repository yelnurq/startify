package evaluation

import (
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"maps"
	"os"
	"slices"
	"strings"
	"unicode"

	"github.com/google/uuid"

	"github.com/yelnurq/startify/backend/internal/ai"
	"github.com/yelnurq/startify/backend/internal/ai/llm"
	"github.com/yelnurq/startify/backend/internal/ai/mock"
	"github.com/yelnurq/startify/backend/internal/incubator"
	"github.com/yelnurq/startify/backend/internal/platform/config"
	"github.com/yelnurq/startify/backend/internal/platform/tenancy"
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
	Steps json.RawMessage `json:"steps"`
	// Override replaces fields of the steps (after a seed is resolved), so
	// that a case can be a strong project with one weakness.
	Override map[string]map[string]any `json:"override"`
	// Drop removes whole steps.
	Drop   []string `json:"drop"`
	Expect Expect   `json:"expect"`
}

// Expect is what a sound reviewer concludes about a case.
type Expect struct {
	MinTotal *int     `json:"min_total"`
	MaxTotal *int     `json:"max_total"`
	Levels   []string `json:"levels"`
	// Language of the texts: kk, ru or en.
	Language string         `json:"language"`
	RedFlags *RedFlagExpect `json:"red_flags"`
	// Above names weaker cases this one must score higher than.
	Above []string `json:"above"`
}

// RedFlagExpect is a weakness the review must find.
type RedFlagExpect struct {
	Min int `json:"min"`
	// High: at least one red flag is of high severity.
	High bool `json:"high"`
	// Mentions: each entry lists alternatives ("LTV|CAC|юнит"), one of which
	// a red flag must name (case-insensitive).
	Mentions []string `json:"mentions"`
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
	names := map[string]bool{}
	for i := range cases {
		c := &cases[i]
		if names[c.Name] {
			return nil, fmt.Errorf("evaluation: case %s: duplicate name", c.Name)
		}
		names[c.Name] = true
		steps := map[string]incubator.Values{}
		var ref string
		if json.Unmarshal(c.Steps, &ref) == nil {
			title, ok := strings.CutPrefix(ref, "@seed:")
			found := map[string]incubator.Values(nil)
			if ok {
				found, ok = seed(title)
			}
			if !ok {
				return nil, fmt.Errorf("evaluation: case %s: unknown steps %q", c.Name, ref)
			}
			for k, v := range found {
				steps[k] = maps.Clone(v)
			}
		} else if err := json.Unmarshal(c.Steps, &steps); err != nil {
			return nil, fmt.Errorf("evaluation: case %s: steps: %w", c.Name, err)
		}
		for step, fields := range c.Override {
			if steps[step] == nil {
				steps[step] = incubator.Values{}
			}
			maps.Copy(steps[step], fields)
		}
		for _, step := range c.Drop {
			delete(steps, step)
		}
		if c.Steps, err = json.Marshal(steps); err != nil {
			return nil, err
		}
	}
	for i := range cases {
		for _, weaker := range cases[i].Expect.Above {
			if !names[weaker] {
				return nil, fmt.Errorf("evaluation: case %s: unknown case %q in above", cases[i].Name, weaker)
			}
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
	Case string
	// Evaluated: the model gave a usable evaluation.
	Evaluated bool
	Total     int
	Level     string
	Problems  []string
}

// HarnessGateway is a gateway without a database whose "evaluation" feature
// goes to one provider (and model, if given): what `make ai-eval` runs.
func HarnessGateway(cfg config.AI, providers []llm.Provider, provider, model string, log *slog.Logger) (*ai.Gateway, error) {
	route := provider
	if model != "" {
		route += ":" + model
	}
	cfg.Routes = "evaluation=" + route
	return ai.NewGateway(nil, providers, cfg, log)
}

// RunCase evaluates a case through the gateway exactly as the worker does
// (route of the "evaluation" feature, schema check, parsing) and checks the
// expectations: score, level, language and red flags.
func RunCase(ctx context.Context, g *ai.Gateway, c *Case) Outcome {
	if _, ok := tenancy.FromContext(ctx); !ok {
		ctx = tenancy.With(ctx, tenancy.Tenant{Slug: "ai-eval", Locale: c.Locale})
	}
	out := Outcome{Case: c.Name}
	scored, err := Evaluate(ctx, g, DefaultRubric, c.Data(), c.Locale, uuid.Nil)
	if err != nil {
		out.Problems = append(out.Problems, "evaluation: "+err.Error())
		return out
	}
	out.Evaluated, out.Total, out.Level = true, scored.Total, scored.Level
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
	for _, sc := range scored.Scores {
		all = all && sc == 10
	}
	if all {
		out.Problems = append(out.Problems, "every criterion got 10")
	}
	if p := language(scored.Result, e.Language); p != "" {
		out.Problems = append(out.Problems, p)
	}
	out.Problems = append(out.Problems, checkRedFlags(scored.Result.RedFlags, e.RedFlags)...)
	return out
}

// language checks that the texts a team reads are in the requested language.
func language(r Result, want string) string {
	text := r.Summary + " " + strings.Join(r.Recommendations, " ")
	cyrillic := strings.IndexFunc(text, func(c rune) bool { return unicode.Is(unicode.Cyrillic, c) }) >= 0
	switch {
	case want == "kk" && !strings.ContainsAny(strings.ToLower(text), "әғқңөұүһі"):
		return "the texts are not in Kazakh"
	case want == "ru" && !cyrillic:
		return "the texts are not in Russian"
	case want == "en" && cyrillic:
		return "the texts are not in English"
	}
	return ""
}

func checkRedFlags(flags []RedFlag, e *RedFlagExpect) []string {
	if e == nil {
		return nil
	}
	var problems []string
	if len(flags) < e.Min {
		problems = append(problems, fmt.Sprintf("%d red flags, expected at least %d", len(flags), e.Min))
	}
	if e.High && !slices.ContainsFunc(flags, func(f RedFlag) bool { return f.Severity == "high" }) {
		problems = append(problems, "no red flag of high severity")
	}
	for _, mention := range e.Mentions {
		found := slices.ContainsFunc(flags, func(f RedFlag) bool {
			text := strings.ToLower(f.Title + " " + f.Detail)
			return slices.ContainsFunc(strings.Split(mention, "|"), func(term string) bool {
				return strings.Contains(text, strings.ToLower(term))
			})
		})
		if !found {
			problems = append(problems, fmt.Sprintf("no red flag names %q", mention))
		}
	}
	return problems
}

// Rank checks that stronger projects score above weaker ones; problems are
// added to the outcome of the stronger case.
func Rank(cases []Case, outcomes []Outcome) {
	byName := map[string]*Outcome{}
	for i := range outcomes {
		byName[outcomes[i].Case] = &outcomes[i]
	}
	for i := range cases {
		strong := byName[cases[i].Name]
		if strong == nil || !strong.Evaluated {
			continue
		}
		for _, name := range cases[i].Expect.Above {
			if weak := byName[name]; weak != nil && weak.Evaluated && strong.Total <= weak.Total {
				strong.Problems = append(strong.Problems, fmt.Sprintf("total %d is not above %s (%d)", strong.Total, name, weak.Total))
			}
		}
	}
}

// FencedMock answers like the mock but wraps structured answers in a
// sentence and a Markdown fence, as many local models do: the harness runs
// it to prove that evaluations read the value the gateway validated (С-16).
type FencedMock struct{ mock.Provider }

var _ llm.Provider = (*FencedMock)(nil)

// Name implements llm.Provider.
func (p *FencedMock) Name() string { return "mock-fenced" }

// Complete implements llm.Provider.
func (p *FencedMock) Complete(ctx context.Context, req llm.CompletionRequest) (*llm.CompletionResponse, error) {
	res, err := p.Provider.Complete(ctx, req)
	if err == nil && req.JSONSchema != nil {
		res.Text = "Here is the evaluation:\n```json\n" + res.Text + "\n```"
	}
	return res, err
}

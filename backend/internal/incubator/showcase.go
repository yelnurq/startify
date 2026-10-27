package incubator

import (
	"context"
	"fmt"
	"strings"

	"github.com/google/uuid"

	"github.com/yelnurq/startify/backend/internal/incubator/incubatordb"
	"github.com/yelnurq/startify/backend/internal/platform/db"
)

// ShowcaseDraft is the showcase page suggested from the incubator answers.
// It never contains sensitive fields (unit economics, the novelty and the
// unfair advantage of the solution), whatever the project hides: they are
// left out before the draft is assembled.
type ShowcaseDraft struct {
	Tagline  string
	Problem  string
	Solution string
	Market   string
	Traction string
	Ask      string
	VideoURL string
	// Roles maps the names in the team step to their roles.
	Roles map[string]string
}

// Showcase limits of the page sections.
const (
	showcaseLong  = 1500
	showcaseShort = 1000
	showcaseAsk   = 800
)

// DraftShowcase assembles the showcase page from the project's answers.
func DraftShowcase(ctx context.Context, database *db.DB, projectID uuid.UUID, tagline string) (ShowcaseDraft, error) {
	var rows []incubatordb.ProjectStepDatum
	err := database.ReadTx(ctx, func(ctx context.Context) error {
		var err error
		rows, err = incubatordb.New(database.Conn(ctx)).ListSteps(ctx, projectID)
		return err
	})
	if err != nil {
		return ShowcaseDraft{}, fmt.Errorf("incubator: showcase draft: %w", err)
	}
	tmpl := Default()
	steps := map[string]Values{}
	for i := range rows {
		if st, ok := tmpl.Step(rows[i].StepKey); ok {
			steps[rows[i].StepKey] = st.Redact(decodeValues(rows[i].Data))
		}
	}
	get := func(step, field string) string {
		s, _ := steps[step][field].(string)
		return strings.TrimSpace(s)
	}
	paragraphs := func(parts ...string) string {
		var keep []string
		for _, p := range parts {
			if p != "" {
				keep = append(keep, p)
			}
		}
		return strings.Join(keep, "\n\n")
	}

	d := ShowcaseDraft{
		Tagline:  cut(firstNonEmpty(tagline, get("pitch", "one_liner"), get("solution", "uvp")), 160),
		Problem:  cut(paragraphs(get("problem", "statement"), get("problem", "cost")), showcaseLong),
		Solution: cut(paragraphs(get("solution", "solution"), get("solution", "uvp")), showcaseLong),
		Traction: cut(paragraphs(get("metrics", "traction"), sentence(get("metrics", "north_star"))), showcaseShort),
		Ask:      cut(paragraphs(get("pitch", "ask"), get("roadmap", "use_of_funds")), showcaseAsk),
		VideoURL: get("pitch", "video_url"),
		Roles:    map[string]string{},
	}
	var market []string
	for _, k := range []string{"tam", "sam", "som"} {
		if v, ok := steps["market_size"][k].(float64); ok {
			market = append(market, strings.ToUpper(k)+": "+FormatKZT(v))
		}
	}
	d.Market = cut(paragraphs(strings.Join(market, " · "), get("competitors", "difference")), showcaseShort)
	members, _ := steps["team"]["members"].([]any)
	for _, it := range members {
		m, _ := it.(map[string]any)
		name, _ := m["name"].(string)
		role, _ := m["role"].(string)
		if name = strings.TrimSpace(name); name != "" {
			d.Roles[strings.ToLower(name)] = strings.TrimSpace(role)
		}
	}
	return d, nil
}

// StageOf returns the stage of a step of the default template.
func StageOf(stepKey string) string {
	if st, ok := Default().Step(stepKey); ok {
		return st.Stage()
	}
	return Default().Stages[0].Key
}

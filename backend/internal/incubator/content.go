package incubator

import (
	"encoding/json"
	"math"
	"strconv"
	"strings"
	"unicode/utf8"

	"github.com/yelnurq/startify/backend/internal/ai/llm"
)

// PitchContent is the text of the pitch materials, written by the model from
// the project data (or assembled from it by the mock).
type PitchContent struct {
	Tagline     string  `json:"tagline"`
	Problem     string  `json:"problem"`
	Customer    string  `json:"customer"`
	Solution    string  `json:"solution"`
	Market      string  `json:"market"`
	Competition string  `json:"competition"`
	Business    string  `json:"business_model"`
	Traction    string  `json:"traction"`
	Team        string  `json:"team"`
	Ask         string  `json:"ask"`
	Slides      []Slide `json:"slides"`
}

// Slide is one slide of the pitch deck.
type Slide struct {
	Title   string   `json:"title"`
	Bullets []string `json:"bullets"`
}

// DeckSlides is the length of the pitch deck.
const DeckSlides = 12

func text(maxLength int) map[string]any {
	return map[string]any{"type": "string", "maxLength": maxLength}
}

// pitchSchema constrains PitchContent.
var pitchSchema = llm.Schema{Name: "pitch_content", JSON: map[string]any{
	"type": "object",
	"required": []any{
		"tagline", "problem", "customer", "solution", "market", "competition", "business_model", "traction", "team", "ask", "slides",
	},
	"properties": map[string]any{
		"tagline": text(160), "problem": text(700), "customer": text(500), "solution": text(700), "market": text(500),
		"competition": text(500), "business_model": text(500), "traction": text(500), "team": text(500), "ask": text(500),
		"slides": map[string]any{
			"type": "array", "minItems": 1, "maxItems": DeckSlides,
			"items": map[string]any{
				"type": "object", "required": []any{"title", "bullets"},
				"properties": map[string]any{
					"title":   text(80),
					"bullets": map[string]any{"type": "array", "maxItems": 4, "items": text(160)},
				},
			},
		},
	},
}}

var slideTitles = map[string][DeckSlides]string{
	"kk": {"", "Мәселе", "Клиент", "Шешім", "Нарық", "Бәсекелестер", "Бизнес-модель", "Трекшн және метрикалар", "Команда", "Жол картасы", "Бізге не қажет", "Байланыс"},
	"ru": {"", "Проблема", "Клиент", "Решение", "Рынок", "Конкуренты", "Бизнес-модель", "Трекшн и метрики", "Команда", "Дорожная карта", "Что нам нужно", "Контакты"},
	"en": {"", "Problem", "Customer", "Solution", "Market", "Competitors", "Business model", "Traction and metrics", "Team", "Roadmap", "The ask", "Contact"},
}

// nbsp keeps an amount on one line.
const nbsp = rune(0xa0)

// FormatKZT writes an amount of tenge in groups of three digits joined by
// no-break spaces: 1 500 000 ₸.
func FormatKZT(v float64) string {
	n := strconv.FormatInt(int64(math.Round(v)), 10)
	var b strings.Builder
	for i, r := range n {
		if i > 0 && (len(n)-i)%3 == 0 {
			b.WriteRune(nbsp)
		}
		b.WriteRune(r)
	}
	b.WriteRune(nbsp)
	return b.String() + "₸"
}

// sentence ends a fragment with a full stop unless it has a final mark.
func sentence(s string) string {
	s = strings.TrimSpace(s)
	if s == "" || strings.ContainsRune(".!?…", []rune(s)[len([]rune(s))-1]) {
		return s
	}
	return s + "."
}

// sentences splits a text into at most n short sentences for bullets.
func sentences(s string, n int) []string {
	var out []string
	for _, part := range strings.FieldsFunc(s, func(r rune) bool { return r == '.' || r == '!' || r == '?' || r == '\n' }) {
		part = strings.TrimSpace(part)
		if utf8.RuneCountInString(part) < 3 {
			continue
		}
		out = append(out, cut(part, 150))
		if len(out) == n {
			break
		}
	}
	return out
}

// assemble builds the materials from the answers alone, without inventing
// anything: what the team has not written stays out.
func assemble(d promptData, locale string) PitchContent {
	get := func(step, field string) string {
		s, _ := d.Steps[step][field].(string)
		return strings.TrimSpace(s)
	}
	rows := func(step, field string) []map[string]any {
		items, _ := d.Steps[step][field].([]any)
		out := []map[string]any{}
		for _, it := range items {
			if m, ok := it.(map[string]any); ok {
				out = append(out, m)
			}
		}
		return out
	}
	list := func(step, field string, n int) []string {
		items, _ := d.Steps[step][field].([]any)
		var out []string
		for _, it := range items {
			if s, ok := it.(string); ok && strings.TrimSpace(s) != "" && len(out) < n {
				out = append(out, strings.TrimSpace(s))
			}
		}
		return out
	}
	join := func(parts ...string) string {
		var keep []string
		for _, p := range parts {
			if p != "" {
				keep = append(keep, p)
			}
		}
		return strings.Join(keep, " ")
	}
	first := func(s string) string {
		if out := sentences(s, 1); len(out) > 0 {
			return out[0] + "."
		}
		return ""
	}

	c := PitchContent{
		Tagline:     firstNonEmpty(get("pitch", "one_liner"), get("solution", "uvp"), d.Project.Tagline),
		Problem:     join(first(get("problem", "statement")), first(get("problem", "cost"))),
		Customer:    join(first(get("audience", "segments")), first(get("audience", "early_adopters"))),
		Solution:    join(first(get("solution", "solution")), first(get("solution", "uvp"))),
		Competition: first(get("competitors", "difference")),
		Business:    join(first(get("business_model", "revenue")), first(get("business_model", "pricing"))),
		Traction:    join(sentence(get("metrics", "north_star")), first(get("metrics", "traction"))),
		Ask:         join(first(get("pitch", "ask")), first(get("roadmap", "use_of_funds"))),
	}
	var market []string
	for _, k := range []string{"tam", "sam", "som"} {
		if v, ok := d.Steps["market_size"][k].(float64); ok {
			market = append(market, strings.ToUpper(k)+": "+FormatKZT(v))
		}
	}
	c.Market = strings.Join(market, " · ")
	var team []string
	for _, m := range rows("team", "members") {
		name, _ := m["name"].(string)
		role, _ := m["role"].(string)
		if name != "" {
			team = append(team, strings.TrimSpace(name+" — "+role))
		}
	}
	c.Team = strings.Join(team, "; ")

	titles := slideTitles[locale]
	var competitors, milestones []string
	for _, r := range rows("competitors", "list") {
		if name, _ := r["name"].(string); name != "" {
			competitors = append(competitors, name)
		}
	}
	for _, r := range rows("roadmap", "milestones") {
		period, _ := r["period"].(string)
		goal, _ := r["goal"].(string)
		if goal != "" && len(milestones) < 4 {
			milestones = append(milestones, strings.TrimSpace(period+": "+goal))
		}
	}
	ask := sentences(get("pitch", "ask"), 2)
	if v, ok := d.Steps["roadmap"]["funding"].(float64); ok && v > 0 {
		ask = append(ask, FormatKZT(v))
	}
	bullets := [DeckSlides][]string{
		{firstNonEmpty(c.Tagline, d.Project.Title)},
		sentences(get("problem", "statement")+". "+get("problem", "cost"), 3),
		sentences(get("audience", "segments")+". "+get("audience", "early_adopters"), 3),
		append(sentences(get("solution", "solution"), 2), list("solution", "features", 2)...),
		market,
		append(limit(competitors, 3), sentences(get("competitors", "difference"), 1)...),
		sentences(get("business_model", "revenue")+". "+get("business_model", "pricing"), 3),
		append(nonEmpty(get("metrics", "north_star")), sentences(get("metrics", "traction"), 2)...),
		limit(team, 4),
		milestones,
		ask,
		nonEmpty(get("pitch", "contact")),
	}
	for i, b := range bullets {
		title := titles[i]
		if i == 0 {
			title = d.Project.Title
		}
		if len(b) == 0 && i != 0 {
			continue
		}
		b = limit(b, 4)
		for j := range b {
			b[j] = cut(b[j], 160)
		}
		c.Slides = append(c.Slides, Slide{Title: cut(title, 80), Bullets: b})
	}
	for _, f := range []*string{&c.Customer, &c.Market, &c.Competition, &c.Business, &c.Traction, &c.Team, &c.Ask} {
		*f = cut(*f, 500)
	}
	c.Tagline, c.Problem, c.Solution = cut(c.Tagline, 160), cut(c.Problem, 700), cut(c.Solution, 700)
	return c
}

// cut shortens a text to at most n characters.
func cut(s string, n int) string {
	if utf8.RuneCountInString(s) <= n {
		return s
	}
	return string([]rune(s)[:n-1]) + "…"
}

func firstNonEmpty(vs ...string) string {
	for _, v := range vs {
		if v != "" {
			return v
		}
	}
	return ""
}

func nonEmpty(s string) []string {
	if s == "" {
		return nil
	}
	return []string{s}
}

func limit(s []string, n int) []string {
	if len(s) > n {
		return s[:n]
	}
	return s
}

func mockMaterials(req llm.CompletionRequest) string {
	var d promptData
	_ = json.Unmarshal([]byte(between(req.System, "project_data")), &d)
	raw, err := json.Marshal(assemble(d, localeOf(req)))
	if err != nil {
		return "{}"
	}
	return string(raw)
}

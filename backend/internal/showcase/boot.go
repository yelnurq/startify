package showcase

import (
	_ "embed"
	"html/template"
	"net/http"
	"strings"

	"golang.org/x/text/language"

	"github.com/yelnurq/startify/backend/internal/apiconv"
	"github.com/yelnurq/startify/backend/internal/apigen"
	"github.com/yelnurq/startify/backend/internal/platform/apperr"
	"github.com/yelnurq/startify/backend/internal/showcase/showcasedb"
)

// The first screen of public pages is drawn here, in the language the web
// client will choose, so that phones see the page before the client has
// loaded; the client draws the same page behind it and then removes it
// (frontend src/app/boot.ts).

//go:embed boot.html
var bootHTML string

var bootTemplate = template.Must(template.New("boot").Parse(bootHTML))

// bootLocales are the client's languages; without a choice it falls back to
// Kazakh (frontend src/shared/i18n DEFAULT_LOCALE).
var bootLocales = []string{"kk", "ru", "en"}

// localeCookie mirrors the language saved by the web client.
const localeCookie = "qadam_locale"

// bootText are the client's strings shown on the first screen (namespace:key
// of frontend/src/locales; a test keeps them equal).
var bootText = map[string]apiconv.Text{
	"landing:hero.eyebrow":     {"kk": "Университеттің AI-инновация платформасы", "ru": "AI-платформа инноваций университета", "en": "AI platform for university innovation"},
	"landing:hero.titleLead":   {"kk": "Идеядан нарыққа —", "ru": "От идеи до рынка —", "en": "From idea to market —"},
	"landing:hero.titleAccent": {"kk": "бір қадам", "ru": "один шаг", "en": "one step"},
	"landing:hero.subtitle": {
		"kk": "Qadam студенттік жобаны алғашқы идеядан Demo Day-ге және инвестормен кездесуге дейін жеткізеді: AI-ментор, команда, сараптама және витрина — бәрі бір жерде.",
		"ru": "Qadam ведёт студенческий проект от первой идеи до Demo Day и встречи с инвестором: AI-ментор, команда, экспертиза и витрина — в одном месте.",
		"en": "Qadam takes a student project from the first idea to Demo Day and a meeting with an investor: an AI mentor, a team, expert review and a showcase — all in one place.",
	},
	"showcase:catalog.eyebrow": {"kk": "Жобалар витринасы", "ru": "Витрина проектов", "en": "Project showcase"},
	"showcase:catalog.title":   {"kk": "Серіктес іздеп жүрген жобалар", "ru": "Проекты, которые ищут партнёров", "en": "Projects looking for partners"},
	"showcase:catalog.subtitle": {
		"kk": "{{university}} студенттік стартаптары мен инновациялық жобалары: сала, кезең және команда іздейтін қолдау бойынша іздеңіз.",
		"ru": "Студенческие стартапы и инновационные проекты {{university}}: ищите по отрасли, стадии и тому, что нужно команде.",
		"en": "Student startups and innovation projects of {{university}}: search by industry, stage and what the team needs.",
	},
	"showcase:catalog.titleNational": {"kk": "Қазақстан университеттерінің жобалары", "ru": "Проекты университетов Казахстана", "en": "Projects of Kazakhstan's universities"},
	"showcase:catalog.subtitleNational": {
		"kk": "Qadam платформасындағы барлық университеттің ашық жобалары — идеядан инвестицияға дайындыққа дейін.",
		"ru": "Публичные проекты всех университетов на платформе Qadam — от идеи до готовности к инвестициям.",
		"en": "Public projects of every university on Qadam — from an idea to investment readiness.",
	},
	"showcase:site.showcase": {"kk": "Витрина", "ru": "Витрина", "en": "Showcase"},
	"common:breadcrumbs":     {"kk": "Навигациялық тізбек", "ru": "Навигационная цепочка", "en": "Breadcrumb"},
}

// coverGradients repeat the placeholder covers of the client's cards
// (frontend src/features/showcase/card.tsx, GRADIENTS and INDUSTRY_GRADIENT).
var coverGradients = []string{
	"from-deep-700 to-ink-950",
	"from-ink-800 to-ink-600",
	"from-primary-700 to-deep-900",
	"from-deep-900 to-deep-800",
}

var industryGradient = map[string]int{
	"agritech": 2, "edtech": 0, "fintech": 3, "healthtech": 1, "logistics": 0, "cleantech": 2, "govtech": 1,
	"foodtech": 3, "fashion": 3, "ecommerce": 0, "manufacturing": 1, "energy": 3, "tourism": 2, "social": 1, "other": 0,
}

// bootPage is what the first screen shows.
type bootPage struct {
	Lang       string
	University string
	// Kind is landing, catalog or project.
	Kind     string
	Kicker   string
	Title    string
	Accent   string
	Text     string
	Crumbs   string
	Cover    string
	Gradient string
	Industry string
	Stage    string
}

func bootString(key, locale string) string { return bootText[key].Best(locale) }

// bootLocale picks the language the web client will show: the saved choice,
// then the browser's languages in order, then Kazakh.
func bootLocale(r *http.Request) string {
	supported := func(tag string) string {
		base := strings.ToLower(strings.SplitN(tag, "-", 2)[0])
		for _, l := range bootLocales {
			if base == l {
				return l
			}
		}
		return ""
	}
	if c, err := r.Cookie(localeCookie); err == nil {
		if l := supported(c.Value); l != "" {
			return l
		}
	}
	tags, _, err := language.ParseAcceptLanguage(r.Header.Get("Accept-Language"))
	if err == nil {
		for _, tag := range tags {
			if l := supported(tag.String()); l != "" {
				return l
			}
		}
	}
	return bootLocales[0]
}

func renderBoot(p *bootPage) (string, error) {
	var b strings.Builder
	if err := bootTemplate.Execute(&b, p); err != nil {
		return "", apperr.Internal(err)
	}
	return strings.TrimSpace(b.String()), nil
}

// bootUniversity names the university in the header for the language.
func bootUniversity(u apigen.UniversityBrief, locale string) string { return best(u.Name, locale) }

func landingBoot(u apigen.UniversityBrief, locale string) bootPage {
	return bootPage{
		Lang: locale, University: bootUniversity(u, locale), Kind: "landing",
		Kicker: bootString("landing:hero.eyebrow", locale),
		Title:  bootString("landing:hero.titleLead", locale), Accent: bootString("landing:hero.titleAccent", locale),
		Text: bootString("landing:hero.subtitle", locale),
	}
}

func catalogBoot(u apigen.UniversityBrief, locale string, national bool) bootPage {
	p := bootPage{Lang: locale, University: bootUniversity(u, locale), Kind: "catalog"}
	short := best(u.ShortName, locale)
	if national {
		p.Kicker = bootString("showcase:catalog.eyebrow", locale)
		p.Title, p.Text = bootString("showcase:catalog.titleNational", locale), bootString("showcase:catalog.subtitleNational", locale)
		return p
	}
	p.Kicker = short
	if p.Kicker == "" {
		p.Kicker = bootString("showcase:catalog.eyebrow", locale)
	}
	p.Title = bootString("showcase:catalog.title", locale)
	p.Text = strings.ReplaceAll(bootString("showcase:catalog.subtitle", locale), "{{university}}", p.Kicker)
	return p
}

func projectBoot(e *showcasedb.ShowcaseEntry, u apigen.UniversityBrief, locale string) bootPage {
	p := bootPage{
		Lang: locale, University: bootUniversity(u, locale), Kind: "project",
		Kicker: bootString("showcase:site.showcase", locale), Crumbs: bootString("common:breadcrumbs", locale),
		Title: e.Title, Text: e.Tagline,
		Gradient: coverGradients[industryGradient[e.Industry]],
		Industry: industryLabels[e.Industry].Best(locale), Stage: stageLabels[e.Stage].Best(locale),
	}
	if url := apiconv.FileURL(e.CoverFileID); url != nil {
		p.Cover = *url
	}
	return p
}

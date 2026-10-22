package showcase

import (
	_ "embed"
	"fmt"
	"html/template"
	"net/http"
	"strings"

	"golang.org/x/text/language"

	"github.com/yelnurq/startify/backend/internal/apiconv"
	"github.com/yelnurq/startify/backend/internal/apigen"
	"github.com/yelnurq/startify/backend/internal/platform/apperr"
	"github.com/yelnurq/startify/backend/internal/showcase/showcasedb"
	"github.com/yelnurq/startify/backend/internal/tenant"
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
	"landing:hero.primary":           {"kk": "Жобаны бастау", "ru": "Начать проект", "en": "Start a project"},
	"landing:hero.secondary":         {"kk": "Витринаны көру", "ru": "Смотреть витрину", "en": "Browse the showcase"},
	"landing:route.label":            {"kk": "Жоба маршруты", "ru": "Маршрут проекта", "en": "The route of a project"},
	"landing:route.idea":             {"kk": "Идея", "ru": "Идея", "en": "Idea"},
	"landing:route.incubator":        {"kk": "Инкубатор", "ru": "Инкубатор", "en": "Incubator"},
	"landing:route.evaluation":       {"kk": "AI-бағалау", "ru": "AI-оценка", "en": "AI evaluation"},
	"landing:route.showcase":         {"kk": "Витрина", "ru": "Витрина", "en": "Showcase"},
	"landing:route.demoDay":          {"kk": "Demo Day", "ru": "Demo Day", "en": "Demo Day"},
	"landing:route.market":           {"kk": "Нарық", "ru": "Рынок", "en": "Market"},
	"landing:route.hints.idea":       {"kk": "Мәселе және ол кімге қатысты", "ru": "Проблема и для кого она", "en": "The problem and whose it is"},
	"landing:route.hints.incubator":  {"kk": "AI-ментормен 14 қадам", "ru": "14 шагов с AI-ментором", "en": "14 steps with an AI mentor"},
	"landing:route.hints.evaluation": {"kk": "Университет рубрикасы бойынша балл", "ru": "Балл по рубрике вуза", "en": "A score by the university rubric"},
	"landing:route.hints.showcase":   {"kk": "Серіктестерге арналған бет", "ru": "Страница для партнёров", "en": "A page for partners"},
	"landing:route.hints.demoDay":    {"kk": "Питч және дауыс беру", "ru": "Питч и голосование", "en": "A pitch and a live vote"},
	"landing:route.hints.market":     {"kk": "Пилоттар мен инвестициялар", "ru": "Пилоты и инвестиции", "en": "Pilots and investment"},
	"showcase:catalog.eyebrow":       {"kk": "Жобалар витринасы", "ru": "Витрина проектов", "en": "Project showcase"},
	"showcase:catalog.title":         {"kk": "Серіктес іздеп жүрген жобалар", "ru": "Проекты, которые ищут партнёров", "en": "Projects looking for partners"},
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

// routeStops are the milestones of the landing hero.
var routeStops = []string{"idea", "incubator", "evaluation", "showcase", "demoDay", "market"}

// bootStop is a milestone of the route strip.
type bootStop struct {
	Number, Name, Hint string
}

// bootPage is what the first screen shows.
type bootPage struct {
	Lang       string
	University string
	// Logo is the university's seal; without one the Qadam mark is drawn.
	Logo string
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
	// Landing hero: the actions and the route strip.
	Primary    string
	Secondary  string
	RouteLabel string
	Route      []bootStop
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

func bootLogo(u apigen.UniversityBrief) string {
	if u.LogoUrl == nil {
		return ""
	}
	return *u.LogoUrl
}

func landingBoot(u tenant.University, locale string) bootPage {
	p := bootPage{
		Lang: locale, University: bootUniversity(u.Brief, locale), Logo: bootLogo(u.Brief), Kind: "landing",
		Kicker: bootString("landing:hero.eyebrow", locale),
		Title:  bootString("landing:hero.titleLead", locale), Accent: bootString("landing:hero.titleAccent", locale),
		Text:    bootString("landing:hero.subtitle", locale),
		Primary: bootString("landing:hero.primary", locale), Secondary: bootString("landing:hero.secondary", locale),
		RouteLabel: bootString("landing:route.label", locale),
	}
	if u.CoverURL != nil {
		p.Cover = *u.CoverURL
	}
	for i, stop := range routeStops {
		p.Route = append(p.Route, bootStop{
			Number: fmt.Sprintf("%02d", i+1),
			Name:   bootString("landing:route."+stop, locale), Hint: bootString("landing:route.hints."+stop, locale),
		})
	}
	return p
}

func catalogBoot(u apigen.UniversityBrief, locale string, national bool) bootPage {
	p := bootPage{Lang: locale, University: bootUniversity(u, locale), Logo: bootLogo(u), Kind: "catalog"}
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
		Lang: locale, University: bootUniversity(u, locale), Logo: bootLogo(u), Kind: "project",
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

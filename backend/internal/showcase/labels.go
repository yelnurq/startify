package showcase

import "github.com/yelnurq/startify/backend/internal/apiconv"

// Texts of search engine pages and link previews; they match the web
// client's showcase dictionary (meta, stages, industries, levels).
var (
	catalogTitle = apiconv.Text{
		"kk": "Жобалар витринасы — {university}", "ru": "Витрина проектов — {university}", "en": "Project showcase — {university}",
	}
	catalogDescription = apiconv.Text{
		"kk": "{university} студенттік стартаптары мен инновациялық жобалары: сала, кезең, факультет және команда іздейтін қолдау бойынша іздеу.",
		"ru": "Студенческие стартапы и инновационные проекты {university}: поиск по отрасли, стадии, факультету и тому, что нужно команде.",
		"en": "Student startups and innovation projects of {university}: search by industry, stage, faculty and what the team needs.",
	}
	nationalTitle = apiconv.Text{
		"kk": "Қазақстан университеттерінің жобалар витринасы", "ru": "Витрина проектов университетов Казахстана",
		"en": "Project showcase of Kazakhstan's universities",
	}
	nationalDescription = apiconv.Text{
		"kk": "Qadam платформасындағы Қазақстан университеттерінің ашық студенттік жобалары.",
		"ru": "Публичные студенческие проекты университетов Казахстана на платформе Qadam.",
		"en": "Public student projects of Kazakhstan's universities on the Qadam platform.",
	}
	projectTitle = apiconv.Text{
		"kk": "{title} — {university} витринасы", "ru": "{title} — витрина {university}", "en": "{title} — {university} showcase",
	}
	scoreLabel    = apiconv.Text{"kk": "Дайындық балы", "ru": "Балл готовности", "en": "Readiness score"}
	problemLabel  = apiconv.Text{"kk": "Мәселе", "ru": "Проблема", "en": "Problem"}
	solutionLabel = apiconv.Text{"kk": "Шешім", "ru": "Решение", "en": "Solution"}
	marketLabel   = apiconv.Text{"kk": "Нарық", "ru": "Рынок", "en": "Market"}
	tractionLabel = apiconv.Text{"kk": "Трекшн", "ru": "Трекшн", "en": "Traction"}
	askLabel      = apiconv.Text{"kk": "Жобаға не қажет", "ru": "Что нужно проекту", "en": "What the project needs"}
)

var stageLabels = map[string]apiconv.Text{
	"idea":    {"kk": "Идея", "ru": "Идея", "en": "Idea"},
	"market":  {"kk": "Нарық", "ru": "Рынок", "en": "Market"},
	"product": {"kk": "Өнім", "ru": "Продукт", "en": "Product"},
	"launch":  {"kk": "Іске қосу", "ru": "Запуск", "en": "Launch"},
}

var levelLabels = map[string]apiconv.Text{
	"idea":              {"kk": "Идея", "ru": "Идея", "en": "Idea"},
	"validated_problem": {"kk": "Мәселе расталған", "ru": "Проблема подтверждена", "en": "Validated problem"},
	"mvp_ready":         {"kk": "MVP-ге дайын", "ru": "Готов к MVP", "en": "MVP-ready"},
	"investment_ready":  {"kk": "Инвестицияға дайын", "ru": "Готов к инвестициям", "en": "Investment-ready"},
}

var industryLabels = map[string]apiconv.Text{
	"agritech":      {"kk": "Агротех", "ru": "Агротех", "en": "Agritech"},
	"edtech":        {"kk": "Білім беру", "ru": "Образование", "en": "Education"},
	"fintech":       {"kk": "Финтех", "ru": "Финтех", "en": "Fintech"},
	"healthtech":    {"kk": "Медицина және денсаулық", "ru": "Медицина и здоровье", "en": "Health and medicine"},
	"logistics":     {"kk": "Логистика", "ru": "Логистика", "en": "Logistics"},
	"cleantech":     {"kk": "Экология және таза технологиялар", "ru": "Экология и чистые технологии", "en": "Ecology and clean tech"},
	"govtech":       {"kk": "Мемлекеттік қызметтер және қала", "ru": "Госуслуги и город", "en": "Public services and cities"},
	"foodtech":      {"kk": "Тағам технологиялары", "ru": "Пищевые технологии", "en": "Food technology"},
	"fashion":       {"kk": "Жеңіл өнеркәсіп және сән", "ru": "Лёгкая промышленность и мода", "en": "Light industry and fashion"},
	"ecommerce":     {"kk": "Сауда және e-commerce", "ru": "Торговля и e-commerce", "en": "Retail and e-commerce"},
	"manufacturing": {"kk": "Өндіріс", "ru": "Производство", "en": "Manufacturing"},
	"energy":        {"kk": "Энергетика", "ru": "Энергетика", "en": "Energy"},
	"tourism":       {"kk": "Туризм", "ru": "Туризм", "en": "Tourism"},
	"social":        {"kk": "Әлеуметтік жоба", "ru": "Социальный проект", "en": "Social impact"},
	"other":         {"kk": "Басқа", "ru": "Другое", "en": "Other"},
}

package seed

import (
	"context"
	"fmt"
	"time"

	"github.com/google/uuid"

	"github.com/yelnurq/startify/backend/internal/apiconv"
	"github.com/yelnurq/startify/backend/internal/auth"
	"github.com/yelnurq/startify/backend/internal/platform/db"
	"github.com/yelnurq/startify/backend/internal/platform/jobs"
)

// DemoPassword is the password of every demo account. Demo data is for
// development and presentations, never for production.
const DemoPassword = "Qadam-Demo-2026!"

// ConsentVersion is the version of the demo consent text.
const ConsentVersion = "2026-09"

// KazUTB is the first customer of the platform.
var KazUTB = TenantSpec{
	Slug: "kazutb",
	Name: apiconv.Text{
		"kk": "Қ. Құлажанов атындағы Қазақ технология және бизнес университеті",
		"ru": "Казахский университет технологии и бизнеса имени К. Кулажанова",
		"en": "Kazakh University of Technology and Business named after K. Kulazhanov",
	},
	ShortName:       apiconv.Text{"kk": "ҚазТБУ", "ru": "КазУТБ", "en": "KazUTB"},
	Locale:          "kk",
	Color:           "#067A75",
	SelfSignup:      true,
	IPPolicyVersion: "2026-09",
	IPPolicyText: apiconv.Text{
		"kk": "Жоба нәтижелеріне құқық оның авторларына — команда мүшелеріне тиесілі. Егер жоба университет грантын алса немесе оның жабдығын оқу сабақтарынан тыс пайдаланса, грант төленгенге дейін тараптар құқықтар туралы келісімге қол қояды; әдепкі бойынша университет оқу және ғылыми мақсаттар үшін өтеусіз ерекше емес лицензия алады. Жоба материалдарының қайсысы платформаның басқа пайдаланушыларына көрінетінін команда өзі шешеді. Менторлар мен қазылар жоба материалдарын оны сүйемелдеу мен бағалаудан тыс пайдаланбауға міндеттенеді.",
		"ru": "Права на результаты проекта принадлежат его авторам — участникам команды. Если проект получает грант университета или использует его оборудование сверх учебных занятий, до выплаты гранта стороны подписывают соглашение о правах; по умолчанию университет получает безвозмездную неисключительную лицензию для учебных и научных целей. Команда сама решает, какие материалы проекта видны другим пользователям платформы. Менторы и жюри обязуются не использовать материалы проекта вне его сопровождения и оценки.",
		"en": "The rights to a project's results belong to its authors, the members of the team. When a project receives a university grant or uses university equipment beyond regular classes, the parties sign an agreement on rights before the grant is paid; by default the university receives a free non-exclusive licence for teaching and research. The team decides which project materials other users of the platform can see. Mentors and jury members undertake not to use project materials outside mentoring and evaluating the project.",
	},
	ConsentVersion: ConsentVersion,
	ConsentText: apiconv.Text{
		"kk": "Мен Қ. Құлажанов атындағы ҚазТБУ-ға «Дербес деректер және оларды қорғау туралы» Қазақстан Республикасының Заңына сәйкес Qadam платформасында университеттің инновациялық және жобалық қызметіне қатысу мақсатында дербес деректерімді (аты-жөні, электрондық пошта, оқу туралы мәліметтер, жоба материалдары) жинауға және өңдеуге келісім беремін.",
		"ru": "Я даю согласие КазУТБ им. К. Кулажанова на сбор и обработку моих персональных данных (ФИО, адрес электронной почты, сведения об обучении, материалы проектов) в целях участия в инновационной и проектной деятельности университета на платформе Qadam в соответствии с Законом Республики Казахстан «О персональных данных и их защите».",
		"en": "I consent to KazUTB named after K. Kulazhanov collecting and processing my personal data (full name, e-mail address, study details, project materials) for taking part in the university's innovation and project activities on the Qadam platform, in accordance with the Law of the Republic of Kazakhstan \"On Personal Data and Their Protection\".",
	},
}

// Sandbox is a second university used to check tenant isolation.
var Sandbox = TenantSpec{
	Slug:           "sandbox",
	Name:           apiconv.Text{"kk": "Qadam сынақ университеті", "ru": "Тестовый университет Qadam", "en": "Qadam Sandbox University"},
	ShortName:      apiconv.Text{"kk": "Сынақ", "ru": "Песочница", "en": "Sandbox"},
	Locale:         "ru",
	Color:          "#7C3AED",
	SelfSignup:     true,
	ConsentVersion: ConsentVersion,
	ConsentText:    KazUTB.ConsentText,
}

func bachelor(code string, year int16) Group { return Group{Code: code, Level: "bachelor", Year: year} }

// KazUTBStructure is the demo structure of KazUTB.
var KazUTBStructure = []Faculty{
	{
		Code: "FIT",
		Name: apiconv.Text{"kk": "Ақпараттық технологиялар факультеті", "ru": "Факультет информационных технологий", "en": "Faculty of Information Technology"},
		Departments: []Department{
			{
				Code: "IS", Name: apiconv.Text{"kk": "Ақпараттық жүйелер кафедрасы", "ru": "Кафедра информационных систем", "en": "Department of Information Systems"},
				Groups: []Group{bachelor("ИС-22-1", 2022), bachelor("ИС-23-1", 2023), bachelor("ИС-24-1", 2024), {Code: "М-ИС-25-1", Level: "master", Year: 2025}},
			},
			{
				Code: "CS", Name: apiconv.Text{"kk": "Есептеуіш техника және бағдарламалық қамтамасыз ету кафедрасы", "ru": "Кафедра вычислительной техники и программного обеспечения", "en": "Department of Computer Engineering and Software"},
				Groups: []Group{bachelor("ВТ-22-1", 2022), bachelor("ВТ-23-1", 2023), bachelor("ВТ-24-2", 2024)},
			},
			{
				Code: "AU", Name: apiconv.Text{"kk": "Автоматтандыру және басқару кафедрасы", "ru": "Кафедра автоматизации и управления", "en": "Department of Automation and Control"},
				Groups: []Group{bachelor("АУ-23-1", 2023), bachelor("АУ-24-1", 2024)},
			},
		},
	},
	{
		Code: "FTFS",
		Name: apiconv.Text{"kk": "Тағам технологиясы және қауіпсіздігі факультеті", "ru": "Факультет пищевых технологий и безопасности", "en": "Faculty of Food Technology and Safety"},
		Departments: []Department{
			{
				Code: "FT", Name: apiconv.Text{"kk": "Тағам өнімдерінің технологиясы кафедрасы", "ru": "Кафедра технологии пищевых продуктов", "en": "Department of Food Technology"},
				Groups: []Group{bachelor("ТПП-22-1", 2022), bachelor("ТПП-23-1", 2023)},
			},
			{
				Code: "CTE", Name: apiconv.Text{"kk": "Химия, химиялық технология және экология кафедрасы", "ru": "Кафедра химии, химической технологии и экологии", "en": "Department of Chemistry, Chemical Technology and Ecology"},
				Groups: []Group{bachelor("ХТ-23-1", 2023), bachelor("ЭК-24-1", 2024)},
			},
		},
	},
	{
		Code: "FDLI",
		Name: apiconv.Text{"kk": "Дизайн және жеңіл өнеркәсіп технологиялары факультеті", "ru": "Факультет дизайна и технологий лёгкой промышленности", "en": "Faculty of Design and Light Industry Technology"},
		Departments: []Department{
			{
				Code: "DES", Name: apiconv.Text{"kk": "Дизайн кафедрасы", "ru": "Кафедра дизайна", "en": "Department of Design"},
				Groups: []Group{bachelor("ДИЗ-23-1", 2023), bachelor("ДИЗ-24-1", 2024)},
			},
			{
				Code: "LIT", Name: apiconv.Text{"kk": "Жеңіл өнеркәсіп технологиясы кафедрасы", "ru": "Кафедра технологии лёгкой промышленности", "en": "Department of Light Industry Technology"},
				Groups: []Group{bachelor("ТЛП-24-1", 2024)},
			},
		},
	},
	{
		Code: "FEB",
		Name: apiconv.Text{"kk": "Экономика және бизнес факультеті", "ru": "Факультет экономики и бизнеса", "en": "Faculty of Economics and Business"},
		Departments: []Department{
			{
				Code: "FA", Name: apiconv.Text{"kk": "Қаржы және есеп кафедрасы", "ru": "Кафедра финансов и учёта", "en": "Department of Finance and Accounting"},
				Groups: []Group{bachelor("ФУ-22-1", 2022), bachelor("ФУ-24-1", 2024)},
			},
			{
				Code: "MM", Name: apiconv.Text{"kk": "Менеджмент және маркетинг кафедрасы", "ru": "Кафедра менеджмента и маркетинга", "en": "Department of Management and Marketing"},
				Groups: []Group{bachelor("МЕН-23-1", 2023), bachelor("МРК-24-1", 2024)},
			},
			{
				Code: "TS", Name: apiconv.Text{"kk": "Туризм және сервис кафедрасы", "ru": "Кафедра туризма и сервиса", "en": "Department of Tourism and Service"},
				Groups: []Group{bachelor("ТУР-23-1", 2023)},
			},
		},
	},
}

// KazUTBMembers are the demo accounts of KazUTB.
var KazUTBMembers = []Member{
	{
		Email: "admin@qadam.test", FullName: "Ерлан Сейтқазиев", Locale: "ru", Roles: []string{"tenant_admin", "innovation_manager"},
		Slug: "erlan-seitkaziev", Headline: "Офис коммерциализации КазУТБ",
	},
	{
		Email: "aigerim@qadam.test", FullName: "Айгерім Сәрсенова", Locale: "kk", Roles: []string{"student"}, Group: "ИС-23-1", Course: 3,
		Slug: "aigerim-sarsenova", Public: true, LookTeam: true,
		Headline:  "Backend-әзірлеуші, кампусқа арналған сервистер жасаймын",
		Bio:       "Ақпараттық жүйелер мамандығының 3-курс студентімін. Go және PostgreSQL-мен жұмыс істеймін, университет өмірін ыңғайлы ететін өнімдерге қызығамын.",
		Skills:    []string{"Go", "PostgreSQL", "Docker", "REST API"},
		Interests: []string{"EdTech", "Smart Campus", "Open Source"},
	},
	{
		Email: "nurlan@qadam.test", FullName: "Нұрлан Жұмабеков", Locale: "ru", Roles: []string{"student"}, Group: "ВТ-22-1", Course: 4,
		Slug: "nurlan-zhumabekov", Public: true, LookTeam: true,
		Headline:  "Компьютерное зрение для сельского хозяйства",
		Bio:       "Делаю AgroScan — определение болезней пшеницы по фото со смартфона. Ищу команду, чтобы довести прототип до пилота с фермерами Акмолинской области.",
		Skills:    []string{"Python", "PyTorch", "OpenCV", "FastAPI"},
		Interests: []string{"AgroTech", "Computer Vision"},
	},
	{
		Email: "dana@qadam.test", FullName: "Дана Ахметова", Locale: "ru", Roles: []string{"student"}, Group: "ТПП-23-1", Course: 3,
		Slug: "dana-akhmetova", Public: true,
		Headline:  "Технолог пищевых производств, интересуюсь устойчивой упаковкой",
		Skills:    []string{"Пищевая химия", "HACCP", "Figma"},
		Interests: []string{"FoodTech", "Экология"},
	},
	{
		Email: "timur@qadam.test", FullName: "Тимур Ким", Locale: "en", Roles: []string{"student"}, Group: "М-ИС-25-1", Course: 1,
		Slug: "timur-kim", Public: true, LookTeam: true,
		Headline:  "Data scientist, NLP for the Kazakh language",
		Skills:    []string{"Python", "NLP", "Transformers", "SQL"},
		Interests: []string{"AI", "GovTech", "Language technology"},
	},
	{
		Email: "madina@qadam.test", FullName: "Мадина Оспанова", Locale: "kk", Roles: []string{"student"}, Group: "МРК-24-1", Course: 2,
		Slug:      "madina-ospanova",
		Skills:    []string{"SMM", "Маркетингтік зерттеу", "Canva"},
		Interests: []string{"Startups", "Marketing"},
	},
	{
		Email: "gulnar@qadam.test", FullName: "Гүлнар Қасымова", Locale: "ru", Roles: []string{"supervisor", "mentor"},
		Slug: "gulnar-kassymova", Public: true,
		Headline: "PhD, доцент кафедры информационных систем",
		Bio:      "Руковожу студенческими проектами в области анализа данных и цифровизации университета. Помогаю превратить курсовую работу в продукт.",
	},
	{
		Email: "askhat@qadam.test", FullName: "Асхат Нұрғалиев", Locale: "ru", Roles: []string{"jury", "partner"},
		Slug: "askhat-nurgaliyev", Headline: "Венчурный аналитик, член жюри Demo Day",
	},
}

// KazUTBRequests are demo requests on the teammates board.
var KazUTBRequests = []TeamRequest{
	{
		Author: "nurlan@qadam.test", Title: "AgroScan: нужен дизайнер и специалист по данным",
		Description: "Прототип уже распознаёт три болезни пшеницы с точностью 87% на нашем датасете. Нужен дизайнер мобильного интерфейса для фермеров и человек, который поможет собрать и разметить данные с полей. Весной планируем пилот в двух хозяйствах.",
		Roles:       []string{"designer", "data_scientist"}, Skills: []string{"Figma", "Python", "Разметка данных"},
	},
	{
		Author: "aigerim@qadam.test", Title: "Кампустағы спорт залдарын брондау сервисі — фронтенд-әзірлеуші іздейміз",
		Description: "Студенттер мен оқытушылар спорт залдары мен коворкингтерді онлайн брондай алатын сервис жасап жатырмыз. Backend дайын (Go, PostgreSQL), енді React-пен ыңғайлы интерфейс жасайтын адам керек.",
		Roles:       []string{"developer"}, Skills: []string{"React", "TypeScript"},
	},
	{
		Author: "dana@qadam.test", Title: "Съедобная упаковка из отходов переработки яблок",
		Description: "Разрабатываем плёнку для упаковки снеков из яблочного жмыха — отхода местных соковых производств. Нужны технолог-химик для подбора рецептуры и человек, который посчитает экономику и найдёт первых покупателей.",
		Roles:       []string{"domain_expert", "business"}, Skills: []string{"Химическая технология", "Финансовая модель"},
	},
	{
		Author: "timur@qadam.test", Title: "Kazakh-language chatbot for the admissions office",
		Description: "We are building an assistant that answers applicants' questions in Kazakh and Russian using the university's official documents. Looking for a backend developer and someone to run the summer campaign with the admissions office.",
		Roles:       []string{"developer", "marketer"}, Skills: []string{"Python", "Telegram Bot API", "SMM"},
	},
}

// GeneratedStudents is the size of the generated student body, next to the
// named demo students.
const GeneratedStudents = 55

// AllKazUTBMembers are every demo account of KazUTB.
func AllKazUTBMembers() []Member {
	all := append([]Member{}, KazUTBMembers...)
	all = append(all, KazUTBStaff...)
	return append(all, Students(GeneratedStudents)...)
}

// Demo creates the demonstration installation: KazUTB with structure,
// accounts, team requests, knowledge base and student projects at every
// stage of the incubator, and the sandbox university.
func Demo(ctx context.Context, database *db.DB) error {
	kazutb, err := Tenant(ctx, database, KazUTB)
	if err != nil {
		return err
	}
	if err := Structure(ctx, database, kazutb.ID, KazUTBStructure); err != nil {
		return err
	}
	hash, err := auth.HashPassword(DemoPassword)
	if err != nil {
		return err
	}
	authors := map[string]uuid.UUID{}
	members := AllKazUTBMembers()
	for i := range members {
		m := members[i]
		m.PasswordHash = hash
		id, err := Account(ctx, database, kazutb.ID, m)
		if err != nil {
			return err
		}
		authors[m.Email] = id
	}
	if err := TeamRequests(ctx, database, kazutb.ID, authors, KazUTBRequests); err != nil {
		return err
	}
	queue, err := jobs.NewQueue(database)
	if err != nil {
		return err
	}
	if err := KnowledgeDocuments(ctx, database, queue, kazutb.ID, KazUTBKnowledge); err != nil {
		return err
	}
	if err := Projects(ctx, database, kazutb.ID, authors, KazUTBProjects, time.Now()); err != nil {
		return err
	}
	if err := Evaluations(ctx, database, kazutb.ID, authors, KazUTBProjects, time.Now()); err != nil {
		return err
	}
	if err := Showcase(ctx, database, kazutb.ID, KazUTBProjects, KazUTBShowcase, time.Now()); err != nil {
		return err
	}

	sandbox, err := Tenant(ctx, database, Sandbox)
	if err != nil {
		return err
	}
	if _, err := Account(ctx, database, sandbox.ID, Member{
		Email: "sandbox-admin@qadam.test", FullName: "Sandbox Admin", Locale: "en", PasswordHash: hash,
		Roles: []string{"tenant_admin"}, Slug: "sandbox-admin",
	}); err != nil {
		return fmt.Errorf("seed: sandbox: %w", err)
	}
	return nil
}

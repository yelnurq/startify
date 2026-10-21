package seed

import (
	"fmt"
	"strings"

	"github.com/yelnurq/startify/backend/internal/user"
)

// Names are combined deterministically into the demo student body: the same
// seed always produces the same people.
var (
	femaleNames = []string{
		"Аружан", "Әсел", "Балжан", "Гүлназ", "Жансая", "Зарина", "Камила", "Меруерт", "Назерке", "Сабина",
		"Томирис", "Ұлжан", "Айым", "Інжу", "Ақбота", "Нұрай", "Аяна", "Дильназ", "Анель", "Мөлдір",
	}
	maleNames = []string{
		"Әлихан", "Арман", "Бауыржан", "Дәурен", "Ерасыл", "Жандос", "Ильяс", "Қуаныш", "Мирас", "Нұрсұлтан",
		"Олжас", "Санжар", "Темірлан", "Ұлан", "Шыңғыс", "Диас", "Абылай", "Ернар", "Рауан", "Максим",
	}
	// surnames holds the male and female forms.
	surnames = [][2]string{
		{"Ахметов", "Ахметова"},
		{"Серікбаев", "Серікбаева"},
		{"Төлеуов", "Төлеуова"},
		{"Мұратов", "Мұратова"},
		{"Ибраев", "Ибраева"},
		{"Әбдірахманов", "Әбдірахманова"},
		{"Бекмағанбетов", "Бекмағанбетова"},
		{"Тұрсынов", "Тұрсынова"},
		{"Есенов", "Есенова"},
		{"Сұлтанов", "Сұлтанова"},
		{"Кенжебаев", "Кенжебаева"},
		{"Байжанов", "Байжанова"},
		{"Әлімов", "Әлімова"},
		{"Смағұлов", "Смағұлова"},
		{"Ким", "Ким"},
		{"Пак", "Пак"},
		{"Цой", "Цой"},
		{"Иванов", "Иванова"},
		{"Жақсылықов", "Жақсылықова"},
		{"Нұрғалиев", "Нұрғалиева"},
	}

	// groupsByTrack lists the demo groups with their course in 2026/27.
	studentGroups = []struct {
		code   string
		course int16
		track  string
	}{
		{"ИС-22-1", 4, "it"},
		{"ИС-23-1", 3, "it"},
		{"ИС-24-1", 2, "it"},
		{"М-ИС-25-1", 1, "it"},
		{"ВТ-22-1", 4, "it"},
		{"ВТ-23-1", 3, "it"},
		{"ВТ-24-2", 2, "it"},
		{"АУ-23-1", 3, "it"},
		{"АУ-24-1", 2, "it"},
		{"ТПП-22-1", 4, "food"},
		{"ТПП-23-1", 3, "food"},
		{"ХТ-23-1", 3, "food"},
		{"ЭК-24-1", 2, "food"},
		{"ФУ-22-1", 4, "biz"},
		{"ФУ-24-1", 2, "biz"},
		{"МЕН-23-1", 3, "biz"},
		{"МРК-24-1", 2, "biz"},
		{"ТУР-23-1", 3, "biz"},
		{"ДИЗ-23-1", 3, "design"},
		{"ДИЗ-24-1", 2, "design"},
		{"ТЛП-24-1", 2, "design"},
	}

	trackSkills = map[string][]string{
		"it":     {"Python", "Go", "React", "TypeScript", "PostgreSQL", "Machine Learning", "Flutter", "Docker", "Компьютерное зрение", "UX-исследования"},
		"food":   {"Пищевая химия", "HACCP", "Микробиология", "Технология хлебопечения", "Упаковка", "Контроль качества", "Excel"},
		"biz":    {"Финансовое моделирование", "Маркетинг", "SMM", "Продажи", "Customer Development", "Бухучёт", "Power BI"},
		"design": {"Figma", "Adobe Illustrator", "3D-моделирование", "Конструирование одежды", "Брендинг", "Blender"},
	}
	trackInterests = map[string][]string{
		"it":     {"EdTech", "AgroTech", "GovTech", "FinTech", "Искусственный интеллект"},
		"food":   {"FoodTech", "Экология", "AgroTech", "Здоровое питание"},
		"biz":    {"FinTech", "Туризм", "Социальное предпринимательство", "Ритейл"},
		"design": {"Мода", "Устойчивое производство", "Креативные индустрии", "Этнодизайн"},
	}
)

// emailPart transliterates a name for an e-mail address.
func emailPart(s string) string {
	return strings.ReplaceAll(user.Slugify(s), "-", "")
}

// Students returns the demo student body: n students spread over the groups,
// with skills, interests and public profiles for most of them.
func Students(n int) []Member {
	out := make([]Member, 0, n)
	usedEmails := map[string]bool{}
	usedSlugs := map[string]bool{}
	for i := range n {
		female := i%2 == 0
		first := maleNames[(i/2)%len(maleNames)]
		form := 0
		if female {
			first = femaleNames[(i/2)%len(femaleNames)]
			form = 1
		}
		// The surname shifts on every round of first names, so no two students share both.
		last := surnames[(i*7+(i/40)*3+3)%len(surnames)][form]
		name := first + " " + last

		email := emailPart(first) + "." + emailPart(last)
		for k := 2; usedEmails[email]; k++ {
			email = fmt.Sprintf("%s.%s%d", emailPart(first), emailPart(last), k)
		}
		usedEmails[email] = true
		slug := user.Slugify(name)
		for k := 2; usedSlugs[slug]; k++ {
			slug = fmt.Sprintf("%s-%d", user.Slugify(name), k)
		}
		usedSlugs[slug] = true

		g := studentGroups[i%len(studentGroups)]
		skills := trackSkills[g.track]
		interests := trackInterests[g.track]
		m := Member{
			Email:     email + "@qadam.test",
			FullName:  name,
			Locale:    []string{"kk", "ru", "kk", "ru", "en"}[i%5],
			Roles:     []string{"student"},
			Group:     g.code,
			Course:    g.course,
			Slug:      slug,
			Public:    i%3 != 2,
			LookTeam:  i%4 == 1,
			Skills:    []string{skills[i%len(skills)], skills[(i+3)%len(skills)], skills[(i+5)%len(skills)]},
			Interests: []string{interests[i%len(interests)], interests[(i+2)%len(interests)]},
		}
		if m.Skills[0] == m.Skills[2] {
			m.Skills = m.Skills[:2]
		}
		out = append(out, m)
	}
	return out
}

// KazUTBStaff are the mentors, jury, managers and leadership of KazUTB.
var KazUTBStaff = []Member{
	{
		Email: "innovation@qadam.test", FullName: "Динара Әбенова", Locale: "ru", Roles: []string{"innovation_manager"},
		Slug: "dinara-abenova", Headline: "Руководитель центра инноваций и коммерциализации",
	},
	{
		Email: "rector@qadam.test", FullName: "Серік Нұрмағанбетов", Locale: "kk", Roles: []string{"leadership"},
		Slug: "serik-nurmaganbetov", Headline: "Проректор по науке и инновациям",
	},
	{
		Email: "bakhyt.mentor@qadam.test", FullName: "Бахыт Оразалиев", Locale: "ru", Roles: []string{"mentor"},
		Slug: "bakhyt-orazaliyev", Public: true, Headline: "Сооснователь агротех-стартапа, ментор акселераторов",
		Skills: []string{"AgroTech", "Fundraising", "Go-to-market"},
	},
	{
		Email: "irina.mentor@qadam.test", FullName: "Ирина Пак", Locale: "ru", Roles: []string{"mentor"},
		Slug: "irina-pak", Public: true, Headline: "Product-менеджер в финтехе, 9 лет в продуктах",
		Skills: []string{"Product Management", "FinTech", "Customer Development"},
	},
	{
		Email: "yerlan.mentor@qadam.test", FullName: "Ерлан Құдайбергенов", Locale: "kk", Roles: []string{"mentor", "supervisor"},
		Slug: "yerlan-kudaibergenov", Public: true, Headline: "PhD, тағам технологиясы кафедрасының доценті",
		Skills: []string{"FoodTech", "HACCP", "Ғылыми жобалар"},
	},
	{
		Email: "aliya.mentor@qadam.test", FullName: "Әлия Серғазина", Locale: "ru", Roles: []string{"mentor"},
		Slug: "aliya-sergazina", Public: true, Headline: "Дизайн-директор брендингового агентства",
		Skills: []string{"Брендинг", "UX", "Этнодизайн"},
	},
	{
		Email: "timur.mentor@qadam.test", FullName: "Тимур Ахметжанов", Locale: "en", Roles: []string{"mentor"},
		Slug: "timur-akhmetzhanov", Public: true, Headline: "ML engineer, builds language models for Kazakh",
		Skills: []string{"Machine Learning", "NLP", "MLOps"},
	},
	{
		Email: "saule.mentor@qadam.test", FullName: "Сәуле Байтұрсынова", Locale: "kk", Roles: []string{"mentor", "supervisor"},
		Slug: "saule-baitursynova", Public: true, Headline: "Қаржы және есеп кафедрасының меңгерушісі",
		Skills: []string{"Қаржылық модельдеу", "Бизнес-жоспар"},
	},
	{
		Email: "oleg.mentor@qadam.test", FullName: "Олег Цой", Locale: "ru", Roles: []string{"mentor"},
		Slug: "oleg-tsoy", Public: true, Headline: "Юрист по интеллектуальной собственности",
		Skills: []string{"Патенты", "Лицензирование", "IP-стратегия"},
	},
	{
		Email: "jury.venture@qadam.test", FullName: "Марат Жүнісов", Locale: "ru", Roles: []string{"jury", "partner"},
		Slug: "marat-zhunisov", Headline: "Партнёр венчурного фонда",
	},
	{
		Email: "jury.akimat@qadam.test", FullName: "Гаухар Әлиева", Locale: "kk", Roles: []string{"jury"},
		Slug: "gaukhar-aliyeva", Headline: "Цифрландыру басқармасының бас маманы",
	},
	{
		Email: "jury.industry@qadam.test", FullName: "Виктор Ли", Locale: "ru", Roles: []string{"jury", "partner"},
		Slug: "viktor-li", Headline: "Технический директор пищевого холдинга",
	},
	{
		Email: "jury.techpark@qadam.test", FullName: "Ләззат Мұқанова", Locale: "ru", Roles: []string{"jury"},
		Slug: "lazzat-mukanova", Headline: "Эксперт технопарка, оценивает 100+ проектов в год",
	},
	{
		Email: "jury.alumni@qadam.test", FullName: "Нұржан Төлегенов", Locale: "kk", Roles: []string{"jury"},
		Slug: "nurzhan-tolegenov", Headline: "Түлек, EdTech стартапының негізін қалаушы",
	},
}

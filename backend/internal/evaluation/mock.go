package evaluation

import (
	"encoding/json"
	"fmt"
	"hash/fnv"
	"math"
	"slices"
	"strings"
	"unicode/utf8"

	"github.com/yelnurq/startify/backend/internal/ai/llm"
	"github.com/yelnurq/startify/backend/internal/ai/mock"
	"github.com/yelnurq/startify/backend/internal/incubator"
)

// RegisterMock teaches the mock provider to evaluate: scores follow how
// completely and concretely the project answers each criterion's steps, so
// the demo and the evaluation harness behave like a strict reviewer without
// a model.
func RegisterMock() {
	mock.Register(llm.FeatureEvaluation, func(req llm.CompletionRequest) string {
		if strings.HasPrefix(req.Metadata.PromptVersion, "evaluation@") {
			return mockEvaluate(req)
		}
		if req.JSONSchema == nil {
			return mock.Reply(req)
		}
		h := fnv.New64a()
		_, _ = h.Write([]byte(req.System))
		raw, _ := json.Marshal(mock.Fake(req.JSONSchema.JSON, h.Sum64()))
		return string(raw)
	})
}

// projectData is the JSON the evaluation prompt carries.
type projectData struct {
	Project struct {
		Title    string `json:"title"`
		Industry string `json:"industry"`
	} `json:"project"`
	Steps map[string]incubator.Values `json:"steps"`
}

func lastBlock(text, tag string) string {
	i := strings.LastIndex(text, "<"+tag+">")
	if i < 0 {
		return ""
	}
	inner, _, _ := strings.Cut(text[i+len(tag)+2:], "</"+tag+">")
	return strings.TrimSpace(inner)
}

func (d projectData) str(step, field string) string {
	s, _ := d.Steps[step][field].(string)
	return strings.TrimSpace(s)
}

func (d projectData) num(step, field string) (float64, bool) {
	v, ok := d.Steps[step][field].(float64)
	return v, ok
}

func (d projectData) rows(step, field string) []map[string]any {
	items, _ := d.Steps[step][field].([]any)
	var out []map[string]any
	for _, it := range items {
		if m, ok := it.(map[string]any); ok {
			out = append(out, m)
		}
	}
	return out
}

func (d projectData) grade(step string) float64 {
	st, ok := incubator.Default().Step(step)
	if !ok {
		return 0
	}
	return st.Grade(d.Steps[step])
}

func long(s string, n int) bool { return utf8.RuneCountInString(s) >= n }

func clamp(v float64) int { return int(math.Max(0, math.Min(10, math.Round(v)))) }

// priority sectors of Kazakhstan's development programmes.
var priority = []string{"agritech", "edtech", "healthtech", "cleantech", "energy", "govtech", "social", "foodtech", "logistics"}

// ltvCAC is the lifetime value over acquisition cost, when computable.
func (d projectData) ltvCAC() (float64, bool) {
	price, ok1 := d.num("unit_economics", "price")
	cost, ok2 := d.num("unit_economics", "variable_cost")
	cac, ok3 := d.num("unit_economics", "cac")
	n, ok4 := d.num("unit_economics", "purchases")
	if !ok1 || !ok2 || !ok3 || !ok4 || cac <= 0 {
		return 0, false
	}
	return (price - cost) * n / cac, true
}

// scores grades every criterion 0–10.
func (d projectData) scores() map[string]int {
	s := map[string]int{}
	evidence := long(d.str("problem", "evidence"), 80)
	bonus := func(cond bool, v float64) float64 {
		if cond {
			return v
		}
		return 0
	}

	s["problem"] = clamp((2*d.grade("problem")+d.grade("audience")+d.grade("journey"))/4*8 + bonus(evidence, 2))

	tam, okT := d.num("market_size", "tam")
	sam, okS := d.num("market_size", "sam")
	som, okO := d.num("market_size", "som")
	market := (d.grade("market_size") + d.grade("competitors")) / 2 * 8
	if okT && okS && okO {
		if tam >= sam && sam >= som && som > 0 {
			market++
		} else {
			market -= 2
		}
	}
	sources, _ := d.Steps["market_size"]["sources"].([]any)
	s["market"] = clamp(market + bonus(len(sources) >= 2, 1))

	s["solution"] = clamp((2*d.grade("solution")+d.grade("mvp"))/3*8 +
		bonus(long(d.str("solution", "novelty"), 120), 1) + bonus(d.str("solution", "unfair_advantage") != "", 1))

	business := (d.grade("business_model") + d.grade("unit_economics")) / 2 * 7
	if r, ok := d.ltvCAC(); ok {
		switch {
		case r >= 3:
			business += 3
		case r >= 1:
			business++
		default:
			business--
		}
	}
	s["business_model"] = clamp(business)

	members := d.rows("team", "members")
	fullTime := 0
	for _, m := range members {
		if m["commitment"] == "full_time" {
			fullTime++
		}
	}
	s["team"] = clamp(d.grade("team")*7 + bonus(len(members) >= 3, 2) + bonus(len(members) == 2, 1) + bonus(fullTime >= 2, 1))

	confirmed := 0
	for _, h := range d.rows("risks", "hypotheses") {
		if h["status"] == "confirmed" {
			confirmed++
		}
	}
	current := 0
	for _, m := range d.rows("metrics", "metrics") {
		if v, _ := m["current"].(string); strings.TrimSpace(v) != "" {
			current++
		}
	}
	stage := map[string]float64{"prototype": 1, "mvp": 2, "launched": 3}[d.str("mvp", "status")]
	s["validation"] = clamp(bonus(evidence, 2) + bonus(long(d.str("metrics", "traction"), 40), 2) +
		math.Min(float64(confirmed), 3) + stage + bonus(current >= 2, 1))

	mitigated := 0
	for _, r := range d.rows("risks", "risks") {
		if v, _ := r["mitigation"].(string); strings.TrimSpace(v) != "" {
			mitigated++
		}
	}
	s["feasibility"] = clamp((d.grade("mvp")+d.grade("roadmap")+d.grade("risks"))/3*8 +
		bonus(len(d.rows("roadmap", "milestones")) >= 4, 1) + bonus(mitigated >= 3, 1))

	impact := 2.0
	if slices.Contains(priority, d.Project.Industry) {
		impact = 4
	}
	if len(d.Steps) == 0 {
		impact = 0
	}
	s["impact"] = clamp(impact + d.grade("solution")*3 + d.grade("problem")*2 + bonus(d.str("roadmap", "support") != "", 1))
	return s
}

func band(score int) int {
	switch {
	case score >= 7:
		return 2
	case score >= 4:
		return 1
	}
	return 0
}

// justifications[key][locale] = {low, mid, high}.
var justifications = map[string]map[string][3]string{
	"problem": {
		"kk": {"Мәселе жалпы сөздермен тұжырымдалған немесе сипатталмаған: оны кім сезінетіні және оның құны қанша екені белгісіз. Проблемалық сұхбаттардан бастаңыз.", "Мәселе түсінікті, бірақ оның құны мен дәлелдері әлсіз сипатталған. Сұхбаттардан алынған сандарды қосып, фактілерді болжамдардан ажыратыңыз.", "Мәселе нақты сипатталған: ол кімде, қандай жағдайда туындайтыны және оның құны түсінікті. Дәлелдер бар — бұл жобаның күшті жағы."},
		"ru": {"Проблема сформулирована общими словами или не описана: неясно, кто её испытывает и сколько она стоит. Начните с проблемных интервью.", "Проблема понятна, но её цена и подтверждения описаны слабо. Добавьте цифры из интервью и отделите факты от допущений.", "Проблема описана конкретно: понятно, у кого она возникает, в какой ситуации и чего стоит. Есть подтверждения — это сильная сторона проекта."},
		"en": {"The problem is stated in general terms or missing: it is unclear who has it and what it costs. Start with problem interviews.", "The problem is clear, but its cost and evidence are thin. Add numbers from interviews and separate facts from assumptions.", "The problem is specific: it is clear who has it, in what situation and what it costs. There is evidence — a strength of the project."},
	},
	"market": {
		"kk": {"Нарық көлемі мен бәсекелестер іс жүзінде сипатталмаған — сұраныс бар ма, кімді ығыстырасыз, белгісіз.", "Нарық бағаланған, бірақ формуласы немесе дереккөздері толық емес, бәсекелестермен салыстыру үстірт.", "Нарық дереккөздерімен төменнен жоғары есептелген, бәсекелестер мен баламалар әділ аталған."},
		"ru": {"Размер рынка и конкуренты почти не описаны — непонятно, есть ли спрос и кого вы вытесняете.", "Оценка рынка есть, но формула или источники неполные, а сравнение с конкурентами поверхностное.", "Рынок посчитан снизу вверх с источниками, конкуренты и альтернативы названы честно."},
		"en": {"Market size and competitors are barely described — it is unclear whether there is demand and whom you replace.", "There is a market estimate, but the formula or sources are incomplete and the competitor comparison is shallow.", "The market is sized bottom-up with sources; competitors and alternatives are named fairly."},
	},
	"solution": {
		"kk": {"Шешім бұлыңғыр сипатталған: пайдаланушы нақты не алатыны және оның үйреншікті тәсілдерден несі жақсы екені түсініксіз.", "Шешім түсінікті, бірақ оның пайдаланушы қиындықтарымен байланысын және баламалардан айырмашылығын айқынырақ көрсету керек.", "Шешім басты қиындықтарға тікелей жауап береді, құндылық ұсынысы анық, жаңалығы нақты."},
		"ru": {"Решение описано размыто: неясно, что именно получит пользователь и чем это лучше привычных способов.", "Решение понятно, но связь с болями пользователей и отличие от альтернатив нужно показать явнее.", "Решение прямо отвечает на главные боли, ценностное предложение сформулировано ясно, новизна конкретна."},
		"en": {"The solution is vague: it is unclear what exactly the user gets and why it beats the usual ways.", "The solution is clear, but its link to user pains and its difference from alternatives need to be shown more explicitly.", "The solution answers the key pains directly, the value proposition is clear and the novelty is concrete."},
	},
	"business_model": {
		"kk": {"Кім және не үшін төлейтіні сипатталмаған, юнит-экономика есептелмеген.", "Табыс табу моделі сипатталған, бірақ юнит-экономика тексерілмеген болжамдарға сүйенеді немесе әзірге сәйкес келмейді.", "Кім және қанша төлейтіні түсінікті; юнит-экономика сәйкес келеді, болжамдар көрсетілген."},
		"ru": {"Не описано, кто и за что платит, юнит-экономика не посчитана.", "Модель монетизации описана, но юнит-экономика опирается на непроверенные допущения или пока не сходится.", "Понятно, кто платит и сколько; юнит-экономика сходится, допущения указаны."},
		"en": {"It is not described who pays for what, and the unit economics are not calculated.", "The monetisation model is described, but the unit economics rely on untested assumptions or do not work yet.", "It is clear who pays and how much; the unit economics work and the assumptions are stated."},
	},
	"team": {
		"kk": {"Жоба бір-екі адамға ғана сүйенеді, негізгі рөлдер жабылмаған.", "Команда бар, бірақ негізгі рөлдер жетіспейді немесе мүшелердің жұмыспен қамтылуы түсініксіз.", "Команда өнімді, технологияны және сатуды қамтиды, мүшелердің тиісті тәжірибесі бар."},
		"ru": {"Проект держится на одном-двух людях, ключевые роли не закрыты.", "Команда есть, но не хватает ключевых ролей или занятость участников неясна.", "Команда закрывает продукт, технологии и продажи, у участников есть релевантный опыт."},
		"en": {"The project rests on one or two people; key roles are not covered.", "There is a team, but key roles are missing or the members' commitment is unclear.", "The team covers product, technology and sales, and members have relevant experience."},
	},
	"validation": {
		"kk": {"Жоба әзірге нақты пайдаланушыларда тексерілмеген — бәрі болжамдарға сүйенеді.", "Тексеруге алғашқы қадамдар жасалған, бірақ расталған болжамдар мен нақты метрикалар әзірге аз.", "Болжамдар пайдаланушыларда тексерілген, пилот немесе жұмыс істейтін MVP және ағымдағы мәндері бар метрикалар бар."},
		"ru": {"Проект пока не проверен на реальных пользователях — всё держится на допущениях.", "Первые шаги к проверке сделаны, но подтверждённых гипотез и реальных метрик пока мало.", "Гипотезы проверены на пользователях, есть пилот или работающий MVP и метрики с текущими значениями."},
		"en": {"The project has not been tested with real users yet — everything rests on assumptions.", "First steps towards validation are made, but confirmed hypotheses and real metrics are still few.", "Hypotheses were tested with users; there is a pilot or a working MVP and metrics with current values."},
	},
	"feasibility": {
		"kk": {"Команда өнімді қалай және не есебінен құратыны түсініксіз: MVP жоспары мен кезеңдер жоқ.", "Іске асыру жоспары бар, бірақ мерзімдер немесе ресурстар оптимистік көрінеді, тәуекелдердің бір бөлігі пысықталмаған.", "MVP мен жол картасы команда үшін шынайы, техникалық тәуекелдер аталған және оларды азайту жоспары бар."},
		"ru": {"Неясно, как и за счёт чего команда построит продукт: нет плана MVP и этапов.", "План реализации есть, но сроки или ресурсы выглядят оптимистично, часть рисков не проработана.", "MVP и дорожная карта реалистичны для команды, технические риски названы и есть план их снижения."},
		"en": {"It is unclear how the team will build the product: there is no MVP plan or milestones.", "There is a delivery plan, but the timing or resources look optimistic and some risks are not addressed.", "The MVP and the roadmap are realistic for the team; technical risks are named with a plan to reduce them."},
	},
	"impact": {
		"kk": {"Әлеуметтік әсер және Қазақстанның даму басымдықтарымен байланыс ашылмаған.", "Ел басымдықтарымен байланыс және әлеуметтік әсер бар, бірақ оларды нақтырақ сипаттау керек — кім үшін және қандай ауқымда.", "Жоба Қазақстанның басымдықтарына қызмет етеді және айтарлықтай әлеуметтік әсер береді; мұны қорғауда атап өткен жөн."},
		"ru": {"Социальный эффект и связь с приоритетами развития Казахстана не раскрыты.", "Связь с приоритетами страны и социальный эффект есть, но их стоит описать конкретнее — для кого и в каком масштабе.", "Проект работает на приоритеты Казахстана и даёт заметный социальный эффект; это стоит подчеркнуть на защите."},
		"en": {"Social impact and the link to Kazakhstan's development priorities are not described.", "There is a link to national priorities and social impact, but describe them more concretely — for whom and at what scale.", "The project serves Kazakhstan's priorities and has a visible social impact; worth stressing at the defence."},
	},
}

var advice = map[string]map[string]string{
	"problem":        {"kk": "5–10 проблемалық сұхбат өткізіп, «Мәселе» қадамына дәйексөздер мен сандар қосыңыз.", "ru": "Проведите 5–10 проблемных интервью и добавьте цитаты и цифры в шаг «Проблема».", "en": "Run 5–10 problem interviews and add quotes and numbers to the Problem step."},
	"market":         {"kk": "TAM/SAM/SOM-ды төменнен жоғары қайта есептеп, әр санның дереккөзін көрсетіңіз.", "ru": "Пересчитайте TAM/SAM/SOM снизу вверх и укажите источник каждой цифры.", "en": "Recalculate TAM/SAM/SOM bottom-up and name the source of each number."},
	"solution":       {"kk": "Әр негізгі функцияны пайдаланушының нақты қиындығымен байланыстырып, MVP-ден артығын алып тастаңыз.", "ru": "Свяжите каждую ключевую функцию с конкретной болью пользователя и уберите лишнее из MVP.", "en": "Tie every key feature to a specific user pain and cut the rest from the MVP."},
	"business_model": {"kk": "Бір клиентке шаққандағы юнит-экономиканы есептеп, кемінде 10 әлеуетті клиентте төлеуге дайындығын тексеріңіз.", "ru": "Посчитайте юнит-экономику на одного клиента и проверьте готовность платить хотя бы на 10 потенциальных клиентах.", "en": "Work out unit economics per customer and test willingness to pay with at least 10 potential customers."},
	"team":           {"kk": "Жетіспейтін негізгі рөлге қатысушы табыңыз — «Командаға іздеймін» тақтасын пайдаланыңыз.", "ru": "Найдите участника на недостающую ключевую роль — используйте доску «Ищу в команду».", "en": "Find someone for the missing key role — use the \"Looking for teammates\" board."},
	"validation":     {"kk": "Нақты пайдаланушылармен бір апталық эксперимент бастап, метрикаларды алдын ала және кейін тіркеңіз.", "ru": "Запустите недельный эксперимент с реальными пользователями и зафиксируйте метрики до и после.", "en": "Run a one-week experiment with real users and record the metrics before and after."},
	"feasibility":    {"kk": "MVP-ді өлшенетін нәтижесі бар 2–4 апталық кезеңдерге бөліп, басты техникалық тәуекелді атаңыз.", "ru": "Разбейте MVP на этапы по 2–4 недели с измеримым результатом и назовите главный технический риск.", "en": "Split the MVP into 2–4 week milestones with measurable results and name the main technical risk."},
	"impact":         {"kk": "Жоба кімге және қандай ауқымда көмектесетінін және қандай қолдау бағдарламаларына сәйкес келетінін сипаттаңыз.", "ru": "Опишите, кому и в каком масштабе поможет проект и с какими программами поддержки он совпадает.", "en": "Describe whom the project helps and at what scale, and which support programmes it matches."},
}

// redFlags[name][locale] = {title, detail}.
var redFlags = map[string]map[string][2]string{
	"no_evidence": {
		"kk": {"Мәселе расталмаған", "Жобада сұхбат немесе бақылау деректері жоқ — сұраныс бағасы болжамдарға сүйенеді."},
		"ru": {"Проблема не подтверждена", "В проекте нет данных интервью или наблюдений — оценка спроса держится на допущениях."},
		"en": {"The problem is not confirmed", "The project has no interview or observation data — demand rests on assumptions."},
	},
	"ltv_cac_low": {
		"kk": {"Клиент әкелетін табысынан қымбатқа түседі", "LTV/CAC 1-ден төмен: қазіргі болжамдар бойынша әр жаңа клиент шығын әкеледі."},
		"ru": {"Клиент обходится дороже, чем приносит", "LTV/CAC ниже 1: при текущих допущениях каждый новый клиент приносит убыток."},
		"en": {"A customer costs more than they bring", "LTV/CAC is below 1: under the current assumptions every new customer loses money."},
	},
	"ltv_cac_mid": {
		"kk": {"Юнит-экономика шегінде", "LTV/CAC 1 мен 3 аралығында: беріктік қоры жоқтың қасы, тарту құнын тексеріңіз."},
		"ru": {"Юнит-экономика на грани", "LTV/CAC между 1 и 3: запаса прочности почти нет, проверьте стоимость привлечения."},
		"en": {"Unit economics on the edge", "LTV/CAC is between 1 and 3: there is little margin; check the acquisition cost."},
	},
	"solo": {
		"kk": {"Жоба бір адамға сүйенеді", "Командада бір ғана мүше — негізгі рөлдер мен тұрақтылық күмәнді."},
		"ru": {"Проект держится на одном человеке", "В команде один участник — ключевые роли и устойчивость под вопросом."},
		"en": {"One-person project", "The team has one member — key roles and resilience are in question."},
	},
	"market_inconsistent": {
		"kk": {"Нарық бағасы қайшылықты", "SOM SAM-нан немесе SAM TAM-нан үлкен — формулаларды тексеріңіз."},
		"ru": {"Оценка рынка противоречива", "SOM больше SAM или SAM больше TAM — проверьте формулы."},
		"en": {"Inconsistent market sizing", "SOM exceeds SAM or SAM exceeds TAM — check the formulas."},
	},
	"no_competitors": {
		"kk": {"Бәсекелестер аталмаған", "Бәсекелестердің жоқтығы әдетте баламалардың зерттелмегенін білдіреді, олардың жоқ екенін емес."},
		"ru": {"Не названы конкуренты", "Отсутствие конкурентов обычно значит, что альтернативы не изучены, а не что их нет."},
		"en": {"No competitors named", "No competitors usually means the alternatives were not studied, not that there are none."},
	},
}

var genericHypotheses = map[string][]string{
	"kk": {"Мақсатты пайдаланушылар мәселеге аптасына кемінде бір рет тап болады.", "Пайдаланушылар шешім үшін көрсетілген бағадан кем емес төлеуге дайын.", "Таңдалған арна клиенттерді жоспарланған CAC-тан арзан әкеледі.", "Пайдаланушылар бірінші аптадан кейін өнімге қайта оралады.", "Серіктестер пилотқа ақысыз қатысуға дайын."},
	"ru": {"Целевые пользователи сталкиваются с проблемой не реже раза в неделю.", "Пользователи готовы платить за решение не меньше заявленной цены.", "Выбранный канал приводит клиентов дешевле планируемого CAC.", "Пользователи вернутся к продукту после первой недели.", "Партнёры готовы участвовать в пилоте без оплаты."},
	"en": {"Target users face the problem at least once a week.", "Users are willing to pay at least the stated price.", "The chosen channel brings customers below the planned CAC.", "Users come back to the product after the first week.", "Partners are ready to join a pilot without payment."},
}

// experimentText[locale] = {title prefix, how, metric, success}.
var experimentText = map[string][4]string{
	"kk": {"Тексеру: ", "Мақсатты пайдаланушылармен 10 қысқа сұхбат немесе лендинг-тест өткізіп, жауаптарды тіркеңіз.", "Болжамды растаған пайдаланушылардың үлесі", "Кемінде 40% растау"},
	"ru": {"Проверка: ", "Проведите 10 коротких интервью или лендинг-тест с целевыми пользователями и зафиксируйте ответы.", "Доля пользователей, подтвердивших гипотезу", "Не меньше 40% подтверждений"},
	"en": {"Test: ", "Run 10 short interviews or a landing-page test with target users and record the answers.", "Share of users who confirm the hypothesis", "At least 40% confirm"},
}

var summaryText = map[string]string{
	"kk": "Жоба «%s» бөлігінде ең күшті, «%s» бөлігінде ең әлсіз. Төмендегі ұсыныстардан бастаңыз: олар бағаны ең көп арттырады.",
	"ru": "Сильнее всего проект в части «%s», слабее всего — «%s». Начните с рекомендаций ниже: они дадут наибольший прирост оценки.",
	"en": "The project is strongest in \"%s\" and weakest in \"%s\". Start with the recommendations below: they raise the score the most.",
}

func cut(s string, n int) string {
	if utf8.RuneCountInString(s) <= n {
		return s
	}
	return string([]rune(s)[:n-1]) + "…"
}

func mockEvaluate(req llm.CompletionRequest) string {
	raw, err := json.Marshal(MockResult(lastBlock(req.System, "project_data"), req.Metadata.Locale))
	if err != nil {
		return "{}"
	}
	return string(raw)
}

// MockResult evaluates project data (incubator.ProjectContext JSON) the way
// the mock provider does; the demo seed uses it for evaluation history.
func MockResult(data, locale string) Result {
	if locale != "kk" && locale != "en" {
		locale = "ru"
	}
	var d projectData
	_ = json.Unmarshal([]byte(data), &d)
	scores := d.scores()

	var out Result
	for _, c := range DefaultRubric {
		score := scores[c.Key]
		out.Criteria = append(out.Criteria, Score{Key: c.Key, Score: score, Justification: justifications[c.Key][locale][band(score)]})
	}

	flag := func(name, severity string) {
		t := redFlags[name][locale]
		out.RedFlags = append(out.RedFlags, RedFlag{Title: t[0], Detail: t[1], Severity: severity})
	}
	if !long(d.str("problem", "evidence"), 80) {
		flag("no_evidence", "high")
	}
	if r, ok := d.ltvCAC(); ok {
		switch {
		case r < 1:
			flag("ltv_cac_low", "high")
		case r < 3:
			flag("ltv_cac_mid", "medium")
		}
	}
	if members := d.rows("team", "members"); len(members) == 1 {
		flag("solo", "medium")
	}
	tam, okT := d.num("market_size", "tam")
	sam, okS := d.num("market_size", "sam")
	som, okO := d.num("market_size", "som")
	if okT && okS && okO && (som > sam || sam > tam) {
		flag("market_inconsistent", "medium")
	}
	if len(d.rows("competitors", "list")) == 0 && len(d.Steps) > 0 {
		flag("no_competitors", "medium")
	}
	if out.RedFlags == nil {
		out.RedFlags = []RedFlag{}
	}

	// Hypotheses: the team's untested ones first, then typical ones.
	type experiment struct{ how, success string }
	var plans []experiment
	for _, h := range d.rows("risks", "hypotheses") {
		text, _ := h["hypothesis"].(string)
		if status, _ := h["status"].(string); status == "untested" && strings.TrimSpace(text) != "" && len(out.Hypotheses) < 5 {
			out.Hypotheses = append(out.Hypotheses, cut(strings.TrimSpace(text), 400))
			how, _ := h["experiment"].(string)
			success, _ := h["success"].(string)
			plans = append(plans, experiment{how, success})
		}
	}
	for _, g := range genericHypotheses[locale] {
		if len(out.Hypotheses) >= 3 {
			break
		}
		out.Hypotheses = append(out.Hypotheses, g)
		plans = append(plans, experiment{})
	}
	words := experimentText[locale]
	for i := range min(3, len(out.Hypotheses)) {
		how, success := words[1], words[3]
		if strings.TrimSpace(plans[i].how) != "" {
			how = cut(plans[i].how, 500)
		}
		if strings.TrimSpace(plans[i].success) != "" {
			success = cut(plans[i].success, 300)
		}
		out.Experiments = append(out.Experiments, Experiment{
			Title: cut(words[0]+out.Hypotheses[i], 200), How: how, Metric: words[2], Success: success, Day: 1 + i*3,
		})
	}

	// Recommendations: the three weakest criteria.
	order := make([]Criterion, len(DefaultRubric))
	copy(order, DefaultRubric)
	slices.SortStableFunc(order, func(a, b Criterion) int { return scores[a.Key] - scores[b.Key] })
	for _, c := range order[:3] {
		out.Recommendations = append(out.Recommendations, advice[c.Key][locale])
	}
	best, worst := order[len(order)-1], order[0]
	out.Summary = fmt.Sprintf(summaryText[locale], best.Title.Best(locale), worst.Title.Best(locale))
	return out
}

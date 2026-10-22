package incubator

import (
	"encoding/json"
	"fmt"
	"hash/fnv"
	"regexp"
	"strings"
	"unicode"
	"unicode/utf8"

	"github.com/yelnurq/startify/backend/internal/ai/llm"
	"github.com/yelnurq/startify/backend/internal/ai/mock"
)

// RegisterMock teaches the mock provider the incubator's answers: mentor
// replies that follow the step, reviews built from the step's criteria and
// materials assembled from the project's own data. The demo runs on them
// without any model.
func RegisterMock() {
	mock.Register(llm.FeatureMentor, func(req llm.CompletionRequest) string {
		switch {
		case strings.HasPrefix(req.Metadata.PromptVersion, "mentor@"):
			return mockMentor(req)
		case strings.HasPrefix(req.Metadata.PromptVersion, "step_check@"):
			return mockCheck(req)
		case req.JSONSchema != nil:
			return fakeJSON(req)
		}
		return mock.Reply(req)
	})
	mock.Register(llm.FeatureGeneration, func(req llm.CompletionRequest) string {
		if strings.HasPrefix(req.Metadata.PromptVersion, "materials@") {
			return mockMaterials(req)
		}
		return fakeJSON(req)
	})
}

func fakeJSON(req llm.CompletionRequest) string {
	if req.JSONSchema == nil {
		return mock.Reply(req)
	}
	raw, err := json.Marshal(mock.Fake(req.JSONSchema.JSON, hashOf(req.System+mock.LastUserMessage(req))))
	if err != nil {
		return "{}"
	}
	return string(raw)
}

func hashOf(s string) uint64 {
	h := fnv.New64a()
	_, _ = h.Write([]byte(s))
	return h.Sum64()
}

// between returns the text inside the last <tag>…</tag> of a prompt: the
// instructions mention the tag before the data block.
func between(text, tag string) string {
	i := strings.LastIndex(text, "<"+tag+">")
	if i < 0 {
		return ""
	}
	inner, _, _ := strings.Cut(text[i+len(tag)+2:], "</"+tag+">")
	return strings.TrimSpace(inner)
}

func localeOf(req llm.CompletionRequest) string {
	switch req.Metadata.Locale {
	case "kk", "en":
		return req.Metadata.Locale
	}
	return "ru"
}

var mentorText = map[string]map[string]string{
	"start": {
		"kk": "«%s» қадамынан бастайық. %s",
		"ru": "Давайте начнём с шага «%s». %s",
		"en": "Let us start with the step \"%s\". %s",
	},
	"gap": {
		"kk": "Жақсы бастама. Қадамды аяқтау үшін «%s» өрісін толтыру қалды.",
		"ru": "Хорошее начало. Чтобы закрыть шаг, осталось заполнить поле «%s».",
		"en": "Good start. To complete the step, fill in \"%s\".",
	},
	"full": {
		"kk": "Қадам мазмұнды көрінеді — енді оны күшейтейік.",
		"ru": "Шаг уже выглядит содержательно — давайте сделаем его сильнее.",
		"en": "The step already has substance — let us make it stronger.",
	},
	"suggest": {
		"kk": "Мына құрылымнан бастауға болады — жақшадағыны өз деректеріңізбен ауыстырыңыз:",
		"ru": "Можно оттолкнуться от такой структуры — замените скобки своими данными:",
		"en": "You could start from this structure — replace the brackets with your own facts:",
	},
	"knowledge": {
		"kk": "«%s» құжатын қараңыз — онда осы тақырып бойынша толығырақ айтылған.",
		"ru": "Загляните в документ «%s» — там есть подробности по этой теме.",
		"en": "Have a look at the document \"%s\" — it covers this topic in more detail.",
	},
}

var mentorQuestions = map[string]map[string][2]string{
	"problem": {
		"kk": {"Бұл мәселеге ең жиі нақты кім тап болады — сондай бір адамды сипаттап бере аласыз ба?", "Олар қанша уақыт немесе ақша жоғалтады және сіз мұны қайдан білесіз — сұхбаттан ба, әлде бұл әзірге болжам ба?"},
		"ru": {"Кто конкретно сталкивается с этой проблемой чаще всего — сможете описать одного такого человека?", "Сколько времени или денег они теряют, и откуда вы это знаете — из интервью или это пока допущение?"},
		"en": {"Who exactly runs into this problem most often — can you describe one such person?", "How much time or money do they lose, and how do you know — from interviews, or is it still an assumption?"},
	},
	"audience": {
		"kk": {"Сегменттеріңіз жасы бойынша емес, мінез-құлқы бойынша немен ерекшеленеді?", "Олардың қайсысы мәселені соншалық өткір сезінеді, шешімді өзі іздеп жүр?"},
		"ru": {"Чем ваши сегменты отличаются по поведению, а не по возрасту?", "Кто из них испытывает проблему так остро, что уже ищет решение сам?"},
		"en": {"How do your segments differ in behaviour rather than age?", "Which of them feels the problem so sharply that they already look for a fix?"},
	},
	"journey": {
		"kk": {"Пайдаланушы үшін бұл оқиға қай сәттен басталып, немен аяқталады?", "Қай кезеңде оған ең қиын — сол сәтте ол не істейді?"},
		"ru": {"С какого момента у пользователя начинается эта история и чем она заканчивается?", "На каком этапе ему хуже всего — и что он в этот момент делает?"},
		"en": {"Where does this story start for the user, and where does it end?", "At which stage is it worst for them — and what do they do at that moment?"},
	},
	"solution": {
		"kk": {"Шешіміңіз басты қиындықтардың қайсысын бірінші жояды және пайдаланушы мұны қалай байқайды?", "Пайдаланушы үйреншікті баламаның орнына неге сізді таңдайды?"},
		"ru": {"Какую из главных болей ваше решение снимает первой и как пользователь это заметит?", "Почему пользователь выберет вас, а не привычную альтернативу?"},
		"en": {"Which key pain does your solution remove first, and how will the user notice?", "Why would the user choose you over the familiar alternative?"},
	},
	"market_size": {
		"kk": {"Қазақстанда әлеуетті клиенттер қанша және бұл сан қайдан алынды — мысалы, Ұлттық статистика бюросының деректерінен бе?", "Бір клиент қаншалықты жиі және қандай сомаға сатып алады?"},
		"ru": {"Сколько потенциальных клиентов в Казахстане и откуда эта цифра — например, из данных Бюро национальной статистики?", "Как часто и на какую сумму будет покупать один клиент?"},
		"en": {"How many potential customers are there in Kazakhstan, and where does the number come from — the Bureau of National Statistics, for example?", "How often and for how much will one customer buy?"},
	},
	"competitors": {
		"kk": {"Болашақ клиенттеріңіз ештеңе сатып алмаса, бүгін не істейді?", "Басты бәсекелесіңіздің ең күшті жағы неде және сіз онымен қалай күресесіз?"},
		"ru": {"Что делают ваши будущие клиенты сегодня, если ничего не покупают?", "В чём сильнее всего ваш главный конкурент и как вы с этим справитесь?"},
		"en": {"What do your future customers do today if they buy nothing?", "Where is your main competitor strongest, and how will you deal with it?"},
	},
	"business_model": {
		"kk": {"Нақты кім төлейді — пайдаланушы ма, ұйым ба — және не үшін?", "Алғашқы 100 клиентті қай арна арқылы тартасыз және бұл қаншаға түседі?"},
		"ru": {"Кто именно платит — пользователь или организация — и за что?", "Через какой канал вы привлечёте первых 100 клиентов и сколько это будет стоить?"},
		"en": {"Who exactly pays — the user or an organisation — and for what?", "Through which channel will you win the first 100 customers, and what will it cost?"},
	},
	"team": {
		"kk": {"Командада сату мен клиенттермен сөйлесуге кім жауап береді?", "MVP жасау үшін қай рөл ең көп жетіспейді?"},
		"ru": {"Кто в команде отвечает за продажи и разговоры с клиентами?", "Какой роли сильнее всего не хватает, чтобы сделать MVP?"},
		"en": {"Who in the team is responsible for sales and talking to customers?", "Which role is missing most to build the MVP?"},
	},
	"mvp": {
		"kk": {"MVP қандай бір болжамды тексеруі керек?", "Оны кодсыз, мысалы прототиппен немесе қолмен көрсетілетін қызметпен, тезірек тексеруге бола ма?"},
		"ru": {"Какую одну гипотезу должен проверить MVP?", "Можно ли проверить её ещё быстрее — без кода, например прототипом или ручной услугой?"},
		"en": {"Which single hypothesis must the MVP test?", "Could you test it even faster — without code, with a prototype or a manual service?"},
	},
	"metrics": {
		"kk": {"Пайдаланушының қай әрекеті оның құндылық алғанын ең жақсы көрсетеді?", "Осы айда өз әрекеттеріңізбен қай екі-үш метриканы өзгерте аласыз?"},
		"ru": {"Какое действие пользователя лучше всего показывает, что он получил ценность?", "Какие две-три метрики вы можете изменить своими действиями уже в этом месяце?"},
		"en": {"Which user action best shows they got value?", "Which two or three metrics can you move with your own actions this month?"},
	},
	"unit_economics": {
		"kk": {"Сіздің есеп бірлігіңіз не — клиент пе, тапсырыс па, әлде айлық жазылым ба?", "CAC саны қайдан алынды — арнаны тексеруден бе, әлде әзірге болжам ба?"},
		"ru": {"Что считается вашей единицей — клиент, заказ или подписка в месяц?", "Откуда цифра CAC — из теста канала или это пока предположение?"},
		"en": {"What is your unit — a customer, an order or a monthly subscription?", "Where does the CAC figure come from — a channel test, or is it still a guess?"},
	},
	"risks": {
		"kk": {"Қай тәуекел іске асса, жобаны ең тез тоқтатады?", "Қай болжамды 7 күнде тексеруге болады және оның расталғанын қалай түсінесіз?"},
		"ru": {"Какой риск убьёт проект быстрее всего, если сбудется?", "Какую гипотезу можно проверить за 7 дней и как вы поймёте, что она подтвердилась?"},
		"en": {"Which risk would kill the project fastest if it came true?", "Which hypothesis can you test in 7 days, and how will you know it was confirmed?"},
	},
	"roadmap": {
		"kk": {"3 айдан кейін қандай өлшенетін нәтиже алғыңыз келеді?", "Жобаға қандай қолдау бағдарламалары сай келеді — мысалы, университет гранттары немесе акселераторлар? Шарттарын ресми сайттардан тексерген жөн."},
		"ru": {"Какой измеримый результат вы хотите получить через три месяца?", "Какие программы поддержки подходят проекту — например, гранты вуза или акселераторы? Условия стоит проверить на официальных сайтах."},
		"en": {"Which measurable result do you want in three months?", "Which support programmes fit the project — university grants or accelerators, for example? Check their terms on official websites."},
	},
	"pitch": {
		"kk": {"Жобаны ол туралы ештеңе білмейтін адамға бір сөйлеммен түсіндіре аласыз ба?", "Тыңдаушылардан нақты не сұрайсыз — қаржы, пилот, серіктес пе, әлде ментор ма?"},
		"ru": {"Сможете объяснить проект одной фразой человеку, который о нём ничего не знает?", "Что конкретно вы просите у слушателей — деньги, пилот, партнёра или ментора?"},
		"en": {"Can you explain the project in one sentence to someone who knows nothing about it?", "What exactly do you ask the audience for — money, a pilot, a partner or a mentor?"},
	},
}

// mentorSkeletons are structures the mock proposes for the main text field of
// a step. {field|placeholder} takes the value of a field of the step (or the
// project title for "title") and falls back to the placeholder.
var mentorSkeletons = map[string]struct {
	field string
	text  map[string]string
}{
	"problem": {"statement", map[string]string{
		"kk": "{who|[Нақты кім]} [қандай жағдайда] [нені] істей алмайды. Мәселенің құны: {cost|[қанша уақыт немесе ақша, қаншалықты жиі]}. Қазір олар {alternatives|[мәселені қалай шешеді]}, бірақ [бұл шешімнің несі ұнамайды].",
		"ru": "{who|[Кто именно]} в ситуации, когда [опишите момент], не может [что сделать]. Цена проблемы: {cost|[сколько времени или денег и как часто]}. Сейчас они {alternatives|[как решают проблему]}, но [чем это их не устраивает].",
		"en": "{who|[Who exactly]}, when [describe the moment], cannot [do what]. The cost: {cost|[how much time or money, how often]}. Today they {alternatives|[how they cope]}, but [what is wrong with that].",
	}},
	"audience": {"early_adopters", map[string]string{
		"kk": "Алғашқы болып [қай топ] келеді, себебі [мәселе оларда ең өткір / олар шешім іздеп жүр]. Оларды [нақты қай жерден] табуға болады.",
		"ru": "Первыми придут [какая группа], потому что [у них проблема острее всего / они уже ищут решение]. Найти их можно [где именно].",
		"en": "[Which group] will come first because [the problem hurts them most / they already look for a fix]. You can reach them [where exactly].",
	}},
	"journey": {"job", map[string]string{
		"kk": "Қашан [жағдай] болса, мен [не істегім] келеді, [қандай нәтиже алу] үшін.",
		"ru": "Когда [ситуация], я хочу [что сделать], чтобы [какой результат получить].",
		"en": "When [situation], I want to [action], so I can [outcome].",
	}},
	"solution": {"uvp", map[string]string{
		"kk": "{title|[Өнім]} [кімге] [қандай нәтижені] [уақыт / баға] ішінде алуға көмектеседі, [баламадан] айырмашылығы — [неде].",
		"ru": "{title|[Продукт]} помогает [кому] [получить какой результат] за [время / цену], в отличие от [альтернативы].",
		"en": "{title|[Product]} helps [whom] [get which result] in [time / for what price], unlike [alternative].",
	}},
	"market_size": {"som_method", map[string]string{
		"kk": "SOM = [1–3 жылда нақты тартуға болатын клиенттер саны] × [жылына сатып алу] × [орташа чек] = [сома] теңге. Болжам: [клиенттер саны неге негізделген].",
		"ru": "SOM = [число клиентов, которых реально привлечь за 1–3 года] × [покупок в год] × [средний чек] = [сумма] тенге. Допущение: [на чём основано число клиентов].",
		"en": "SOM = [customers you can realistically win in 1–3 years] × [purchases per year] × [average price] = [amount] tenge. Assumption: [what the customer count is based on].",
	}},
	"competitors": {"difference", map[string]string{
		"kk": "[Басты бәсекелестен] айырмашылығымыз — [негізгі айырмашылық], сондықтан клиент [қандай пайда алады]. Мұны қайталау қиын, себебі [кедергі].",
		"ru": "В отличие от [главного конкурента], мы [ключевое отличие], поэтому клиент [получает какую выгоду]. Повторить это сложно, потому что [барьер].",
		"en": "Unlike [main competitor], we [key difference], so the customer [gets what benefit]. It is hard to copy because [barrier].",
	}},
	"business_model": {"revenue", map[string]string{
		"kk": "[Кім] [не үшін] төлейді — [модель: жазылым / бір реттік сату / комиссия]. Орташа чек [сома] теңге, төлем [қаншалықты жиі].",
		"ru": "Платит [кто] за [что] — [модель: подписка / разовая продажа / комиссия]. Средний чек [сумма] тенге, оплата [как часто].",
		"en": "[Who] pays for [what] — [model: subscription / one-off sale / commission]. Average price [amount] tenge, paid [how often].",
	}},
	"team": {"why_us", map[string]string{
		"kk": "Біз [жоба тақырыбы бойынша не істедік]; бізде [негізгі тәжірибе немесе қолжетімділік] бар. Бізге [рөл] жетіспейді, оны [қалай] табамыз.",
		"ru": "Мы [что уже сделали по теме проекта]; у нас есть [ключевой опыт или доступ]. Не хватает [роль], и мы найдём её [как].",
		"en": "We have [what we already did on this topic]; we bring [key experience or access]. We lack [role] and will find them [how].",
	}},
	"mvp": {"test_plan", map[string]string{
		"kk": "MVP-ді [қанша] пайдаланушыға [қай сегменттен] [мерзім] ішінде көрсетеміз. Табыс — [өлшенетін шек, мысалы 10 адамның 4-еуі төлеуге дайын] болса.",
		"ru": "Покажем MVP [скольким] пользователям из [какого сегмента] за [срок]. Успех — если [измеримый порог, например 4 из 10 готовы платить].",
		"en": "We will show the MVP to [how many] users from [which segment] within [time]. Success is [a measurable threshold, e.g. 4 of 10 ready to pay].",
	}},
	"metrics": {"why", map[string]string{
		"kk": "Бұл метрика клиент [құндылық алғанда] ғана өседі, оған [2–3 тетік-метрика] әсер етеді.",
		"ru": "Эта метрика растёт, только когда клиент [получает ценность], и на неё влияют [2–3 метрики-рычага].",
		"en": "This metric grows only when the customer [gets value], and it is driven by [2–3 lever metrics].",
	}},
	"unit_economics": {"assumptions", map[string]string{
		"kk": "Чек [сома] — [дереккөз: сұхбаттар, бәсекелестердің бағалары] бойынша. CAC [сома] — [арна және оның құны] бойынша. Клиенттің сатып алу саны [сан] — болжам, [қалай] тексереміз.",
		"ru": "Чек [сумма] — по [источник: интервью, цены конкурентов]. CAC [сумма] — из [канал и его стоимость]. Покупок за жизнь клиента [число] — допущение, проверим [как].",
		"en": "Price [amount] — from [source: interviews, competitor prices]. CAC [amount] — from [channel and its cost]. Purchases per lifetime [number] — an assumption we will test by [how].",
	}},
	"roadmap": {"use_of_funds", map[string]string{
		"kk": "[Сома] теңге: [үлес]% — [әзірлеуге], [үлес]% — [пилотқа / маркетингке], [үлес]% — [басқасына]. Бұл [кезеңге] дейін жетеді, онда біз [өлшенетін нәтиже].",
		"ru": "[Сумма] тенге: [доля]% — на [разработку], [доля]% — на [пилот / маркетинг], [доля]% — на [прочее]. Этого хватит до [этап], где мы [измеримый результат].",
		"en": "[Amount] tenge: [share]% for [development], [share]% for [pilot / marketing], [share]% for [other]. It lasts until [milestone], where we [measurable result].",
	}},
	"pitch": {"one_liner", map[string]string{
		"kk": "{title|[Жоба]} — [кімге] арналған [не], ол [қандай нәтижеге] жетуге көмектеседі.",
		"ru": "{title|[Проект]} — [что это] для [кого], которое помогает [получить какой результат].",
		"en": "{title|[The project]} is [what] for [whom] that helps [achieve what].",
	}},
}

var asksForWording = []string{
	"сформулир", "улучш", "предлож", "перепиш", "вариант", "пример",
	"жақсарт", "ұсын", "тұжырымда", "қайта жаз", "мысал",
	"improve", "rewrite", "suggest", "wording", "draft", "example",
}

// promptData is the project context the incubator gives the mentor and the
// materials prompt as JSON.
type promptData struct {
	Project struct {
		Title    string `json:"title"`
		Tagline  string `json:"tagline"`
		Industry string `json:"industry"`
	} `json:"project"`
	Step *struct {
		Key    string `json:"key"`
		Values Values `json:"values"`
	} `json:"step,omitempty"`
	Steps map[string]Values `json:"steps"`
}

func mockMentor(req llm.CompletionRequest) string {
	locale := localeOf(req)
	var data promptData
	_ = json.Unmarshal([]byte(between(req.System, "project_data")), &data)
	key, values := "problem", Values{}
	if data.Step != nil {
		key, values = data.Step.Key, data.Step.Values
	}
	step, ok := Default().Step(key)
	if !ok {
		return mock.Reply(req)
	}
	question := mock.LastUserMessage(req)
	var parts []string

	gaps := step.Missing(values)
	filled, _ := step.Filled(values)
	switch {
	case filled == 0:
		parts = append(parts, fmt.Sprintf(mentorText["start"][locale], step.Title.Best(locale), step.Goal.Best(locale)))
	case len(gaps) > 0:
		parts = append(parts, fmt.Sprintf(mentorText["gap"][locale], step.Field(gaps[0].Field).Label.Best(locale)))
	default:
		parts = append(parts, mentorText["full"][locale])
	}

	qs := mentorQuestions[key][locale]
	n := int(hashOf(question) % 2)
	parts = append(parts, qs[n])
	if filled > 0 {
		parts = append(parts, qs[1-n])
	}

	lower := strings.ToLower(question)
	wants := strings.Contains(req.System, "Mode: co-author")
	for _, w := range asksForWording {
		wants = wants || strings.Contains(lower, w)
	}
	if sk, ok := mentorSkeletons[key]; ok && wants {
		text := fillSkeleton(sk.text[locale], values, data.Project.Title)
		parts = append(parts, mentorText["suggest"][locale]+"\n<suggestion field=\""+sk.field+"\">"+text+"</suggestion>")
	}

	if title := firstSourceTitle(req.System); title != "" {
		parts = append(parts, fmt.Sprintf(mentorText["knowledge"][locale], title))
	}
	return strings.Join(parts, "\n\n")
}

var sourceTitle = regexp.MustCompile(`<source title="([^"]+)">`)

func firstSourceTitle(system string) string {
	if m := sourceTitle.FindStringSubmatch(system); m != nil {
		return m[1]
	}
	return ""
}

var skeletonSlot = regexp.MustCompile(`\{([a-z_]+)\|([^}]+)\}`)

func fillSkeleton(text string, values Values, title string) string {
	var b strings.Builder
	last := 0
	for _, loc := range skeletonSlot.FindAllStringSubmatchIndex(text, -1) {
		b.WriteString(text[last:loc[0]])
		key, placeholder := text[loc[2]:loc[3]], text[loc[4]:loc[5]]
		v := title
		if key != "title" {
			v, _ = values[key].(string)
		}
		switch v = firstClause(v); {
		case v == "":
			b.WriteString(placeholder)
		case key != "title" && !sentenceStart(b.String()):
			b.WriteString(lowerFirst(v))
		default:
			b.WriteString(v)
		}
		last = loc[1]
	}
	b.WriteString(text[last:])
	return b.String()
}

func sentenceStart(prefix string) bool {
	p := strings.TrimSpace(prefix)
	return p == "" || strings.HasSuffix(p, ".") || strings.HasSuffix(p, "!") || strings.HasSuffix(p, "?")
}

// lowerFirst lowers the first letter of a value inserted mid-sentence, but
// keeps abbreviations such as "КазУТБ" or "IT".
func lowerFirst(s string) string {
	r := []rune(s)
	if len(r) > 1 && unicode.IsUpper(r[1]) {
		return s
	}
	r[0] = unicode.ToLower(r[0])
	return string(r)
}

// firstClause shortens a value to its first clause, without the final stop,
// cutting long ones at a word boundary.
func firstClause(s string) string {
	s = strings.TrimSpace(s)
	if i := strings.IndexAny(s, ".!?;\n"); i > 0 {
		s = s[:i]
	}
	if r := []rune(s); len(r) > 120 {
		cut := string(r[:120])
		if i := strings.LastIndex(cut, " "); i > 60 {
			cut = cut[:i]
		}
		s = strings.TrimRight(cut, ",:— ") + "…"
	}
	return strings.TrimSpace(s)
}

var checkText = map[string]map[string]string{
	"missing": {
		"kk": "Әзірге толтырылмаған: «%s».",
		"ru": "Пока не заполнено: «%s».",
		"en": "Not filled in yet: \"%s\".",
	},
	"weak": {
		"kk": "«%s» өрісіне нақтылық қосыңыз: сандар, мысалдар, дереккөздер.",
		"ru": "Добавьте конкретики в «%s»: цифры, примеры, источники.",
		"en": "Add specifics to \"%s\": numbers, examples, sources.",
	},
	"ok": {
		"kk": "Жақсы: жауап нақты. Оның болжамға емес, фактілерге сүйенетінін тексеріңіз.",
		"ru": "Хорошо: ответ конкретный. Проверьте, что он опирается на факты, а не на допущения.",
		"en": "Good: the answer is concrete. Check that it rests on facts, not assumptions.",
	},
	"ready": {
		"kk": "Қадам дайын: жауаптар нақты және критерийлерге сай. Қорғау алдында сандарды дереккөздер бойынша қайта тексеріңіз.",
		"ru": "Шаг готов: ответы конкретные и закрывают критерии. Перед защитой перепроверьте цифры по источникам.",
		"en": "The step is ready: the answers are concrete and meet the criteria. Re-check the numbers against sources before the defence.",
	},
	"not_ready": {
		"kk": "Қадамды күшейту керек: %d/%d критерий толық орындалмаған. Тізімнің бірінші тармағынан бастаңыз.",
		"ru": "Шаг нужно усилить: %d из %d критериев выполнены не полностью. Начните с первого пункта списка.",
		"en": "The step needs work: %d of %d criteria are not fully met. Start with the first item on the list.",
	},
}

// fieldLevel grades how well a field is answered: 0 missing, 1 weak, 2 ok.
func fieldLevel(f *Field, v any) int {
	switch f.Type {
	case TypeList, TypeTable:
		n := f.filled(v)
		switch {
		case n == 0:
			return 0
		case n < max(f.MinItems, 1):
			return 1
		}
		return 2
	case TypeNumber:
		if _, ok := v.(float64); ok {
			return 2
		}
		return 0
	default:
		s, _ := v.(string)
		n := utf8.RuneCountInString(strings.TrimSpace(s))
		strong := max(f.MinLength*3/2, 20)
		if f.Type == TypeTextarea {
			strong = max(strong, 60)
		}
		switch {
		case n == 0:
			return 0
		case n < strong:
			return 1
		}
		return 2
	}
}

func mockCheck(req llm.CompletionRequest) string {
	locale := localeOf(req)
	var answers struct {
		Step   string `json:"step"`
		Values Values `json:"values"`
	}
	_ = json.Unmarshal([]byte(between(req.System, "step_answers")), &answers)
	step, ok := Default().Step(answers.Step)
	if !ok {
		return fakeJSON(req)
	}
	statuses := []string{"missing", "weak", "ok"}
	items := make([]map[string]any, 0, len(step.Criteria))
	weak, missing := 0, 0
	for _, c := range step.Criteria {
		level := 2
		var worst *Field
		for _, key := range c.Fields {
			f := step.Field(key)
			if l := fieldLevel(f, answers.Values[key]); worst == nil || l < level {
				level, worst = l, f
			}
		}
		status := statuses[level]
		advice := checkText["ok"][locale]
		if level < 2 {
			advice = fmt.Sprintf(checkText[status][locale], worst.Label.Best(locale))
			if hint := worst.Hint.Best(locale); level == 0 && hint != "" {
				advice += " " + hint
			}
		}
		switch level {
		case 0:
			missing++
		case 1:
			weak++
		}
		items = append(items, map[string]any{"criterion": c.Text.Best(locale), "status": status, "advice": advice})
	}
	ready := missing == 0 && weak <= 1
	summary := checkText["ready"][locale]
	if !ready {
		summary = fmt.Sprintf(checkText["not_ready"][locale], missing+weak, len(step.Criteria))
	}
	raw, _ := json.Marshal(map[string]any{"ready": ready, "summary": summary, "items": items})
	return string(raw)
}

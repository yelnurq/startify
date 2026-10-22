package seed

import (
	"context"
	"fmt"

	"github.com/google/uuid"

	"github.com/yelnurq/startify/backend/internal/knowledge"
	"github.com/yelnurq/startify/backend/internal/knowledge/knowledgedb"
	"github.com/yelnurq/startify/backend/internal/platform/db"
	"github.com/yelnurq/startify/backend/internal/platform/jobs"
)

// KnowledgeDocument is a demo document of the knowledge base.
type KnowledgeDocument struct {
	Title string
	Body  string
}

// KnowledgeDocuments adds the documents missing by title and schedules their
// indexing; the worker embeds them with the configured provider.
func KnowledgeDocuments(ctx context.Context, database *db.DB, queue jobs.Enqueuer, tenantID uuid.UUID, docs []KnowledgeDocument) error {
	err := database.Tx(db.WithTenant(ctx, tenantID), func(ctx context.Context) error {
		q := knowledgedb.New(database.Conn(ctx))
		existing, err := q.ListDocuments(ctx, knowledgedb.ListDocumentsParams{RowLimit: 1000})
		if err != nil {
			return err
		}
		have := map[string]bool{}
		for i := range existing {
			have[existing[i].Title] = true
		}
		for i := range docs {
			if have[docs[i].Title] {
				continue
			}
			doc, err := q.CreateDocument(ctx, knowledgedb.CreateDocumentParams{Title: docs[i].Title, Body: docs[i].Body})
			if err != nil {
				return err
			}
			if err := queue.Enqueue(ctx, knowledge.IndexArgs{TenantID: tenantID, DocumentID: doc.ID}, nil); err != nil {
				return err
			}
		}
		return nil
	})
	if err != nil {
		return fmt.Errorf("seed: knowledge: %w", err)
	}
	return nil
}

// KazUTBKnowledge are fictional regulations the demo mentor cites.
var KazUTBKnowledge = []KnowledgeDocument{
	{
		Title: "Положение о конкурсе студенческих стартап-проектов «Qadam Startup Cup 2026»",
		Body: `Демонстрационный документ платформы Qadam.

## 1. Общие положения

Конкурс студенческих стартап-проектов «Qadam Startup Cup 2026» проводится Офисом коммерциализации КазУТБ им. К. Кулажанова. Цель конкурса — выявить и поддержать проекты студентов и магистрантов, готовые к созданию прототипа и выходу на первых пользователей.

Организатор конкурса — Офис коммерциализации и трансфера технологий. Вопросы по конкурсу принимаются по внутренней почте офиса и на платформе Qadam в разделе «Конкурсы».

## 2. Участники

В конкурсе участвуют команды от 2 до 5 человек. Капитан команды должен быть студентом или магистрантом КазУТБ очной формы обучения. В команду могут входить студенты других вузов и выпускники, но не более двух человек.

У каждой команды должен быть научный руководитель или ментор из числа преподавателей университета. Один преподаватель может сопровождать не более трёх команд.

## 3. Сроки

Приём заявок открыт с 1 октября до 1 декабря 2026 года включительно. Заявки, поданные после 23:59 по времени Астаны 1 декабря, не рассматриваются.

Отборочный этап проходит с 2 по 15 декабря 2026 года. Финал с публичной защитой проектов состоится 22 января 2027 года.

## 4. Требования к заявке

Заявка подаётся только через платформу Qadam и включает: описание проблемы и целевой аудитории, предлагаемое решение, анализ конкурентов, результаты не менее пяти интервью с потенциальными пользователями, план работ на шесть месяцев и смету расходов.

К заявке прикладывается презентация (не более 12 слайдов) и видео-питч длительностью до 3 минут.

## 5. Критерии оценки

Экспертная комиссия оценивает проекты по пяти критериям по шкале от 0 до 10 баллов: актуальность проблемы и подтверждение спроса (вес 25%), качество решения и его новизна (20%), рынок и бизнес-модель (20%), команда (20%), реалистичность плана и сметы (15%).

Каждую заявку оценивают не менее трёх экспертов. Эксперт, связанный с командой, обязан заявить о конфликте интересов и не оценивает этот проект.

## 6. Призы и поддержка

Победители получают гранты на развитие проекта: первое место — 3 000 000 тенге, второе — 2 000 000 тенге, третье — 1 000 000 тенге. Ещё до десяти команд проходят в акселерационную программу университета.

Грант расходуется по смете в течение 12 месяцев: на материалы, оборудование, сервисы, прототипирование и участие в выставках. Выплата вознаграждений участникам команды из гранта не допускается.`,
	},
	{
		Title: "Студенттік инновациялық жобаларды гранттық қаржыландыру ережесі",
		Body: `Qadam платформасының демонстрациялық құжаты.

## 1. Жалпы ережелер

Осы ереже Қ. Құлажанов атындағы ҚазТБУ студенттері мен магистранттарының инновациялық жобаларына ішкі грант беру тәртібін анықтайды. Грантты университеттің Инновациялық даму қоры қаржыландырады.

## 2. Грант мөлшері

Бір жобаға берілетін грант мөлшері 1 500 000 теңгеге дейін. Грант 12 айға беріледі және екі кезеңмен аударылады: шарт жасалғаннан кейін 60%, аралық есеп қабылданғаннан кейін 40%.

## 3. Өтінім беру

Өтінім Qadam платформасы арқылы ғана қабылданады. Өтінімге жобаның сипаттамасы, күнтізбелік жоспар, шығыстар сметасы және жетекшінің ұсыным хаты қоса беріледі.

Өтінімдер жыл сайын 1 наурыздан 15 сәуірге дейін қабылданады.

## 4. Есеп беру

Грант алушы алты айдан кейін аралық есеп, ал грант мерзімі аяқталғаннан кейін 30 күн ішінде қорытынды есеп тапсырады. Есепке шығыстарды растайтын құжаттар мен жобаның нәтижелері (прототип, пайдаланушылармен жүргізілген тестілер, жарияланымдар) қоса беріледі.

Есеп уақытында тапсырылмаса, келесі кезеңнің қаржысы тоқтатылады.`,
	},
	{
		Title: "Intellectual property guide for student projects",
		Body: `A demonstration document of the Qadam platform.

## Who owns the result

Results created by students on their own initiative and without the university's funding belong to the authors. When a project is financed by a university grant or uses university equipment beyond regular classes, the university and the authors sign an agreement on rights before the grant is paid; by default the authors keep the rights and the university receives a free non-exclusive licence for teaching and research.

## Protecting an invention

Before publishing or presenting a technical solution in public, contact the Commercialisation Office: a public disclosure may prevent patenting. The office helps prepare an application for a utility model or an invention to the National Institute of Intellectual Property and covers the state fees for projects of the Qadam Startup Cup finalists.

## Software and databases

Source code and databases are protected by copyright from the moment they are created. Registration with the authorised body is optional but helps in disputes; the office prepares the documents free of charge.

## Confidentiality

Mentors and jury members keep project materials confidential and do not use them outside the evaluation. Teams decide which parts of a project are visible to other users of the platform.`,
	},
	{
		Title: "Методические рекомендации: проблемное интервью с пользователями",
		Body: `Демонстрационный документ платформы Qadam.

## Зачем проводить интервью

Проблемное интервью проверяет, существует ли проблема у выбранной аудитории и насколько она болезненна. Его проводят до разработки решения: так команда не тратит месяцы на продукт, который никому не нужен.

## Как подготовиться

Сформулируйте гипотезу: кто испытывает проблему, в какой ситуации и как решает её сейчас. Подготовьте 6–8 открытых вопросов о прошлом опыте человека. Найдите не менее пяти собеседников из целевой аудитории, которые не являются вашими друзьями или родственниками.

## Правила интервью

Спрашивайте о фактах из прошлого, а не о будущем: «Когда это случилось в последний раз?» вместо «Купили бы вы такое приложение?». Не рассказывайте о своём решении до конца интервью. Слушайте больше, чем говорите, и записывайте точные формулировки собеседника.

## Как оценить результаты

После каждого интервью отметьте: подтвердилась ли проблема, как часто она возникает, сколько времени или денег теряет человек, чем он пользуется сейчас. Если проблема подтвердилась менее чем у трёх собеседников из пяти, пересмотрите гипотезу или аудиторию.`,
	},
}

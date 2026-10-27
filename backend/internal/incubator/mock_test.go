package incubator

import (
	"context"
	"encoding/json"
	"strings"
	"testing"

	"github.com/yelnurq/startify/backend/internal/ai/llm"
	"github.com/yelnurq/startify/backend/internal/ai/mock"
	"github.com/yelnurq/startify/backend/internal/ai/schema"
)

func ask(t *testing.T, feature llm.Feature, version, system, question, locale string, s *llm.Schema) string {
	t.Helper()
	RegisterMock()
	t.Cleanup(func() {
		mock.Register(llm.FeatureMentor, nil)
		mock.Register(llm.FeatureGeneration, nil)
	})
	res, err := (&mock.Provider{}).Complete(context.Background(), llm.CompletionRequest{
		System: system, JSONSchema: s,
		Messages: []llm.Message{{Role: llm.RoleUser, Content: question}},
		Metadata: llm.RequestMeta{Feature: feature, PromptVersion: version, Locale: locale},
	})
	if err != nil {
		t.Fatal(err)
	}
	return res.Text
}

func projectData(t *testing.T, step string, values Values, steps map[string]Values) string {
	t.Helper()
	d := map[string]any{
		"project": map[string]any{"title": "AgroScan", "tagline": "", "industry": "agritech"},
		"steps":   steps,
	}
	if step != "" {
		d["step"] = map[string]any{"key": step, "values": values}
	}
	raw, err := json.Marshal(d)
	if err != nil {
		t.Fatal(err)
	}
	return "Mode: coach\nThe project data is between the <project_data> tags.\n<project_data>\n" + string(raw) + "\n</project_data>"
}

func TestMockMentorFollowsTheStep(t *testing.T) {
	empty := ask(t, llm.FeatureMentor, "mentor@2", projectData(t, "problem", Values{}, nil), "С чего начать?", "ru", nil)
	if !strings.Contains(empty, "Давайте начнём с шага «Проблема и для кого она»") || strings.Contains(empty, "<suggestion") {
		t.Fatalf("empty step:\n%s", empty)
	}

	values := Values{"statement": "Фермеры поздно замечают болезни пшеницы.", "who": "Фермеры Костанайской области с полями до 500 га."}
	system := projectData(t, "problem", values, nil) + `<source title="Положение о конкурсе">…</source>`
	reply := ask(t, llm.FeatureMentor, "mentor@2", system, "Помоги сформулировать проблему", "ru", nil)
	for _, want := range []string{
		"осталось заполнить поле «Формулировка проблемы»",
		`<suggestion field="statement">Фермеры Костанайской области с полями до 500 га в ситуации`,
		"[сколько времени или денег и как часто]",
		"«Положение о конкурсе»",
	} {
		if !strings.Contains(reply, want) {
			t.Errorf("missing %q in:\n%s", want, reply)
		}
	}

	kk := ask(t, llm.FeatureMentor, "mentor@2", projectData(t, "pitch", Values{}, nil), "Мысал келтірші", "kk", nil)
	if !strings.Contains(kk, `<suggestion field="one_liner">AgroScan — `) || !strings.Contains(kk, "қадамынан бастайық") {
		t.Fatalf("kk:\n%s", kk)
	}
	if other := ask(t, llm.FeatureMentor, "connection_check@1", "", "ping", "en", nil); strings.Contains(other, "<suggestion") || other == "" {
		t.Fatalf("other prompts keep the default reply: %q", other)
	}
}

func TestMockCheckGradesByCriteria(t *testing.T) {
	v, err := schema.Compile(checkSchema.Name, checkSchema.JSON)
	if err != nil {
		t.Fatal(err)
	}
	answers := func(values Values) string {
		raw, _ := json.Marshal(map[string]any{"step": "problem", "values": values})
		return "<step_answers>\n" + string(raw) + "\n</step_answers>"
	}
	out := ask(t, llm.FeatureMentor, "step_check@1", answers(Values{
		"statement":    "Фермеры Северного Казахстана теряют до 20% урожая пшеницы, потому что замечают болезни слишком поздно, когда обработка уже не помогает.",
		"alternatives": "Агрономы объезжают поля раз в неделю.",
	}), "", "ru", &checkSchema)
	doc, err := v.Validate(out)
	if err != nil {
		t.Fatalf("%v\n%s", err, out)
	}
	res := doc.(map[string]any)
	items := res["items"].([]any)
	statuses := make([]string, len(items))
	for i, it := range items {
		statuses[i] = it.(map[string]any)["status"].(string)
	}
	if strings.Join(statuses, ",") != "ok,missing,weak,missing" || res["ready"] != false {
		t.Fatalf("%v\n%s", statuses, out)
	}
	if advice := items[1].(map[string]any)["advice"].(string); !strings.Contains(advice, "Пока не заполнено: «Цена проблемы».") {
		t.Fatal(advice)
	}
}

func TestMockMaterialsUseOnlyTheProjectData(t *testing.T) {
	v, err := schema.Compile(pitchSchema.Name, pitchSchema.JSON)
	if err != nil {
		t.Fatal(err)
	}
	steps := map[string]Values{
		"problem":     {"statement": "Фермеры поздно замечают болезни пшеницы. Потери достигают 20% урожая."},
		"solution":    {"uvp": "Диагностика болезни по фото листа за 10 секунд", "features": []any{"Фото с телефона", "Карта очагов"}},
		"market_size": {"tam": 1500000000.0, "som": 45000000.0},
		"team":        {"members": []any{map[string]any{"name": "Нурлан Ахметов", "role": "CEO"}}},
		"pitch":       {"ask": "Ищем пилот в двух хозяйствах."},
	}
	out := ask(t, llm.FeatureGeneration, "materials@1", projectData(t, "", nil, steps), "", "en", &pitchSchema)
	doc, err := v.Validate(out)
	if err != nil {
		t.Fatalf("%v\n%s", err, out)
	}
	var c PitchContent
	if err := json.Unmarshal([]byte(out), &c); err != nil {
		t.Fatal(err)
	}
	_ = doc
	market := strings.ReplaceAll(c.Market, string(nbsp), " ")
	if c.Tagline != "Диагностика болезни по фото листа за 10 секунд" || market != "TAM: 1 500 000 000 ₸ · SOM: 45 000 000 ₸" {
		t.Fatalf("%+v", c)
	}
	if c.Slides[0].Title != "AgroScan" || c.Team != "Нурлан Ахметов — CEO" {
		t.Fatalf("%+v", c)
	}
	titles := make([]string, len(c.Slides))
	for i, s := range c.Slides {
		titles[i] = s.Title
	}
	// The deck keeps the 12 classic slides; a slide without data says what to
	// add instead of being invented.
	if strings.Join(titles, "|") != "AgroScan|Problem|Customer|Solution|Market|Competitors|Business model|Traction and metrics|Team|Roadmap|The ask|Contact" {
		t.Fatal(titles)
	}
	if b := c.Slides[2].Bullets; len(b) != 1 || b[0] != "The team has not filled in this section yet" {
		t.Fatalf("customer slide %v", b)
	}
	if b := c.Slides[1].Bullets; len(b) == 0 || !strings.Contains(b[0], "Фермеры") {
		t.Fatalf("problem slide %v", b)
	}
}

func TestDeckSchemaAsksForTenToTwelveSlides(t *testing.T) {
	v, err := schema.Compile(pitchSchema.Name, pitchSchema.JSON)
	if err != nil {
		t.Fatal(err)
	}
	deck := func(n int) string {
		slides := make([]map[string]any, n)
		for i := range slides {
			slides[i] = map[string]any{"title": "S", "bullets": []string{"b"}}
		}
		raw, _ := json.Marshal(map[string]any{
			"tagline": "", "problem": "", "customer": "", "solution": "", "market": "", "competition": "",
			"business_model": "", "traction": "", "team": "", "ask": "", "slides": slides,
		})
		return string(raw)
	}
	for n, ok := range map[int]bool{6: false, 9: false, 10: true, 12: true, 13: false} {
		if _, err := v.Validate(deck(n)); (err == nil) != ok {
			t.Errorf("%d slides: %v", n, err)
		}
	}
}

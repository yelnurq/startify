package evaluation

import (
	"context"
	"encoding/json"
	"strings"
	"testing"

	"github.com/yelnurq/startify/backend/internal/ai/llm"
	"github.com/yelnurq/startify/backend/internal/ai/mock"
	"github.com/yelnurq/startify/backend/internal/ai/schema"
	"github.com/yelnurq/startify/backend/internal/incubator"
)

func TestRubric(t *testing.T) {
	if err := DefaultRubric.Validate(); err != nil {
		t.Fatal(err)
	}
	custom := WithWeights(map[string]int{"problem": 20, "impact": 5})
	if err := custom.Validate(); err != nil {
		t.Fatal(err)
	}
	broken := WithWeights(map[string]int{"problem": 30})
	if err := broken.Validate(); err == nil || !strings.Contains(err.Error(), "115") {
		t.Fatalf("weights must add up to 100: %v", err)
	}
	all := map[string]int{}
	for _, c := range DefaultRubric {
		all[c.Key] = 10
	}
	if DefaultRubric.Total(all) != 100 || DefaultRubric.Total(map[string]int{}) != 0 {
		t.Fatal("total")
	}
	all["problem"] = 5
	if DefaultRubric.Total(all) != 93 { // 100 - 15*5/10 = 92.5, rounded half up
		t.Fatal(DefaultRubric.Total(all))
	}
	for total, level := range map[int]string{0: LevelIdea, 34: LevelIdea, 35: LevelValidatedProblem, 55: LevelMVPReady, 75: LevelInvestmentReady, 100: LevelInvestmentReady} {
		if Level(total) != level {
			t.Errorf("%d: %s", total, Level(total))
		}
	}
}

func evaluate(t *testing.T, steps map[string]incubator.Values, industry, locale string) Result {
	t.Helper()
	RegisterMock()
	t.Cleanup(func() { mock.Register(llm.FeatureEvaluation, nil) })
	data, _ := json.Marshal(map[string]any{"project": map[string]any{"title": "X", "industry": industry}, "steps": steps})
	s := resultSchema(DefaultRubric)
	res, err := (&mock.Provider{}).Complete(context.Background(), llm.CompletionRequest{
		System:     "The project is between the <project_data> tags.\n<project_data>\n" + string(data) + "\n</project_data>",
		JSONSchema: &s,
		Messages:   []llm.Message{{Role: llm.RoleUser, Content: "Evaluate."}},
		Metadata:   llm.RequestMeta{Feature: llm.FeatureEvaluation, PromptVersion: "evaluation@2", Locale: locale},
	})
	if err != nil {
		t.Fatal(err)
	}
	v, err := schema.Compile(s.Name, s.JSON)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := v.Validate(res.Text); err != nil {
		t.Fatalf("%v\n%s", err, res.Text)
	}
	var out Result
	if err := json.Unmarshal([]byte(res.Text), &out); err != nil {
		t.Fatal(err)
	}
	return out
}

func scoresOf(r Result) map[string]int {
	out := map[string]int{}
	for _, c := range r.Criteria {
		out[c.Key] = c.Score
	}
	return out
}

func TestMockEvaluationIsStrictAndConsistent(t *testing.T) {
	empty := evaluate(t, nil, "other", "ru")
	if total := DefaultRubric.Total(scoresOf(empty)); total > 10 || Level(total) != LevelIdea {
		t.Fatalf("an empty project scores %d", total)
	}
	if len(empty.Hypotheses) != 3 || len(empty.Recommendations) != 3 || len(empty.RedFlags) != 1 {
		t.Fatalf("%+v", empty)
	}

	weak := map[string]incubator.Values{
		"problem": {"statement": "Студентам трудно найти свободную аудиторию для самоподготовки.", "who": "Студенты"},
	}
	strong := map[string]incubator.Values{
		"problem": {
			"statement":    "Фермеры Северного Казахстана теряют до 20% урожая пшеницы, потому что замечают болезни слишком поздно, когда обработка уже не помогает.",
			"who":          "Хозяйства Костанайской и Акмолинской областей с полями от 100 до 500 гектаров, около 4 тысяч хозяйств по нашей оценке.",
			"alternatives": "Агроном объезжает поля раз в неделю; дроны и спутниковый мониторинг слишком дороги для небольших хозяйств.",
			"cost":         "В плохой год потери доходят до 45 000 ₸ с гектара; болезнь распространяется за 5–7 дней.",
			"evidence":     "Провели 14 интервью с фермерами в Костанайской области: 11 из 14 назвали позднее обнаружение болезней главной проблемой сезона.",
		},
		"unit_economics": {"unit": "Хозяйство в сезон", "price": 60000.0, "variable_cost": 8000.0, "cac": 15000.0, "purchases": 4.0, "assumptions": "Цена из интервью с 14 фермерами, CAC — по стоимости выездной демонстрации в районе."},
		"team": {"members": []any{
			map[string]any{"name": "Нурлан", "role": "CEO", "commitment": "full_time"},
			map[string]any{"name": "Дана", "role": "ML", "commitment": "full_time"},
			map[string]any{"name": "Арман", "role": "Продажи", "commitment": "part_time"},
		}, "why_us": "Двое из нас выросли на фермах и уже собрали датасет из 3 000 фотографий листьев."},
	}
	w, s := scoresOf(evaluate(t, weak, "agritech", "ru")), scoresOf(evaluate(t, strong, "agritech", "ru"))
	// The problem criterion also covers the audience and the journey, empty here.
	if w["problem"] >= s["problem"] || s["problem"] < 6 || s["team"] < 8 || s["business_model"] <= w["business_model"] || w["impact"] > 5 {
		t.Fatalf("weak %v strong %v", w, s)
	}

	// Evidence and confirmed hypotheses only raise the score.
	more := map[string]incubator.Values{}
	for k, v := range strong {
		more[k] = v
	}
	more["metrics"] = incubator.Values{"traction": "Весной провели пилот в двух хозяйствах: 1 200 гектаров под наблюдением, 3 вспышки найдены раньше агронома."}
	if m := DefaultRubric.Total(scoresOf(evaluate(t, more, "agritech", "ru"))); m <= DefaultRubric.Total(s) {
		t.Fatalf("traction did not raise the score: %d <= %d", m, DefaultRubric.Total(s))
	}

	kk := evaluate(t, strong, "agritech", "kk")
	if !strings.ContainsAny(kk.Summary, "әғқңөұүі") || !strings.Contains(kk.Criteria[0].Justification, "Мәселе түсінікті") {
		t.Fatalf("kk: %+v", kk)
	}
}

// A model that wraps its JSON in a fence or a sentence is still understood,
// like by the gateway's schema check (С-16).
func TestParseAcceptsFencedJSON(t *testing.T) {
	raw, err := json.Marshal(MockResult("{}", "ru"))
	if err != nil {
		t.Fatal(err)
	}
	for _, text := range []string{string(raw), "```json\n" + string(raw) + "\n```", "Вот оценка:\n" + string(raw)} {
		if _, scores, err := Parse(DefaultRubric, text); err != nil || len(scores) != len(DefaultRubric) {
			t.Fatalf("%q: %v", text[:min(len(text), 20)], err)
		}
	}
	if _, _, err := Parse(DefaultRubric, `{"criteria": []}`); err == nil {
		t.Fatal("an answer without the rubric must be rejected")
	}
}

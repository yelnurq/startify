package evaluation_test

import (
	"context"
	"testing"

	"github.com/yelnurq/startify/backend/internal/ai/llm"
	"github.com/yelnurq/startify/backend/internal/ai/mock"
	"github.com/yelnurq/startify/backend/internal/evaluation"
	"github.com/yelnurq/startify/backend/internal/incubator"
	"github.com/yelnurq/startify/backend/internal/seed"
)

// TestGoldenCasesWithTheMock is `make ai-eval` on the mock provider.
func TestGoldenCasesWithTheMock(t *testing.T) {
	evaluation.RegisterMock()
	t.Cleanup(func() { mock.Register(llm.FeatureEvaluation, nil) })
	cases, err := evaluation.LoadCases("testdata/cases.json", func(title string) (map[string]incubator.Values, bool) {
		for i := range seed.KazUTBProjects {
			if p := &seed.KazUTBProjects[i]; p.Title == title {
				out := map[string]incubator.Values{}
				for k, v := range p.Steps {
					out[k] = v
				}
				return out, true
			}
		}
		return nil, false
	})
	if err != nil {
		t.Fatal(err)
	}
	if len(cases) < 6 {
		t.Fatalf("%d cases", len(cases))
	}
	for i := range cases {
		out := evaluation.RunCase(context.Background(), &mock.Provider{}, "", &cases[i])
		if len(out.Problems) > 0 {
			t.Errorf("%s: %v", out.Case, out.Problems)
		}
	}
	if _, err := evaluation.LoadCases("testdata/cases.json", func(string) (map[string]incubator.Values, bool) { return nil, false }); err == nil {
		t.Fatal("an unknown seed project must fail")
	}
}

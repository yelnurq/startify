package evaluation_test

import (
	"context"
	"encoding/json"
	"io"
	"log/slog"
	"strings"
	"testing"

	"github.com/yelnurq/startify/backend/internal/ai/llm"
	"github.com/yelnurq/startify/backend/internal/ai/mock"
	"github.com/yelnurq/startify/backend/internal/evaluation"
	"github.com/yelnurq/startify/backend/internal/incubator"
	"github.com/yelnurq/startify/backend/internal/platform/config"
	"github.com/yelnurq/startify/backend/internal/seed"
)

func seedSteps(title string) (map[string]incubator.Values, bool) {
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
}

// TestGoldenCasesWithTheMock is `make ai-eval` on the mock provider, through
// the gateway like the worker. The fenced variant answers like many local
// models, with the JSON inside a Markdown fence: evaluations must still be
// read (С-16).
func TestGoldenCasesWithTheMock(t *testing.T) {
	evaluation.RegisterMock()
	t.Cleanup(func() { mock.Register(llm.FeatureEvaluation, nil) })
	cases, err := evaluation.LoadCases("testdata/cases.json", seedSteps)
	if err != nil {
		t.Fatal(err)
	}
	if len(cases) < 20 {
		t.Fatalf("%d cases, the harness needs at least 20", len(cases))
	}
	flagged, ranked := 0, 0
	for i := range cases {
		if cases[i].Expect.RedFlags != nil {
			flagged++
		}
		ranked += len(cases[i].Expect.Above)
	}
	if flagged < 5 || ranked < 10 {
		t.Fatalf("%d cases check red flags and %d pairs the ranking: too few", flagged, ranked)
	}
	providers := []llm.Provider{&mock.Provider{}, &evaluation.FencedMock{}}
	log := slog.New(slog.NewTextHandler(io.Discard, nil))
	for _, provider := range []string{"mock", "mock-fenced"} {
		g, err := evaluation.HarnessGateway(config.AI{Default: "mock", Timezone: "Asia/Almaty"}, providers, provider, "", log)
		if err != nil {
			t.Fatal(err)
		}
		outcomes := make([]evaluation.Outcome, len(cases))
		for i := range cases {
			outcomes[i] = evaluation.RunCase(context.Background(), g, &cases[i])
		}
		evaluation.Rank(cases, outcomes)
		for _, out := range outcomes {
			if len(out.Problems) > 0 {
				t.Errorf("%s / %s: %v", provider, out.Case, out.Problems)
			}
		}
	}
	if _, err := evaluation.LoadCases("testdata/cases.json", func(string) (map[string]incubator.Values, bool) { return nil, false }); err == nil {
		t.Fatal("an unknown seed project must fail")
	}
}

// The harness itself must catch a reviewer that misses red flags or ranks
// projects wrongly, not only pass a good one.
func TestHarnessCatchesABadReviewer(t *testing.T) {
	mock.Register(llm.FeatureEvaluation, func(llm.CompletionRequest) string {
		// Every project gets the same middling review without red flags.
		r := evaluation.MockResult("{}", "ru")
		for i := range r.Criteria {
			r.Criteria[i].Score = 5
		}
		r.RedFlags = []evaluation.RedFlag{}
		raw, _ := json.Marshal(r)
		return string(raw)
	})
	t.Cleanup(func() { mock.Register(llm.FeatureEvaluation, nil) })
	cases, err := evaluation.LoadCases("testdata/cases.json", seedSteps)
	if err != nil {
		t.Fatal(err)
	}
	g, err := evaluation.HarnessGateway(config.AI{Default: "mock", Timezone: "Asia/Almaty"}, []llm.Provider{&mock.Provider{}}, "mock", "",
		slog.New(slog.NewTextHandler(io.Discard, nil)))
	if err != nil {
		t.Fatal(err)
	}
	outcomes := make([]evaluation.Outcome, len(cases))
	for i := range cases {
		outcomes[i] = evaluation.RunCase(context.Background(), g, &cases[i])
	}
	evaluation.Rank(cases, outcomes)
	var all []string
	for _, out := range outcomes {
		all = append(all, out.Problems...)
	}
	text := strings.Join(all, "\n")
	for _, want := range []string{"no red flag names", "no red flag of high severity", "is not above", "is below"} {
		if !strings.Contains(text, want) {
			t.Errorf("the harness missed %q:\n%s", want, text)
		}
	}
}

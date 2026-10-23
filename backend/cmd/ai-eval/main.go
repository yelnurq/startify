// Command ai-eval runs the evaluation prompt on golden projects and checks
// that the model scores them like a sound reviewer: the answer follows the
// schema, empty and vague projects stay low, complete projects score high and
// above their weaker versions, red flags are found, instructions hidden in the
// project are ignored, the language is right.
//
// Calls go through the AI gateway exactly as in the worker (routes, schema
// check with a corrective retry, parsing), without a database. The model is
// chosen like in the platform (QADAM_AI_DEFAULT, QADAM_AI_ROUTES for the
// "evaluation" feature, provider keys); without settings the deterministic
// mock is evaluated, once as is and once wrapping its JSON in a Markdown
// fence as many local models do. Run it with `make ai-eval`.
package main

import (
	"context"
	"flag"
	"fmt"
	"log/slog"
	"os"
	"strings"
	"text/tabwriter"
	"time"

	"github.com/yelnurq/startify/backend/internal/ai"
	"github.com/yelnurq/startify/backend/internal/ai/llm"
	"github.com/yelnurq/startify/backend/internal/evaluation"
	"github.com/yelnurq/startify/backend/internal/incubator"
	"github.com/yelnurq/startify/backend/internal/platform/config"
	"github.com/yelnurq/startify/backend/internal/seed"
)

func main() {
	if err := run(); err != nil {
		fmt.Fprintln(os.Stderr, "ai-eval:", err)
		os.Exit(1)
	}
}

func seedSteps(title string) (map[string]incubator.Values, bool) {
	for i := range seed.KazUTBProjects {
		p := &seed.KazUTBProjects[i]
		if p.Title == title {
			out := map[string]incubator.Values{}
			for k, v := range p.Steps {
				out[k] = v
			}
			return out, true
		}
	}
	return nil, false
}

func run() error {
	cases := flag.String("cases", "internal/evaluation/testdata/cases.json", "golden cases")
	only := flag.String("provider", "", "provider to evaluate instead of the configured route")
	model := flag.String("model", "", "model instead of the provider's default")
	flag.Parse()

	cfg, err := config.AIFromEnviron(os.Environ())
	if err != nil {
		return err
	}
	evaluation.RegisterMock()
	providers := append(ai.ConfiguredProviders(cfg, 0), &evaluation.FencedMock{})
	routes, err := ai.ParseRoutes(cfg.Routes)
	if err != nil {
		return err
	}
	name, chosen := cfg.Default, *model
	if r := routes[llm.FeatureEvaluation]; len(r) > 0 {
		name = r[0].Provider
		if chosen == "" {
			chosen = r[0].Model
		}
	}
	if *only != "" {
		name = *only
	}
	variants := []string{name}
	if name == "mock" {
		variants = append(variants, "mock-fenced")
	}
	list, err := evaluation.LoadCases(*cases, seedSteps)
	if err != nil {
		return err
	}
	log := slog.New(slog.NewTextHandler(os.Stderr, &slog.HandlerOptions{Level: slog.LevelError}))

	failed, total := 0, 0
	for _, variant := range variants {
		g, err := evaluation.HarnessGateway(cfg, providers, variant, chosen, log)
		if err != nil {
			return err
		}
		label := variant
		if chosen != "" {
			label += " / " + chosen
		}
		fmt.Printf("Evaluating %d cases with %s\n\n", len(list), label)
		outcomes := make([]evaluation.Outcome, len(list))
		for i := range list {
			ctx, cancel := context.WithTimeout(context.Background(), 5*time.Minute)
			outcomes[i] = evaluation.RunCase(ctx, g, &list[i])
			cancel()
		}
		evaluation.Rank(list, outcomes)
		w := tabwriter.NewWriter(os.Stdout, 0, 0, 2, ' ', 0)
		fmt.Fprintln(w, "CASE\tTOTAL\tLEVEL\tRESULT")
		for _, out := range outcomes {
			total++
			result := "ok"
			if len(out.Problems) > 0 {
				failed++
				result = "FAIL: " + strings.Join(out.Problems, "; ")
			}
			fmt.Fprintf(w, "%s\t%d\t%s\t%s\n", out.Case, out.Total, out.Level, result)
		}
		if err := w.Flush(); err != nil {
			return err
		}
		fmt.Println()
	}
	if failed > 0 {
		return fmt.Errorf("%d of %d cases failed", failed, total)
	}
	fmt.Println("All cases passed.")
	return nil
}

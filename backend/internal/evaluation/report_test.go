package evaluation

import (
	"bytes"
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/yelnurq/startify/backend/internal/apigen"
	"github.com/yelnurq/startify/backend/internal/platform/gotenberg"
)

func sampleReport(lang string) reportData {
	data := `{"project":{"title":"AgroScan","industry":"agritech"},"steps":{"problem":{"statement":"Фермеры Северного Казахстана теряют до 20% урожая пшеницы, потому что замечают болезни слишком поздно.","who":"Хозяйства Костанайской области с полями от 100 до 500 га."}}}`
	r := MockResult(data, lang)
	scores := map[string]int{}
	d := reportData{
		Lang: lang, L: reportLabels[lang], Color: "#00A3C4", University: "КазУТБ", Title: "AgroScan <b>", Summary: r.Summary,
		Hypotheses: r.Hypotheses, Recommends: r.Recommendations,
	}
	for i, c := range r.Criteria {
		scores[c.Key] = c.Score
		d.Criteria = append(d.Criteria, reportCriterion{
			Title: DefaultRubric[i].Title.Best(lang), Weight: DefaultRubric[i].Weight, Score: c.Score, AIScore: max(c.Score-2, 0),
			Overridden: i == 0, Justification: c.Justification,
		})
	}
	for _, f := range r.RedFlags {
		d.RedFlags = append(d.RedFlags, apigen.RedFlag{Title: f.Title, Detail: f.Detail, Severity: apigen.RedFlagSeverity(f.Severity)})
	}
	for _, e := range r.Experiments {
		d.Experiments = append(d.Experiments, apigen.Experiment{Title: e.Title, How: e.How, Metric: e.Metric, Success: e.Success, Day: e.Day})
	}
	d.Total = DefaultRubric.Total(scores)
	d.Level = LevelNames[Level(d.Total)][lang]
	prev := d.Total - 7
	d.PreviousTotal, d.Delta = &prev, "+7"
	d.Disclaimer = "mock-evaluation · evaluation@2"
	return d
}

func TestReportRenders(t *testing.T) {
	var buf bytes.Buffer
	if err := reportTemplate.ExecuteTemplate(&buf, "report.html", sampleReport("ru")); err != nil {
		t.Fatal(err)
	}
	html := buf.String()
	if !strings.Contains(html, "AI-оценка проекта") || strings.Contains(html, "AgroScan <b>") || !strings.Contains(html, "Скорректировано экспертом") {
		t.Fatal(html)
	}
}

// TestReportWithGotenberg converts the report with a real Gotenberg
// (QADAM_GOTENBERG_URL) and writes it to QADAM_RENDER_DIR for a visual check.
func TestReportWithGotenberg(t *testing.T) {
	url := os.Getenv("QADAM_GOTENBERG_URL")
	if url == "" {
		t.Skip("QADAM_GOTENBERG_URL is not set")
	}
	c := gotenberg.New(url, time.Minute)
	for _, lang := range []string{"kk", "ru", "en"} {
		var buf bytes.Buffer
		if err := reportTemplate.ExecuteTemplate(&buf, "report.html", sampleReport(lang)); err != nil {
			t.Fatal(err)
		}
		pdf, err := c.HTMLToPDF(context.Background(), buf.Bytes(), gotenberg.Page{PreferCSSPageSize: true})
		if err != nil {
			t.Fatal(err)
		}
		if dir := os.Getenv("QADAM_RENDER_DIR"); dir != "" {
			_ = os.WriteFile(filepath.Join(dir, lang+"-evaluation.pdf"), pdf, 0o600)
		}
	}
}

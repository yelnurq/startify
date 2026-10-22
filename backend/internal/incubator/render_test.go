package incubator

import (
	"bytes"
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/yelnurq/startify/backend/internal/platform/gotenberg"
)

func sampleContent() (ProjectCard, PitchContent) {
	card := ProjectCard{Title: "AgroScan", Tagline: "Болезни пшеницы — по фото листа", Industry: "agritech"}
	steps := map[string]Values{
		"problem":  {"statement": "Фермеры Костанайской области теряют до 20% урожая пшеницы: болезни замечают слишком поздно.", "cost": "До 45 000 ₸ с гектара в плохой год."},
		"audience": {"segments": "Хозяйства с полями от 100 до 500 га.", "early_adopters": "Фермеры, уже работающие с агрономами-консультантами."},
		"solution": {"solution": "Мобильное приложение распознаёт болезнь по фото листа и советует обработку.", "uvp": "Диагноз за 10 секунд без агронома", "features": []any{"Фото с телефона", "Карта очагов", "Советы по обработке"}},
		"market_size": {
			"tam": 12_000_000_000.0, "sam": 1_800_000_000.0, "som": 90_000_000.0,
		},
		"competitors":    {"list": []any{map[string]any{"name": "Агроном-консультант"}, map[string]any{"name": "Plantix"}}, "difference": "Работаем без интернета в поле и на казахском языке."},
		"business_model": {"revenue": "Подписка хозяйства на сезон.", "pricing": "60 000 ₸ за сезон для хозяйства до 500 га."},
		"metrics":        {"north_star": "Гектары под наблюдением", "traction": "Пилот в двух хозяйствах весной."},
		"team":           {"members": []any{map[string]any{"name": "Нұрлан Жұмабеков", "role": "CEO"}, map[string]any{"name": "Дана Ахметова", "role": "Data scientist"}}},
		"pitch":          {"ask": "Ищем пилот в двух хозяйствах и грант 3 000 000 ₸.", "contact": "agroscan.team@qadam.test"},
	}
	return card, assemble(promptData{Steps: steps, Project: struct {
		Title    string `json:"title"`
		Tagline  string `json:"tagline"`
		Industry string `json:"industry"`
	}{card.Title, card.Tagline, card.Industry}}, "ru")
}

func TestDocumentsRender(t *testing.T) {
	card, content := sampleContent()
	content.Problem += " <script>alert(1)</script>"
	brand := Brand{University: "КазУТБ", Color: "#00A3C4"}
	d := newDocData(card, content, brand, "ru", time.Date(2026, 10, 22, 12, 0, 0, 0, time.UTC))
	for _, name := range []string{"one_pager.html", "deck.html"} {
		html, err := render(name, d)
		if err != nil {
			t.Fatal(err)
		}
		s := string(html)
		if !strings.Contains(s, "AgroScan") || strings.Contains(s, "<script>alert") || !strings.Contains(s, "22 октября 2026.") && name == "one_pager.html" {
			t.Fatalf("%s:\n%s", name, s)
		}
	}
	w := &MaterialWorker{}
	pd := d
	if err := w.passport(&pd, MaterialArgs{Origin: "https://kazutb.qadam.kz"}, nil); err != nil {
		t.Fatal(err)
	}
	html, err := render("passport.html", pd)
	if err != nil {
		t.Fatal(err)
	}
	if s := string(html); !strings.Contains(s, "<svg") || !strings.Contains(s, "0 из 14 шагов") || !strings.Contains(s, "Паспорт стартапа") {
		t.Fatalf("passport:\n%s", s)
	}
}

// TestDocumentsWithGotenberg converts the documents with a real Gotenberg
// (QADAM_GOTENBERG_URL, e.g. http://localhost:3300 of `make dev`) and writes
// them to QADAM_RENDER_DIR for a visual check.
func TestDocumentsWithGotenberg(t *testing.T) {
	url := os.Getenv("QADAM_GOTENBERG_URL")
	if url == "" {
		t.Skip("QADAM_GOTENBERG_URL is not set")
	}
	dir := os.Getenv("QADAM_RENDER_DIR")
	c := gotenberg.New(url, time.Minute)
	ctx := context.Background()
	for _, locale := range []string{"ru", "kk", "en"} {
		card, content := sampleContent()
		brand := Brand{University: "КазУТБ", Color: "#00A3C4"}
		d := newDocData(card, content, brand, locale, time.Now())
		w := &MaterialWorker{Gotenberg: c}
		pd := d
		if err := w.passport(&pd, MaterialArgs{Origin: "https://kazutb.qadam.kz"}, nil); err != nil {
			t.Fatal(err)
		}
		docs := map[string]docData{"one_pager.html": d, "deck.html": d, "passport.html": pd}
		for name, data := range docs {
			pdf, err := w.pdf(ctx, name, data)
			if err != nil {
				t.Fatalf("%s: %v", name, err)
			}
			if !bytes.HasPrefix(pdf, []byte("%PDF")) {
				t.Fatalf("%s: not a PDF", name)
			}
			if dir != "" {
				_ = os.WriteFile(filepath.Join(dir, locale+"-"+strings.TrimSuffix(name, ".html")+".pdf"), pdf, 0o600)
			}
		}
		deck, err := BuildDeck(card.Title, d.Tagline, locale, content.Slides, brand)
		if err != nil {
			t.Fatal(err)
		}
		// LibreOffice must open the presentation.
		pdf, err := c.OfficeToPDF(ctx, "deck.pptx", deck)
		if err != nil {
			t.Fatalf("pptx: %v", err)
		}
		if dir != "" {
			_ = os.WriteFile(filepath.Join(dir, locale+"-deck.pptx"), deck, 0o600)
			_ = os.WriteFile(filepath.Join(dir, locale+"-deck-from-pptx.pdf"), pdf, 0o600)
		}
	}
}

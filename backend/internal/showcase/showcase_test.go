package showcase

import (
	"bytes"
	"image/png"
	"slices"
	"strings"
	"testing"
	"time"

	"github.com/yelnurq/startify/backend/internal/apigen"
	"github.com/yelnurq/startify/backend/internal/platform/apperr"
	"github.com/yelnurq/startify/backend/internal/showcase/showcasedb"
)

func TestSlugify(t *testing.T) {
	for title, want := range map[string]string{
		"AgroScan": "agroscan",
		"Kiiz Lab": "kiiz-lab",
		"Ауыл Мед — телемедицина":    "auyl-med-telemeditsina",
		"Қала сигналы":               "qala-signaly",
		"  ЕНТ: 100 баллов!  ":       "ent-100-ballov",
		"AI":                         "ai-project",
		"日本":                         "project",
		strings.Repeat("долго ", 20): "dolgo-dolgo-dolgo-dolgo-dolgo-dolgo-dolgo-dolgo-dolgo-dolgo",
	} {
		got := Slugify(title)
		if got != want || !slugPattern.MatchString(got) || len(got) > 60 {
			t.Errorf("Slugify(%q) = %q, want %q", title, got, want)
		}
	}
}

func TestTSQuery(t *testing.T) {
	for q, want := range map[string]string{
		"агро сканер":     "агро:* & сканер:*",
		"AI-ментор":       "ai:* & ментор:*",
		"' || ! & (x":     "",
		"a б":             "",
		"Қазақ тілі 2026": "қазақ:* & тілі:* & 2026:*",
	} {
		got := tsQuery(q)
		if (want == "") != (got == nil) || (got != nil && *got != want) {
			t.Errorf("tsQuery(%q) = %v, want %q", q, got, want)
		}
	}
}

func TestCheckPage(t *testing.T) {
	problem := "  Фермеры теряют урожай.\r\nКаждый год.  "
	in, err := check(apigen.ShowcaseEntryUpdate{
		Slug: "AgroScan", Title: " AgroScan ", Tagline: "Диагноз по фото",
		Sections: apigen.ShowcaseSections{Problem: &problem}, Needs: []apigen.ShowcaseNeed{"pilot", "pilot", "team"},
		Tags: []string{"Computer  Vision", "computer vision", "B2B"}, Visibility: apigen.ShowcaseVisibilityPublic,
	})
	if err != nil {
		t.Fatal(err)
	}
	if in.slug != "agroscan" || in.title != "AgroScan" || in.sections.Problem != "Фермеры теряют урожай.\nКаждый год." ||
		strings.Join(in.needs, ",") != "pilot,team" || strings.Join(in.tags, ",") != "computer vision,b2b" {
		t.Fatalf("checked %+v", in)
	}
	long := strings.Repeat("я", 1501)
	video := "https://rutube.ru/video/1"
	_, err = check(apigen.ShowcaseEntryUpdate{
		Slug: "a", Title: "A", Sections: apigen.ShowcaseSections{Solution: &long}, VideoUrl: &video,
		Needs: []apigen.ShowcaseNeed{"money"}, Visibility: "everyone",
	})
	ae, ok := apperr.As(err)
	if !ok {
		t.Fatalf("not a validation error: %v", err)
	}
	for _, field := range []string{"slug", "title", "sections.solution", "video_url", "needs", "visibility"} {
		if !slices.ContainsFunc(ae.Fields, func(f apperr.FieldError) bool { return f.Field == field }) {
			t.Errorf("no error for %s: %+v", field, ae.Fields)
		}
	}
}

func TestRenderMeta(t *testing.T) {
	shell := []byte("<!doctype html>\n<html lang=\"kk\">\n  <head>\n    <meta name=\"description\" content=\"Qadam\" />\n    <title>Qadam</title>\n  </head>\n  <body>\n    <div id=\"root\"></div>\n  </body>\n</html>\n")
	published := time.Date(2026, 10, 1, 9, 0, 0, 0, time.UTC)
	out := string(render(shell, meta{
		Lang: "ru", Title: `"Агро" <Скан>`, Description: "Фото & диагноз", Canonical: "https://kazutb.kz/showcase/agroscan",
		Image: "https://kazutb.kz/og.png", Kind: "article", Published: &published,
		JSONLD: map[string]any{"name": "</script><script>alert(1)</script>"}, Body: "<h1>AgroScan</h1>",
	}))
	for _, want := range []string{
		`<html lang="ru">`, `<title>&#34;Агро&#34; &lt;Скан&gt;</title>`, `<meta name="description" content="Фото &amp; диагноз" />`,
		`<meta property="og:locale" content="ru_RU" />`, `<meta property="article:published_time" content="2026-10-01T09:00:00Z" />`,
		`</script>`, `<div id="root"></div>` + "\n    <noscript><h1>AgroScan</h1></noscript>",
	} {
		if !strings.Contains(out, want) {
			t.Fatalf("no %s in\n%s", want, out)
		}
	}
	if strings.Count(out, `name="description"`) != 1 || strings.Contains(out, "</script><script>") {
		t.Fatalf("unsafe or duplicated:\n%s", out)
	}
}

func TestClip(t *testing.T) {
	if got := clip("короткий текст", 160); got != "короткий текст" {
		t.Fatal(got)
	}
	got := clip(strings.Repeat("слово ", 40), 50)
	if !strings.HasSuffix(got, "…") || len([]rune(got)) > 50 || strings.Contains(got, "  ") {
		t.Fatalf("%q", got)
	}
}

func TestProjectMetaDescribesThePage(t *testing.T) {
	published := time.Date(2026, 10, 1, 9, 0, 0, 0, time.UTC)
	e := showcasedb.ShowcaseEntry{
		Slug: "agroscan", Title: "AgroScan", Industry: "agritech", Version: 3, PublishedAt: &published, UpdatedAt: published,
		Sections: []byte(`{"problem":"Фермеры поздно замечают болезни пшеницы.","solution":"Фото листа — диагноз за 10 секунд."}`),
		Team:     []byte(`[{"name":"Нұрлан Ахметов","role":"CEO"}]`), Tags: []string{"ai", "b2b"},
	}
	m := projectMeta(&e, "kk", "https://kazutb.kz", "ҚазТБУ")
	ld, _ := m.JSONLD.(map[string]any)
	if m.Title != "AgroScan — ҚазТБУ витринасы" || m.Description != "Фермеры поздно замечают болезни пшеницы." ||
		m.Image != "https://kazutb.kz/api/v1/showcase/entries/agroscan/og.png?v=3" || ld["genre"] != "Агротех" || ld["keywords"] != "ai, b2b" ||
		!strings.Contains(m.Body, "<h2>Мәселе</h2>") {
		t.Fatalf("meta %+v", m)
	}
}

func TestPlainImage(t *testing.T) {
	out, err := plainImage("#7C3AED", nil)
	if err != nil {
		t.Fatal(err)
	}
	img, err := png.Decode(bytes.NewReader(out))
	if err != nil || img.Bounds().Dx() != ogWidth || img.Bounds().Dy() != ogHeight {
		t.Fatalf("%v %v", img.Bounds(), err)
	}
	if r, g, b, _ := img.At(5, 5).RGBA(); r>>8 != 0x7C || g>>8 != 0x3A || b>>8 != 0xED {
		t.Fatalf("brand stripe %x %x %x", r>>8, g>>8, b>>8)
	}
}

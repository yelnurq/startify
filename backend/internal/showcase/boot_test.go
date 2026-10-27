package showcase

import (
	"encoding/json"
	"io/fs"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"testing"

	"github.com/google/uuid"

	"github.com/yelnurq/startify/backend/internal/apiconv"
	"github.com/yelnurq/startify/backend/internal/apigen"
	"github.com/yelnurq/startify/backend/internal/showcase/showcasedb"
	"github.com/yelnurq/startify/backend/internal/tenant"
)

// frontendDir is the web client next to the backend in the repository; the
// tests that compare with it are skipped where only the backend is present.
func frontendDir(t *testing.T) string {
	t.Helper()
	dir := filepath.Join("..", "..", "..", "frontend", "src")
	if _, err := os.Stat(dir); err != nil {
		t.Skip("the web client's sources are not available")
	}
	return dir
}

func TestBootLocale(t *testing.T) {
	for _, tc := range []struct {
		cookie, accept, want string
	}{
		{"", "", "kk"},
		{"ru", "en-US,en;q=0.9", "ru"},
		{"xx", "en-US,en;q=0.9,ru;q=0.8", "en"},
		{"", "de-DE,de;q=0.9,ru;q=0.5", "ru"},
		{"", "kk-KZ", "kk"},
		{"", "fr-FR", "kk"},
		{"", "not a header;;;", "kk"},
	} {
		r := httptest.NewRequest(http.MethodGet, "/", http.NoBody)
		if tc.cookie != "" {
			r.AddCookie(&http.Cookie{Name: localeCookie, Value: tc.cookie})
		}
		if tc.accept != "" {
			r.Header.Set("Accept-Language", tc.accept)
		}
		if got := bootLocale(r); got != tc.want {
			t.Errorf("cookie %q, Accept-Language %q: %s, want %s", tc.cookie, tc.accept, got, tc.want)
		}
	}
}

// flatten turns a translation catalogue into "a.b.c" keys.
func flatten(prefix string, v any, out map[string]string) {
	switch x := v.(type) {
	case map[string]any:
		for k, child := range x {
			key := k
			if prefix != "" {
				key = prefix + "." + k
			}
			flatten(key, child, out)
		}
	case string:
		out[prefix] = x
	}
}

func catalogue(t *testing.T, dir, locale, ns string) map[string]string {
	t.Helper()
	raw, err := os.ReadFile(filepath.Join(dir, "locales", locale, ns+".json"))
	if err != nil {
		t.Fatal(err)
	}
	var tree any
	if err := json.Unmarshal(raw, &tree); err != nil {
		t.Fatal(err)
	}
	out := map[string]string{}
	flatten("", tree, out)
	return out
}

// The first screen and the search engine texts say exactly what the web
// client says.
func TestTextsMatchTheWebClient(t *testing.T) {
	dir := frontendDir(t)
	same := func(name string, ours apiconv.Text, ns, key string) {
		for _, locale := range bootLocales {
			theirs, ok := catalogue(t, dir, locale, ns)[key]
			if !ok {
				t.Errorf("%s: %s:%s is missing in the %s catalogue", name, ns, key, locale)
				continue
			}
			theirs = strings.ReplaceAll(theirs, "{{university}}", "{university}")
			theirs = strings.ReplaceAll(theirs, "{{title}}", "{title}")
			if ours[locale] != theirs {
				t.Errorf("%s (%s): %q, the web client says %q", name, locale, ours[locale], theirs)
			}
		}
	}
	for key, text := range bootText {
		ns, k, _ := strings.Cut(key, ":")
		mine := apiconv.Text{}
		for l, v := range text {
			mine[l] = strings.ReplaceAll(v, "{{university}}", "{university}")
		}
		same(key, mine, ns, k)
	}
	same("catalogTitle", catalogTitle, "showcase", "meta.catalogTitle")
	same("catalogDescription", catalogDescription, "showcase", "meta.catalogDescription")
	same("nationalTitle", nationalTitle, "showcase", "meta.nationalTitle")
	same("nationalDescription", nationalDescription, "showcase", "meta.nationalDescription")
	same("projectTitle", projectTitle, "showcase", "meta.projectTitle")
	for key, text := range industryLabels {
		same("industry "+key, text, "showcase", "industries."+key)
	}
	for key, text := range stageLabels {
		same("stage "+key, text, "showcase", "stages."+key)
	}
}

var classAttr = regexp.MustCompile(`class="([^"]*)"`)

// Tailwind builds rules only for classes found in the client's sources: a
// class used by the first screen alone would have no style.
func TestBootClassesExistInTheWebClient(t *testing.T) {
	dir := frontendDir(t)
	var sources strings.Builder
	err := filepath.WalkDir(dir, func(path string, d fs.DirEntry, err error) error {
		if err != nil || d.IsDir() || strings.HasSuffix(path, ".test.tsx") {
			return err
		}
		if ext := filepath.Ext(path); ext == ".tsx" || ext == ".ts" || ext == ".css" {
			raw, err := os.ReadFile(path)
			if err != nil {
				return err
			}
			sources.Write(raw)
		}
		return nil
	})
	if err != nil {
		t.Fatal(err)
	}
	all := sources.String()
	classes := map[string]bool{}
	for _, m := range classAttr.FindAllStringSubmatch(bootHTML, -1) {
		for _, c := range strings.Fields(m[1]) {
			if !strings.Contains(c, "{{") {
				classes[c] = true
			}
		}
	}
	for _, g := range coverGradients {
		for _, c := range strings.Fields(g) {
			classes[c] = true
		}
	}
	if len(classes) < 50 {
		t.Fatalf("found only %d classes", len(classes))
	}
	for c := range classes {
		if !strings.Contains(all, c) {
			t.Errorf("class %q of the first screen does not appear in the web client", c)
		}
	}
}

func TestRenderBoot(t *testing.T) {
	u := apigen.UniversityBrief{
		Name:      apiconv.Text{"kk": "Қ. Құлажанов атындағы ҚазТБУ", "ru": "КазУТБ имени К. Кулажанова"}.API(),
		ShortName: apiconv.Text{"kk": "ҚазТБУ", "ru": "КазУТБ"}.API(),
	}
	coverURL, sealURL := "/api/v1/files/cover/content", "/api/v1/files/seal/content"
	u.LogoUrl = &sealURL
	landing := landingBoot(tenant.University{Brief: u, CoverURL: &coverURL}, "ru")
	out, err := renderBoot(&landing)
	if err != nil {
		t.Fatal(err)
	}
	for _, want := range []string{
		`<div id="boot" lang="ru"`, "КазУТБ имени К. Кулажанова", "От идеи до рынка — <em", "один шаг",
		`<img src="/api/v1/files/seal/content"`, `<img src="/api/v1/files/cover/content"`, "Начать проект", "Смотреть витрину",
		`aria-label="Маршрут проекта"`, `tracking-label text-primary-300">06</span>`, "Пилоты и инвестиции",
	} {
		if !strings.Contains(out, want) {
			t.Errorf("landing: no %q in\n%s", want, out)
		}
	}

	catalog := catalogBoot(u, "kk", false)
	if out, _ = renderBoot(&catalog); !strings.Contains(out, "ҚазТБУ студенттік стартаптары") || !strings.Contains(out, "Серіктес іздеп жүрген жобалар") {
		t.Errorf("catalogue:\n%s", out)
	}
	national := catalogBoot(u, "en", true)
	if out, _ = renderBoot(&national); !strings.Contains(out, "Projects of Kazakhstan&#39;s universities") {
		t.Errorf("national catalogue:\n%s", out)
	}

	cover := uuid.New()
	entry := showcasedb.ShowcaseEntry{Title: `Agro<script>`, Tagline: "Wheat & rust", Industry: "agritech", Stage: "launch"}
	project := projectBoot(&entry, u, "ru")
	out, _ = renderBoot(&project)
	for _, want := range []string{"Agro&lt;script&gt;", "Wheat &amp; rust", "Агротех", "Запуск", coverGradients[2], `href="/showcase">Витрина`} {
		if !strings.Contains(out, want) {
			t.Errorf("project: no %q in\n%s", want, out)
		}
	}
	entry.CoverFileID = &cover
	project = projectBoot(&entry, u, "ru")
	if out, _ = renderBoot(&project); !strings.Contains(out, `<img src="/api/v1/files/`+cover.String()+`/content"`) {
		t.Errorf("project with a cover:\n%s", out)
	}
}

// The browser tab of a university's page shows its seal instead of the
// platform's icon.
func TestRenderPutsTheSealInTheTab(t *testing.T) {
	shell := []byte(`<html lang="kk"><head><link rel="icon" type="image/svg+xml" href="/favicon.svg" /><title>Qadam</title></head><body><div id="root"></div></body></html>`)
	page := string(render(shell, meta{Lang: "ru", Title: "Витрина", Icon: "/api/v1/files/seal/content"}))
	if strings.Contains(page, "favicon.svg") || strings.Count(page, `href="/api/v1/files/seal/content"`) != 2 {
		t.Fatalf("icons: %s", page)
	}
	if page = string(render(shell, meta{Lang: "ru", Title: "Витрина"})); !strings.Contains(page, "favicon.svg") {
		t.Fatalf("without a seal the platform icon stays: %s", page)
	}
}

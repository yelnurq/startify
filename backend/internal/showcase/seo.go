package showcase

import (
	"context"
	"encoding/json"
	"encoding/xml"
	"errors"
	"fmt"
	"html"
	"io"
	"log/slog"
	"net/http"
	"regexp"
	"strings"
	"sync"
	"time"
	"unicode/utf8"

	"github.com/go-chi/chi/v5"
	"github.com/google/uuid"

	"github.com/yelnurq/startify/backend/internal/apiconv"
	"github.com/yelnurq/startify/backend/internal/apigen"
	"github.com/yelnurq/startify/backend/internal/platform/apperr"
	"github.com/yelnurq/startify/backend/internal/platform/httpx"
	"github.com/yelnurq/startify/backend/internal/platform/tenancy"
	"github.com/yelnurq/startify/backend/internal/showcase/showcasedb"
	"github.com/yelnurq/startify/backend/internal/tenant"
)

const (
	shellMaxBytes = 1 << 20
	siteName      = "Qadam"
	catalogItems  = 24
)

// spaCSP is the policy of the web client (deploy/nginx/security-headers.conf):
// pages built here run the same application.
const spaCSP = "default-src 'self'; script-src 'self'; style-src 'self' 'unsafe-inline'; img-src 'self' data: blob:; " +
	"font-src 'self'; connect-src 'self'; media-src 'self' blob:; frame-src https://www.youtube-nocookie.com https://player.vimeo.com; " +
	"frame-ancestors 'none'; base-uri 'self'; form-action 'self'; object-src 'none'"

// fallbackShell is served when the web client cannot be reached: crawlers
// still get the page description.
const fallbackShell = `<!doctype html>
<html lang="kk">
  <head>
    <meta charset="UTF-8" />
    <meta name="viewport" content="width=device-width, initial-scale=1.0" />
    <title>Qadam</title>
  </head>
  <body>
    <div id="root"></div>
  </body>
</html>
`

// shell keeps the web client's index.html that search engine pages are
// built on. Every page asks the web server again with the copy's ETag (a
// cheap 304 while nothing changed): a release replaces the hashed asset
// files at once, and a stale copy would point at scripts that are gone.
type shell struct {
	url    string
	client *http.Client

	mu   sync.Mutex
	html []byte
	etag string
}

func newShell(url string) *shell {
	return &shell{url: url, client: &http.Client{Timeout: 3 * time.Second}}
}

func (s *shell) get(ctx context.Context) []byte {
	if s.url == "" {
		return []byte(fallbackShell)
	}
	s.mu.Lock()
	cached, etag := s.html, s.etag
	s.mu.Unlock()
	fresh, freshTag, err := s.fetch(ctx, etag)
	switch {
	case err != nil:
		slog.WarnContext(ctx, "showcase: web client shell unavailable", slog.Any("error", err))
		if cached != nil {
			return cached
		}
		return []byte(fallbackShell)
	case fresh == nil:
		return cached
	}
	s.mu.Lock()
	s.html, s.etag = fresh, freshTag
	s.mu.Unlock()
	return fresh
}

// fetch returns the page and its ETag, or nil when it is still etag.
func (s *shell) fetch(ctx context.Context, etag string) ([]byte, string, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, s.url, http.NoBody)
	if err != nil {
		return nil, "", err
	}
	if etag != "" {
		req.Header.Set("If-None-Match", etag)
	}
	res, err := s.client.Do(req)
	if err != nil {
		return nil, "", err
	}
	defer func() { _ = res.Body.Close() }()
	if res.StatusCode == http.StatusNotModified && etag != "" {
		return nil, etag, nil
	}
	if res.StatusCode != http.StatusOK {
		return nil, "", fmt.Errorf("status %s", res.Status)
	}
	body, err := io.ReadAll(io.LimitReader(res.Body, shellMaxBytes))
	if err != nil {
		return nil, "", err
	}
	if !strings.Contains(string(body), "</head>") {
		return nil, "", errors.New("not an html page")
	}
	return body, res.Header.Get("ETag"), nil
}

// meta describes a page to search engines and social networks.
type meta struct {
	Lang        string
	Title       string
	Description string
	Canonical   string
	Image       string
	ImageAlt    string
	// Kind is the Open Graph type: website or article.
	Kind      string
	NoIndex   bool
	Published *time.Time
	Modified  *time.Time
	JSONLD    any
	// Body is the escaped HTML shown to clients without JavaScript.
	Body string
	// Boot is the first screen drawn before the web client loads.
	Boot string
}

var (
	langAttr = regexp.MustCompile(`(<html[^>]*\slang=")[^"]*(")`)
	titleTag = regexp.MustCompile(`(?s)<title>.*?</title>`)
	descTag  = regexp.MustCompile(`(?i)<meta\s+name="description"[^>]*>\s*`)
)

var ogLocales = map[string]string{"kk": "kk_KZ", "ru": "ru_RU", "en": "en_US"}

// render puts the description of a page into the shell.
func render(shell []byte, m meta) []byte {
	esc := html.EscapeString
	var head strings.Builder
	tag := func(attr, key, value string) {
		if value != "" {
			fmt.Fprintf(&head, "    <meta %s=\"%s\" content=\"%s\" />\n", attr, key, esc(value))
		}
	}
	tag("name", "description", m.Description)
	if m.NoIndex {
		tag("name", "robots", "noindex")
	}
	if m.Canonical != "" {
		fmt.Fprintf(&head, "    <link rel=\"canonical\" href=\"%s\" />\n", esc(m.Canonical))
	}
	tag("property", "og:type", m.Kind)
	tag("property", "og:site_name", siteName)
	tag("property", "og:locale", ogLocales[m.Lang])
	tag("property", "og:title", m.Title)
	tag("property", "og:description", m.Description)
	tag("property", "og:url", m.Canonical)
	card := "summary"
	if m.Image != "" {
		card = "summary_large_image"
		tag("property", "og:image", m.Image)
		tag("property", "og:image:width", "1200")
		tag("property", "og:image:height", "630")
		tag("property", "og:image:alt", m.ImageAlt)
	}
	if m.Published != nil {
		tag("property", "article:published_time", m.Published.UTC().Format(time.RFC3339))
	}
	if m.Modified != nil {
		tag("property", "article:modified_time", m.Modified.UTC().Format(time.RFC3339))
	}
	tag("name", "twitter:card", card)
	tag("name", "twitter:title", m.Title)
	tag("name", "twitter:description", m.Description)
	tag("name", "twitter:image", m.Image)
	if m.JSONLD != nil {
		// encoding/json escapes <, > and &, so the data cannot close the script.
		if raw, err := json.Marshal(m.JSONLD); err == nil {
			fmt.Fprintf(&head, "    <script type=\"application/ld+json\">%s</script>\n", raw)
		}
	}

	out := string(shell)
	out = langAttr.ReplaceAllString(out, "${1}"+esc(m.Lang)+"${2}")
	out = descTag.ReplaceAllLiteralString(out, "")
	out = titleTag.ReplaceAllLiteralString(out, "<title>"+esc(m.Title)+"</title>")
	out = strings.Replace(out, "</head>", head.String()+"  </head>", 1)
	if m.Boot != "" {
		out = strings.Replace(out, `<div id="root"></div>`, m.Boot+"\n    "+`<div id="root"></div>`, 1)
	}
	if m.Body != "" {
		noscript := "<noscript>" + m.Body + "</noscript>"
		if strings.Contains(out, `<div id="root"></div>`) {
			out = strings.Replace(out, `<div id="root"></div>`, `<div id="root"></div>`+"\n    "+noscript, 1)
		} else {
			out = strings.Replace(out, "</body>", noscript+"\n  </body>", 1)
		}
	}
	return []byte(out)
}

// clip shortens a description for search results.
func clip(s string, n int) string {
	s = strings.Join(strings.Fields(s), " ")
	if utf8.RuneCountInString(s) <= n {
		return s
	}
	r := []rune(s)[:n-1]
	if i := strings.LastIndex(string(r), " "); i > n/2 {
		return string(r)[:i] + "…"
	}
	return string(r) + "…"
}

func fill(t apiconv.Text, locale string, pairs ...string) string {
	return strings.NewReplacer(pairs...).Replace(t.Best(locale))
}

func best(t apigen.LocalizedText, locale string) string { return apiconv.TextFromAPI(t).Best(locale) }

// SEORoutes mounts the pages for search engines and link previews (the
// showcase, sitemap.xml and robots.txt). They run after the tenant
// middleware and without sign-in, so they describe only public pages.
func (s *Service) SEORoutes(r chi.Router) {
	for path, page := range map[string]http.HandlerFunc{
		"/": s.landingPage, "/showcase": s.catalogPage, "/showcase/": s.catalogPage, "/showcase/{slug}": s.projectPage,
		"/sitemap.xml": s.sitemap, "/robots.txt": s.robots,
	} {
		// Monitors and some crawlers ask with HEAD; the server drops the body.
		r.Get(path, page)
		r.Head(path, page)
	}
}

func (s *Service) writePage(w http.ResponseWriter, r *http.Request, status int, m meta) {
	page := render(s.shell.get(r.Context()), m)
	h := w.Header()
	h.Set("Content-Type", "text/html; charset=utf-8")
	h.Set("Cache-Control", "no-cache")
	// The first screen follows the visitor's language.
	h.Set("Vary", "Accept-Language, Cookie")
	h.Set("Content-Security-Policy", spaCSP)
	h.Set("Cross-Origin-Resource-Policy", "same-origin")
	w.WriteHeader(status)
	_, _ = w.Write(page)
}

// university returns the tenant of the address with its brand and site, and
// its short name in the tenant's language.
func (s *Service) university(r *http.Request) (tenancy.Tenant, tenant.University, string, error) {
	t, err := tenancy.Require(r.Context())
	if err != nil {
		return t, tenant.University{}, "", err
	}
	universities, err := s.Tenants.Universities(r.Context(), []uuid.UUID{t.ID})
	if err != nil {
		return t, tenant.University{}, "", err
	}
	u := universities[t.ID]
	return t, u, best(u.Brief.ShortName, t.Locale), nil
}

func (s *Service) landingPage(w http.ResponseWriter, r *http.Request) {
	t, u, name, err := s.university(r)
	if err != nil {
		httpx.WriteError(w, r, err)
		return
	}
	full := best(u.Brief.Name, t.Locale)
	m := meta{
		Lang: t.Locale, Kind: "website", Canonical: u.Origin + "/",
		Title:       fill(landingTitle, t.Locale, "{university}", name),
		Description: bootString("landing:hero.subtitle", t.Locale),
	}
	m.Body = fmt.Sprintf("<main><h1>%s</h1><p>%s</p><p><a href=\"%s/showcase\">%s</a></p></main>",
		html.EscapeString(m.Title), html.EscapeString(m.Description), html.EscapeString(u.Origin),
		html.EscapeString(fill(catalogTitle, t.Locale, "{university}", name)))
	m.JSONLD = map[string]any{
		"@context": "https://schema.org", "@type": "WebSite", "name": siteName, "url": m.Canonical,
		"description": m.Description, "inLanguage": m.Lang,
		"publisher": map[string]any{"@type": "CollegeOrUniversity", "name": full},
	}
	boot := landingBoot(u, bootLocale(r))
	if m.Boot, err = renderBoot(&boot); err != nil {
		httpx.WriteError(w, r, err)
		return
	}
	s.writePage(w, r, http.StatusOK, m)
}

func (s *Service) catalogPage(w http.ResponseWriter, r *http.Request) {
	t, u, name, err := s.university(r)
	if err != nil {
		httpx.WriteError(w, r, err)
		return
	}
	national := r.URL.Query().Get("scope") == "national"
	params := apigen.ListShowcaseParams{}
	limit := catalogItems
	params.Limit = &limit
	origin := u.Origin
	m := meta{
		Lang: t.Locale, Kind: "website", Canonical: origin + "/showcase",
		Title:       fill(catalogTitle, t.Locale, "{university}", name),
		Description: fill(catalogDescription, t.Locale, "{university}", name),
	}
	if national {
		scope := apigen.ListShowcaseParamsScopeNational
		params.Scope = &scope
		m.Canonical += "?scope=national"
		m.Title, m.Description = nationalTitle.Best(t.Locale), nationalDescription.Best(t.Locale)
	}
	// The routes run without sign-in: search engines see public pages only.
	page, err := s.List(r.Context(), params)
	if err != nil {
		httpx.WriteError(w, r, err)
		return
	}
	items := make([]map[string]any, len(page.Items))
	var body strings.Builder
	fmt.Fprintf(&body, "<main><h1>%s</h1><p>%s</p><ul>", html.EscapeString(m.Title), html.EscapeString(m.Description))
	for i := range page.Items {
		c := &page.Items[i]
		items[i] = map[string]any{"@type": "ListItem", "position": i + 1, "url": c.Url, "name": c.Title}
		fmt.Fprintf(&body, "<li><a href=\"%s\">%s</a> — %s</li>", html.EscapeString(c.Url), html.EscapeString(c.Title), html.EscapeString(c.Tagline))
	}
	body.WriteString("</ul></main>")
	m.Body = body.String()
	m.JSONLD = map[string]any{
		"@context": "https://schema.org", "@type": "CollectionPage", "name": m.Title, "description": m.Description,
		"url": m.Canonical, "inLanguage": m.Lang, "isPartOf": map[string]any{"@type": "WebSite", "name": siteName, "url": origin + "/"},
		"mainEntity": map[string]any{"@type": "ItemList", "numberOfItems": len(items), "itemListElement": items},
	}
	boot := catalogBoot(u.Brief, bootLocale(r), national)
	if m.Boot, err = renderBoot(&boot); err != nil {
		httpx.WriteError(w, r, err)
		return
	}
	s.writePage(w, r, http.StatusOK, m)
}

func (s *Service) projectPage(w http.ResponseWriter, r *http.Request) {
	t, u, name, err := s.university(r)
	if err != nil {
		httpx.WriteError(w, r, err)
		return
	}
	row, _, err := s.published(r.Context(), chi.URLParam(r, "slug"))
	if err != nil {
		var ae *apperr.Error
		if !errors.As(err, &ae) || ae.Kind != apperr.KindNotFound {
			httpx.WriteError(w, r, err)
			return
		}
		// Unknown pages and pages for members only look the same; the web
		// client still opens them for members.
		s.writePage(w, r, http.StatusNotFound, meta{
			Lang: t.Locale, Kind: "website", NoIndex: true,
			Title: fill(catalogTitle, t.Locale, "{university}", name),
		})
		return
	}
	m := projectMeta(&row, t.Locale, u.Origin, name)
	boot := projectBoot(&row, u.Brief, bootLocale(r))
	if m.Boot, err = renderBoot(&boot); err != nil {
		httpx.WriteError(w, r, err)
		return
	}
	s.writePage(w, r, http.StatusOK, m)
}

func projectMeta(e *showcasedb.ShowcaseEntry, locale, origin, university string) meta {
	esc := html.EscapeString
	x := sectionsOf(e.Sections)
	description := e.Tagline
	if description == "" {
		description = x.Problem
	}
	m := meta{
		Lang: locale, Kind: "article", Canonical: origin + "/showcase/" + e.Slug,
		Title:       fill(projectTitle, locale, "{title}", e.Title, "{university}", university),
		Description: clip(description, 160), Image: imageURL(origin, e), ImageAlt: e.Title,
		Published: e.PublishedAt, Modified: &e.UpdatedAt,
	}
	var body strings.Builder
	fmt.Fprintf(&body, "<article><h1>%s</h1>", esc(e.Title))
	if e.Tagline != "" {
		fmt.Fprintf(&body, "<p>%s</p>", esc(e.Tagline))
	}
	for _, sec := range []struct {
		label apiconv.Text
		text  string
	}{{problemLabel, x.Problem}, {solutionLabel, x.Solution}, {marketLabel, x.Market}, {tractionLabel, x.Traction}, {askLabel, x.Ask}} {
		if sec.text != "" {
			fmt.Fprintf(&body, "<h2>%s</h2><p>%s</p>", esc(sec.label.Best(locale)), esc(sec.text))
		}
	}
	fmt.Fprintf(&body, "<p><a href=\"%s/showcase\">%s</a></p></article>", esc(origin), esc(university))
	m.Body = body.String()

	members := teamAPI(e.Team)
	team := make([]map[string]any, 0, len(members))
	for _, p := range members {
		team = append(team, map[string]any{"@type": "Person", "name": p.Name})
	}
	ld := map[string]any{
		"@context": "https://schema.org", "@type": "CreativeWork", "name": e.Title, "headline": clip(e.Title, 110),
		"description": m.Description, "url": m.Canonical, "image": m.Image, "inLanguage": locale,
		"genre": industryLabels[e.Industry].Best(locale), "dateModified": e.UpdatedAt.UTC().Format(time.RFC3339),
		"sourceOrganization": map[string]any{"@type": "CollegeOrUniversity", "name": university, "url": origin + "/"},
	}
	if len(e.Tags) > 0 {
		ld["keywords"] = strings.Join(e.Tags, ", ")
	}
	if e.PublishedAt != nil {
		ld["datePublished"] = e.PublishedAt.UTC().Format(time.RFC3339)
	}
	if len(team) > 0 {
		ld["creator"] = team
	}
	m.JSONLD = ld
	return m
}

type sitemapURL struct {
	Loc     string `xml:"loc"`
	LastMod string `xml:"lastmod,omitempty"`
}

type urlSet struct {
	XMLName xml.Name     `xml:"urlset"`
	XMLNS   string       `xml:"xmlns,attr"`
	URLs    []sitemapURL `xml:"url"`
}

func (s *Service) sitemap(w http.ResponseWriter, r *http.Request) {
	_, u, _, err := s.university(r)
	origin := u.Origin
	if err != nil {
		httpx.WriteError(w, r, err)
		return
	}
	var rows []showcasedb.ListSitemapRow
	err = s.DB.ReadTx(r.Context(), func(ctx context.Context) error {
		var err error
		rows, err = s.q(ctx).ListSitemap(ctx)
		return err
	})
	if err != nil {
		httpx.WriteError(w, r, err)
		return
	}
	set := urlSet{XMLNS: "http://www.sitemaps.org/schemas/sitemap/0.9", URLs: []sitemapURL{
		{Loc: origin + "/"}, {Loc: origin + "/showcase"}, {Loc: origin + "/showcase?scope=national"},
	}}
	for _, row := range rows {
		set.URLs = append(set.URLs, sitemapURL{Loc: origin + "/showcase/" + row.Slug, LastMod: row.UpdatedAt.UTC().Format("2006-01-02")})
	}
	out, err := xml.MarshalIndent(set, "", "  ")
	if err != nil {
		httpx.WriteError(w, r, err)
		return
	}
	w.Header().Set("Content-Type", "application/xml; charset=utf-8")
	w.Header().Set("Cache-Control", "public, max-age=3600")
	_, _ = w.Write([]byte(xml.Header))
	_, _ = w.Write(out)
}

// robotsDisallowed are the areas of the web client behind sign-in.
var robotsDisallowed = []string{
	"/app/", "/login", "/register", "/onboarding", "/forgot-password", "/reset-password", "/verify-email", "/design", "/team/",
}

func (s *Service) robots(w http.ResponseWriter, r *http.Request) {
	_, u, _, err := s.university(r)
	origin := u.Origin
	if err != nil {
		httpx.WriteError(w, r, err)
		return
	}
	var b strings.Builder
	b.WriteString("User-agent: *\n")
	for _, p := range robotsDisallowed {
		b.WriteString("Disallow: " + p + "\n")
	}
	b.WriteString("\nSitemap: " + origin + "/sitemap.xml\n")
	w.Header().Set("Content-Type", "text/plain; charset=utf-8")
	w.Header().Set("Cache-Control", "public, max-age=86400")
	_, _ = w.Write([]byte(b.String()))
}

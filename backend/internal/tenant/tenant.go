// Package tenant resolves the university from the request host and serves its
// public settings (branding, sign-up rules, SSO providers, consent text).
package tenant

import (
	"context"
	"errors"
	"fmt"
	"net"
	"net/http"
	"net/url"
	"strings"
	"sync"
	"time"

	"github.com/google/uuid"

	"github.com/yelnurq/startify/backend/internal/apiconv"
	"github.com/yelnurq/startify/backend/internal/apigen"
	"github.com/yelnurq/startify/backend/internal/platform/apperr"
	"github.com/yelnurq/startify/backend/internal/platform/config"
	"github.com/yelnurq/startify/backend/internal/platform/db"
	"github.com/yelnurq/startify/backend/internal/platform/httpx"
	"github.com/yelnurq/startify/backend/internal/platform/tenancy"
	"github.com/yelnurq/startify/backend/internal/tenant/tenantdb"
)

// Resolved hosts are cached for a minute; unknown hosts only briefly, so a
// newly created university appears at once. Host headers are chosen by
// clients, so the cache is bounded.
const (
	cacheTTL        = time.Minute
	missCacheTTL    = 5 * time.Second
	maxCacheEntries = 1024
	maxHostLength   = 253
)

// Service resolves and describes tenants.
type Service struct {
	db         *db.DB
	baseDomain string
	defaultTen string
	publicURL  *url.URL
	// fallbackHosts serve the default tenant: the bare base domain, the host
	// of the public URL and localhost (development). Any other unknown host
	// belongs to no tenant.
	fallbackHosts map[string]bool

	mu         sync.Mutex
	cache      map[string]cacheEntry
	cacheLimit int
}

type cacheEntry struct {
	// tenant carries the origin of the host for links in letters.
	tenant  tenancy.Tenant
	found   bool
	expires time.Time
}

// NewService returns the tenant service.
func NewService(database *db.DB, cfg config.Tenant, publicURL string) (*Service, error) {
	u, err := url.Parse(publicURL)
	if err != nil {
		return nil, fmt.Errorf("tenant: public url: %w", err)
	}
	base := strings.ToLower(strings.TrimSpace(cfg.BaseDomain))
	fallback := map[string]bool{"localhost": true, strings.ToLower(u.Hostname()): true}
	if base != "" {
		fallback[base] = true
	}
	return &Service{
		db:            database,
		baseDomain:    base,
		defaultTen:    strings.ToLower(strings.TrimSpace(cfg.Default)),
		publicURL:     u,
		fallbackHosts: fallback,
		cache:         map[string]cacheEntry{},
		cacheLimit:    maxCacheEntries,
	}, nil
}

// normalizeHost lower-cases a Host header and drops its port and trailing dot.
func normalizeHost(hostHeader string) string {
	host := strings.ToLower(strings.TrimSpace(hostHeader))
	if h, _, err := net.SplitHostPort(host); err == nil {
		host = h
	}
	return strings.TrimSuffix(host, ".")
}

// Resolve maps a Host header to a tenant. The bool reports whether one was
// found. The origin of the tenant (where links in letters lead) is built from
// the stored domain or the configuration, never from the port or spelling the
// client sent.
func (s *Service) Resolve(ctx context.Context, hostHeader string) (tenancy.Tenant, bool, error) {
	host := normalizeHost(hostHeader)
	if host == "" || len(host) > maxHostLength {
		return tenancy.Tenant{}, false, nil
	}
	s.mu.Lock()
	entry, cached := s.cache[host]
	s.mu.Unlock()
	if !cached || time.Now().After(entry.expires) {
		var err error
		entry, err = s.lookup(ctx, host)
		if err != nil {
			return tenancy.Tenant{}, false, err
		}
		s.remember(host, entry)
	}
	if !entry.found {
		return tenancy.Tenant{}, false, nil
	}
	return entry.tenant, true, nil
}

// remember caches a lookup. When the cache is full, expired entries go first,
// then arbitrary ones (map order is random); they are looked up again on demand.
func (s *Service) remember(host string, entry cacheEntry) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if _, ok := s.cache[host]; !ok && len(s.cache) >= s.cacheLimit {
		now := time.Now()
		for h, e := range s.cache {
			if now.After(e.expires) {
				delete(s.cache, h)
			}
		}
		for h := range s.cache {
			if len(s.cache) < s.cacheLimit {
				break
			}
			delete(s.cache, h)
		}
	}
	s.cache[host] = entry
}

func (s *Service) lookup(ctx context.Context, host string) (cacheEntry, error) {
	q := tenantdb.New(s.db.Conn(ctx))
	entry := cacheEntry{expires: time.Now().Add(cacheTTL)}
	var origin string

	row, err := q.GetTenantByDomain(ctx, host)
	switch {
	case err == nil:
		entry.found, origin = true, s.hostOrigin(host)
	case db.IsNoRows(err):
		if slug, ok := strings.CutSuffix(host, "."+s.baseDomain); ok && slug != "" && !strings.Contains(slug, ".") {
			row, err = q.GetTenantBySlug(ctx, slug)
			if err == nil {
				entry.found, origin = true, s.hostOrigin(host)
			}
		}
		if !entry.found && s.defaultTen != "" && s.fallbackHosts[host] {
			row, err = q.GetTenantBySlug(ctx, s.defaultTen)
			if err == nil {
				entry.found, origin = true, strings.TrimSuffix(s.publicURL.String(), "/")
			}
		}
		if err != nil && !db.IsNoRows(err) {
			return entry, fmt.Errorf("tenant: lookup %s: %w", host, err)
		}
	default:
		return entry, fmt.Errorf("tenant: lookup %s: %w", host, err)
	}
	if entry.found && row.Status == "suspended" {
		entry.found = false
	}
	if !entry.found {
		entry.expires = time.Now().Add(missCacheTTL)
		return entry, nil
	}
	entry.tenant = tenancy.Tenant{ID: row.ID, Slug: row.Slug, Locale: row.DefaultLocale, Origin: origin}
	return entry, nil
}

// hostOrigin is the address of a host that belongs to a tenant: the scheme
// and port of the public URL with the host as stored.
func (s *Service) hostOrigin(host string) string {
	if port := s.publicURL.Port(); port != "" {
		host = net.JoinHostPort(host, port)
	}
	return s.publicURL.Scheme + "://" + host
}

// Middleware attaches the tenant of the host to the request and scopes data
// access to it. Requests for unknown hosts (including a forged Host header
// sent straight to the API) continue without a tenant; handlers that need one
// answer 404 tenant.not_found.
func (s *Service) Middleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		t, ok, err := s.Resolve(r.Context(), r.Host)
		if err != nil {
			httpx.WriteError(w, r, apperr.Internal(err))
			return
		}
		if ok {
			ctx := tenancy.With(r.Context(), t)
			ctx = db.WithTenant(ctx, t.ID)
			r = r.WithContext(ctx)
		}
		next.ServeHTTP(w, r)
	})
}

// Brief returns the short description used in sessions and profiles.
func (s *Service) Brief(ctx context.Context, id uuid.UUID) (apigen.TenantBrief, error) {
	row, err := tenantdb.New(s.db.Conn(ctx)).GetTenantByID(ctx, id)
	if err != nil {
		return apigen.TenantBrief{}, fmt.Errorf("tenant: brief: %w", err)
	}
	return apigen.TenantBrief{
		Id:        row.ID,
		Slug:      row.Slug,
		Name:      apiconv.ParseText(row.Name).API(),
		ShortName: apiconv.ParseText(row.ShortName).API(),
	}, nil
}

// University is how a university appears next to its projects on the
// showcase: its brand and the address of its site.
type University struct {
	Brief  apigen.UniversityBrief
	Origin string
	// CoverURL is the photo for the hero of the university's pages.
	CoverURL *string
}

// Universities describes universities by id (a platform registry, readable
// from any university).
func (s *Service) Universities(ctx context.Context, ids []uuid.UUID) (map[uuid.UUID]University, error) {
	out := map[uuid.UUID]University{}
	if len(ids) == 0 {
		return out, nil
	}
	err := s.db.ReadTx(ctx, func(ctx context.Context) error {
		rows, err := tenantdb.New(s.db.Conn(ctx)).ListTenantsByIDs(ctx, ids)
		for i := range rows {
			r := &rows[i]
			out[r.ID] = University{
				Brief: apigen.UniversityBrief{
					Slug: r.Slug, Name: apiconv.ParseText(r.Name).API(), ShortName: apiconv.ParseText(r.ShortName).API(),
					Color: r.PrimaryColor, LogoUrl: apiconv.FileURL(r.LogoFileID),
				},
				Origin:   s.origin(r.Slug, r.PrimaryDomain),
				CoverURL: apiconv.FileURL(r.CoverFileID),
			}
		}
		return err
	})
	if err != nil {
		return nil, fmt.Errorf("tenant: universities: %w", err)
	}
	return out, nil
}

// Suspended lists the universities whose platform is suspended: their sites
// do not open, so their pages leave the national showcase too.
func (s *Service) Suspended(ctx context.Context) ([]uuid.UUID, error) {
	var ids []uuid.UUID
	err := s.db.ReadTx(ctx, func(ctx context.Context) error {
		var err error
		ids, err = tenantdb.New(s.db.Conn(ctx)).ListSuspendedTenantIDs(ctx)
		return err
	})
	if err != nil {
		return nil, fmt.Errorf("tenant: suspended: %w", err)
	}
	return ids, nil
}

// PlatformOrigin is the address of the platform's public site (the default
// university): the one home of pages every university shows alike, such as
// the national showcase.
func (s *Service) PlatformOrigin() string { return strings.TrimSuffix(s.publicURL.String(), "/") }

// origin is the address of a university's site: its primary domain, the
// public URL for the default university, or a subdomain of the base domain.
func (s *Service) origin(slug, domain string) string {
	switch {
	case domain != "":
		return s.hostOrigin(strings.ToLower(domain))
	case slug == s.defaultTen || s.baseDomain == "":
		return strings.TrimSuffix(s.publicURL.String(), "/")
	}
	return s.hostOrigin(slug + "." + s.baseDomain)
}

// Settings are the sign-up rules of the current tenant.
type Settings struct {
	SelfSignup     bool
	EmailDomains   []string
	ConsentVersion string
	ConsentText    apiconv.Text
	Name           apiconv.Text
	ShortName      apiconv.Text
	// IPPolicyVersion is empty when the university has no intellectual
	// property policy to accept.
	IPPolicyVersion string
	IPPolicyText    apiconv.Text
	PrimaryColor    string
	// ContactModeration sends partners' contact requests to an innovation
	// manager before the team sees them.
	ContactModeration bool
}

// CurrentSettings loads the sign-up rules of the request's tenant.
func (s *Service) CurrentSettings(ctx context.Context) (Settings, error) {
	t, err := tenancy.Require(ctx)
	if err != nil {
		return Settings{}, err
	}
	var out Settings
	err = s.db.ReadTx(ctx, func(ctx context.Context) error {
		q := tenantdb.New(s.db.Conn(ctx))
		row, err := q.GetTenantByID(ctx, t.ID)
		if err != nil {
			return err
		}
		settings, err := q.GetTenantSettings(ctx, t.ID)
		if err != nil {
			return err
		}
		out = Settings{
			SelfSignup:     settings.SelfSignup,
			EmailDomains:   settings.EmailDomains,
			ConsentVersion: settings.ConsentVersion,
			ConsentText:    apiconv.ParseText(settings.ConsentText),
			Name:           apiconv.ParseText(row.Name),
			ShortName:      apiconv.ParseText(row.ShortName),
			PrimaryColor:   row.PrimaryColor,

			ContactModeration: settings.ContactModeration,
		}
		if text := apiconv.ParseText(settings.IpPolicyText); len(text) > 0 && settings.IpPolicyVersion != "" {
			out.IPPolicyVersion, out.IPPolicyText = settings.IpPolicyVersion, text
		}
		return nil
	})
	if err != nil {
		return Settings{}, fmt.Errorf("tenant: settings: %w", err)
	}
	return out, nil
}

// LoginOptions tells the sign-in page about sign-in methods other modules
// own (the university directory).
type LoginOptions interface {
	LoginOptions(ctx context.Context) (apigen.DirectoryLogin, error)
}

// Handler serves the tenant operations of the API.
type Handler struct {
	svc   *Service
	login LoginOptions
}

// NewHandler returns the tenant handler; login may be nil (no directory).
func NewHandler(svc *Service, login LoginOptions) *Handler { return &Handler{svc: svc, login: login} }

// GetCurrentTenant implements apigen.StrictServerInterface.
func (h *Handler) GetCurrentTenant(ctx context.Context, _ apigen.GetCurrentTenantRequestObject) (apigen.GetCurrentTenantResponseObject, error) {
	t, err := tenancy.Require(ctx)
	if err != nil {
		return nil, err
	}
	var out apigen.Tenant
	err = h.svc.db.ReadTx(ctx, func(ctx context.Context) error {
		q := tenantdb.New(h.svc.db.Conn(ctx))
		row, err := q.GetTenantByID(ctx, t.ID)
		if err != nil {
			return err
		}
		settings, err := q.GetTenantSettings(ctx, t.ID)
		if err != nil {
			return err
		}
		providers, err := q.ListEnabledOidcProviders(ctx, t.ID)
		if err != nil {
			return err
		}
		sso := make([]apigen.SsoProvider, 0, len(providers))
		for _, p := range providers {
			sso = append(sso, apigen.SsoProvider{Id: p.ID, Name: apiconv.ParseText(p.Name).API()})
		}
		out = apigen.Tenant{
			Id:            row.ID,
			Slug:          row.Slug,
			Name:          apiconv.ParseText(row.Name).API(),
			ShortName:     apiconv.ParseText(row.ShortName).API(),
			DefaultLocale: apigen.Locale(row.DefaultLocale),
			PrimaryColor:  row.PrimaryColor,
			LogoUrl:       apiconv.FileURL(row.LogoFileID),
			CoverUrl:      apiconv.FileURL(row.CoverFileID),
			SelfSignup:    settings.SelfSignup,
			EmailDomains:  settings.EmailDomains,
			SsoProviders:  sso,
			Consent: apigen.ConsentPolicy{
				Version: settings.ConsentVersion,
				Text:    apiconv.ParseText(settings.ConsentText).API(),
			},
		}
		if text := apiconv.ParseText(settings.IpPolicyText); len(text) > 0 && settings.IpPolicyVersion != "" {
			out.IpPolicy = &apigen.IpPolicy{Version: settings.IpPolicyVersion, Text: text.API()}
		}
		return nil
	})
	if err == nil && h.login != nil {
		out.DirectoryLogin, err = h.login.LoginOptions(ctx)
	}
	if err != nil {
		if errors.Is(err, context.Canceled) {
			return nil, err
		}
		return nil, fmt.Errorf("tenant: current: %w", err)
	}
	return apigen.GetCurrentTenant200JSONResponse(out), nil
}

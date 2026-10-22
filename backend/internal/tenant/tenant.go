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
// newly created university appears at once.
const (
	cacheTTL     = time.Minute
	missCacheTTL = 5 * time.Second
)

// Service resolves and describes tenants.
type Service struct {
	db         *db.DB
	baseDomain string
	defaultTen string
	publicURL  *url.URL

	mu    sync.Mutex
	cache map[string]cacheEntry
}

type cacheEntry struct {
	tenant  tenancy.Tenant
	found   bool
	trusted bool
	expires time.Time
}

// NewService returns the tenant service.
func NewService(database *db.DB, cfg config.Tenant, publicURL string) (*Service, error) {
	u, err := url.Parse(publicURL)
	if err != nil {
		return nil, fmt.Errorf("tenant: public url: %w", err)
	}
	return &Service{
		db:         database,
		baseDomain: strings.ToLower(strings.TrimSpace(cfg.BaseDomain)),
		defaultTen: strings.ToLower(strings.TrimSpace(cfg.Default)),
		publicURL:  u,
		cache:      map[string]cacheEntry{},
	}, nil
}

// Resolve maps a Host header to a tenant. The bool reports whether one was found.
func (s *Service) Resolve(ctx context.Context, hostHeader string) (tenancy.Tenant, bool, error) {
	host := strings.ToLower(hostHeader)
	if h, _, err := net.SplitHostPort(host); err == nil {
		host = h
	}
	host = strings.TrimSuffix(host, ".")

	s.mu.Lock()
	entry, cached := s.cache[host]
	s.mu.Unlock()
	if !cached || time.Now().After(entry.expires) {
		var err error
		entry, err = s.lookup(ctx, host)
		if err != nil {
			return tenancy.Tenant{}, false, err
		}
		s.mu.Lock()
		s.cache[host] = entry
		s.mu.Unlock()
	}
	if !entry.found {
		return tenancy.Tenant{}, false, nil
	}
	t := entry.tenant
	// Links in e-mails go to the address the user came from only when that
	// address is known to belong to the tenant; otherwise to the configured
	// public URL (no password-reset poisoning through a forged Host header).
	if entry.trusted {
		t.Origin = s.publicURL.Scheme + "://" + hostHeader
	} else {
		t.Origin = strings.TrimSuffix(s.publicURL.String(), "/")
	}
	return t, true, nil
}

func (s *Service) lookup(ctx context.Context, host string) (cacheEntry, error) {
	q := tenantdb.New(s.db.Conn(ctx))
	entry := cacheEntry{expires: time.Now().Add(cacheTTL)}

	row, err := q.GetTenantByDomain(ctx, host)
	switch {
	case err == nil:
		entry.found, entry.trusted = true, true
	case db.IsNoRows(err):
		if slug, ok := strings.CutSuffix(host, "."+s.baseDomain); ok && !strings.Contains(slug, ".") {
			row, err = q.GetTenantBySlug(ctx, slug)
			if err == nil {
				entry.found, entry.trusted = true, true
			}
		}
		if !entry.found && s.defaultTen != "" {
			row, err = q.GetTenantBySlug(ctx, s.defaultTen)
			if err == nil {
				entry.found = true
				entry.trusted = host == s.baseDomain
			}
		}
		if err != nil && !db.IsNoRows(err) {
			return entry, fmt.Errorf("tenant: lookup %s: %w", host, err)
		}
	default:
		return entry, fmt.Errorf("tenant: lookup %s: %w", host, err)
	}
	if !entry.found {
		entry.expires = time.Now().Add(missCacheTTL)
	}
	if entry.found {
		if row.Status == "suspended" {
			entry.found = false
		} else {
			entry.tenant = tenancy.Tenant{ID: row.ID, Slug: row.Slug, Locale: row.DefaultLocale}
		}
	}
	return entry, nil
}

// Middleware attaches the tenant of the host to the request and scopes data
// access to it. Requests for unknown hosts continue without a tenant; handlers
// that need one answer 404 tenant.not_found.
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

// origin is the address of a university's site: its primary domain, the
// public URL for the default university, or a subdomain of the base domain.
func (s *Service) origin(slug, domain string) string {
	switch {
	case domain != "":
		return s.publicURL.Scheme + "://" + domain
	case slug == s.defaultTen || s.baseDomain == "":
		return strings.TrimSuffix(s.publicURL.String(), "/")
	}
	host := slug + "." + s.baseDomain
	if port := s.publicURL.Port(); port != "" {
		host = net.JoinHostPort(host, port)
	}
	return s.publicURL.Scheme + "://" + host
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

// Handler serves the tenant operations of the API.
type Handler struct {
	svc *Service
}

// NewHandler returns the tenant handler.
func NewHandler(svc *Service) *Handler { return &Handler{svc: svc} }

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
	if err != nil {
		if errors.Is(err, context.Canceled) {
			return nil, err
		}
		return nil, fmt.Errorf("tenant: current: %w", err)
	}
	return apigen.GetCurrentTenant200JSONResponse(out), nil
}

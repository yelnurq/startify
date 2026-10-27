package auth

import (
	"context"
	"crypto/subtle"
	"encoding/json"
	"errors"
	"fmt"
	"log/slog"
	"net/http"
	"net/url"
	"strings"
	"sync"
	"time"
	"unicode"

	"github.com/coreos/go-oidc/v3/oidc"
	"github.com/google/uuid"
	"golang.org/x/oauth2"

	"github.com/yelnurq/startify/backend/internal/apigen"
	"github.com/yelnurq/startify/backend/internal/platform/apperr"
	"github.com/yelnurq/startify/backend/internal/platform/secrets"
	"github.com/yelnurq/startify/backend/internal/platform/tenancy"
	"github.com/yelnurq/startify/backend/internal/tenant"
	"github.com/yelnurq/startify/backend/internal/tenant/tenantdb"
)

const oidcStateTTL = 10 * time.Minute

var (
	errOIDC          = apperr.Unauthorized("auth.oidc_failed", "single sign-on failed")
	errSSOLinkDenied = apperr.Forbidden("auth.sso_link_denied",
		"the account belongs to another university; sign in with its password or e-mailed code")
)

// oidcState travels in a sealed cookie between the start and the callback.
type oidcState struct {
	Provider uuid.UUID `json:"p"`
	State    string    `json:"s"`
	Nonce    string    `json:"n"`
	Verifier string    `json:"v"`
	Redirect string    `json:"r"`
	Expires  int64     `json:"e"`
}

// oidcProviders caches discovered identity providers by issuer.
type oidcProviders struct {
	client *http.Client
	mu     sync.Mutex
	byURL  map[string]*oidc.Provider
}

func newOIDCProviders() *oidcProviders {
	return &oidcProviders{client: &http.Client{Timeout: 10 * time.Second}, byURL: map[string]*oidc.Provider{}}
}

func (o *oidcProviders) get(ctx context.Context, issuer string) (*oidc.Provider, error) {
	o.mu.Lock()
	p, ok := o.byURL[issuer]
	o.mu.Unlock()
	if ok {
		return p, nil
	}
	// The provider keeps this context to refresh signing keys later.
	p, err := oidc.NewProvider(oidc.ClientContext(context.WithoutCancel(ctx), o.client), issuer)
	if err != nil {
		return nil, fmt.Errorf("auth: discover %s: %w", issuer, err)
	}
	o.mu.Lock()
	o.byURL[issuer] = p
	o.mu.Unlock()
	return p, nil
}

// SetOIDCClient replaces the HTTP client used to reach identity providers (tests).
func (s *Service) SetOIDCClient(c *http.Client) {
	s.oidc = &oidcProviders{client: c, byURL: map[string]*oidc.Provider{}}
}

type oidcSetup struct {
	provider tenantdb.TenantOidcProvider
	conf     oauth2.Config
	op       *oidc.Provider
}

func (s *Service) oidcSetup(ctx context.Context, providerID uuid.UUID) (oidcSetup, error) {
	t, err := tenancy.Require(ctx)
	if err != nil {
		return oidcSetup{}, err
	}
	prov, err := s.tenants.OIDCProvider(ctx, providerID)
	if errors.Is(err, tenant.ErrNoProvider) {
		return oidcSetup{}, errOIDC
	}
	if err != nil {
		return oidcSetup{}, err
	}
	op, err := s.oidc.get(ctx, prov.Issuer)
	if err != nil {
		return oidcSetup{}, err
	}
	secret, err := s.keys.Open(prov.ClientSecretEnc)
	if err != nil {
		return oidcSetup{}, fmt.Errorf("auth: provider secret: %w", err)
	}
	return oidcSetup{
		provider: prov,
		op:       op,
		conf: oauth2.Config{
			ClientID:     prov.ClientID,
			ClientSecret: string(secret),
			Endpoint:     op.Endpoint(),
			RedirectURL:  t.Origin + "/api/v1/auth/oidc/" + providerID.String() + "/callback",
			Scopes:       prov.Scopes,
		},
	}, nil
}

// safeRedirect keeps post-sign-in redirects on this site: only a path that
// starts with a single slash, without scheme, host, backslash or control
// characters. Browsers drop tabs and line breaks from addresses and read a
// backslash as a slash, so "/\t/evil.example" would lead to //evil.example.
func safeRedirect(r *string) string {
	if r == nil {
		return "/"
	}
	v := strings.TrimSpace(*r)
	if strings.ContainsFunc(v, func(c rune) bool { return c == '\\' || unicode.IsControl(c) }) {
		return "/"
	}
	u, err := url.Parse(v)
	if err != nil || u.Scheme != "" || u.Host != "" || u.User != nil || u.Opaque != "" ||
		!strings.HasPrefix(v, "/") || !strings.HasPrefix(u.Path, "/") || strings.HasPrefix(u.Path, "//") {
		return "/"
	}
	return v
}

// StartOIDC returns the authorization URL of the provider and the cookie
// holding the state of the flow (with PKCE verifier and nonce).
func (s *Service) StartOIDC(ctx context.Context, providerID uuid.UUID, redirect *string) (string, *http.Cookie, error) {
	setup, err := s.oidcSetup(ctx, providerID)
	if err != nil {
		return "", nil, err
	}
	st := oidcState{
		Provider: providerID,
		State:    tokenEncoding.EncodeToString(secrets.RandomToken(24)),
		Nonce:    tokenEncoding.EncodeToString(secrets.RandomToken(24)),
		Verifier: oauth2.GenerateVerifier(),
		Redirect: safeRedirect(redirect),
		Expires:  s.now().Add(oidcStateTTL).Unix(),
	}
	raw, err := json.Marshal(st)
	if err != nil {
		return "", nil, fmt.Errorf("auth: oidc state: %w", err)
	}
	sealed, err := s.keys.Seal(raw)
	if err != nil {
		return "", nil, err
	}
	authURL := setup.conf.AuthCodeURL(st.State, oidc.Nonce(st.Nonce), oauth2.S256ChallengeOption(st.Verifier))
	return authURL, s.cookies.oidcState(tokenEncoding.EncodeToString(sealed)), nil
}

type oidcClaims struct {
	Email         string `json:"email"`
	EmailVerified *bool  `json:"email_verified"`
	Name          string `json:"name"`
	GivenName     string `json:"given_name"`
	FamilyName    string `json:"family_name"`
}

func (c oidcClaims) fullName() string {
	if n := strings.TrimSpace(c.Name); n != "" {
		return n
	}
	if n := strings.TrimSpace(c.GivenName + " " + c.FamilyName); n != "" {
		return n
	}
	local, _, _ := strings.Cut(c.Email, "@")
	return local
}

// CompleteOIDC finishes single sign-on: it checks the state, exchanges the
// code, verifies the ID token and signs the account in, linking or creating
// it. It returns the session cookies and where to send the browser.
func (s *Service) CompleteOIDC(ctx context.Context, providerID uuid.UUID, stateCookie string, params apigen.CompleteOidcParams) ([]*http.Cookie, string, error) {
	st, err := s.openState(stateCookie)
	if err != nil || st.Provider != providerID || s.now().Unix() > st.Expires {
		return nil, "", errOIDC
	}
	if params.Error != nil {
		slog.WarnContext(ctx, "identity provider returned an error", slog.String("error", *params.Error))
		return nil, "", errOIDC
	}
	if params.State == nil || params.Code == nil || subtle.ConstantTimeCompare([]byte(*params.State), []byte(st.State)) != 1 {
		return nil, "", errOIDC
	}
	setup, err := s.oidcSetup(ctx, providerID)
	if err != nil {
		return nil, "", err
	}
	tok, err := setup.conf.Exchange(oidc.ClientContext(ctx, s.oidc.client), *params.Code, oauth2.VerifierOption(st.Verifier))
	if err != nil {
		slog.WarnContext(ctx, "code exchange failed", slog.Any("error", err))
		return nil, "", errOIDC
	}
	rawID, ok := tok.Extra("id_token").(string)
	if !ok {
		return nil, "", errOIDC
	}
	idt, err := setup.op.VerifierContext(oidc.ClientContext(ctx, s.oidc.client), &oidc.Config{ClientID: setup.provider.ClientID}).Verify(ctx, rawID)
	if err != nil || subtle.ConstantTimeCompare([]byte(idt.Nonce), []byte(st.Nonce)) != 1 {
		slog.WarnContext(ctx, "ID token rejected", slog.Any("error", err))
		return nil, "", errOIDC
	}
	var claims oidcClaims
	if err := idt.Claims(&claims); err != nil {
		return nil, "", errOIDC
	}
	email := normalizeEmail(claims.Email)
	if !strings.Contains(email, "@") {
		return nil, "", errOIDC
	}
	verified := claims.EmailVerified != nil && *claims.EmailVerified
	_, cookies, err := s.SignInExternal(ctx, External{
		Provider: "oidc:" + providerID.String(), Subject: idt.Subject, Email: email, FullName: claims.fullName(),
		Vouched: verified, Verified: verified, AutoProvision: setup.provider.AutoProvision,
		Method: "oidc", Metadata: map[string]any{"provider": providerID.String()},
	})
	if err != nil {
		return nil, "", err
	}
	return append([]*http.Cookie{s.cookies.oidcCleared()}, cookies...), st.Redirect, nil
}

// mayLinkSSO allows linking an existing account to this university's
// identity provider or directory: the account is a member here, or its
// address is on one of the university's e-mail domains.
func (s *Service) mayLinkSSO(ctx context.Context, userID uuid.UUID, email string) error {
	if _, member, err := s.users.Membership(ctx, userID); err != nil || member {
		return err
	}
	settings, err := s.tenants.CurrentSettings(ctx)
	if err != nil {
		return err
	}
	if !domainListed(email, settings.EmailDomains) {
		return errSSOLinkDenied
	}
	return nil
}

func (s *Service) openState(cookie string) (oidcState, error) {
	var st oidcState
	sealed, err := tokenEncoding.DecodeString(cookie)
	if err != nil {
		return st, err
	}
	raw, err := s.keys.Open(sealed)
	if err != nil {
		return st, err
	}
	if err := json.Unmarshal(raw, &st); err != nil {
		return st, err
	}
	return st, nil
}

// signInErrorURL sends the browser back to the sign-in page with the reason.
func signInErrorURL(err error) string {
	code := errOIDC.Code
	var e *apperr.Error
	if errors.As(err, &e) && strings.HasPrefix(e.Code, "auth.") {
		code = e.Code
	}
	return "/login?" + url.Values{"error": {code}}.Encode()
}

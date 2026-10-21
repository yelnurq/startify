package app

import (
	"context"
	"encoding/json"
	"net/http"
	"net/url"
	"strings"
	"testing"

	"github.com/golang-jwt/jwt/v5"
	"github.com/google/uuid"
	"github.com/oauth2-proxy/mockoidc"

	"github.com/yelnurq/startify/backend/internal/apiconv"
	"github.com/yelnurq/startify/backend/internal/apigen"
	"github.com/yelnurq/startify/backend/internal/platform/db"
	"github.com/yelnurq/startify/backend/internal/tenant/tenantdb"
)

// ssoUser is an account of the university identity provider.
type ssoUser struct {
	sub, email, name string
	verified         bool
}

type ssoClaims struct {
	*mockoidc.IDTokenClaims
	Email         string `json:"email"`
	EmailVerified bool   `json:"email_verified"`
	Name          string `json:"name"`
}

func (u ssoUser) ID() string { return u.sub }

func (u ssoUser) Userinfo([]string) ([]byte, error) {
	return json.Marshal(map[string]any{"email": u.email, "name": u.name})
}

func (u ssoUser) Claims(_ []string, c *mockoidc.IDTokenClaims) (jwt.Claims, error) {
	return &ssoClaims{IDTokenClaims: c, Email: u.email, EmailVerified: u.verified, Name: u.name}, nil
}

func startIDP(t *testing.T, h *harness, tenantID uuid.UUID) (*mockoidc.MockOIDC, uuid.UUID) {
	t.Helper()
	m, err := mockoidc.Run()
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = m.Shutdown() })
	cfg := m.Config()
	secret, err := h.keys.Seal([]byte(cfg.ClientSecret))
	if err != nil {
		t.Fatal(err)
	}
	var id uuid.UUID
	err = h.db.App.Tx(db.WithTenant(context.Background(), tenantID), func(ctx context.Context) error {
		id, err = tenantdb.New(h.db.App.Conn(ctx)).CreateOidcProvider(ctx, tenantdb.CreateOidcProviderParams{
			TenantID: tenantID, Name: apiconv.Text{"ru": "Единый вход КазУТБ", "kk": "ҚазТБУ бірыңғай кіру"}.JSON(),
			Issuer: cfg.Issuer, ClientID: cfg.ClientID, ClientSecretEnc: secret,
			Scopes: []string{"openid", "email", "profile"}, AutoProvision: true,
		})
		return err
	})
	if err != nil {
		t.Fatal(err)
	}
	return m, id
}

// signInWithSSO walks the browser through the identity provider and returns
// where the platform sent it at the end.
func signInWithSSO(t *testing.T, b *browser, provider uuid.UUID, tamper func(url.Values)) string {
	t.Helper()
	rec := b.do(http.MethodGet, "/api/v1/auth/oidc/"+provider.String()+"/start?redirect=/team", nil)
	expect(t, rec, http.StatusFound)
	authorize := rec.Header().Get("Location")
	if _, ok := b.jar["qadam_oidc"]; !ok || !strings.Contains(authorize, "code_challenge=") || !strings.Contains(authorize, "nonce=") {
		t.Fatalf("start: %s %v", authorize, b.jar)
	}
	client := &http.Client{CheckRedirect: func(*http.Request, []*http.Request) error { return http.ErrUseLastResponse }}
	resp, err := client.Get(authorize)
	if err != nil {
		t.Fatal(err)
	}
	_ = resp.Body.Close()
	callback, err := url.Parse(resp.Header.Get("Location"))
	if err != nil || callback.Host != kazutbHost {
		t.Fatalf("identity provider redirected to %q", resp.Header.Get("Location"))
	}
	query := callback.Query()
	if tamper != nil {
		tamper(query)
	}
	rec = b.do(http.MethodGet, callback.Path+"?"+query.Encode(), nil)
	expect(t, rec, http.StatusFound)
	if _, ok := b.jar["qadam_oidc"]; ok {
		t.Fatal("state cookie must be cleared")
	}
	return rec.Header().Get("Location")
}

func TestSingleSignOn(t *testing.T) {
	h := newHarness(t)
	ts := seedTenants(t, h)
	idp, provider := startIDP(t, h, ts.kazutb)

	sso := decode[apigen.Tenant](t, h.browser(t, kazutbHost).do(http.MethodGet, "/api/v1/tenant", nil)).SsoProviders
	if len(sso) != 1 || sso[0].Id != provider || *sso[0].Name.Ru != "Единый вход КазУТБ" {
		t.Fatalf("providers: %+v", sso)
	}

	// The first sign-in creates the account and the membership.
	b := h.browser(t, kazutbHost)
	idp.QueueUser(ssoUser{sub: "kazutb-1001", email: "Meruert@qadam.test", name: "Меруерт Сұлтанова", verified: true})
	if to := signInWithSSO(t, b, provider, nil); to != "/team" {
		t.Fatalf("redirected to %q", to)
	}
	me := decode[apigen.Me](t, b.do(http.MethodGet, "/api/v1/me", nil))
	if me.User.FullName != "Меруерт Сұлтанова" || string(me.User.Email) != "meruert@qadam.test" || !me.User.EmailVerified ||
		me.Membership.Status != apigen.MembershipStatusActive || me.Profile.PublicSlug != "meruert-sultanova" {
		t.Fatalf("provisioned: %+v", me)
	}

	// The next sign-in finds the same account by its identity.
	again := h.browser(t, kazutbHost)
	idp.QueueUser(ssoUser{sub: "kazutb-1001", email: "changed@qadam.test", name: "Меруерт", verified: true})
	signInWithSSO(t, again, provider, nil)
	if id := decode[apigen.Session](t, again.do(http.MethodGet, "/api/v1/auth/session", nil)).User.Id; id != me.User.Id {
		t.Fatalf("second sign-in created another account: %s", id)
	}

	// An existing account is linked only when the provider vouches for the address.
	register(t, h.browser(t, kazutbHost), "linked@qadam.test", "Связанный Аккаунт")
	idp.QueueUser(ssoUser{sub: "kazutb-2002", email: "linked@qadam.test", name: "Самозванец", verified: false})
	if to := signInWithSSO(t, h.browser(t, kazutbHost), provider, nil); to != "/login?error=auth.oidc_failed" {
		t.Fatalf("unverified link: %q", to)
	}
	idp.QueueUser(ssoUser{sub: "kazutb-2002", email: "linked@qadam.test", name: "Связанный Аккаунт", verified: true})
	linked := h.browser(t, kazutbHost)
	signInWithSSO(t, linked, provider, nil)
	if name := decode[apigen.Session](t, linked.do(http.MethodGet, "/api/v1/auth/session", nil)).User.FullName; name != "Связанный Аккаунт" {
		t.Fatalf("linked account: %s", name)
	}

	// A forged state is refused.
	idp.QueueUser(ssoUser{sub: "kazutb-3003", email: "forged@qadam.test", name: "Forged", verified: true})
	forged := signInWithSSO(t, h.browser(t, kazutbHost), provider, func(q url.Values) { q.Set("state", "forged") })
	if forged != "/login?error=auth.oidc_failed" {
		t.Fatalf("forged state: %q", forged)
	}
	// Providers of one university cannot be used on another.
	rec := h.browser(t, sandboxHost).do(http.MethodGet, "/api/v1/auth/oidc/"+provider.String()+"/start", nil)
	if rec.Code != http.StatusFound || rec.Header().Get("Location") != "/login?error=auth.oidc_failed" {
		t.Fatalf("foreign provider: %d %s", rec.Code, rec.Header().Get("Location"))
	}
}

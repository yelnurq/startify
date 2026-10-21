package auth

import (
	"bytes"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/google/uuid"

	"github.com/yelnurq/startify/backend/internal/platform/apperr"
	"github.com/yelnurq/startify/backend/internal/platform/authz"
	"github.com/yelnurq/startify/backend/internal/platform/secrets"
)

func TestPasswordHashing(t *testing.T) {
	hash, err := HashPassword("Steppe-Falcon-Rises-42")
	if err != nil {
		t.Fatal(err)
	}
	if !strings.HasPrefix(hash, "$argon2id$v=19$m=19456,t=2,p=1$") {
		t.Fatalf("unexpected hash format %q", hash)
	}
	if ok, err := VerifyPassword(hash, "Steppe-Falcon-Rises-42"); err != nil || !ok {
		t.Fatalf("right password rejected: %v", err)
	}
	if ok, _ := VerifyPassword(hash, "steppe-falcon-rises-42"); ok {
		t.Fatal("wrong password accepted")
	}
	other, _ := HashPassword("Steppe-Falcon-Rises-42")
	if other == hash {
		t.Fatal("salt is not random")
	}
	for _, bad := range []string{"", "$argon2id$v=19$m=1,t=1,p=1$x", "$bcrypt$...", "$argon2id$v=18$m=19456,t=2,p=1$c2FsdA$aGFzaA"} {
		if _, err := VerifyPassword(bad, "x"); err == nil {
			t.Errorf("malformed hash %q accepted", bad)
		}
	}
}

func TestPasswordPolicy(t *testing.T) {
	weak := []struct{ password, email, name string }{
		{"short1!", "a@b.kz", ""},
		{"qwertyuiop", "a@b.kz", ""},
		{"Password2024!", "a@b.kz", ""},
		{"йцукенгшщз", "a@b.kz", ""},
		{"1111111111", "a@b.kz", ""},
		{"aaaaaaaaaaaa", "a@b.kz", ""},
		{"aigerim.s2006", "aigerim.s@qadam.kz", ""},
		{"Sarsenova2005!", "x@qadam.kz", "Aigerim Sarsenova"},
		{strings.Repeat("Long-Pass-", 13), "a@b.kz", ""},
	}
	for _, w := range weak {
		if err := CheckPassword(w.password, w.email, w.name); !apperr.Is(err, "auth.password_weak") {
			t.Errorf("%q accepted (%v)", w.password, err)
		}
	}
	for _, ok := range []string{"Steppe-Falcon-Rises-42", "Qadam-Demo-2026!", "көктем жаңбыры 2026", "correct horse battery"} {
		if err := CheckPassword(ok, "student@qadam.kz", "Aigerim Sarsenova"); err != nil {
			t.Errorf("%q rejected: %v", ok, err)
		}
	}
}

func testKeys(t *testing.T) *secrets.Keyring {
	t.Helper()
	k, err := secrets.NewKeyring(strings.Repeat("k", 40))
	if err != nil {
		t.Fatal(err)
	}
	return k
}

func TestAccessTokens(t *testing.T) {
	tokens := newAccessTokens(testKeys(t), 15*time.Minute)
	p := authz.Principal{UserID: uuid.New(), TenantID: uuid.New(), SessionID: uuid.New(), Roles: []authz.Role{authz.RoleStudent, authz.RoleMentor}}
	now := time.Now()
	s, err := tokens.issue(p, 7, now)
	if err != nil {
		t.Fatal(err)
	}
	c, err := tokens.parse(s)
	if err != nil {
		t.Fatal(err)
	}
	got, err := c.Principal()
	if err != nil || got.UserID != p.UserID || got.TenantID != p.TenantID || got.SessionID != p.SessionID || len(got.Roles) != 2 || c.RoleVersion != 7 {
		t.Fatalf("round trip: %+v %+v %v", got, c, err)
	}

	expired, _ := tokens.issue(p, 0, now.Add(-time.Hour))
	if _, err := tokens.parse(expired); err == nil {
		t.Fatal("expired token accepted")
	}
	other := newAccessTokens(testKeys(t), time.Minute)
	other.key = []byte("another-key-another-key-another-k")
	forged, _ := other.issue(p, 0, now)
	if _, err := tokens.parse(forged); err == nil {
		t.Fatal("token signed with another key accepted")
	}
	parts := strings.Split(s, ".")
	if _, err := tokens.parse("eyJhbGciOiJub25lIn0." + parts[1] + "."); err == nil {
		t.Fatal("unsigned token accepted")
	}
}

func TestOneTimeSecrets(t *testing.T) {
	keys := testKeys(t)
	token, hash := newLinkToken(keys)
	if len(token) < 40 || !bytes.Equal(linkHash(keys, token), hash) {
		t.Fatal("link token hash is not reproducible")
	}
	code, err := newCode()
	if err != nil || len(code) != 6 || strings.Trim(code, "0123456789") != "" {
		t.Fatalf("code %q: %v", code, err)
	}
	a, b, u := uuid.New(), uuid.New(), uuid.New()
	if bytes.Equal(codeHash(keys, a, u, "123456"), codeHash(keys, b, u, "123456")) {
		t.Fatal("equal codes of different tokens must not collide")
	}
	r1, h1 := newRefreshToken()
	if !bytes.Equal(refreshHash(r1), h1) {
		t.Fatal("refresh hash mismatch")
	}
}

func TestCSRFMiddleware(t *testing.T) {
	ok := http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) { w.WriteHeader(http.StatusNoContent) })
	h := CSRF(ok)
	cases := []struct {
		name   string
		method string
		path   string
		setup  func(r *http.Request)
		want   int
	}{
		{"safe method", http.MethodGet, "/api/v1/me", func(r *http.Request) { r.Header.Set("Sec-Fetch-Site", "cross-site") }, 204},
		{"anonymous same origin", http.MethodPost, "/api/v1/auth/login", func(r *http.Request) { r.Header.Set("Sec-Fetch-Site", "same-origin") }, 204},
		{"cross site", http.MethodPost, "/api/v1/auth/login", func(r *http.Request) { r.Header.Set("Sec-Fetch-Site", "cross-site") }, 403},
		{"same site subdomain", http.MethodPost, "/api/v1/auth/login", func(r *http.Request) { r.Header.Set("Sec-Fetch-Site", "same-site") }, 403},
		{"foreign origin", http.MethodPost, "/api/v1/auth/login", func(r *http.Request) { r.Header.Set("Origin", "https://evil.example") }, 403},
		{"null origin", http.MethodPost, "/api/v1/auth/login", func(r *http.Request) { r.Header.Set("Origin", "null") }, 403},
		{"session without token", http.MethodPatch, "/api/v1/me", func(r *http.Request) {
			r.AddCookie(&http.Cookie{Name: CookieAccess, Value: "x"})
			r.AddCookie(&http.Cookie{Name: CookieCSRF, Value: "t"})
		}, 403},
		{"session with wrong token", http.MethodPatch, "/api/v1/me", func(r *http.Request) {
			r.AddCookie(&http.Cookie{Name: CookieRefresh, Value: "x"})
			r.AddCookie(&http.Cookie{Name: CookieCSRF, Value: "t"})
			r.Header.Set(HeaderCSRF, "u")
		}, 403},
		{"session with token", http.MethodPatch, "/api/v1/me", func(r *http.Request) {
			r.AddCookie(&http.Cookie{Name: CookieAccess, Value: "x"})
			r.AddCookie(&http.Cookie{Name: CookieCSRF, Value: "t"})
			r.Header.Set(HeaderCSRF, "t")
			r.Header.Set("Origin", "http://kazutb.localhost")
		}, 204},
		{"stale cookie on sign in", http.MethodPost, "/api/v1/auth/login", func(r *http.Request) {
			r.AddCookie(&http.Cookie{Name: CookieRefresh, Value: "x"})
		}, 204},
	}
	for _, c := range cases {
		req := httptest.NewRequest(c.method, "http://kazutb.localhost"+c.path, http.NoBody)
		c.setup(req)
		rec := httptest.NewRecorder()
		h.ServeHTTP(rec, req)
		if rec.Code != c.want {
			t.Errorf("%s: status %d, want %d", c.name, rec.Code, c.want)
		}
	}
}

func TestSafeRedirect(t *testing.T) {
	for in, want := range map[string]string{
		"/team":                 "/team",
		"/profiles/aru?tab=1":   "/profiles/aru?tab=1",
		"//evil.example":        "/",
		`/\evil.example`:        "/",
		"https://evil.example/": "/",
		"team":                  "/",
		"/ok\r\nSet-Cookie: x":  "/",
	} {
		v := in
		if got := safeRedirect(&v); got != want {
			t.Errorf("safeRedirect(%q) = %q, want %q", in, got, want)
		}
	}
	if safeRedirect(nil) != "/" {
		t.Error("nil redirect")
	}
}

func TestEmailDomains(t *testing.T) {
	domains := []string{"kazutb.edu.kz", "@stud.kazutb.kz"}
	for email, want := range map[string]bool{
		"a@kazutb.edu.kz":         true,
		"a@mail.kazutb.edu.kz":    true,
		"b@stud.kazutb.kz":        true,
		"c@gmail.com":             false,
		"d@evilkazutb.edu.kz":     false,
		"e@kazutb.edu.kz.evil.kz": false,
	} {
		if got := domainAllowed(email, domains); got != want {
			t.Errorf("%s: %v", email, got)
		}
	}
	if !domainAllowed("x@any.org", nil) {
		t.Error("no restriction must allow every domain")
	}
}

func TestCookies(t *testing.T) {
	jar := cookieJar{secure: true, accessTTL: 15 * time.Minute, refreshTTL: 720 * time.Hour}
	cookies := jar.session("access", "refresh")
	if len(cookies) != 3 {
		t.Fatal(cookies)
	}
	for _, c := range cookies {
		if !c.Secure {
			t.Errorf("%s not secure", c.Name)
		}
		switch c.Name {
		case CookieAccess:
			if !c.HttpOnly || c.Path != "/api" || c.MaxAge != 900 {
				t.Errorf("access cookie %+v", c)
			}
		case CookieRefresh:
			if !c.HttpOnly || c.Path != refreshPath || c.SameSite != http.SameSiteStrictMode {
				t.Errorf("refresh cookie %+v", c)
			}
		case CookieCSRF:
			if c.HttpOnly || c.Value == "" {
				t.Errorf("csrf cookie %+v", c)
			}
		}
	}
	for _, c := range jar.cleared() {
		if c.MaxAge >= 0 {
			t.Errorf("%s not cleared", c.Name)
		}
	}
}

package auth

import (
	"encoding/json"
	"net/http"
	"time"

	"github.com/yelnurq/startify/backend/internal/platform/httpx"
	"github.com/yelnurq/startify/backend/internal/platform/secrets"
)

// Cookie names. The access and refresh tokens are invisible to scripts; the
// CSRF token is read by the web client and echoed in X-CSRF-Token.
const (
	CookieAccess  = "qadam_at"
	CookieRefresh = "qadam_rt"
	CookieCSRF    = "qadam_csrf"
	CookieOIDC    = "qadam_oidc"

	HeaderCSRF = "X-CSRF-Token"

	refreshPath = "/api/v1/auth"
	oidcPath    = "/api/v1/auth/oidc"
)

// cookieJar builds the session cookies.
type cookieJar struct {
	secure     bool
	accessTTL  time.Duration
	refreshTTL time.Duration
}

func (j cookieJar) cookie(name, value, path string, ttl time.Duration, httpOnly bool, sameSite http.SameSite) *http.Cookie {
	// Secure follows the scheme of the public URL (plain HTTP only in
	// development); the CSRF cookie is meant to be read by the web client.
	c := &http.Cookie{ //nolint:gosec // see above
		Name: name, Value: value, Path: path, HttpOnly: httpOnly, Secure: j.secure, SameSite: sameSite,
		MaxAge: int(ttl.Seconds()),
	}
	if ttl <= 0 {
		c.MaxAge = -1
	}
	return c
}

func (j cookieJar) access(token string) *http.Cookie {
	return j.cookie(CookieAccess, token, "/api", j.accessTTL, true, http.SameSiteLaxMode)
}

func (j cookieJar) refresh(token string) *http.Cookie {
	// Only the web client's own requests use the refresh token.
	return j.cookie(CookieRefresh, token, refreshPath, j.refreshTTL, true, http.SameSiteStrictMode)
}

func (j cookieJar) csrf(token string) *http.Cookie {
	return j.cookie(CookieCSRF, token, "/", j.refreshTTL, false, http.SameSiteLaxMode)
}

// session returns the cookies of a new session.
func (j cookieJar) session(access, refresh string) []*http.Cookie {
	return []*http.Cookie{j.access(access), j.refresh(refresh), j.csrf(tokenEncoding.EncodeToString(secrets.RandomToken(24)))}
}

// cleared expires every session cookie.
func (j cookieJar) cleared() []*http.Cookie {
	return []*http.Cookie{
		j.cookie(CookieAccess, "", "/api", 0, true, http.SameSiteLaxMode),
		j.cookie(CookieRefresh, "", refreshPath, 0, true, http.SameSiteStrictMode),
		j.cookie(CookieCSRF, "", "/", 0, false, http.SameSiteLaxMode),
	}
}

func (j cookieJar) oidcState(value string) *http.Cookie {
	// Lax: the identity provider returns with a top-level GET navigation.
	return j.cookie(CookieOIDC, value, oidcPath, oidcStateTTL, true, http.SameSiteLaxMode)
}

func (j cookieJar) oidcCleared() *http.Cookie {
	return j.cookie(CookieOIDC, "", oidcPath, 0, true, http.SameSiteLaxMode)
}

// cookieResponse writes a status with cookies and a JSON body or a redirect.
// It implements the response interfaces of every operation that changes the
// session, since the generated types carry a single Set-Cookie header.
type cookieResponse struct {
	status      int
	cookies     []*http.Cookie
	body        any
	contentType string
	location    string
}

func (c cookieResponse) write(w http.ResponseWriter) error {
	for _, ck := range c.cookies {
		http.SetCookie(w, ck)
	}
	w.Header().Set("Cache-Control", "no-store")
	if c.location != "" {
		w.Header().Set("Location", c.location)
	}
	if c.body == nil {
		w.WriteHeader(c.status)
		return nil
	}
	ct := c.contentType
	if ct == "" {
		ct = "application/json"
	}
	w.Header().Set("Content-Type", ct)
	w.WriteHeader(c.status)
	return json.NewEncoder(w).Encode(c.body)
}

// problemResponse renders an error and replaces the session cookies.
func problemResponse(r *http.Request, err error, cookies []*http.Cookie) cookieResponse {
	p := httpx.ProblemFrom(r, err)
	return cookieResponse{status: p.Status, cookies: cookies, body: p, contentType: httpx.ContentTypeProblem}
}

func (c cookieResponse) VisitLoginResponse(w http.ResponseWriter) error { return c.write(w) }

func (c cookieResponse) VisitRegisterResponse(w http.ResponseWriter) error { return c.write(w) }

func (c cookieResponse) VisitVerifyLoginCodeResponse(w http.ResponseWriter) error { return c.write(w) }

func (c cookieResponse) VisitVerifyEmailResponse(w http.ResponseWriter) error { return c.write(w) }

func (c cookieResponse) VisitRefreshSessionResponse(w http.ResponseWriter) error { return c.write(w) }

func (c cookieResponse) VisitLogoutResponse(w http.ResponseWriter) error { return c.write(w) }

func (c cookieResponse) VisitRevokeSessionResponse(w http.ResponseWriter) error { return c.write(w) }

func (c cookieResponse) VisitStartOidcResponse(w http.ResponseWriter) error { return c.write(w) }

func (c cookieResponse) VisitCompleteOidcResponse(w http.ResponseWriter) error { return c.write(w) }

func (c cookieResponse) VisitRevokeAllSessionsResponse(w http.ResponseWriter) error {
	return c.write(w)
}

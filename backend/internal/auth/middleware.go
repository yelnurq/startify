package auth

import (
	"crypto/subtle"
	"log/slog"
	"net/http"
	"net/url"

	"github.com/yelnurq/startify/backend/internal/platform/apperr"
	"github.com/yelnurq/startify/backend/internal/platform/authz"
	"github.com/yelnurq/startify/backend/internal/platform/httpx"
	"github.com/yelnurq/startify/backend/internal/platform/tenancy"
)

var errCSRF = apperr.Forbidden("auth.csrf_failed", "the request did not come from this application")

// signInPaths act before a session exists: a stale cookie left from an ended
// session must not block signing in again. Sec-Fetch-Site and Origin still apply.
var signInPaths = map[string]bool{
	"/api/v1/auth/login":           true,
	"/api/v1/auth/register":        true,
	"/api/v1/auth/otp/request":     true,
	"/api/v1/auth/otp/verify":      true,
	"/api/v1/auth/password/forgot": true,
	"/api/v1/auth/password/reset":  true,
	"/api/v1/auth/email/verify":    true,
	"/api/v1/auth/email/resend":    true,
}

// Authenticate attaches the principal of a valid access token to the request.
// Requests without one continue anonymously; operations that need a user
// answer 401, and the web client refreshes the session and retries.
func (s *Service) Authenticate(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		c, err := r.Cookie(CookieAccess)
		if err != nil || c.Value == "" {
			next.ServeHTTP(w, r)
			return
		}
		t, ok := tenancy.FromContext(r.Context())
		if !ok {
			next.ServeHTTP(w, r)
			return
		}
		claims, err := s.tokens.parse(c.Value)
		if err != nil || claims.TenantID != t.ID {
			next.ServeHTTP(w, r)
			return
		}
		p, err := claims.Principal()
		if err != nil {
			next.ServeHTTP(w, r)
			return
		}
		live, err := s.revocations.current(r.Context(), claims, p.UserID)
		if err != nil {
			// Without the revocation store a revoked session cannot be told
			// apart from a live one: fail closed.
			slog.ErrorContext(r.Context(), "session check unavailable", slog.Any("error", err))
			httpx.WriteError(w, r, apperr.Unavailable("auth.unavailable", "sign-in is temporarily unavailable"))
			return
		}
		if !live {
			next.ServeHTTP(w, r)
			return
		}
		next.ServeHTTP(w, r.WithContext(authz.WithPrincipal(r.Context(), p)))
	})
}

// CSRF rejects state-changing requests from other sites. Browsers report the
// initiator in Sec-Fetch-Site and Origin; requests carrying session cookies
// must also echo the CSRF cookie in X-CSRF-Token (double submit).
func CSRF(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.Method {
		case http.MethodGet, http.MethodHead, http.MethodOptions:
			next.ServeHTTP(w, r)
			return
		}
		if site := r.Header.Get("Sec-Fetch-Site"); site != "" && site != "same-origin" && site != "none" {
			httpx.WriteError(w, r, errCSRF)
			return
		}
		if origin := r.Header.Get("Origin"); origin != "" {
			u, err := url.Parse(origin)
			if err != nil || u.Host != r.Host {
				httpx.WriteError(w, r, errCSRF)
				return
			}
		}
		if hasSessionCookie(r) && !signInPaths[r.URL.Path] {
			c, err := r.Cookie(CookieCSRF)
			header := r.Header.Get(HeaderCSRF)
			if err != nil || c.Value == "" || subtle.ConstantTimeCompare([]byte(c.Value), []byte(header)) != 1 {
				httpx.WriteError(w, r, errCSRF)
				return
			}
		}
		next.ServeHTTP(w, r)
	})
}

func hasSessionCookie(r *http.Request) bool {
	for _, name := range []string{CookieAccess, CookieRefresh} {
		if c, err := r.Cookie(name); err == nil && c.Value != "" {
			return true
		}
	}
	return false
}

package httpx

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"mime"
	"net"
	"net/http"
	"net/netip"
	"regexp"
	"runtime/debug"
	"slices"
	"strings"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/go-chi/chi/v5/middleware"
	"github.com/google/uuid"

	"github.com/yelnurq/startify/backend/internal/platform/apperr"
	"github.com/yelnurq/startify/backend/internal/platform/logger"
)

type ctxKey int

const (
	requestIDKey ctxKey = iota
	clientIPKey
	userAgentKey
	requestKey
)

const maxUserAgent = 512

var requestIDPattern = regexp.MustCompile(`^[A-Za-z0-9._-]{8,64}$`)

// RequestID propagates a well-formed incoming X-Request-ID or assigns a new one.
func RequestID(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		id := r.Header.Get("X-Request-ID")
		if !requestIDPattern.MatchString(id) {
			id = uuid.Must(uuid.NewV7()).String()
		}
		w.Header().Set("X-Request-ID", id)
		ctx := context.WithValue(r.Context(), requestIDKey, id)
		ctx = logger.With(ctx, slog.String("request_id", id))
		next.ServeHTTP(w, r.WithContext(ctx))
	})
}

// RequestIDFrom returns the request id stored by RequestID.
func RequestIDFrom(ctx context.Context) string {
	id, _ := ctx.Value(requestIDKey).(string)
	return id
}

// ClientIP resolves the client address, honouring X-Forwarded-For only when the
// direct peer is one of the trusted proxies. Exactly one proxy hop is trusted:
// the client is the address that proxy saw, the last entry of the header.
// Nginx replaces the header with its $remote_addr instead of appending to it,
// so earlier entries are whatever the client claimed and are ignored; a TLS
// proxy in front of Nginx is handled by Nginx's real_ip module, not here.
func ClientIP(trusted []netip.Prefix) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			ip := resolveClientIP(r, trusted)
			ua := r.UserAgent()
			if len(ua) > maxUserAgent {
				ua = ua[:maxUserAgent]
			}
			ctx := context.WithValue(r.Context(), clientIPKey, ip)
			ctx = context.WithValue(ctx, userAgentKey, ua)
			next.ServeHTTP(w, r.WithContext(ctx))
		})
	}
}

func resolveClientIP(r *http.Request, trusted []netip.Prefix) netip.Addr {
	peer := parseAddr(r.RemoteAddr)
	if !peer.IsValid() || !slices.ContainsFunc(trusted, func(p netip.Prefix) bool { return p.Contains(peer) }) {
		return peer
	}
	values := r.Header.Values("X-Forwarded-For")
	if len(values) == 0 {
		return peer
	}
	hops := strings.Split(values[len(values)-1], ",")
	if hop := parseAddr(strings.TrimSpace(hops[len(hops)-1])); hop.IsValid() {
		return hop
	}
	return peer
}

func parseAddr(s string) netip.Addr {
	if host, _, err := net.SplitHostPort(s); err == nil {
		s = host
	}
	a, err := netip.ParseAddr(s)
	if err != nil {
		return netip.Addr{}
	}
	return a.Unmap()
}

// ClientIPFrom returns the address resolved by ClientIP.
func ClientIPFrom(ctx context.Context) netip.Addr {
	ip, _ := ctx.Value(clientIPKey).(netip.Addr)
	return ip
}

// UserAgentFrom returns the (truncated) User-Agent stored by ClientIP.
func UserAgentFrom(ctx context.Context) string {
	ua, _ := ctx.Value(userAgentKey).(string)
	return ua
}

// Recoverer converts panics into 500 problem responses and logs the stack.
func Recoverer(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		defer func() {
			rec := recover()
			if rec == nil {
				return
			}
			if err, ok := rec.(error); ok && errors.Is(err, http.ErrAbortHandler) {
				panic(rec)
			}
			slog.ErrorContext(r.Context(), "panic in handler",
				slog.Any("panic", rec), slog.String("stack", string(debug.Stack())))
			WriteError(w, r, apperr.Internal(fmt.Errorf("panic: %v", rec)))
		}()
		next.ServeHTTP(w, r)
	})
}

// AccessLog writes one structured record per request.
func AccessLog(log *slog.Logger) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			start := time.Now()
			ww := middleware.NewWrapResponseWriter(w, r.ProtoMajor)
			next.ServeHTTP(ww, r)

			status := ww.Status()
			if status == 0 {
				status = http.StatusOK
			}
			level := slog.LevelInfo
			switch {
			case status >= http.StatusInternalServerError:
				level = slog.LevelError
			case status >= http.StatusBadRequest:
				level = slog.LevelWarn
			}
			route := ""
			if rc := chi.RouteContext(r.Context()); rc != nil {
				route = rc.RoutePattern()
			}
			log.LogAttrs(r.Context(), level, "http request",
				slog.String("method", r.Method),
				slog.String("path", r.URL.Path),
				slog.String("route", route),
				slog.Int("status", status),
				slog.Int("bytes", ww.BytesWritten()),
				slog.Duration("duration", time.Since(start)),
				slog.String("ip", ClientIPFrom(r.Context()).String()),
			)
		})
	}
}

// SecurityHeaders sets conservative headers for API responses. HTML pages
// rendered by the server set their own Content-Security-Policy.
func SecurityHeaders(hsts bool) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			h := w.Header()
			h.Set("X-Content-Type-Options", "nosniff")
			h.Set("Referrer-Policy", "strict-origin-when-cross-origin")
			h.Set("X-Frame-Options", "DENY")
			h.Set("Cross-Origin-Opener-Policy", "same-origin")
			h.Set("Cross-Origin-Resource-Policy", "same-origin")
			h.Set("Permissions-Policy", "camera=(), microphone=(), geolocation=(), payment=()")
			h.Set("Content-Security-Policy", "default-src 'none'; frame-ancestors 'none'; base-uri 'none'")
			if hsts {
				h.Set("Strict-Transport-Security", "max-age=63072000; includeSubDomains")
			}
			next.ServeHTTP(w, r)
		})
	}
}

// MaxBody limits the request body size; multipart uploads may be as large as
// uploadLimit (handlers enforce the limit of each kind of file).
func MaxBody(limit, uploadLimit int64) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			n := limit
			if mt, _, err := mime.ParseMediaType(r.Header.Get("Content-Type")); err == nil && mt == "multipart/form-data" {
				n = uploadLimit
			}
			r.Body = http.MaxBytesReader(w, r.Body, n)
			next.ServeHTTP(w, r)
		})
	}
}

// KeepRequest makes the request available to handlers that see only a
// context (the generated strict server): cookies and conditional headers.
func KeepRequest(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		next.ServeHTTP(w, r.WithContext(context.WithValue(r.Context(), requestKey, r)))
	})
}

// RequestFrom returns the request stored by KeepRequest.
func RequestFrom(ctx context.Context) (*http.Request, bool) {
	r, ok := ctx.Value(requestKey).(*http.Request)
	return r, ok
}

// Timeout bounds the request context. Server-sent event streams are exempt:
// they hold the connection open on purpose and manage their own lifetime.
func Timeout(d time.Duration) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			if strings.Contains(r.Header.Get("Accept"), "text/event-stream") {
				next.ServeHTTP(w, r)
				return
			}
			ctx, cancel := context.WithTimeout(r.Context(), d)
			defer cancel()
			next.ServeHTTP(w, r.WithContext(ctx))
		})
	}
}

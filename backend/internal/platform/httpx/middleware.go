package httpx

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
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

// NoStoreByDefault marks API responses private and not to be stored unless
// the handler chooses its own caching (public pictures; the showcase, which
// revalidates). Answers depend on the signed-in member: neither a shared
// cache nor the browser's history may show them to someone else, e.g. after
// signing out on a shared computer.
func NoStoreByDefault(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// Handlers that cache differently replace the header.
		w.Header().Set("Cache-Control", "private, no-store")
		next.ServeHTTP(w, r)
	})
}

// MaxBody limits the request body size. Requests to the upload routes may be
// as large as uploadLimit (handlers enforce the limit of each kind of file);
// the route decides, not the Content-Type header, which any client can set.
func MaxBody(limit, uploadLimit int64, uploads Routes) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			n := limit
			if uploads.Match(r) {
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

// Timeout bounds the request context. The event-stream routes are exempt:
// they hold the connection open on purpose, end with the model call and set
// their own write deadlines (NewSSE). The route decides, not the Accept
// header, so a client cannot lift the timeout of any other endpoint.
func Timeout(d time.Duration, streams Routes) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			if streams.Match(r) {
				next.ServeHTTP(w, r)
				return
			}
			ctx, cancel := context.WithTimeout(r.Context(), d)
			defer cancel()
			next.ServeHTTP(w, r.WithContext(ctx))
		})
	}
}

// Routes matches requests against route templates such as
// "POST /api/v1/projects/{projectId}/steps/{stepKey}/mentor", where a {name}
// segment stands for any non-empty segment.
type Routes struct {
	byMethod map[string][][]string
}

// NewRoutes parses "METHOD /path" templates.
func NewRoutes(templates ...string) Routes {
	rs := Routes{byMethod: map[string][][]string{}}
	for _, t := range templates {
		method, path, _ := strings.Cut(t, " ")
		rs.byMethod[method] = append(rs.byMethod[method], strings.Split(path, "/"))
	}
	return rs
}

// Match reports whether the request fits one of the templates. Like chi, it
// compares the escaped path when the request has one.
func (rs Routes) Match(r *http.Request) bool {
	templates := rs.byMethod[r.Method]
	if len(templates) == 0 {
		return false
	}
	path := r.URL.RawPath
	if path == "" {
		path = r.URL.Path
	}
	segments := strings.Split(path, "/")
	return slices.ContainsFunc(templates, func(template []string) bool {
		if len(template) != len(segments) {
			return false
		}
		for i, s := range template {
			if strings.HasPrefix(s, "{") && strings.HasSuffix(s, "}") {
				if segments[i] == "" {
					return false
				}
			} else if s != segments[i] {
				return false
			}
		}
		return true
	})
}

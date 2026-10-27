package httpx

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"io"
	"net/http"
	"regexp"
	"strconv"
	"time"

	"github.com/yelnurq/startify/backend/internal/platform/apperr"
)

// IdempotencyRecord is the stored outcome of a request made with an Idempotency-Key.
type IdempotencyRecord struct {
	Fingerprint string
	Completed   bool
	Status      int
	Header      http.Header
	Body        []byte
}

// IdempotencyStore persists idempotency keys.
type IdempotencyStore interface {
	// Reserve claims (scope, key) for a new request. If the key is already taken
	// it returns the stored record with fresh == false.
	Reserve(ctx context.Context, scope, key, fingerprint string, ttl time.Duration) (rec IdempotencyRecord, fresh bool, err error)
	// Complete stores the response of a reserved request.
	Complete(ctx context.Context, scope, key string, rec IdempotencyRecord) error
	// Release frees a reservation so the client may retry, e.g. after a 5xx.
	Release(ctx context.Context, scope, key string) error
}

// IdempotencyScopeFunc returns the namespace of keys for a request (tenant and
// user); ok == false disables idempotency for the request.
type IdempotencyScopeFunc func(r *http.Request) (scope string, ok bool)

// HeaderIdempotentReplayed marks a response replayed from the idempotency store.
const HeaderIdempotentReplayed = "Idempotent-Replayed"

var idempotencyKeyPattern = regexp.MustCompile(`^[\x21-\x7E]{8,255}$`)

// replayedHeaders are the response headers stored and replayed with a response.
var replayedHeaders = []string{"Content-Type", "Location", "ETag", "Content-Language"}

// Idempotency makes POST requests carrying an Idempotency-Key safe to retry:
// the first response is stored and replayed for repeated keys; a reused key with
// a different payload is rejected.
func Idempotency(store IdempotencyStore, scopeOf IdempotencyScopeFunc, ttl time.Duration) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			key := r.Header.Get("Idempotency-Key")
			if key == "" || r.Method != http.MethodPost {
				next.ServeHTTP(w, r)
				return
			}
			if !idempotencyKeyPattern.MatchString(key) {
				WriteError(w, r, apperr.BadRequest("idempotency.invalid_key",
					"Idempotency-Key must be 8-255 printable ASCII characters"))
				return
			}
			scope, ok := scopeOf(r)
			if !ok {
				next.ServeHTTP(w, r)
				return
			}

			body, err := io.ReadAll(r.Body)
			if err != nil {
				WriteError(w, r, err)
				return
			}
			r.Body = io.NopCloser(bytes.NewReader(body))
			fingerprint := requestFingerprint(r, body)

			ctx := r.Context()
			rec, fresh, err := store.Reserve(ctx, scope, key, fingerprint, ttl)
			if err != nil {
				WriteError(w, r, apperr.Internal(err))
				return
			}
			if !fresh {
				switch {
				case rec.Fingerprint != fingerprint:
					WriteError(w, r, apperr.New(apperr.KindValidation, "idempotency.key_reused",
						"the Idempotency-Key was already used with a different request"))
				case !rec.Completed:
					WriteError(w, r, apperr.Conflict("idempotency.in_progress",
						"a request with this Idempotency-Key is still being processed"))
				default:
					replay(w, rec)
				}
				return
			}

			rw := &recordingWriter{ResponseWriter: w, status: http.StatusOK}
			next.ServeHTTP(rw, r)

			// The outcome must be recorded even if the client disconnected.
			saveCtx, cancel := context.WithTimeout(context.WithoutCancel(ctx), 5*time.Second)
			defer cancel()
			if rw.status >= http.StatusInternalServerError {
				_ = store.Release(saveCtx, scope, key)
				return
			}
			stored := IdempotencyRecord{
				Fingerprint: fingerprint,
				Completed:   true,
				Status:      rw.status,
				Header:      http.Header{},
				Body:        rw.body.Bytes(),
			}
			for _, h := range replayedHeaders {
				if v := rw.Header().Get(h); v != "" {
					stored.Header.Set(h, v)
				}
			}
			_ = store.Complete(saveCtx, scope, key, stored)
		})
	}
}

func requestFingerprint(r *http.Request, body []byte) string {
	h := sha256.New()
	h.Write([]byte(r.Method))
	h.Write([]byte{0})
	h.Write([]byte(r.URL.RequestURI()))
	h.Write([]byte{0})
	h.Write(body)
	return hex.EncodeToString(h.Sum(nil))
}

func replay(w http.ResponseWriter, rec IdempotencyRecord) {
	for k, vs := range rec.Header {
		for _, v := range vs {
			w.Header().Add(k, v)
		}
	}
	w.Header().Set(HeaderIdempotentReplayed, "true")
	w.Header().Set("Content-Length", strconv.Itoa(len(rec.Body)))
	w.WriteHeader(rec.Status)
	_, _ = w.Write(rec.Body)
}

type recordingWriter struct {
	http.ResponseWriter
	status      int
	wroteHeader bool
	body        bytes.Buffer
}

func (w *recordingWriter) WriteHeader(status int) {
	if !w.wroteHeader {
		w.status = status
		w.wroteHeader = true
	}
	w.ResponseWriter.WriteHeader(status)
}

func (w *recordingWriter) Write(p []byte) (int, error) {
	if !w.wroteHeader {
		w.WriteHeader(http.StatusOK)
	}
	w.body.Write(p)
	return w.ResponseWriter.Write(p)
}

func (w *recordingWriter) Unwrap() http.ResponseWriter { return w.ResponseWriter }

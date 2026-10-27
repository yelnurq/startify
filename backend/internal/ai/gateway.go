// Package ai is the only way modules reach language models. The Gateway picks
// a provider and model per university and feature with fallbacks, hides
// personal data from external providers, validates structured output against
// its schema (with one corrective retry), enforces daily limits and records
// tokens, cost and prompt version of every call (D-031).
package ai

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"log/slog"
	"math"
	"strconv"
	"strings"
	"time"
	"unicode/utf8"

	"github.com/google/uuid"

	"github.com/yelnurq/startify/backend/internal/ai/aidb"
	"github.com/yelnurq/startify/backend/internal/ai/llm"
	"github.com/yelnurq/startify/backend/internal/ai/mock"
	"github.com/yelnurq/startify/backend/internal/ai/redact"
	"github.com/yelnurq/startify/backend/internal/ai/schema"
	"github.com/yelnurq/startify/backend/internal/platform/apperr"
	"github.com/yelnurq/startify/backend/internal/platform/authz"
	"github.com/yelnurq/startify/backend/internal/platform/config"
	"github.com/yelnurq/startify/backend/internal/platform/db"
	"github.com/yelnurq/startify/backend/internal/platform/tenancy"
)

// Usage statuses.
const (
	statusOK            = "ok"
	statusError         = "error"
	statusRefused       = "refused"
	statusInvalidOutput = "invalid_output"
	statusCancelled     = "cancelled"
)

// Default limits of a university without its own settings.
const (
	DefaultDailyUserTokens   = 200_000
	DefaultDailyTenantTokens = 20_000_000
)

// Errors with stable codes.
var (
	errUnavailable   = apperr.Unavailable("ai.unavailable", "no language model is available right now")
	errRefused       = apperr.New(apperr.KindValidation, "ai.refused", "the model declined this request")
	errInvalidOutput = apperr.New(apperr.KindUnavailable, "ai.invalid_output", "the model answer could not be understood")
)

// Gateway routes model calls.
type Gateway struct {
	db        *db.DB
	providers map[string]llm.Provider
	order     []string
	defaults  Routes
	zone      *time.Location
	log       *slog.Logger
	now       func() time.Time
	// minSimilarity is the relevance floor of knowledge passages.
	minSimilarity float64
}

// mockMinSimilarity is the relevance floor of the mock's hashed vectors:
// they only overlap on shared words, so related texts score about 0.1–0.5
// and unrelated ones below 0.05.
const mockMinSimilarity = 0.06

// embeds reports whether a provider can serve embeddings.
func embeds(p llm.Provider) bool { return p.DefaultModel(llm.FeatureEmbedding) != "" }

// NewGateway returns the gateway over the configured providers. Without a
// database (the evaluation harness, `make ai-eval`) Complete, Stream and Embed
// use the default settings, no limits and record nothing; the rest of the
// gateway needs the database.
func NewGateway(database *db.DB, providers []llm.Provider, cfg config.AI, log *slog.Logger) (*Gateway, error) {
	zone, err := time.LoadLocation(cfg.Timezone)
	if err != nil {
		return nil, fmt.Errorf("ai: time zone: %w", err)
	}
	g := &Gateway{db: database, providers: map[string]llm.Provider{}, zone: zone, log: log, now: time.Now, minSimilarity: cfg.EmbeddingMinSimilarity}
	for _, p := range providers {
		g.providers[p.Name()] = p
		g.order = append(g.order, p.Name())
	}
	configured, err := ParseRoutes(cfg.Routes)
	if err != nil {
		return nil, err
	}
	g.defaults = Routes{}
	for _, f := range llm.Features {
		routes := configured[f]
		if len(routes) == 0 {
			routes = []Route{{Provider: cfg.Default}}
			// Without embeddings at the default provider (Anthropic, a server
			// without an embedding model) use the local server, else the mock.
			if f == llm.FeatureEmbedding {
				for _, name := range []string{cfg.Default, "openai", "mock"} {
					if p, ok := g.providers[name]; ok && embeds(p) {
						routes = []Route{{Provider: name}}
						break
					}
				}
			}
		}
		for _, r := range routes {
			p, ok := g.providers[r.Provider]
			if !ok {
				return nil, fmt.Errorf("ai: feature %s routes to %q, which is not configured", f, r.Provider)
			}
			if f == llm.FeatureEmbedding && !embeds(p) {
				return nil, fmt.Errorf("ai: feature embedding routes to %q, which has no embedding model", r.Provider)
			}
		}
		g.defaults[f] = routes
	}
	for _, name := range g.order {
		p := g.providers[name]
		if p.Local() {
			continue
		}
		attrs := []any{slog.String("provider", name)}
		if e, ok := p.(interface{ Endpoint() string }); ok {
			attrs = append(attrs, slog.String("endpoint", e.Endpoint()),
				slog.String("hint", "set QADAM_AI_OPENAI_LOCAL=true only for a server inside the university network"))
		}
		log.Warn("AI provider outside the university network: personal data is masked before sending and users are warned", attrs...)
	}
	return g, nil
}

// MinSimilarity is the cosine similarity below which a passage embedded by
// model is not related to a question.
func (g *Gateway) MinSimilarity(model string) float64 {
	if model == mock.EmbeddingModel {
		return mockMinSimilarity
	}
	return g.minSimilarity
}

// SetClock replaces the clock (tests).
func (g *Gateway) SetClock(now func() time.Time) { g.now = now }

// Provider describes a configured provider.
type Provider struct {
	Name  string
	Local bool
	// Models are the defaults per feature.
	Models map[llm.Feature]string
}

// Providers lists the configured providers.
func (g *Gateway) Providers() []Provider {
	out := make([]Provider, 0, len(g.order))
	for _, name := range g.order {
		p := g.providers[name]
		models := map[llm.Feature]string{}
		for _, f := range llm.Features {
			if m := p.DefaultModel(f); m != "" {
				models[f] = m
			}
		}
		out = append(out, Provider{Name: name, Local: p.Local(), Models: models})
	}
	return out
}

// DefaultRoutes are the routes from the server configuration.
func (g *Gateway) DefaultRoutes() Routes { return g.defaults }

// Settings are a university's choices.
type Settings struct {
	// Routes override the defaults per feature.
	Routes            Routes
	LocalOnly         bool
	DailyUserTokens   int
	DailyTenantTokens int64
	MentorMode        string
	UpdatedAt         time.Time
}

// Settings returns the settings of the current university.
func (g *Gateway) Settings(ctx context.Context) (Settings, error) {
	if _, err := tenancy.Require(ctx); err != nil {
		return Settings{}, err
	}
	s := Settings{Routes: Routes{}, DailyUserTokens: DefaultDailyUserTokens, DailyTenantTokens: DefaultDailyTenantTokens, MentorMode: "coach"}
	if g.db == nil {
		return s, nil
	}
	err := g.db.ReadTx(ctx, func(ctx context.Context) error {
		row, err := aidb.New(g.db.Conn(ctx)).GetSettings(ctx)
		if db.IsNoRows(err) {
			return nil
		}
		if err != nil {
			return err
		}
		s = settingsOf(row)
		return nil
	})
	if err != nil {
		return s, fmt.Errorf("ai: settings: %w", err)
	}
	return s, nil
}

func settingsOf(row aidb.TenantAiSetting) Settings {
	s := Settings{
		Routes: Routes{}, LocalOnly: row.LocalOnly, DailyUserTokens: int(row.DailyUserTokens),
		DailyTenantTokens: row.DailyTenantTokens, MentorMode: row.MentorMode, UpdatedAt: row.UpdatedAt,
	}
	_ = json.Unmarshal(row.Routes, &s.Routes)
	return s
}

// ETag is the version of the settings for optimistic locking; "0" before the
// university saved any.
func (s Settings) ETag() string {
	if s.UpdatedAt.IsZero() {
		return `"0"`
	}
	return `"` + strconv.FormatInt(s.UpdatedAt.UnixNano(), 36) + `"`
}

// SaveSettings stores the settings of the current university. Routes must
// name configured providers; with LocalOnly every route must be local.
func (g *Gateway) SaveSettings(ctx context.Context, s Settings, ifMatch *string) (Settings, error) {
	if _, err := tenancy.Require(ctx); err != nil {
		return s, err
	}
	for f, routes := range s.Routes {
		if !f.Valid() {
			return s, apperr.Validation(apperr.FieldError{Field: "routes", Code: "invalid"})
		}
		for _, r := range routes {
			p, ok := g.providers[r.Provider]
			if !ok {
				return s, apperr.Validation(apperr.FieldError{Field: "routes." + string(f), Code: "not_found"})
			}
			if s.LocalOnly && !p.Local() {
				return s, apperr.Validation(apperr.FieldError{Field: "routes." + string(f), Code: "external_not_allowed"})
			}
			if f == llm.FeatureEmbedding && !embeds(p) {
				return s, apperr.Validation(apperr.FieldError{Field: "routes." + string(f), Code: "not_supported"})
			}
		}
	}
	if s.MentorMode != "coach" && s.MentorMode != "coauthor" {
		return s, apperr.Validation(apperr.FieldError{Field: "mentor_mode", Code: "invalid"})
	}
	routes, err := json.Marshal(s.Routes)
	if err != nil {
		return s, fmt.Errorf("ai: routes: %w", err)
	}
	var saved Settings
	err = g.db.Tx(ctx, func(ctx context.Context) error {
		q := aidb.New(g.db.Conn(ctx))
		if ifMatch != nil {
			row, err := q.GetSettings(ctx)
			if err != nil && !db.IsNoRows(err) {
				return err
			}
			current := Settings{}
			if err == nil {
				current = settingsOf(row)
			}
			if strings.TrimPrefix(*ifMatch, "W/") != current.ETag() {
				return apperr.PreconditionFailed("precondition.failed", "the settings were changed since they were loaded")
			}
		}
		row, err := q.UpsertSettings(ctx, aidb.UpsertSettingsParams{
			Routes: routes, LocalOnly: s.LocalOnly, DailyUserTokens: int32(s.DailyUserTokens), //nolint:gosec // validated by the caller
			DailyTenantTokens: s.DailyTenantTokens, MentorMode: s.MentorMode,
		})
		if err != nil {
			return err
		}
		saved = settingsOf(row)
		return nil
	})
	if err != nil {
		if _, ok := apperr.As(err); ok {
			return saved, err
		}
		return saved, fmt.Errorf("ai: save settings: %w", err)
	}
	return saved, nil
}

// routesFor resolves the routes of a feature for a university.
func (g *Gateway) routesFor(s Settings, f llm.Feature) []Route {
	routes := s.Routes[f]
	if len(routes) == 0 {
		routes = g.defaults[f]
	}
	out := make([]Route, 0, len(routes))
	for _, r := range routes {
		p, ok := g.providers[r.Provider]
		if !ok || (s.LocalOnly && !p.Local()) || (f == llm.FeatureEmbedding && !embeds(p)) {
			continue
		}
		out = append(out, r)
	}
	return out
}

// EmbeddingModel names the vectors the current university's documents get:
// the default model of the first provider on the embedding route.
func (g *Gateway) EmbeddingModel(ctx context.Context) (string, error) {
	s, err := g.Settings(ctx)
	if err != nil {
		return "", err
	}
	routes := g.routesFor(s, llm.FeatureEmbedding)
	if len(routes) == 0 {
		return "", errUnavailable
	}
	return g.providers[routes[0].Provider].DefaultModel(llm.FeatureEmbedding), nil
}

// External reports whether a feature may send data outside the university.
func (g *Gateway) External(s Settings, f llm.Feature) bool {
	for _, r := range g.routesFor(s, f) {
		if !g.providers[r.Provider].Local() {
			return true
		}
	}
	return false
}

// Call is a request for a model answer.
type Call struct {
	Feature       llm.Feature
	PromptVersion string
	System        string
	Messages      []llm.Message
	MaxTokens     int
	Effort        llm.Effort
	// Schema, when set, makes the answer one JSON value matching it.
	Schema *llm.Schema
	Locale string
	// UserID is the member the call is made for; defaults to the signed-in user.
	UserID uuid.UUID
}

// Result is a model answer.
type Result struct {
	Text          string
	JSON          any
	Provider      string
	Model         string
	PromptVersion string
	Usage         llm.Usage
}

type prepared struct {
	tenant   tenancy.Tenant
	userID   uuid.UUID
	settings Settings
	routes   []Route
	// reservation holds the tokens of the call while it runs (uuid.Nil
	// without limits).
	reservation uuid.UUID
}

func (g *Gateway) prepare(ctx context.Context, c *Call) (prepared, error) {
	t, err := tenancy.Require(ctx)
	if err != nil {
		return prepared{}, err
	}
	if c.UserID == uuid.Nil {
		if p, ok := authz.FromContext(ctx); ok {
			c.UserID = p.UserID
		}
	}
	if c.MaxTokens == 0 {
		c.MaxTokens = 4096
	}
	settings, err := g.Settings(ctx)
	if err != nil {
		return prepared{}, err
	}
	routes := g.routesFor(settings, c.Feature)
	if len(routes) == 0 {
		return prepared{}, errUnavailable
	}
	pr := prepared{tenant: t, userID: c.UserID, settings: settings, routes: routes}
	if c.Feature != llm.FeatureEmbedding {
		if pr.reservation, err = g.reserve(ctx, settings, c.UserID, reservedTokens(*c)); err != nil {
			return prepared{}, err
		}
	}
	return pr, nil
}

// request builds the provider request of a route, redacting personal data
// for providers outside the university.
func (g *Gateway) request(c Call, pr prepared, r Route) (llm.Provider, llm.CompletionRequest, *redact.Redactor) {
	p := g.providers[r.Provider]
	model := r.Model
	if model == "" {
		model = p.DefaultModel(c.Feature)
	}
	req := llm.CompletionRequest{
		Model: model, System: c.System, MaxTokens: c.MaxTokens, Effort: c.Effort, JSONSchema: c.Schema,
		Messages: append([]llm.Message(nil), c.Messages...),
		Metadata: llm.RequestMeta{TenantID: pr.tenant.ID, UserID: pr.userID, Feature: c.Feature, PromptVersion: c.PromptVersion, Locale: c.Locale},
	}
	if p.Local() {
		return p, req, nil
	}
	red := redact.New()
	req.System = red.Redact(req.System)
	for i := range req.Messages {
		req.Messages[i].Content = red.Redact(req.Messages[i].Content)
	}
	if red.Count() > 0 {
		req.System = strings.TrimSpace(req.System + "\n\n" + redact.Notice)
	}
	return p, req, red
}

// Complete returns a model answer, trying the routes of the feature in turn.
// With a schema, Result.Text is the JSON value alone (without a Markdown
// fence or a sentence around it) and Result.JSON its decoded form.
func (g *Gateway) Complete(ctx context.Context, c Call) (*Result, error) {
	pr, err := g.prepare(ctx, &c)
	if err != nil {
		return nil, err
	}
	defer g.release(ctx, pr)
	var validator *schema.Validator
	if c.Schema != nil {
		if validator, err = schema.Compile(c.Schema.Name, c.Schema.JSON); err != nil {
			return nil, apperr.Internal(err)
		}
	}
	lastErr := error(errUnavailable)
	for _, r := range pr.routes {
		p, req, red := g.request(c, pr, r)
		res, err := g.attempt(ctx, c, pr, p, req, red, validator)
		if err == nil {
			return res, nil
		}
		if ctx.Err() != nil || errors.Is(err, errInvalidOutput) {
			return nil, err
		}
		lastErr = err
		g.log.WarnContext(ctx, "model route failed, trying the next", slog.String("feature", string(c.Feature)),
			slog.String("provider", r.Provider), slog.String("model", req.Model), slog.Any("error", err))
	}
	return nil, lastErr
}

func (g *Gateway) attempt(ctx context.Context, c Call, pr prepared, p llm.Provider, req llm.CompletionRequest,
	red *redact.Redactor, validator *schema.Validator,
) (*Result, error) {
	started := g.now()
	resp, err := p.Complete(ctx, req)
	if err != nil {
		g.record(ctx, c, pr, p.Name(), req.Model, usageOf(resp), started, failureStatus(ctx, err), err)
		return nil, publicError(ctx, err)
	}
	usage := resp.Usage
	if validator != nil && resp.StopReason == llm.StopMaxTokens {
		// A JSON value cut at the token limit can never be valid: ask again
		// with room to finish instead of "correcting" it with the same
		// limit (Н-24).
		retry := req
		retry.MaxTokens = min(req.MaxTokens*2, maxStructuredTokens)
		g.log.WarnContext(ctx, "structured model output hit the token limit, retrying with a larger one",
			slog.String("feature", string(c.Feature)), slog.String("model", req.Model), slog.Int("max_tokens", retry.MaxTokens))
		second, err := p.Complete(ctx, retry)
		if err != nil {
			g.record(ctx, c, pr, p.Name(), req.Model, usage.Add(usageOf(second)), started, failureStatus(ctx, err), err)
			return nil, publicError(ctx, err)
		}
		usage = usage.Add(second.Usage)
		if second.StopReason == llm.StopMaxTokens {
			g.record(ctx, c, pr, p.Name(), req.Model, usage, started, statusInvalidOutput, errTruncated)
			return nil, errInvalidOutput
		}
		req, resp = retry, second
	}
	text := restore(red, resp.Text)
	res := &Result{Text: text, Provider: p.Name(), Model: resp.Model, PromptVersion: c.PromptVersion}
	if validator != nil {
		doc, verr := validator.Validate(text)
		if verr != nil {
			// One corrective retry: show the model what was wrong.
			retry := req
			retry.Messages = append(append([]llm.Message(nil), req.Messages...),
				llm.Message{Role: llm.RoleAssistant, Content: resp.Text},
				llm.Message{Role: llm.RoleUser, Content: "Your reply does not match the required JSON schema: " + verr.Error() +
					". Reply again with only the corrected JSON value."})
			second, err := p.Complete(ctx, retry)
			if err != nil {
				g.record(ctx, c, pr, p.Name(), req.Model, usage.Add(usageOf(second)), started, failureStatus(ctx, err), err)
				return nil, publicError(ctx, err)
			}
			usage = usage.Add(second.Usage)
			text = restore(red, second.Text)
			doc, verr = validator.Validate(text)
			if verr != nil {
				g.record(ctx, c, pr, p.Name(), req.Model, usage, started, statusInvalidOutput, verr)
				g.log.WarnContext(ctx, "model output failed validation twice", slog.String("feature", string(c.Feature)),
					slog.String("model", req.Model), slog.Any("error", verr))
				return nil, errInvalidOutput
			}
		}
		// Callers read the value that was validated, not the reply around it:
		// local models often wrap JSON in a Markdown fence (С-16).
		res.Text = schema.Extract(text)
		res.JSON = doc
	}
	res.Usage = usage
	g.record(ctx, c, pr, p.Name(), resp.Model, usage, started, statusOK, nil)
	return res, nil
}

// maxStructuredTokens caps the larger limit of a retry after truncation.
const maxStructuredTokens = 32000

// errTruncated: the answer did not fit the token limit even after a retry.
var errTruncated = errors.New("ai: output truncated at the token limit")

func usageOf(resp *llm.CompletionResponse) llm.Usage {
	if resp == nil {
		return llm.Usage{}
	}
	return resp.Usage
}

func restore(red *redact.Redactor, text string) string {
	if red == nil {
		return text
	}
	return red.Restore(text)
}

func failureStatus(ctx context.Context, err error) string {
	switch {
	case ctx.Err() != nil || errors.Is(err, context.Canceled):
		return statusCancelled
	case errors.Is(err, llm.ErrRefused):
		return statusRefused
	default:
		return statusError
	}
}

// publicError turns a provider error into one users can be shown.
func publicError(ctx context.Context, err error) error {
	switch {
	case ctx.Err() != nil:
		return ctx.Err()
	case errors.Is(err, llm.ErrRefused):
		return errRefused
	default:
		return errUnavailable.Wrap(err)
	}
}

func (g *Gateway) record(ctx context.Context, c Call, pr prepared, provider, model string, u llm.Usage,
	started time.Time, status string, cause error,
) {
	if g.db == nil {
		return
	}
	params := aidb.InsertUsageParams{
		Feature: string(c.Feature), Provider: provider, Model: model,
		InputTokens: int32(u.InputTokens), OutputTokens: int32(u.OutputTokens), //nolint:gosec // token counts fit
		CacheReadTokens: int32(u.CacheReadTokens), CacheWriteTokens: int32(u.CacheWriteTokens), //nolint:gosec // token counts fit
		CostMicros: costMicros(model, u), LatencyMs: int32(g.now().Sub(started).Milliseconds()), //nolint:gosec // bounded by timeouts
		Status: status,
	}
	if pr.userID != uuid.Nil {
		params.UserID = &pr.userID
	}
	if c.PromptVersion != "" {
		params.PromptVersion = &c.PromptVersion
	}
	if cause != nil {
		code := errorCode(cause)
		params.ErrorCode = &code
	}
	// The call happened even if the client went away: always record it.
	recordCtx, cancel := context.WithTimeout(context.WithoutCancel(ctx), 5*time.Second)
	defer cancel()
	err := g.db.Tx(recordCtx, func(ctx context.Context) error {
		return aidb.New(g.db.Conn(ctx)).InsertUsage(ctx, params)
	})
	if err != nil {
		g.log.ErrorContext(ctx, "AI usage not recorded", slog.Any("error", err))
	}
}

func errorCode(err error) string {
	switch {
	case errors.Is(err, context.Canceled), errors.Is(err, context.DeadlineExceeded):
		return "cancelled"
	case errors.Is(err, llm.ErrRefused):
		return "refused"
	case errors.Is(err, schema.ErrInvalid):
		return "invalid_output"
	case errors.Is(err, errTruncated):
		return "truncated"
	case errors.Is(err, llm.ErrBadRequest):
		return "bad_request"
	default:
		return "unavailable"
	}
}

// dayStart is the start of today in the limits' time zone.
func (g *Gateway) dayStart() time.Time {
	now := g.now().In(g.zone)
	return time.Date(now.Year(), now.Month(), now.Day(), 0, 0, 0, 0, g.zone)
}

// reservationTTL bounds how long a call in progress holds its tokens: the
// reservation of a process that died stops counting after it. It is longer
// than any call (an evaluation job ends after 10 minutes).
const reservationTTL = 15 * time.Minute

// reservedTokens is what a call may use at most: its input (about four
// characters per token) and the longest answer it allows.
func reservedTokens(c Call) int32 {
	in := utf8.RuneCountInString(c.System)
	for _, m := range c.Messages {
		in += utf8.RuneCountInString(m.Content)
	}
	return int32(min((in+3)/4+max(c.MaxTokens, 1), math.MaxInt32)) //nolint:gosec // clamped
}

// reserve checks the daily limits and holds the tokens of a call until it
// ends. Calls in progress count with what they may use, so parallel requests
// cannot overshoot a limit by more than one call (Н-21). The check and the
// reservation are serialised per university.
func (g *Gateway) reserve(ctx context.Context, s Settings, userID uuid.UUID, tokens int32) (uuid.UUID, error) {
	if g.db == nil || (s.DailyUserTokens == 0 && s.DailyTenantTokens == 0) {
		return uuid.Nil, nil
	}
	since := g.dayStart()
	var (
		id    uuid.UUID
		scope string
	)
	var user *uuid.UUID
	if userID != uuid.Nil {
		user = &userID
	}
	err := g.db.Tx(ctx, func(ctx context.Context) error {
		q := aidb.New(g.db.Conn(ctx))
		if err := q.LockLimits(ctx); err != nil {
			return err
		}
		spent := func(user *uuid.UUID) (int64, error) {
			used, err := q.TokensSince(ctx, aidb.TokensSinceParams{Since: since, UserID: user})
			if err != nil {
				return 0, err
			}
			held, err := q.ReservedTokens(ctx, user)
			return used + held, err
		}
		if s.DailyTenantTokens > 0 {
			n, err := spent(nil)
			if err != nil {
				return err
			}
			if n >= s.DailyTenantTokens {
				scope = "university"
				return nil
			}
		}
		if s.DailyUserTokens > 0 && user != nil {
			n, err := spent(user)
			if err != nil {
				return err
			}
			if n >= int64(s.DailyUserTokens) {
				scope = "user"
				return nil
			}
		}
		var err error
		id, err = q.CreateReservation(ctx, aidb.CreateReservationParams{UserID: user, Tokens: tokens, TtlSeconds: reservationTTL.Seconds()})
		return err
	})
	if err != nil {
		return uuid.Nil, fmt.Errorf("ai: limits: %w", err)
	}
	if scope == "" {
		return id, nil
	}
	reset := since.Add(24 * time.Hour)
	wait := reset.Sub(g.now())
	return uuid.Nil, apperr.RateLimited("ai.limit_reached", "the daily AI limit is reached", wait).WithParams(map[string]any{
		"scope": scope, "resets_at": reset.Format("15:04"), "retry_after_seconds": int(wait.Seconds()) + 1,
	})
}

// release frees the tokens of a finished call; its usage is recorded by then.
func (g *Gateway) release(ctx context.Context, pr prepared) {
	if pr.reservation == uuid.Nil {
		return
	}
	ctx, cancel := context.WithTimeout(context.WithoutCancel(ctx), 5*time.Second)
	defer cancel()
	err := g.db.Tx(ctx, func(ctx context.Context) error {
		return aidb.New(g.db.Conn(ctx)).DeleteReservation(ctx, pr.reservation)
	})
	if err != nil {
		g.log.WarnContext(ctx, "AI reservation not released; it expires on its own", slog.Any("error", err))
	}
}

// PurgeReservations removes the expired reservations of every university
// (left by processes that died during a call).
func (g *Gateway) PurgeReservations(ctx context.Context, _ time.Time) (int64, error) {
	var n int64
	err := g.db.Tx(db.WithSystem(ctx), func(ctx context.Context) error {
		var err error
		n, err = aidb.New(g.db.Conn(ctx)).DeleteExpiredReservations(ctx)
		return err
	})
	if err != nil {
		return 0, fmt.Errorf("ai: purge reservations: %w", err)
	}
	return n, nil
}

// Status is what a member needs to know about AI before using it.
type Status struct {
	// External: some AI feature, embeddings included, sends data to a
	// provider outside the university.
	External   bool
	Limit      int
	Used       int64
	ResetsAt   time.Time
	MentorMode string
}

// Status describes AI for the signed-in member.
func (g *Gateway) Status(ctx context.Context) (Status, error) {
	p, err := authz.Require(ctx)
	if err != nil {
		return Status{}, err
	}
	s, err := g.Settings(ctx)
	if err != nil {
		return Status{}, err
	}
	since := g.dayStart()
	var used int64
	err = g.db.ReadTx(ctx, func(ctx context.Context) error {
		used, err = aidb.New(g.db.Conn(ctx)).TokensSince(ctx, aidb.TokensSinceParams{Since: since, UserID: &p.UserID})
		return err
	})
	if err != nil {
		return Status{}, fmt.Errorf("ai: status: %w", err)
	}
	// Every feature counts: the mentor's question is embedded to search the
	// knowledge base, so an external embedding provider sees it too (Н-27).
	external := false
	for _, f := range llm.Features {
		external = external || g.External(s, f)
	}
	return Status{External: external, Limit: s.DailyUserTokens, Used: used, ResetsAt: since.Add(24 * time.Hour), MentorMode: s.MentorMode}, nil
}

// Event is a piece of a streamed answer: text, or the final result, or an error.
type Event struct {
	Delta  string
	Result *Result
	Err    error
}

// Stream streams a model answer. Routes are tried until one starts
// streaming; a failure after the first words is reported, not retried.
// Cancelling ctx (the client went away) stops the model.
func (g *Gateway) Stream(ctx context.Context, c Call) (<-chan Event, error) {
	if c.Schema != nil {
		return nil, apperr.Internal(errors.New("ai: structured output is not streamed"))
	}
	pr, err := g.prepare(ctx, &c)
	if err != nil {
		return nil, err
	}
	lastErr := error(errUnavailable)
	for _, r := range pr.routes {
		p, req, red := g.request(c, pr, r)
		started := g.now()
		events, err := p.Stream(ctx, req)
		if err != nil {
			g.record(ctx, c, pr, p.Name(), req.Model, llm.Usage{}, started, failureStatus(ctx, err), err)
			if ctx.Err() != nil {
				g.release(ctx, pr)
				return nil, ctx.Err()
			}
			lastErr = publicError(ctx, err)
			g.log.WarnContext(ctx, "model route failed to stream, trying the next", slog.String("feature", string(c.Feature)),
				slog.String("provider", r.Provider), slog.String("model", req.Model), slog.Any("error", err))
			continue
		}
		out := make(chan Event, 16)
		go g.relay(ctx, c, pr, p, req, red, started, events, out)
		return out, nil
	}
	g.release(ctx, pr)
	return nil, lastErr
}

func (g *Gateway) relay(ctx context.Context, c Call, pr prepared, p llm.Provider, req llm.CompletionRequest,
	red *redact.Redactor, started time.Time, events <-chan llm.StreamEvent, out chan<- Event,
) {
	defer close(out)
	// Runs before close(out): the tokens are free once the caller sees the end.
	defer g.release(ctx, pr)
	var restorer *redact.Stream
	if red != nil {
		restorer = red.Stream()
	}
	var shown strings.Builder
	send := func(ev Event) bool {
		select {
		case out <- ev:
			return true
		case <-ctx.Done():
			return false
		}
	}
	for ev := range events {
		switch {
		case ev.Err != nil:
			g.record(ctx, c, pr, p.Name(), req.Model, estimate(req, shown.String()), started, failureStatus(ctx, ev.Err), ev.Err)
			send(Event{Err: publicError(ctx, ev.Err)})
			return
		case ev.Done != nil:
			if restorer != nil {
				if tail := restorer.Flush(); tail != "" {
					shown.WriteString(tail)
					if !send(Event{Delta: tail}) {
						g.record(ctx, c, pr, p.Name(), req.Model, estimate(req, shown.String()), started, statusCancelled, ctx.Err())
						return
					}
				}
			}
			g.record(ctx, c, pr, p.Name(), ev.Done.Model, ev.Done.Usage, started, statusOK, nil)
			send(Event{Result: &Result{
				Text: restore(red, ev.Done.Text), Provider: p.Name(), Model: ev.Done.Model,
				PromptVersion: c.PromptVersion, Usage: ev.Done.Usage,
			}})
			return
		default:
			delta := ev.Delta
			if restorer != nil {
				delta = restorer.Push(delta)
			}
			if delta == "" {
				continue
			}
			shown.WriteString(delta)
			if !send(Event{Delta: delta}) {
				g.record(ctx, c, pr, p.Name(), req.Model, estimate(req, shown.String()), started, statusCancelled, ctx.Err())
				return
			}
		}
	}
	if ctx.Err() != nil {
		g.record(ctx, c, pr, p.Name(), req.Model, estimate(req, shown.String()), started, statusCancelled, ctx.Err())
	}
}

// estimate approximates the tokens of an interrupted call (about four
// characters per token), so that cancelled streams still count.
func estimate(req llm.CompletionRequest, output string) llm.Usage {
	in := utf8.RuneCountInString(req.System)
	for _, m := range req.Messages {
		in += utf8.RuneCountInString(m.Content)
	}
	return llm.Usage{InputTokens: (in + 3) / 4, OutputTokens: (utf8.RuneCountInString(output) + 3) / 4}
}

// Embed returns vectors of texts and the model that produced them.
func (g *Gateway) Embed(ctx context.Context, texts []string) ([][]float32, string, error) {
	c := Call{Feature: llm.FeatureEmbedding}
	pr, err := g.prepare(ctx, &c)
	if err != nil {
		return nil, "", err
	}
	lastErr := error(errUnavailable)
	for _, r := range pr.routes {
		p := g.providers[r.Provider]
		input := texts
		if !p.Local() {
			red := redact.New()
			input = make([]string, len(texts))
			for i, t := range texts {
				input[i] = red.Redact(t)
			}
		}
		started := g.now()
		vectors, model, err := p.Embed(ctx, input)
		if err == nil {
			err = checkVectors(vectors, len(texts))
		}
		if model == "" {
			model = p.DefaultModel(llm.FeatureEmbedding)
		}
		chars := 0
		for _, t := range texts {
			chars += utf8.RuneCountInString(t)
		}
		usage := llm.Usage{InputTokens: (chars + 3) / 4}
		if err != nil {
			g.record(ctx, c, pr, p.Name(), p.DefaultModel(llm.FeatureEmbedding), usage, started, failureStatus(ctx, err), err)
			if ctx.Err() != nil {
				return nil, "", ctx.Err()
			}
			lastErr = publicError(ctx, err)
			continue
		}
		g.record(ctx, c, pr, p.Name(), model, usage, started, statusOK, nil)
		return vectors, model, nil
	}
	return nil, "", lastErr
}

// checkVectors makes sure a provider returned one non-empty vector per text,
// all of one size: callers index the result and store it next to vectors of
// the same model (Н-26).
func checkVectors(vectors [][]float32, n int) error {
	if len(vectors) != n {
		return fmt.Errorf("%w: %d vectors for %d texts", llm.ErrUnavailable, len(vectors), n)
	}
	for i, v := range vectors {
		if len(v) == 0 || len(v) != len(vectors[0]) {
			return fmt.Errorf("%w: vector %d has %d dimensions", llm.ErrUnavailable, i, len(v))
		}
	}
	return nil
}

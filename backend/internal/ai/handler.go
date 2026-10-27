package ai

import (
	"context"
	"fmt"
	"net/http"
	"time"

	openapi_types "github.com/oapi-codegen/runtime/types"

	"github.com/yelnurq/startify/backend/internal/ai/aidb"
	"github.com/yelnurq/startify/backend/internal/ai/llm"
	"github.com/yelnurq/startify/backend/internal/ai/prompts"
	"github.com/yelnurq/startify/backend/internal/apigen"
	"github.com/yelnurq/startify/backend/internal/platform/apperr"
	"github.com/yelnurq/startify/backend/internal/platform/authz"
	"github.com/yelnurq/startify/backend/internal/platform/httpx"
	"github.com/yelnurq/startify/backend/internal/platform/tenancy"
	"github.com/yelnurq/startify/backend/internal/platform/validate"
)

// TenantNamer names the current university in a language.
type TenantNamer func(ctx context.Context, locale string) string

// Handler serves the AI operations of the API.
type Handler struct {
	g    *Gateway
	name TenantNamer
}

// NewHandler returns the AI handler.
func NewHandler(g *Gateway, name TenantNamer) *Handler { return &Handler{g: g, name: name} }

// GetAiStatus implements apigen.StrictServerInterface.
func (h *Handler) GetAiStatus(ctx context.Context, _ apigen.GetAiStatusRequestObject) (apigen.GetAiStatusResponseObject, error) {
	s, err := h.g.Status(ctx)
	if err != nil {
		return nil, err
	}
	return apigen.GetAiStatus200JSONResponse{
		External: s.External, DailyLimit: s.Limit, UsedToday: int(s.Used), ResetsAt: s.ResetsAt,
		MentorMode: apigen.MentorMode(s.MentorMode),
	}, nil
}

func routesAPI(r Routes) apigen.AiRoutes {
	out := apigen.AiRoutes{}
	for f, routes := range r {
		list := make([]apigen.AiRoute, len(routes))
		for i, route := range routes {
			list[i] = apigen.AiRoute{Provider: route.Provider}
			if route.Model != "" {
				m := route.Model
				list[i].Model = &m
			}
		}
		out[string(f)] = list
	}
	return out
}

func (h *Handler) settingsAPI(s Settings) apigen.AiSettings {
	providers := h.g.Providers()
	list := make([]apigen.AiProvider, len(providers))
	for i, p := range providers {
		models := map[string]string{}
		for f, m := range p.Models {
			models[string(f)] = m
		}
		list[i] = apigen.AiProvider{Name: p.Name, Local: p.Local, Models: models}
	}
	return apigen.AiSettings{
		Routes: routesAPI(s.Routes), DefaultRoutes: routesAPI(h.g.DefaultRoutes()), Providers: list,
		LocalOnly: s.LocalOnly, DailyUserTokens: s.DailyUserTokens, DailyTenantTokens: s.DailyTenantTokens,
		MentorMode: apigen.MentorMode(s.MentorMode),
	}
}

// GetAiSettings implements apigen.StrictServerInterface.
func (h *Handler) GetAiSettings(ctx context.Context, _ apigen.GetAiSettingsRequestObject) (apigen.GetAiSettingsResponseObject, error) {
	if _, err := authz.RequireRole(ctx, authz.RoleTenantAdmin); err != nil {
		return nil, err
	}
	s, err := h.g.Settings(ctx)
	if err != nil {
		return nil, err
	}
	tag := s.ETag()
	return apigen.GetAiSettings200JSONResponse{Body: h.settingsAPI(s), Headers: apigen.GetAiSettings200ResponseHeaders{ETag: &tag}}, nil
}

// UpdateAiSettings implements apigen.StrictServerInterface.
func (h *Handler) UpdateAiSettings(ctx context.Context, req apigen.UpdateAiSettingsRequestObject) (apigen.UpdateAiSettingsResponseObject, error) {
	if _, err := authz.RequireRole(ctx, authz.RoleTenantAdmin); err != nil {
		return nil, err
	}
	body := req.Body
	var v validate.Errors
	if body.DailyUserTokens < 0 || body.DailyUserTokens > 100_000_000 {
		v.Add("daily_user_tokens", validate.CodeInvalid, nil)
	}
	if body.DailyTenantTokens < 0 || body.DailyTenantTokens > 100_000_000_000 {
		v.Add("daily_tenant_tokens", validate.CodeInvalid, nil)
	}
	routes := Routes{}
	for name, list := range body.Routes {
		f := llm.Feature(name)
		if !f.Valid() {
			v.Add("routes", validate.CodeInvalid, nil)
			continue
		}
		if len(list) > 5 {
			v.Add("routes."+name, validate.CodeMaxItems, map[string]any{"count": 5})
			continue
		}
		for _, r := range list {
			route := Route{Provider: r.Provider}
			if r.Model != nil {
				route.Model = validate.Clean(*r.Model, false)
			}
			routes[f] = append(routes[f], route)
		}
	}
	if err := v.Err(); err != nil {
		return nil, err
	}
	saved, err := h.g.SaveSettings(ctx, Settings{
		Routes: routes, LocalOnly: body.LocalOnly, DailyUserTokens: body.DailyUserTokens,
		DailyTenantTokens: body.DailyTenantTokens, MentorMode: string(body.MentorMode),
	}, req.Params.IfMatch)
	if err != nil {
		return nil, err
	}
	tag := saved.ETag()
	return apigen.UpdateAiSettings200JSONResponse{Body: h.settingsAPI(saved), Headers: apigen.UpdateAiSettings200ResponseHeaders{ETag: &tag}}, nil
}

// CheckAiConnection implements apigen.StrictServerInterface.
func (h *Handler) CheckAiConnection(ctx context.Context, req apigen.CheckAiConnectionRequestObject) (apigen.CheckAiConnectionResponseObject, error) {
	if _, err := authz.RequireRole(ctx, authz.RoleTenantAdmin); err != nil {
		return nil, err
	}
	t, err := tenancy.Require(ctx)
	if err != nil {
		return nil, err
	}
	feature := llm.Feature(req.Body.Feature)
	if !feature.Valid() {
		return nil, apperr.Validation(apperr.FieldError{Field: "feature", Code: validate.CodeInvalid})
	}
	locale := t.Locale
	if req.Body.Locale != nil && req.Body.Locale.Valid() {
		locale = string(*req.Body.Locale)
	}
	if feature == llm.FeatureEmbedding {
		started := time.Now()
		vectors, model, err := h.g.Embed(ctx, []string{"Qadam: проверка подключения"})
		if err != nil {
			return nil, err
		}
		return embeddingCheck{model: model, dims: len(vectors[0]), took: time.Since(started)}, nil
	}
	system, version, err := prompts.Render("connection_check", prompts.ConnectionCheck{Tenant: h.name(ctx, locale), Locale: locale})
	if err != nil {
		return nil, apperr.Internal(err)
	}
	events, err := h.g.Stream(ctx, Call{
		Feature: feature, PromptVersion: version, System: system, Locale: locale, MaxTokens: 600, Effort: llm.EffortLow,
		Messages: []llm.Message{{Role: llm.RoleUser, Content: "Check the connection."}},
	})
	if err != nil {
		return nil, err
	}
	r, ok := httpx.RequestFrom(ctx)
	if !ok {
		r = &http.Request{}
	}
	return streamResponse{request: r, events: events}, nil
}

// checkResult is the final event of a connection check (AiCheckResult).
type checkResult struct {
	Provider      string `json:"provider"`
	Model         string `json:"model"`
	PromptVersion string `json:"prompt_version"`
	InputTokens   int    `json:"input_tokens"`
	OutputTokens  int    `json:"output_tokens"`
}

// streamResponse relays model events as server-sent events.
type streamResponse struct {
	request *http.Request
	events  <-chan Event
}

func (s streamResponse) VisitCheckAiConnectionResponse(w http.ResponseWriter) error {
	sse := httpx.NewSSE(w)
	for ev := range s.events {
		// A failed write means the client went away; the request context is
		// then cancelled, which stops the model.
		if s.send(sse, ev) != nil {
			break
		}
	}
	return nil //nolint:nilerr // the stream is already answered; there is nobody to report to
}

func (s streamResponse) send(sse *httpx.SSE, ev Event) error {
	switch {
	case ev.Err != nil:
		return sse.Event("error", httpx.ProblemFrom(s.request, ev.Err))
	case ev.Result != nil:
		return sse.Event("done", checkResult{
			Provider: ev.Result.Provider, Model: ev.Result.Model, PromptVersion: ev.Result.PromptVersion,
			InputTokens: ev.Result.Usage.InputTokens, OutputTokens: ev.Result.Usage.OutputTokens,
		})
	default:
		return sse.Event("delta", map[string]string{"text": ev.Delta})
	}
}

type embeddingCheck struct {
	model string
	dims  int
	took  time.Duration
}

func (e embeddingCheck) VisitCheckAiConnectionResponse(w http.ResponseWriter) error {
	sse := httpx.NewSSE(w)
	_ = sse.Event("delta", map[string]string{"text": fmt.Sprintf("%s · %d", e.model, e.dims)})
	_ = sse.Event("done", checkResult{Model: e.model, PromptVersion: ""})
	return nil
}

// GetAiUsage implements apigen.StrictServerInterface.
func (h *Handler) GetAiUsage(ctx context.Context, req apigen.GetAiUsageRequestObject) (apigen.GetAiUsageResponseObject, error) {
	if _, err := authz.RequireRole(ctx, authz.RoleTenantAdmin, authz.RoleLeadership); err != nil {
		return nil, err
	}
	days := 30
	if req.Params.Days != nil {
		days = min(max(*req.Params.Days, 1), 365)
	}
	report, err := h.g.Usage(ctx, days)
	if err != nil {
		return nil, err
	}
	return apigen.GetAiUsage200JSONResponse(report), nil
}

func usd(micros int64) float64 { return float64(micros) / 1e6 }

// Usage reports AI calls of the current university over the last days.
func (g *Gateway) Usage(ctx context.Context, days int) (apigen.AiUsageReport, error) {
	if _, err := tenancy.Require(ctx); err != nil {
		return apigen.AiUsageReport{}, err
	}
	since := g.dayStart().AddDate(0, 0, -(days - 1))
	report := apigen.AiUsageReport{
		Since: since, ByFeature: []apigen.AiUsageByFeature{}, ByDay: []apigen.AiUsageByDay{}, ByModel: []apigen.AiUsageByModel{},
	}
	err := g.db.ReadTx(ctx, func(ctx context.Context) error {
		q := aidb.New(g.db.Conn(ctx))
		features, err := q.UsageByFeature(ctx, since)
		if err != nil {
			return err
		}
		var cost int64
		for _, f := range features {
			report.Totals.Requests += int(f.Requests)
			report.Totals.InputTokens += f.InputTokens
			report.Totals.OutputTokens += f.OutputTokens
			cost += f.CostMicros
			report.ByFeature = append(report.ByFeature, apigen.AiUsageByFeature{
				Feature: apigen.AiFeature(f.Feature), Requests: int(f.Requests), InputTokens: f.InputTokens,
				OutputTokens: f.OutputTokens, CostUsd: usd(f.CostMicros), Failures: int(f.Failures),
			})
		}
		report.Totals.CostUsd = usd(cost)
		byDay, err := q.UsageByDay(ctx, aidb.UsageByDayParams{Zone: g.zone.String(), Since: since})
		if err != nil {
			return err
		}
		for _, d := range byDay {
			report.ByDay = append(report.ByDay, apigen.AiUsageByDay{
				Day: openapi_types.Date{Time: d.Day}, Requests: int(d.Requests), Tokens: d.Tokens, CostUsd: usd(d.CostMicros),
			})
		}
		byModel, err := q.UsageByModel(ctx, since)
		if err != nil {
			return err
		}
		for _, m := range byModel {
			report.ByModel = append(report.ByModel, apigen.AiUsageByModel{
				Provider: m.Provider, Model: m.Model, Requests: int(m.Requests), Tokens: m.Tokens, CostUsd: usd(m.CostMicros),
			})
		}
		return nil
	})
	if err != nil {
		return report, fmt.Errorf("ai: usage: %w", err)
	}
	return report, nil
}

package ai_test

import (
	"context"
	"errors"
	"io"
	"log/slog"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/google/uuid"

	"github.com/yelnurq/startify/backend/internal/ai"
	"github.com/yelnurq/startify/backend/internal/ai/llm"
	"github.com/yelnurq/startify/backend/internal/ai/mock"
	"github.com/yelnurq/startify/backend/internal/platform/apperr"
	"github.com/yelnurq/startify/backend/internal/platform/authz"
	"github.com/yelnurq/startify/backend/internal/platform/config"
	"github.com/yelnurq/startify/backend/internal/platform/db"
	"github.com/yelnurq/startify/backend/internal/platform/tenancy"
	"github.com/yelnurq/startify/backend/internal/platform/testpg"
	"github.com/yelnurq/startify/backend/internal/seed"
)

var pg *testpg.Server

func TestMain(m *testing.M) { testpg.Main(m, &pg) }

// scripted answers with prepared replies and records what it was asked.
type scripted struct {
	name    string
	local   bool
	mu      sync.Mutex
	replies []func(req llm.CompletionRequest) (*llm.CompletionResponse, error)
	asked   []llm.CompletionRequest
}

func (s *scripted) Name() string                    { return s.name }
func (s *scripted) Local() bool                     { return s.local }
func (s *scripted) DefaultModel(llm.Feature) string { return s.name + "-model" }
func (s *scripted) Embed(context.Context, []string) ([][]float32, string, error) {
	return nil, "", llm.ErrNotSupported
}

func (s *scripted) Complete(_ context.Context, req llm.CompletionRequest) (*llm.CompletionResponse, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.asked = append(s.asked, req)
	if len(s.replies) == 0 {
		return nil, llm.ErrUnavailable
	}
	next := s.replies[0]
	s.replies = s.replies[1:]
	return next(req)
}

func (s *scripted) Stream(ctx context.Context, req llm.CompletionRequest) (<-chan llm.StreamEvent, error) {
	res, err := s.Complete(ctx, req)
	if err != nil {
		return nil, err
	}
	ch := make(chan llm.StreamEvent, 2)
	ch <- llm.StreamEvent{Delta: res.Text}
	ch <- llm.StreamEvent{Done: res}
	close(ch)
	return ch, nil
}

func answer(text string, tokens int) func(llm.CompletionRequest) (*llm.CompletionResponse, error) {
	return func(req llm.CompletionRequest) (*llm.CompletionResponse, error) {
		return &llm.CompletionResponse{Text: text, Model: req.Model, Usage: llm.Usage{InputTokens: tokens, OutputTokens: tokens}}, nil
	}
}

func fail(err error) func(llm.CompletionRequest) (*llm.CompletionResponse, error) {
	return func(llm.CompletionRequest) (*llm.CompletionResponse, error) { return nil, err }
}

type env struct {
	db      *testpg.Database
	ctx     context.Context //nolint:containedctx // test fixture carrying the tenant scope
	user    uuid.UUID
	gateway *ai.Gateway
}

func setup(t *testing.T, routes string, providers ...llm.Provider) env {
	t.Helper()
	database := pg.NewDB(t)
	ctx := context.Background()
	tenant, err := seed.Tenant(ctx, database.App, seed.Sandbox)
	if err != nil {
		t.Fatal(err)
	}
	user, err := seed.Account(ctx, database.App, tenant.ID, seed.Member{
		Email: "ai-user@qadam.test", FullName: "Ай Пайдаланушы", Locale: "kk", Password: "Steppe-Falcon-Rises-42",
		Roles: []string{"student"}, Slug: "ai-user",
	})
	if err != nil {
		t.Fatal(err)
	}
	g, err := ai.NewGateway(database.App, append([]llm.Provider{&mock.Provider{}}, providers...),
		config.AI{Default: "mock", Routes: routes, Timezone: "Asia/Almaty"}, slog.New(slog.NewTextHandler(io.Discard, nil)))
	if err != nil {
		t.Fatal(err)
	}
	ctx = tenancy.With(ctx, tenancy.Tenant{ID: tenant.ID, Slug: "sandbox", Locale: "ru"})
	ctx = db.WithTenant(ctx, tenant.ID)
	ctx = authz.WithPrincipal(ctx, authz.Principal{UserID: user, TenantID: tenant.ID, Roles: []authz.Role{authz.RoleStudent}})
	return env{db: database, ctx: ctx, user: user, gateway: g}
}

type usageRow struct {
	provider, status, version string
	tokens                    int
}

func usage(t *testing.T, e env) []usageRow {
	t.Helper()
	var out []usageRow
	err := e.db.App.ReadTx(e.ctx, func(ctx context.Context) error {
		rows, err := e.db.App.Conn(ctx).Query(ctx,
			"SELECT provider, status, coalesce(prompt_version, ''), input_tokens + output_tokens FROM ai_usage ORDER BY id")
		if err != nil {
			return err
		}
		defer rows.Close()
		for rows.Next() {
			var r usageRow
			if err := rows.Scan(&r.provider, &r.status, &r.version, &r.tokens); err != nil {
				return err
			}
			out = append(out, r)
		}
		return rows.Err()
	})
	if err != nil {
		t.Fatal(err)
	}
	return out
}

func TestFallbackAndAccounting(t *testing.T) {
	cloud := &scripted{name: "anthropic", replies: []func(llm.CompletionRequest) (*llm.CompletionResponse, error){fail(llm.ErrUnavailable)}}
	e := setup(t, "mentor=anthropic,mock", cloud)
	res, err := e.gateway.Complete(e.ctx, ai.Call{
		Feature: llm.FeatureMentor, PromptVersion: "mentor@1", System: "Ты ментор.", Locale: "ru",
		Messages: []llm.Message{{Role: llm.RoleUser, Content: "С чего начать?"}},
	})
	if err != nil {
		t.Fatal(err)
	}
	if res.Provider != "mock" || res.PromptVersion != "mentor@1" || res.Text == "" {
		t.Fatalf("result %+v", res)
	}
	rows := usage(t, e)
	if len(rows) != 2 || rows[0].provider != "anthropic" || rows[0].status != "error" || rows[1].status != "ok" || rows[1].version != "mentor@1" {
		t.Fatalf("usage %+v", rows)
	}

	// A refusal also falls through to the next route; when all fail, the user sees why.
	refusing := &scripted{name: "anthropic", replies: []func(llm.CompletionRequest) (*llm.CompletionResponse, error){fail(llm.ErrRefused)}}
	e = setup(t, "evaluation=anthropic", refusing)
	_, err = e.gateway.Complete(e.ctx, ai.Call{Feature: llm.FeatureEvaluation, Messages: []llm.Message{{Role: llm.RoleUser, Content: "x"}}})
	if !apperr.Is(err, "ai.refused") {
		t.Fatalf("refusal: %v", err)
	}
}

var verdict = &llm.Schema{Name: "gateway-test-verdict", JSON: map[string]any{
	"type": "object", "required": []any{"score"}, "additionalProperties": false,
	"properties": map[string]any{"score": map[string]any{"type": "integer", "minimum": 0, "maximum": 10}},
}}

func TestStructuredOutputGetsOneCorrection(t *testing.T) {
	model := &scripted{name: "openai", local: true, replies: []func(llm.CompletionRequest) (*llm.CompletionResponse, error){
		answer(`{"score": 42}`, 10), answer("```json\n{\"score\": 7}\n```", 10),
	}}
	e := setup(t, "evaluation=openai", model)
	res, err := e.gateway.Complete(e.ctx, ai.Call{
		Feature: llm.FeatureEvaluation, Schema: verdict,
		Messages: []llm.Message{{Role: llm.RoleUser, Content: "Оцени"}},
	})
	if err != nil {
		t.Fatal(err)
	}
	if res.JSON.(map[string]any)["score"] == nil || res.Usage.InputTokens != 20 {
		t.Fatalf("result %+v", res)
	}
	retry := model.asked[1].Messages
	if !strings.Contains(retry[len(retry)-1].Content, "/score") || retry[len(retry)-2].Content != `{"score": 42}` {
		t.Fatalf("the correction must show the error: %+v", retry)
	}

	model.replies = []func(llm.CompletionRequest) (*llm.CompletionResponse, error){answer("nope", 5), answer("still nope", 5)}
	_, err = e.gateway.Complete(e.ctx, ai.Call{
		Feature: llm.FeatureEvaluation, Schema: verdict,
		Messages: []llm.Message{{Role: llm.RoleUser, Content: "Оцени"}},
	})
	if !apperr.Is(err, "ai.invalid_output") {
		t.Fatalf("expected invalid output: %v", err)
	}
	if rows := usage(t, e); rows[len(rows)-1].status != "invalid_output" {
		t.Fatalf("usage %+v", rows)
	}
}

func TestPersonalDataStaysInside(t *testing.T) {
	cloud := &scripted{name: "anthropic", replies: []func(llm.CompletionRequest) (*llm.CompletionResponse, error){
		answer("Напишите [EMAIL_1] или позвоните [PHONE_1].", 5),
	}}
	e := setup(t, "mentor=anthropic", cloud)
	res, err := e.gateway.Complete(e.ctx, ai.Call{
		Feature: llm.FeatureMentor, System: "Контакт команды: team@kazutb.edu.kz",
		Messages: []llm.Message{{Role: llm.RoleUser, Content: "Мой телефон +7 701 234 56 78, ИИН 950315400125"}},
	})
	if err != nil {
		t.Fatal(err)
	}
	sent := cloud.asked[0]
	for _, secret := range []string{"team@kazutb", "701 234", "950315400125"} {
		if strings.Contains(sent.System+sent.Messages[0].Content, secret) {
			t.Fatalf("%q reached the external provider", secret)
		}
	}
	if !strings.Contains(sent.System, "Keep placeholders exactly") {
		t.Fatal("the model must be told about placeholders")
	}
	if res.Text != "Напишите team@kazutb.edu.kz или позвоните +7 701 234 56 78." {
		t.Fatalf("not restored: %q", res.Text)
	}

	// A provider inside the university gets the text as is.
	local := &scripted{name: "openai", local: true, replies: []func(llm.CompletionRequest) (*llm.CompletionResponse, error){answer("ok", 1)}}
	e = setup(t, "mentor=openai", local)
	if _, err := e.gateway.Complete(e.ctx, ai.Call{
		Feature: llm.FeatureMentor, System: "team@kazutb.edu.kz",
		Messages: []llm.Message{{Role: llm.RoleUser, Content: "x"}},
	}); err != nil {
		t.Fatal(err)
	}
	if local.asked[0].System != "team@kazutb.edu.kz" {
		t.Fatal("local providers must get the original text")
	}
}

func TestLocalOnlyAndSettings(t *testing.T) {
	cloud := &scripted{name: "anthropic"}
	e := setup(t, "mentor=anthropic,mock", cloud)
	s, err := e.gateway.Settings(e.ctx)
	if err != nil || s.DailyUserTokens != ai.DefaultDailyUserTokens || !e.gateway.External(s, llm.FeatureMentor) {
		t.Fatalf("defaults %+v %v", s, err)
	}
	s.LocalOnly = true
	s.Routes = ai.Routes{llm.FeatureMentor: {{Provider: "anthropic", Model: "claude-sonnet-5-5"}}}
	if _, err := e.gateway.SaveSettings(e.ctx, s, nil); !apperr.Is(err, "validation.failed") {
		t.Fatalf("external route with local_only: %v", err)
	}
	s.Routes = ai.Routes{}
	saved, err := e.gateway.SaveSettings(e.ctx, s, nil)
	if err != nil {
		t.Fatal(err)
	}
	if e.gateway.External(saved, llm.FeatureMentor) {
		t.Fatal("local_only must hide external routes")
	}
	res, err := e.gateway.Complete(e.ctx, ai.Call{Feature: llm.FeatureMentor, Messages: []llm.Message{{Role: llm.RoleUser, Content: "x"}}})
	if err != nil || res.Provider != "mock" || len(cloud.asked) != 0 {
		t.Fatalf("local only: %+v %v %d", res, err, len(cloud.asked))
	}
	stale := `"stale"`
	if _, err := e.gateway.SaveSettings(e.ctx, saved, &stale); !apperr.Is(err, "precondition.failed") {
		t.Fatalf("stale If-Match: %v", err)
	}
	tag := saved.ETag()
	if _, err := e.gateway.SaveSettings(e.ctx, saved, &tag); err != nil {
		t.Fatalf("fresh If-Match: %v", err)
	}
}

func TestDailyLimit(t *testing.T) {
	model := &scripted{name: "openai", local: true, replies: []func(llm.CompletionRequest) (*llm.CompletionResponse, error){answer("ok", 80)}}
	e := setup(t, "summary=openai", model)
	almaty, _ := time.LoadLocation("Asia/Almaty")
	now := time.Date(2026, 10, 22, 21, 30, 0, 0, almaty)
	e.gateway.SetClock(func() time.Time { return now })
	s, _ := e.gateway.Settings(e.ctx)
	s.DailyUserTokens = 100
	if _, err := e.gateway.SaveSettings(e.ctx, s, nil); err != nil {
		t.Fatal(err)
	}
	call := ai.Call{Feature: llm.FeatureSummary, Messages: []llm.Message{{Role: llm.RoleUser, Content: "x"}}}
	if _, err := e.gateway.Complete(e.ctx, call); err != nil {
		t.Fatal(err)
	}
	_, err := e.gateway.Complete(e.ctx, call)
	ae, ok := apperr.As(err)
	if !ok || ae.Code != "ai.limit_reached" || ae.Params["resets_at"] != "00:00" || ae.Params["scope"] != "user" ||
		ae.RetryAfter != 150*time.Minute {
		t.Fatalf("limit: %+v", err)
	}
	status, err := e.gateway.Status(e.ctx)
	if err != nil || status.Used != 160 || status.Limit != 100 || !status.ResetsAt.Equal(time.Date(2026, 10, 23, 0, 0, 0, 0, almaty)) {
		t.Fatalf("status %+v %v", status, err)
	}
}

func TestStreamCancellationIsRecorded(t *testing.T) {
	database := pg.NewDB(t)
	ctx := context.Background()
	tenant, err := seed.Tenant(ctx, database.App, seed.Sandbox)
	if err != nil {
		t.Fatal(err)
	}
	g, err := ai.NewGateway(database.App, []llm.Provider{&mock.Provider{ChunkDelay: 30 * time.Millisecond}},
		config.AI{Default: "mock", Timezone: "Asia/Almaty"}, slog.New(slog.NewTextHandler(io.Discard, nil)))
	if err != nil {
		t.Fatal(err)
	}
	ctx = db.WithTenant(tenancy.With(ctx, tenancy.Tenant{ID: tenant.ID}), tenant.ID)
	streamCtx, cancel := context.WithCancel(ctx)
	events, err := g.Stream(streamCtx, ai.Call{
		Feature: llm.FeatureMentor, Locale: "en",
		Messages: []llm.Message{{Role: llm.RoleUser, Content: "Help me"}},
	})
	if err != nil {
		t.Fatal(err)
	}
	first := <-events
	if first.Delta == "" {
		t.Fatalf("first event %+v", first)
	}
	cancel()
	for range events {
	}
	e := env{db: database, ctx: ctx}
	deadline := time.Now().Add(2 * time.Second)
	for {
		rows := usage(t, e)
		if len(rows) == 1 && rows[0].status == "cancelled" && rows[0].tokens > 0 {
			break
		}
		if time.Now().After(deadline) {
			t.Fatalf("usage %+v", rows)
		}
		time.Sleep(20 * time.Millisecond)
	}

	vectors, model, err := g.Embed(ctx, []string{"Положение о конкурсе"})
	if err != nil || model != mock.EmbeddingModel || len(vectors) != 1 {
		t.Fatalf("embed %v %s", err, model)
	}
	if _, err := g.Stream(ctx, ai.Call{Feature: llm.FeatureMentor, Schema: verdict}); err == nil {
		t.Fatal("structured output must not stream")
	}
}

func TestParseRoutes(t *testing.T) {
	r, err := ai.ParseRoutes("mentor=anthropic:claude-sonnet-5-5, openai ; embedding=openai")
	if err != nil || len(r[llm.FeatureMentor]) != 2 || r[llm.FeatureMentor][0].Model != "claude-sonnet-5-5" || r[llm.FeatureEmbedding][0].Provider != "openai" {
		t.Fatalf("%+v %v", r, err)
	}
	for _, bad := range []string{"chat=mock", "mentor=", "mentor"} {
		if _, err := ai.ParseRoutes(bad); err == nil {
			t.Errorf("%q accepted", bad)
		}
	}
	if _, err := ai.NewGateway(nil, []llm.Provider{&mock.Provider{}}, config.AI{Default: "mock", Routes: "mentor=anthropic", Timezone: "UTC"},
		slog.New(slog.NewTextHandler(io.Discard, nil))); err == nil || !errors.Is(err, err) {
		t.Fatal("a route to an unconfigured provider must fail at start-up")
	}
}

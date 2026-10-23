package ai_test

import (
	"context"
	"errors"
	"fmt"
	"io"
	"log/slog"
	"strings"
	"sync"
	"sync/atomic"
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
	// noEmbeddings makes it a provider like Anthropic; embed answers Embed.
	noEmbeddings bool
	embed        func(texts []string) ([][]float32, string, error)
	// streamErr fails Stream before the first words, like a 401 or a 529.
	streamErr error
}

func (s *scripted) Name() string { return s.name }
func (s *scripted) Local() bool  { return s.local }
func (s *scripted) DefaultModel(f llm.Feature) string {
	if f == llm.FeatureEmbedding && s.noEmbeddings {
		return ""
	}
	return s.name + "-model"
}

func (s *scripted) Embed(_ context.Context, texts []string) ([][]float32, string, error) {
	if s.embed != nil {
		return s.embed(texts)
	}
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
	if s.streamErr != nil {
		s.mu.Lock()
		s.asked = append(s.asked, req)
		s.mu.Unlock()
		return nil, s.streamErr
	}
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

// A stream that fails before the first words goes to the next provider of
// the route, as Complete does (С-15).
func TestStreamFallsBackToTheNextProvider(t *testing.T) {
	overloaded := fmt.Errorf("%w: HTTP 529 overloaded", llm.ErrUnavailable)
	cloud := &scripted{name: "anthropic", streamErr: overloaded}
	e := setup(t, "mentor=anthropic,mock", cloud)
	events, err := e.gateway.Stream(e.ctx, ai.Call{
		Feature: llm.FeatureMentor, Locale: "ru", Messages: []llm.Message{{Role: llm.RoleUser, Content: "С чего начать?"}},
	})
	if err != nil {
		t.Fatal(err)
	}
	var result *ai.Result
	for ev := range events {
		if ev.Err != nil {
			t.Fatal(ev.Err)
		}
		if ev.Result != nil {
			result = ev.Result
		}
	}
	if result == nil || result.Provider != "mock" || len(cloud.asked) != 1 {
		t.Fatalf("result %+v after %d cloud requests", result, len(cloud.asked))
	}
	rows := usage(t, e)
	if len(rows) != 2 || rows[0].provider != "anthropic" || rows[0].status != "error" || rows[1].provider != "mock" || rows[1].status != "ok" {
		t.Fatalf("usage %+v", rows)
	}

	// Without another provider the user is told AI is unavailable.
	e = setup(t, "mentor=anthropic", &scripted{name: "anthropic", streamErr: overloaded})
	_, err = e.gateway.Stream(e.ctx, ai.Call{Feature: llm.FeatureMentor, Messages: []llm.Message{{Role: llm.RoleUser, Content: "x"}}})
	if !apperr.Is(err, "ai.unavailable") {
		t.Fatalf("no fallback: %v", err)
	}
}

// Callers get the validated JSON value, not the fence a local model wrapped
// it in (С-16).
func TestStructuredOutputIsTheBareValue(t *testing.T) {
	model := &scripted{name: "openai", local: true, replies: []func(llm.CompletionRequest) (*llm.CompletionResponse, error){
		answer("Вот оценка:\n```json\n{\"score\": 7}\n```", 10),
	}}
	e := setup(t, "evaluation=openai", model)
	res, err := e.gateway.Complete(e.ctx, ai.Call{Feature: llm.FeatureEvaluation, Schema: verdict, Messages: []llm.Message{{Role: llm.RoleUser, Content: "Оцени"}}})
	if err != nil || res.Text != `{"score": 7}` || len(model.asked) != 1 {
		t.Fatalf("%+v %v", res, err)
	}
}

func truncated(text string) func(llm.CompletionRequest) (*llm.CompletionResponse, error) {
	return func(req llm.CompletionRequest) (*llm.CompletionResponse, error) {
		return &llm.CompletionResponse{Text: text, Model: req.Model, StopReason: llm.StopMaxTokens, Usage: llm.Usage{InputTokens: 5, OutputTokens: 5}}, nil
	}
}

// A structured answer cut at the token limit is asked again with room to
// finish; cut twice, it is a clear failure (Н-24).
func TestTruncatedStructuredOutputGetsMoreRoom(t *testing.T) {
	model := &scripted{name: "openai", local: true, replies: []func(llm.CompletionRequest) (*llm.CompletionResponse, error){
		truncated(`{"sco`), answer(`{"score": 3}`, 5),
	}}
	e := setup(t, "evaluation=openai", model)
	call := ai.Call{Feature: llm.FeatureEvaluation, Schema: verdict, MaxTokens: 1000, Messages: []llm.Message{{Role: llm.RoleUser, Content: "Оцени"}}}
	res, err := e.gateway.Complete(e.ctx, call)
	if err != nil || res.Text != `{"score": 3}` {
		t.Fatalf("%+v %v", res, err)
	}
	if retry := model.asked[1]; retry.MaxTokens != 2000 || len(retry.Messages) != 1 {
		t.Fatalf("the retry must have a larger limit and no correction: %+v", retry)
	}

	model.replies = []func(llm.CompletionRequest) (*llm.CompletionResponse, error){truncated(`{"sc`), truncated(`{"score`)}
	if _, err := e.gateway.Complete(e.ctx, call); !apperr.Is(err, "ai.invalid_output") || len(model.asked) != 4 {
		t.Fatalf("cut twice: %v after %d requests", err, len(model.asked))
	}
	var code string
	err = e.db.App.ReadTx(e.ctx, func(ctx context.Context) error {
		return e.db.App.Conn(ctx).QueryRow(ctx, "SELECT coalesce(error_code, '') FROM ai_usage ORDER BY id DESC LIMIT 1").Scan(&code)
	})
	if err != nil || code != "truncated" {
		t.Fatalf("usage error code %q %v", code, err)
	}
}

// slow holds every answer until released, like a model that thinks.
type slow struct {
	entered atomic.Int32
	gate    chan struct{}
}

func (s *slow) Name() string                    { return "openai" }
func (s *slow) Local() bool                     { return true }
func (s *slow) DefaultModel(llm.Feature) string { return "slow-model" }
func (s *slow) Embed(context.Context, []string) ([][]float32, string, error) {
	return nil, "", llm.ErrNotSupported
}

func (s *slow) Stream(context.Context, llm.CompletionRequest) (<-chan llm.StreamEvent, error) {
	return nil, llm.ErrNotSupported
}

func (s *slow) Complete(ctx context.Context, req llm.CompletionRequest) (*llm.CompletionResponse, error) {
	s.entered.Add(1)
	select {
	case <-s.gate:
	case <-ctx.Done():
		return nil, ctx.Err()
	}
	return &llm.CompletionResponse{Text: "ok", Model: req.Model, Usage: llm.Usage{InputTokens: 50, OutputTokens: 350}}, nil
}

// Calls in progress count against the limit with what they may use, so a
// burst of parallel requests cannot overshoot it many times over (Н-21).
func TestParallelCallsCannotOvershootTheLimit(t *testing.T) {
	model := &slow{gate: make(chan struct{})}
	e := setup(t, "summary=openai", model)
	s, err := e.gateway.Settings(e.ctx)
	if err != nil {
		t.Fatal(err)
	}
	s.DailyUserTokens = 1000
	if _, err := e.gateway.SaveSettings(e.ctx, s, nil); err != nil {
		t.Fatal(err)
	}
	const calls = 8
	var wg sync.WaitGroup
	var limited atomic.Int32
	failures := make(chan error, calls)
	for range calls {
		wg.Add(1)
		go func() {
			defer wg.Done()
			_, err := e.gateway.Complete(e.ctx, ai.Call{Feature: llm.FeatureSummary, MaxTokens: 400, Messages: []llm.Message{{Role: llm.RoleUser, Content: "x"}}})
			switch {
			case apperr.Is(err, "ai.limit_reached"):
				limited.Add(1)
			case err != nil:
				failures <- err
			}
		}()
	}
	deadline := time.Now().Add(10 * time.Second)
	for model.entered.Load()+limited.Load() < calls {
		if time.Now().After(deadline) {
			t.Fatalf("%d calls at the model, %d limited", model.entered.Load(), limited.Load())
		}
		time.Sleep(10 * time.Millisecond)
	}
	close(model.gate)
	wg.Wait()
	close(failures)
	for err := range failures {
		t.Fatal(err)
	}
	// Each call may use about 400 tokens: three fit under 1000 before the
	// reservations reach the limit.
	if n := model.entered.Load(); n > 3 || limited.Load() < calls-3 {
		t.Fatalf("%d of %d parallel calls reached the model under a limit of 1000 tokens", n, calls)
	}
	var held int
	err = e.db.App.ReadTx(e.ctx, func(ctx context.Context) error {
		return e.db.App.Conn(ctx).QueryRow(ctx, "SELECT count(*) FROM ai_reservations").Scan(&held)
	})
	if err != nil || held != 0 {
		t.Fatalf("finished calls must release their tokens: %d %v", held, err)
	}
}

// Embeddings go only to providers that have them; a provider answering with
// too few or ragged vectors is a failure, never a panic (Н-26).
func TestEmbeddingRoutes(t *testing.T) {
	log := slog.New(slog.NewTextHandler(io.Discard, nil))
	cloud := &scripted{name: "anthropic", noEmbeddings: true}
	_, err := ai.NewGateway(nil, []llm.Provider{&mock.Provider{}, cloud}, config.AI{Default: "mock", Routes: "embedding=anthropic", Timezone: "UTC"}, log)
	if err == nil || !strings.Contains(err.Error(), "no embedding model") {
		t.Fatalf("an embedding route to Anthropic must fail at start-up: %v", err)
	}
	g, err := ai.NewGateway(nil, []llm.Provider{&mock.Provider{}, cloud}, config.AI{Default: "anthropic", Timezone: "UTC"}, log)
	if err != nil || g.DefaultRoutes()[llm.FeatureEmbedding][0].Provider != "mock" {
		t.Fatalf("default embedding route %v", err)
	}

	e := setup(t, "", cloud)
	s, _ := e.gateway.Settings(e.ctx)
	s.Routes = ai.Routes{llm.FeatureEmbedding: {{Provider: "anthropic"}}}
	_, err = e.gateway.SaveSettings(e.ctx, s, nil)
	if ae, ok := apperr.As(err); !ok || len(ae.Fields) != 1 || ae.Fields[0].Field != "routes.embedding" || ae.Fields[0].Code != "not_supported" {
		t.Fatalf("saving an embedding route to Anthropic: %v", err)
	}

	short := &scripted{name: "openai", local: true, embed: func([]string) ([][]float32, string, error) {
		return [][]float32{{1, 0}}, "short-model", nil
	}}
	e = setup(t, "embedding=openai,mock", short)
	vectors, model, err := e.gateway.Embed(e.ctx, []string{"Положение", "о конкурсе"})
	if err != nil || model != mock.EmbeddingModel || len(vectors) != 2 {
		t.Fatalf("too few vectors must fall through to the next route: %v %s %d", err, model, len(vectors))
	}
	ragged := &scripted{name: "openai", local: true, embed: func([]string) ([][]float32, string, error) {
		return [][]float32{{1, 0}, {1}}, "ragged-model", nil
	}}
	e = setup(t, "embedding=openai", ragged)
	if _, _, err := e.gateway.Embed(e.ctx, []string{"a", "b"}); !apperr.Is(err, "ai.unavailable") {
		t.Fatalf("ragged vectors: %v", err)
	}
}

// The indexer's and the questions' embeddings are accounted but do not eat
// the daily limit of students (С-17); an external embedding provider is
// disclosed like any other (Н-27).
func TestEmbeddingsAreAccountedNotLimited(t *testing.T) {
	cloud := &scripted{name: "openai", embed: func(texts []string) ([][]float32, string, error) {
		out := make([][]float32, len(texts))
		for i := range out {
			out[i] = []float32{1, 0}
		}
		return out, "cloud-embedding", nil
	}}
	e := setup(t, "embedding=openai", cloud)
	s, _ := e.gateway.Settings(e.ctx)
	s.DailyUserTokens = 100
	if _, err := e.gateway.SaveSettings(e.ctx, s, nil); err != nil {
		t.Fatal(err)
	}
	if _, _, err := e.gateway.Embed(e.ctx, []string{strings.Repeat("Положение о конкурсе стартап-проектов. ", 30)}); err != nil {
		t.Fatal(err)
	}
	if _, err := e.gateway.Complete(e.ctx, ai.Call{Feature: llm.FeatureSummary, MaxTokens: 10, Messages: []llm.Message{{Role: llm.RoleUser, Content: "x"}}}); err != nil {
		t.Fatalf("embedding tokens must not count against the daily limit: %v", err)
	}
	status, err := e.gateway.Status(e.ctx)
	if err != nil || !status.External || status.Used >= 100 {
		t.Fatalf("status %+v %v", status, err)
	}
	if rows := usage(t, e); len(rows) != 2 || rows[0].tokens < 200 {
		t.Fatalf("embeddings must still be accounted: %+v", rows)
	}
}

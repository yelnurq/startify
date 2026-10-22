package config

import (
	"strings"
	"testing"
	"time"
)

func validEnviron() []string {
	return []string{
		"QADAM_DATABASE_URL=postgres://qadam_app:secret@localhost:5432/qadam",
		"QADAM_AUTH_SECRET_KEY=test-secret-key-with-enough-length-0123456789",
		"QADAM_S3_ENDPOINT=localhost:9000",
		"QADAM_S3_ACCESS_KEY=qadam",
		"QADAM_S3_SECRET_KEY=qadam-secret",
		"PATH=/usr/bin",
	}
}

func TestFromEnvironDefaults(t *testing.T) {
	cfg, err := FromEnviron(validEnviron())
	if err != nil {
		t.Fatalf("FromEnviron: %v", err)
	}
	if cfg.Env != EnvDevelopment {
		t.Errorf("Env = %q, want development", cfg.Env)
	}
	if cfg.HTTP.Addr != ":8080" || cfg.HTTP.ReadHeaderTimeout != 5*time.Second {
		t.Errorf("unexpected HTTP defaults: %+v", cfg.HTTP)
	}
	if cfg.Database.MigrateURL != cfg.Database.URL {
		t.Errorf("MigrateURL should default to URL, got %q", cfg.Database.MigrateURL)
	}
	if cfg.App.DefaultLocale != "kk" {
		t.Errorf("DefaultLocale = %q, want kk", cfg.App.DefaultLocale)
	}
	if len(cfg.HTTP.TrustedPrefixes()) != 0 {
		t.Errorf("expected no trusted proxies, got %v", cfg.HTTP.TrustedPrefixes())
	}
}

func TestFromEnvironOverrides(t *testing.T) {
	env := append(validEnviron(),
		"QADAM_ENV=test",
		"QADAM_HTTP_ADDR=:9999",
		"QADAM_HTTP_REQUEST_TIMEOUT=45s",
		"QADAM_HTTP_TRUSTED_PROXIES=10.0.0.0/8, 172.16.0.1/12",
		"QADAM_DATABASE_MAX_CONNS=50",
		"QADAM_LOG_FORMAT=text",
	)
	cfg, err := FromEnviron(env)
	if err != nil {
		t.Fatalf("FromEnviron: %v", err)
	}
	if cfg.Env != EnvTest || cfg.HTTP.Addr != ":9999" || cfg.HTTP.RequestTimeout != 45*time.Second {
		t.Errorf("overrides not applied: env=%q http=%+v", cfg.Env, cfg.HTTP)
	}
	if cfg.Database.MaxConns != 50 || cfg.Log.Format != "text" {
		t.Errorf("overrides not applied: db=%+v log=%+v", cfg.Database, cfg.Log)
	}
	prefixes := cfg.HTTP.TrustedPrefixes()
	if len(prefixes) != 2 || prefixes[1].String() != "172.16.0.0/12" {
		t.Errorf("TrustedPrefixes = %v, want masked 10.0.0.0/8 and 172.16.0.0/12", prefixes)
	}
}

func TestFromEnvironRejectsUnknownVariable(t *testing.T) {
	_, err := FromEnviron(append(validEnviron(), "QADAM_HTTP_PORT=8080"))
	if err == nil || !strings.Contains(err.Error(), "port") {
		t.Fatalf("expected error naming the unknown key, got %v", err)
	}
}

func TestValidateCollectsAllErrors(t *testing.T) {
	_, err := FromEnviron([]string{
		"QADAM_ENV=staging",
		"QADAM_LOG_LEVEL=verbose",
		"QADAM_HTTP_TRUSTED_PROXIES=not-a-cidr",
	})
	if err == nil {
		t.Fatal("expected validation error")
	}
	for _, want := range []string{"QADAM_ENV", "QADAM_LOG_LEVEL", "QADAM_DATABASE_URL", "not-a-cidr"} {
		if !strings.Contains(err.Error(), want) {
			t.Errorf("error should mention %s, got:\n%v", want, err)
		}
	}
}

func TestProductionRequiresHTTPS(t *testing.T) {
	_, err := FromEnviron(append(validEnviron(), "QADAM_ENV=production", "QADAM_APP_PUBLIC_URL=http://qadam.kz"))
	if err == nil || !strings.Contains(err.Error(), "https") {
		t.Fatalf("expected https error, got %v", err)
	}
}

func TestProductionHardening(t *testing.T) {
	env := append(validEnviron(),
		"QADAM_ENV=production",
		"QADAM_APP_PUBLIC_URL=https://qadam.kz",
		"QADAM_AUTH_SECRET_KEY=dev-only-secret-change-me-please-0123456789",
		"QADAM_FILES_SCAN=optional",
	)
	_, err := FromEnviron(env)
	if err == nil {
		t.Fatal("expected production hardening errors")
	}
	for _, want := range []string{"QADAM_AUTH_SECRET_KEY looks like a development value", "QADAM_SMTP_HOST", "QADAM_FILES_SCAN must be required"} {
		if !strings.Contains(err.Error(), want) {
			t.Errorf("error should mention %q, got: %v", want, err)
		}
	}
}

func TestSecretKeyLength(t *testing.T) {
	_, err := FromEnviron(append(validEnviron(), "QADAM_AUTH_SECRET_KEY=short"))
	if err == nil || !strings.Contains(err.Error(), "at least 32") {
		t.Fatalf("expected secret length error, got %v", err)
	}
}

func TestEnvKey(t *testing.T) {
	cases := map[string]string{
		"QADAM_ENV":                "env",
		"QADAM_DATABASE_URL":       "database.url",
		"QADAM_DATABASE_MAX_CONNS": "database.max_conns",
		"QADAM_APP_PUBLIC_URL":     "app.public_url",
	}
	for in, want := range cases {
		if got := envKey(in); got != want {
			t.Errorf("envKey(%q) = %q, want %q", in, got, want)
		}
	}
}

func TestAIConfig(t *testing.T) {
	cfg, err := FromEnviron(validEnviron())
	if err != nil {
		t.Fatal(err)
	}
	if cfg.AI.Default != "mock" || cfg.AI.Timezone != "Asia/Almaty" || !cfg.AI.AnthropicServerFallbacks || !cfg.AI.OpenAILocal {
		t.Fatalf("defaults %+v", cfg.AI)
	}
	cfg, err = FromEnviron(append(validEnviron(),
		"QADAM_AI_DEFAULT=openai", "QADAM_AI_OPENAI_BASE_URL=http://ollama:11434/v1", "QADAM_AI_OPENAI_MODEL=qwen3:14b",
		"QADAM_AI_OPENAI_EMBEDDING_MODEL=bge-m3", "QADAM_AI_ROUTES=mentor=openai;embedding=openai"))
	if err != nil || cfg.AI.OpenAIModel != "qwen3:14b" || cfg.AI.Routes != "mentor=openai;embedding=openai" {
		t.Fatalf("openai %+v %v", cfg.AI, err)
	}
	for env, want := range map[string]string{
		"QADAM_AI_DEFAULT=anthropic":            "QADAM_AI_ANTHROPIC_API_KEY is required",
		"QADAM_AI_DEFAULT=openai":               "QADAM_AI_OPENAI_BASE_URL and QADAM_AI_OPENAI_MODEL are required",
		"QADAM_AI_DEFAULT=gpt":                  "QADAM_AI_DEFAULT must be",
		"QADAM_AI_OPENAI_BASE_URL=ollama:11434": "must be an http(s) URL",
		"QADAM_AI_TIMEZONE=Mars/Olympus":        "unknown time zone",
		"QADAM_AI_REQUEST_TIMEOUT=1s":           "QADAM_AI_REQUEST_TIMEOUT",
	} {
		if _, err := FromEnviron(append(validEnviron(), env)); err == nil || !strings.Contains(err.Error(), want) {
			t.Errorf("%s: %v", env, err)
		}
	}
}

func TestAIFromEnviron(t *testing.T) {
	ai, err := AIFromEnviron([]string{"QADAM_AI_DEFAULT=mock", "QADAM_DATABASE_URL=ignored", "PATH=/bin"})
	if err != nil || ai.Default != "mock" || ai.Timezone != "Asia/Almaty" {
		t.Fatalf("%+v %v", ai, err)
	}
	if _, err := AIFromEnviron([]string{"QADAM_AI_DEFAULT=anthropic"}); err == nil {
		t.Fatal("anthropic without a key must fail")
	}
}

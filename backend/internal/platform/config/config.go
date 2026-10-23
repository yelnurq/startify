// Package config loads process configuration from QADAM_* environment variables
// and validates it strictly at start-up: a typo in a variable name or an invalid
// value stops the process instead of silently falling back to a default.
package config

import (
	"errors"
	"fmt"
	"net/netip"
	"net/url"
	"os"
	"slices"
	"strings"
	"time"

	"github.com/go-viper/mapstructure/v2"
	"github.com/knadh/koanf/providers/confmap"
	"github.com/knadh/koanf/v2"
)

const envPrefix = "QADAM_"

// Env is the deployment environment.
type Env string

// Supported environments.
const (
	EnvDevelopment Env = "development"
	EnvTest        Env = "test"
	EnvProduction  Env = "production"
)

// Config is the complete process configuration.
type Config struct {
	Env      Env      `koanf:"env"`
	App      App      `koanf:"app"`
	HTTP     HTTP     `koanf:"http"`
	Database Database `koanf:"database"`
	Valkey   Valkey   `koanf:"valkey"`
	Log      Log      `koanf:"log"`
	OTel     OTel     `koanf:"otel"`
	Worker   Worker   `koanf:"worker"`
	Tenant   Tenant   `koanf:"tenant"`
	Auth     Auth     `koanf:"auth"`
	SMTP     SMTP     `koanf:"smtp"`
	S3       S3       `koanf:"s3"`
	Files    Files    `koanf:"files"`
	AI       AI       `koanf:"ai"`
	// Gotenberg renders HTML into PDF (pitch materials, reports).
	Gotenberg Gotenberg `koanf:"gotenberg"`
}

// Gotenberg is the document conversion service; without a URL documents
// cannot be generated and requests for them fail with a clear error.
type Gotenberg struct {
	URL     string        `koanf:"url"`
	Timeout time.Duration `koanf:"timeout"`
}

// AI configures language-model providers. Which provider and model serve a
// feature is set by Routes and can be overridden per university.
type AI struct {
	// Default is the provider of features without a route: mock, anthropic or openai.
	Default string `koanf:"default"`
	// Routes lists providers per feature with fallbacks, e.g.
	// "mentor=anthropic:claude-sonnet-5-5,openai;embedding=openai".
	Routes string `koanf:"routes"`

	AnthropicAPIKey  string `koanf:"anthropic_api_key"`
	AnthropicBaseURL string `koanf:"anthropic_base_url"`
	// AnthropicServerFallbacks lets the API re-run a request declined by a
	// model's safety classifiers on a recommended fallback model.
	AnthropicServerFallbacks bool `koanf:"anthropic_server_fallbacks"`

	// OpenAI-compatible server: OpenAI, vLLM, Ollama, LM Studio.
	OpenAIBaseURL        string `koanf:"openai_base_url"`
	OpenAIAPIKey         string `koanf:"openai_api_key"`
	OpenAIModel          string `koanf:"openai_model"`
	OpenAIEmbeddingModel string `koanf:"openai_embedding_model"`
	// OpenAILocal says whether the server is inside the university network
	// (data sent to it is not redacted and users see no external-provider
	// warning): "true", "false", or "auto" — local only for a loopback or
	// private address or a single-label name such as a Docker service.
	OpenAILocal string `koanf:"openai_local"`
	// EmbeddingMinSimilarity is the cosine similarity below which a passage
	// of the knowledge base is not related to a question.
	EmbeddingMinSimilarity float64 `koanf:"embedding_min_similarity"`

	// Timezone sets when daily limits reset.
	Timezone       string        `koanf:"timezone"`
	RequestTimeout time.Duration `koanf:"request_timeout"`
}

// Tenant configures how host names map to universities.
type Tenant struct {
	// BaseDomain: "kazutb.<base>" resolves to the tenant with slug "kazutb".
	BaseDomain string `koanf:"base_domain"`
	// Default is the tenant served on the bare base domain and unknown hosts
	// (development and single-university installations). Empty disables it.
	Default string `koanf:"default"`
}

// Auth configures sessions.
type Auth struct {
	// SecretKey is the master secret; the keys for access tokens, one-time
	// codes and encryption of stored secrets are derived from it (HKDF).
	SecretKey  string        `koanf:"secret_key"`
	AccessTTL  time.Duration `koanf:"access_ttl"`
	RefreshTTL time.Duration `koanf:"refresh_ttl"`
}

// SMTP configures outgoing e-mail. An empty host disables delivery: letters are logged.
type SMTP struct {
	Host     string `koanf:"host"`
	Port     int    `koanf:"port"`
	Username string `koanf:"username"`
	Password string `koanf:"password"`
	From     string `koanf:"from"`
	FromName string `koanf:"from_name"`
	// TLS is none, starttls or tls.
	TLS string `koanf:"tls"`
}

// S3 configures the object storage for uploaded and generated files.
type S3 struct {
	Endpoint  string `koanf:"endpoint"`
	AccessKey string `koanf:"access_key"`
	SecretKey string `koanf:"secret_key"`
	Bucket    string `koanf:"bucket"`
	Region    string `koanf:"region"`
	UseSSL    bool   `koanf:"use_ssl"`
}

// Files configures upload checks.
type Files struct {
	// Scan is required (reject uploads when ClamAV is unavailable), optional
	// (scan when reachable) or off.
	Scan       string `koanf:"scan"`
	ClamAVAddr string `koanf:"clamav_addr"`
	MaxBytes   int64  `koanf:"max_bytes"`
}

// App holds product-level settings.
type App struct {
	// PublicURL is the canonical external URL of the web application.
	PublicURL string `koanf:"public_url"`
	// DefaultLocale is used when neither the user nor the tenant chose a language.
	DefaultLocale string `koanf:"default_locale"`
	// ShellURL is where the API reads the web client's index.html to build
	// the showcase pages for search engines and link previews.
	ShellURL string `koanf:"shell_url"`
}

// HTTP configures the API server.
type HTTP struct {
	Addr              string        `koanf:"addr"`
	ReadHeaderTimeout time.Duration `koanf:"read_header_timeout"`
	ReadTimeout       time.Duration `koanf:"read_timeout"`
	IdleTimeout       time.Duration `koanf:"idle_timeout"`
	ShutdownTimeout   time.Duration `koanf:"shutdown_timeout"`
	// RequestTimeout bounds non-streaming requests; streaming endpoints manage their own deadlines.
	RequestTimeout time.Duration `koanf:"request_timeout"`
	MaxBodyBytes   int64         `koanf:"max_body_bytes"`
	// TrustedProxies lists CIDRs whose X-Forwarded-For header is trusted.
	TrustedProxies []string `koanf:"trusted_proxies"`

	trustedPrefixes []netip.Prefix
}

// TrustedPrefixes returns the parsed TrustedProxies.
func (h HTTP) TrustedPrefixes() []netip.Prefix { return h.trustedPrefixes }

// Database configures PostgreSQL access.
type Database struct {
	// URL is the DSN of the application role (subject to row-level security).
	URL string `koanf:"url"`
	// MigrateURL is the DSN of the schema owner used by migrations; defaults to URL.
	MigrateURL string `koanf:"migrate_url"`
	MaxConns   int32  `koanf:"max_conns"`
	MinConns   int32  `koanf:"min_conns"`
}

// Valkey configures the cache and rate-limit store (Redis protocol).
type Valkey struct {
	Addr     string `koanf:"addr"`
	Password string `koanf:"password"`
	DB       int    `koanf:"db"`
}

// Log configures structured logging.
type Log struct {
	Level  string `koanf:"level"`
	Format string `koanf:"format"`
}

// OTel configures OpenTelemetry tracing; tracing is disabled when Endpoint is empty.
type OTel struct {
	Endpoint    string  `koanf:"endpoint"`
	Insecure    bool    `koanf:"insecure"`
	SampleRatio float64 `koanf:"sample_ratio"`
}

// Worker configures the background job processor.
type Worker struct {
	// Addr serves health checks and metrics of the worker process.
	Addr        string `koanf:"addr"`
	Concurrency int    `koanf:"concurrency"`
}

func defaults() map[string]any {
	return map[string]any{
		"env":                           string(EnvDevelopment),
		"app.public_url":                "http://localhost:3000",
		"app.default_locale":            "kk",
		"http.addr":                     ":8080",
		"http.read_header_timeout":      "5s",
		"http.read_timeout":             "60s",
		"http.idle_timeout":             "120s",
		"http.shutdown_timeout":         "20s",
		"http.request_timeout":          "30s",
		"http.max_body_bytes":           1 << 20,
		"http.trusted_proxies":          "",
		"database.max_conns":            20,
		"database.min_conns":            2,
		"valkey.addr":                   "localhost:6379",
		"valkey.db":                     0,
		"log.level":                     "info",
		"log.format":                    "json",
		"otel.sample_ratio":             1.0,
		"worker.addr":                   ":8081",
		"worker.concurrency":            20,
		"tenant.base_domain":            "localhost",
		"tenant.default":                "",
		"auth.access_ttl":               "15m",
		"auth.refresh_ttl":              "720h",
		"smtp.port":                     1025,
		"smtp.from":                     "no-reply@qadam.kz",
		"smtp.from_name":                "Qadam",
		"smtp.tls":                      "none",
		"s3.bucket":                     "qadam",
		"s3.region":                     "us-east-1",
		"files.scan":                    "optional",
		"gotenberg.timeout":             "90s",
		"files.max_bytes":               25 << 20,
		"ai.default":                    "mock",
		"ai.routes":                     "",
		"ai.anthropic_server_fallbacks": true,
		"ai.openai_local":               "auto",
		"ai.embedding_min_similarity":   0.3,
		"ai.timezone":                   "Asia/Almaty",
		"ai.request_timeout":            "120s",
	}
}

// Load reads the configuration from the process environment.
func Load() (*Config, error) { return FromEnviron(os.Environ()) }

// FromEnviron builds the configuration from KEY=VALUE pairs; QADAM_ENV maps to
// "env" and QADAM_DATABASE_MAX_CONNS to "database.max_conns".
func FromEnviron(environ []string) (*Config, error) {
	k := koanf.New(".")
	if err := k.Load(confmap.Provider(defaults(), "."), nil); err != nil {
		return nil, fmt.Errorf("config: load defaults: %w", err)
	}

	values := map[string]any{}
	for _, kv := range environ {
		name, value, ok := strings.Cut(kv, "=")
		if !ok || !strings.HasPrefix(name, envPrefix) {
			continue
		}
		values[envKey(name)] = value
	}
	if err := k.Load(confmap.Provider(values, "."), nil); err != nil {
		return nil, fmt.Errorf("config: load environment: %w", err)
	}

	var cfg Config
	decoder := &mapstructure.DecoderConfig{
		DecodeHook: mapstructure.ComposeDecodeHookFunc(
			mapstructure.StringToTimeDurationHookFunc(),
			mapstructure.StringToSliceHookFunc(","),
		),
		Result:           &cfg,
		WeaklyTypedInput: true,
		ErrorUnused:      true,
		TagName:          "koanf",
	}
	if err := k.UnmarshalWithConf("", &cfg, koanf.UnmarshalConf{Tag: "koanf", DecoderConfig: decoder}); err != nil {
		return nil, fmt.Errorf("config: %w", err)
	}
	if err := cfg.finalize(); err != nil {
		return nil, err
	}
	return &cfg, nil
}

func envKey(name string) string {
	rest := strings.ToLower(strings.TrimPrefix(name, envPrefix))
	section, field, ok := strings.Cut(rest, "_")
	if !ok {
		return rest
	}
	return section + "." + field
}

func (c *Config) finalize() error {
	if c.Database.MigrateURL == "" {
		c.Database.MigrateURL = c.Database.URL
	}
	c.HTTP.TrustedProxies = slices.DeleteFunc(c.HTTP.TrustedProxies, func(s string) bool {
		return strings.TrimSpace(s) == ""
	})
	return c.Validate()
}

// Validate reports every invalid setting at once.
func (c *Config) Validate() error {
	var errs []error
	fail := func(format string, args ...any) { errs = append(errs, fmt.Errorf(format, args...)) }

	switch c.Env {
	case EnvDevelopment, EnvTest, EnvProduction:
	default:
		fail("QADAM_ENV must be development, test or production, got %q", c.Env)
	}

	if u, err := url.Parse(c.App.PublicURL); err != nil || u.Scheme == "" || u.Host == "" {
		fail("QADAM_APP_PUBLIC_URL must be an absolute URL, got %q", c.App.PublicURL)
	} else if c.Env == EnvProduction && u.Scheme != "https" {
		fail("QADAM_APP_PUBLIC_URL must use https in production")
	}
	if !slices.Contains([]string{"kk", "ru", "en"}, c.App.DefaultLocale) {
		fail("QADAM_APP_DEFAULT_LOCALE must be kk, ru or en, got %q", c.App.DefaultLocale)
	}

	if c.HTTP.Addr == "" {
		fail("QADAM_HTTP_ADDR is required")
	}
	for _, d := range []struct {
		name  string
		value time.Duration
	}{
		{"QADAM_HTTP_READ_HEADER_TIMEOUT", c.HTTP.ReadHeaderTimeout},
		{"QADAM_HTTP_READ_TIMEOUT", c.HTTP.ReadTimeout},
		{"QADAM_HTTP_IDLE_TIMEOUT", c.HTTP.IdleTimeout},
		{"QADAM_HTTP_SHUTDOWN_TIMEOUT", c.HTTP.ShutdownTimeout},
		{"QADAM_HTTP_REQUEST_TIMEOUT", c.HTTP.RequestTimeout},
	} {
		if d.value <= 0 {
			fail("%s must be a positive duration", d.name)
		}
	}
	if c.HTTP.MaxBodyBytes < 1024 {
		fail("QADAM_HTTP_MAX_BODY_BYTES must be at least 1024")
	}
	c.HTTP.trustedPrefixes = c.HTTP.trustedPrefixes[:0]
	for _, cidr := range c.HTTP.TrustedProxies {
		p, err := netip.ParsePrefix(strings.TrimSpace(cidr))
		if err != nil {
			fail("QADAM_HTTP_TRUSTED_PROXIES: invalid CIDR %q", cidr)
			continue
		}
		c.HTTP.trustedPrefixes = append(c.HTTP.trustedPrefixes, p.Masked())
	}

	for _, dsn := range []struct{ name, value string }{
		{"QADAM_DATABASE_URL", c.Database.URL},
		{"QADAM_DATABASE_MIGRATE_URL", c.Database.MigrateURL},
	} {
		if u, err := url.Parse(dsn.value); dsn.value == "" || err != nil || (u.Scheme != "postgres" && u.Scheme != "postgresql") {
			fail("%s must be a postgres:// URL", dsn.name)
		}
	}
	if c.Database.MaxConns < 1 || c.Database.MinConns < 0 || c.Database.MinConns > c.Database.MaxConns {
		fail("QADAM_DATABASE_MAX_CONNS must be >= 1 and >= QADAM_DATABASE_MIN_CONNS >= 0")
	}

	if c.Valkey.Addr == "" {
		fail("QADAM_VALKEY_ADDR is required")
	}

	if !slices.Contains([]string{"debug", "info", "warn", "error"}, c.Log.Level) {
		fail("QADAM_LOG_LEVEL must be debug, info, warn or error, got %q", c.Log.Level)
	}
	if c.Log.Format != "json" && c.Log.Format != "text" {
		fail("QADAM_LOG_FORMAT must be json or text, got %q", c.Log.Format)
	}

	if c.OTel.SampleRatio < 0 || c.OTel.SampleRatio > 1 {
		fail("QADAM_OTEL_SAMPLE_RATIO must be within [0, 1]")
	}

	if c.Worker.Addr == "" {
		fail("QADAM_WORKER_ADDR is required")
	}
	if c.Worker.Concurrency < 1 || c.Worker.Concurrency > 500 {
		fail("QADAM_WORKER_CONCURRENCY must be within [1, 500]")
	}

	if strings.TrimSpace(c.Tenant.BaseDomain) == "" {
		fail("QADAM_TENANT_BASE_DOMAIN is required")
	}

	if len(c.Auth.SecretKey) < 32 {
		fail("QADAM_AUTH_SECRET_KEY must be at least 32 characters (openssl rand -base64 48)")
	} else if c.Env == EnvProduction && strings.Contains(strings.ToLower(c.Auth.SecretKey), "dev") {
		fail("QADAM_AUTH_SECRET_KEY looks like a development value; generate a new one for production")
	}
	if c.Auth.AccessTTL < time.Minute || c.Auth.AccessTTL > time.Hour {
		fail("QADAM_AUTH_ACCESS_TTL must be within [1m, 1h]")
	}
	if c.Auth.RefreshTTL < time.Hour || c.Auth.RefreshTTL > 90*24*time.Hour {
		fail("QADAM_AUTH_REFRESH_TTL must be within [1h, 2160h]")
	}

	if !slices.Contains([]string{"none", "starttls", "tls"}, c.SMTP.TLS) {
		fail("QADAM_SMTP_TLS must be none, starttls or tls, got %q", c.SMTP.TLS)
	}
	if c.Env == EnvProduction && c.SMTP.Host == "" {
		fail("QADAM_SMTP_HOST is required in production")
	}

	if c.Env != EnvTest && (c.S3.Endpoint == "" || c.S3.AccessKey == "" || c.S3.SecretKey == "") {
		fail("QADAM_S3_ENDPOINT, QADAM_S3_ACCESS_KEY and QADAM_S3_SECRET_KEY are required")
	}
	if c.S3.Bucket == "" {
		fail("QADAM_S3_BUCKET is required")
	}

	if !slices.Contains([]string{"required", "optional", "off"}, c.Files.Scan) {
		fail("QADAM_FILES_SCAN must be required, optional or off, got %q", c.Files.Scan)
	}
	if c.Files.Scan == "required" && c.Files.ClamAVAddr == "" {
		fail("QADAM_FILES_CLAMAV_ADDR is required when QADAM_FILES_SCAN=required")
	}
	if c.Env == EnvProduction && c.Files.Scan != "required" {
		fail("QADAM_FILES_SCAN must be required in production")
	}
	if c.App.ShellURL != "" {
		if u, err := url.Parse(c.App.ShellURL); err != nil || (u.Scheme != "http" && u.Scheme != "https") || u.Host == "" {
			fail("QADAM_APP_SHELL_URL must be an http(s) URL, got %q", c.App.ShellURL)
		}
	}
	if c.Gotenberg.URL != "" {
		if u, err := url.Parse(c.Gotenberg.URL); err != nil || (u.Scheme != "http" && u.Scheme != "https") || u.Host == "" {
			fail("QADAM_GOTENBERG_URL must be an http(s) URL, got %q", c.Gotenberg.URL)
		}
	}
	if c.Gotenberg.Timeout < 5*time.Second || c.Gotenberg.Timeout > 10*time.Minute {
		fail("QADAM_GOTENBERG_TIMEOUT must be between 5s and 10m")
	}
	if c.Files.MaxBytes < 1<<20 || c.Files.MaxBytes > 200<<20 {
		fail("QADAM_FILES_MAX_BYTES must be within [1 MiB, 200 MiB]")
	}

	switch c.AI.Default {
	case "mock":
	case "anthropic":
		if c.AI.AnthropicAPIKey == "" {
			fail("QADAM_AI_ANTHROPIC_API_KEY is required when QADAM_AI_DEFAULT=anthropic")
		}
	case "openai":
		if c.AI.OpenAIBaseURL == "" || c.AI.OpenAIModel == "" {
			fail("QADAM_AI_OPENAI_BASE_URL and QADAM_AI_OPENAI_MODEL are required when QADAM_AI_DEFAULT=openai")
		}
	default:
		fail("QADAM_AI_DEFAULT must be mock, anthropic or openai, got %q", c.AI.Default)
	}
	if c.AI.OpenAIBaseURL != "" {
		if u, err := url.Parse(c.AI.OpenAIBaseURL); err != nil || (u.Scheme != "http" && u.Scheme != "https") || u.Host == "" {
			fail("QADAM_AI_OPENAI_BASE_URL must be an http(s) URL, got %q", c.AI.OpenAIBaseURL)
		}
	}
	if !slices.Contains([]string{"auto", "true", "false"}, c.AI.OpenAILocal) {
		fail("QADAM_AI_OPENAI_LOCAL must be auto, true or false, got %q", c.AI.OpenAILocal)
	}
	if c.AI.EmbeddingMinSimilarity < 0 || c.AI.EmbeddingMinSimilarity >= 1 {
		fail("QADAM_AI_EMBEDDING_MIN_SIMILARITY must be within [0, 1)")
	}
	if _, err := time.LoadLocation(c.AI.Timezone); err != nil {
		fail("QADAM_AI_TIMEZONE: unknown time zone %q", c.AI.Timezone)
	}
	if c.AI.RequestTimeout < 10*time.Second || c.AI.RequestTimeout > 30*time.Minute {
		fail("QADAM_AI_REQUEST_TIMEOUT must be within [10s, 30m]")
	}

	if len(errs) > 0 {
		return fmt.Errorf("invalid configuration:\n%w", errors.Join(errs...))
	}
	return nil
}

// OpenAIIsLocal reports whether the OpenAI-compatible server is inside the
// university network. Unless the operator says so, only an address that
// cannot be on the internet counts: by default data is masked (С-10).
func (a AI) OpenAIIsLocal() bool {
	switch a.OpenAILocal {
	case "true":
		return true
	case "auto":
		return LocalEndpoint(a.OpenAIBaseURL)
	default:
		return false
	}
}

// LocalEndpoint reports whether a URL obviously points inside the network:
// localhost, a loopback, private or link-local address, or a single-label
// host name such as a Docker Compose service ("ollama").
func LocalEndpoint(raw string) bool {
	u, err := url.Parse(raw)
	if err != nil {
		return false
	}
	host := strings.TrimSuffix(strings.ToLower(u.Hostname()), ".")
	if host == "" {
		return false
	}
	if addr, err := netip.ParseAddr(host); err == nil {
		addr = addr.Unmap()
		return addr.IsLoopback() || addr.IsPrivate() || addr.IsLinkLocalUnicast()
	}
	return host == "localhost" || strings.HasSuffix(host, ".localhost") || host == "host.docker.internal" ||
		!strings.Contains(host, ".")
}

// IsProduction reports whether the process runs in production.
func (c *Config) IsProduction() bool { return c.Env == EnvProduction }

// AIFromEnviron reads only the language-model settings (QADAM_AI_*) for tools
// that run without the rest of the platform, such as the evaluation harness.
func AIFromEnviron(environ []string) (AI, error) {
	// Placeholders satisfy the settings the tool does not use.
	env := []string{
		"QADAM_DATABASE_URL=postgres://unused@localhost/unused",
		"QADAM_AUTH_SECRET_KEY=" + strings.Repeat("q", 48),
		"QADAM_S3_ENDPOINT=unused:9000", "QADAM_S3_ACCESS_KEY=unused", "QADAM_S3_SECRET_KEY=unused",
	}
	for _, kv := range environ {
		if strings.HasPrefix(kv, envPrefix+"AI_") {
			env = append(env, kv)
		}
	}
	cfg, err := FromEnviron(env)
	if err != nil {
		return AI{}, err
	}
	return cfg.AI, nil
}

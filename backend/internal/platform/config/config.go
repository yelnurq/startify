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
}

// App holds product-level settings.
type App struct {
	// PublicURL is the canonical external URL of the web application.
	PublicURL string `koanf:"public_url"`
	// DefaultLocale is used when neither the user nor the tenant chose a language.
	DefaultLocale string `koanf:"default_locale"`
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
		"env":                      string(EnvDevelopment),
		"app.public_url":           "http://localhost:3000",
		"app.default_locale":       "kk",
		"http.addr":                ":8080",
		"http.read_header_timeout": "5s",
		"http.read_timeout":        "60s",
		"http.idle_timeout":        "120s",
		"http.shutdown_timeout":    "20s",
		"http.request_timeout":     "30s",
		"http.max_body_bytes":      1 << 20,
		"http.trusted_proxies":     "",
		"database.max_conns":       20,
		"database.min_conns":       2,
		"valkey.addr":              "localhost:6379",
		"valkey.db":                0,
		"log.level":                "info",
		"log.format":               "json",
		"otel.sample_ratio":        1.0,
		"worker.addr":              ":8081",
		"worker.concurrency":       20,
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

	if len(errs) > 0 {
		return fmt.Errorf("invalid configuration:\n%w", errors.Join(errs...))
	}
	return nil
}

// IsProduction reports whether the process runs in production.
func (c *Config) IsProduction() bool { return c.Env == EnvProduction }

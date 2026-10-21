package logger

import (
	"bytes"
	"context"
	"encoding/json"
	"log/slog"
	"testing"

	"github.com/yelnurq/startify/backend/internal/platform/config"
)

func TestContextAttributesAreLogged(t *testing.T) {
	var buf bytes.Buffer
	log := New(config.Log{Level: "debug", Format: "json"}, &buf)

	ctx := With(context.Background(), slog.String("request_id", "r-1"))
	ctx = With(ctx, slog.String("tenant", "kazutb"))
	log.InfoContext(ctx, "hello", slog.Int("n", 1))

	var rec map[string]any
	if err := json.Unmarshal(buf.Bytes(), &rec); err != nil {
		t.Fatalf("invalid JSON log line %q: %v", buf.String(), err)
	}
	for k, want := range map[string]any{"msg": "hello", "request_id": "r-1", "tenant": "kazutb", "n": float64(1)} {
		if rec[k] != want {
			t.Errorf("%s = %v, want %v", k, rec[k], want)
		}
	}
}

func TestLevelFiltering(t *testing.T) {
	var buf bytes.Buffer
	log := New(config.Log{Level: "warn", Format: "text"}, &buf)
	log.Info("hidden")
	if buf.Len() != 0 {
		t.Fatalf("info record must be filtered at warn level, got %q", buf.String())
	}
	log.Warn("shown")
	if !bytes.Contains(buf.Bytes(), []byte("shown")) {
		t.Fatalf("warn record missing: %q", buf.String())
	}
}

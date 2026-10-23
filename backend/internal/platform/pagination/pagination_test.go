package pagination

import (
	"testing"

	"github.com/yelnurq/startify/backend/internal/platform/apperr"
)

func ptr(n int) *int { return &n }

func TestLimit(t *testing.T) {
	for _, c := range []struct {
		requested *int
		want      int
	}{{nil, DefaultLimit}, {ptr(1), 1}, {ptr(37), 37}, {ptr(MaxLimit), MaxLimit}} {
		if got, err := Limit(c.requested); err != nil || got != c.want {
			t.Errorf("Limit(%v) = %d, %v; want %d", c.requested, got, err, c.want)
		}
	}
	for _, bad := range []int{0, -5, MaxLimit + 1, 1 << 30} {
		_, err := Limit(ptr(bad))
		e, ok := apperr.As(err)
		if !ok || e.Code != "request.invalid_limit" || e.Kind != apperr.KindBadRequest || e.Params["max"] != MaxLimit {
			t.Errorf("Limit(%d): %v", bad, err)
		}
	}
}

func TestLimitOr(t *testing.T) {
	if got, err := LimitOr(nil, 30, 50); err != nil || got != 30 {
		t.Fatalf("default: %d %v", got, err)
	}
	if got, err := LimitOr(ptr(50), 30, 50); err != nil || got != 50 {
		t.Fatalf("maximum: %d %v", got, err)
	}
	_, err := LimitOr(ptr(51), 30, 50)
	if e, ok := apperr.As(err); !ok || e.Params["max"] != 50 {
		t.Fatalf("over the maximum: %v", err)
	}
}

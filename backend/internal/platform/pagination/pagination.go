// Package pagination encodes opaque cursors for keyset pagination.
package pagination

import (
	"encoding/base64"
	"encoding/json"
	"fmt"

	"github.com/yelnurq/startify/backend/internal/platform/apperr"
)

// DefaultLimit and MaxLimit bound page sizes.
const (
	DefaultLimit = 20
	MaxLimit     = 100
)

// Limit returns the requested page size, or DefaultLimit when none was
// asked for. A size outside [1, MaxLimit] is refused, as openapi.yaml
// declares, rather than quietly changed.
func Limit(requested *int) (int, error) { return LimitOr(requested, DefaultLimit, MaxLimit) }

// LimitOr is Limit with an operation's own default and maximum.
func LimitOr(requested *int, fallback, maxLimit int) (int, error) {
	if requested == nil {
		return fallback, nil
	}
	if *requested < 1 || *requested > maxLimit {
		return 0, apperr.BadRequest("request.invalid_limit", fmt.Sprintf("limit must be between 1 and %d", maxLimit)).
			WithParams(map[string]any{"min": 1, "max": maxLimit})
	}
	return *requested, nil
}

// Encode turns the sort key of the last item into an opaque cursor.
func Encode(key any) string {
	raw, err := json.Marshal(key)
	if err != nil {
		panic(err) // keys are plain structs
	}
	return base64.RawURLEncoding.EncodeToString(raw)
}

// Decode reads a cursor produced by Encode into key.
func Decode(cursor string, key any) error {
	raw, err := base64.RawURLEncoding.DecodeString(cursor)
	if err != nil || json.Unmarshal(raw, key) != nil {
		return apperr.BadRequest("request.invalid_cursor", "the cursor is invalid")
	}
	return nil
}

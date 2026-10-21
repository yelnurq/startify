// Package pagination encodes opaque cursors for keyset pagination.
package pagination

import (
	"encoding/base64"
	"encoding/json"

	"github.com/yelnurq/startify/backend/internal/platform/apperr"
)

// DefaultLimit and MaxLimit bound page sizes.
const (
	DefaultLimit = 20
	MaxLimit     = 100
)

// Limit normalises a requested page size.
func Limit(requested *int) int {
	if requested == nil || *requested < 1 {
		return DefaultLimit
	}
	return min(*requested, MaxLimit)
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

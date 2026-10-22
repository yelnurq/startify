package httpx

import (
	"context"
	"strings"
)

// Locale is the interface language of the request (Accept-Language: the web
// client sends the language the user chose), or "" when it names none of
// kk, ru and en.
func Locale(ctx context.Context) string {
	r, ok := RequestFrom(ctx)
	if !ok {
		return ""
	}
	for part := range strings.SplitSeq(r.Header.Get("Accept-Language"), ",") {
		tag, _, _ := strings.Cut(strings.TrimSpace(part), ";")
		tag, _, _ = strings.Cut(strings.ToLower(tag), "-")
		if tag == "kk" || tag == "ru" || tag == "en" {
			return tag
		}
	}
	return ""
}

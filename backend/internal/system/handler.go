// Package system serves platform metadata.
package system

import (
	"context"
	"time"

	"github.com/yelnurq/startify/backend/internal/apigen"
	"github.com/yelnurq/startify/backend/internal/platform/buildinfo"
)

// Handler implements the system operations of the API.
type Handler struct {
	defaultLocale apigen.Locale
	now           func() time.Time
}

// NewHandler returns the system handler.
func NewHandler(defaultLocale string) *Handler {
	return &Handler{defaultLocale: apigen.Locale(defaultLocale), now: time.Now}
}

// GetMeta implements apigen.StrictServerInterface.
func (h *Handler) GetMeta(context.Context, apigen.GetMetaRequestObject) (apigen.GetMetaResponseObject, error) {
	return apigen.GetMeta200JSONResponse{
		Product:       buildinfo.Product,
		Version:       buildinfo.Version,
		Commit:        buildinfo.Commit,
		BuiltAt:       buildinfo.BuiltAt,
		Locales:       []apigen.Locale{apigen.LocaleKk, apigen.LocaleRu, apigen.LocaleEn},
		DefaultLocale: h.defaultLocale,
		ServerTime:    h.now().UTC(),
	}, nil
}

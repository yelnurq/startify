package directory

import (
	"context"

	"github.com/yelnurq/startify/backend/internal/apigen"
)

// Handler serves the directory operations of the API (tag directory).
type Handler struct {
	svc *Service
}

// NewHandler returns the directory handler.
func NewHandler(svc *Service) *Handler { return &Handler{svc: svc} }

// GetDirectorySettings implements apigen.StrictServerInterface.
func (h *Handler) GetDirectorySettings(ctx context.Context, _ apigen.GetDirectorySettingsRequestObject) (apigen.GetDirectorySettingsResponseObject, error) {
	s, tag, err := h.svc.Settings(ctx)
	if err != nil {
		return nil, err
	}
	return apigen.GetDirectorySettings200JSONResponse{Body: s, Headers: apigen.GetDirectorySettings200ResponseHeaders{ETag: &tag}}, nil
}

// UpdateDirectorySettings implements apigen.StrictServerInterface.
func (h *Handler) UpdateDirectorySettings(ctx context.Context, req apigen.UpdateDirectorySettingsRequestObject) (apigen.UpdateDirectorySettingsResponseObject, error) {
	s, tag, err := h.svc.Save(ctx, *req.Body, req.Params.IfMatch)
	if err != nil {
		return nil, err
	}
	return apigen.UpdateDirectorySettings200JSONResponse{Body: s, Headers: apigen.UpdateDirectorySettings200ResponseHeaders{ETag: &tag}}, nil
}

// TestDirectoryConnection implements apigen.StrictServerInterface.
func (h *Handler) TestDirectoryConnection(ctx context.Context, req apigen.TestDirectoryConnectionRequestObject) (apigen.TestDirectoryConnectionResponseObject, error) {
	out, err := h.svc.TestConnection(ctx, *req.Body)
	if err != nil {
		return nil, err
	}
	return apigen.TestDirectoryConnection200JSONResponse(out), nil
}

// TestDirectoryUser implements apigen.StrictServerInterface.
func (h *Handler) TestDirectoryUser(ctx context.Context, req apigen.TestDirectoryUserRequestObject) (apigen.TestDirectoryUserResponseObject, error) {
	out, err := h.svc.TestUser(ctx, *req.Body)
	if err != nil {
		return nil, err
	}
	return apigen.TestDirectoryUser200JSONResponse(out), nil
}

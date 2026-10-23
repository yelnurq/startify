package showcase

import (
	"context"

	"github.com/yelnurq/startify/backend/internal/apigen"
)

// Handler serves the showcase operations of the API.
type Handler struct {
	svc *Service
}

// NewHandler returns the showcase handler.
func NewHandler(svc *Service) *Handler { return &Handler{svc: svc} }

// GetShowcaseEntry implements apigen.StrictServerInterface.
func (h *Handler) GetShowcaseEntry(ctx context.Context, req apigen.GetShowcaseEntryRequestObject) (apigen.GetShowcaseEntryResponseObject, error) {
	e, tag, err := h.svc.Entry(ctx, req.ProjectId)
	if err != nil {
		return nil, err
	}
	return apigen.GetShowcaseEntry200JSONResponse{Body: e, Headers: apigen.GetShowcaseEntry200ResponseHeaders{ETag: &tag}}, nil
}

// UpdateShowcaseEntry implements apigen.StrictServerInterface.
func (h *Handler) UpdateShowcaseEntry(ctx context.Context, req apigen.UpdateShowcaseEntryRequestObject) (apigen.UpdateShowcaseEntryResponseObject, error) {
	e, tag, err := h.svc.Update(ctx, req.ProjectId, *req.Body, req.Params.IfMatch)
	if err != nil {
		return nil, err
	}
	return apigen.UpdateShowcaseEntry200JSONResponse{Body: e, Headers: apigen.UpdateShowcaseEntry200ResponseHeaders{ETag: &tag}}, nil
}

// tagOf is the ETag of a page as the API returns it: every change of the
// page, its cover or its status included, answers with the version to send
// back in If-Match.
func tagOf(e apigen.ShowcaseEntry) *string {
	t := ETag(int32(e.Version)) //nolint:gosec // versions are small
	return &t
}

// SetShowcaseCover implements apigen.StrictServerInterface.
func (h *Handler) SetShowcaseCover(ctx context.Context, req apigen.SetShowcaseCoverRequestObject) (apigen.SetShowcaseCoverResponseObject, error) {
	e, err := h.svc.SetCover(ctx, req.ProjectId, req.Body)
	if err != nil {
		return nil, err
	}
	return apigen.SetShowcaseCover200JSONResponse{Body: e, Headers: apigen.SetShowcaseCover200ResponseHeaders{ETag: tagOf(e)}}, nil
}

// DeleteShowcaseCover implements apigen.StrictServerInterface.
func (h *Handler) DeleteShowcaseCover(ctx context.Context, req apigen.DeleteShowcaseCoverRequestObject) (apigen.DeleteShowcaseCoverResponseObject, error) {
	e, err := h.svc.DeleteCover(ctx, req.ProjectId)
	if err != nil {
		return nil, err
	}
	return apigen.DeleteShowcaseCover200JSONResponse{Body: e, Headers: apigen.DeleteShowcaseCover200ResponseHeaders{ETag: tagOf(e)}}, nil
}

// PublishShowcaseEntry implements apigen.StrictServerInterface.
func (h *Handler) PublishShowcaseEntry(ctx context.Context, req apigen.PublishShowcaseEntryRequestObject) (apigen.PublishShowcaseEntryResponseObject, error) {
	e, err := h.svc.Publish(ctx, req.ProjectId)
	if err != nil {
		return nil, err
	}
	return apigen.PublishShowcaseEntry200JSONResponse{Body: e, Headers: apigen.PublishShowcaseEntry200ResponseHeaders{ETag: tagOf(e)}}, nil
}

// UnpublishShowcaseEntry implements apigen.StrictServerInterface.
func (h *Handler) UnpublishShowcaseEntry(ctx context.Context, req apigen.UnpublishShowcaseEntryRequestObject) (apigen.UnpublishShowcaseEntryResponseObject, error) {
	e, err := h.svc.Unpublish(ctx, req.ProjectId)
	if err != nil {
		return nil, err
	}
	return apigen.UnpublishShowcaseEntry200JSONResponse{Body: e, Headers: apigen.UnpublishShowcaseEntry200ResponseHeaders{ETag: tagOf(e)}}, nil
}

// ListProjectContacts implements apigen.StrictServerInterface.
func (h *Handler) ListProjectContacts(ctx context.Context, req apigen.ListProjectContactsRequestObject) (apigen.ListProjectContactsResponseObject, error) {
	list, err := h.svc.ProjectContacts(ctx, req.ProjectId, req.Params.Cursor, req.Params.Limit)
	if err != nil {
		return nil, err
	}
	return apigen.ListProjectContacts200JSONResponse(list), nil
}

// ListShowcaseReviews implements apigen.StrictServerInterface.
func (h *Handler) ListShowcaseReviews(ctx context.Context, req apigen.ListShowcaseReviewsRequestObject) (apigen.ListShowcaseReviewsResponseObject, error) {
	list, err := h.svc.Reviews(ctx, req.Params.Status)
	if err != nil {
		return nil, err
	}
	return apigen.ListShowcaseReviews200JSONResponse(list), nil
}

// ReviewShowcaseEntry implements apigen.StrictServerInterface.
func (h *Handler) ReviewShowcaseEntry(ctx context.Context, req apigen.ReviewShowcaseEntryRequestObject) (apigen.ReviewShowcaseEntryResponseObject, error) {
	e, err := h.svc.Review(ctx, req.ProjectId, *req.Body, req.Params.IfMatch)
	if err != nil {
		return nil, err
	}
	return apigen.ReviewShowcaseEntry200JSONResponse{Body: e, Headers: apigen.ReviewShowcaseEntry200ResponseHeaders{ETag: tagOf(e)}}, nil
}

// ListContactRequests implements apigen.StrictServerInterface.
func (h *Handler) ListContactRequests(ctx context.Context, req apigen.ListContactRequestsRequestObject) (apigen.ListContactRequestsResponseObject, error) {
	list, err := h.svc.Contacts(ctx, req.Params.Status, req.Params.Cursor, req.Params.Limit)
	if err != nil {
		return nil, err
	}
	return apigen.ListContactRequests200JSONResponse(list), nil
}

// ModerateContactRequest implements apigen.StrictServerInterface.
func (h *Handler) ModerateContactRequest(ctx context.Context, req apigen.ModerateContactRequestRequestObject) (apigen.ModerateContactRequestResponseObject, error) {
	c, err := h.svc.Moderate(ctx, req.ContactId, req.Body.Forward)
	if err != nil {
		return nil, err
	}
	return apigen.ModerateContactRequest200JSONResponse(c), nil
}

// ListShowcase implements apigen.StrictServerInterface.
func (h *Handler) ListShowcase(ctx context.Context, req apigen.ListShowcaseRequestObject) (apigen.ListShowcaseResponseObject, error) {
	page, err := h.svc.List(ctx, req.Params)
	if err != nil {
		return nil, err
	}
	return apigen.ListShowcase200JSONResponse(page), nil
}

// GetShowcaseFacets implements apigen.StrictServerInterface.
func (h *Handler) GetShowcaseFacets(ctx context.Context, req apigen.GetShowcaseFacetsRequestObject) (apigen.GetShowcaseFacetsResponseObject, error) {
	national := req.Params.Scope != nil && *req.Params.Scope == apigen.GetShowcaseFacetsParamsScopeNational
	f, err := h.svc.Facets(ctx, national)
	if err != nil {
		return nil, err
	}
	return apigen.GetShowcaseFacets200JSONResponse(f), nil
}

// GetShowcaseHighlights implements apigen.StrictServerInterface.
func (h *Handler) GetShowcaseHighlights(ctx context.Context, _ apigen.GetShowcaseHighlightsRequestObject) (apigen.GetShowcaseHighlightsResponseObject, error) {
	out, err := h.svc.Highlights(ctx)
	if err != nil {
		return nil, err
	}
	return apigen.GetShowcaseHighlights200JSONResponse(out), nil
}

// GetShowcaseProject implements apigen.StrictServerInterface.
func (h *Handler) GetShowcaseProject(ctx context.Context, req apigen.GetShowcaseProjectRequestObject) (apigen.GetShowcaseProjectResponseObject, error) {
	p, err := h.svc.Page(ctx, req.Slug)
	if err != nil {
		return nil, err
	}
	return apigen.GetShowcaseProject200JSONResponse(p), nil
}

// ContactShowcaseProject implements apigen.StrictServerInterface.
func (h *Handler) ContactShowcaseProject(ctx context.Context, req apigen.ContactShowcaseProjectRequestObject) (apigen.ContactShowcaseProjectResponseObject, error) {
	out, err := h.svc.Contact(ctx, req.Slug, *req.Body)
	if err != nil {
		return nil, err
	}
	return apigen.ContactShowcaseProject202JSONResponse(out), nil
}

// GetShowcaseImage implements apigen.StrictServerInterface.
func (h *Handler) GetShowcaseImage(ctx context.Context, req apigen.GetShowcaseImageRequestObject) (apigen.GetShowcaseImageResponseObject, error) {
	data, err := h.svc.Image(ctx, req.Slug)
	if err != nil {
		return nil, err
	}
	return imageResponse{data: data}, nil
}

package evaluation

import (
	"context"

	"github.com/yelnurq/startify/backend/internal/apigen"
)

// Handler serves the evaluation operations of the API.
type Handler struct {
	svc *Service
}

// NewHandler returns the evaluation handler.
func NewHandler(svc *Service) *Handler { return &Handler{svc: svc} }

// ListEvaluations implements apigen.StrictServerInterface.
func (h *Handler) ListEvaluations(ctx context.Context, req apigen.ListEvaluationsRequestObject) (apigen.ListEvaluationsResponseObject, error) {
	list, err := h.svc.List(ctx, req.ProjectId)
	if err != nil {
		return nil, err
	}
	return apigen.ListEvaluations200JSONResponse(list), nil
}

// RequestEvaluation implements apigen.StrictServerInterface.
func (h *Handler) RequestEvaluation(ctx context.Context, req apigen.RequestEvaluationRequestObject) (apigen.RequestEvaluationResponseObject, error) {
	e, err := h.svc.Request(ctx, req.ProjectId)
	if err != nil {
		return nil, err
	}
	return apigen.RequestEvaluation202JSONResponse(e), nil
}

// GetEvaluation implements apigen.StrictServerInterface.
func (h *Handler) GetEvaluation(ctx context.Context, req apigen.GetEvaluationRequestObject) (apigen.GetEvaluationResponseObject, error) {
	e, err := h.svc.Get(ctx, req.ProjectId, req.EvaluationId)
	if err != nil {
		return nil, err
	}
	return apigen.GetEvaluation200JSONResponse(e), nil
}

// OverrideEvaluationScore implements apigen.StrictServerInterface.
func (h *Handler) OverrideEvaluationScore(ctx context.Context, req apigen.OverrideEvaluationScoreRequestObject) (apigen.OverrideEvaluationScoreResponseObject, error) {
	e, err := h.svc.Override(ctx, req.ProjectId, req.EvaluationId, *req.Body)
	if err != nil {
		return nil, err
	}
	return apigen.OverrideEvaluationScore200JSONResponse(e), nil
}

// GetEvaluationReport implements apigen.StrictServerInterface.
func (h *Handler) GetEvaluationReport(ctx context.Context, req apigen.GetEvaluationReportRequestObject) (apigen.GetEvaluationReportResponseObject, error) {
	lang := ""
	if req.Params.Locale != nil && req.Params.Locale.Valid() {
		lang = string(*req.Params.Locale)
	}
	pdf, name, err := h.svc.Report(ctx, req.ProjectId, req.EvaluationId, lang)
	if err != nil {
		return nil, err
	}
	return pdfResponse{name: name, data: pdf}, nil
}

// GetRubric implements apigen.StrictServerInterface.
func (h *Handler) GetRubric(ctx context.Context, _ apigen.GetRubricRequestObject) (apigen.GetRubricResponseObject, error) {
	r, tag, err := h.svc.Rubric(ctx)
	if err != nil {
		return nil, err
	}
	return apigen.GetRubric200JSONResponse{Body: r, Headers: apigen.GetRubric200ResponseHeaders{ETag: &tag}}, nil
}

// UpdateRubric implements apigen.StrictServerInterface.
func (h *Handler) UpdateRubric(ctx context.Context, req apigen.UpdateRubricRequestObject) (apigen.UpdateRubricResponseObject, error) {
	r, tag, err := h.svc.UpdateRubric(ctx, req.Body.Weights, req.Params.IfMatch)
	if err != nil {
		return nil, err
	}
	return apigen.UpdateRubric200JSONResponse{Body: r, Headers: apigen.UpdateRubric200ResponseHeaders{ETag: &tag}}, nil
}

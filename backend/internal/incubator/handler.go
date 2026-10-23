package incubator

import (
	"context"

	"github.com/yelnurq/startify/backend/internal/apigen"
	"github.com/yelnurq/startify/backend/internal/platform/apperr"
	"github.com/yelnurq/startify/backend/internal/platform/authz"
	"github.com/yelnurq/startify/backend/internal/platform/pagination"
)

// Handler serves the incubator operations of the API.
type Handler struct {
	svc *Service
}

// NewHandler returns the incubator handler.
func NewHandler(svc *Service) *Handler { return &Handler{svc: svc} }

// GetIncubatorTemplate implements apigen.StrictServerInterface.
func (h *Handler) GetIncubatorTemplate(ctx context.Context, _ apigen.GetIncubatorTemplateRequestObject) (apigen.GetIncubatorTemplateResponseObject, error) {
	if _, err := authz.Require(ctx); err != nil {
		return nil, err
	}
	return apigen.GetIncubatorTemplate200JSONResponse(h.svc.TemplateAPI()), nil
}

// ListProjectSteps implements apigen.StrictServerInterface.
func (h *Handler) ListProjectSteps(ctx context.Context, req apigen.ListProjectStepsRequestObject) (apigen.ListProjectStepsResponseObject, error) {
	list, err := h.svc.Steps(ctx, req.ProjectId)
	if err != nil {
		return nil, err
	}
	return apigen.ListProjectSteps200JSONResponse(list), nil
}

func tag(s apigen.Step) *string {
	t := ETag(int32(s.Version)) //nolint:gosec // versions are small
	return &t
}

// GetProjectStep implements apigen.StrictServerInterface.
func (h *Handler) GetProjectStep(ctx context.Context, req apigen.GetProjectStepRequestObject) (apigen.GetProjectStepResponseObject, error) {
	s, err := h.svc.Step(ctx, req.ProjectId, req.StepKey)
	if err != nil {
		return nil, err
	}
	return apigen.GetProjectStep200JSONResponse{Body: s, Headers: apigen.GetProjectStep200ResponseHeaders{ETag: tag(s)}}, nil
}

func ifMatch(v string) (string, error) {
	if v == "" {
		return "", apperr.PreconditionRequired("precondition.required", "send If-Match with the version you edit")
	}
	return v, nil
}

// UpdateProjectStep implements apigen.StrictServerInterface.
func (h *Handler) UpdateProjectStep(ctx context.Context, req apigen.UpdateProjectStepRequestObject) (apigen.UpdateProjectStepResponseObject, error) {
	version, err := ifMatch(req.Params.IfMatch)
	if err != nil {
		return nil, err
	}
	source := "edit"
	if req.Body.Source != nil && *req.Body.Source == apigen.StepUpdateSourceMentor {
		source = "mentor"
	}
	s, err := h.svc.Save(ctx, req.ProjectId, req.StepKey, req.Body.Values, source, version)
	if err != nil {
		return nil, err
	}
	return apigen.UpdateProjectStep200JSONResponse{Body: s, Headers: apigen.UpdateProjectStep200ResponseHeaders{ETag: tag(s)}}, nil
}

// CheckProjectStep implements apigen.StrictServerInterface.
func (h *Handler) CheckProjectStep(ctx context.Context, req apigen.CheckProjectStepRequestObject) (apigen.CheckProjectStepResponseObject, error) {
	s, err := h.svc.Check(ctx, req.ProjectId, req.StepKey)
	if err != nil {
		return nil, err
	}
	return apigen.CheckProjectStep200JSONResponse{Body: s, Headers: apigen.CheckProjectStep200ResponseHeaders{ETag: tag(s)}}, nil
}

// CompleteProjectStep implements apigen.StrictServerInterface.
func (h *Handler) CompleteProjectStep(ctx context.Context, req apigen.CompleteProjectStepRequestObject) (apigen.CompleteProjectStepResponseObject, error) {
	s, err := h.svc.Complete(ctx, req.ProjectId, req.StepKey)
	if err != nil {
		return nil, err
	}
	return apigen.CompleteProjectStep200JSONResponse{Body: s, Headers: apigen.CompleteProjectStep200ResponseHeaders{ETag: tag(s)}}, nil
}

// ApproveProjectStep implements apigen.StrictServerInterface.
func (h *Handler) ApproveProjectStep(ctx context.Context, req apigen.ApproveProjectStepRequestObject) (apigen.ApproveProjectStepResponseObject, error) {
	s, err := h.svc.Approve(ctx, req.ProjectId, req.StepKey, req.Body.Comment)
	if err != nil {
		return nil, err
	}
	return apigen.ApproveProjectStep200JSONResponse{Body: s, Headers: apigen.ApproveProjectStep200ResponseHeaders{ETag: tag(s)}}, nil
}

// ListStepRevisions implements apigen.StrictServerInterface.
func (h *Handler) ListStepRevisions(ctx context.Context, req apigen.ListStepRevisionsRequestObject) (apigen.ListStepRevisionsResponseObject, error) {
	page, err := h.svc.Revisions(ctx, req.ProjectId, req.StepKey, req.Params)
	if err != nil {
		return nil, err
	}
	return apigen.ListStepRevisions200JSONResponse(page), nil
}

// RestoreStepRevision implements apigen.StrictServerInterface.
func (h *Handler) RestoreStepRevision(ctx context.Context, req apigen.RestoreStepRevisionRequestObject) (apigen.RestoreStepRevisionResponseObject, error) {
	version, err := ifMatch(req.Params.IfMatch)
	if err != nil {
		return nil, err
	}
	s, err := h.svc.Restore(ctx, req.ProjectId, req.StepKey, req.RevisionId, version)
	if err != nil {
		return nil, err
	}
	return apigen.RestoreStepRevision200JSONResponse{Body: s, Headers: apigen.RestoreStepRevision200ResponseHeaders{ETag: tag(s)}}, nil
}

// GetMentorConversation implements apigen.StrictServerInterface.
func (h *Handler) GetMentorConversation(ctx context.Context, req apigen.GetMentorConversationRequestObject) (apigen.GetMentorConversationResponseObject, error) {
	c, err := h.svc.Conversation(ctx, req.ProjectId, req.StepKey, req.Params)
	if err != nil {
		return nil, err
	}
	return apigen.GetMentorConversation200JSONResponse(c), nil
}

// SendMentorMessage implements apigen.StrictServerInterface.
func (h *Handler) SendMentorMessage(ctx context.Context, req apigen.SendMentorMessageRequestObject) (apigen.SendMentorMessageResponseObject, error) {
	return h.svc.Ask(ctx, req.ProjectId, req.StepKey, *req.Body)
}

// ListProjectComments implements apigen.StrictServerInterface.
func (h *Handler) ListProjectComments(ctx context.Context, req apigen.ListProjectCommentsRequestObject) (apigen.ListProjectCommentsResponseObject, error) {
	page, err := h.svc.Comments(ctx, req.ProjectId, req.Params)
	if err != nil {
		return nil, err
	}
	return apigen.ListProjectComments200JSONResponse(page), nil
}

// CreateProjectComment implements apigen.StrictServerInterface.
func (h *Handler) CreateProjectComment(ctx context.Context, req apigen.CreateProjectCommentRequestObject) (apigen.CreateProjectCommentResponseObject, error) {
	c, err := h.svc.AddComment(ctx, req.ProjectId, *req.Body)
	if err != nil {
		return nil, err
	}
	return apigen.CreateProjectComment201JSONResponse(c), nil
}

// UpdateProjectComment implements apigen.StrictServerInterface.
func (h *Handler) UpdateProjectComment(ctx context.Context, req apigen.UpdateProjectCommentRequestObject) (apigen.UpdateProjectCommentResponseObject, error) {
	c, err := h.svc.UpdateComment(ctx, req.ProjectId, req.CommentId, *req.Body)
	if err != nil {
		return nil, err
	}
	return apigen.UpdateProjectComment200JSONResponse(c), nil
}

// DeleteProjectComment implements apigen.StrictServerInterface.
func (h *Handler) DeleteProjectComment(ctx context.Context, req apigen.DeleteProjectCommentRequestObject) (apigen.DeleteProjectCommentResponseObject, error) {
	if err := h.svc.DeleteComment(ctx, req.ProjectId, req.CommentId); err != nil {
		return nil, err
	}
	return apigen.DeleteProjectComment204Response{}, nil
}

// UpdateProjectPresence implements apigen.StrictServerInterface.
func (h *Handler) UpdateProjectPresence(ctx context.Context, req apigen.UpdateProjectPresenceRequestObject) (apigen.UpdateProjectPresenceResponseObject, error) {
	p, err := h.svc.Presence(ctx, req.ProjectId, req.Body.StepKey)
	if err != nil {
		return nil, err
	}
	return apigen.UpdateProjectPresence200JSONResponse(p), nil
}

// ListProjectActivity implements apigen.StrictServerInterface.
func (h *Handler) ListProjectActivity(ctx context.Context, req apigen.ListProjectActivityRequestObject) (apigen.ListProjectActivityResponseObject, error) {
	limit, err := pagination.LimitOr(req.Params.Limit, 30, pagination.MaxLimit)
	if err != nil {
		return nil, err
	}
	list, err := h.svc.Activity(ctx, req.ProjectId, limit)
	if err != nil {
		return nil, err
	}
	return apigen.ListProjectActivity200JSONResponse(list), nil
}

// ListProjectMaterials implements apigen.StrictServerInterface.
func (h *Handler) ListProjectMaterials(ctx context.Context, req apigen.ListProjectMaterialsRequestObject) (apigen.ListProjectMaterialsResponseObject, error) {
	list, err := h.svc.Materials(ctx, req.ProjectId)
	if err != nil {
		return nil, err
	}
	return apigen.ListProjectMaterials200JSONResponse(list), nil
}

// CreateProjectMaterial implements apigen.StrictServerInterface.
func (h *Handler) CreateProjectMaterial(ctx context.Context, req apigen.CreateProjectMaterialRequestObject) (apigen.CreateProjectMaterialResponseObject, error) {
	m, err := h.svc.RequestMaterial(ctx, req.ProjectId, *req.Body)
	if err != nil {
		return nil, err
	}
	return apigen.CreateProjectMaterial202JSONResponse(m), nil
}

// GetProjectMaterialContent implements apigen.StrictServerInterface.
func (h *Handler) GetProjectMaterialContent(ctx context.Context, req apigen.GetProjectMaterialContentRequestObject) (apigen.GetProjectMaterialContentResponseObject, error) {
	f, body, err := h.svc.MaterialFile(ctx, req.ProjectId, req.MaterialId)
	if err != nil {
		return nil, err
	}
	return materialResponse{file: f, body: body}, nil
}

package auth

import (
	"context"
	"net/http"

	"github.com/yelnurq/startify/backend/internal/apigen"
	"github.com/yelnurq/startify/backend/internal/platform/httpx"
)

// Handler serves the auth operations of the API.
type Handler struct {
	svc *Service
}

// NewHandler returns the auth handler.
func NewHandler(svc *Service) *Handler { return &Handler{svc: svc} }

func cookieValue(ctx context.Context, name string) string {
	r, ok := httpx.RequestFrom(ctx)
	if !ok {
		return ""
	}
	c, err := r.Cookie(name)
	if err != nil {
		return ""
	}
	return c.Value
}

func request(ctx context.Context) *http.Request {
	if r, ok := httpx.RequestFrom(ctx); ok {
		return r
	}
	return &http.Request{}
}

// Register implements apigen.StrictServerInterface.
func (h *Handler) Register(ctx context.Context, req apigen.RegisterRequestObject) (apigen.RegisterResponseObject, error) {
	session, cookies, err := h.svc.Register(ctx, *req.Body)
	if err != nil {
		return nil, err
	}
	return cookieResponse{status: http.StatusCreated, cookies: cookies, body: session}, nil
}

// Login implements apigen.StrictServerInterface.
func (h *Handler) Login(ctx context.Context, req apigen.LoginRequestObject) (apigen.LoginResponseObject, error) {
	session, cookies, err := h.svc.Login(ctx, *req.Body)
	if err != nil {
		return nil, err
	}
	return cookieResponse{status: http.StatusOK, cookies: cookies, body: session}, nil
}

// RequestLoginCode implements apigen.StrictServerInterface.
func (h *Handler) RequestLoginCode(ctx context.Context, req apigen.RequestLoginCodeRequestObject) (apigen.RequestLoginCodeResponseObject, error) {
	if err := h.svc.RequestLoginCode(ctx, *req.Body); err != nil {
		return nil, err
	}
	return apigen.RequestLoginCode202Response{}, nil
}

// VerifyLoginCode implements apigen.StrictServerInterface.
func (h *Handler) VerifyLoginCode(ctx context.Context, req apigen.VerifyLoginCodeRequestObject) (apigen.VerifyLoginCodeResponseObject, error) {
	session, cookies, err := h.svc.VerifyLoginCode(ctx, *req.Body)
	if err != nil {
		return nil, err
	}
	return cookieResponse{status: http.StatusOK, cookies: cookies, body: session}, nil
}

// RefreshSession implements apigen.StrictServerInterface.
func (h *Handler) RefreshSession(ctx context.Context, _ apigen.RefreshSessionRequestObject) (apigen.RefreshSessionResponseObject, error) {
	cookies, err := h.svc.Refresh(ctx, cookieValue(ctx, CookieRefresh), cookieValue(ctx, CookieCSRF))
	if err != nil {
		if p := httpx.ProblemFrom(request(ctx), err); p.Status == http.StatusUnauthorized {
			// The session is over: clear the cookies so the client stops trying.
			return problemResponse(request(ctx), err, h.svc.cookies.cleared()), nil
		}
		return nil, err
	}
	return cookieResponse{status: http.StatusNoContent, cookies: cookies}, nil
}

// Logout implements apigen.StrictServerInterface.
func (h *Handler) Logout(ctx context.Context, _ apigen.LogoutRequestObject) (apigen.LogoutResponseObject, error) {
	if err := h.svc.Logout(ctx, cookieValue(ctx, CookieRefresh)); err != nil {
		return nil, err
	}
	return cookieResponse{status: http.StatusNoContent, cookies: h.svc.cookies.cleared()}, nil
}

// GetSession implements apigen.StrictServerInterface.
func (h *Handler) GetSession(ctx context.Context, _ apigen.GetSessionRequestObject) (apigen.GetSessionResponseObject, error) {
	session, err := h.svc.sessionFor(ctx)
	if err != nil {
		return nil, err
	}
	return apigen.GetSession200JSONResponse(session), nil
}

// ListSessions implements apigen.StrictServerInterface.
func (h *Handler) ListSessions(ctx context.Context, _ apigen.ListSessionsRequestObject) (apigen.ListSessionsResponseObject, error) {
	items, err := h.svc.Sessions(ctx)
	if err != nil {
		return nil, err
	}
	return apigen.ListSessions200JSONResponse{Items: items}, nil
}

// RevokeSession implements apigen.StrictServerInterface.
func (h *Handler) RevokeSession(ctx context.Context, req apigen.RevokeSessionRequestObject) (apigen.RevokeSessionResponseObject, error) {
	current, err := h.svc.RevokeSession(ctx, req.SessionId)
	if err != nil {
		return nil, err
	}
	if current {
		return cookieResponse{status: http.StatusNoContent, cookies: h.svc.cookies.cleared()}, nil
	}
	return apigen.RevokeSession204Response{}, nil
}

// RevokeAllSessions implements apigen.StrictServerInterface.
func (h *Handler) RevokeAllSessions(ctx context.Context, _ apigen.RevokeAllSessionsRequestObject) (apigen.RevokeAllSessionsResponseObject, error) {
	if err := h.svc.RevokeOtherSessions(ctx); err != nil {
		return nil, err
	}
	return apigen.RevokeAllSessions204Response{}, nil
}

// VerifyEmail implements apigen.StrictServerInterface.
func (h *Handler) VerifyEmail(ctx context.Context, req apigen.VerifyEmailRequestObject) (apigen.VerifyEmailResponseObject, error) {
	if err := h.svc.VerifyEmail(ctx, req.Body.Token); err != nil {
		return nil, err
	}
	return apigen.VerifyEmail204Response{}, nil
}

// ResendVerificationEmail implements apigen.StrictServerInterface.
func (h *Handler) ResendVerificationEmail(ctx context.Context, _ apigen.ResendVerificationEmailRequestObject) (apigen.ResendVerificationEmailResponseObject, error) {
	if err := h.svc.ResendVerification(ctx); err != nil {
		return nil, err
	}
	return apigen.ResendVerificationEmail202Response{}, nil
}

// ForgotPassword implements apigen.StrictServerInterface.
func (h *Handler) ForgotPassword(ctx context.Context, req apigen.ForgotPasswordRequestObject) (apigen.ForgotPasswordResponseObject, error) {
	if err := h.svc.ForgotPassword(ctx, *req.Body); err != nil {
		return nil, err
	}
	return apigen.ForgotPassword202Response{}, nil
}

// ResetPassword implements apigen.StrictServerInterface.
func (h *Handler) ResetPassword(ctx context.Context, req apigen.ResetPasswordRequestObject) (apigen.ResetPasswordResponseObject, error) {
	if err := h.svc.ResetPassword(ctx, *req.Body); err != nil {
		return nil, err
	}
	return apigen.ResetPassword204Response{}, nil
}

// ChangePassword implements apigen.StrictServerInterface.
func (h *Handler) ChangePassword(ctx context.Context, req apigen.ChangePasswordRequestObject) (apigen.ChangePasswordResponseObject, error) {
	if err := h.svc.ChangePassword(ctx, *req.Body); err != nil {
		return nil, err
	}
	return apigen.ChangePassword204Response{}, nil
}

// StartOidc implements apigen.StrictServerInterface. Failures send the
// browser back to the sign-in page: this is a navigation, not an API call.
func (h *Handler) StartOidc(ctx context.Context, req apigen.StartOidcRequestObject) (apigen.StartOidcResponseObject, error) {
	location, cookie, err := h.svc.StartOIDC(ctx, req.ProviderId, req.Params.Redirect)
	if err != nil {
		return cookieResponse{status: http.StatusFound, location: signInErrorURL(err)}, nil
	}
	return cookieResponse{status: http.StatusFound, cookies: []*http.Cookie{cookie}, location: location}, nil
}

// CompleteOidc implements apigen.StrictServerInterface.
func (h *Handler) CompleteOidc(ctx context.Context, req apigen.CompleteOidcRequestObject) (apigen.CompleteOidcResponseObject, error) {
	cookies, location, err := h.svc.CompleteOIDC(ctx, req.ProviderId, cookieValue(ctx, CookieOIDC), req.Params)
	if err != nil {
		return cookieResponse{
			status: http.StatusFound, cookies: []*http.Cookie{h.svc.cookies.oidcCleared()}, location: signInErrorURL(err),
		}, nil
	}
	return cookieResponse{status: http.StatusFound, cookies: cookies, location: location}, nil
}

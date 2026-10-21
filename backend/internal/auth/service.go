// Package auth signs members in and keeps their sessions: passwords
// (argon2id), e-mailed one-time codes, university single sign-on (OIDC),
// short-lived access tokens with rotating refresh tokens in httpOnly cookies,
// CSRF protection, and revocation (D-022).
package auth

import (
	"context"
	"crypto/hmac"
	"fmt"
	"net/http"
	"net/netip"
	"slices"
	"strings"
	"time"

	"github.com/google/uuid"

	"github.com/yelnurq/startify/backend/internal/apigen"
	"github.com/yelnurq/startify/backend/internal/audit"
	"github.com/yelnurq/startify/backend/internal/auth/authdb"
	"github.com/yelnurq/startify/backend/internal/mail"
	"github.com/yelnurq/startify/backend/internal/platform/apperr"
	"github.com/yelnurq/startify/backend/internal/platform/authz"
	"github.com/yelnurq/startify/backend/internal/platform/config"
	"github.com/yelnurq/startify/backend/internal/platform/db"
	"github.com/yelnurq/startify/backend/internal/platform/httpx"
	"github.com/yelnurq/startify/backend/internal/platform/ratelimit"
	"github.com/yelnurq/startify/backend/internal/platform/secrets"
	"github.com/yelnurq/startify/backend/internal/platform/tenancy"
	"github.com/yelnurq/startify/backend/internal/platform/validate"
	"github.com/yelnurq/startify/backend/internal/tenant"
	"github.com/yelnurq/startify/backend/internal/user"
)

// Token purposes and lifetimes.
const (
	purposeVerifyEmail   = "verify_email"
	purposeResetPassword = "reset_password"
	purposeInvite        = "invite"
	purposeLoginCode     = "login_code"

	verifyEmailTTL   = 24 * time.Hour
	resetPasswordTTL = time.Hour
	inviteTTL        = 7 * 24 * time.Hour
	loginCodeTTL     = 10 * time.Minute

	maxCodeAttempts = 5
	// rotationGrace tolerates a refresh token presented again right after it
	// was rotated: parallel tabs refreshing at the same moment.
	rotationGrace = 30 * time.Second

	providerPassword = "password"
)

// Errors with stable codes.
var (
	errInvalidCredentials = apperr.Unauthorized("auth.invalid_credentials", "the e-mail or password is wrong")
	errCodeInvalid        = apperr.Unauthorized("auth.code_invalid", "the code is wrong or expired")
	errTokenInvalid       = apperr.BadRequest("auth.token_invalid", "the link is invalid or expired")
	errSessionExpired     = apperr.Unauthorized("auth.session_expired", "the session has expired")
	errSessionRevoked     = apperr.Unauthorized("auth.session_revoked", "the session was ended")
	errNotMember          = apperr.Forbidden("auth.not_a_member", "the account is not a member of this university")
	errSuspended          = apperr.Forbidden("auth.account_suspended", "access to the account is suspended")
	errEmailTaken         = apperr.Conflict("auth.email_taken", "the e-mail is already registered")
	errSessionNotFound    = apperr.NotFound("session.not_found", "the session does not exist")
	errTooManyAttempts    = "auth.too_many_attempts"
)

// Limits of abuse-prone operations. A whole campus often reaches the
// platform through one NAT address, so limits per address are sized for a
// lecture hall signing in at once; guessing is stopped by the limits per
// account and per code (D-029).
var (
	limitLoginIP      = ratelimit.Limit{Rate: 600, Burst: 120, Period: time.Hour}
	limitLoginEmail   = ratelimit.Limit{Rate: 10, Burst: 10, Period: 15 * time.Minute}
	limitRegisterIP   = ratelimit.Limit{Rate: 300, Burst: 100, Period: time.Hour}
	limitCodeIP       = ratelimit.Limit{Rate: 300, Burst: 100, Period: time.Hour}
	limitCodeEmail    = ratelimit.PerHour(5)
	limitVerifyIP     = ratelimit.Limit{Rate: 600, Burst: 120, Period: time.Hour}
	limitRecoveryIP   = ratelimit.Limit{Rate: 120, Burst: 40, Period: time.Hour}
	limitRecoveryMail = ratelimit.PerHour(5)
	limitResendUser   = ratelimit.PerHour(5)
	limitPasswordUser = ratelimit.PerHour(10)
)

// Service implements sign-in and session management.
type Service struct {
	cfg         config.Auth
	db          *db.DB
	keys        *secrets.Keyring
	tokens      *accessTokens
	revocations *Revocations
	limiter     *ratelimit.Limiter
	mailer      *mail.Mailer
	users       *user.Service
	tenants     *tenant.Service
	audit       *audit.Logger
	cookies     cookieJar
	oidc        *oidcProviders
	now         func() time.Time
}

// Deps are the collaborators of the service.
type Deps struct {
	Config      config.Auth
	PublicURL   string
	DB          *db.DB
	Keys        *secrets.Keyring
	Revocations *Revocations
	Limiter     *ratelimit.Limiter
	Mailer      *mail.Mailer
	Users       *user.Service
	Tenants     *tenant.Service
	Audit       *audit.Logger
}

// NewService returns the auth service.
func NewService(d Deps) *Service {
	return &Service{
		cfg:         d.Config,
		db:          d.DB,
		keys:        d.Keys,
		tokens:      newAccessTokens(d.Keys, d.Config.AccessTTL),
		revocations: d.Revocations,
		limiter:     d.Limiter,
		mailer:      d.Mailer,
		users:       d.Users,
		tenants:     d.Tenants,
		audit:       d.Audit,
		cookies: cookieJar{
			secure:     strings.HasPrefix(d.PublicURL, "https://"),
			accessTTL:  d.Config.AccessTTL,
			refreshTTL: d.Config.RefreshTTL,
		},
		oidc: newOIDCProviders(),
		now:  time.Now,
	}
}

func (s *Service) q(ctx context.Context) *authdb.Queries { return authdb.New(s.db.Conn(ctx)) }

func normalizeEmail(e string) string { return strings.ToLower(strings.TrimSpace(e)) }

func clientKey(ctx context.Context) string {
	if ip := httpx.ClientIPFrom(ctx); ip.IsValid() {
		return ip.String()
	}
	return "unknown"
}

// outcome carries an error that must be returned after the transaction
// commits (for example a counted failed attempt).
type outcome struct{ err error }

// startSession records a session and returns its cookies (inside the
// transaction of ctx). The role version is read before the membership, so a
// concurrent change of roles always invalidates the issued token.
func (s *Service) startSession(ctx context.Context, userID uuid.UUID, superAdmin bool) ([]*http.Cookie, error) {
	t, err := tenancy.Require(ctx)
	if err != nil {
		return nil, err
	}
	version, err := s.revocations.roleVersion(ctx, t.ID, userID)
	if err != nil {
		return nil, err
	}
	m, ok, err := s.users.Membership(ctx, userID)
	if err != nil {
		return nil, err
	}
	if !ok {
		return nil, errNotMember
	}
	sid, err := uuid.NewV7()
	if err != nil {
		return nil, fmt.Errorf("auth: session id: %w", err)
	}
	refresh, hash := newRefreshToken()
	now := s.now()
	_, err = s.q(ctx).CreateSession(ctx, authdb.CreateSessionParams{
		ID: sid, TenantID: t.ID, UserID: userID, RefreshHash: hash,
		UserAgent: userAgent(ctx), Ip: clientIP(ctx), ExpiresAt: now.Add(s.cfg.RefreshTTL),
	})
	if err != nil {
		return nil, fmt.Errorf("auth: create session: %w", err)
	}
	access, err := s.tokens.issue(principal(t.ID, sid, userID, superAdmin, m.Roles), version, now)
	if err != nil {
		return nil, err
	}
	return s.cookies.session(access, refresh), nil
}

func principal(tenantID, sid, userID uuid.UUID, superAdmin bool, roles []string) authz.Principal {
	p := authz.Principal{UserID: userID, TenantID: tenantID, SessionID: sid, SuperAdmin: superAdmin}
	for _, r := range roles {
		p.Roles = append(p.Roles, authz.Role(r))
	}
	return p
}

func userAgent(ctx context.Context) *string {
	if ua := httpx.UserAgentFrom(ctx); ua != "" {
		return &ua
	}
	return nil
}

func clientIP(ctx context.Context) *netip.Addr {
	if ip := httpx.ClientIPFrom(ctx); ip.IsValid() {
		return &ip
	}
	return nil
}

// admit checks that a signed-in user may enter the current university and
// activates an invitation (inside the transaction of ctx).
func (s *Service) admit(ctx context.Context, u authdb.User) error {
	if u.Status != "active" {
		return errSuspended
	}
	m, ok, err := s.users.Membership(ctx, u.ID)
	if err != nil {
		return err
	}
	switch {
	case !ok:
		return errNotMember
	case m.Status == user.StatusSuspended:
		return errSuspended
	case m.Status == user.StatusInvited:
		return s.users.Activate(ctx, u.ID)
	}
	return nil
}

func (s *Service) tenantName(ctx context.Context, locale string) string {
	settings, err := s.tenants.CurrentSettings(ctx)
	if err != nil {
		return "Qadam"
	}
	return settings.Name.Best(locale)
}

func link(ctx context.Context, path, token string) string {
	t, _ := tenancy.FromContext(ctx)
	return t.Origin + path + "?token=" + token
}

// Register creates an account with e-mail and password and signs it in.
func (s *Service) Register(ctx context.Context, body apigen.RegisterRequest) (apigen.Session, []*http.Cookie, error) {
	if _, err := tenancy.Require(ctx); err != nil {
		return apigen.Session{}, nil, err
	}
	if err := s.limiter.Allow(ctx, "register:ip:"+clientKey(ctx), limitRegisterIP, errTooManyAttempts); err != nil {
		return apigen.Session{}, nil, err
	}
	settings, err := s.tenants.CurrentSettings(ctx)
	if err != nil {
		return apigen.Session{}, nil, err
	}
	if !settings.SelfSignup {
		return apigen.Session{}, nil, apperr.Forbidden("auth.signup_disabled", "self sign-up is closed")
	}
	email := normalizeEmail(string(body.Email))
	var v validate.Errors
	name := v.Text("full_name", body.FullName, 2, 120)
	if !body.Locale.Valid() {
		v.Add("locale", validate.CodeInvalid, nil)
	}
	if err := v.Err(); err != nil {
		return apigen.Session{}, nil, err
	}
	if !domainAllowed(email, settings.EmailDomains) {
		return apigen.Session{}, nil, apperr.New(apperr.KindValidation, "auth.email_domain_not_allowed",
			"use an e-mail address of the university").WithParams(map[string]any{"domains": strings.Join(settings.EmailDomains, ", ")})
	}
	if body.ConsentVersion != settings.ConsentVersion {
		return apigen.Session{}, nil, apperr.New(apperr.KindValidation, "auth.consent_required", "accept the current consent")
	}
	if err := CheckPassword(body.Password, email, name); err != nil {
		return apigen.Session{}, nil, err
	}

	existing, err := s.q(ctx).GetUserByEmail(ctx, email)
	switch {
	case err == nil:
		return s.joinExisting(ctx, existing, body.Password, settings.ConsentVersion)
	case !db.IsNoRows(err):
		return apigen.Session{}, nil, fmt.Errorf("auth: find user: %w", err)
	}

	hash, err := HashPassword(body.Password)
	if err != nil {
		return apigen.Session{}, nil, err
	}
	consent := settings.ConsentVersion
	var cookies []*http.Cookie
	var userID uuid.UUID
	err = s.db.Tx(ctx, func(ctx context.Context) error {
		q := s.q(ctx)
		u, err := q.CreateUser(ctx, authdb.CreateUserParams{Email: email, FullName: name, Locale: string(body.Locale)})
		if db.IsUniqueViolation(err, "") {
			return errEmailTaken
		}
		if err != nil {
			return err
		}
		userID = u.ID
		if err := q.UpsertPasswordIdentity(ctx, authdb.UpsertPasswordIdentityParams{
			UserID: u.ID, Subject: u.ID.String(), PasswordHash: &hash,
		}); err != nil {
			return err
		}
		if err := s.users.EnsureProfile(ctx, u.ID, u.FullName); err != nil {
			return err
		}
		if _, err := s.users.Join(ctx, u.ID, []string{string(authz.RoleStudent)}, user.StatusActive, &consent); err != nil {
			return err
		}
		if err := s.sendVerification(ctx, u.ID, u.Email, u.FullName, u.Locale); err != nil {
			return err
		}
		if err := s.audit.Record(ctx, audit.Event{Action: "auth.register", ActorID: u.ID, TargetType: "user", TargetID: u.ID.String()}); err != nil {
			return err
		}
		cookies, err = s.startSession(ctx, u.ID, u.IsSuperAdmin)
		return err
	})
	if err != nil {
		return apigen.Session{}, nil, err
	}
	session, err := s.users.Session(ctx, userID)
	return session, cookies, err
}

// joinExisting handles registration with an e-mail that already has an
// account: the right password proves ownership and joins this university.
func (s *Service) joinExisting(ctx context.Context, u authdb.User, password, consent string) (apigen.Session, []*http.Cookie, error) {
	hash, err := s.q(ctx).GetPasswordHash(ctx, u.ID)
	if db.IsNoRows(err) {
		_, _ = VerifyPassword(burnPasswordCheck(), password)
		return apigen.Session{}, nil, errEmailTaken
	}
	if err != nil {
		return apigen.Session{}, nil, fmt.Errorf("auth: password: %w", err)
	}
	if ok, err := VerifyPassword(hash, password); err != nil || !ok {
		return apigen.Session{}, nil, errEmailTaken
	}
	var cookies []*http.Cookie
	err = s.db.Tx(ctx, func(ctx context.Context) error {
		if _, err := s.users.Join(ctx, u.ID, []string{string(authz.RoleStudent)}, user.StatusActive, &consent); err != nil {
			return err
		}
		if err := s.users.EnsureProfile(ctx, u.ID, u.FullName); err != nil {
			return err
		}
		if err := s.admit(ctx, u); err != nil {
			return err
		}
		if err := s.audit.Record(ctx, audit.Event{Action: "auth.join", ActorID: u.ID, TargetType: "user", TargetID: u.ID.String()}); err != nil {
			return err
		}
		cookies, err = s.startSession(ctx, u.ID, u.IsSuperAdmin)
		return err
	})
	if err != nil {
		return apigen.Session{}, nil, err
	}
	session, err := s.users.Session(ctx, u.ID)
	return session, cookies, err
}

func domainAllowed(email string, domains []string) bool {
	if len(domains) == 0 {
		return true
	}
	_, domain, ok := strings.Cut(email, "@")
	if !ok {
		return false
	}
	return slices.ContainsFunc(domains, func(d string) bool {
		d = strings.ToLower(strings.TrimPrefix(strings.TrimSpace(d), "@"))
		return domain == d || strings.HasSuffix(domain, "."+d)
	})
}

// Login signs in with e-mail and password.
func (s *Service) Login(ctx context.Context, body apigen.LoginRequest) (apigen.Session, []*http.Cookie, error) {
	if _, err := tenancy.Require(ctx); err != nil {
		return apigen.Session{}, nil, err
	}
	email := normalizeEmail(string(body.Email))
	if err := s.limiter.Allow(ctx, "login:ip:"+clientKey(ctx), limitLoginIP, errTooManyAttempts); err != nil {
		return apigen.Session{}, nil, err
	}
	if err := s.limiter.Allow(ctx, "login:email:"+email, limitLoginEmail, errTooManyAttempts); err != nil {
		return apigen.Session{}, nil, err
	}
	u, err := s.q(ctx).GetUserByEmail(ctx, email)
	if db.IsNoRows(err) {
		_, _ = VerifyPassword(burnPasswordCheck(), body.Password)
		return apigen.Session{}, nil, errInvalidCredentials
	}
	if err != nil {
		return apigen.Session{}, nil, fmt.Errorf("auth: find user: %w", err)
	}
	hash, err := s.q(ctx).GetPasswordHash(ctx, u.ID)
	if db.IsNoRows(err) {
		_, _ = VerifyPassword(burnPasswordCheck(), body.Password)
		return apigen.Session{}, nil, errInvalidCredentials
	}
	if err != nil {
		return apigen.Session{}, nil, fmt.Errorf("auth: password: %w", err)
	}
	ok, err := VerifyPassword(hash, body.Password)
	if err != nil {
		return apigen.Session{}, nil, fmt.Errorf("auth: verify: %w", err)
	}
	if !ok {
		_ = s.db.Tx(ctx, func(ctx context.Context) error {
			return s.audit.Record(ctx, audit.Event{Action: "auth.login_failed", ActorID: u.ID, TargetType: "user", TargetID: u.ID.String()})
		})
		return apigen.Session{}, nil, errInvalidCredentials
	}

	var cookies []*http.Cookie
	err = s.db.Tx(ctx, func(ctx context.Context) error {
		if err := s.admit(ctx, u); err != nil {
			return err
		}
		if err := s.q(ctx).TouchIdentity(ctx, authdb.TouchIdentityParams{UserID: u.ID, Provider: providerPassword}); err != nil {
			return err
		}
		if err := s.audit.Record(ctx, audit.Event{
			Action: "auth.login", ActorID: u.ID, TargetType: "user", TargetID: u.ID.String(),
			Metadata: map[string]any{"method": "password"},
		}); err != nil {
			return err
		}
		cookies, err = s.startSession(ctx, u.ID, u.IsSuperAdmin)
		return err
	})
	if err != nil {
		return apigen.Session{}, nil, err
	}
	_ = s.limiter.Reset(ctx, "login:email:"+email)
	session, err := s.users.Session(ctx, u.ID)
	return session, cookies, err
}

// RequestLoginCode e-mails a sign-in code. The answer never reveals whether
// the address has an account.
func (s *Service) RequestLoginCode(ctx context.Context, body apigen.EmailRequest) error {
	if _, err := tenancy.Require(ctx); err != nil {
		return err
	}
	email := normalizeEmail(string(body.Email))
	if err := s.limiter.Allow(ctx, "otp:ip:"+clientKey(ctx), limitCodeIP, errTooManyAttempts); err != nil {
		return err
	}
	if err := s.limiter.Allow(ctx, "otp:email:"+email, limitCodeEmail, errTooManyAttempts); err != nil {
		return err
	}
	u, err := s.q(ctx).GetUserByEmail(ctx, email)
	if db.IsNoRows(err) {
		return nil
	}
	if err != nil {
		return fmt.Errorf("auth: find user: %w", err)
	}
	if u.Status != "active" {
		return nil
	}
	return s.db.Tx(ctx, func(ctx context.Context) error {
		m, ok, err := s.users.Membership(ctx, u.ID)
		if err != nil || !ok || m.Status == user.StatusSuspended {
			return err
		}
		q := s.q(ctx)
		if err := q.ConsumeUserTokens(ctx, authdb.ConsumeUserTokensParams{UserID: u.ID, Purposes: []string{purposeLoginCode}}); err != nil {
			return err
		}
		code, err := newCode()
		if err != nil {
			return err
		}
		if err := s.createToken(ctx, u.ID, purposeLoginCode, loginCodeTTL, func(id uuid.UUID) []byte {
			return codeHash(s.keys, id, u.ID, code)
		}); err != nil {
			return err
		}
		return s.mailer.Send(ctx, u.Email, u.FullName, mail.TemplateLoginCode, u.Locale, map[string]string{
			"name": u.FullName, "code": code, "minutes": "10", "tenant": s.tenantName(ctx, u.Locale),
		})
	})
}

func (s *Service) createToken(ctx context.Context, userID uuid.UUID, purpose string, ttl time.Duration, hash func(id uuid.UUID) []byte) error {
	t, err := tenancy.Require(ctx)
	if err != nil {
		return err
	}
	id, err := uuid.NewV7()
	if err != nil {
		return fmt.Errorf("auth: token id: %w", err)
	}
	err = s.q(ctx).CreateAuthToken(ctx, authdb.CreateAuthTokenParams{
		ID: id, TenantID: t.ID, UserID: userID, Purpose: purpose, TokenHash: hash(id), ExpiresAt: s.now().Add(ttl),
	})
	if err != nil {
		return fmt.Errorf("auth: create %s token: %w", purpose, err)
	}
	return nil
}

// VerifyLoginCode signs in with an e-mailed code.
func (s *Service) VerifyLoginCode(ctx context.Context, body apigen.LoginCodeRequest) (apigen.Session, []*http.Cookie, error) {
	if _, err := tenancy.Require(ctx); err != nil {
		return apigen.Session{}, nil, err
	}
	if err := s.limiter.Allow(ctx, "otp-verify:ip:"+clientKey(ctx), limitVerifyIP, errTooManyAttempts); err != nil {
		return apigen.Session{}, nil, err
	}
	code := strings.TrimSpace(body.Code)
	if len(code) != 6 || strings.Trim(code, "0123456789") != "" {
		return apigen.Session{}, nil, errCodeInvalid
	}
	u, err := s.q(ctx).GetUserByEmail(ctx, normalizeEmail(string(body.Email)))
	if db.IsNoRows(err) {
		return apigen.Session{}, nil, errCodeInvalid
	}
	if err != nil {
		return apigen.Session{}, nil, fmt.Errorf("auth: find user: %w", err)
	}

	var cookies []*http.Cookie
	var out outcome
	err = s.db.Tx(ctx, func(ctx context.Context) error {
		q := s.q(ctx)
		tokens, err := q.ListActiveLoginCodes(ctx, u.ID)
		if err != nil {
			return err
		}
		var match *authdb.AuthToken
		for i := range tokens {
			if hmac.Equal(tokens[i].TokenHash, codeHash(s.keys, tokens[i].ID, u.ID, code)) {
				match = &tokens[i]
			}
		}
		if match == nil {
			for i := range tokens {
				t := &tokens[i]
				attempts, err := q.CountLoginCodeAttempt(ctx, t.ID)
				if err != nil {
					return err
				}
				if attempts >= maxCodeAttempts {
					if _, err := q.ConsumeAuthToken(ctx, t.ID); err != nil {
						return err
					}
				}
			}
			out.err = errCodeInvalid
			return nil
		}
		if n, err := q.ConsumeAuthToken(ctx, match.ID); err != nil || n == 0 {
			if err == nil {
				out.err = errCodeInvalid
			}
			return err
		}
		if err := q.MarkEmailVerified(ctx, u.ID); err != nil {
			return err
		}
		if err := s.admit(ctx, u); err != nil {
			return err
		}
		if err := s.audit.Record(ctx, audit.Event{
			Action: "auth.login", ActorID: u.ID, TargetType: "user", TargetID: u.ID.String(),
			Metadata: map[string]any{"method": "code"},
		}); err != nil {
			return err
		}
		cookies, err = s.startSession(ctx, u.ID, u.IsSuperAdmin)
		return err
	})
	if err == nil {
		err = out.err
	}
	if err != nil {
		return apigen.Session{}, nil, err
	}
	session, err := s.users.Session(ctx, u.ID)
	return session, cookies, err
}

// Refresh rotates the refresh token and issues a new access token. A refresh
// token used again after its rotation grace period means it was copied: the
// session is ended for everyone holding it. No cookies are returned when a
// parallel request has just rotated the token.
func (s *Service) Refresh(ctx context.Context, refresh, csrf string) ([]*http.Cookie, error) {
	t, err := tenancy.Require(ctx)
	if err != nil {
		return nil, err
	}
	if refresh == "" {
		return nil, errSessionExpired
	}
	hash := refreshHash(refresh)
	now := s.now()
	var cookies []*http.Cookie
	var out outcome
	var revoked []uuid.UUID
	err = s.db.Tx(ctx, func(ctx context.Context) error {
		q := s.q(ctx)
		sess, err := q.GetSessionByRefreshHash(ctx, hash)
		if db.IsNoRows(err) {
			prev, err := q.GetSessionByPreviousHash(ctx, hash)
			if db.IsNoRows(err) {
				out.err = errSessionExpired
				return nil
			}
			if err != nil {
				return err
			}
			if prev.RevokedAt == nil && prev.RotatedAt != nil && now.Sub(*prev.RotatedAt) < rotationGrace {
				return nil
			}
			reason := "refresh_reuse"
			if _, err := q.RevokeSession(ctx, authdb.RevokeSessionParams{Reason: &reason, ID: prev.ID, UserID: prev.UserID}); err != nil {
				return err
			}
			revoked = append(revoked, prev.ID)
			out.err = errSessionRevoked
			return s.audit.Record(ctx, audit.Event{Action: "auth.refresh_reuse", ActorID: prev.UserID, TargetType: "session", TargetID: prev.ID.String()})
		}
		if err != nil {
			return err
		}
		if sess.RevokedAt != nil {
			out.err = errSessionRevoked
			return nil
		}
		if !now.Before(sess.ExpiresAt) {
			out.err = errSessionExpired
			return nil
		}
		u, err := q.GetUserByID(ctx, sess.UserID)
		if err != nil {
			return err
		}
		version, err := s.revocations.roleVersion(ctx, t.ID, u.ID)
		if err != nil {
			return err
		}
		m, ok, err := s.users.Membership(ctx, u.ID)
		if err != nil {
			return err
		}
		if u.Status != "active" || !ok || m.Status == user.StatusSuspended {
			reason := "access_lost"
			if _, err := q.RevokeSession(ctx, authdb.RevokeSessionParams{Reason: &reason, ID: sess.ID, UserID: sess.UserID}); err != nil {
				return err
			}
			revoked = append(revoked, sess.ID)
			out.err = errSessionRevoked
			return nil
		}
		next, nextHash := newRefreshToken()
		n, err := q.RotateSession(ctx, authdb.RotateSessionParams{
			NewHash: nextHash, Ip: clientIP(ctx), UserAgent: userAgent(ctx), ID: sess.ID, OldHash: hash,
		})
		if err != nil {
			return err
		}
		if n == 0 {
			// Rotated by a parallel request between the read and the update.
			return nil
		}
		if err := s.users.Touch(ctx, u.ID); err != nil {
			return err
		}
		access, err := s.tokens.issue(principal(t.ID, sess.ID, u.ID, u.IsSuperAdmin, m.Roles), version, now)
		if err != nil {
			return err
		}
		if csrf == "" {
			csrf = tokenEncoding.EncodeToString(secrets.RandomToken(24))
		}
		cookies = []*http.Cookie{s.cookies.access(access), s.cookies.refresh(next), s.cookies.csrf(csrf)}
		return nil
	})
	if err != nil {
		return nil, err
	}
	if err := s.revocations.deny(ctx, revoked...); err != nil {
		return nil, err
	}
	return cookies, out.err
}

// Logout ends the current session, identified by the access token or, when
// it has expired, by the refresh token.
func (s *Service) Logout(ctx context.Context, refresh string) error {
	if _, err := tenancy.Require(ctx); err != nil {
		return err
	}
	var sid uuid.UUID
	err := s.db.Tx(ctx, func(ctx context.Context) error {
		q := s.q(ctx)
		var userID uuid.UUID
		if p, ok := authz.FromContext(ctx); ok {
			sid, userID = p.SessionID, p.UserID
		} else if refresh != "" {
			sess, err := q.GetSessionByRefreshHash(ctx, refreshHash(refresh))
			if db.IsNoRows(err) {
				return nil
			}
			if err != nil {
				return err
			}
			sid, userID = sess.ID, sess.UserID
		} else {
			return nil
		}
		reason := "logout"
		if _, err := q.RevokeSession(ctx, authdb.RevokeSessionParams{Reason: &reason, ID: sid, UserID: userID}); err != nil {
			return err
		}
		return s.audit.Record(ctx, audit.Event{Action: "auth.logout", ActorID: userID, TargetType: "session", TargetID: sid.String()})
	})
	if err != nil {
		return fmt.Errorf("auth: logout: %w", err)
	}
	if sid != uuid.Nil {
		return s.revocations.deny(ctx, sid)
	}
	return nil
}

// Sessions lists the active sessions of the signed-in user.
func (s *Service) Sessions(ctx context.Context) ([]apigen.SessionInfo, error) {
	p, err := authz.Require(ctx)
	if err != nil {
		return nil, err
	}
	var out []apigen.SessionInfo
	err = s.db.ReadTx(ctx, func(ctx context.Context) error {
		rows, err := s.q(ctx).ListActiveSessions(ctx, p.UserID)
		if err != nil {
			return err
		}
		out = make([]apigen.SessionInfo, 0, len(rows))
		for i := range rows {
			r := &rows[i]
			info := apigen.SessionInfo{
				CreatedAt: r.CreatedAt, Current: r.ID == p.SessionID, Id: r.ID,
				LastUsedAt: r.LastUsedAt, UserAgent: r.UserAgent,
			}
			if r.Ip != nil {
				ip := r.Ip.String()
				info.Ip = &ip
			}
			out = append(out, info)
		}
		return nil
	})
	if err != nil {
		return nil, fmt.Errorf("auth: sessions: %w", err)
	}
	return out, nil
}

// RevokeSession ends one session of the signed-in user; the bool reports
// whether it was the current one.
func (s *Service) RevokeSession(ctx context.Context, id uuid.UUID) (bool, error) {
	p, err := authz.Require(ctx)
	if err != nil {
		return false, err
	}
	err = s.db.Tx(ctx, func(ctx context.Context) error {
		reason := "user"
		n, err := s.q(ctx).RevokeSession(ctx, authdb.RevokeSessionParams{Reason: &reason, ID: id, UserID: p.UserID})
		if err != nil {
			return err
		}
		if n == 0 {
			return errSessionNotFound
		}
		return s.audit.Record(ctx, audit.Event{Action: "auth.session_revoked", TargetType: "session", TargetID: id.String()})
	})
	if err != nil {
		return false, err
	}
	return id == p.SessionID, s.revocations.deny(ctx, id)
}

// RevokeOtherSessions ends every session of the user except the current one.
func (s *Service) RevokeOtherSessions(ctx context.Context) error {
	p, err := authz.Require(ctx)
	if err != nil {
		return err
	}
	var sids []uuid.UUID
	err = s.db.Tx(ctx, func(ctx context.Context) error {
		reason := "user"
		sids, err = s.q(ctx).RevokeUserSessions(ctx, authdb.RevokeUserSessionsParams{Reason: &reason, UserID: p.UserID, ExceptID: &p.SessionID})
		if err != nil {
			return err
		}
		return s.audit.Record(ctx, audit.Event{
			Action: "auth.sessions_revoked", TargetType: "user", TargetID: p.UserID.String(),
			Metadata: map[string]any{"count": len(sids)},
		})
	})
	if err != nil {
		return fmt.Errorf("auth: revoke sessions: %w", err)
	}
	return s.revocations.deny(ctx, sids...)
}

func (s *Service) sendVerification(ctx context.Context, userID uuid.UUID, email, name, locale string) error {
	token, hash := newLinkToken(s.keys)
	if err := s.createToken(ctx, userID, purposeVerifyEmail, verifyEmailTTL, func(uuid.UUID) []byte { return hash }); err != nil {
		return err
	}
	return s.mailer.Send(ctx, email, name, mail.TemplateVerifyEmail, locale, map[string]string{
		"name": name, "tenant": s.tenantName(ctx, locale), "link": link(ctx, "/verify-email", token),
	})
}

// VerifyEmail confirms the address with the token from the letter.
func (s *Service) VerifyEmail(ctx context.Context, token string) error {
	if _, err := tenancy.Require(ctx); err != nil {
		return err
	}
	return s.db.Tx(ctx, func(ctx context.Context) error {
		q := s.q(ctx)
		t, err := q.GetAuthToken(ctx, authdb.GetAuthTokenParams{TokenHash: linkHash(s.keys, strings.TrimSpace(token)), Purposes: []string{purposeVerifyEmail}})
		if db.IsNoRows(err) {
			return errTokenInvalid
		}
		if err != nil {
			return err
		}
		if n, err := q.ConsumeAuthToken(ctx, t.ID); err != nil || n == 0 {
			if err == nil {
				err = errTokenInvalid
			}
			return err
		}
		if err := q.MarkEmailVerified(ctx, t.UserID); err != nil {
			return err
		}
		return s.audit.Record(ctx, audit.Event{Action: "auth.email_verified", ActorID: t.UserID, TargetType: "user", TargetID: t.UserID.String()})
	})
}

// ResendVerification sends the verification letter again.
func (s *Service) ResendVerification(ctx context.Context) error {
	p, err := authz.Require(ctx)
	if err != nil {
		return err
	}
	if err := s.limiter.Allow(ctx, "verify-resend:user:"+p.UserID.String(), limitResendUser, errTooManyAttempts); err != nil {
		return err
	}
	return s.db.Tx(ctx, func(ctx context.Context) error {
		q := s.q(ctx)
		u, err := q.GetUserByID(ctx, p.UserID)
		if err != nil {
			return err
		}
		if u.EmailVerifiedAt != nil {
			return nil
		}
		if err := q.ConsumeUserTokens(ctx, authdb.ConsumeUserTokensParams{UserID: u.ID, Purposes: []string{purposeVerifyEmail}}); err != nil {
			return err
		}
		return s.sendVerification(ctx, u.ID, u.Email, u.FullName, u.Locale)
	})
}

// ForgotPassword e-mails a reset link without revealing whether the address
// has an account.
func (s *Service) ForgotPassword(ctx context.Context, body apigen.EmailRequest) error {
	if _, err := tenancy.Require(ctx); err != nil {
		return err
	}
	email := normalizeEmail(string(body.Email))
	if err := s.limiter.Allow(ctx, "recovery:ip:"+clientKey(ctx), limitRecoveryIP, errTooManyAttempts); err != nil {
		return err
	}
	if err := s.limiter.Allow(ctx, "recovery:email:"+email, limitRecoveryMail, errTooManyAttempts); err != nil {
		return err
	}
	u, err := s.q(ctx).GetUserByEmail(ctx, email)
	if db.IsNoRows(err) {
		return nil
	}
	if err != nil {
		return fmt.Errorf("auth: find user: %w", err)
	}
	if u.Status != "active" {
		return nil
	}
	return s.db.Tx(ctx, func(ctx context.Context) error {
		m, ok, err := s.users.Membership(ctx, u.ID)
		if err != nil || !ok || m.Status == user.StatusSuspended {
			return err
		}
		if err := s.q(ctx).ConsumeUserTokens(ctx, authdb.ConsumeUserTokensParams{UserID: u.ID, Purposes: []string{purposeResetPassword}}); err != nil {
			return err
		}
		token, hash := newLinkToken(s.keys)
		if err := s.createToken(ctx, u.ID, purposeResetPassword, resetPasswordTTL, func(uuid.UUID) []byte { return hash }); err != nil {
			return err
		}
		return s.mailer.Send(ctx, u.Email, u.FullName, mail.TemplatePasswordReset, u.Locale, map[string]string{
			"name": u.FullName, "tenant": s.tenantName(ctx, u.Locale), "link": link(ctx, "/reset-password", token),
		})
	})
}

// ResetPassword sets a new password with a reset or invitation link. Every
// session of the account ends.
func (s *Service) ResetPassword(ctx context.Context, body apigen.PasswordResetRequest) error {
	if _, err := tenancy.Require(ctx); err != nil {
		return err
	}
	if err := s.limiter.Allow(ctx, "recovery:ip:"+clientKey(ctx), limitRecoveryIP, errTooManyAttempts); err != nil {
		return err
	}
	tokenHash := linkHash(s.keys, strings.TrimSpace(body.Token))
	purposes := []string{purposeResetPassword, purposeInvite}
	var u authdb.User
	err := s.db.ReadTx(ctx, func(ctx context.Context) error {
		t, err := s.q(ctx).GetAuthToken(ctx, authdb.GetAuthTokenParams{TokenHash: tokenHash, Purposes: purposes})
		if db.IsNoRows(err) {
			return errTokenInvalid
		}
		if err != nil {
			return err
		}
		u, err = s.q(ctx).GetUserByID(ctx, t.UserID)
		return err
	})
	if err != nil {
		return err
	}
	if err := CheckPassword(body.Password, u.Email, u.FullName); err != nil {
		return err
	}
	hash, err := HashPassword(body.Password)
	if err != nil {
		return err
	}
	var sids []uuid.UUID
	err = s.db.Tx(ctx, func(ctx context.Context) error {
		q := s.q(ctx)
		t, err := q.GetAuthToken(ctx, authdb.GetAuthTokenParams{TokenHash: tokenHash, Purposes: purposes})
		if db.IsNoRows(err) {
			return errTokenInvalid
		}
		if err != nil {
			return err
		}
		if err := q.ConsumeUserTokens(ctx, authdb.ConsumeUserTokensParams{UserID: u.ID, Purposes: purposes}); err != nil {
			return err
		}
		if err := q.UpsertPasswordIdentity(ctx, authdb.UpsertPasswordIdentityParams{UserID: u.ID, Subject: u.ID.String(), PasswordHash: &hash}); err != nil {
			return err
		}
		if err := q.MarkEmailVerified(ctx, u.ID); err != nil {
			return err
		}
		if t.Purpose == purposeInvite {
			if err := s.users.Activate(ctx, u.ID); err != nil {
				return err
			}
		}
		reason := "password_reset"
		if sids, err = q.RevokeUserSessions(ctx, authdb.RevokeUserSessionsParams{Reason: &reason, UserID: u.ID}); err != nil {
			return err
		}
		return s.audit.Record(ctx, audit.Event{
			Action: "auth.password_reset", ActorID: u.ID, TargetType: "user", TargetID: u.ID.String(),
			Metadata: map[string]any{"via": t.Purpose},
		})
	})
	if err != nil {
		return err
	}
	return s.revocations.deny(ctx, sids...)
}

// ChangePassword replaces the password and ends the other sessions.
func (s *Service) ChangePassword(ctx context.Context, body apigen.PasswordChangeRequest) error {
	p, err := authz.Require(ctx)
	if err != nil {
		return err
	}
	if err := s.limiter.Allow(ctx, "password:user:"+p.UserID.String(), limitPasswordUser, errTooManyAttempts); err != nil {
		return err
	}
	u, err := s.q(ctx).GetUserByID(ctx, p.UserID)
	if err != nil {
		return fmt.Errorf("auth: user: %w", err)
	}
	current, err := s.q(ctx).GetPasswordHash(ctx, p.UserID)
	switch {
	case db.IsNoRows(err):
		// Accounts created by single sign-on or invitation may set a first password.
		if body.CurrentPassword != "" {
			return apperr.New(apperr.KindValidation, "auth.password_incorrect", "the current password is wrong")
		}
	case err != nil:
		return fmt.Errorf("auth: password: %w", err)
	default:
		if ok, err := VerifyPassword(current, body.CurrentPassword); err != nil || !ok {
			return apperr.New(apperr.KindValidation, "auth.password_incorrect", "the current password is wrong")
		}
	}
	if err := CheckPassword(body.NewPassword, u.Email, u.FullName); err != nil {
		return err
	}
	hash, err := HashPassword(body.NewPassword)
	if err != nil {
		return err
	}
	var sids []uuid.UUID
	err = s.db.Tx(ctx, func(ctx context.Context) error {
		q := s.q(ctx)
		if err := q.UpsertPasswordIdentity(ctx, authdb.UpsertPasswordIdentityParams{UserID: u.ID, Subject: u.ID.String(), PasswordHash: &hash}); err != nil {
			return err
		}
		reason := "password_changed"
		if sids, err = q.RevokeUserSessions(ctx, authdb.RevokeUserSessionsParams{Reason: &reason, UserID: u.ID, ExceptID: &p.SessionID}); err != nil {
			return err
		}
		return s.audit.Record(ctx, audit.Event{Action: "auth.password_changed", TargetType: "user", TargetID: u.ID.String()})
	})
	if err != nil {
		return fmt.Errorf("auth: change password: %w", err)
	}
	return s.revocations.deny(ctx, sids...)
}

// Invitee is an account to invite to the current university.
type Invitee struct {
	UserID   uuid.UUID
	Email    string
	FullName string
	Locale   string
}

// Invite e-mails a link to set a password (inside the transaction of ctx).
func (s *Service) Invite(ctx context.Context, in Invitee) error {
	if err := s.q(ctx).ConsumeUserTokens(ctx, authdb.ConsumeUserTokensParams{UserID: in.UserID, Purposes: []string{purposeInvite}}); err != nil {
		return fmt.Errorf("auth: invite: %w", err)
	}
	token, hash := newLinkToken(s.keys)
	if err := s.createToken(ctx, in.UserID, purposeInvite, inviteTTL, func(uuid.UUID) []byte { return hash }); err != nil {
		return err
	}
	return s.mailer.Send(ctx, in.Email, in.FullName, mail.TemplateInvite, in.Locale, map[string]string{
		"name": in.FullName, "tenant": s.tenantName(ctx, in.Locale), "link": link(ctx, "/reset-password", token) + "&invite=1",
	})
}

// sessionFor returns the session view of the signed-in user.
func (s *Service) sessionFor(ctx context.Context) (apigen.Session, error) {
	p, err := authz.Require(ctx)
	if err != nil {
		return apigen.Session{}, err
	}
	return s.users.Session(ctx, p.UserID)
}

package auth

import (
	"context"
	"fmt"
	"log/slog"
	"maps"
	"net/http"
	"time"

	"github.com/google/uuid"

	"github.com/yelnurq/startify/backend/internal/apigen"
	"github.com/yelnurq/startify/backend/internal/audit"
	"github.com/yelnurq/startify/backend/internal/auth/authdb"
	"github.com/yelnurq/startify/backend/internal/platform/apperr"
	"github.com/yelnurq/startify/backend/internal/platform/authz"
	"github.com/yelnurq/startify/backend/internal/platform/db"
	"github.com/yelnurq/startify/backend/internal/platform/tenancy"
	"github.com/yelnurq/startify/backend/internal/user"
)

var errDirectoryDisabled = apperr.NotFound("directory.disabled", "sign-in with the university directory is not enabled")

// External is an identity the university vouches for: an account of its
// single sign-on provider or of its directory. Both sign in through
// SignInExternal, so accounts are found, linked and created by the same rules
// (D-024, D-067).
type External struct {
	// Provider names the source in user_identities: "oidc:<provider id>" or
	// "ldap:<tenant id>"; Subject is the permanent identifier there.
	Provider string
	Subject  string
	Email    string
	FullName string
	// Vouched means the source vouches that the address belongs to the
	// person (email_verified of a provider; always for the university's own
	// directory). Only a vouched address links an existing account, and only
	// an account of this university (D-067).
	Vouched bool
	// Verified means the address counts as confirmed: a new account starts
	// with it verified and an existing one gets it verified.
	Verified bool
	// AutoProvision creates the account and the membership on a first sign-in.
	AutoProvision bool
	// Roles of a new membership; student when empty.
	Roles []string
	// ManagedRoles, when not nil, are synchronised on every sign-in: those of
	// them the member holds are replaced with Roles; other roles stay. A
	// member left without roles gets FallbackRole.
	ManagedRoles []string
	FallbackRole string
	// Method and Metadata describe the sign-in in the audit log.
	Method   string
	Metadata map[string]any
}

func (e External) newRoles() []string {
	if len(e.Roles) == 0 {
		return []string{string(authz.RoleStudent)}
	}
	return e.Roles
}

// SignInExternal signs in the account of an identity: it finds the account
// by the identity, else links one by e-mail or creates it, makes it a member
// of the university (or synchronises its managed roles), checks that it may
// enter and starts a session — all in one transaction, with the sign-in in
// the audit log. It returns the user and the session cookies.
func (s *Service) SignInExternal(ctx context.Context, ext External) (uuid.UUID, []*http.Cookie, error) {
	t, err := tenancy.Require(ctx)
	if err != nil {
		return uuid.Nil, nil, err
	}
	var (
		cookies      []*http.Cookie
		revoked      []uuid.UUID
		userID       uuid.UUID
		rolesChanged bool
	)
	err = s.db.Tx(ctx, func(ctx context.Context) error {
		q := s.q(ctx)
		u, err := q.GetIdentityUser(ctx, authdb.GetIdentityUserParams{Provider: ext.Provider, Subject: ext.Subject})
		if db.IsNoRows(err) {
			u, revoked, err = s.linkExternal(ctx, ext, t.Locale)
			if err != nil {
				return err
			}
			if err := q.CreateIdentity(ctx, authdb.CreateIdentityParams{UserID: u.ID, Provider: ext.Provider, Subject: ext.Subject}); err != nil {
				return err
			}
		} else if err != nil {
			return err
		}
		userID = u.ID
		if _, ok, err := s.users.Membership(ctx, u.ID); err != nil {
			return err
		} else if !ok {
			if !ext.AutoProvision {
				return errNotMember
			}
			// Members provisioned by the university's own identity source
			// are processed under the university's legal basis (D-023).
			if _, err := s.users.Join(ctx, u.ID, ext.newRoles(), user.StatusActive, nil); err != nil {
				return err
			}
		} else if ext.ManagedRoles != nil {
			if rolesChanged, err = s.users.SyncRoles(ctx, u.ID, ext.ManagedRoles, ext.Roles, ext.FallbackRole); err != nil {
				return err
			}
		}
		if err := s.users.EnsureProfile(ctx, u.ID, u.FullName); err != nil {
			return err
		}
		if err := s.admit(ctx, u); err != nil {
			return err
		}
		if ext.Verified {
			if err := q.MarkEmailVerified(ctx, u.ID); err != nil {
				return err
			}
		}
		if err := q.TouchIdentity(ctx, authdb.TouchIdentityParams{UserID: u.ID, Provider: ext.Provider}); err != nil {
			return err
		}
		meta := map[string]any{"method": ext.Method}
		maps.Copy(meta, ext.Metadata)
		if rolesChanged {
			meta["roles_synced"] = true
		}
		if err := s.audit.Record(ctx, audit.Event{
			Action: "auth.login", ActorID: u.ID, TargetType: "user", TargetID: u.ID.String(), Metadata: meta,
		}); err != nil {
			return err
		}
		cookies, err = s.startSession(ctx, u.ID, u.IsSuperAdmin)
		return err
	})
	if err != nil {
		return uuid.Nil, nil, err
	}
	if err := s.revocations.deny(ctx, revoked...); err != nil {
		return uuid.Nil, nil, err
	}
	if rolesChanged {
		// Once more after the commit, as for a change by an administrator: a
		// token refreshed meanwhile carries the new version with the old
		// roles. The new session refreshes once; the change is saved, so a
		// failure is logged, not returned.
		if err := s.revocations.RolesChanged(context.WithoutCancel(ctx), t.ID, userID); err != nil {
			slog.WarnContext(ctx, "role version not bumped after commit", slog.Any("error", err), slog.String("user", userID.String()))
		}
	}
	return userID, cookies, nil
}

// linkExternal finds the account of a first sign-in with an identity by
// e-mail, or creates it (inside the transaction of ctx). An existing account
// is linked only when the source vouches for the address, and only when the
// account is already a member of this university or its address is on one of
// the university's e-mail domains: an identity source speaks for its own
// university, not for accounts of others (audit С-5).
//
// Linking proves control of the address. An account whose e-mail was never
// verified may have been registered by a stranger, so its password goes and
// its sessions end in every university; the revoked sessions are returned
// for the deny-list.
func (s *Service) linkExternal(ctx context.Context, ext External, locale string) (authdb.User, []uuid.UUID, error) {
	q := s.q(ctx)
	u, err := q.GetUserByEmail(ctx, ext.Email)
	switch {
	case err == nil:
		if !ext.Vouched {
			return u, nil, errOIDC
		}
		if err := s.mayLinkSSO(ctx, u.ID, ext.Email); err != nil {
			return u, nil, err
		}
		if u.EmailVerifiedAt != nil {
			return u, nil, nil
		}
		if err := q.DeletePasswordIdentity(ctx, u.ID); err != nil {
			return u, nil, err
		}
		sids, err := s.revokeEverywhere(ctx, u.ID, "sso_linked", nil)
		if err != nil {
			return u, nil, err
		}
		if err := q.MarkEmailVerified(ctx, u.ID); err != nil {
			return u, nil, err
		}
		return u, sids, nil
	case !db.IsNoRows(err):
		return u, nil, err
	case !ext.AutoProvision:
		return u, nil, errNotMember
	}
	var verifiedAt *time.Time
	if ext.Verified {
		now := s.now()
		verifiedAt = &now
	}
	name := ext.FullName
	if len([]rune(name)) < 2 {
		name = ext.Email
	}
	u, err = q.CreateUser(ctx, authdb.CreateUserParams{Email: ext.Email, FullName: name, Locale: locale, EmailVerifiedAt: verifiedAt})
	return u, nil, err
}

// Outcomes and reasons of a planned external sign-in (the administrator's
// check of a directory user).
const (
	PlanExisting = "existing"
	PlanLink     = "link"
	PlanCreate   = "create"
	PlanDenied   = "denied"

	ReasonLinkDenied       = "link_denied"
	ReasonAutoProvisionOff = "auto_provision_off"
	ReasonSuspended        = "suspended"
	ReasonEmailUnverified  = "email_unverified"
	ReasonJoins            = "joins"
)

// Plan is what signing in with an identity would do, by the rules of
// SignInExternal.
type Plan struct {
	Outcome string
	// Reasons explain a refusal or note what would happen.
	Reasons []string
}

// Allowed reports whether the sign-in would succeed.
func (p Plan) Allowed() bool { return p.Outcome != PlanDenied }

// PlanExternal tells what SignInExternal would do with the identity, without
// changing anything.
func (s *Service) PlanExternal(ctx context.Context, ext External) (Plan, error) {
	if _, err := tenancy.Require(ctx); err != nil {
		return Plan{}, err
	}
	var plan Plan
	err := s.db.ReadTx(ctx, func(ctx context.Context) error {
		q := s.q(ctx)
		u, err := q.GetIdentityUser(ctx, authdb.GetIdentityUserParams{Provider: ext.Provider, Subject: ext.Subject})
		switch {
		case err == nil:
			plan.Outcome = PlanExisting
		case !db.IsNoRows(err):
			return err
		default:
			u, err = q.GetUserByEmail(ctx, ext.Email)
			switch {
			case db.IsNoRows(err):
				if !ext.AutoProvision {
					plan = Plan{Outcome: PlanDenied, Reasons: []string{ReasonAutoProvisionOff}}
					return nil
				}
				plan.Outcome = PlanCreate
				if !ext.Verified {
					plan.Reasons = append(plan.Reasons, ReasonEmailUnverified)
				}
				return nil
			case err != nil:
				return err
			}
			if !ext.Vouched {
				plan = Plan{Outcome: PlanDenied, Reasons: []string{ReasonLinkDenied}}
				return nil
			}
			if err := s.mayLinkSSO(ctx, u.ID, ext.Email); err != nil {
				if apperr.Is(err, errSSOLinkDenied.Code) {
					plan = Plan{Outcome: PlanDenied, Reasons: []string{ReasonLinkDenied}}
					return nil
				}
				return err
			}
			plan.Outcome = PlanLink
		}
		m, member, err := s.users.Membership(ctx, u.ID)
		if err != nil {
			return err
		}
		switch {
		case u.Status != "active" || (member && m.Status == user.StatusSuspended):
			plan = Plan{Outcome: PlanDenied, Reasons: []string{ReasonSuspended}}
		case !member && !ext.AutoProvision:
			plan = Plan{Outcome: PlanDenied, Reasons: []string{ReasonAutoProvisionOff}}
		case !member:
			plan.Reasons = append(plan.Reasons, ReasonJoins)
		}
		return nil
	})
	if err != nil {
		return Plan{}, fmt.Errorf("auth: plan sign-in: %w", err)
	}
	return plan, nil
}

// DirectoryAuthenticator checks a login and password against the directory
// of the current university (the directory module). It limits attempts,
// records failures and answers with the errors a person may see; a success
// is the identity to sign in.
type DirectoryAuthenticator interface {
	Authenticate(ctx context.Context, username, password string) (External, error)
}

// UseDirectory enables sign-in with the university directory.
func (s *Service) UseDirectory(d DirectoryAuthenticator) { s.directory = d }

// DirectoryLogin signs in with the login and password of the university
// directory.
func (s *Service) DirectoryLogin(ctx context.Context, body apigen.DirectoryLoginRequest) (apigen.Session, []*http.Cookie, error) {
	if _, err := tenancy.Require(ctx); err != nil {
		return apigen.Session{}, nil, err
	}
	if s.directory == nil {
		return apigen.Session{}, nil, errDirectoryDisabled
	}
	ext, err := s.directory.Authenticate(ctx, body.Username, body.Password)
	if err != nil {
		return apigen.Session{}, nil, err
	}
	userID, cookies, err := s.SignInExternal(ctx, ext)
	if err != nil {
		return apigen.Session{}, nil, err
	}
	session, err := s.users.Session(ctx, userID)
	return session, cookies, err
}

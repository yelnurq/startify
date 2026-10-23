// Package directory signs members in with the accounts of the university's
// directory (Active Directory or another LDAP server) and keeps the settings
// the university administrator edits: the connection, the service account
// (its password sealed), the search of users, groups and roles, and the
// labels of the sign-in page. Accounts, sessions and the rules of linking
// belong to the auth module (auth.SignInExternal).
package directory

import (
	"bytes"
	"context"
	"crypto/hmac"
	"encoding/base64"
	"encoding/binary"
	"encoding/json"
	"errors"
	"fmt"
	"log/slog"
	"slices"
	"strconv"
	"strings"
	"time"
	"unicode/utf8"

	"github.com/google/uuid"

	"github.com/yelnurq/startify/backend/internal/apigen"
	"github.com/yelnurq/startify/backend/internal/audit"
	"github.com/yelnurq/startify/backend/internal/auth"
	"github.com/yelnurq/startify/backend/internal/directory/directorydb"
	"github.com/yelnurq/startify/backend/internal/platform/apperr"
	"github.com/yelnurq/startify/backend/internal/platform/authz"
	"github.com/yelnurq/startify/backend/internal/platform/db"
	"github.com/yelnurq/startify/backend/internal/platform/httpx"
	"github.com/yelnurq/startify/backend/internal/platform/ratelimit"
	"github.com/yelnurq/startify/backend/internal/platform/secrets"
	"github.com/yelnurq/startify/backend/internal/platform/tenancy"
	"github.com/yelnurq/startify/backend/internal/platform/validate"
	"github.com/yelnurq/startify/backend/internal/tenant"
)

// Errors a person signing in may see. A wrong password, an unknown login and
// a login that matches several entries all read as auth.invalid_credentials,
// so the answer does not tell which logins exist.
var (
	errDisabled           = apperr.NotFound("directory.disabled", "sign-in with the university directory is not enabled")
	errUnavailable        = apperr.Unavailable("directory.unavailable", "the university directory does not answer; sign in with e-mail")
	errInvalidCredentials = apperr.Unauthorized("auth.invalid_credentials", "the login or password is wrong")
	errNotAllowed         = apperr.Forbidden("directory.not_allowed", "the directory account may not sign in to Qadam")
	errEmailMissing       = apperr.Forbidden("directory.email_missing", "the directory account has no e-mail address")
	errPasswordExpired    = apperr.Forbidden("directory.password_expired", "the password has expired; change it in the university's systems")
	errAccountDisabled    = apperr.Forbidden("directory.account_disabled", "the directory account is disabled")
	errTestRequired       = apperr.New(apperr.KindValidation, "directory.test_required",
		"check the connection with these settings before turning sign-in on")
	errStale = apperr.PreconditionFailed("precondition.failed", "the settings were changed since they were loaded")
)

const tooManyTests = "directory.too_many_tests"

// Limits. Attempts per login stay below the usual lockout threshold of
// Active Directory (10), so Qadam never locks a university account out by
// itself; attempts per address share the allowance of password sign-in.
var (
	limitLoginIP    = ratelimit.Limit{Rate: 600, Burst: 120, Period: time.Hour}
	limitLoginUser  = ratelimit.Limit{Rate: 5, Burst: 5, Period: 15 * time.Minute}
	limitTestTenant = ratelimit.PerMinute(30)
)

// proofTTL is how long a successful connection check allows turning sign-in on.
const proofTTL = time.Hour

// defaultFailFloor is the least time of a failed sign-in, well above the
// round trip of a bind: an unknown login answers as late as a wrong password.
const defaultFailFloor = 800 * time.Millisecond

// Service keeps the settings and checks logins against the directory.
type Service struct {
	db      *db.DB
	keys    *secrets.Keyring
	auth    *auth.Service
	tenants *tenant.Service
	audit   *audit.Logger
	limiter *ratelimit.Limiter
	rules   Rules
	now     func() time.Time
	// failFloor is the least time of a failed sign-in (see holdFailure).
	failFloor time.Duration
}

// Deps are the collaborators of the service.
type Deps struct {
	DB      *db.DB
	Keys    *secrets.Keyring
	Auth    *auth.Service
	Tenants *tenant.Service
	Audit   *audit.Logger
	Limiter *ratelimit.Limiter
	// Production refuses unencrypted connections.
	Production bool
}

// NewService returns the directory service and enables directory sign-in in
// the auth service.
func NewService(d Deps) *Service {
	s := &Service{
		db: d.DB, keys: d.Keys, auth: d.Auth, tenants: d.Tenants, audit: d.Audit, limiter: d.Limiter,
		rules: Rules{PlainAllowed: !d.Production}, now: time.Now, failFloor: defaultFailFloor,
	}
	d.Auth.UseDirectory(s)
	return s
}

// SetFailFloor changes the least time of a failed sign-in (tests).
func (s *Service) SetFailFloor(d time.Duration) { s.failFloor = d }

func (s *Service) q(ctx context.Context) *directorydb.Queries {
	return directorydb.New(s.db.Conn(ctx))
}

// stored are the saved settings of the university.
type stored struct {
	cfg        Config
	configured bool
	version    int32
	sealed     []byte
}

func (st stored) etag() string { return `"` + strconv.Itoa(int(st.version)) + `"` }

func storedOf(row directorydb.TenantDirectorySetting) stored {
	return stored{cfg: configOf(row), configured: true, version: row.Version, sealed: row.BindPasswordEnc}
}

// load reads the settings (inside the transaction of ctx, if any).
func (s *Service) load(ctx context.Context) (stored, error) {
	row, err := s.q(ctx).GetSettings(ctx)
	if db.IsNoRows(err) {
		return stored{cfg: Defaults()}, nil
	}
	if err != nil {
		return stored{}, fmt.Errorf("directory: settings: %w", err)
	}
	return storedOf(row), nil
}

func (s *Service) read(ctx context.Context) (stored, error) {
	var st stored
	err := s.db.ReadTx(ctx, func(ctx context.Context) error {
		var err error
		st, err = s.load(ctx)
		return err
	})
	return st, err
}

// password opens the stored bind password.
func (s *Service) password(st stored) (string, error) {
	if len(st.sealed) == 0 {
		return "", nil
	}
	raw, err := s.keys.Open(st.sealed)
	if err != nil {
		return "", fmt.Errorf("directory: bind password: %w", err)
	}
	return string(raw), nil
}

func presetsAPI() []apigen.DirectoryPresetDefaults {
	out := make([]apigen.DirectoryPresetDefaults, len(Presets))
	for i := range Presets {
		p := &Presets[i]
		out[i] = apigen.DirectoryPresetDefaults{
			Preset: apigen.DirectoryPreset(p.Name), UserFilter: p.UserFilter, LoginAttr: p.LoginAttr,
			EmailAttr: p.EmailAttr, EmailFallbackAttr: ptr(p.EmailFallbackAttr), NameAttr: p.NameAttr,
			GivenNameAttr: ptr(p.GivenNameAttr), SurnameAttr: ptr(p.SurnameAttr), IdAttr: p.IDAttr,
			GroupsAttr: p.GroupsAttr, NestedGroups: p.NestedGroups, BaseDnExample: p.BaseDNExample,
		}
	}
	return out
}

func (s *Service) settingsAPI(st stored) apigen.DirectorySettings {
	c := st.cfg
	mappings := make([]apigen.DirectoryRoleMapping, len(c.RoleMappings))
	for i, m := range c.RoleMappings {
		mappings[i] = apigen.DirectoryRoleMapping{GroupDn: m.GroupDN, Role: apigen.Role(m.Role)}
	}
	return apigen.DirectorySettings{
		Enabled: c.Enabled, Preset: apigen.DirectoryPreset(c.Preset), Url: strings.Join(c.URLs, " "),
		StartTls: c.StartTLS, CaCertPem: ptr(c.CACertPEM), TimeoutSeconds: int(c.Timeout / time.Second),
		BindDn: c.BindDN, UserBaseDn: c.UserBaseDN, UserFilter: c.UserFilter,
		LoginAttr: c.LoginAttr, EmailAttr: c.EmailAttr, EmailFallbackAttr: ptr(c.EmailFallbackAttr),
		NameAttr: c.NameAttr, GivenNameAttr: ptr(c.GivenNameAttr), SurnameAttr: ptr(c.SurnameAttr),
		IdAttr: c.IDAttr, GroupsAttr: c.GroupsAttr, NestedGroups: c.NestedGroups,
		RoleMappings: mappings, DefaultRole: apigen.Role(c.DefaultRole), RequiredGroupDn: ptr(c.RequiredGroupDN),
		AutoProvision: c.AutoProvision, SyncRoles: c.SyncRoles,
		LoginLabel: textAPI(c.LoginLabel), UsernameHint: textAPI(c.UsernameHint),
		HasBindPassword: len(st.sealed) > 0, Configured: st.configured, PlainLdapAllowed: s.rules.PlainAllowed,
		Presets: presetsAPI(),
	}
}

// Settings returns the settings of the university with their ETag.
func (s *Service) Settings(ctx context.Context) (apigen.DirectorySettings, string, error) {
	if _, err := authz.RequireRole(ctx, authz.RoleTenantAdmin); err != nil {
		return apigen.DirectorySettings{}, "", err
	}
	if _, err := tenancy.Require(ctx); err != nil {
		return apigen.DirectorySettings{}, "", err
	}
	st, err := s.read(ctx)
	if err != nil {
		return apigen.DirectorySettings{}, "", err
	}
	return s.settingsAPI(st), st.etag(), nil
}

// Save stores the settings. The bind password is kept unless a new one is
// given. Sign-in is turned on (or stays on after a change of the
// connection) only with the proof of a successful check of the same
// connection.
func (s *Service) Save(ctx context.Context, in apigen.DirectorySettingsUpdate, ifMatch *string) (apigen.DirectorySettings, string, error) {
	if _, err := authz.RequireRole(ctx, authz.RoleTenantAdmin); err != nil {
		return apigen.DirectorySettings{}, "", err
	}
	t, err := tenancy.Require(ctx)
	if err != nil {
		return apigen.DirectorySettings{}, "", err
	}
	next, err := parseInput(in, s.rules)
	if err != nil {
		return apigen.DirectorySettings{}, "", err
	}
	var sealed []byte
	if next.BindPassword != "" {
		if sealed, err = s.keys.Seal([]byte(next.BindPassword)); err != nil {
			return apigen.DirectorySettings{}, "", err
		}
	}
	var saved stored
	err = s.db.Tx(ctx, func(ctx context.Context) error {
		q := s.q(ctx)
		current := stored{cfg: Defaults()}
		row, err := q.LockSettings(ctx)
		switch {
		case err == nil:
			current = storedOf(row)
		case !db.IsNoRows(err):
			return err
		}
		if ifMatch != nil && strings.TrimPrefix(strings.TrimSpace(*ifMatch), "W/") != current.etag() {
			return errStale
		}
		passwordChanged := next.BindPassword != ""
		if !passwordChanged {
			// The stored password goes into the row as it is (the CHECK of
			// an enabled row sees the inserted values, not the merged ones).
			sealed = current.sealed
			if next.BindPassword, err = s.password(current); err != nil {
				return err
			}
		}
		if next.Enabled {
			if next.BindPassword == "" {
				return apperr.Validation(apperr.FieldError{Field: "bind_password", Code: validate.CodeRequired})
			}
			if current.cfg.BindPassword, err = s.password(current); err != nil {
				return err
			}
			reconnects := !current.cfg.Enabled || !bytes.Equal(current.cfg.connectionKey(), next.connectionKey())
			if reconnects && !s.validProof(t.ID, in.TestProof, next) {
				return errTestRequired
			}
		}
		mappings, err := json.Marshal(next.RoleMappings)
		if err != nil {
			return err
		}
		row, err = q.UpsertSettings(ctx, directorydb.UpsertSettingsParams{
			Enabled: next.Enabled, Preset: next.Preset, Url: strings.Join(next.URLs, " "), StartTls: next.StartTLS,
			CaCertPem: ptr(next.CACertPEM), TimeoutSeconds: int16(next.Timeout / time.Second), //nolint:gosec // 1..60
			BindDn: next.BindDN, BindPasswordEnc: sealed, UserBaseDn: next.UserBaseDN, UserFilter: next.UserFilter,
			LoginAttr: next.LoginAttr, EmailAttr: next.EmailAttr, EmailFallbackAttr: ptr(next.EmailFallbackAttr),
			NameAttr: next.NameAttr, GivenNameAttr: ptr(next.GivenNameAttr), SurnameAttr: ptr(next.SurnameAttr),
			IDAttr: next.IDAttr, GroupsAttr: next.GroupsAttr, NestedGroups: next.NestedGroups,
			RoleMappings: mappings, DefaultRole: next.DefaultRole, RequiredGroupDn: ptr(next.RequiredGroupDN),
			AutoProvision: next.AutoProvision, SyncRoles: next.SyncRoles,
			LoginLabel: next.LoginLabel.JSON(), UsernameHint: next.UsernameHint.JSON(),
		})
		if err != nil {
			return err
		}
		saved = storedOf(row)
		// The audit names what changed, never the password itself.
		return s.audit.Record(ctx, audit.Event{
			Action: "directory.settings_updated", TargetType: "tenant", TargetID: t.ID.String(),
			Metadata: map[string]any{
				"changed": changedFields(current, next, passwordChanged), "enabled": next.Enabled,
				"enabled_before": current.cfg.Enabled, "preset": next.Preset, "url": strings.Join(next.URLs, " "),
				"bind_dn": next.BindDN, "password_changed": passwordChanged,
			},
		})
	})
	if err != nil {
		if _, ok := apperr.As(err); ok {
			return apigen.DirectorySettings{}, "", err
		}
		return apigen.DirectorySettings{}, "", fmt.Errorf("directory: save: %w", err)
	}
	return s.settingsAPI(saved), saved.etag(), nil
}

// changedFields lists the settings an update changes, for the audit log.
func changedFields(before stored, after Config, passwordChanged bool) []string {
	b, a := before.cfg, after
	fields := []string{}
	add := func(name string, changed bool) {
		if changed {
			fields = append(fields, name)
		}
	}
	add("enabled", b.Enabled != a.Enabled)
	add("preset", b.Preset != a.Preset)
	add("url", !slices.Equal(b.URLs, a.URLs))
	add("start_tls", b.StartTLS != a.StartTLS)
	add("ca_cert_pem", b.CACertPEM != a.CACertPEM)
	add("timeout_seconds", b.Timeout != a.Timeout)
	add("bind_dn", b.BindDN != a.BindDN)
	add("bind_password", passwordChanged)
	add("user_base_dn", b.UserBaseDN != a.UserBaseDN)
	add("user_filter", b.UserFilter != a.UserFilter)
	add("attributes", b.LoginAttr != a.LoginAttr || b.EmailAttr != a.EmailAttr || b.EmailFallbackAttr != a.EmailFallbackAttr ||
		b.NameAttr != a.NameAttr || b.GivenNameAttr != a.GivenNameAttr || b.SurnameAttr != a.SurnameAttr ||
		b.IDAttr != a.IDAttr || b.GroupsAttr != a.GroupsAttr || b.NestedGroups != a.NestedGroups)
	add("role_mappings", !slices.Equal(b.RoleMappings, a.RoleMappings))
	add("default_role", b.DefaultRole != a.DefaultRole)
	add("required_group_dn", b.RequiredGroupDN != a.RequiredGroupDN)
	add("auto_provision", b.AutoProvision != a.AutoProvision)
	add("sync_roles", b.SyncRoles != a.SyncRoles)
	add("labels", !maps2Equal(b.LoginLabel, a.LoginLabel) || !maps2Equal(b.UsernameHint, a.UsernameHint))
	return fields
}

func maps2Equal(a, b map[string]string) bool {
	if len(a) != len(b) {
		return false
	}
	for k, v := range a {
		if b[k] != v {
			return false
		}
	}
	return true
}

// proof signs a successful check of a connection: the university, the time
// it expires and the connection itself (the password only as part of the
// keyed hash).
func (s *Service) proof(tenantID uuid.UUID, c Config, expires time.Time) string {
	var exp [8]byte
	binary.BigEndian.PutUint64(exp[:], uint64(expires.Unix())) //nolint:gosec // a time after 1970
	mac := s.keys.MAC(secrets.PurposeDirectoryTest, tenantID[:], exp[:], c.connectionKey())
	return base64.RawURLEncoding.EncodeToString(append(exp[:], mac...))
}

func (s *Service) validProof(tenantID uuid.UUID, proof *string, c Config) bool {
	if proof == nil {
		return false
	}
	raw, err := base64.RawURLEncoding.DecodeString(*proof)
	if err != nil || len(raw) < 9 {
		return false
	}
	expires := time.Unix(int64(binary.BigEndian.Uint64(raw[:8])), 0) //nolint:gosec // checked below
	if s.now().After(expires) {
		return false
	}
	want, err := base64.RawURLEncoding.DecodeString(s.proof(tenantID, c, expires))
	return err == nil && hmac.Equal(raw, want)
}

// draft reads settings of the form for a check, with the stored password
// unless a new one is given.
func (s *Service) draft(ctx context.Context, in apigen.DirectorySettingsUpdate) (Config, error) {
	c, err := parseInput(in, s.rules)
	if err != nil {
		return c, err
	}
	if c.BindPassword == "" {
		st, err := s.read(ctx)
		if err != nil {
			return c, err
		}
		if c.BindPassword, err = s.password(st); err != nil {
			return c, err
		}
	}
	return c, nil
}

func (s *Service) checkAllowed(ctx context.Context) (tenancy.Tenant, error) {
	if _, err := authz.RequireRole(ctx, authz.RoleTenantAdmin); err != nil {
		return tenancy.Tenant{}, err
	}
	t, err := tenancy.Require(ctx)
	if err != nil {
		return t, err
	}
	return t, s.limiter.Allow(ctx, "directory-test:tenant:"+t.ID.String(), limitTestTenant, tooManyTests)
}

// TestConnection checks the settings of the form step by step.
func (s *Service) TestConnection(ctx context.Context, in apigen.DirectorySettingsUpdate) (apigen.DirectoryConnectionTest, error) {
	t, err := s.checkAllowed(ctx)
	if err != nil {
		return apigen.DirectoryConnectionTest{}, err
	}
	c, err := s.draft(ctx, in)
	if err != nil {
		return apigen.DirectoryConnectionTest{}, err
	}
	rec := &recorder{}
	if sess, err := open(c, s.rules, rec); err == nil {
		if err := sess.bindService(c.BindPassword, rec); err == nil {
			if err := sess.checkBase(rec); err == nil {
				sess.describeServer(rec)
			}
		}
		sess.close()
	}
	rec.skipRest()
	out := apigen.DirectoryConnectionTest{Ok: !rec.failed(), Steps: rec.steps}
	if out.Ok {
		p := s.proof(t.ID, c, s.now().Add(proofTTL))
		out.Proof = &p
	}
	return out, nil
}

// skipRest marks the steps a failure left out.
func (r *recorder) skipRest() {
	for _, name := range []apigen.DirectoryTestStepStep{stepConnect, stepTLS, stepBind, stepSearch, stepServer} {
		if !slices.ContainsFunc(r.steps, func(s apigen.DirectoryTestStep) bool { return s.Step == name }) {
			r.steps = append(r.steps, step(name, statusSkipped, "", "", ""))
		}
	}
}

// TestUser looks a user up without a password and tells what signing in
// would do.
func (s *Service) TestUser(ctx context.Context, req apigen.DirectoryUserTestRequest) (apigen.DirectoryUserTest, error) {
	t, err := s.checkAllowed(ctx)
	if err != nil {
		return apigen.DirectoryUserTest{}, err
	}
	c, err := s.draft(ctx, req.Settings)
	if err != nil {
		return apigen.DirectoryUserTest{}, err
	}
	login := NormalizeLogin(c.Preset, req.Username)
	if login == "" || utf8.RuneCountInString(login) > 256 {
		return apigen.DirectoryUserTest{}, apperr.Validation(apperr.FieldError{Field: "username", Code: validate.CodeRequired})
	}
	out := apigen.DirectoryUserTest{Steps: []apigen.DirectoryTestStep{}}
	rec := &recorder{}
	sess, err := open(c, s.rules, rec)
	if err == nil {
		defer sess.close()
		err = sess.bindService(c.BindPassword, rec)
	}
	if err != nil {
		rec.skipRest()
		out.Code, out.Steps = ptr(lookupConnection), rec.steps
		return out, nil
	}
	e, err := sess.findUser(login)
	switch {
	case errors.Is(err, errNotFound):
		out.Code, out.Steps = ptr(lookupNotFound), rec.steps
		return out, nil
	case errors.Is(err, errAmbiguous):
		out.Code, out.Steps = ptr(lookupAmbiguous), rec.steps
		return out, nil
	case err != nil:
		var se *stepError
		code := codeUnknown
		if errors.As(err, &se) {
			code = se.code
		}
		rec.add(step(stepSearch, statusFailed, code, err.Error(), sess.url))
		rec.skipRest()
		out.Code, out.Steps = ptr(lookupConnection), rec.steps
		return out, nil
	}
	entry := mapEntry(c, e)
	entry.Groups = sess.groups(entry)
	roles, byDefault := c.RolesFor(entry.Groups)
	info := apigen.DirectoryUserInfo{
		Dn: entry.DN, Login: entry.Login, Email: ptr(entry.Email), FullName: entry.FullName, Subject: entry.Subject,
		Groups: entry.Groups, Roles: make([]apigen.Role, len(roles)),
		SignIn: apigen.DirectorySignInCheck{Reasons: []apigen.DirectorySignInCheckReasons{}},
	}
	for i, r := range roles {
		info.Roles[i] = apigen.Role(r)
	}
	check := &info.SignIn
	refuse := func(reason string) {
		check.Outcome = apigen.DirectorySignInCheckOutcomeDenied
		check.Reasons = append(check.Reasons, apigen.DirectorySignInCheckReasons(reason))
	}
	if c.RequiredGroupDN != "" && !InGroup(entry.Groups, c.RequiredGroupDN) {
		refuse("required_group")
	}
	if entry.Email == "" {
		refuse("email_missing")
	}
	if check.Outcome != apigen.DirectorySignInCheckOutcomeDenied {
		ext, err := s.external(ctx, t.ID, c, entry, roles, login)
		if err != nil {
			return out, err
		}
		plan, err := s.auth.PlanExternal(ctx, ext)
		if err != nil {
			return out, err
		}
		check.Outcome = apigen.DirectorySignInCheckOutcome(plan.Outcome)
		for _, r := range plan.Reasons {
			check.Reasons = append(check.Reasons, apigen.DirectorySignInCheckReasons(r))
		}
	}
	if byDefault {
		check.Reasons = append(check.Reasons, "default_role")
	}
	check.Allowed = check.Outcome != apigen.DirectorySignInCheckOutcomeDenied
	out.Found, out.User, out.Steps = true, &info, rec.steps
	return out, nil
}

// external is the identity of a directory user for the auth module.
func (s *Service) external(ctx context.Context, tenantID uuid.UUID, c Config, entry Entry, roles []string, login string) (auth.External, error) {
	settings, err := s.tenants.CurrentSettings(ctx)
	if err != nil {
		return auth.External{}, err
	}
	ext := auth.External{
		Provider: "ldap:" + tenantID.String(), Subject: entry.Subject, Email: entry.Email, FullName: entry.FullName,
		// The university's directory vouches for its people: linking follows
		// the rules of single sign-on (D-067). A new account starts with a
		// confirmed address only on the university's e-mail domains.
		Vouched: true, Verified: auth.DomainListed(entry.Email, settings.EmailDomains),
		AutoProvision: c.AutoProvision, Roles: roles, FallbackRole: c.DefaultRole,
		Method: "directory", Metadata: map[string]any{"login": login},
	}
	if c.SyncRoles {
		ext.ManagedRoles = c.ManagedRoles()
	}
	return ext, nil
}

// LoginOptions tells the sign-in page whether the directory is offered.
func (s *Service) LoginOptions(ctx context.Context) (apigen.DirectoryLogin, error) {
	st, err := s.read(ctx)
	if err != nil {
		return apigen.DirectoryLogin{}, err
	}
	if !st.configured || !st.cfg.Enabled {
		return apigen.DirectoryLogin{}, nil
	}
	return apigen.DirectoryLogin{Enabled: true, Label: textAPI(st.cfg.LoginLabel), UsernameHint: textAPI(st.cfg.UsernameHint)}, nil
}

// Authenticate implements auth.DirectoryAuthenticator: it checks the login
// and password against the directory and returns the identity to sign in.
func (s *Service) Authenticate(ctx context.Context, username, password string) (auth.External, error) {
	t, err := tenancy.Require(ctx)
	if err != nil {
		return auth.External{}, err
	}
	st, err := s.read(ctx)
	if err != nil {
		return auth.External{}, err
	}
	if !st.configured || !st.cfg.Enabled {
		return auth.External{}, errDisabled
	}
	c := st.cfg
	login := NormalizeLogin(c.Preset, username)
	var v validate.Errors
	if login == "" {
		v.Add("username", validate.CodeRequired, nil)
	} else if utf8.RuneCountInString(login) > 256 {
		v.Add("username", validate.CodeMaxLength, map[string]any{"count": 256})
	}
	// A blank password is refused before the directory sees it: many
	// servers, Active Directory among them, take a bind with an empty
	// password for an anonymous one and report success.
	if strings.TrimSpace(password) == "" {
		v.Add("password", validate.CodeRequired, nil)
	} else if len(password) > maxPasswordBytes {
		v.Add("password", validate.CodeMaxLength, map[string]any{"count": maxPasswordBytes})
	}
	if err := v.Err(); err != nil {
		return auth.External{}, err
	}
	if err := s.limiter.Allow(ctx, "login:ip:"+clientKey(ctx), limitLoginIP, "auth.too_many_attempts"); err != nil {
		return auth.External{}, err
	}
	userKey := "directory:user:" + t.ID.String() + ":" + strings.ToLower(login)
	if err := s.limiter.Allow(ctx, userKey, limitLoginUser, "auth.too_many_attempts"); err != nil {
		return auth.External{}, err
	}
	start := time.Now()
	ext, failure, dn := s.authenticate(ctx, t.ID, c, st, login, password)
	if failure != nil {
		s.holdFailure(ctx, start)
		s.recordFailure(ctx, login, dn, failure)
		return auth.External{}, failure.public
	}
	_ = s.limiter.Reset(ctx, userKey)
	return ext, nil
}

// failure is a refused sign-in: what the person sees and what the
// administrator reads in the audit log.
type failure struct {
	public error
	reason string
	detail string
}

func (s *Service) authenticate(ctx context.Context, tenantID uuid.UUID, c Config, st stored, login, password string) (auth.External, *failure, string) {
	fail := func(public error, reason string, err error) *failure {
		f := &failure{public: public, reason: reason}
		if err != nil {
			f.detail = err.Error()
		}
		return f
	}
	bindPassword, err := s.password(st)
	if err != nil {
		return auth.External{}, fail(errUnavailable, "bind_password", err), ""
	}
	sess, err := open(c, s.rules, nil)
	if err != nil {
		return auth.External{}, fail(errUnavailable, reasonOf(err), err), ""
	}
	defer sess.close()
	if err := sess.bindService(bindPassword, nil); err != nil {
		return auth.External{}, fail(errUnavailable, "service_"+reasonOf(err), err), ""
	}
	e, err := sess.findUser(login)
	switch {
	case errors.Is(err, errNotFound):
		return auth.External{}, fail(errInvalidCredentials, lookupNotFound, nil), ""
	case errors.Is(err, errAmbiguous):
		slog.WarnContext(ctx, "directory login matches several entries", slog.String("login", login))
		return auth.External{}, fail(errInvalidCredentials, lookupAmbiguous, nil), ""
	case err != nil:
		return auth.External{}, fail(errUnavailable, "search_"+reasonOf(err), err), ""
	}
	entry := mapEntry(c, e)
	if err := sess.conn.Bind(entry.DN, password); err != nil {
		switch bindRefusal(err) {
		case bindExpired:
			return auth.External{}, fail(errPasswordExpired, "password_expired", nil), entry.DN
		case bindDisabled:
			return auth.External{}, fail(errAccountDisabled, "account_disabled", nil), entry.DN
		case bindDenied:
			return auth.External{}, fail(errNotAllowed, "logon_restricted", nil), entry.DN
		case bindWrong:
			return auth.External{}, fail(errInvalidCredentials, codeInvalidCredentials, nil), entry.DN
		}
		return auth.External{}, fail(errUnavailable, "bind_"+classifyResult(err), err), entry.DN
	}
	if c.NestedGroups {
		// Back to the service account: a user may not read the groups.
		if err := sess.bindService(bindPassword, nil); err != nil {
			return auth.External{}, fail(errUnavailable, "service_"+reasonOf(err), err), entry.DN
		}
	}
	entry.Groups = sess.groups(entry)
	if c.RequiredGroupDN != "" && !InGroup(entry.Groups, c.RequiredGroupDN) {
		return auth.External{}, fail(errNotAllowed, "required_group", nil), entry.DN
	}
	if entry.Email == "" {
		return auth.External{}, fail(errEmailMissing, "email_missing", nil), entry.DN
	}
	roles, _ := c.RolesFor(entry.Groups)
	ext, err := s.external(ctx, tenantID, c, entry, roles, login)
	if err != nil {
		return auth.External{}, fail(err, "internal", err), entry.DN
	}
	ext.Metadata["dn"] = entry.DN
	return ext, nil, entry.DN
}

func reasonOf(err error) string {
	var se *stepError
	if errors.As(err, &se) {
		return se.code
	}
	return codeUnknown
}

// holdFailure delays a refusal until failFloor has passed since start, so the
// time of the answer does not tell an unknown login from a wrong password.
func (s *Service) holdFailure(ctx context.Context, start time.Time) {
	wait := s.failFloor - time.Since(start)
	if wait <= 0 {
		return
	}
	t := time.NewTimer(wait)
	defer t.Stop()
	select {
	case <-ctx.Done():
	case <-t.C:
	}
}

// recordFailure writes a refused sign-in to the audit log: the login, the DN
// when the entry was found and the reason; never the password.
func (s *Service) recordFailure(ctx context.Context, login, dn string, f *failure) {
	if apperr.Is(f.public, errUnavailable.Code) {
		slog.WarnContext(ctx, "directory unavailable", slog.String("reason", f.reason), slog.String("error", f.detail))
	}
	meta := map[string]any{"method": "directory", "reason": f.reason}
	if dn != "" {
		meta["dn"] = dn
	}
	target := login
	if utf8.RuneCountInString(target) > 128 {
		target = string([]rune(target)[:128])
	}
	err := s.db.Tx(ctx, func(ctx context.Context) error {
		return s.audit.Record(ctx, audit.Event{Action: "auth.login_failed", TargetType: "directory_user", TargetID: target, Metadata: meta})
	})
	if err != nil {
		slog.WarnContext(ctx, "failed sign-in not recorded", slog.Any("error", err))
	}
}

func clientKey(ctx context.Context) string {
	if ip := httpx.ClientIPFrom(ctx); ip.IsValid() {
		return ip.String()
	}
	return "unknown"
}

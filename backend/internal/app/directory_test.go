package app

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"slices"
	"strings"
	"testing"

	"github.com/go-ldap/ldap/v3"
	"github.com/google/uuid"
	"github.com/redis/go-redis/v9"

	"github.com/yelnurq/startify/backend/internal/apigen"
	"github.com/yelnurq/startify/backend/internal/audit"
	"github.com/yelnurq/startify/backend/internal/directory"
	"github.com/yelnurq/startify/backend/internal/directory/directorydb"
	"github.com/yelnurq/startify/backend/internal/directory/ldaptest"
	"github.com/yelnurq/startify/backend/internal/platform/apperr"
	"github.com/yelnurq/startify/backend/internal/platform/authz"
	"github.com/yelnurq/startify/backend/internal/platform/db"
	"github.com/yelnurq/startify/backend/internal/platform/ratelimit"
	"github.com/yelnurq/startify/backend/internal/platform/tenancy"
	"github.com/yelnurq/startify/backend/internal/seed"
)

// Logins and passwords of the demo directory (deploy/ldap).
const (
	ldapStudent         = "sarsenova.a"
	ldapStudentPassword = "Aigerim-Ldap-2026"
	ldapStudentEmail    = "aigerim@qadam.test"
	ldapNewcomer        = "akhmetov.n"
	ldapNewcomerPass    = "Nursultan-Ldap-2026"
	ldapManager         = "abenova.d"
	ldapManagerPass     = "Dinara-Ldap-2026"
	ldapAdmin           = "seitkaziev.e"
	ldapAdminPass       = "Erlan-Ldap-2026"
)

// directoryHarness is KazUTB with the demo directory configured (off) and an
// administrator signed in.
type directoryHarness struct {
	h     *harness
	ts    tenants
	srv   *ldaptest.Server
	admin *browser
}

func newDirectoryHarness(t *testing.T) *directoryHarness {
	t.Helper()
	srv := ldaptest.Shared(t)
	h := newHarness(t)
	ts := seedTenants(t, h)
	h.services.Directory.SetFailFloor(0)
	if err := seed.Directory(context.Background(), h.db.App, h.keys, srv.URL()); err != nil {
		t.Fatal(err)
	}
	return &directoryHarness{h: h, ts: ts, srv: srv, admin: seedAdmin(t, h, ts)}
}

func (d *directoryHarness) settings(t *testing.T) (apigen.DirectorySettings, string) {
	t.Helper()
	rec := d.admin.do(http.MethodGet, "/api/v1/admin/directory", nil)
	expect(t, rec, http.StatusOK)
	d.h.validate(t, http.MethodGet, "/api/v1/admin/directory", rec)
	return decode[apigen.DirectorySettings](t, rec), rec.Header().Get("ETag")
}

// updateOf turns saved settings into the body of an update.
func updateOf(t *testing.T, s apigen.DirectorySettings) apigen.DirectorySettingsUpdate {
	t.Helper()
	raw, err := json.Marshal(s)
	if err != nil {
		t.Fatal(err)
	}
	var in apigen.DirectorySettingsUpdate
	if err := json.Unmarshal(raw, &in); err != nil {
		t.Fatal(err)
	}
	return in
}

func (d *directoryHarness) check(t *testing.T, in apigen.DirectorySettingsUpdate) apigen.DirectoryConnectionTest {
	t.Helper()
	rec := d.admin.do(http.MethodPost, "/api/v1/admin/directory/test", in)
	expect(t, rec, http.StatusOK)
	d.h.validate(t, http.MethodPost, "/api/v1/admin/directory/test", rec)
	return decode[apigen.DirectoryConnectionTest](t, rec)
}

// save checks the connection when sign-in is to be on and saves the form.
func (d *directoryHarness) save(t *testing.T, change func(*apigen.DirectorySettingsUpdate)) apigen.DirectorySettings {
	t.Helper()
	current, etag := d.settings(t)
	in := updateOf(t, current)
	change(&in)
	if in.Enabled {
		res := d.check(t, in)
		if !res.Ok || res.Proof == nil {
			t.Fatalf("connection check: %+v", res.Steps)
		}
		in.TestProof = res.Proof
	}
	rec := d.admin.do(http.MethodPut, "/api/v1/admin/directory", in, ifMatchHeader(etag))
	expect(t, rec, http.StatusOK)
	d.h.validate(t, http.MethodPut, "/api/v1/admin/directory", rec)
	return decode[apigen.DirectorySettings](t, rec)
}

func (d *directoryHarness) enable(t *testing.T, change func(*apigen.DirectorySettingsUpdate)) {
	t.Helper()
	d.save(t, func(in *apigen.DirectorySettingsUpdate) {
		in.Enabled = true
		if change != nil {
			change(in)
		}
	})
}

func directoryLogin(b *browser, username, pass string) *httptest.ResponseRecorder {
	return b.do(http.MethodPost, "/api/v1/auth/directory/login", apigen.DirectoryLoginRequest{Username: username, Password: pass})
}

func (d *directoryHarness) signIn(t *testing.T, username, pass string) (*browser, apigen.Session) {
	t.Helper()
	b := d.h.browser(t, kazutbHost)
	rec := directoryLogin(b, username, pass)
	expect(t, rec, http.StatusOK)
	d.h.validate(t, http.MethodPost, "/api/v1/auth/directory/login", rec)
	return b, decode[apigen.Session](t, rec)
}

// auditMetadata returns the metadata of the events of an action, newest first.
func auditMetadata(t *testing.T, h *harness, action string) []string {
	t.Helper()
	return scalar[[]string](t, h, `SELECT coalesce(array_agg(metadata::text ORDER BY created_at DESC), '{}') FROM audit_log WHERE action = $1`, action)
}

func TestDirectorySignIn(t *testing.T) {
	d := newDirectoryHarness(t)
	h := d.h
	existing, err := seed.Account(context.Background(), h.db.App, d.ts.kazutb, seed.Member{
		Email: ldapStudentEmail, FullName: "Айгерім Сәрсенова", Locale: "kk", Password: password, Roles: []string{"student"}, Slug: "aigerim",
	})
	if err != nil {
		t.Fatal(err)
	}

	// Off until the administrator turns it on.
	tenant := decode[apigen.Tenant](t, h.browser(t, kazutbHost).do(http.MethodGet, "/api/v1/tenant", nil))
	if tenant.DirectoryLogin.Enabled {
		t.Fatal("directory sign-in is off after the seed")
	}
	expectProblem(t, directoryLogin(h.browser(t, kazutbHost), ldapStudent, ldapStudentPassword), http.StatusNotFound, "directory.disabled")

	s, _ := d.settings(t)
	if !s.Configured || !s.HasBindPassword || s.Enabled || !s.PlainLdapAllowed || len(s.Presets) != 4 {
		t.Fatalf("seeded settings: %+v", s)
	}
	d.enable(t, nil)
	tenant = decode[apigen.Tenant](t, h.browser(t, kazutbHost).do(http.MethodGet, "/api/v1/tenant", nil))
	if !tenant.DirectoryLogin.Enabled || tenant.DirectoryLogin.Label == nil || *tenant.DirectoryLogin.Label.Ru != "Учётная запись КазУТБ" {
		t.Fatalf("tenant: %+v", tenant.DirectoryLogin)
	}

	// The existing member signs in with the directory password and keeps
	// the account.
	b, session := d.signIn(t, ldapStudent, ldapStudentPassword)
	if session.User.Id != existing || string(session.User.Email) != ldapStudentEmail || session.Tenant.Slug != "kazutb" {
		t.Fatalf("session: %+v", session)
	}
	expect(t, b.do(http.MethodGet, "/api/v1/me", nil), http.StatusOK)
	if _, again := d.signIn(t, "  "+ldapStudent+" ", ldapStudentPassword); again.User.Id != existing {
		t.Fatal("the identity finds the same account")
	}

	// A wrong password, an unknown login and an injected filter look the same.
	for _, c := range []struct{ login, pass, reason string }{
		{ldapStudent, "Wrong-Password-42", "invalid_credentials"},
		{"nobody.here", ldapStudentPassword, "not_found"},
		{"*)(uid=*", ldapStudentPassword, "not_found"},
		{"*", "Some-Password-7", "not_found"},
	} {
		expectProblem(t, directoryLogin(h.browser(t, kazutbHost), c.login, c.pass), http.StatusUnauthorized, "auth.invalid_credentials")
		if meta := auditMetadata(t, h, "auth.login_failed"); !strings.Contains(meta[0], `"reason": "`+c.reason+`"`) || strings.Contains(meta[0], c.pass) {
			t.Fatalf("%s: audit %s", c.login, meta[0])
		}
	}
	// An empty or blank password never reaches the directory.
	for _, blank := range []string{"", "   ", "\t"} {
		p := expectProblem(t, directoryLogin(h.browser(t, kazutbHost), ldapStudent, blank), http.StatusUnprocessableEntity, "validation.failed")
		if len(p.Errors) != 1 || p.Errors[0].Field != "password" {
			t.Fatalf("blank password: %+v", p)
		}
	}
	expectProblem(t, directoryLogin(h.browser(t, kazutbHost), "  ", ldapStudentPassword), http.StatusUnprocessableEntity, "validation.failed")

	// A newcomer gets an account and the role of the groups.
	_, newcomer := d.signIn(t, ldapNewcomer, ldapNewcomerPass)
	if string(newcomer.User.Email) != "nursultan.akhmetov@qadam.test" || newcomer.User.FullName != "Нұрсұлтан Ахметов" ||
		newcomer.User.EmailVerified || !slices.Equal(newcomer.Membership.Roles, []apigen.Role{apigen.RoleStudent}) {
		t.Fatalf("newcomer: %+v %+v", newcomer.User, newcomer.Membership)
	}
	// Nested groups: innovation is inside staff.
	_, manager := d.signIn(t, ldapManager, ldapManagerPass)
	if !slices.Equal(manager.Membership.Roles, []apigen.Role{apigen.RoleSupervisor, apigen.RoleInnovationManager}) {
		t.Fatalf("manager roles: %v", manager.Membership.Roles)
	}
	if meta := auditMetadata(t, h, "auth.login"); !strings.Contains(meta[0], `"method": "directory"`) || !strings.Contains(meta[0], ldapManager) {
		t.Fatalf("audit: %s", meta[0])
	}
}

func TestDirectoryLinkingRules(t *testing.T) {
	d := newDirectoryHarness(t)
	h := d.h
	d.enable(t, nil)

	// The directory of one university cannot take over an account of
	// another: not a member here, and the address is not on its domains.
	register(t, h.browser(t, sandboxHost), "nurlan@qadam.test", "Басқа Нұрлан")
	expectProblem(t, directoryLogin(h.browser(t, kazutbHost), "zhumabekov.n", "Nurlan-Ldap-2026"), http.StatusForbidden, "auth.sso_link_denied")
	if s := login(t, h.browser(t, sandboxHost), "nurlan@qadam.test", password); s.User.FullName != "Басқа Нұрлан" {
		t.Fatalf("foreign account changed: %+v", s.User)
	}

	// An unverified account of this university is taken over: its password
	// goes and its sessions end everywhere.
	squatter := h.browser(t, kazutbHost)
	register(t, squatter, "gulnar@qadam.test", "Бөтен Адам")
	h.db.Exec(t, "UPDATE users SET email_verified_at = NULL WHERE email = 'gulnar@qadam.test'")
	_, linked := d.signIn(t, "kassymova.g", "Gulnar-Ldap-2026")
	if !linked.User.EmailVerified || string(linked.User.Email) != "gulnar@qadam.test" {
		t.Fatalf("linked: %+v", linked)
	}
	expectProblem(t, squatter.do(http.MethodPost, "/api/v1/auth/refresh", nil), http.StatusUnauthorized, "auth.session_revoked")
	expectProblem(t, h.browser(t, kazutbHost).do(http.MethodPost, "/api/v1/auth/login",
		apigen.LoginRequest{Email: openapiEmail("gulnar@qadam.test"), Password: password}), http.StatusUnauthorized, "auth.invalid_credentials")

	// Without auto-provisioning newcomers are refused.
	d.save(t, func(in *apigen.DirectorySettingsUpdate) { in.AutoProvision = false })
	expectProblem(t, directoryLogin(h.browser(t, kazutbHost), ldapNewcomer, ldapNewcomerPass), http.StatusForbidden, "auth.not_a_member")

	// Only members of the required group, nested ones too.
	d.save(t, func(in *apigen.DirectorySettingsUpdate) {
		in.AutoProvision = true
		in.RequiredGroupDn = ptr("cn=staff,ou=Groups,dc=kazutb,dc=local")
	})
	expectProblem(t, directoryLogin(h.browser(t, kazutbHost), ldapNewcomer, ldapNewcomerPass), http.StatusForbidden, "directory.not_allowed")
	d.signIn(t, ldapManager, ldapManagerPass)

	// No e-mail in the directory: refused with a reason.
	admin := d.srv.Admin(t)
	add := ldap.NewAddRequest("uid=no.mail,ou=Staff,dc=kazutb,dc=local", nil)
	add.Attribute("objectClass", []string{"inetOrgPerson"})
	add.Attribute("uid", []string{"no.mail"})
	add.Attribute("cn", []string{"Пошта Жоқ"})
	add.Attribute("sn", []string{"Жоқ"})
	add.Attribute("userPassword", []string{"No-Mail-Ldap-2026"})
	if err := admin.Add(add); err != nil && !ldap.IsErrorWithCode(err, ldap.LDAPResultEntryAlreadyExists) {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = admin.Del(ldap.NewDelRequest("uid=no.mail,ou=Staff,dc=kazutb,dc=local", nil)) })
	d.save(t, func(in *apigen.DirectorySettingsUpdate) { in.RequiredGroupDn = nil })
	expectProblem(t, directoryLogin(h.browser(t, kazutbHost), "no.mail", "No-Mail-Ldap-2026"), http.StatusForbidden, "directory.email_missing")

	// A member blocked by the administrator stays blocked.
	_, student := d.signIn(t, ldapNewcomer, ldapNewcomerPass)
	expect(t, d.admin.do(http.MethodPatch, "/api/v1/admin/members/"+student.User.Id.String(),
		apigen.MemberUpdate{Status: ptr(apigen.MemberUpdateStatusSuspended)}), http.StatusOK)
	expectProblem(t, directoryLogin(h.browser(t, kazutbHost), ldapNewcomer, ldapNewcomerPass), http.StatusForbidden, "auth.account_suspended")
}

func TestDirectoryRoleSync(t *testing.T) {
	d := newDirectoryHarness(t)
	h := d.h
	_, id := account(t, h, d.ts, ldapStudentEmail, "Айгерім Сәрсенова", "student", "supervisor", "mentor")
	_, erlanID := account(t, h, d.ts, "admin@qadam.test", "Ерлан Сейтқазиев", "tenant_admin")
	d.enable(t, func(in *apigen.DirectorySettingsUpdate) { in.SyncRoles = true })

	// The roles of the mappings follow the directory; mentor is not managed.
	old := h.browser(t, kazutbHost)
	login(t, old, ldapStudentEmail, password)
	_, s := d.signIn(t, ldapStudent, ldapStudentPassword)
	if s.User.Id != id || !slices.Equal(s.Membership.Roles, []apigen.Role{apigen.RoleStudent, apigen.RoleMentor}) {
		t.Fatalf("synced roles: %v", s.Membership.Roles)
	}
	// Tokens with the old roles stop working at once.
	expectProblem(t, old.do(http.MethodGet, "/api/v1/me", nil), http.StatusUnauthorized, "auth.unauthenticated")
	expect(t, old.do(http.MethodPost, "/api/v1/auth/refresh", nil), http.StatusNoContent)
	if me := decode[apigen.Me](t, old.do(http.MethodGet, "/api/v1/me", nil)); slices.Contains(me.Membership.Roles, apigen.RoleSupervisor) {
		t.Fatalf("refreshed roles: %v", me.Membership.Roles)
	}
	if meta := auditMetadata(t, h, "member.roles_synced"); len(meta) != 1 || !strings.Contains(meta[0], `"source": "directory"`) {
		t.Fatalf("audit: %v", meta)
	}
	// Signing in again changes nothing.
	d.signIn(t, ldapStudent, ldapStudentPassword)
	if meta := auditMetadata(t, h, "member.roles_synced"); len(meta) != 1 {
		t.Fatalf("unchanged roles synced again: %v", meta)
	}

	// The last administrator keeps tenant_admin even when the directory no
	// longer gives it (the group of administrators is not mapped here).
	d.save(t, func(in *apigen.DirectorySettingsUpdate) {
		in.Enabled, in.SyncRoles = true, true
		in.RoleMappings = []apigen.DirectoryRoleMapping{{GroupDn: "cn=students,ou=Groups,dc=kazutb,dc=local", Role: apigen.RoleTenantAdmin}}
	})
	h.db.Exec(t, "UPDATE memberships SET roles = '{student}' WHERE user_id = (SELECT id FROM users WHERE email = 'rector-office@qadam.test')")
	_, admin := d.signIn(t, ldapAdmin, ldapAdminPass)
	if admin.User.Id != erlanID || !slices.Contains(admin.Membership.Roles, apigen.RoleTenantAdmin) {
		t.Fatalf("last administrator lost the role: %v", admin.Membership.Roles)
	}
	// With another administrator the directory decides.
	h.db.Exec(t, "UPDATE memberships SET roles = '{tenant_admin}' WHERE user_id = (SELECT id FROM users WHERE email = 'rector-office@qadam.test')")
	if _, again := d.signIn(t, ldapAdmin, ldapAdminPass); slices.Contains(again.Membership.Roles, apigen.RoleTenantAdmin) {
		t.Fatalf("roles: %v", again.Membership.Roles)
	}
}

func TestDirectoryChecks(t *testing.T) {
	d := newDirectoryHarness(t)
	h := d.h
	s, _ := d.settings(t)
	in := updateOf(t, s)

	res := d.check(t, in)
	steps := []string{}
	for _, st := range res.Steps {
		steps = append(steps, string(st.Step)+":"+string(st.Status))
	}
	if !res.Ok || res.Proof == nil || strings.Join(steps, " ") != "connect:ok tls:warning bind:ok search:ok server:ok" {
		t.Fatalf("check: %v %+v", steps, res)
	}

	// The form's password is used without being saved; a wrong one fails
	// the bind step and gives no proof.
	wrong := in
	wrong.BindPassword = ptr("Wrong-Bind-Password")
	if res := d.check(t, wrong); res.Ok || res.Proof != nil || res.Steps[2].Code == nil || *res.Steps[2].Code != "invalid_credentials" {
		t.Fatalf("wrong bind: %+v", res.Steps)
	}
	unreachable := in
	unreachable.Url = "ldap://127.0.0.1:1"
	if res := d.check(t, unreachable); res.Ok || *res.Steps[0].Code != "refused" || res.Steps[1].Status != apigen.DirectoryTestStepStatusSkipped {
		t.Fatalf("unreachable: %+v", res.Steps)
	}

	// A user check without a password.
	rec := d.admin.do(http.MethodPost, "/api/v1/admin/directory/test-user", apigen.DirectoryUserTestRequest{Settings: in, Username: ldapManager})
	expect(t, rec, http.StatusOK)
	h.validate(t, http.MethodPost, "/api/v1/admin/directory/test-user", rec)
	user := decode[apigen.DirectoryUserTest](t, rec)
	if !user.Found || user.User == nil || user.User.Dn != "uid=abenova.d,ou=Staff,dc=kazutb,dc=local" ||
		*user.User.Email != "innovation@qadam.test" || len(user.User.Groups) != 2 ||
		!slices.Equal(user.User.Roles, []apigen.Role{apigen.RoleSupervisor, apigen.RoleInnovationManager}) ||
		!user.User.SignIn.Allowed || user.User.SignIn.Outcome != apigen.DirectorySignInCheckOutcomeCreate ||
		!slices.Contains(user.User.SignIn.Reasons, "email_unverified") {
		t.Fatalf("user: %+v %+v", user, user.User)
	}
	for login, code := range map[string]string{"nobody.here": "not_found", "*": "not_found"} {
		res := decode[apigen.DirectoryUserTest](t, d.admin.do(http.MethodPost, "/api/v1/admin/directory/test-user",
			apigen.DirectoryUserTestRequest{Settings: in, Username: login}))
		if res.Found || res.Code == nil || *res.Code != code {
			t.Fatalf("%s: %+v", login, res)
		}
	}
	register(t, h.browser(t, sandboxHost), "aigerim@qadam.test", "Басқа Айгерім")
	res2 := decode[apigen.DirectoryUserTest](t, d.admin.do(http.MethodPost, "/api/v1/admin/directory/test-user",
		apigen.DirectoryUserTestRequest{Settings: in, Username: ldapStudent}))
	if res2.User == nil || res2.User.SignIn.Allowed || !slices.Contains(res2.User.SignIn.Reasons, "link_denied") {
		t.Fatalf("foreign account: %+v", res2.User)
	}

	// Only administrators.
	student, _ := account(t, h, d.ts, "student-check@qadam.test", "Студент Тексеру", "student")
	expectProblem(t, student.do(http.MethodGet, "/api/v1/admin/directory", nil), http.StatusForbidden, "rbac.forbidden")
	expectProblem(t, student.do(http.MethodPost, "/api/v1/admin/directory/test", in), http.StatusForbidden, "rbac.forbidden")
	expectProblem(t, student.do(http.MethodPut, "/api/v1/admin/directory", in), http.StatusForbidden, "rbac.forbidden")
}

func TestDirectorySettingsAreProtected(t *testing.T) {
	d := newDirectoryHarness(t)
	h := d.h
	s, etag := d.settings(t)

	// The bind password is write-only.
	rec := d.admin.do(http.MethodGet, "/api/v1/admin/directory", nil)
	if strings.Contains(rec.Body.String(), seed.DemoDirectoryPassword) || strings.Contains(rec.Body.String(), "\"bind_password\"") {
		t.Fatalf("password returned: %s", rec.Body)
	}
	in := updateOf(t, s)

	// Turning sign-in on needs a successful check of the same connection.
	in.Enabled = true
	expectProblem(t, d.admin.do(http.MethodPut, "/api/v1/admin/directory", in), http.StatusUnprocessableEntity, "directory.test_required")
	other := in
	other.Url = "ldap://127.0.0.1:1"
	proof := d.check(t, in).Proof
	other.TestProof = proof
	expectProblem(t, d.admin.do(http.MethodPut, "/api/v1/admin/directory", other), http.StatusUnprocessableEntity, "directory.test_required")
	in.TestProof = proof
	rec = d.admin.do(http.MethodPut, "/api/v1/admin/directory", in, ifMatchHeader(etag))
	expect(t, rec, http.StatusOK)
	saved := decode[apigen.DirectorySettings](t, rec)
	if !saved.Enabled || !saved.HasBindPassword || rec.Header().Get("ETag") == etag {
		t.Fatalf("saved: %+v", saved)
	}
	// A stale version is refused.
	expectProblem(t, d.admin.do(http.MethodPut, "/api/v1/admin/directory", in, ifMatchHeader(etag)), http.StatusPreconditionFailed, "precondition.failed")

	// An empty password keeps the stored one; labels change without a new check.
	in = updateOf(t, saved)
	in.BindPassword = ptr("")
	in.LoginLabel = &apigen.LocalizedText{Ru: ptr("Вход КазУТБ")}
	expect(t, d.admin.do(http.MethodPut, "/api/v1/admin/directory", in), http.StatusOK)
	d.signIn(t, ldapNewcomer, ldapNewcomerPass)
	// A new password changes the connection: it needs a check, and a wrong one fails it.
	in.BindPassword = ptr("Another-Bind-Password")
	expectProblem(t, d.admin.do(http.MethodPut, "/api/v1/admin/directory", in), http.StatusUnprocessableEntity, "directory.test_required")
	if res := d.check(t, in); res.Ok {
		t.Fatal("a wrong bind password passed the check")
	}
	for _, meta := range auditMetadata(t, h, "directory.settings_updated") {
		if strings.Contains(meta, seed.DemoDirectoryPassword) || strings.Contains(meta, "Another-Bind-Password") {
			t.Fatalf("password in the audit log: %s", meta)
		}
	}
	if enc := scalar[[]byte](t, h, "SELECT bind_password_enc FROM tenant_directory_settings"); strings.Contains(string(enc), seed.DemoDirectoryPassword) {
		t.Fatal("bind password stored in clear")
	}

	// Another university sees nothing of it (RLS), at the API and in the database.
	sandboxAdmin := h.browser(t, sandboxHost)
	if _, err := seed.Account(context.Background(), h.db.App, d.ts.sandbox, seed.Member{
		Email: "sandbox-admin@qadam.test", FullName: "Сынақ Әкімші", Locale: "ru", Password: password,
		Roles: []string{"tenant_admin"}, Slug: "sandbox-admin",
	}); err != nil {
		t.Fatal(err)
	}
	login(t, sandboxAdmin, "sandbox-admin@qadam.test", password)
	theirs := decode[apigen.DirectorySettings](t, sandboxAdmin.do(http.MethodGet, "/api/v1/admin/directory", nil))
	if theirs.Configured || theirs.Enabled || theirs.Url != "" || theirs.HasBindPassword {
		t.Fatalf("sandbox sees KazUTB's settings: %+v", theirs)
	}
	expectProblem(t, directoryLogin(h.browser(t, sandboxHost), ldapStudent, ldapStudentPassword), http.StatusNotFound, "directory.disabled")
	err := h.db.App.ReadTx(db.WithTenant(context.Background(), d.ts.sandbox), func(ctx context.Context) error {
		_, err := directorydb.New(h.db.App.Conn(ctx)).GetSettings(ctx)
		return err
	})
	if !db.IsNoRows(err) {
		t.Fatalf("RLS: %v", err)
	}
	err = h.db.App.Tx(db.WithTenant(context.Background(), d.ts.sandbox), func(ctx context.Context) error {
		_, err := h.db.App.Conn(ctx).Exec(ctx, "UPDATE tenant_directory_settings SET enabled = false")
		return err
	})
	if err != nil || !scalar[bool](t, h, "SELECT enabled FROM tenant_directory_settings") {
		t.Fatalf("an update from another university reached the row: %v", err)
	}
}

func TestDirectoryLimits(t *testing.T) {
	d := newDirectoryHarness(t)
	h := d.h
	d.enable(t, nil)
	for range 5 {
		expectProblem(t, directoryLogin(h.browser(t, kazutbHost), ldapStudent, "Wrong-Password-1"), http.StatusUnauthorized, "auth.invalid_credentials")
	}
	// The same login written another way counts too.
	expectProblem(t, directoryLogin(h.browser(t, kazutbHost), strings.ToUpper(ldapStudent), ldapStudentPassword),
		http.StatusTooManyRequests, "auth.too_many_attempts")

	s, _ := d.settings(t)
	in := updateOf(t, s)
	for range 29 {
		expect(t, d.admin.do(http.MethodPost, "/api/v1/admin/directory/test", in), http.StatusOK)
	}
	expectProblem(t, d.admin.do(http.MethodPost, "/api/v1/admin/directory/test-user",
		apigen.DirectoryUserTestRequest{Settings: in, Username: ldapStudent}), http.StatusTooManyRequests, "directory.too_many_tests")
}

func TestDirectoryRefusesPlainLDAPInProduction(t *testing.T) {
	d := newDirectoryHarness(t)
	h := d.h
	prod := directory.NewService(directory.Deps{
		DB: h.db.App, Keys: h.keys, Auth: h.services.Auth, Tenants: h.services.Tenants, Audit: audit.NewLogger(h.db.App),
		Limiter: ratelimit.New(redis.NewClient(&redis.Options{Addr: h.redis.Addr()})), Production: true,
	})
	s, _ := d.settings(t)
	in := updateOf(t, s)
	adminID := scalar[uuid.UUID](t, h, "SELECT id FROM users WHERE email = 'rector-office@qadam.test'")
	ctx := tenancy.With(context.Background(), tenancy.Tenant{ID: d.ts.kazutb, Slug: "kazutb", Locale: "kk"})
	ctx = db.WithTenant(ctx, d.ts.kazutb)
	ctx = authz.WithPrincipal(ctx, authz.Principal{UserID: adminID, TenantID: d.ts.kazutb, Roles: []authz.Role{authz.RoleTenantAdmin}})

	_, _, err := prod.Save(ctx, in, nil)
	e, ok := apperr.As(err)
	if !ok || e.Kind != apperr.KindValidation || len(e.Fields) != 1 || e.Fields[0].Field != "url" || e.Fields[0].Code != "plain_ldap" {
		t.Fatalf("production saved plain LDAP: %v", err)
	}
	if _, err := prod.TestConnection(ctx, in); err == nil {
		t.Fatal("production checked plain LDAP")
	}
	// StartTLS with the university's certification authority is fine.
	in.StartTls = true
	in.CaCertPem = ptr(d.srv.CAPEM)
	res, err := prod.TestConnection(ctx, in)
	if err != nil || !res.Ok {
		t.Fatalf("StartTLS in production: %v %+v", err, res.Steps)
	}
	if _, _, err := prod.Save(ctx, in, nil); err != nil {
		t.Fatal(err)
	}
}

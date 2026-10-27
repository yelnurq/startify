package directory

import (
	"errors"
	"slices"
	"strings"
	"testing"
	"time"

	"github.com/go-ldap/ldap/v3"

	"github.com/yelnurq/startify/backend/internal/apigen"
	"github.com/yelnurq/startify/backend/internal/directory/ldaptest"
)

// demoConfig is the configuration of the demo directory (deploy/ldap).
func demoConfig(srv *ldaptest.Server) Config {
	p, _ := PresetNamed(PresetOpenLDAP)
	return Config{
		Preset: p.Name, URLs: []string{srv.URL()}, StartTLS: true, CACertPEM: srv.CAPEM, Timeout: 5 * time.Second,
		BindDN: ldaptest.BindDN, BindPassword: ldaptest.BindPassword, UserBaseDN: ldaptest.BaseDN, UserFilter: p.UserFilter,
		LoginAttr: p.LoginAttr, EmailAttr: p.EmailAttr, NameAttr: p.NameAttr, GivenNameAttr: p.GivenNameAttr,
		SurnameAttr: p.SurnameAttr, IDAttr: p.IDAttr, GroupsAttr: p.GroupsAttr, DefaultRole: "student",
	}
}

func stepsOf(rec *recorder) string {
	parts := []string{}
	for _, s := range rec.steps {
		part := string(s.Step) + ":" + string(s.Status)
		if s.Code != nil {
			part += ":" + *s.Code
		}
		parts = append(parts, part)
	}
	return strings.Join(parts, " ")
}

func TestTLSNeedsTheUniversityAuthority(t *testing.T) {
	srv := ldaptest.Shared(t)
	c := demoConfig(srv)

	// StartTLS with the certification authority of the university.
	rec := &recorder{}
	sess, err := open(c, Rules{}, rec)
	if err != nil {
		t.Fatalf("StartTLS: %v (%s)", err, stepsOf(rec))
	}
	sess.close()
	tlsStep := rec.steps[1]
	if tlsStep.Step != stepTLS || tlsStep.Status != statusOK || tlsStep.Certificate == nil ||
		!strings.Contains(tlsStep.Certificate.Subject, "localhost") || tlsStep.Certificate.Issuer != "CN=Qadam test CA" {
		t.Fatalf("tls step: %+v", tlsStep)
	}

	// Without it the server is not trusted: no skip-verify fallback.
	c.CACertPEM = ""
	rec = &recorder{}
	if _, err := open(c, Rules{}, rec); err == nil || stepsOf(rec) != "connect:ok tls:failed:tls_unknown_authority" {
		t.Fatalf("unknown authority: %v (%s)", err, stepsOf(rec))
	}
	c.URLs = []string{srv.LDAPSURL()}
	c.StartTLS = false
	rec = &recorder{}
	if _, err := open(c, Rules{}, rec); err == nil || stepsOf(rec) != "connect:ok tls:failed:tls_unknown_authority" {
		t.Fatalf("ldaps unknown authority: %v (%s)", err, stepsOf(rec))
	}

	// The TLS port with the authority of the university.
	c.CACertPEM = srv.CAPEM
	rec = &recorder{}
	sess, err = open(c, Rules{}, rec)
	if err != nil || stepsOf(rec) != "connect:ok tls:ok" {
		t.Fatalf("ldaps: %v (%s)", err, stepsOf(rec))
	}
	sess.close()

	// A name the certificate is not for.
	c.URLs = []string{strings.Replace(srv.LDAPSURL(), "localhost", "127.0.0.2", 1)}
	if strings.Contains(srv.LDAPSURL(), "localhost") {
		rec = &recorder{}
		_, err := open(c, Rules{}, rec)
		if err == nil {
			t.Fatalf("wrong host name accepted (%s)", stepsOf(rec))
		}
		if got := stepsOf(rec); strings.HasPrefix(got, "connect:ok") && got != "connect:ok tls:failed:tls_hostname" {
			t.Fatalf("wrong host name: %s", got)
		}
	}
}

func TestFailoverAndUnreachableServers(t *testing.T) {
	srv := ldaptest.Shared(t)
	c := demoConfig(srv)
	c.Timeout = 2 * time.Second
	c.URLs = []string{"ldap://127.0.0.1:1", "ldap://dc01.qadam-test.invalid:389", srv.URL()}
	rec := &recorder{}
	sess, err := open(c, Rules{}, rec)
	if err != nil {
		t.Fatalf("failover: %v (%s)", err, stepsOf(rec))
	}
	defer sess.close()
	if got := stepsOf(rec); got != "connect:failed:refused connect:failed:dns connect:ok tls:ok" || sess.url != srv.URL() {
		t.Fatalf("steps: %s", got)
	}
}

func TestServiceBindAndBase(t *testing.T) {
	srv := ldaptest.Shared(t)
	c := demoConfig(srv)
	rec := &recorder{}
	sess, err := open(c, Rules{}, rec)
	if err != nil {
		t.Fatal(err)
	}
	defer sess.close()

	// An empty password is never sent: Active Directory would take it for an
	// anonymous bind and answer success (OpenLDAP refuses those by itself).
	if err := sess.bindService("", rec); err == nil || !strings.Contains(stepsOf(rec), "bind:failed:bind_password_missing") {
		t.Fatalf("empty password: %v (%s)", err, stepsOf(rec))
	}
	rec = &recorder{}
	if err := sess.bindService("wrong-password", rec); err == nil || stepsOf(rec) != "bind:failed:invalid_credentials" {
		t.Fatalf("wrong password: %v (%s)", err, stepsOf(rec))
	}
	rec = &recorder{}
	if err := sess.bindService(c.BindPassword, rec); err != nil {
		t.Fatal(err)
	}
	if err := sess.checkBase(rec); err != nil {
		t.Fatal(err)
	}
	sess.describeServer(rec)
	server := rec.steps[2]
	if rec.steps[1].Count == nil || *rec.steps[1].Count != 1 || server.Vendor == nil || *server.Vendor != "OpenLDAP" ||
		server.NamingContexts == nil || !slices.Contains(*server.NamingContexts, ldaptest.BaseDN) {
		t.Fatalf("search and server: %+v %+v", rec.steps[1], server)
	}

	sess.cfg.UserBaseDN = "ou=Missing,dc=kazutb,dc=local"
	rec = &recorder{}
	if err := sess.checkBase(rec); err == nil || stepsOf(rec) != "search:failed:no_such_object" {
		t.Fatalf("missing base: %v (%s)", err, stepsOf(rec))
	}
	sess.cfg.UserBaseDN = ldaptest.BaseDN
	sess.cfg.UserFilter = "(&(objectClass=posixAccount)(uid={username}))"
	rec = &recorder{}
	if err := sess.checkBase(rec); err != nil || stepsOf(rec) != "search:warning:no_entries" {
		t.Fatalf("filter that finds nobody: %v (%s)", err, stepsOf(rec))
	}
}

func TestFindUsers(t *testing.T) {
	srv := ldaptest.Shared(t)
	c := demoConfig(srv)
	c.NestedGroups = true
	sess, err := open(c, Rules{}, nil)
	if err != nil {
		t.Fatal(err)
	}
	defer sess.close()
	if err := sess.bindService(c.BindPassword, nil); err != nil {
		t.Fatal(err)
	}

	e, err := sess.findUser("sarsenova.a")
	if err != nil {
		t.Fatal(err)
	}
	student := mapEntry(c, e)
	if student.DN != "uid=sarsenova.a,ou=Students,dc=kazutb,dc=local" || student.Email != "aigerim@qadam.test" ||
		student.FullName != "Айгерім Сәрсенова" || len(student.Subject) != 36 || strings.HasPrefix(student.Subject, "dn:") {
		t.Fatalf("student: %+v", student)
	}
	if groups := sess.groups(student); !slices.Equal(groups, []string{"cn=students,ou=Groups,dc=kazutb,dc=local"}) {
		t.Fatalf("student groups: %v", groups)
	}

	// Nested groups: innovation is a member of staff.
	e, err = sess.findUser("abenova.d")
	if err != nil {
		t.Fatal(err)
	}
	manager := mapEntry(c, e)
	groups := sess.groups(manager)
	if !InGroup(groups, "cn=innovation,ou=Groups,dc=kazutb,dc=local") || !InGroup(groups, "cn=staff,ou=Groups,dc=kazutb,dc=local") || len(groups) != 2 {
		t.Fatalf("nested groups: %v", groups)
	}
	sess.cfg.NestedGroups = false
	if groups := sess.groups(manager); len(groups) != 1 {
		t.Fatalf("direct groups only: %v", groups)
	}

	// The login is a value, never part of the filter.
	for _, injected := range []string{"*", "*)(uid=*", "sarsenova.a)(|(uid=*", "*)(objectClass=*"} {
		if _, err := sess.findUser(injected); !errors.Is(err, errNotFound) {
			t.Errorf("%q: %v, want not found", injected, err)
		}
	}
	if _, err := sess.findUser("nobody.here"); !errors.Is(err, errNotFound) {
		t.Fatalf("unknown: %v", err)
	}

	// Two entries with one login: nobody signs in with it.
	admin := srv.Admin(t)
	for _, ou := range []string{"Staff", "Students"} {
		dn := "uid=twin.login,ou=" + ou + ",dc=kazutb,dc=local"
		add := ldap.NewAddRequest(dn, nil)
		add.Attribute("objectClass", []string{"inetOrgPerson"})
		add.Attribute("uid", []string{"twin.login"})
		add.Attribute("cn", []string{"Twin " + ou})
		add.Attribute("sn", []string{"Twin"})
		if err := admin.Add(add); err != nil && !ldap.IsErrorWithCode(err, ldap.LDAPResultEntryAlreadyExists) {
			t.Fatal(err)
		}
		t.Cleanup(func() { _ = admin.Del(ldap.NewDelRequest(dn, nil)) })
	}
	if _, err := sess.findUser("twin.login"); !errors.Is(err, errAmbiguous) {
		t.Fatalf("ambiguous: %v", err)
	}
}

func TestUserBindRefusals(t *testing.T) {
	srv := ldaptest.Shared(t)
	c := demoConfig(srv)
	sess, err := open(c, Rules{}, nil)
	if err != nil {
		t.Fatal(err)
	}
	defer sess.close()
	err = sess.conn.Bind("uid=sarsenova.a,ou=Students,dc=kazutb,dc=local", "Wrong-Password-1")
	if got := bindRefusal(err); got != bindWrong {
		t.Fatalf("wrong password: %q (%v)", got, err)
	}
	if err := sess.conn.Bind("uid=sarsenova.a,ou=Students,dc=kazutb,dc=local", "Aigerim-Ldap-2026"); err != nil {
		t.Fatalf("right password: %v", err)
	}
}

func TestConnectionCheckReportsEveryStep(t *testing.T) {
	srv := ldaptest.Shared(t)
	c := demoConfig(srv)
	rec := &recorder{}
	if sess, err := open(c, Rules{}, rec); err == nil {
		if err := sess.bindService("wrong", rec); err == nil {
			t.Fatal("wrong bind accepted")
		}
		sess.close()
	}
	rec.skipRest()
	if got := stepsOf(rec); got != "connect:ok tls:ok bind:failed:invalid_credentials search:skipped server:skipped" {
		t.Fatalf("steps: %s", got)
	}
	if !rec.failed() {
		t.Fatal("a failed step fails the check")
	}
	for _, s := range rec.steps {
		if s.Step == apigen.DirectoryTestStepStepBind && s.Detail != nil && strings.Contains(*s.Detail, "wrong") {
			t.Fatal("the password must not appear in the detail")
		}
	}
}

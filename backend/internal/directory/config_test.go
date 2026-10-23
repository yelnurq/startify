package directory

import (
	"crypto/tls"
	"crypto/x509"
	"errors"
	"fmt"
	"net"
	"slices"
	"strings"
	"syscall"
	"testing"
	"time"

	"github.com/go-ldap/ldap/v3"
	"github.com/google/uuid"

	"github.com/yelnurq/startify/backend/internal/apigen"
	"github.com/yelnurq/startify/backend/internal/platform/apperr"
	"github.com/yelnurq/startify/backend/internal/platform/secrets"
)

func str(s string) *string { return &s }

// validInput is a complete form of the Active Directory preset.
func validInput() apigen.DirectorySettingsUpdate {
	p, _ := PresetNamed(PresetActiveDirectory)
	return apigen.DirectorySettingsUpdate{
		Preset: apigen.DirectoryPreset(p.Name), Url: "ldaps://dc01.kazutb.local:636 ldaps://dc02.kazutb.local",
		TimeoutSeconds: 10, BindDn: "CN=qadam-svc,OU=Service Accounts,DC=kazutb,DC=local",
		UserBaseDn: "OU=Students,DC=kazutb,DC=local", UserFilter: p.UserFilter,
		LoginAttr: p.LoginAttr, EmailAttr: p.EmailAttr, EmailFallbackAttr: str(p.EmailFallbackAttr),
		NameAttr: p.NameAttr, GivenNameAttr: str(p.GivenNameAttr), SurnameAttr: str(p.SurnameAttr),
		IdAttr: p.IDAttr, GroupsAttr: p.GroupsAttr, NestedGroups: true, DefaultRole: apigen.RoleStudent,
		RoleMappings: []apigen.DirectoryRoleMapping{
			{GroupDn: "CN=Qadam Admins,OU=Groups,DC=kazutb,DC=local", Role: apigen.RoleTenantAdmin},
		},
		AutoProvision: true,
		LoginLabel:    &apigen.LocalizedText{Ru: str("  Учётная запись КазУТБ ")},
	}
}

func fieldCodes(t *testing.T, err error) map[string]string {
	t.Helper()
	e, ok := apperr.As(err)
	if !ok || e.Kind != apperr.KindValidation {
		t.Fatalf("want a validation error, got %v", err)
	}
	out := map[string]string{}
	for _, f := range e.Fields {
		out[f.Field] = f.Code
	}
	return out
}

func TestPresetsFillEverything(t *testing.T) {
	names := []string{}
	for _, p := range Presets {
		names = append(names, p.Name)
		if !strings.Contains(p.UserFilter, Placeholder) {
			t.Errorf("%s: filter %q has no placeholder", p.Name, p.UserFilter)
		}
		if _, err := ldap.CompileFilter(UserFilter(p.UserFilter, "ivanov.a")); err != nil {
			t.Errorf("%s: filter does not compile: %v", p.Name, err)
		}
		for _, a := range []string{p.LoginAttr, p.EmailAttr, p.NameAttr, p.IDAttr, p.GroupsAttr} {
			if !attrPattern.MatchString(a) {
				t.Errorf("%s: attribute %q", p.Name, a)
			}
		}
	}
	if !slices.Equal(names, []string{PresetActiveDirectory, PresetOpenLDAP, PresetFreeIPA, PresetCustom}) {
		t.Fatalf("presets: %v", names)
	}
	ad, _ := PresetNamed(PresetActiveDirectory)
	if ad.IDAttr != "objectGUID" || !ad.NestedGroups || ad.EmailFallbackAttr != "userPrincipalName" ||
		ad.UserFilter != "(&(objectCategory=person)(objectClass=user)(sAMAccountName={username}))" {
		t.Fatalf("active directory: %+v", ad)
	}
	if ipa, _ := PresetNamed(PresetFreeIPA); ipa.IDAttr != "ipaUniqueID" || !strings.HasPrefix(ipa.BaseDNExample, "cn=users,cn=accounts") {
		t.Fatalf("freeipa: %+v", ipa)
	}
	if ol, _ := PresetNamed(PresetOpenLDAP); ol.IDAttr != "entryUUID" || ol.LoginAttr != "uid" {
		t.Fatalf("openldap: %+v", ol)
	}
	if d := Defaults(); d.Preset != PresetActiveDirectory || d.Enabled || d.Timeout != 10*time.Second || d.DefaultRole != "student" {
		t.Fatalf("defaults: %+v", d)
	}
}

func TestParseInputAcceptsAValidForm(t *testing.T) {
	c, err := parseInput(validInput(), Rules{})
	if err != nil {
		t.Fatal(err)
	}
	if len(c.URLs) != 2 || c.Timeout != 10*time.Second || c.LoginLabel["ru"] != "Учётная запись КазУТБ" || len(c.LoginLabel) != 1 {
		t.Fatalf("parsed: %+v", c)
	}
	if c.Plain() {
		t.Fatal("ldaps is not plain")
	}
}

func TestParseInputRejectsBadValues(t *testing.T) {
	in := validInput()
	in.Url = "https://dc01.kazutb.local ldap://user:pw@dc02"
	in.BindDn = "not a dn"
	in.UserBaseDn = ""
	in.UserFilter = "(sAMAccountName=ivanov)"
	in.LoginAttr = "sam account"
	in.CaCertPem = str("-----BEGIN CERTIFICATE-----\nAAAA\n-----END CERTIFICATE-----")
	in.DefaultRole = "super_admin"
	in.TimeoutSeconds = 0
	in.RoleMappings = append(in.RoleMappings, in.RoleMappings[0], apigen.DirectoryRoleMapping{GroupDn: "x", Role: "rector"})
	codes := fieldCodes(t, parseOnly(in, Rules{}))
	want := map[string]string{
		"url": codeLDAPURL, "bind_dn": codeLDAPDN, "user_base_dn": "required", "user_filter": codePlaceholder,
		"login_attr": codeLDAPAttribute, "ca_cert_pem": codePEM, "default_role": "invalid", "timeout_seconds": "min",
		"role_mappings.1.group_dn": codeDuplicate, "role_mappings.2.group_dn": codeLDAPDN, "role_mappings.2.role": "invalid",
	}
	for field, code := range want {
		if codes[field] != code {
			t.Errorf("%s: code %q, want %q (all: %v)", field, codes[field], code, codes)
		}
	}

	in = validInput()
	in.UserFilter = "(&(uid={username})"
	if codes := fieldCodes(t, parseOnly(in, Rules{})); codes["user_filter"] != codeLDAPFilter {
		t.Fatalf("unbalanced filter: %v", codes)
	}
	in = validInput()
	in.Url = strings.Repeat("ldaps://dc.kazutb.local ", 6)
	if codes := fieldCodes(t, parseOnly(in, Rules{})); codes["url"] != "maxItems" {
		t.Fatalf("too many addresses: %v", codes)
	}
}

func parseOnly(in apigen.DirectorySettingsUpdate, rules Rules) error {
	_, err := parseInput(in, rules)
	return err
}

func TestPlainLDAPOnlyOutsideProduction(t *testing.T) {
	in := validInput()
	in.Url = "ldap://dc01.kazutb.local:389"
	if codes := fieldCodes(t, parseOnly(in, Rules{PlainAllowed: false})); codes["url"] != codePlainLDAP {
		t.Fatalf("production accepted plain LDAP: %v", codes)
	}
	c, err := parseInput(in, Rules{PlainAllowed: true})
	if err != nil || !c.Plain() {
		t.Fatalf("development: %v %v", err, c.Plain())
	}
	// StartTLS encrypts ldap:// before the password is sent.
	in.StartTls = true
	if c, err := parseInput(in, Rules{PlainAllowed: false}); err != nil || c.Plain() {
		t.Fatalf("StartTLS in production: %v", err)
	}
	// Refused at connection time too, before anything is sent.
	rec := &recorder{}
	_, err = open(Config{URLs: []string{"ldap://127.0.0.1:1"}, Timeout: time.Second}, Rules{}, rec)
	var se *stepError
	if !errors.As(err, &se) || se.code != codePlainNotAllowed || len(rec.steps) != 1 || rec.steps[0].Step != stepTLS {
		t.Fatalf("open plain in production: %v %+v", err, rec.steps)
	}
}

func TestUserFilterEscapesTheLogin(t *testing.T) {
	filter := "(&(objectClass=inetOrgPerson)(uid={username}))"
	for login, want := range map[string]string{
		"ivanov.a":   "(&(objectClass=inetOrgPerson)(uid=ivanov.a))",
		"*)(uid=*":   `(&(objectClass=inetOrgPerson)(uid=\2a\29\28uid=\2a))`,
		`a\b`:        `(&(objectClass=inetOrgPerson)(uid=a\5cb))`,
		"x)(|(uid=*": `(&(objectClass=inetOrgPerson)(uid=x\29\28|\28uid=\2a))`,
		"\x00":       `(&(objectClass=inetOrgPerson)(uid=\00))`,
	} {
		got := UserFilter(filter, login)
		if got != want {
			t.Errorf("%q: %s, want %s", login, got, want)
		}
		if _, err := ldap.CompileFilter(got); err != nil {
			t.Errorf("%q: %v", login, err)
		}
	}
	if got := UserFilter("(|(uid={username})(mail={username}))", "a*"); got != `(|(uid=a\2a)(mail=a\2a))` {
		t.Fatalf("every placeholder: %s", got)
	}
}

func TestNormalizeLogin(t *testing.T) {
	for _, c := range []struct{ preset, in, want string }{
		{PresetActiveDirectory, `KAZUTB\ivanov.a`, "ivanov.a"},
		{PresetActiveDirectory, "ivanov.a@kazutb.local", "ivanov.a"},
		{PresetActiveDirectory, `  kazutb\Ivanov.A@kazutb.local `, "Ivanov.A"},
		{PresetActiveDirectory, "ivanov.a", "ivanov.a"},
		{PresetOpenLDAP, "ivanov.a@kazutb.local", "ivanov.a@kazutb.local"},
		{PresetOpenLDAP, "\tsarsenova.a\n", "sarsenova.a"},
		{PresetOpenLDAP, "a\x00b", ""},
		{PresetActiveDirectory, `KAZUTB\`, ""},
		{PresetActiveDirectory, "   ", ""},
	} {
		if got := NormalizeLogin(c.preset, c.in); got != c.want {
			t.Errorf("%s %q: %q, want %q", c.preset, c.in, got, c.want)
		}
	}
}

func TestFormatGUID(t *testing.T) {
	// objectGUID {f0e1d2c3-b4a5-9687-7869-5a4b3c2d1e0f} as Active Directory sends it.
	raw := []byte{0xc3, 0xd2, 0xe1, 0xf0, 0xa5, 0xb4, 0x87, 0x96, 0x78, 0x69, 0x5a, 0x4b, 0x3c, 0x2d, 0x1e, 0x0f}
	if got := FormatGUID(raw); got != "f0e1d2c3-b4a5-9687-7869-5a4b3c2d1e0f" {
		t.Fatalf("guid: %s", got)
	}
	if _, err := uuid.Parse(FormatGUID(raw)); err != nil {
		t.Fatal(err)
	}
	e := ldap.NewEntry("CN=Иванов,OU=Students,DC=kazutb,DC=local", nil)
	e.Attributes = []*ldap.EntryAttribute{{Name: "objectGUID", ByteValues: [][]byte{raw}, Values: []string{string(raw)}}}
	if got := subjectOf("objectguid", e); got != "f0e1d2c3-b4a5-9687-7869-5a4b3c2d1e0f" {
		t.Fatalf("subject: %s", got)
	}
	if got := subjectOf("entryUUID", e); got != "dn:cn=иванов,ou=students,dc=kazutb,dc=local" {
		t.Fatalf("missing id falls back to the DN: %s", got)
	}
}

func TestMapEntry(t *testing.T) {
	c := Defaults()
	e := ldap.NewEntry("CN=Ivanov A,OU=Students,DC=kazutb,DC=local", map[string][]string{
		"sAMAccountName":    {"ivanov.a"},
		"userPrincipalName": {"Ivanov.A@KazUTB.kz"},
		"givenName":         {"Айбек"},
		"sn":                {"Иванов"},
		"memberOf":          {"CN=Students,OU=Groups,DC=kazutb,DC=local"},
	})
	got := mapEntry(c, e)
	if got.Login != "ivanov.a" || got.Email != "ivanov.a@kazutb.kz" || got.FullName != "Айбек Иванов" ||
		len(got.Groups) != 1 || !strings.HasPrefix(got.Subject, "dn:") {
		t.Fatalf("mapped: %+v", got)
	}
	for _, bad := range []string{"ivanov", "a b@kazutb.kz", "@kazutb.kz", "x@localhost"} {
		if mailOf(bad) != "" {
			t.Errorf("%q passed as an address", bad)
		}
	}
}

func TestRolesForGroups(t *testing.T) {
	c := Defaults()
	c.RoleMappings = []RoleMapping{
		{GroupDN: "CN=Staff,OU=Groups,DC=kazutb,DC=local", Role: "supervisor"},
		{GroupDN: "CN=Admins,OU=Groups,DC=kazutb,DC=local", Role: "tenant_admin"},
		{GroupDN: "CN=Staff,OU=Groups,DC=kazutb,DC=local", Role: "mentor"},
	}
	roles, byDefault := c.RolesFor([]string{"cn=staff,ou=groups,dc=kazutb,dc=local"})
	if byDefault || !slices.Equal(roles, []string{"supervisor", "mentor"}) {
		t.Fatalf("staff: %v %v", roles, byDefault)
	}
	if roles, byDefault := c.RolesFor(nil); !byDefault || !slices.Equal(roles, []string{"student"}) {
		t.Fatalf("no group: %v", roles)
	}
	if got := c.ManagedRoles(); !slices.Equal(got, []string{"supervisor", "tenant_admin", "mentor"}) {
		t.Fatalf("managed: %v", got)
	}
	if !InGroup([]string{"CN=Admins, OU=Groups, DC=kazutb, DC=local"}, "cn=admins,ou=groups,dc=kazutb,dc=local") {
		t.Fatal("DNs compare without case and spaces")
	}
}

func TestDomainOf(t *testing.T) {
	if got := domainOf("CN=Иванов,OU=Students,DC=kazutb,DC=local"); got != "DC=kazutb,DC=local" {
		t.Fatalf("domain: %s", got)
	}
	if got := domainOf("uid=x,ou=people"); got != "" {
		t.Fatalf("no domain: %s", got)
	}
}

func TestBindRefusalReadsActiveDirectoryCodes(t *testing.T) {
	ad := func(data string) error {
		return ldap.NewError(ldap.LDAPResultInvalidCredentials,
			fmt.Errorf("80090308: LdapErr: DSID-0C09044E, comment: AcceptSecurityContext error, data %s, v4f7c", data))
	}
	for data, want := range map[string]string{
		"52e": bindWrong, "525": bindWrong, "775": bindWrong, "532": bindExpired, "773": bindExpired,
		"533": bindDisabled, "701": bindDisabled, "530": bindDenied, "531": bindDenied,
	} {
		if got := bindRefusal(ad(data)); got != want {
			t.Errorf("data %s: %q, want %q", data, got, want)
		}
	}
	if got := bindRefusal(ldap.NewError(ldap.LDAPResultInvalidCredentials, errors.New(""))); got != bindWrong {
		t.Fatalf("OpenLDAP: %q", got)
	}
	if got := bindRefusal(ldap.NewError(ldap.LDAPResultBusy, errors.New("busy"))); got != "" {
		t.Fatalf("not a refusal: %q", got)
	}
}

func TestClassifyErrors(t *testing.T) {
	cert := &x509.Certificate{}
	for name, c := range map[string]struct {
		err  error
		want string
	}{
		"dns":       {&net.OpError{Op: "dial", Err: &net.DNSError{Err: "no such host", Name: "dc01.kazutb.local"}}, codeDNS},
		"refused":   {ldap.NewError(ldap.ErrorNetwork, &net.OpError{Op: "dial", Err: syscall.ECONNREFUSED}), codeRefused},
		"timeout":   {ldap.NewError(ldap.ErrorNetwork, errors.New("ldap: connection timed out")), codeTimeout},
		"authority": {&tls.CertificateVerificationError{Err: x509.UnknownAuthorityError{Cert: cert}}, codeUnknownAuthority},
		"starttls":  {ldap.NewError(ldap.ErrorNetwork, errors.New("TLS handshake failed (tls: failed to verify certificate: x509: certificate signed by unknown authority)")), codeUnknownAuthority},
		"hostname":  {x509.HostnameError{Certificate: cert, Host: "dc01"}, codeHostname},
		"expired":   {x509.CertificateInvalidError{Cert: cert, Reason: x509.Expired}, codeExpired},
		"not tls":   {tls.RecordHeaderError{Msg: "first record does not look like a TLS handshake"}, codeHandshake},
	} {
		if got := classifyNetwork(c.err); got != c.want {
			t.Errorf("%s: %q, want %q", name, got, c.want)
		}
	}
	for code, want := range map[uint16]string{
		ldap.LDAPResultInvalidCredentials: codeInvalidCredentials, ldap.LDAPResultNoSuchObject: codeNoSuchObject,
		ldap.LDAPResultInsufficientAccessRights: codeInsufficientAccess, ldap.LDAPResultStrongAuthRequired: codeStrongAuth,
		ldap.LDAPResultUnwillingToPerform: codeUnwilling, ldap.LDAPResultReferral: codeReferral,
	} {
		if got := classifyResult(ldap.NewError(code, errors.New("x"))); got != want {
			t.Errorf("result %d: %q, want %q", code, got, want)
		}
	}
}

func TestProofBindsTheConnection(t *testing.T) {
	keys, err := secrets.NewKeyring("test-secret-key-for-qadam-directory-tests-0123456789")
	if err != nil {
		t.Fatal(err)
	}
	now := time.Date(2026, 10, 23, 12, 0, 0, 0, time.UTC)
	s := &Service{keys: keys, now: func() time.Time { return now }}
	tenantID, other := uuid.New(), uuid.New()
	c, err := parseInput(validInput(), Rules{})
	if err != nil {
		t.Fatal(err)
	}
	c.BindPassword = "secret"
	p := s.proof(tenantID, c, now.Add(proofTTL))
	if !s.validProof(tenantID, &p, c) {
		t.Fatal("proof of the same connection")
	}
	changed := c
	changed.BindPassword = "other"
	labels := c
	labels.LoginLabel = map[string]string{"en": "KazUTB account"}
	forged := p[:len(p)-2] + "AA"
	for name, ok := range map[string]bool{
		"another password":   s.validProof(tenantID, &p, changed),
		"another university": s.validProof(other, &p, c),
		"forged":             s.validProof(tenantID, &forged, c),
		"missing":            s.validProof(tenantID, nil, c),
	} {
		if ok {
			t.Errorf("%s accepted", name)
		}
	}
	if !s.validProof(tenantID, &p, labels) {
		t.Fatal("labels are not part of the connection")
	}
	now = now.Add(2 * proofTTL)
	if s.validProof(tenantID, &p, c) {
		t.Fatal("expired proof accepted")
	}
}

func TestChangedFieldsNeverCarryThePassword(t *testing.T) {
	before := stored{cfg: Defaults()}
	after := Defaults()
	after.URLs = []string{"ldaps://dc01"}
	after.BindPassword = "Very-Secret-Bind"
	fields := changedFields(before, after, true)
	if !slices.Contains(fields, "url") || !slices.Contains(fields, "bind_password") || slices.Contains(fields, "labels") {
		t.Fatalf("changed: %v", fields)
	}
	for _, f := range fields {
		if strings.Contains(f, "Secret") {
			t.Fatal("password in the audit")
		}
	}
}

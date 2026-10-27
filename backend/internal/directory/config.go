package directory

import (
	"crypto/x509"
	"encoding/json"
	"encoding/pem"
	"net/url"
	"regexp"
	"slices"
	"strconv"
	"strings"
	"time"
	"unicode/utf8"

	"github.com/go-ldap/ldap/v3"

	"github.com/yelnurq/startify/backend/internal/apiconv"
	"github.com/yelnurq/startify/backend/internal/apigen"
	"github.com/yelnurq/startify/backend/internal/directory/directorydb"
	"github.com/yelnurq/startify/backend/internal/platform/authz"
	"github.com/yelnurq/startify/backend/internal/platform/validate"
)

// Kinds of directories.
const (
	PresetActiveDirectory = "active_directory"
	PresetOpenLDAP        = "openldap"
	PresetFreeIPA         = "freeipa"
	PresetCustom          = "custom"
)

// Placeholder is replaced with the escaped login in the user filter.
const Placeholder = "{username}"

// Field error codes beyond those of package validate (common:validation.*).
const (
	codeLDAPURL       = "ldap_url"
	codeLDAPDN        = "ldap_dn"
	codeLDAPFilter    = "ldap_filter"
	codePlaceholder   = "ldap_placeholder"
	codeLDAPAttribute = "ldap_attribute"
	codePEM           = "pem"
	codePlainLDAP     = "plain_ldap"
	codeDuplicate     = "duplicate"
)

// Limits of the settings (repeated in openapi.yaml).
const (
	maxURLs          = 5
	maxURLLength     = 1000
	maxDNLength      = 1024
	maxFilterLength  = 1024
	maxCALength      = 64 << 10
	maxMappings      = 50
	maxLabelLength   = 80
	maxHintLength    = 160
	maxPasswordBytes = 256
	defaultTimeout   = 10 * time.Second
)

// Preset holds the defaults of a kind of directory.
type Preset struct {
	Name              string
	UserFilter        string
	LoginAttr         string
	EmailAttr         string
	EmailFallbackAttr string
	NameAttr          string
	GivenNameAttr     string
	SurnameAttr       string
	IDAttr            string
	GroupsAttr        string
	NestedGroups      bool
	// BaseDNExample is shown next to the base DN field.
	BaseDNExample string
}

// Presets are the defaults of every kind, in the order of the admin page.
var Presets = []Preset{
	{
		Name:       PresetActiveDirectory,
		UserFilter: "(&(objectCategory=person)(objectClass=user)(sAMAccountName={username}))",
		LoginAttr:  "sAMAccountName", EmailAttr: "mail", EmailFallbackAttr: "userPrincipalName",
		NameAttr: "displayName", GivenNameAttr: "givenName", SurnameAttr: "sn",
		IDAttr: "objectGUID", GroupsAttr: "memberOf", NestedGroups: true,
		BaseDNExample: "OU=Students,DC=kazutb,DC=local",
	},
	{
		Name:       PresetOpenLDAP,
		UserFilter: "(&(objectClass=inetOrgPerson)(uid={username}))",
		LoginAttr:  "uid", EmailAttr: "mail", NameAttr: "cn", GivenNameAttr: "givenName", SurnameAttr: "sn",
		IDAttr: "entryUUID", GroupsAttr: "memberOf",
		BaseDNExample: "ou=Students,dc=kazutb,dc=local",
	},
	{
		Name:       PresetFreeIPA,
		UserFilter: "(&(objectClass=person)(uid={username}))",
		LoginAttr:  "uid", EmailAttr: "mail", NameAttr: "cn", GivenNameAttr: "givenName", SurnameAttr: "sn",
		IDAttr: "ipaUniqueID", GroupsAttr: "memberOf",
		BaseDNExample: "cn=users,cn=accounts,dc=kazutb,dc=local",
	},
	{
		Name:       PresetCustom,
		UserFilter: "(uid={username})",
		LoginAttr:  "uid", EmailAttr: "mail", NameAttr: "cn", GivenNameAttr: "givenName", SurnameAttr: "sn",
		IDAttr: "entryUUID", GroupsAttr: "memberOf",
		BaseDNExample: "ou=people,dc=example,dc=org",
	},
}

// PresetNamed returns the preset of a kind.
func PresetNamed(name string) (Preset, bool) {
	i := slices.IndexFunc(Presets, func(p Preset) bool { return p.Name == name })
	if i < 0 {
		return Preset{}, false
	}
	return Presets[i], true
}

// RoleMapping gives members of a group a role.
type RoleMapping struct {
	GroupDN string `json:"group_dn"`
	Role    string `json:"role"`
}

// Config is the directory sign-in of a university.
type Config struct {
	Enabled      bool
	Preset       string
	URLs         []string
	StartTLS     bool
	CACertPEM    string
	Timeout      time.Duration
	BindDN       string
	BindPassword string
	UserBaseDN   string
	UserFilter   string

	LoginAttr         string
	EmailAttr         string
	EmailFallbackAttr string
	NameAttr          string
	GivenNameAttr     string
	SurnameAttr       string
	IDAttr            string
	GroupsAttr        string
	NestedGroups      bool

	RoleMappings    []RoleMapping
	DefaultRole     string
	RequiredGroupDN string
	AutoProvision   bool
	SyncRoles       bool

	LoginLabel   apiconv.Text
	UsernameHint apiconv.Text
}

// Defaults are the settings of a university that has saved none: Active
// Directory, off.
func Defaults() Config {
	p := Presets[0]
	return Config{
		Preset: p.Name, Timeout: defaultTimeout, UserFilter: p.UserFilter, LoginAttr: p.LoginAttr,
		EmailAttr: p.EmailAttr, EmailFallbackAttr: p.EmailFallbackAttr, NameAttr: p.NameAttr,
		GivenNameAttr: p.GivenNameAttr, SurnameAttr: p.SurnameAttr, IDAttr: p.IDAttr, GroupsAttr: p.GroupsAttr,
		NestedGroups: p.NestedGroups, DefaultRole: string(authz.RoleStudent), AutoProvision: true,
		RoleMappings: []RoleMapping{},
	}
}

func deref(s *string) string {
	if s == nil {
		return ""
	}
	return *s
}

func ptr(s string) *string {
	if s == "" {
		return nil
	}
	return &s
}

// splitURLs reads one or more addresses separated by spaces or commas.
func splitURLs(s string) []string {
	return strings.FieldsFunc(s, func(r rune) bool { return r == ' ' || r == ',' || r == '\t' || r == '\n' || r == '\r' })
}

// configOf reads stored settings (the sealed password is opened by the caller).
func configOf(row directorydb.TenantDirectorySetting) Config {
	c := Config{
		Enabled: row.Enabled, Preset: row.Preset, URLs: splitURLs(row.Url), StartTLS: row.StartTls,
		CACertPEM: deref(row.CaCertPem), Timeout: time.Duration(row.TimeoutSeconds) * time.Second,
		BindDN: row.BindDn, UserBaseDN: row.UserBaseDn, UserFilter: row.UserFilter,
		LoginAttr: row.LoginAttr, EmailAttr: row.EmailAttr, EmailFallbackAttr: deref(row.EmailFallbackAttr),
		NameAttr: row.NameAttr, GivenNameAttr: deref(row.GivenNameAttr), SurnameAttr: deref(row.SurnameAttr),
		IDAttr: row.IDAttr, GroupsAttr: row.GroupsAttr, NestedGroups: row.NestedGroups,
		DefaultRole: row.DefaultRole, RequiredGroupDN: deref(row.RequiredGroupDn),
		AutoProvision: row.AutoProvision, SyncRoles: row.SyncRoles,
		LoginLabel: apiconv.ParseText(row.LoginLabel), UsernameHint: apiconv.ParseText(row.UsernameHint),
		RoleMappings: []RoleMapping{},
	}
	_ = json.Unmarshal(row.RoleMappings, &c.RoleMappings)
	return c
}

func textOf(t *apigen.LocalizedText, maxLen int, field string, v *validate.Errors) apiconv.Text {
	out := apiconv.Text{}
	if t == nil {
		return out
	}
	for lang, value := range map[string]*string{"kk": t.Kk, "ru": t.Ru, "en": t.En} {
		if value == nil {
			continue
		}
		s := validate.Clean(*value, false)
		if s == "" {
			continue
		}
		if utf8.RuneCountInString(s) > maxLen {
			v.Add(field+"."+lang, validate.CodeMaxLength, map[string]any{"count": maxLen})
		}
		out[lang] = s
	}
	return out
}

func textAPI(t apiconv.Text) *apigen.LocalizedText {
	if len(t) == 0 {
		return nil
	}
	api := t.API()
	return &api
}

var attrPattern = regexp.MustCompile(`^(?:[A-Za-z][A-Za-z0-9-]{0,63}|[012](?:\.\d+)+)$`)

// Rules decide what a saved configuration may contain.
type Rules struct {
	// PlainAllowed accepts ldap:// without StartTLS (never in production).
	PlainAllowed bool
}

// parseInput validates settings from the API (for saving or a check).
// Errors name the fields of DirectorySettingsUpdate.
func parseInput(in apigen.DirectorySettingsUpdate, rules Rules) (Config, error) {
	var v validate.Errors
	c := Config{
		Enabled: in.Enabled, Preset: string(in.Preset), StartTLS: in.StartTls,
		NestedGroups: in.NestedGroups, AutoProvision: in.AutoProvision, SyncRoles: in.SyncRoles,
		DefaultRole: string(in.DefaultRole), RoleMappings: []RoleMapping{},
	}
	if _, ok := PresetNamed(c.Preset); !ok {
		v.Add("preset", validate.CodeInvalid, nil)
	}

	raw := strings.TrimSpace(in.Url)
	c.URLs = splitURLs(raw)
	switch {
	case raw == "":
		v.Add("url", validate.CodeRequired, nil)
	case len(raw) > maxURLLength:
		v.Add("url", validate.CodeMaxLength, map[string]any{"count": maxURLLength})
	case len(c.URLs) > maxURLs:
		v.Add("url", validate.CodeMaxItems, map[string]any{"count": maxURLs})
	default:
		for _, u := range c.URLs {
			scheme, ok := checkURL(u)
			if !ok {
				v.Add("url", codeLDAPURL, nil)
				break
			}
			if scheme == "ldap" && !c.StartTLS && !rules.PlainAllowed {
				v.Add("url", codePlainLDAP, nil)
				break
			}
		}
	}
	timeout := in.TimeoutSeconds
	v.IntRange("timeout_seconds", &timeout, 1, 60)
	c.Timeout = time.Duration(in.TimeoutSeconds) * time.Second

	if ca := strings.TrimSpace(deref(in.CaCertPem)); ca != "" {
		if len(ca) > maxCALength || !validPEM(ca) {
			v.Add("ca_cert_pem", codePEM, nil)
		}
		c.CACertPEM = ca
	}

	c.BindDN = checkDN(&v, "bind_dn", in.BindDn, true)
	c.UserBaseDN = checkDN(&v, "user_base_dn", in.UserBaseDn, true)
	c.RequiredGroupDN = checkDN(&v, "required_group_dn", deref(in.RequiredGroupDn), false)

	c.UserFilter = strings.TrimSpace(in.UserFilter)
	switch {
	case c.UserFilter == "":
		v.Add("user_filter", validate.CodeRequired, nil)
	case len(c.UserFilter) > maxFilterLength:
		v.Add("user_filter", validate.CodeMaxLength, map[string]any{"count": maxFilterLength})
	case !strings.Contains(c.UserFilter, Placeholder):
		v.Add("user_filter", codePlaceholder, nil)
	default:
		if _, err := ldap.CompileFilter(UserFilter(c.UserFilter, "probe")); err != nil {
			v.Add("user_filter", codeLDAPFilter, nil)
		}
	}

	c.LoginAttr = checkAttr(&v, "login_attr", in.LoginAttr, true)
	c.EmailAttr = checkAttr(&v, "email_attr", in.EmailAttr, true)
	c.EmailFallbackAttr = checkAttr(&v, "email_fallback_attr", deref(in.EmailFallbackAttr), false)
	c.NameAttr = checkAttr(&v, "name_attr", in.NameAttr, true)
	c.GivenNameAttr = checkAttr(&v, "given_name_attr", deref(in.GivenNameAttr), false)
	c.SurnameAttr = checkAttr(&v, "surname_attr", deref(in.SurnameAttr), false)
	c.IDAttr = checkAttr(&v, "id_attr", in.IdAttr, true)
	c.GroupsAttr = checkAttr(&v, "groups_attr", in.GroupsAttr, true)

	if !validRole(c.DefaultRole) {
		v.Add("default_role", validate.CodeInvalid, nil)
	}
	if len(in.RoleMappings) > maxMappings {
		v.Add("role_mappings", validate.CodeMaxItems, map[string]any{"count": maxMappings})
	}
	for i, m := range in.RoleMappings {
		if i >= maxMappings {
			break
		}
		field := "role_mappings." + strconv.Itoa(i)
		dn := checkDN(&v, field+".group_dn", m.GroupDn, true)
		if !validRole(string(m.Role)) {
			v.Add(field+".role", validate.CodeInvalid, nil)
		}
		// A group may give several roles, but each pair once.
		if dn != "" && slices.ContainsFunc(c.RoleMappings, func(r RoleMapping) bool {
			return r.Role == string(m.Role) && sameDN(r.GroupDN, dn)
		}) {
			v.Add(field+".group_dn", codeDuplicate, nil)
		}
		c.RoleMappings = append(c.RoleMappings, RoleMapping{GroupDN: dn, Role: string(m.Role)})
	}

	c.LoginLabel = textOf(in.LoginLabel, maxLabelLength, "login_label", &v)
	c.UsernameHint = textOf(in.UsernameHint, maxHintLength, "username_hint", &v)
	if in.BindPassword != nil {
		if len(*in.BindPassword) > maxPasswordBytes {
			v.Add("bind_password", validate.CodeMaxLength, map[string]any{"count": maxPasswordBytes})
		}
		c.BindPassword = *in.BindPassword
	}
	if err := v.Err(); err != nil {
		return c, err
	}
	return c, nil
}

// checkURL accepts ldap://host[:port] and ldaps://host[:port] only.
func checkURL(raw string) (string, bool) {
	u, err := url.Parse(raw)
	if err != nil || u.User != nil || u.RawQuery != "" || u.Fragment != "" || u.Opaque != "" {
		return "", false
	}
	scheme := strings.ToLower(u.Scheme)
	if (scheme != "ldap" && scheme != "ldaps") || u.Hostname() == "" || (u.Path != "" && u.Path != "/") {
		return "", false
	}
	if p := u.Port(); p != "" {
		n, err := strconv.Atoi(p)
		if err != nil || n < 1 || n > 65535 {
			return "", false
		}
	}
	return scheme, true
}

func validPEM(s string) bool {
	rest := []byte(s)
	found := false
	for {
		var block *pem.Block
		block, rest = pem.Decode(rest)
		if block == nil {
			break
		}
		if block.Type != "CERTIFICATE" {
			return false
		}
		if _, err := x509.ParseCertificate(block.Bytes); err != nil {
			return false
		}
		found = true
	}
	return found && strings.TrimSpace(string(rest)) == ""
}

func checkDN(v *validate.Errors, field, raw string, required bool) string {
	s := strings.TrimSpace(raw)
	switch {
	case s == "" && required:
		v.Add(field, validate.CodeRequired, nil)
	case s == "":
	case len(s) > maxDNLength:
		v.Add(field, validate.CodeMaxLength, map[string]any{"count": maxDNLength})
	default:
		if dn, err := ldap.ParseDN(s); err != nil || len(dn.RDNs) == 0 {
			v.Add(field, codeLDAPDN, nil)
		}
	}
	return s
}

func checkAttr(v *validate.Errors, field, raw string, required bool) string {
	s := strings.TrimSpace(raw)
	switch {
	case s == "" && required:
		v.Add(field, validate.CodeRequired, nil)
	case s == "":
	case !attrPattern.MatchString(s):
		v.Add(field, codeLDAPAttribute, nil)
	}
	return s
}

func validRole(r string) bool { return authz.Role(r).Valid() }

// sameDN compares DNs as the directory does (attribute names and values
// without regard to case); unparsable ones compare as text.
func sameDN(a, b string) bool {
	da, errA := ldap.ParseDN(a)
	db, errB := ldap.ParseDN(b)
	if errA != nil || errB != nil {
		return strings.EqualFold(strings.TrimSpace(a), strings.TrimSpace(b))
	}
	return da.EqualFold(db)
}

// UserFilter puts the login, escaped, into the filter: a login cannot
// change the structure of the filter (`*)(uid=*` stays a literal value).
func UserFilter(filter, login string) string {
	return strings.ReplaceAll(filter, Placeholder, ldap.EscapeFilter(login))
}

// Plain reports whether the configuration sends passwords unencrypted.
func (c Config) Plain() bool {
	for _, u := range c.URLs {
		if scheme, _ := checkURL(u); scheme == "ldap" && !c.StartTLS {
			return true
		}
	}
	return false
}

// ManagedRoles are the roles the role mappings give.
func (c Config) ManagedRoles() []string {
	out := []string{}
	for _, m := range c.RoleMappings {
		if !slices.Contains(out, m.Role) {
			out = append(out, m.Role)
		}
	}
	return out
}

// RolesFor maps the groups of a user to roles: the roles of matching
// mappings, else the default role. It also reports whether the default was
// used.
func (c Config) RolesFor(groups []string) ([]string, bool) {
	var roles []string
	for _, m := range c.RoleMappings {
		if slices.Contains(roles, m.Role) {
			continue
		}
		if slices.ContainsFunc(groups, func(g string) bool { return sameDN(g, m.GroupDN) }) {
			roles = append(roles, m.Role)
		}
	}
	if len(roles) == 0 {
		return []string{c.DefaultRole}, true
	}
	ordered := make([]string, 0, len(roles))
	for _, r := range authz.AllRoles {
		if slices.Contains(roles, string(r)) {
			ordered = append(ordered, string(r))
		}
	}
	return ordered, false
}

// InGroup reports whether the groups contain the DN.
func InGroup(groups []string, dn string) bool {
	return slices.ContainsFunc(groups, func(g string) bool { return sameDN(g, dn) })
}

// connectionKey is what a connection test checks; a change of it needs a
// new test before sign-in is turned on.
func (c Config) connectionKey() []byte {
	key, _ := json.Marshal([]any{
		c.URLs, c.StartTLS, c.CACertPEM, c.BindDN, c.BindPassword, c.UserBaseDN,
	})
	return key
}

package directory

import (
	"crypto/tls"
	"crypto/x509"
	"encoding/hex"
	"errors"
	"fmt"
	"net"
	"net/url"
	"regexp"
	"slices"
	"strings"
	"syscall"
	"time"
	"unicode"
	"unicode/utf8"

	"github.com/go-ldap/ldap/v3"

	"github.com/yelnurq/startify/backend/internal/apigen"
)

// Steps of a connection check and their statuses.
const (
	stepConnect = apigen.DirectoryTestStepStepConnect
	stepTLS     = apigen.DirectoryTestStepStepTls
	stepBind    = apigen.DirectoryTestStepStepBind
	stepSearch  = apigen.DirectoryTestStepStepSearch
	stepServer  = apigen.DirectoryTestStepStepServer

	statusOK      = apigen.DirectoryTestStepStatusOk
	statusWarning = apigen.DirectoryTestStepStatusWarning
	statusFailed  = apigen.DirectoryTestStepStatusFailed
	statusSkipped = apigen.DirectoryTestStepStatusSkipped
)

// Reasons of a failed or questionable step (directory:reasons.* of the web client).
const (
	codeDNS                = "dns"
	codeRefused            = "refused"
	codeTimeout            = "timeout"
	codeUnreachable        = "unreachable"
	codeUnknownAuthority   = "tls_unknown_authority"
	codeHostname           = "tls_hostname"
	codeExpired            = "tls_expired"
	codeExpiring           = "tls_expiring"
	codeHandshake          = "tls_handshake"
	codeStartTLS           = "start_tls_failed"
	codePlain              = "plain"
	codePlainNotAllowed    = "plain_not_allowed"
	codeInvalidCredentials = "invalid_credentials" //nolint:gosec // G101: a reason code, not a credential
	codePasswordMissing    = "bind_password_missing"
	codeNoSuchObject       = "no_such_object"
	codeInsufficientAccess = "insufficient_access"
	codeNoEntries          = "no_entries"
	codeUnwilling          = "unwilling_to_perform"
	codeStrongAuth         = "strong_auth_required"
	codeReferral           = "referral"
	codeProtocol           = "protocol"
	codeUnknown            = "unknown"
)

// Codes of a failed user lookup.
const (
	lookupNotFound   = "not_found"
	lookupAmbiguous  = "ambiguous"
	lookupConnection = "connection"
)

var (
	errNotFound  = errors.New("directory: no entry matches the login")
	errAmbiguous = errors.New("directory: several entries match the login")
)

// adActiveDirectoryCapability is announced in the RootDSE of Active Directory.
const adActiveDirectoryCapability = "1.2.840.113556.1.4.800"

// matchingRuleInChain makes Active Directory follow nested group membership
// (LDAP_MATCHING_RULE_IN_CHAIN).
const matchingRuleInChain = "1.2.840.113556.1.4.1941"

// expiringWithin warns about a server certificate that expires soon.
const expiringWithin = 30 * 24 * time.Hour

// Bounds of group resolution, so a cycle or a huge tree cannot stall sign-in.
const (
	maxGroups     = 500
	maxGroupDepth = 8
)

// recorder collects the steps of a check; nil records nothing (sign-in).
type recorder struct {
	steps []apigen.DirectoryTestStep
}

func (r *recorder) add(s apigen.DirectoryTestStep) {
	if r == nil {
		return
	}
	r.steps = append(r.steps, s)
}

func (r *recorder) failed() bool {
	return r != nil && slices.ContainsFunc(r.steps, func(s apigen.DirectoryTestStep) bool { return s.Status == statusFailed })
}

func step(name apigen.DirectoryTestStepStep, status apigen.DirectoryTestStepStatus, code, detail, server string) apigen.DirectoryTestStep {
	return apigen.DirectoryTestStep{Step: name, Status: status, Code: ptr(code), Detail: ptr(detail), Server: ptr(server)}
}

// stepError is a failure of a step with its reason.
type stepError struct {
	step apigen.DirectoryTestStepStep
	code string
	err  error
}

func (e *stepError) Error() string {
	return fmt.Sprintf("directory: %s: %s: %v", e.step, e.code, e.err)
}
func (e *stepError) Unwrap() error { return e.err }

// session is a connection to one server of the directory.
type session struct {
	conn *ldap.Conn
	url  string
	cfg  Config
}

func (s *session) close() { _ = s.conn.Close() }

// tlsConfig trusts the public authorities of the system and the
// university's own certification authority; the server name is always
// verified.
func tlsConfig(host, caPEM string) *tls.Config {
	roots, err := x509.SystemCertPool()
	if err != nil || roots == nil {
		roots = x509.NewCertPool()
	}
	if caPEM != "" {
		roots.AppendCertsFromPEM([]byte(caPEM))
	}
	return &tls.Config{ServerName: host, RootCAs: roots, MinVersion: tls.VersionTLS12}
}

// open connects to the first server that answers, in the order of the
// addresses, with TLS (ldaps:// or StartTLS) unless plain connections are
// allowed. Each failed address and the connection made are recorded.
func open(c Config, rules Rules, rec *recorder) (*session, error) {
	var last error
	for _, raw := range c.URLs {
		u, err := url.Parse(raw)
		if err != nil {
			last = &stepError{step: stepConnect, code: codeProtocol, err: err}
			rec.add(step(stepConnect, statusFailed, codeProtocol, err.Error(), raw))
			continue
		}
		scheme, host := strings.ToLower(u.Scheme), u.Hostname()
		plain := scheme == "ldap" && !c.StartTLS
		if plain && !rules.PlainAllowed {
			last = &stepError{step: stepTLS, code: codePlainNotAllowed, err: errors.New("unencrypted LDAP is not allowed in production")}
			rec.add(step(stepTLS, statusFailed, codePlainNotAllowed, "", raw))
			continue
		}
		cfg := tlsConfig(host, c.CACertPEM)
		conn, err := ldap.DialURL(raw, ldap.DialWithDialer(&net.Dialer{Timeout: c.Timeout}), ldap.DialWithTLSConfig(cfg))
		if err != nil {
			code := classifyNetwork(err)
			if isTLSCode(code) {
				rec.add(step(stepConnect, statusOK, "", "", raw))
				rec.add(step(stepTLS, statusFailed, code, err.Error(), raw))
				last = &stepError{step: stepTLS, code: code, err: err}
			} else {
				rec.add(step(stepConnect, statusFailed, code, err.Error(), raw))
				last = &stepError{step: stepConnect, code: code, err: err}
			}
			continue
		}
		conn.SetTimeout(c.Timeout)
		rec.add(step(stepConnect, statusOK, "", "", raw))
		if scheme == "ldap" && c.StartTLS {
			if err := conn.StartTLS(cfg); err != nil {
				_ = conn.Close()
				code := classifyNetwork(err)
				if !isTLSCode(code) {
					code = codeStartTLS
				}
				rec.add(step(stepTLS, statusFailed, code, err.Error(), raw))
				last = &stepError{step: stepTLS, code: code, err: err}
				continue
			}
		}
		if plain {
			rec.add(step(stepTLS, statusWarning, codePlain, "", raw))
		} else {
			rec.add(tlsStep(conn, raw))
		}
		return &session{conn: conn, url: raw, cfg: c}, nil
	}
	if last == nil {
		last = &stepError{step: stepConnect, code: codeProtocol, err: errors.New("no address")}
	}
	return nil, last
}

// tlsStep describes the certificate the server presented.
func tlsStep(conn *ldap.Conn, server string) apigen.DirectoryTestStep {
	s := step(stepTLS, statusOK, "", "", server)
	state, ok := conn.TLSConnectionState()
	if !ok || len(state.PeerCertificates) == 0 {
		return s
	}
	cert := state.PeerCertificates[0]
	s.Certificate = &apigen.DirectoryCertificate{
		Subject: cert.Subject.String(), Issuer: cert.Issuer.String(), NotAfter: cert.NotAfter, DnsNames: cert.DNSNames,
	}
	if cert.DNSNames == nil {
		s.Certificate.DnsNames = []string{}
	}
	if time.Until(cert.NotAfter) < expiringWithin {
		s.Status, s.Code = statusWarning, ptr(codeExpiring)
	}
	return s
}

func isTLSCode(code string) bool { return strings.HasPrefix(code, "tls_") }

// classifyNetwork names the reason a connection failed.
func classifyNetwork(err error) string {
	var dnsErr *net.DNSError
	var unknown x509.UnknownAuthorityError
	var hostname x509.HostnameError
	var invalid x509.CertificateInvalidError
	var record tls.RecordHeaderError
	var errno syscall.Errno
	var netErr net.Error
	msg := strings.ToLower(err.Error())
	switch {
	case errors.As(err, &dnsErr):
		return codeDNS
	case errors.As(err, &unknown) || strings.Contains(msg, "certificate signed by unknown authority"):
		return codeUnknownAuthority
	case errors.As(err, &hostname) || strings.Contains(msg, "certificate is valid for") || strings.Contains(msg, "doesn't contain any ip sans"):
		return codeHostname
	case (errors.As(err, &invalid) && invalid.Reason == x509.Expired) || strings.Contains(msg, "has expired or is not yet valid"):
		return codeExpired
	case errors.As(err, &record) || strings.Contains(msg, "tls:") || strings.Contains(msg, "x509:"):
		return codeHandshake
	// 10061 is WSAECONNREFUSED: Windows reports its own number.
	case (errors.As(err, &errno) && (errno == syscall.ECONNREFUSED || errno == 10061)) || strings.Contains(msg, "refused"):
		return codeRefused
	case (errors.As(err, &netErr) && netErr.Timeout()) || strings.Contains(msg, "timed out") || strings.Contains(msg, "timeout"):
		return codeTimeout
	case strings.Contains(msg, "unreachable") || strings.Contains(msg, "no route to host"):
		return codeUnreachable
	case strings.Contains(msg, "eof") || strings.Contains(msg, "reset by peer") || strings.Contains(msg, "malformed"):
		return codeProtocol
	}
	return codeUnknown
}

// classifyResult names the reason an operation was refused.
func classifyResult(err error) string {
	switch {
	case ldap.IsErrorWithCode(err, ldap.LDAPResultInvalidCredentials):
		return codeInvalidCredentials
	case ldap.IsErrorWithCode(err, ldap.LDAPResultNoSuchObject):
		return codeNoSuchObject
	case ldap.IsErrorAnyOf(err, ldap.LDAPResultInsufficientAccessRights, ldap.LDAPResultInappropriateAuthentication):
		return codeInsufficientAccess
	case ldap.IsErrorWithCode(err, ldap.LDAPResultUnwillingToPerform):
		return codeUnwilling
	case ldap.IsErrorAnyOf(err, ldap.LDAPResultStrongAuthRequired, ldap.LDAPResultConfidentialityRequired):
		return codeStrongAuth
	case ldap.IsErrorWithCode(err, ldap.LDAPResultReferral):
		return codeReferral
	case ldap.IsErrorWithCode(err, ldap.LDAPResultTimeLimitExceeded):
		return codeTimeout
	case ldap.IsErrorWithCode(err, ldap.ErrorNetwork):
		return classifyNetwork(err)
	}
	return codeUnknown
}

// bindService signs the session in as the service account. An empty
// password is refused here: the server would take it for an anonymous bind
// and report success.
func (s *session) bindService(password string, rec *recorder) error {
	if s.cfg.BindDN == "" || password == "" {
		rec.add(step(stepBind, statusFailed, codePasswordMissing, "", s.url))
		return &stepError{step: stepBind, code: codePasswordMissing, err: errors.New("no bind password")}
	}
	if err := s.conn.Bind(s.cfg.BindDN, password); err != nil {
		code := classifyResult(err)
		rec.add(step(stepBind, statusFailed, code, err.Error(), s.url))
		return &stepError{step: stepBind, code: code, err: err}
	}
	rec.add(step(stepBind, statusOK, "", "", s.url))
	return nil
}

func (s *session) timeLimit() int {
	return max(1, int(s.cfg.Timeout/time.Second))
}

// checkBase makes sure the base DN exists and the filter finds someone.
func (s *session) checkBase(rec *recorder) error {
	base := ldap.NewSearchRequest(s.cfg.UserBaseDN, ldap.ScopeBaseObject, ldap.NeverDerefAliases, 1, s.timeLimit(), false,
		"(objectClass=*)", []string{"1.1"}, nil)
	if _, err := s.conn.Search(base); err != nil {
		code := classifyResult(err)
		rec.add(step(stepSearch, statusFailed, code, err.Error(), s.url))
		return &stepError{step: stepSearch, code: code, err: err}
	}
	// The placeholder becomes a wildcard here, in a filter of the
	// administrator: no login takes part in it.
	probe := strings.ReplaceAll(s.cfg.UserFilter, Placeholder, "*")
	req := ldap.NewSearchRequest(s.cfg.UserBaseDN, ldap.ScopeWholeSubtree, ldap.NeverDerefAliases, 1, s.timeLimit(), false,
		probe, []string{"1.1"}, nil)
	res, err := s.conn.Search(req)
	if err != nil && !ldap.IsErrorWithCode(err, ldap.LDAPResultSizeLimitExceeded) {
		code := classifyResult(err)
		rec.add(step(stepSearch, statusFailed, code, err.Error(), s.url))
		return &stepError{step: stepSearch, code: code, err: err}
	}
	count := 0
	if res != nil {
		count = min(len(res.Entries), 1)
	}
	st := step(stepSearch, statusOK, "", "", s.url)
	st.Count = &count
	if count == 0 {
		st.Status, st.Code = statusWarning, ptr(codeNoEntries)
	}
	rec.add(st)
	return nil
}

// describeServer reads the RootDSE: the kind of server and its naming
// contexts (a hint for the base DN). A server that hides it is not an error.
func (s *session) describeServer(rec *recorder) {
	req := ldap.NewSearchRequest("", ldap.ScopeBaseObject, ldap.NeverDerefAliases, 1, s.timeLimit(), false, "(objectClass=*)",
		[]string{"vendorName", "vendorVersion", "namingContexts", "defaultNamingContext", "supportedCapabilities", "objectClass", "dnsHostName"}, nil)
	res, err := s.conn.Search(req)
	if err != nil || len(res.Entries) == 0 {
		detail := ""
		if err != nil {
			detail = err.Error()
		}
		rec.add(step(stepServer, statusWarning, codeUnknown, detail, s.url))
		return
	}
	e := res.Entries[0]
	vendor := "LDAP"
	switch {
	case slices.Contains(e.GetAttributeValues("supportedCapabilities"), adActiveDirectoryCapability):
		vendor = "Microsoft Active Directory"
		if h := e.GetAttributeValue("dnsHostName"); h != "" {
			vendor += " (" + h + ")"
		}
	case e.GetAttributeValue("vendorName") != "":
		vendor = strings.TrimSpace(e.GetAttributeValue("vendorName") + " " + e.GetAttributeValue("vendorVersion"))
	case slices.ContainsFunc(e.GetAttributeValues("objectClass"), func(c string) bool { return strings.EqualFold(c, "OpenLDAProotDSE") }):
		vendor = "OpenLDAP"
	}
	contexts := []string{}
	if d := e.GetAttributeValue("defaultNamingContext"); d != "" {
		contexts = append(contexts, d)
	}
	for _, nc := range e.GetAttributeValues("namingContexts") {
		if !slices.ContainsFunc(contexts, func(c string) bool { return strings.EqualFold(c, nc) }) {
			contexts = append(contexts, nc)
		}
	}
	st := step(stepServer, statusOK, "", "", s.url)
	st.Vendor, st.NamingContexts = &vendor, &contexts
	rec.add(st)
}

// Entry is a user found in the directory, mapped to what Qadam keeps.
type Entry struct {
	DN       string
	Login    string
	Email    string
	FullName string
	Subject  string
	Groups   []string
}

func (s *session) userAttributes() []string {
	c := s.cfg
	attrs := []string{}
	for _, a := range []string{c.LoginAttr, c.EmailAttr, c.EmailFallbackAttr, c.NameAttr, c.GivenNameAttr, c.SurnameAttr, c.IDAttr, c.GroupsAttr} {
		if a != "" && !slices.ContainsFunc(attrs, func(x string) bool { return strings.EqualFold(x, a) }) {
			attrs = append(attrs, a)
		}
	}
	return attrs
}

// findUser looks the login up with the user filter. At most two entries are
// asked for: two mean the login is ambiguous and nobody signs in with it.
func (s *session) findUser(login string) (*ldap.Entry, error) {
	req := ldap.NewSearchRequest(s.cfg.UserBaseDN, ldap.ScopeWholeSubtree, ldap.NeverDerefAliases, 2, s.timeLimit(), false,
		UserFilter(s.cfg.UserFilter, login), s.userAttributes(), nil)
	res, err := s.conn.Search(req)
	switch {
	case ldap.IsErrorWithCode(err, ldap.LDAPResultSizeLimitExceeded):
		return nil, errAmbiguous
	case err != nil:
		return nil, &stepError{step: stepSearch, code: classifyResult(err), err: err}
	case len(res.Entries) == 0:
		return nil, errNotFound
	case len(res.Entries) > 1:
		return nil, errAmbiguous
	}
	return res.Entries[0], nil
}

// mapEntry reads the attributes of a user entry.
func mapEntry(c Config, e *ldap.Entry) Entry {
	out := Entry{
		DN:     e.DN,
		Login:  strings.TrimSpace(e.GetEqualFoldAttributeValue(c.LoginAttr)),
		Groups: slices.Clone(e.GetEqualFoldAttributeValues(c.GroupsAttr)),
	}
	if out.Groups == nil {
		out.Groups = []string{}
	}
	out.Email = mailOf(e.GetEqualFoldAttributeValue(c.EmailAttr))
	if out.Email == "" && c.EmailFallbackAttr != "" {
		out.Email = mailOf(e.GetEqualFoldAttributeValue(c.EmailFallbackAttr))
	}
	out.FullName = strings.Join(strings.Fields(e.GetEqualFoldAttributeValue(c.NameAttr)), " ")
	if out.FullName == "" {
		parts := []string{}
		for _, a := range []string{c.GivenNameAttr, c.SurnameAttr} {
			if a != "" {
				if v := strings.TrimSpace(e.GetEqualFoldAttributeValue(a)); v != "" {
					parts = append(parts, v)
				}
			}
		}
		out.FullName = strings.Join(parts, " ")
	}
	if out.FullName == "" {
		out.FullName = out.Login
	}
	out.Subject = subjectOf(c.IDAttr, e)
	return out
}

// mailOf accepts an address that looks like one.
func mailOf(v string) string {
	v = strings.ToLower(strings.TrimSpace(v))
	local, domain, ok := strings.Cut(v, "@")
	if !ok || local == "" || !strings.Contains(domain, ".") || len(v) > 254 ||
		strings.ContainsFunc(v, func(r rune) bool { return unicode.IsSpace(r) || unicode.IsControl(r) }) || strings.Contains(domain, "@") {
		return ""
	}
	return v
}

// subjectOf returns the permanent identifier of an entry: objectGUID in its
// canonical form, other identifiers as text, the DN when the entry has none.
func subjectOf(attr string, e *ldap.Entry) string {
	raw := e.GetEqualFoldRawAttributeValue(attr)
	switch {
	case len(raw) == 0:
		return "dn:" + strings.ToLower(e.DN)
	case strings.EqualFold(attr, "objectGUID") && len(raw) == 16:
		return FormatGUID(raw)
	case utf8.Valid(raw) && !strings.ContainsFunc(string(raw), unicode.IsControl):
		return strings.ToLower(strings.TrimSpace(string(raw)))
	}
	return hex.EncodeToString(raw)
}

// FormatGUID writes a binary objectGUID of Active Directory in the canonical
// form: the first three groups are little-endian on the wire.
func FormatGUID(b []byte) string {
	if len(b) != 16 {
		return hex.EncodeToString(b)
	}
	return fmt.Sprintf("%02x%02x%02x%02x-%02x%02x-%02x%02x-%02x%02x-%02x%02x%02x%02x%02x%02x",
		b[3], b[2], b[1], b[0], b[5], b[4], b[7], b[6], b[8], b[9], b[10], b[11], b[12], b[13], b[14], b[15])
}

// groups returns the groups of a user, nested ones too when enabled.
func (s *session) groups(entry Entry) []string {
	if !s.cfg.NestedGroups {
		return entry.Groups
	}
	if s.cfg.Preset == PresetActiveDirectory {
		if all, err := s.groupsInChain(entry.DN); err == nil {
			return mergeGroups(entry.Groups, all)
		}
	}
	return s.groupsBreadthFirst(entry.Groups)
}

// groupsInChain asks Active Directory for every group the user belongs to,
// directly or through other groups.
func (s *session) groupsInChain(userDN string) ([]string, error) {
	base := domainOf(userDN)
	if base == "" {
		return nil, errors.New("directory: no domain in the DN")
	}
	filter := "(member:" + matchingRuleInChain + ":=" + ldap.EscapeFilter(userDN) + ")"
	req := ldap.NewSearchRequest(base, ldap.ScopeWholeSubtree, ldap.NeverDerefAliases, maxGroups, s.timeLimit(), false,
		filter, []string{"1.1"}, nil)
	res, err := s.conn.Search(req)
	if err != nil && !ldap.IsErrorWithCode(err, ldap.LDAPResultSizeLimitExceeded) {
		return nil, err
	}
	out := make([]string, 0, len(res.Entries))
	for _, e := range res.Entries {
		out = append(out, e.DN)
	}
	return out, nil
}

// groupsBreadthFirst follows the membership attribute of groups (OpenLDAP
// with the memberOf overlay, FreeIPA): a group that is a member of another
// group carries it in the same attribute as users do.
func (s *session) groupsBreadthFirst(direct []string) []string {
	all := slices.Clone(direct)
	frontier := slices.Clone(direct)
	for depth := 0; depth < maxGroupDepth && len(frontier) > 0 && len(all) < maxGroups; depth++ {
		var next []string
		for _, dn := range frontier {
			req := ldap.NewSearchRequest(dn, ldap.ScopeBaseObject, ldap.NeverDerefAliases, 1, s.timeLimit(), false,
				"(objectClass=*)", []string{s.cfg.GroupsAttr}, nil)
			res, err := s.conn.Search(req)
			if err != nil || len(res.Entries) == 0 {
				continue
			}
			for _, parent := range res.Entries[0].GetEqualFoldAttributeValues(s.cfg.GroupsAttr) {
				if !InGroup(all, parent) && len(all) < maxGroups {
					all = append(all, parent)
					next = append(next, parent)
				}
			}
		}
		frontier = next
	}
	return all
}

func mergeGroups(a, b []string) []string {
	out := slices.Clone(a)
	for _, dn := range b {
		if !InGroup(out, dn) {
			out = append(out, dn)
		}
	}
	return out
}

// domainOf keeps the DC= components of a DN: the root of an Active
// Directory domain.
func domainOf(dn string) string {
	parsed, err := ldap.ParseDN(dn)
	if err != nil {
		return ""
	}
	var parts []string
	for _, rdn := range parsed.RDNs {
		if len(rdn.Attributes) == 1 && strings.EqualFold(rdn.Attributes[0].Type, "DC") {
			parts = append(parts, "DC="+ldap.EscapeDN(rdn.Attributes[0].Value))
		}
	}
	return strings.Join(parts, ",")
}

// Reasons Active Directory gives for a refused bind (the "data" code of the
// diagnostic message). Only codes it returns after checking the password
// are told apart; a locked account reads as a wrong password, so the answer
// reveals nothing to someone who does not know it.
var adDataCode = regexp.MustCompile(`(?i)\bdata ([0-9a-f]{3,8})\b`)

const (
	bindWrong    = "wrong"
	bindExpired  = "expired"
	bindDisabled = "disabled"
	bindDenied   = "denied"
)

// bindRefusal names why a user bind was refused.
func bindRefusal(err error) string {
	if !ldap.IsErrorWithCode(err, ldap.LDAPResultInvalidCredentials) {
		return ""
	}
	m := adDataCode.FindStringSubmatch(err.Error())
	if m == nil {
		return bindWrong
	}
	switch strings.ToLower(m[1]) {
	case "532", "773":
		return bindExpired
	case "533", "701":
		return bindDisabled
	case "530", "531":
		return bindDenied
	}
	return bindWrong
}

// NormalizeLogin cleans the login a person typed. With Active Directory
// `DOMAIN\user` and `user@domain` become `user`: the part before @ or after
// the backslash must be the sAMAccountName.
func NormalizeLogin(preset, raw string) string {
	s := strings.TrimFunc(raw, func(r rune) bool { return unicode.IsSpace(r) || unicode.IsControl(r) })
	if strings.ContainsFunc(s, unicode.IsControl) {
		return ""
	}
	if preset == PresetActiveDirectory {
		if i := strings.LastIndex(s, `\`); i >= 0 {
			s = s[i+1:]
		}
		if i := strings.Index(s, "@"); i >= 0 {
			s = s[:i]
		}
	}
	return strings.TrimSpace(s)
}

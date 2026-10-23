// Package ldaptest runs integration tests against a disposable OpenLDAP: the
// image and the demo directory of docker-compose.dev.yml (deploy/ldap), with
// a certificate for localhost signed by a certification authority of the
// test. One container is shared by the tests of a process.
//
// Usage:
//
//	func TestSomething(t *testing.T) {
//		srv := ldaptest.Shared(t) // skips the test in -short mode
//		...
//	}
package ldaptest

import (
	"bytes"
	"context"
	"crypto/ecdsa"
	"crypto/elliptic"
	"crypto/rand"
	"crypto/x509"
	"crypto/x509/pkix"
	"encoding/pem"
	"fmt"
	"math/big"
	"net"
	"os"
	"path/filepath"
	"runtime"
	"sync"
	"testing"
	"time"

	"github.com/go-ldap/ldap/v3"
	"github.com/testcontainers/testcontainers-go"
	"github.com/testcontainers/testcontainers-go/wait"
)

// The image of docker-compose.dev.yml and the accounts of the demo directory.
const (
	Image         = "osixia/openldap:1.5.0"
	BaseDN        = "dc=kazutb,dc=local"
	AdminDN       = "cn=admin,dc=kazutb,dc=local"
	AdminPassword = "ldaptest-admin-password" //nolint:gosec // G101: a throwaway test container
	BindDN        = "cn=qadam-svc,ou=Service Accounts,dc=kazutb,dc=local"
	BindPassword  = "Qadam-Ldap-Bind-2026" //nolint:gosec // G101: the demo directory of deploy/ldap
)

// Server is a running OpenLDAP.
type Server struct {
	container testcontainers.Container
	host      string
	ldapPort  string
	ldapsPort string
	// CAPEM is the certificate of the authority that signed the server's.
	CAPEM string
}

// URL is the plain (StartTLS-capable) address.
func (s *Server) URL() string { return "ldap://" + net.JoinHostPort(s.host, s.ldapPort) }

// LDAPSURL is the TLS address.
func (s *Server) LDAPSURL() string { return "ldaps://" + net.JoinHostPort(s.host, s.ldapsPort) }

// Admin returns a connection bound as the directory administrator, closed
// when the test ends (to add entries a test needs).
func (s *Server) Admin(tb testing.TB) *ldap.Conn {
	tb.Helper()
	conn, err := ldap.DialURL(s.URL())
	if err != nil {
		tb.Fatal(err)
	}
	tb.Cleanup(func() { _ = conn.Close() })
	if err := conn.Bind(AdminDN, AdminPassword); err != nil {
		tb.Fatal(err)
	}
	return conn
}

var (
	once      sync.Once
	shared    *Server
	errShared error
)

// Shared starts the server once per process; the test is skipped in -short
// mode. Testcontainers removes the container when the process ends.
func Shared(tb testing.TB) *Server {
	tb.Helper()
	if testing.Short() {
		tb.Skip("integration test: needs Docker (run without -short)")
	}
	once.Do(func() { shared, errShared = Start(context.Background()) })
	if errShared != nil {
		tb.Fatalf("ldaptest: %v", errShared)
	}
	return shared
}

// Start launches a server with the LDIF files of deploy/ldap.
func Start(ctx context.Context) (*Server, error) {
	ca, crt, key, err := certificates()
	if err != nil {
		return nil, err
	}
	_, self, _, _ := runtime.Caller(0)
	ldifDir := filepath.Join(filepath.Dir(self), "..", "..", "..", "..", "deploy", "ldap")
	ldifs, err := filepath.Glob(filepath.Join(ldifDir, "*.ldif"))
	if err != nil || len(ldifs) == 0 {
		return nil, fmt.Errorf("no LDIF files in %s", ldifDir)
	}
	const certs = "/container/service/slapd/assets/certs/"
	files := []testcontainers.ContainerFile{
		{Reader: bytes.NewReader(ca), ContainerFilePath: certs + "ca.crt", FileMode: 0o644},
		{Reader: bytes.NewReader(crt), ContainerFilePath: certs + "ldap.crt", FileMode: 0o644},
		{Reader: bytes.NewReader(key), ContainerFilePath: certs + "ldap.key", FileMode: 0o644},
	}
	for _, f := range ldifs {
		content, err := os.ReadFile(f) //nolint:gosec // files of the repository
		if err != nil {
			return nil, err
		}
		files = append(files, testcontainers.ContainerFile{
			Reader: bytes.NewReader(content), ContainerFilePath: "/ldif/" + filepath.Base(f), FileMode: 0o644,
		})
	}
	c, err := testcontainers.GenericContainer(ctx, testcontainers.GenericContainerRequest{
		ContainerRequest: testcontainers.ContainerRequest{
			Image:        Image,
			ExposedPorts: []string{"389/tcp", "636/tcp"},
			Env: map[string]string{
				"LDAP_ORGANISATION":            "KazUTB test",
				"LDAP_DOMAIN":                  "kazutb.local",
				"LDAP_ADMIN_PASSWORD":          AdminPassword,
				"LDAP_CONFIG_PASSWORD":         AdminPassword,
				"LDAP_TLS_VERIFY_CLIENT":       "never",
				"LDAP_SEED_INTERNAL_LDIF_PATH": "/ldif",
			},
			Files:      files,
			WaitingFor: wait.ForLog("slapd starting").WithStartupTimeout(3 * time.Minute),
		},
		Started: true,
	})
	if err != nil {
		return nil, fmt.Errorf("start container: %w", err)
	}
	s := &Server{container: c, CAPEM: string(ca)}
	if s.host, err = c.Host(ctx); err != nil {
		return nil, err
	}
	for name, port := range map[string]*string{"389/tcp": &s.ldapPort, "636/tcp": &s.ldapsPort} {
		p, err := c.MappedPort(ctx, name)
		if err != nil {
			return nil, err
		}
		*port = p.Port()
	}
	// The log line comes before the listener is ready on some hosts.
	deadline := time.Now().Add(time.Minute)
	for {
		conn, err := ldap.DialURL(s.URL())
		if err == nil {
			err = conn.Bind(BindDN, BindPassword)
			_ = conn.Close()
		}
		if err == nil {
			return s, nil
		}
		if time.Now().After(deadline) {
			return nil, fmt.Errorf("directory not ready: %w", err)
		}
		time.Sleep(200 * time.Millisecond)
	}
}

// certificates makes a certification authority and a certificate for
// localhost signed by it (PEM).
func certificates() (caPEM, crtPEM, keyPEM []byte, err error) {
	caKey, err := ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
	if err != nil {
		return nil, nil, nil, err
	}
	now := time.Now()
	caTpl := &x509.Certificate{
		SerialNumber: big.NewInt(1), Subject: pkix.Name{CommonName: "Qadam test CA"},
		NotBefore: now.Add(-time.Hour), NotAfter: now.AddDate(1, 0, 0),
		IsCA: true, BasicConstraintsValid: true, KeyUsage: x509.KeyUsageCertSign | x509.KeyUsageCRLSign,
	}
	caDER, err := x509.CreateCertificate(rand.Reader, caTpl, caTpl, &caKey.PublicKey, caKey)
	if err != nil {
		return nil, nil, nil, err
	}
	caCert, err := x509.ParseCertificate(caDER)
	if err != nil {
		return nil, nil, nil, err
	}
	key, err := ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
	if err != nil {
		return nil, nil, nil, err
	}
	tpl := &x509.Certificate{
		SerialNumber: big.NewInt(2), Subject: pkix.Name{CommonName: "localhost"},
		NotBefore: now.Add(-time.Hour), NotAfter: now.AddDate(1, 0, 0),
		DNSNames: []string{"localhost"}, IPAddresses: []net.IP{net.IPv4(127, 0, 0, 1), net.IPv6loopback},
		KeyUsage: x509.KeyUsageDigitalSignature, ExtKeyUsage: []x509.ExtKeyUsage{x509.ExtKeyUsageServerAuth},
	}
	der, err := x509.CreateCertificate(rand.Reader, tpl, caCert, &key.PublicKey, caKey)
	if err != nil {
		return nil, nil, nil, err
	}
	keyDER, err := x509.MarshalECPrivateKey(key)
	if err != nil {
		return nil, nil, nil, err
	}
	return pem.EncodeToMemory(&pem.Block{Type: "CERTIFICATE", Bytes: caDER}),
		pem.EncodeToMemory(&pem.Block{Type: "CERTIFICATE", Bytes: der}),
		pem.EncodeToMemory(&pem.Block{Type: "EC PRIVATE KEY", Bytes: keyDER}), nil
}

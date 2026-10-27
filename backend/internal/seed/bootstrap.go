package seed

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"errors"
	"fmt"
	"net/mail"
	"strings"

	"github.com/yelnurq/startify/backend/internal/auth"
	"github.com/yelnurq/startify/backend/internal/files"
	"github.com/yelnurq/startify/backend/internal/platform/db"
)

// Installation describes the first start of a real installation: the
// university's address and sign-up rules and its first administrator.
type Installation struct {
	// Domain is the host the university is served on (from the public URL).
	Domain string
	// SelfSignup lets students create accounts themselves (with a confirmed
	// e-mail); EmailDomains, when given, limits it to the university's mail.
	SelfSignup   bool
	EmailDomains []string
	AdminEmail   string
	AdminName    string
	AdminLocale  string
	// AdminPassword is generated when empty.
	AdminPassword string
}

// Bootstrap prepares KazUTB on a real installation: the university with its
// settings, address, seal and campus photo, and the first administrator
// (tenant_admin) with a confirmed e-mail. It is idempotent; running it again
// for the same administrator sets a new password, which is how a lost first
// password is recovered. It returns the administrator's password.
func Bootstrap(ctx context.Context, database *db.DB, store *files.Service, in Installation) (string, error) {
	addr, err := mail.ParseAddress(strings.TrimSpace(in.AdminEmail))
	if err != nil {
		return "", fmt.Errorf("seed: bootstrap: administrator e-mail: %w", err)
	}
	email := strings.ToLower(addr.Address)
	name := strings.TrimSpace(in.AdminName)
	if len([]rune(name)) < 2 {
		return "", errors.New("seed: bootstrap: the administrator's full name is required")
	}
	domain := strings.ToLower(strings.TrimSpace(in.Domain))
	if domain == "" || strings.ContainsAny(domain, "/: ") {
		return "", fmt.Errorf("seed: bootstrap: %q is not a host name", in.Domain)
	}
	locale := in.AdminLocale
	if locale == "" {
		locale = KazUTB.Locale
	}
	password := in.AdminPassword
	if password == "" {
		password = GeneratePassword()
	}
	if err := auth.CheckPassword(password, email, name); err != nil {
		return "", fmt.Errorf("seed: bootstrap: the administrator's password is too weak: %w", err)
	}
	hash, err := auth.HashPassword(password)
	if err != nil {
		return "", err
	}

	spec := KazUTB
	spec.Domains = []string{domain}
	spec.SelfSignup = in.SelfSignup
	spec.EmailDomains = in.EmailDomains
	kazutb, err := Tenant(ctx, database, spec)
	if err != nil {
		return "", err
	}
	if _, err := Account(ctx, database, kazutb.ID, Member{
		Email: email, FullName: name, Locale: locale, PasswordHash: hash,
		Roles: []string{"tenant_admin"}, Slug: "admin-" + randomHex(4),
	}); err != nil {
		return "", err
	}
	if err := Branding(ctx, database, store); err != nil {
		return "", err
	}
	return password, nil
}

// passwordAlphabet leaves out characters that are easy to mistype or confuse
// when a password is read from a terminal (0/O, 1/l/I).
const passwordAlphabet = "abcdefghijkmnpqrstuvwxyzABCDEFGHJKLMNPQRSTUVWXYZ23456789"

// GeneratePassword returns a random password of four groups of five
// characters (about 116 bits), easy to copy from a terminal.
func GeneratePassword() string {
	var b strings.Builder
	for i := range 20 {
		if i > 0 && i%5 == 0 {
			b.WriteByte('-')
		}
		b.WriteByte(passwordAlphabet[randomIndex(len(passwordAlphabet))])
	}
	return b.String()
}

func randomIndex(n int) int {
	// Rejection sampling keeps every character equally likely.
	limit := 256 - 256%n
	var buf [1]byte
	for {
		if _, err := rand.Read(buf[:]); err != nil {
			panic(err)
		}
		if int(buf[0]) < limit {
			return int(buf[0]) % n
		}
	}
}

func randomHex(n int) string {
	buf := make([]byte, n)
	if _, err := rand.Read(buf); err != nil {
		panic(err)
	}
	return hex.EncodeToString(buf)
}

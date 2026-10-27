package auth

import (
	"crypto/rand"
	"crypto/subtle"
	_ "embed"
	"encoding/base64"
	"errors"
	"fmt"
	"runtime"
	"strings"
	"sync"
	"unicode/utf8"

	"golang.org/x/crypto/argon2"

	"github.com/yelnurq/startify/backend/internal/platform/apperr"
)

// argon2id parameters: the OWASP baseline (19 MiB, 2 passes, 1 lane).
const (
	argonMemory  = 19 * 1024
	argonTime    = 2
	argonThreads = 1
	argonKeyLen  = 32
	argonSaltLen = 16
)

// Password length limits (runes).
const (
	MinPasswordLength = 10
	MaxPasswordLength = 128
)

var b64 = base64.RawStdEncoding

// hashSlots bounds concurrent hashing: each one holds 19 MiB.
var hashSlots = make(chan struct{}, max(2, runtime.NumCPU()))

func argonKey(password, salt []byte, t, m uint32, p uint8, n uint32) []byte {
	hashSlots <- struct{}{}
	defer func() { <-hashSlots }()
	return argon2.IDKey(password, salt, t, m, p, n)
}

// HashPassword returns the PHC string of an argon2id hash.
func HashPassword(password string) (string, error) {
	salt := make([]byte, argonSaltLen)
	if _, err := rand.Read(salt); err != nil {
		return "", fmt.Errorf("auth: salt: %w", err)
	}
	key := argonKey([]byte(password), salt, argonTime, argonMemory, argonThreads, argonKeyLen)
	return fmt.Sprintf("$argon2id$v=%d$m=%d,t=%d,p=%d$%s$%s",
		argon2.Version, argonMemory, argonTime, argonThreads, b64.EncodeToString(salt), b64.EncodeToString(key)), nil
}

var errBadHash = errors.New("auth: malformed password hash")

// VerifyPassword compares a password with a PHC string in constant time.
func VerifyPassword(encoded, password string) (bool, error) {
	parts := strings.Split(encoded, "$")
	if len(parts) != 6 || parts[1] != "argon2id" {
		return false, errBadHash
	}
	var version int
	if _, err := fmt.Sscanf(parts[2], "v=%d", &version); err != nil || version != argon2.Version {
		return false, errBadHash
	}
	var m, t uint32
	var p uint8
	if _, err := fmt.Sscanf(parts[3], "m=%d,t=%d,p=%d", &m, &t, &p); err != nil || m == 0 || t == 0 || p == 0 {
		return false, errBadHash
	}
	salt, err := b64.DecodeString(parts[4])
	if err != nil {
		return false, errBadHash
	}
	want, err := b64.DecodeString(parts[5])
	if err != nil || len(want) == 0 {
		return false, errBadHash
	}
	got := argonKey([]byte(password), salt, t, m, p, uint32(len(want))) //nolint:gosec // decoded hash is 32 bytes
	return subtle.ConstantTimeCompare(got, want) == 1, nil
}

// burnPasswordCheck spends the time of a real verification, so that unknown
// e-mail addresses cannot be told apart by response time.
var burnPasswordCheck = sync.OnceValue(func() string {
	h, err := HashPassword("qadam: equalise the timing of unknown accounts")
	if err != nil {
		panic(err)
	}
	return h
})

//go:embed common_passwords.txt
var commonPasswordList string

var commonPasswords = sync.OnceValue(func() map[string]bool {
	set := map[string]bool{}
	for line := range strings.Lines(commonPasswordList) {
		if w := strings.TrimSpace(line); w != "" && !strings.HasPrefix(w, "#") {
			set[strings.ToLower(w)] = true
		}
	}
	return set
})

var errWeakPassword = apperr.New(apperr.KindValidation, "auth.password_weak",
	"use at least 10 characters that are not a common password, your name or your e-mail")

// CheckPassword enforces the password policy: length, no common or
// keyboard-pattern passwords, nothing derived from the e-mail or the name
// (NIST SP 800-63B: no composition rules, block known-bad values).
func CheckPassword(password, email, fullName string) error {
	n := utf8.RuneCountInString(password)
	if n < MinPasswordLength || n > MaxPasswordLength {
		return errWeakPassword
	}
	lower := strings.ToLower(password)
	core := strings.Trim(lower, "0123456789!@#$%^&*()_-+=.,?~ ")
	if commonPasswords()[lower] || commonPasswords()[core] || utf8.RuneCountInString(core) < 4 {
		return errWeakPassword
	}
	if distinctRunes(lower) < 4 {
		return errWeakPassword
	}
	if local, _, ok := strings.Cut(strings.ToLower(email), "@"); ok && len(local) >= 4 && strings.Contains(lower, local) {
		return errWeakPassword
	}
	for part := range strings.FieldsSeq(strings.ToLower(fullName)) {
		if utf8.RuneCountInString(part) >= 4 && strings.Contains(lower, part) && utf8.RuneCountInString(core) <= utf8.RuneCountInString(part)+4 {
			return errWeakPassword
		}
	}
	return nil
}

func distinctRunes(s string) int {
	seen := map[rune]bool{}
	for _, r := range s {
		seen[r] = true
	}
	return len(seen)
}

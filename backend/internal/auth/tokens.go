package auth

import (
	"crypto/rand"
	"crypto/sha256"
	"encoding/base64"
	"errors"
	"fmt"
	"math/big"
	"time"

	"github.com/golang-jwt/jwt/v5"
	"github.com/google/uuid"

	"github.com/yelnurq/startify/backend/internal/platform/authz"
	"github.com/yelnurq/startify/backend/internal/platform/secrets"
)

const issuer = "qadam"

// Claims are the contents of an access token.
type Claims struct {
	jwt.RegisteredClaims
	TenantID    uuid.UUID `json:"tid"`
	SessionID   uuid.UUID `json:"sid"`
	Roles       []string  `json:"roles"`
	SuperAdmin  bool      `json:"sa,omitempty"`
	RoleVersion int64     `json:"rv"`
}

// Principal converts the claims.
func (c Claims) Principal() (authz.Principal, error) {
	userID, err := uuid.Parse(c.Subject)
	if err != nil {
		return authz.Principal{}, fmt.Errorf("auth: subject: %w", err)
	}
	roles := make([]authz.Role, len(c.Roles))
	for i, r := range c.Roles {
		roles[i] = authz.Role(r)
	}
	return authz.Principal{UserID: userID, TenantID: c.TenantID, SessionID: c.SessionID, Roles: roles, SuperAdmin: c.SuperAdmin}, nil
}

// accessTokens signs and verifies short-lived access tokens (HS256 with a key
// derived from the master secret).
type accessTokens struct {
	key []byte
	ttl time.Duration
}

func newAccessTokens(keys *secrets.Keyring, ttl time.Duration) *accessTokens {
	return &accessTokens{key: keys.Key(secrets.PurposeAccessToken), ttl: ttl}
}

func (a *accessTokens) issue(p authz.Principal, roleVersion int64, now time.Time) (string, error) {
	roles := make([]string, len(p.Roles))
	for i, r := range p.Roles {
		roles[i] = string(r)
	}
	claims := Claims{
		RegisteredClaims: jwt.RegisteredClaims{
			ID:        tokenEncoding.EncodeToString(secrets.RandomToken(12)),
			Issuer:    issuer,
			Subject:   p.UserID.String(),
			IssuedAt:  jwt.NewNumericDate(now),
			ExpiresAt: jwt.NewNumericDate(now.Add(a.ttl)),
		},
		TenantID: p.TenantID, SessionID: p.SessionID, Roles: roles, SuperAdmin: p.SuperAdmin, RoleVersion: roleVersion,
	}
	s, err := jwt.NewWithClaims(jwt.SigningMethodHS256, claims).SignedString(a.key)
	if err != nil {
		return "", fmt.Errorf("auth: sign: %w", err)
	}
	return s, nil
}

var errBadToken = errors.New("auth: invalid access token")

func (a *accessTokens) parse(token string) (Claims, error) {
	var c Claims
	_, err := jwt.ParseWithClaims(token, &c, func(*jwt.Token) (any, error) { return a.key, nil },
		jwt.WithValidMethods([]string{jwt.SigningMethodHS256.Alg()}),
		jwt.WithIssuer(issuer),
		jwt.WithExpirationRequired(),
		jwt.WithLeeway(30*time.Second),
	)
	if err != nil {
		return c, errors.Join(errBadToken, err)
	}
	return c, nil
}

var tokenEncoding = base64.RawURLEncoding

// newRefreshToken returns an opaque refresh token and the hash stored for it.
func newRefreshToken() (string, []byte) {
	token := tokenEncoding.EncodeToString(secrets.RandomToken(32))
	return token, refreshHash(token)
}

func refreshHash(token string) []byte {
	sum := sha256.Sum256([]byte(token))
	return sum[:]
}

// newLinkToken returns the token of an e-mailed link and its keyed hash.
func newLinkToken(keys *secrets.Keyring) (string, []byte) {
	token := tokenEncoding.EncodeToString(secrets.RandomToken(32))
	return token, linkHash(keys, token)
}

func linkHash(keys *secrets.Keyring, token string) []byte {
	return keys.MAC(secrets.PurposeOneTimeCode, []byte("link"), []byte(token))
}

// newCode returns a six-digit sign-in code.
func newCode() (string, error) {
	n, err := rand.Int(rand.Reader, big.NewInt(1_000_000))
	if err != nil {
		return "", fmt.Errorf("auth: code: %w", err)
	}
	return fmt.Sprintf("%06d", n.Int64()), nil
}

// codeHash binds a code to its token row, so equal codes never collide.
func codeHash(keys *secrets.Keyring, tokenID, userID uuid.UUID, code string) []byte {
	return keys.MAC(secrets.PurposeOneTimeCode, []byte("code"), tokenID[:], userID[:], []byte(code))
}

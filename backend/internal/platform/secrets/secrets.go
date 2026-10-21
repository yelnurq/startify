// Package secrets derives purpose-specific keys from the master secret and
// encrypts values stored at rest (for example OIDC client secrets).
package secrets

import (
	"crypto/aes"
	"crypto/cipher"
	"crypto/hkdf"
	"crypto/hmac"
	"crypto/rand"
	"crypto/sha256"
	"encoding/binary"
	"errors"
	"fmt"
)

// Purposes of derived keys. Each yields an independent 32-byte key.
const (
	PurposeAccessToken = "qadam/access-token/v1"
	PurposeOneTimeCode = "qadam/one-time-code/v1"
	PurposeSealing     = "qadam/sealing/v1"
	PurposeOIDCState   = "qadam/oidc-state/v1"
)

// Keyring holds the master secret.
type Keyring struct {
	master []byte
}

// NewKeyring returns a keyring for the configured master secret.
func NewKeyring(master string) (*Keyring, error) {
	if len(master) < 32 {
		return nil, errors.New("secrets: master secret must be at least 32 bytes")
	}
	return &Keyring{master: []byte(master)}, nil
}

// Key derives the 32-byte key for a purpose (HKDF-SHA256).
func (k *Keyring) Key(purpose string) []byte {
	key, err := hkdf.Key(sha256.New, k.master, nil, purpose, 32)
	if err != nil {
		// Only possible for an invalid length request, which is a constant here.
		panic(fmt.Sprintf("secrets: derive key: %v", err))
	}
	return key
}

// MAC returns HMAC-SHA256 of the parts under the purpose key; parts are length-prefixed
// so that ("ab","c") and ("a","bc") never collide.
func (k *Keyring) MAC(purpose string, parts ...[]byte) []byte {
	mac := hmac.New(sha256.New, k.Key(purpose))
	for _, p := range parts {
		mac.Write(binary.BigEndian.AppendUint32(nil, uint32(len(p)))) //nolint:gosec // parts are far below 4 GiB
		mac.Write(p)
	}
	return mac.Sum(nil)
}

// Seal encrypts plaintext with AES-256-GCM; the nonce is prepended.
func (k *Keyring) Seal(plaintext []byte) ([]byte, error) {
	gcm, err := k.aead()
	if err != nil {
		return nil, err
	}
	nonce := make([]byte, gcm.NonceSize())
	if _, err := rand.Read(nonce); err != nil {
		return nil, fmt.Errorf("secrets: nonce: %w", err)
	}
	return gcm.Seal(nonce, nonce, plaintext, nil), nil
}

// Open decrypts a value produced by Seal.
func (k *Keyring) Open(sealed []byte) ([]byte, error) {
	gcm, err := k.aead()
	if err != nil {
		return nil, err
	}
	if len(sealed) < gcm.NonceSize() {
		return nil, errors.New("secrets: sealed value too short")
	}
	nonce, ciphertext := sealed[:gcm.NonceSize()], sealed[gcm.NonceSize():]
	plaintext, err := gcm.Open(nil, nonce, ciphertext, nil)
	if err != nil {
		return nil, fmt.Errorf("secrets: open: %w", err)
	}
	return plaintext, nil
}

func (k *Keyring) aead() (cipher.AEAD, error) {
	block, err := aes.NewCipher(k.Key(PurposeSealing))
	if err != nil {
		return nil, fmt.Errorf("secrets: cipher: %w", err)
	}
	return cipher.NewGCM(block)
}

// RandomToken returns n random bytes (for refresh tokens and e-mail links).
func RandomToken(n int) []byte {
	b := make([]byte, n)
	if _, err := rand.Read(b); err != nil {
		panic(fmt.Sprintf("secrets: random: %v", err))
	}
	return b
}

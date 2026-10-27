package secrets

import (
	"bytes"
	"testing"
)

func keyring(t *testing.T) *Keyring {
	t.Helper()
	k, err := NewKeyring("test-master-secret-with-enough-length-0123456789")
	if err != nil {
		t.Fatal(err)
	}
	return k
}

func TestDerivedKeysAreIndependent(t *testing.T) {
	k := keyring(t)
	a, b := k.Key(PurposeAccessToken), k.Key(PurposeOneTimeCode)
	if len(a) != 32 || bytes.Equal(a, b) {
		t.Fatalf("derived keys must be 32 bytes and differ per purpose")
	}
	if !bytes.Equal(a, k.Key(PurposeAccessToken)) {
		t.Fatal("derivation must be deterministic")
	}
}

func TestSealOpenRoundTrip(t *testing.T) {
	k := keyring(t)
	sealed, err := k.Seal([]byte("client-secret"))
	if err != nil {
		t.Fatal(err)
	}
	again, _ := k.Seal([]byte("client-secret"))
	if bytes.Equal(sealed, again) {
		t.Fatal("nonces must make ciphertexts differ")
	}
	plain, err := k.Open(sealed)
	if err != nil || string(plain) != "client-secret" {
		t.Fatalf("open = %q, %v", plain, err)
	}
	sealed[len(sealed)-1] ^= 1
	if _, err := k.Open(sealed); err == nil {
		t.Fatal("tampered ciphertext must not open")
	}
}

func TestMACSeparatesParts(t *testing.T) {
	k := keyring(t)
	if bytes.Equal(k.MAC(PurposeOneTimeCode, []byte("ab"), []byte("c")), k.MAC(PurposeOneTimeCode, []byte("a"), []byte("bc"))) {
		t.Fatal("length prefixing must separate parts")
	}
}

func TestShortMasterRejected(t *testing.T) {
	if _, err := NewKeyring("short"); err == nil {
		t.Fatal("expected error")
	}
}

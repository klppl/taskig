package session

import (
	"bytes"
	"testing"
)

func TestDeriveKeys(t *testing.T) {
	secret := "this-is-a-32-byte-test-secret!!"

	signing, encryption, err := DeriveKeys(secret)
	if err != nil {
		t.Fatalf("DeriveKeys: %v", err)
	}

	if len(signing) != 32 {
		t.Fatalf("signing key length: got %d, want 32", len(signing))
	}
	if len(encryption) != 32 {
		t.Fatalf("encryption key length: got %d, want 32", len(encryption))
	}
	if bytes.Equal(signing, encryption) {
		t.Fatal("signing and encryption keys must differ")
	}

	// Deterministic: same input produces same output
	s2, e2, _ := DeriveKeys(secret)
	if !bytes.Equal(signing, s2) || !bytes.Equal(encryption, e2) {
		t.Fatal("DeriveKeys must be deterministic")
	}
}

func TestEncryptDecryptToken(t *testing.T) {
	secret := "this-is-a-32-byte-test-secret!!"
	_, encKey, err := DeriveKeys(secret)
	if err != nil {
		t.Fatalf("DeriveKeys: %v", err)
	}

	plaintext := []byte(`{"access_token":"ya29.xxx","refresh_token":"1//xxx"}`)

	ciphertext, err := Encrypt(encKey, plaintext)
	if err != nil {
		t.Fatalf("Encrypt: %v", err)
	}

	if bytes.Equal(plaintext, ciphertext) {
		t.Fatal("ciphertext must differ from plaintext")
	}

	decrypted, err := Decrypt(encKey, ciphertext)
	if err != nil {
		t.Fatalf("Decrypt: %v", err)
	}

	if !bytes.Equal(plaintext, decrypted) {
		t.Fatalf("roundtrip failed: got %q, want %q", decrypted, plaintext)
	}
}

func TestDecryptWrongKey(t *testing.T) {
	secret1 := "this-is-a-32-byte-test-secret!!"
	secret2 := "another-32-byte-test-secret!!xx"

	_, key1, _ := DeriveKeys(secret1)
	_, key2, _ := DeriveKeys(secret2)

	ciphertext, _ := Encrypt(key1, []byte("secret data"))

	_, err := Decrypt(key2, ciphertext)
	if err == nil {
		t.Fatal("expected decrypt to fail with wrong key")
	}
}

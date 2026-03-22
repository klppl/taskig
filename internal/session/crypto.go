package session

import (
	"crypto/aes"
	"crypto/cipher"
	"crypto/rand"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"io"

	"golang.org/x/crypto/hkdf"
)

// DeriveKeys uses HKDF-SHA256 to derive a 32-byte signing key and a 32-byte encryption key.
func DeriveKeys(secret string) (signingKey, encryptionKey []byte, err error) {
	master := []byte(secret)

	signingKey = make([]byte, 32)
	r := hkdf.New(sha256.New, master, []byte("google-tasks-signing"), nil)
	if _, err := io.ReadFull(r, signingKey); err != nil {
		return nil, nil, fmt.Errorf("derive signing key: %w", err)
	}

	encryptionKey = make([]byte, 32)
	r = hkdf.New(sha256.New, master, []byte("google-tasks-encryption"), nil)
	if _, err := io.ReadFull(r, encryptionKey); err != nil {
		return nil, nil, fmt.Errorf("derive encryption key: %w", err)
	}

	return signingKey, encryptionKey, nil
}

// Encrypt encrypts plaintext using AES-256-GCM with the given key.
func Encrypt(key, plaintext []byte) ([]byte, error) {
	block, err := aes.NewCipher(key)
	if err != nil {
		return nil, err
	}

	gcm, err := cipher.NewGCM(block)
	if err != nil {
		return nil, err
	}

	nonce := make([]byte, gcm.NonceSize())
	if _, err := io.ReadFull(rand.Reader, nonce); err != nil {
		return nil, err
	}

	return gcm.Seal(nonce, nonce, plaintext, nil), nil
}

// Decrypt decrypts ciphertext produced by Encrypt.
func Decrypt(key, ciphertext []byte) ([]byte, error) {
	block, err := aes.NewCipher(key)
	if err != nil {
		return nil, err
	}

	gcm, err := cipher.NewGCM(block)
	if err != nil {
		return nil, err
	}

	nonceSize := gcm.NonceSize()
	if len(ciphertext) < nonceSize {
		return nil, fmt.Errorf("ciphertext too short")
	}

	nonce, ciphertext := ciphertext[:nonceSize], ciphertext[nonceSize:]
	return gcm.Open(nil, nonce, ciphertext, nil)
}

// GenerateRandomHex returns a random hex string of the given byte length.
func GenerateRandomHex(n int) (string, error) {
	b := make([]byte, n)
	if _, err := rand.Read(b); err != nil {
		return "", err
	}
	return hex.EncodeToString(b), nil
}

package apikeys

import (
	"crypto/rand"
	"crypto/sha256"
	"database/sql"
	"encoding/hex"
	"fmt"
	"time"
)

const keyPrefix = "tsk_"

// APIKey represents a stored API key (without the raw key).
type APIKey struct {
	ID        string
	UserEmail string
	Name      string
	CreatedAt time.Time
}

// GenerateKey creates a new API key and stores its hash in the DB.
// Returns the full raw key (shown once) and the stored record.
func GenerateKey(db *sql.DB, email, name string) (rawKey string, key APIKey, err error) {
	b := make([]byte, 32)
	if _, err := rand.Read(b); err != nil {
		return "", APIKey{}, fmt.Errorf("generate random: %w", err)
	}
	rawKey = keyPrefix + hex.EncodeToString(b)

	idBytes := make([]byte, 16)
	if _, err := rand.Read(idBytes); err != nil {
		return "", APIKey{}, fmt.Errorf("generate id: %w", err)
	}
	id := hex.EncodeToString(idBytes)

	hash := HashKey(rawKey)
	now := time.Now()

	_, err = db.Exec(
		"INSERT INTO api_keys (id, user_email, key_hash, name, created_at) VALUES (?, ?, ?, ?, ?)",
		id, email, hash, name, now,
	)
	if err != nil {
		return "", APIKey{}, fmt.Errorf("insert api key: %w", err)
	}

	return rawKey, APIKey{ID: id, UserEmail: email, Name: name, CreatedAt: now}, nil
}

// LookupByHash finds the user email associated with an API key hash.
func LookupByHash(db *sql.DB, keyHash string) (string, error) {
	var email string
	err := db.QueryRow("SELECT user_email FROM api_keys WHERE key_hash = ?", keyHash).Scan(&email)
	if err != nil {
		return "", fmt.Errorf("lookup api key: %w", err)
	}
	return email, nil
}

// DeleteKey removes an API key by ID, scoped to the user's email.
func DeleteKey(db *sql.DB, id, email string) error {
	result, err := db.Exec("DELETE FROM api_keys WHERE id = ? AND user_email = ?", id, email)
	if err != nil {
		return fmt.Errorf("delete api key: %w", err)
	}
	n, _ := result.RowsAffected()
	if n == 0 {
		return fmt.Errorf("api key not found")
	}
	return nil
}

// HashKey returns the SHA-256 hex hash of a raw API key.
func HashKey(rawKey string) string {
	h := sha256.Sum256([]byte(rawKey))
	return hex.EncodeToString(h[:])
}

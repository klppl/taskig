package session

import (
	"database/sql"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"testing"

	"github.com/alex/google-tasks/internal/database"
)

func setupTestDB(t *testing.T) *sql.DB {
	t.Helper()
	dir := t.TempDir()
	db, err := database.Open(
		filepath.Join(dir, "test.db"),
		filepath.Join("..", "..", "migrations"),
	)
	if err != nil {
		t.Fatalf("open db: %v", err)
	}
	t.Cleanup(func() { db.Close() })
	return db
}

func TestStoreRoundtrip(t *testing.T) {
	db := setupTestDB(t)
	signingKey := []byte("01234567890123456789012345678901")
	encryptionKey := []byte("abcdefghijklmnopqrstuvwxyz012345")

	store := NewSQLiteStore(db, signingKey, encryptionKey, false)

	// Create a session
	r := httptest.NewRequest(http.MethodGet, "/", nil)
	w := httptest.NewRecorder()

	sess, err := store.Get(r, "session")
	if err != nil {
		t.Fatalf("Get: %v", err)
	}

	sess.Values["email"] = "test@example.com"
	sess.Values["oauth_token"] = []byte(`{"access_token":"test"}`)

	if err := store.Save(r, w, sess); err != nil {
		t.Fatalf("Save: %v", err)
	}

	// Read it back with the cookie from the response
	cookies := w.Result().Cookies()
	r2 := httptest.NewRequest(http.MethodGet, "/", nil)
	for _, c := range cookies {
		r2.AddCookie(c)
	}

	sess2, err := store.Get(r2, "session")
	if err != nil {
		t.Fatalf("Get round-trip: %v", err)
	}

	if sess2.Values["email"] != "test@example.com" {
		t.Fatalf("email: got %v, want test@example.com", sess2.Values["email"])
	}
}

func TestStoreDelete(t *testing.T) {
	db := setupTestDB(t)
	signingKey := []byte("01234567890123456789012345678901")
	encryptionKey := []byte("abcdefghijklmnopqrstuvwxyz012345")

	store := NewSQLiteStore(db, signingKey, encryptionKey, false)

	// Create a session
	r := httptest.NewRequest(http.MethodGet, "/", nil)
	w := httptest.NewRecorder()

	sess, _ := store.Get(r, "session")
	sess.Values["email"] = "test@example.com"
	sess.Values["oauth_token"] = []byte(`{"access_token":"test"}`)
	_ = store.Save(r, w, sess)

	// Delete it
	cookies := w.Result().Cookies()
	r2 := httptest.NewRequest(http.MethodGet, "/", nil)
	for _, c := range cookies {
		r2.AddCookie(c)
	}
	w2 := httptest.NewRecorder()

	sess2, _ := store.Get(r2, "session")
	sess2.Options.MaxAge = -1
	if err := store.Save(r2, w2, sess2); err != nil {
		t.Fatalf("Save (delete): %v", err)
	}

	// Verify it's gone
	var count int
	db.QueryRow("SELECT COUNT(*) FROM sessions").Scan(&count)
	if count != 0 {
		t.Fatalf("expected 0 sessions, got %d", count)
	}
}

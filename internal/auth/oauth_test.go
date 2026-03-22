package auth

import (
	"database/sql"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"strings"
	"testing"

	"github.com/alex/google-tasks/internal/config"
	"github.com/alex/google-tasks/internal/database"
	"github.com/alex/google-tasks/internal/session"
	"github.com/labstack/echo/v4"
)

func setupTestStore(t *testing.T) (*sql.DB, *session.SQLiteStore) {
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

	signingKey := []byte("01234567890123456789012345678901")
	encryptionKey := []byte("abcdefghijklmnopqrstuvwxyz012345")
	store := session.NewSQLiteStore(db, signingKey, encryptionKey, false)
	return db, store
}

func TestLoginRedirect(t *testing.T) {
	_, store := setupTestStore(t)
	cfg := &config.Config{
		GoogleClientID:     "test-client-id",
		GoogleClientSecret: "test-secret",
		AllowedEmail:       "test@gmail.com",
		BaseURL:            "http://localhost:8080",
	}

	h := NewHandlers(cfg, store)
	e := echo.New()
	req := httptest.NewRequest(http.MethodGet, "/auth/google", nil)
	rec := httptest.NewRecorder()
	c := e.NewContext(req, rec)

	err := h.HandleLogin(c)
	if err != nil {
		t.Fatalf("HandleLogin: %v", err)
	}

	if rec.Code != http.StatusFound {
		t.Fatalf("status: got %d, want %d", rec.Code, http.StatusFound)
	}

	loc := rec.Header().Get("Location")
	if loc == "" {
		t.Fatal("expected redirect Location header")
	}

	if !strings.Contains(loc, "accounts.google.com") {
		t.Fatalf("expected Google auth URL, got %s", loc)
	}
	if !strings.Contains(loc, "client_id=test-client-id") {
		t.Fatalf("expected client_id in URL, got %s", loc)
	}
}

func TestLogout(t *testing.T) {
	_, store := setupTestStore(t)
	cfg := &config.Config{
		GoogleClientID:     "test-client-id",
		GoogleClientSecret: "test-secret",
		AllowedEmail:       "test@gmail.com",
		BaseURL:            "http://localhost:8080",
	}

	h := NewHandlers(cfg, store)
	e := echo.New()

	// First create a session
	req := httptest.NewRequest(http.MethodGet, "/", nil)
	rec := httptest.NewRecorder()
	sess, _ := store.Get(req, "session")
	sess.Values["email"] = "test@gmail.com"
	sess.Values["oauth_token"] = []byte(`{"access_token":"test"}`)
	store.Save(req, rec, sess)

	// Now logout with that session cookie
	cookies := rec.Result().Cookies()
	logoutReq := httptest.NewRequest(http.MethodPost, "/auth/logout", nil)
	for _, c := range cookies {
		logoutReq.AddCookie(c)
	}
	logoutRec := httptest.NewRecorder()
	c := e.NewContext(logoutReq, logoutRec)

	err := h.HandleLogout(c)
	if err != nil {
		t.Fatalf("HandleLogout: %v", err)
	}

	if logoutRec.Code != http.StatusFound {
		t.Fatalf("status: got %d, want %d", logoutRec.Code, http.StatusFound)
	}
}

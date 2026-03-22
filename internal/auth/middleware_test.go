package auth

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/alex/google-tasks/internal/config"
	"github.com/labstack/echo/v4"
)

func TestMiddlewareRedirectsUnauthenticated(t *testing.T) {
	_, store := setupTestStore(t)
	cfg := &config.Config{
		GoogleClientID:     "test-client-id",
		GoogleClientSecret: "test-secret",
		AllowedEmail:       "test@gmail.com",
		BaseURL:            "http://localhost:8080",
		SessionSecret:      "01234567890123456789012345678901",
	}

	h := NewHandlers(cfg, store)
	mw := NewMiddleware(store, h.OAuthConfig())

	e := echo.New()
	e.GET("/dashboard", func(c echo.Context) error {
		return c.String(200, "ok")
	}, mw.RequireAuth)

	req := httptest.NewRequest(http.MethodGet, "/dashboard", nil)
	rec := httptest.NewRecorder()
	e.ServeHTTP(rec, req)

	if rec.Code != http.StatusFound {
		t.Fatalf("status: got %d, want %d", rec.Code, http.StatusFound)
	}
	if rec.Header().Get("Location") != "/login" {
		t.Fatalf("redirect: got %s, want /login", rec.Header().Get("Location"))
	}
}

func TestMiddlewareAllowsAuthenticated(t *testing.T) {
	_, store := setupTestStore(t)
	cfg := &config.Config{
		GoogleClientID:     "test-client-id",
		GoogleClientSecret: "test-secret",
		AllowedEmail:       "test@gmail.com",
		BaseURL:            "http://localhost:8080",
		SessionSecret:      "01234567890123456789012345678901",
	}

	h := NewHandlers(cfg, store)
	mw := NewMiddleware(store, h.OAuthConfig())

	e := echo.New()
	e.GET("/dashboard", func(c echo.Context) error {
		return c.String(200, "ok")
	}, mw.RequireAuth)

	// Create a session first
	setupReq := httptest.NewRequest(http.MethodGet, "/", nil)
	setupRec := httptest.NewRecorder()
	sess, _ := store.Get(setupReq, "session")
	sess.Values["email"] = "test@gmail.com"
	sess.Values["oauth_token"] = []byte(`{"access_token":"ya29.test","token_type":"Bearer","expiry":"2099-01-01T00:00:00Z"}`)
	store.Save(setupReq, setupRec, sess)

	// Make authenticated request
	cookies := setupRec.Result().Cookies()
	req := httptest.NewRequest(http.MethodGet, "/dashboard", nil)
	for _, c := range cookies {
		req.AddCookie(c)
	}
	rec := httptest.NewRecorder()
	e.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("status: got %d, want %d", rec.Code, http.StatusOK)
	}
}

func TestCSRFMiddlewareRejectsWithoutHeader(t *testing.T) {
	e := echo.New()
	e.POST("/api/test", func(c echo.Context) error {
		return c.String(200, "ok")
	}, RequireXHR)

	req := httptest.NewRequest(http.MethodPost, "/api/test", nil)
	rec := httptest.NewRecorder()
	e.ServeHTTP(rec, req)

	if rec.Code != http.StatusForbidden {
		t.Fatalf("status: got %d, want %d", rec.Code, http.StatusForbidden)
	}
}

func TestCSRFMiddlewareAllowsWithHeader(t *testing.T) {
	e := echo.New()
	e.POST("/api/test", func(c echo.Context) error {
		return c.String(200, "ok")
	}, RequireXHR)

	req := httptest.NewRequest(http.MethodPost, "/api/test", nil)
	req.Header.Set("X-Requested-With", "XMLHttpRequest")
	rec := httptest.NewRecorder()
	e.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("status: got %d, want %d", rec.Code, http.StatusOK)
	}
}

package apikeys

import (
	"context"
	"database/sql"
	"encoding/json"
	"net/http"
	"strings"
	"time"

	"github.com/alex/google-tasks/internal/session"
	"github.com/labstack/echo/v4"
	"golang.org/x/oauth2"
	"google.golang.org/api/option"
	tasks "google.golang.org/api/tasks/v1"
)

type ctxKey string

const (
	tasksClientKey ctxKey = "apiTasksClient"
	emailCtxKey    ctxKey = "apiEmail"
)

// Middleware authenticates requests using API keys.
type Middleware struct {
	db            *sql.DB
	encryptionKey []byte
	oauthConfig   *oauth2.Config
}

// NewMiddleware creates API key auth middleware.
func NewMiddleware(db *sql.DB, encryptionKey []byte, oauthConfig *oauth2.Config) *Middleware {
	return &Middleware{db: db, encryptionKey: encryptionKey, oauthConfig: oauthConfig}
}

// RequireAPIKey is Echo middleware that validates an API key Bearer token.
func (m *Middleware) RequireAPIKey(next echo.HandlerFunc) echo.HandlerFunc {
	return func(c echo.Context) error {
		authHeader := c.Request().Header.Get("Authorization")
		if !strings.HasPrefix(authHeader, "Bearer ") {
			return jsonError(c, http.StatusUnauthorized, "missing or invalid Authorization header")
		}
		rawKey := strings.TrimPrefix(authHeader, "Bearer ")
		if !strings.HasPrefix(rawKey, keyPrefix) {
			return jsonError(c, http.StatusUnauthorized, "invalid API key format")
		}

		hash := HashKey(rawKey)
		email, err := LookupByHash(m.db, hash)
		if err != nil {
			return jsonError(c, http.StatusUnauthorized, "invalid API key")
		}

		svc, err := m.loadTasksService(c.Request().Context(), email)
		if err != nil {
			return jsonError(c, http.StatusUnauthorized, "no active session for this user — please log in via the web UI first")
		}

		ctx := context.WithValue(c.Request().Context(), tasksClientKey, svc)
		ctx = context.WithValue(ctx, emailCtxKey, email)
		c.SetRequest(c.Request().WithContext(ctx))

		return next(c)
	}
}

func (m *Middleware) loadTasksService(ctx context.Context, email string) (*tasks.Service, error) {
	var tokenBlob []byte
	var expiresAt time.Time
	err := m.db.QueryRow(
		"SELECT oauth_token, expires_at FROM sessions WHERE email = ? ORDER BY expires_at DESC LIMIT 1",
		email,
	).Scan(&tokenBlob, &expiresAt)
	if err != nil {
		return nil, err
	}

	if time.Now().After(expiresAt) {
		return nil, echo.NewHTTPError(http.StatusUnauthorized, "session expired")
	}

	decrypted, err := session.Decrypt(m.encryptionKey, tokenBlob)
	if err != nil {
		return nil, err
	}

	var token oauth2.Token
	if err := json.Unmarshal(decrypted, &token); err != nil {
		return nil, err
	}

	tokenSource := m.oauthConfig.TokenSource(ctx, &token)
	return tasks.NewService(ctx, option.WithTokenSource(tokenSource))
}

// GetTasksClient retrieves the Google Tasks service from context (set by API key middleware).
func GetTasksClient(c echo.Context) *tasks.Service {
	svc, _ := c.Request().Context().Value(tasksClientKey).(*tasks.Service)
	return svc
}

// GetEmail retrieves the user's email from context (set by API key middleware).
func GetEmail(c echo.Context) string {
	email, _ := c.Request().Context().Value(emailCtxKey).(string)
	return email
}

func jsonError(c echo.Context, code int, msg string) error {
	return c.JSON(code, map[string]any{"error": msg, "code": code})
}

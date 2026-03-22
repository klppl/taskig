package auth

import (
	"context"
	"encoding/json"
	"net/http"

	"github.com/alex/google-tasks/internal/session"
	"github.com/labstack/echo/v4"
	"golang.org/x/oauth2"

	tasks "google.golang.org/api/tasks/v1"
	"google.golang.org/api/option"
)

type contextKey string

const tasksClientKey contextKey = "tasksClient"
const emailKey contextKey = "email"

type Middleware struct {
	store       *session.SQLiteStore
	oauthConfig *oauth2.Config
}

func NewMiddleware(store *session.SQLiteStore, oauthConfig *oauth2.Config) *Middleware {
	return &Middleware{store: store, oauthConfig: oauthConfig}
}

func (m *Middleware) RequireAuth(next echo.HandlerFunc) echo.HandlerFunc {
	return func(c echo.Context) error {
		sess, _ := m.store.Get(c.Request(), "session")

		data := session.GetSessionData(sess)
		if data == nil {
			return c.Redirect(http.StatusFound, "/login")
		}

		// Deserialize the OAuth token
		var token oauth2.Token
		if err := json.Unmarshal(data.OAuthToken, &token); err != nil {
			sess.Options.MaxAge = -1
			m.store.Save(c.Request(), c.Response(), sess)
			return c.Redirect(http.StatusFound, "/login")
		}

		// Create a token source that handles refresh
		tokenSource := m.oauthConfig.TokenSource(c.Request().Context(), &token)

		// Get a potentially refreshed token
		newToken, err := tokenSource.Token()
		if err != nil {
			sess.Options.MaxAge = -1
			m.store.Save(c.Request(), c.Response(), sess)
			return c.Redirect(http.StatusFound, "/login")
		}

		// If the token was refreshed, persist the new one
		if newToken.AccessToken != token.AccessToken {
			if err := session.SetOAuthToken(sess, newToken); err == nil {
				m.store.Save(c.Request(), c.Response(), sess)
			}
		}

		// Create Tasks API client
		client, err := tasks.NewService(c.Request().Context(),
			option.WithTokenSource(tokenSource),
		)
		if err != nil {
			return c.String(http.StatusInternalServerError, "Failed to create Tasks client")
		}

		// Inject into context
		ctx := context.WithValue(c.Request().Context(), tasksClientKey, client)
		ctx = context.WithValue(ctx, emailKey, data.Email)
		c.SetRequest(c.Request().WithContext(ctx))

		return next(c)
	}
}

// GetTasksClient retrieves the Google Tasks client from the request context.
func GetTasksClient(c echo.Context) *tasks.Service {
	client, _ := c.Request().Context().Value(tasksClientKey).(*tasks.Service)
	return client
}

// GetEmail retrieves the user's email from the request context.
func GetEmail(c echo.Context) string {
	email, _ := c.Request().Context().Value(emailKey).(string)
	return email
}

// RequireXHR is middleware that rejects requests without the X-Requested-With header.
func RequireXHR(next echo.HandlerFunc) echo.HandlerFunc {
	return func(c echo.Context) error {
		if c.Request().Header.Get("X-Requested-With") != "XMLHttpRequest" {
			return c.String(http.StatusForbidden, "Forbidden")
		}
		return next(c)
	}
}

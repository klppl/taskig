package auth

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"

	"github.com/alex/google-tasks/internal/config"
	"github.com/alex/google-tasks/internal/session"
	"github.com/labstack/echo/v4"
	"golang.org/x/oauth2"
	"golang.org/x/oauth2/google"

	goauth2 "google.golang.org/api/oauth2/v2"
	"google.golang.org/api/option"
)

type Handlers struct {
	cfg         *config.Config
	store       *session.SQLiteStore
	oauthConfig *oauth2.Config
}

func NewHandlers(cfg *config.Config, store *session.SQLiteStore) *Handlers {
	return &Handlers{
		cfg:   cfg,
		store: store,
		oauthConfig: &oauth2.Config{
			ClientID:     cfg.GoogleClientID,
			ClientSecret: cfg.GoogleClientSecret,
			RedirectURL:  cfg.BaseURL + "/auth/callback",
			Scopes: []string{
				"openid",
				"email",
				"https://www.googleapis.com/auth/tasks",
			},
			Endpoint: google.Endpoint,
		},
	}
}

func (h *Handlers) HandleLogin(c echo.Context) error {
	state, err := session.GenerateRandomHex(16)
	if err != nil {
		return fmt.Errorf("generate state: %w", err)
	}

	// Store state in a short-lived cookie (not the session store, which only persists email+token)
	secure := h.cfg.BaseURL != "" && len(h.cfg.BaseURL) > 5 && h.cfg.BaseURL[:5] == "https"
	c.SetCookie(&http.Cookie{
		Name:     "oauth_state",
		Value:    state,
		Path:     "/",
		MaxAge:   300, // 5 minutes
		HttpOnly: true,
		Secure:   secure,
		SameSite: http.SameSiteLaxMode,
	})

	url := h.oauthConfig.AuthCodeURL(state, oauth2.AccessTypeOffline, oauth2.ApprovalForce)
	return c.Redirect(http.StatusFound, url)
}

func (h *Handlers) HandleCallback(c echo.Context) error {
	// Validate state from cookie
	stateCookie, err := c.Cookie("oauth_state")
	if err != nil || stateCookie.Value == "" || c.QueryParam("state") != stateCookie.Value {
		return c.String(http.StatusBadRequest, "Invalid state parameter")
	}
	// Clear the state cookie
	c.SetCookie(&http.Cookie{
		Name:   "oauth_state",
		Value:  "",
		Path:   "/",
		MaxAge: -1,
	})

	sess, _ := h.store.Get(c.Request(), "session")

	// Exchange code for token
	token, err := h.oauthConfig.Exchange(context.Background(), c.QueryParam("code"))
	if err != nil {
		return c.String(http.StatusBadRequest, "Failed to exchange authorization code")
	}

	// Get user email
	email, err := h.getUserEmail(c.Request().Context(), token)
	if err != nil {
		return c.String(http.StatusInternalServerError, "Failed to get user info")
	}

	// Whitelist check
	if email != h.cfg.AllowedEmail {
		return c.String(http.StatusForbidden, "Access denied — this app is restricted to the owner's account.")
	}

	// Store token in session
	tokenJSON, err := json.Marshal(token)
	if err != nil {
		return fmt.Errorf("marshal token: %w", err)
	}

	sess.Values["email"] = email
	sess.Values["oauth_token"] = tokenJSON
	if err := h.store.Save(c.Request(), c.Response(), sess); err != nil {
		return fmt.Errorf("save session: %w", err)
	}

	return c.Redirect(http.StatusFound, "/dashboard")
}

func (h *Handlers) HandleLogout(c echo.Context) error {
	sess, _ := h.store.Get(c.Request(), "session")
	sess.Options.MaxAge = -1
	if err := h.store.Save(c.Request(), c.Response(), sess); err != nil {
		return fmt.Errorf("clear session: %w", err)
	}
	return c.Redirect(http.StatusFound, "/login")
}

func (h *Handlers) getUserEmail(ctx context.Context, token *oauth2.Token) (string, error) {
	svc, err := goauth2.NewService(ctx, option.WithTokenSource(
		h.oauthConfig.TokenSource(ctx, token),
	))
	if err != nil {
		return "", fmt.Errorf("create oauth2 service: %w", err)
	}

	info, err := svc.Userinfo.Get().Do()
	if err != nil {
		return "", fmt.Errorf("get userinfo: %w", err)
	}

	return info.Email, nil
}

// OAuthConfig returns the OAuth2 config for use by the auth middleware.
func (h *Handlers) OAuthConfig() *oauth2.Config {
	return h.oauthConfig
}

package i18n

import (
	"context"
	"encoding/json"
	"net/http"
	"os"
	"path/filepath"
	"strings"

	"github.com/labstack/echo/v4"
)

type contextKey string

const localeKey contextKey = "locale"

// translations holds all loaded translations: locale -> key -> value
var translations = map[string]map[string]string{}

// locales holds the list of loaded locale codes
var locales []string

// Load reads all *.json files from dir into the translations map.
func Load(dir string) error {
	entries, err := os.ReadDir(dir)
	if err != nil {
		return err
	}
	for _, e := range entries {
		if e.IsDir() || !strings.HasSuffix(e.Name(), ".json") {
			continue
		}
		code := strings.TrimSuffix(e.Name(), ".json")
		data, err := os.ReadFile(filepath.Join(dir, e.Name()))
		if err != nil {
			return err
		}
		var m map[string]string
		if err := json.Unmarshal(data, &m); err != nil {
			return err
		}
		translations[code] = m
		locales = append(locales, code)
	}
	return nil
}

// Middleware reads the locale cookie and injects it into request context.
func Middleware() echo.MiddlewareFunc {
	return func(next echo.HandlerFunc) echo.HandlerFunc {
		return func(c echo.Context) error {
			locale := readLocale(c)
			ctx := context.WithValue(c.Request().Context(), localeKey, locale)
			c.SetRequest(c.Request().WithContext(ctx))
			return next(c)
		}
	}
}

// T returns the translated string for key in the current locale.
// Falls back to English, then to the key itself.
func T(ctx context.Context, key string) string {
	locale := Locale(ctx)
	if m, ok := translations[locale]; ok {
		if v, ok := m[key]; ok {
			return v
		}
	}
	// Fallback to English
	if locale != "en" {
		if m, ok := translations["en"]; ok {
			if v, ok := m[key]; ok {
				return v
			}
		}
	}
	return key
}

// Locale returns the current locale from context.
func Locale(ctx context.Context) string {
	if v, ok := ctx.Value(localeKey).(string); ok {
		return v
	}
	return "en"
}

// Available returns the list of loaded locale codes.
func Available() []string {
	return locales
}

// HandleSetLocale sets the locale cookie from the form value.
func HandleSetLocale(c echo.Context) error {
	locale := c.FormValue("locale")
	if !isValidLocale(locale) {
		locale = "en"
	}

	cookie := &http.Cookie{
		Name:     "locale",
		Value:    locale,
		Path:     "/",
		HttpOnly: true,
		SameSite: http.SameSiteLaxMode,
		MaxAge:   365 * 24 * 60 * 60,
	}
	c.SetCookie(cookie)

	c.Response().Header().Set("HX-Refresh", "true")
	return c.NoContent(http.StatusNoContent)
}

func readLocale(c echo.Context) string {
	cookie, err := c.Cookie("locale")
	if err != nil || cookie.Value == "" {
		return "en"
	}
	if isValidLocale(cookie.Value) {
		return cookie.Value
	}
	return "en"
}

func isValidLocale(code string) bool {
	_, ok := translations[code]
	return ok
}

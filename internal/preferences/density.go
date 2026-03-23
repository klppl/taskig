package preferences

import (
	"context"
	"net/http"

	"github.com/labstack/echo/v4"
)

type contextKey string

const densityKey contextKey = "layout_density"

// DensityMiddleware reads the layout_density cookie and injects it into request context.
func DensityMiddleware(next echo.HandlerFunc) echo.HandlerFunc {
	return func(c echo.Context) error {
		density := readLayoutDensity(c)
		ctx := context.WithValue(c.Request().Context(), densityKey, density)
		c.SetRequest(c.Request().WithContext(ctx))
		return next(c)
	}
}

// DensityFromContext returns the layout density from the request context.
func DensityFromContext(ctx context.Context) string {
	if v, ok := ctx.Value(densityKey).(string); ok {
		return v
	}
	return "default"
}

// HandleCycleLayoutDensity cycles through compact -> default -> comfortable -> compact.
func HandleCycleLayoutDensity(c echo.Context) error {
	current := readLayoutDensity(c)
	order := map[string]string{
		"compact":     "default",
		"default":     "comfortable",
		"comfortable": "compact",
	}
	next := order[current]

	cookie := &http.Cookie{
		Name:     "layout_density",
		Value:    next,
		Path:     "/",
		HttpOnly: true,
		SameSite: http.SameSiteLaxMode,
		MaxAge:   365 * 24 * 60 * 60,
	}
	c.SetCookie(cookie)

	c.Response().Header().Set("HX-Refresh", "true")
	return c.NoContent(http.StatusNoContent)
}

func readLayoutDensity(c echo.Context) string {
	cookie, err := c.Cookie("layout_density")
	if err != nil || cookie.Value == "" {
		return "default"
	}
	switch cookie.Value {
	case "compact", "comfortable":
		return cookie.Value
	default:
		return "default"
	}
}

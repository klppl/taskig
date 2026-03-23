package main

import (
	"log"
	"strings"

	"github.com/alex/google-tasks/internal/apikeys"
	"github.com/alex/google-tasks/internal/auth"
	"github.com/alex/google-tasks/internal/cache"
	"github.com/alex/google-tasks/internal/config"
	"github.com/alex/google-tasks/internal/database"
	"github.com/alex/google-tasks/internal/i18n"
	"github.com/alex/google-tasks/internal/listcolor"
	"github.com/alex/google-tasks/internal/preferences"
	"github.com/alex/google-tasks/internal/session"
	"github.com/alex/google-tasks/internal/tasks"
	"github.com/alex/google-tasks/templates"
	"github.com/joho/godotenv"
	"github.com/labstack/echo/v4"
	"github.com/labstack/echo/v4/middleware"
)

func main() {
	_ = godotenv.Load()

	cfg, err := config.Load()
	if err != nil {
		log.Fatalf("config: %v", err)
	}

	// Load translations
	if err := i18n.Load("locales"); err != nil {
		log.Fatalf("i18n: %v", err)
	}

	// Wire view functions to break import cycle
	tasks.ViewDashboardPage = templates.DashboardPage
	tasks.ViewTaskListContent = templates.TaskListContent
	tasks.ViewTaskItem = templates.TaskItem
	tasks.ViewTaskDetailPanel = templates.TaskDetailPanel
	tasks.ViewTaskDetailEmpty = templates.TaskDetailEmpty
	tasks.ViewTasklistSidebarOOB = templates.TasklistSidebarOOB
	tasks.ViewTodayContent = templates.TodayContent
	tasks.ViewTaskTree = templates.TaskTree
	apikeys.ViewSettingsPage = templates.SettingsPage
	apikeys.ViewAPIKeyCreated = templates.APIKeyCreated
	apikeys.ViewAPIKeyList = templates.APIKeyList

	// Database
	dbPath := cfg.DBPath
	db, err := database.Open(dbPath, "migrations")
	if err != nil {
		log.Fatalf("database: %v", err)
	}
	defer db.Close()

	// Session store
	signingKey, encryptionKey, err := session.DeriveKeys(cfg.SessionSecret)
	if err != nil {
		log.Fatalf("derive keys: %v", err)
	}
	secureCookies := strings.HasPrefix(cfg.BaseURL, "https://")
	store := session.NewSQLiteStore(db, signingKey, encryptionKey, secureCookies)

	// Auth
	authHandlers := auth.NewHandlers(cfg, store)
	authMiddleware := auth.NewMiddleware(store, authHandlers.OAuthConfig())

	// Task handlers
	appCache := cache.New()
	colorStore := listcolor.NewStore(db)
	taskHandlers := tasks.NewHandlers(appCache, colorStore)

	// API key middleware & handlers
	apiKeyMiddleware := apikeys.NewMiddleware(db, encryptionKey, authHandlers.OAuthConfig())
	apiV1Handlers := apikeys.NewHandlers(db, appCache)

	// Echo setup
	e := echo.New()
	e.Use(middleware.Logger())
	e.Use(middleware.Recover())

	// Static files
	e.Static("/static", "static")

	// Public routes (with i18n for translated UI)
	public := e.Group("", i18n.Middleware())
	public.GET("/login", func(c echo.Context) error {
		return templates.LoginPage().Render(c.Request().Context(), c.Response())
	})
	e.GET("/auth/google", authHandlers.HandleLogin)
	e.GET("/auth/callback", authHandlers.HandleCallback)
	e.POST("/auth/logout", authHandlers.HandleLogout)

	// Redirect root to dashboard
	e.GET("/", func(c echo.Context) error {
		return c.Redirect(302, "/dashboard")
	})

	// Protected routes
	dashboard := e.Group("", authMiddleware.RequireAuth, i18n.Middleware(), preferences.DensityMiddleware)
	dashboard.GET("/dashboard", taskHandlers.HandleDashboard)
	dashboard.GET("/settings", apiV1Handlers.HandleSettingsPage)
	dashboard.POST("/settings/keys", apiV1Handlers.HandleSettingsCreateKey)
	dashboard.DELETE("/settings/keys/:id", apiV1Handlers.HandleSettingsDeleteKey)

	api := e.Group("/api", authMiddleware.RequireAuth, auth.RequireXHR, i18n.Middleware())
	api.GET("/today", taskHandlers.HandleToday)
	api.POST("/preferences/hide-completed", taskHandlers.HandleToggleHideCompleted)
	api.POST("/preferences/layout-density", preferences.HandleCycleLayoutDensity)
	api.POST("/preferences/locale", i18n.HandleSetLocale)
	api.GET("/tasklists/:listId/tasks", taskHandlers.HandleListTasks)
	api.POST("/tasklists/:listId/tasks", taskHandlers.HandleCreateTask)
	api.PATCH("/tasklists/:listId/tasks/:taskId", taskHandlers.HandleUpdateTask)
	api.GET("/tasklists/:listId/tasks/:taskId/detail", taskHandlers.HandleGetDetail)
	api.DELETE("/tasklists/:listId/tasks/:taskId", taskHandlers.HandleDeleteTask)
	api.POST("/tasklists/:listId/tasks/:taskId/reschedule", taskHandlers.HandleRescheduleTask)
	api.POST("/tasklists/:listId/tasks/:taskId/move", taskHandlers.HandleMoveTask)
	api.POST("/tasklists/:listId/tasks/:taskId/subtasks", taskHandlers.HandleCreateSubtask)
	api.POST("/tasklists/:listId/tasks/:taskId/move-to-list", taskHandlers.HandleMoveTaskToList)
	api.POST("/tasklists/:listId/color", taskHandlers.HandleCycleListColor)

	// External JSON API (API key auth)
	v1 := e.Group("/api/v1", apiKeyMiddleware.RequireAPIKey)
	v1.POST("/tasks", apiV1Handlers.HandleCreateTask)
	v1.POST("/tasks/:id/move", apiV1Handlers.HandleMoveTask)
	v1.GET("/lists", apiV1Handlers.HandleListLists)

	// API key management (session auth, no XHR requirement)
	keys := e.Group("/api/v1/keys", authMiddleware.RequireAuth)
	keys.POST("", apiV1Handlers.HandleCreateKey)
	keys.DELETE("/:id", apiV1Handlers.HandleDeleteKey)

	e.Logger.Fatal(e.Start(":" + cfg.Port))
}

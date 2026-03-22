package main

import (
	"log"
	"strings"

	"github.com/alex/google-tasks/internal/auth"
	"github.com/alex/google-tasks/internal/config"
	"github.com/alex/google-tasks/internal/database"
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

	// Wire view functions to break import cycle
	tasks.ViewDashboardPage = templates.DashboardPage
	tasks.ViewTaskListContent = templates.TaskListContent
	tasks.ViewTaskItem = templates.TaskItem
	tasks.ViewTaskDetailPanel = templates.TaskDetailPanel
	tasks.ViewTaskDetailEmpty = templates.TaskDetailEmpty
	tasks.ViewTasklistSidebarOOB = templates.TasklistSidebarOOB
	tasks.ViewTodayContent = templates.TodayContent

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
	taskHandlers := tasks.NewHandlers()

	// Echo setup
	e := echo.New()
	e.Use(middleware.Logger())
	e.Use(middleware.Recover())

	// Static files
	e.Static("/static", "static")

	// Public routes
	e.GET("/login", func(c echo.Context) error {
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
	dashboard := e.Group("", authMiddleware.RequireAuth)
	dashboard.GET("/dashboard", taskHandlers.HandleDashboard)

	api := e.Group("/api", authMiddleware.RequireAuth, auth.RequireXHR)
	api.GET("/today", taskHandlers.HandleToday)
	api.POST("/preferences/hide-completed", taskHandlers.HandleToggleHideCompleted)
	api.GET("/tasklists/:listId/tasks", taskHandlers.HandleListTasks)
	api.POST("/tasklists/:listId/tasks", taskHandlers.HandleCreateTask)
	api.PATCH("/tasklists/:listId/tasks/:taskId", taskHandlers.HandleUpdateTask)
	api.GET("/tasklists/:listId/tasks/:taskId/detail", taskHandlers.HandleGetDetail)
	api.DELETE("/tasklists/:listId/tasks/:taskId", taskHandlers.HandleDeleteTask)
	api.POST("/tasklists/:listId/tasks/:taskId/reschedule", taskHandlers.HandleRescheduleTask)
	api.POST("/tasklists/:listId/tasks/:taskId/move", taskHandlers.HandleMoveTask)

	e.Logger.Fatal(e.Start(":" + cfg.Port))
}

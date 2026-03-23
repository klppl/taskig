package apikeys

import (
	"database/sql"
	"net/http"
	"strings"

	"github.com/alex/google-tasks/internal/auth"
	"github.com/alex/google-tasks/internal/cache"
	"github.com/alex/google-tasks/internal/tasks"
	"github.com/labstack/echo/v4"
)

// Handlers serves the /api/v1/ JSON endpoints.
type Handlers struct {
	db    *sql.DB
	cache *cache.Cache
}

// NewHandlers creates API v1 handlers.
func NewHandlers(db *sql.DB, cache *cache.Cache) *Handlers {
	return &Handlers{db: db, cache: cache}
}

type createTaskRequest struct {
	Title    string `json:"title"`
	ListID   string `json:"list_id"`
	ListName string `json:"list_name"`
	Notes    string `json:"notes"`
	Due      string `json:"due"`
}

type taskResponse struct {
	ID        string `json:"id"`
	Title     string `json:"title"`
	Notes     string `json:"notes"`
	Due       string `json:"due"`
	Completed bool   `json:"completed"`
	ListID    string `json:"list_id"`
}

type listItemResponse struct {
	ID    string `json:"id"`
	Title string `json:"title"`
}

// HandleCreateTask handles POST /api/v1/tasks
func (h *Handlers) HandleCreateTask(c echo.Context) error {
	var req createTaskRequest
	if err := c.Bind(&req); err != nil {
		return jsonError(c, http.StatusBadRequest, "invalid JSON body")
	}

	if req.Title == "" {
		return jsonError(c, http.StatusBadRequest, "title is required")
	}
	if req.ListID == "" && req.ListName == "" {
		return jsonError(c, http.StatusBadRequest, "list_id or list_name is required")
	}

	svc := GetTasksClient(c)
	email := GetEmail(c)
	client := tasks.NewClient(svc, h.cache, email)

	// Resolve list name to ID if needed
	listID := req.ListID
	if listID == "" {
		lists, err := client.ListTaskLists()
		if err != nil {
			return jsonError(c, http.StatusInternalServerError, "failed to fetch task lists")
		}
		for _, l := range lists {
			if strings.EqualFold(l.Title, req.ListName) {
				listID = l.ID
				break
			}
		}
		if listID == "" {
			return jsonError(c, http.StatusNotFound, "list not found: "+req.ListName)
		}
	}

	task, err := client.CreateTask(listID, req.Title, req.Notes)
	if err != nil {
		return jsonError(c, http.StatusInternalServerError, "failed to create task")
	}

	// Set due date if provided
	if req.Due != "" {
		task, err = client.UpdateTask(listID, task.ID, req.Title, req.Notes, req.Due)
		if err != nil {
			return jsonError(c, http.StatusInternalServerError, "task created but failed to set due date")
		}
	}

	return c.JSON(http.StatusCreated, taskResponse{
		ID:        task.ID,
		Title:     task.Title,
		Notes:     task.Notes,
		Due:       task.Due,
		Completed: task.Completed,
		ListID:    listID,
	})
}

// HandleListLists handles GET /api/v1/lists
func (h *Handlers) HandleListLists(c echo.Context) error {
	svc := GetTasksClient(c)
	email := GetEmail(c)
	client := tasks.NewClient(svc, h.cache, email)

	lists, err := client.ListTaskLists()
	if err != nil {
		return jsonError(c, http.StatusInternalServerError, "failed to fetch task lists")
	}

	resp := make([]listItemResponse, len(lists))
	for i, l := range lists {
		resp[i] = listItemResponse{ID: l.ID, Title: l.Title}
	}

	return c.JSON(http.StatusOK, map[string]any{"lists": resp})
}

// HandleCreateKey handles POST /api/v1/keys (session-authed, not API key)
func (h *Handlers) HandleCreateKey(c echo.Context) error {
	email := auth.GetEmail(c)
	if email == "" {
		return jsonError(c, http.StatusUnauthorized, "not authenticated")
	}

	var req struct {
		Name string `json:"name"`
	}
	c.Bind(&req)

	rawKey, key, err := GenerateKey(h.db, email, req.Name)
	if err != nil {
		return jsonError(c, http.StatusInternalServerError, "failed to generate API key")
	}

	return c.JSON(http.StatusCreated, map[string]any{
		"id":      key.ID,
		"key":     rawKey,
		"name":    key.Name,
		"message": "Save this key — it won't be shown again.",
	})
}

// HandleDeleteKey handles DELETE /api/v1/keys/:id (session-authed)
func (h *Handlers) HandleDeleteKey(c echo.Context) error {
	email := auth.GetEmail(c)
	if email == "" {
		return jsonError(c, http.StatusUnauthorized, "not authenticated")
	}

	id := c.Param("id")
	if err := DeleteKey(h.db, id, email); err != nil {
		return jsonError(c, http.StatusNotFound, "API key not found")
	}

	return c.NoContent(http.StatusNoContent)
}

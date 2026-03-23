package apikeys

import (
	"database/sql"
	"net/http"
	"strings"

	"github.com/a-h/templ"
	"github.com/alex/google-tasks/internal/auth"
	"github.com/alex/google-tasks/internal/cache"
	"github.com/alex/google-tasks/internal/tasks"
	"github.com/labstack/echo/v4"
)

// View functions wired at startup to avoid import cycles.
var (
	ViewSettingsPage func(email string, keys []APIKey) templ.Component
	ViewAPIKeyCreated func(rawKey string) templ.Component
	ViewAPIKeyList   func(keys []APIKey) templ.Component
)

// Handlers serves the /api/v1/ JSON endpoints and settings pages.
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

type moveTaskRequest struct {
	DestListID   string `json:"dest_list_id"`
	DestListName string `json:"dest_list_name"`
}

// HandleMoveTask handles POST /api/v1/tasks/:id/move
func (h *Handlers) HandleMoveTask(c echo.Context) error {
	taskID := c.Param("id")
	var req moveTaskRequest
	if err := c.Bind(&req); err != nil {
		return jsonError(c, http.StatusBadRequest, "invalid JSON body")
	}

	if req.DestListID == "" && req.DestListName == "" {
		return jsonError(c, http.StatusBadRequest, "dest_list_id or dest_list_name is required")
	}

	svc := GetTasksClient(c)
	email := GetEmail(c)
	client := tasks.NewClient(svc, h.cache, email)

	lists, err := client.ListTaskLists()
	if err != nil {
		return jsonError(c, http.StatusInternalServerError, "failed to fetch task lists")
	}

	// Resolve destination list
	dstListID := req.DestListID
	if dstListID == "" {
		for _, l := range lists {
			if strings.EqualFold(l.Title, req.DestListName) {
				dstListID = l.ID
				break
			}
		}
		if dstListID == "" {
			return jsonError(c, http.StatusNotFound, "destination list not found: "+req.DestListName)
		}
	}

	// We also need to find the source list by searching all lists for the task.
	// The caller must provide it, or we search.
	srcListID := c.QueryParam("list_id")
	if srcListID == "" {
		// Search all lists for the task
		for _, l := range lists {
			if t, err := client.GetTask(l.ID, taskID); err == nil && t != nil {
				srcListID = l.ID
				break
			}
		}
		if srcListID == "" {
			return jsonError(c, http.StatusNotFound, "task not found in any list")
		}
	}

	if srcListID == dstListID {
		return jsonError(c, http.StatusBadRequest, "task is already in the destination list")
	}

	task, err := client.MoveTaskToList(srcListID, taskID, dstListID)
	if err != nil {
		return jsonError(c, http.StatusInternalServerError, "failed to move task")
	}

	return c.JSON(http.StatusOK, taskResponse{
		ID:        task.ID,
		Title:     task.Title,
		Notes:     task.Notes,
		Due:       task.Due,
		Completed: task.Completed,
		ListID:    dstListID,
	})
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

// HandleDeleteKey handles DELETE /api/v1/keys/:id (session-authed, JSON API)
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

// HandleSettingsPage renders the settings page.
func (h *Handlers) HandleSettingsPage(c echo.Context) error {
	email := auth.GetEmail(c)
	keys, _ := ListKeys(h.db, email)
	return ViewSettingsPage(email, keys).Render(c.Request().Context(), c.Response())
}

// HandleSettingsCreateKey creates a key and returns HTML for the settings page.
func (h *Handlers) HandleSettingsCreateKey(c echo.Context) error {
	email := auth.GetEmail(c)
	name := c.FormValue("name")

	rawKey, _, err := GenerateKey(h.db, email, name)
	if err != nil {
		return c.HTML(http.StatusInternalServerError,
			`<div class="rounded-md bg-red-50 p-3 text-sm text-red-600 dark:bg-red-900/20 dark:text-red-400">Failed to generate key</div>`)
	}

	ctx := c.Request().Context()
	w := c.Response()

	// Render the "key created" banner
	if err := ViewAPIKeyCreated(rawKey).Render(ctx, w); err != nil {
		return err
	}

	// OOB update the key list
	keys, _ := ListKeys(h.db, email)
	_, _ = w.Write([]byte(`<div id="key-list" hx-swap-oob="innerHTML:#key-list">`))
	if err := ViewAPIKeyList(keys).Render(ctx, w); err != nil {
		return err
	}
	_, _ = w.Write([]byte(`</div>`))
	return nil
}

// HandleSettingsDeleteKey deletes a key and returns empty HTML (removes the element).
func (h *Handlers) HandleSettingsDeleteKey(c echo.Context) error {
	email := auth.GetEmail(c)
	id := c.Param("id")
	DeleteKey(h.db, id, email)
	// Return empty string to remove the element via hx-swap="outerHTML"
	return c.String(http.StatusOK, "")
}

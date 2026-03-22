package tasks

import (
	"context"
	"io"
	"net/http"
	"time"

	"github.com/a-h/templ"
	"github.com/alex/google-tasks/internal/auth"
	"github.com/labstack/echo/v4"
)

type Handlers struct{}

func NewHandlers() *Handlers {
	return &Handlers{}
}

func (h *Handlers) HandleDashboard(c echo.Context) error {
	svc := auth.GetTasksClient(c)
	if svc == nil {
		return c.Redirect(http.StatusFound, "/login")
	}

	client := NewClient(svc)

	lists, err := client.ListTaskLists()
	if err != nil {
		return c.String(http.StatusInternalServerError, "Failed to load task lists")
	}

	activeListID := c.QueryParam("list")
	var activeTasks []Task

	if activeListID == "" && len(lists) > 0 {
		activeListID = lists[0].ID
	}

	hideCompleted := readHideCompleted(c)

	if activeListID == "_today" {
		today := time.Now().Format("2006-01-02")
		activeTasks, err = client.ListTodayTasks(today)
		if err != nil {
			return c.String(http.StatusInternalServerError, "Failed to load tasks")
		}
	} else if activeListID != "" {
		activeTasks, err = client.ListTasks(activeListID, !hideCompleted)
		if err != nil {
			return c.String(http.StatusInternalServerError, "Failed to load tasks")
		}
	}

	return ViewDashboardPage(lists, activeListID, activeTasks, hideCompleted).Render(c.Request().Context(), c.Response())
}

func (h *Handlers) HandleListTasks(c echo.Context) error {
	svc := auth.GetTasksClient(c)
	client := NewClient(svc)
	listID := c.Param("listId")
	listTitle := c.QueryParam("title")
	hideCompleted := readHideCompleted(c)

	taskItems, err := client.ListTasks(listID, !hideCompleted)
	if err != nil {
		return renderError(c, "Failed to load tasks")
	}

	ctx := c.Request().Context()
	w := c.Response()

	// Render task list content (primary swap into #task-panel)
	if err := ViewTaskListContent(listID, listTitle, taskItems, hideCompleted).Render(ctx, w); err != nil {
		return err
	}

	// Render sidebar OOB swap to update active highlight
	lists, err := client.ListTaskLists()
	if err != nil {
		return nil
	}
	return ViewTasklistSidebarOOB(lists, listID).Render(ctx, w)
}

func (h *Handlers) HandleCreateTask(c echo.Context) error {
	svc := auth.GetTasksClient(c)
	if svc == nil {
		return c.String(http.StatusBadRequest, "Not authenticated")
	}
	client := NewClient(svc)
	listID := c.Param("listId")
	title := c.FormValue("title")

	if title == "" {
		return c.String(http.StatusBadRequest, "Title is required")
	}

	task, err := client.CreateTask(listID, title, "")
	if err != nil {
		return renderError(c, "Failed to create task")
	}

	return ViewTaskItem(listID, *task, false).Render(c.Request().Context(), c.Response())
}

func (h *Handlers) HandleUpdateTask(c echo.Context) error {
	svc := auth.GetTasksClient(c)
	client := NewClient(svc)
	listID := c.Param("listId")
	taskID := c.Param("taskId")
	listTitle := c.FormValue("listTitle")

	ctx := c.Request().Context()
	w := c.Response()

	// Completion toggle (from checkbox in task list)
	if completed := c.FormValue("completed"); completed != "" {
		task, err := client.CompleteTask(listID, taskID, completed == "true")
		if err != nil {
			return renderError(c, "Failed to update task")
		}
		task.ListTitle = listTitle
		return ViewTaskItem(listID, *task, false).Render(ctx, w)
	}

	// Title/notes/due edit (from detail panel)
	title := c.FormValue("title")
	notes := c.FormValue("notes")
	due := c.FormValue("due")
	if title == "" {
		return c.String(http.StatusBadRequest, "Title is required")
	}

	task, err := client.UpdateTask(listID, taskID, title, notes, due)
	if err != nil {
		return renderError(c, "Failed to update task")
	}

	task.ListTitle = listTitle

	// Render updated detail panel (primary response)
	if err := ViewTaskDetailPanel(listID, *task).Render(ctx, w); err != nil {
		return err
	}

	// OOB update the task item in the list
	return renderOOB(ctx, w, "task-"+taskID, ViewTaskItem(listID, *task, false))
}

func (h *Handlers) HandleGetDetail(c echo.Context) error {
	svc := auth.GetTasksClient(c)
	client := NewClient(svc)
	listID := c.Param("listId")
	taskID := c.Param("taskId")

	task, err := client.GetTask(listID, taskID)
	if err != nil {
		return renderError(c, "Failed to load task")
	}

	task.ListTitle = c.QueryParam("listTitle")
	return ViewTaskDetailPanel(listID, *task).Render(c.Request().Context(), c.Response())
}

func (h *Handlers) HandleDeleteTask(c echo.Context) error {
	svc := auth.GetTasksClient(c)
	client := NewClient(svc)
	listID := c.Param("listId")
	taskID := c.Param("taskId")

	if err := client.DeleteTask(listID, taskID); err != nil {
		return renderError(c, "Failed to delete task")
	}

	// Return empty response (removes task item via hx-swap="outerHTML")
	// plus OOB clear of the detail panel
	ctx := c.Request().Context()
	w := c.Response()
	return renderOOB(ctx, w, "detail-panel", ViewTaskDetailEmpty())
}

func (h *Handlers) HandleToday(c echo.Context) error {
	svc := auth.GetTasksClient(c)
	client := NewClient(svc)
	today := time.Now().Format("2006-01-02")

	taskItems, err := client.ListTodayTasks(today)
	if err != nil {
		return renderError(c, "Failed to load tasks")
	}

	ctx := c.Request().Context()
	w := c.Response()

	if err := ViewTodayContent(taskItems).Render(ctx, w); err != nil {
		return err
	}

	// OOB sidebar update
	lists, err := client.ListTaskLists()
	if err != nil {
		return nil
	}
	return ViewTasklistSidebarOOB(lists, "_today").Render(ctx, w)
}

func (h *Handlers) HandleRescheduleTask(c echo.Context) error {
	svc := auth.GetTasksClient(c)
	client := NewClient(svc)
	listID := c.Param("listId")
	taskID := c.Param("taskId")
	due := c.FormValue("due")
	listTitle := c.FormValue("listTitle")

	if due == "" {
		return c.String(http.StatusBadRequest, "Due date is required")
	}

	task, err := client.PatchDueDate(listID, taskID, due)
	if err != nil {
		return renderError(c, "Failed to reschedule task")
	}

	task.ListTitle = listTitle

	ctx := c.Request().Context()
	w := c.Response()

	today := time.Now().Format("2006-01-02")
	if due <= today {
		// Task still belongs in Today view — return updated item
		if err := ViewTaskItem(listID, *task, true).Render(ctx, w); err != nil {
			return err
		}
	}
	// If due > today, primary response is empty (task removed from DOM)

	// OOB: clear detail panel
	return renderOOB(ctx, w, "detail-panel", ViewTaskDetailEmpty())
}

func (h *Handlers) HandleToggleHideCompleted(c echo.Context) error {
	svc := auth.GetTasksClient(c)
	client := NewClient(svc)
	listID := c.FormValue("listId")
	listTitle := c.FormValue("listTitle")

	hideCompleted := !readHideCompleted(c)

	cookie := &http.Cookie{
		Name:     "hide_completed",
		Path:     "/",
		HttpOnly: true,
		SameSite: http.SameSiteLaxMode,
	}
	if hideCompleted {
		cookie.Value = "1"
		cookie.MaxAge = 365 * 24 * 60 * 60 // 1 year
	} else {
		cookie.MaxAge = -1 // delete
	}
	c.SetCookie(cookie)

	taskItems, err := client.ListTasks(listID, !hideCompleted)
	if err != nil {
		return renderError(c, "Failed to load tasks")
	}

	return ViewTaskListContent(listID, listTitle, taskItems, hideCompleted).Render(c.Request().Context(), c.Response())
}

func readHideCompleted(c echo.Context) bool {
	cookie, err := c.Cookie("hide_completed")
	return err == nil && cookie.Value == "1"
}

func renderError(c echo.Context, msg string) error {
	return c.HTML(http.StatusInternalServerError,
		`<div class="rounded-lg bg-red-50 p-3 text-sm text-red-600 dark:bg-red-900/20 dark:text-red-400">`+msg+`</div>`)
}

// renderOOB writes an out-of-band HTMX swap for the given element ID.
func renderOOB(ctx context.Context, w io.Writer, id string, comp templ.Component) error {
	_, _ = w.Write([]byte(`<div id="` + id + `" hx-swap-oob="innerHTML:#` + id + `">`))
	if err := comp.Render(ctx, w); err != nil {
		return err
	}
	_, _ = w.Write([]byte(`</div>`))
	return nil
}

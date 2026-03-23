package tasks

import (
	"context"
	"io"
	"net/http"
	"strconv"
	"sync"
	"time"

	"github.com/a-h/templ"
	"github.com/alex/google-tasks/internal/auth"
	"github.com/alex/google-tasks/internal/cache"
	"github.com/labstack/echo/v4"
)

type Handlers struct {
	cache *cache.Cache
}

func NewHandlers(c *cache.Cache) *Handlers {
	return &Handlers{cache: c}
}

func (h *Handlers) newClient(c echo.Context) *Client {
	svc := auth.GetTasksClient(c)
	email := auth.GetEmail(c)
	return NewClient(svc, h.cache, email)
}

func (h *Handlers) HandleDashboard(c echo.Context) error {
	svc := auth.GetTasksClient(c)
	if svc == nil {
		return c.Redirect(http.StatusFound, "/login")
	}

	client := h.newClient(c)

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
		flat, err := client.ListTasks(activeListID, !hideCompleted)
		if err != nil {
			return c.String(http.StatusInternalServerError, "Failed to load tasks")
		}
		activeTasks = BuildTaskTree(flat)
	}

	return ViewDashboardPage(lists, activeListID, activeTasks, hideCompleted).Render(c.Request().Context(), c.Response())
}

func (h *Handlers) HandleListTasks(c echo.Context) error {
	client := h.newClient(c)
	listID := c.Param("listId")
	listTitle := c.QueryParam("title")
	hideCompleted := readHideCompleted(c)

	// Fetch tasks and task lists in parallel
	var (
		flatTasks []Task
		lists     []TaskList
		taskErr   error
		listErr   error
	)
	var wg sync.WaitGroup
	wg.Add(2)
	go func() {
		defer wg.Done()
		flatTasks, taskErr = client.ListTasks(listID, !hideCompleted)
	}()
	go func() {
		defer wg.Done()
		lists, listErr = client.ListTaskLists()
	}()
	wg.Wait()

	if taskErr != nil {
		return renderError(c, "Failed to load tasks")
	}
	taskItems := BuildTaskTree(flatTasks)

	ctx := c.Request().Context()
	w := c.Response()

	if err := ViewTaskListContent(listID, listTitle, taskItems, hideCompleted).Render(ctx, w); err != nil {
		return err
	}

	if listErr != nil {
		return nil
	}
	return ViewTasklistSidebarOOB(lists, listID).Render(ctx, w)
}

func (h *Handlers) HandleCreateTask(c echo.Context) error {
	svc := auth.GetTasksClient(c)
	if svc == nil {
		return c.String(http.StatusBadRequest, "Not authenticated")
	}
	client := h.newClient(c)
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

func (h *Handlers) HandleCreateSubtask(c echo.Context) error {
	svc := auth.GetTasksClient(c)
	if svc == nil {
		return c.String(http.StatusBadRequest, "Not authenticated")
	}
	client := h.newClient(c)
	listID := c.Param("listId")
	parentID := c.Param("taskId")
	title := c.FormValue("title")
	depth, _ := strconv.Atoi(c.FormValue("depth"))

	if title == "" {
		return c.String(http.StatusBadRequest, "Title is required")
	}

	task, err := client.CreateSubtask(listID, parentID, title)
	if err != nil {
		return renderError(c, "Failed to create subtask")
	}

	return ViewTaskTree(listID, *task, depth).Render(c.Request().Context(), c.Response())
}

func (h *Handlers) HandleUpdateTask(c echo.Context) error {
	client := h.newClient(c)
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
	lists, _ := client.ListTaskLists()

	// Render updated detail panel (primary response)
	if err := ViewTaskDetailPanel(listID, *task, lists).Render(ctx, w); err != nil {
		return err
	}

	// OOB update the task item in the list
	return renderOOBOuter(ctx, w, "task-"+taskID, ViewTaskItem(listID, *task, false))
}

func (h *Handlers) HandleGetDetail(c echo.Context) error {
	client := h.newClient(c)
	listID := c.Param("listId")
	taskID := c.Param("taskId")

	// Fetch task, subtasks, and task lists in parallel
	var (
		task     *Task
		subtasks []Task
		lists    []TaskList
		taskErr  error
		listErr  error
	)
	var wg sync.WaitGroup
	wg.Add(3)
	go func() {
		defer wg.Done()
		task, taskErr = client.GetTask(listID, taskID)
	}()
	go func() {
		defer wg.Done()
		subtasks, _ = client.ListSubtasks(listID, taskID)
	}()
	go func() {
		defer wg.Done()
		lists, listErr = client.ListTaskLists()
	}()
	wg.Wait()

	if taskErr != nil {
		return renderError(c, "Failed to load task")
	}
	if listErr != nil {
		lists = nil
	}

	task.Children = subtasks
	task.ListTitle = c.QueryParam("listTitle")
	return ViewTaskDetailPanel(listID, *task, lists).Render(c.Request().Context(), c.Response())
}

func (h *Handlers) HandleDeleteTask(c echo.Context) error {
	client := h.newClient(c)
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

func (h *Handlers) HandleMoveTaskToList(c echo.Context) error {
	client := h.newClient(c)
	srcListID := c.Param("listId")
	taskID := c.Param("taskId")
	dstListID := c.FormValue("destListId")

	if dstListID == "" || dstListID == srcListID {
		return c.String(http.StatusBadRequest, "Invalid destination list")
	}

	newTask, err := client.MoveTaskToList(srcListID, taskID, dstListID)
	if err != nil {
		return renderError(c, "Failed to move task")
	}

	// Fetch updated data in parallel: task lists + tasks for the new detail panel
	var (
		lists   []TaskList
		listErr error
	)
	var wg sync.WaitGroup
	wg.Add(1)
	go func() {
		defer wg.Done()
		lists, listErr = client.ListTaskLists()
	}()
	wg.Wait()

	ctx := c.Request().Context()
	w := c.Response()

	// Remove the old task item from the list
	if _, err := w.Write([]byte(`<div id="task-` + taskID + `" hx-swap-oob="delete"></div>`)); err != nil {
		return err
	}

	if listErr == nil {
		// Re-render detail panel for the moved task in its new list
		return renderOOB(ctx, w, "detail-panel", ViewTaskDetailPanel(dstListID, *newTask, lists))
	}
	return renderOOB(ctx, w, "detail-panel", ViewTaskDetailEmpty())
}

func (h *Handlers) HandleToday(c echo.Context) error {
	client := h.newClient(c)
	today := time.Now().Format("2006-01-02")

	// Fetch today tasks and task lists in parallel.
	// ListTodayTasks calls ListTaskLists internally, which will be cached
	// for the second call below.
	var (
		taskItems []Task
		lists     []TaskList
		taskErr   error
		listErr   error
	)
	var wg sync.WaitGroup
	wg.Add(2)
	go func() {
		defer wg.Done()
		taskItems, taskErr = client.ListTodayTasks(today)
	}()
	go func() {
		defer wg.Done()
		lists, listErr = client.ListTaskLists()
	}()
	wg.Wait()

	if taskErr != nil {
		return renderError(c, "Failed to load tasks")
	}

	ctx := c.Request().Context()
	w := c.Response()

	if err := ViewTodayContent(taskItems).Render(ctx, w); err != nil {
		return err
	}

	if listErr != nil {
		return nil
	}
	return ViewTasklistSidebarOOB(lists, "_today").Render(ctx, w)
}

func (h *Handlers) HandleRescheduleTask(c echo.Context) error {
	client := h.newClient(c)
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

func (h *Handlers) HandleMoveTask(c echo.Context) error {
	client := h.newClient(c)
	listID := c.Param("listId")
	taskID := c.Param("taskId")
	previousID := c.FormValue("previous")

	_, err := client.MoveTask(listID, taskID, previousID)
	if err != nil {
		return renderError(c, "Failed to move task")
	}

	return c.NoContent(http.StatusNoContent)
}

func (h *Handlers) HandleToggleHideCompleted(c echo.Context) error {
	client := h.newClient(c)
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

	flatTasks, err := client.ListTasks(listID, !hideCompleted)
	if err != nil {
		return renderError(c, "Failed to load tasks")
	}
	taskItems := BuildTaskTree(flatTasks)

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

// renderOOB writes an out-of-band HTMX innerHTML swap for the given element ID.
func renderOOB(ctx context.Context, w io.Writer, id string, comp templ.Component) error {
	return renderOOBSwap(ctx, w, "innerHTML", id, comp)
}

// renderOOBOuter writes an out-of-band outerHTML swap. Use when the component
// renders its own wrapper element with the target ID (e.g. TaskItem).
func renderOOBOuter(ctx context.Context, w io.Writer, id string, comp templ.Component) error {
	return renderOOBSwap(ctx, w, "outerHTML", id, comp)
}

func renderOOBSwap(ctx context.Context, w io.Writer, swap, id string, comp templ.Component) error {
	_, _ = w.Write([]byte(`<div hx-swap-oob="` + swap + `:#` + id + `">`))
	if err := comp.Render(ctx, w); err != nil {
		return err
	}
	_, _ = w.Write([]byte(`</div>`))
	return nil
}

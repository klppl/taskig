package tasks

import (
	"fmt"
	"strings"
	"sync"
	"time"

	"github.com/alex/google-tasks/internal/cache"
	gtasks "google.golang.org/api/tasks/v1"
)

const (
	taskListsTTL = 60 * time.Second
	tasksTTL     = 30 * time.Second
)

// Task is the app's representation of a Google Task.
type Task struct {
	ID        string
	Title     string
	Notes     string
	Due       string // YYYY-MM-DD or empty
	Completed bool
	Parent    string // parent task ID, empty if top-level
	ListID    string
	ListTitle string // populated in cross-list views (e.g. Today)
	Children  []Task // populated by BuildTaskTree
}

// TaskList is the app's representation of a Google TaskList.
type TaskList struct {
	ID    string
	Title string
}

// Client wraps the Google Tasks API service.
type Client struct {
	svc     *gtasks.Service
	cache   *cache.Cache
	userKey string
}

// NewClient creates a new Tasks API wrapper with caching.
func NewClient(svc *gtasks.Service, c *cache.Cache, userKey string) *Client {
	return &Client{svc: svc, cache: c, userKey: userKey}
}

func (c *Client) cacheKey(parts ...string) string {
	return c.userKey + ":" + strings.Join(parts, ":")
}

// ListTaskLists returns all task lists for the authenticated user.
func (c *Client) ListTaskLists() ([]TaskList, error) {
	key := c.cacheKey("tasklists")
	if cached, ok := c.cache.Get(key); ok {
		return cached.([]TaskList), nil
	}

	resp, err := c.svc.Tasklists.List().MaxResults(100).Do()
	if err != nil {
		return nil, fmt.Errorf("list tasklists: %w", err)
	}

	lists := make([]TaskList, 0, len(resp.Items))
	for _, item := range resp.Items {
		lists = append(lists, ToTaskList(item))
	}
	c.cache.Set(key, lists, taskListsTTL)
	return lists, nil
}

// ListTasks returns all tasks in a given task list.
func (c *Client) ListTasks(listID string, showCompleted bool) ([]Task, error) {
	key := c.cacheKey("tasks", listID, fmt.Sprintf("%v", showCompleted))
	if cached, ok := c.cache.Get(key); ok {
		return cached.([]Task), nil
	}

	resp, err := c.svc.Tasks.List(listID).ShowCompleted(showCompleted).ShowHidden(showCompleted).MaxResults(100).Do()
	if err != nil {
		return nil, fmt.Errorf("list tasks: %w", err)
	}

	tasks := make([]Task, 0, len(resp.Items))
	for _, item := range resp.Items {
		t := ToTask(item)
		t.ListID = listID
		tasks = append(tasks, t)
	}
	c.cache.Set(key, tasks, tasksTTL)
	return tasks, nil
}

// ListTodayTasks returns tasks due today or overdue across all lists.
// Fetches all lists concurrently for speed.
func (c *Client) ListTodayTasks(today string) ([]Task, error) {
	lists, err := c.ListTaskLists()
	if err != nil {
		return nil, err
	}

	type listResult struct {
		tasks []Task
		title string
	}

	results := make([]listResult, len(lists))
	var wg sync.WaitGroup
	wg.Add(len(lists))

	for i, list := range lists {
		go func(idx int, l TaskList) {
			defer wg.Done()
			tasks, err := c.ListTasks(l.ID, false)
			if err != nil {
				return
			}
			results[idx] = listResult{tasks: tasks, title: l.Title}
		}(i, list)
	}
	wg.Wait()

	var result []Task
	for _, lr := range results {
		for _, t := range lr.tasks {
			if t.Due != "" && t.Due <= today {
				t.ListTitle = lr.title
				result = append(result, t)
			}
		}
	}
	return result, nil
}

// ListSubtasks returns direct children of a task.
func (c *Client) ListSubtasks(listID, parentID string) ([]Task, error) {
	all, err := c.ListTasks(listID, true)
	if err != nil {
		return nil, err
	}
	var children []Task
	for _, t := range all {
		if t.Parent == parentID {
			children = append(children, t)
		}
	}
	return children, nil
}

// CreateTask creates a new task in the given list.
func (c *Client) CreateTask(listID, title, notes string) (*Task, error) {
	return c.insertTask(listID, title, notes, "")
}

// CreateSubtask creates a new task as a child of the given parent task.
func (c *Client) CreateSubtask(listID, parentID, title string) (*Task, error) {
	return c.insertTask(listID, title, "", parentID)
}

func (c *Client) insertTask(listID, title, notes, parentID string) (*Task, error) {
	gt := &gtasks.Task{
		Title: title,
		Notes: notes,
	}
	call := c.svc.Tasks.Insert(listID, gt)
	if parentID != "" {
		call = call.Parent(parentID)
	}
	created, err := call.Do()
	if err != nil {
		return nil, fmt.Errorf("create task: %w", err)
	}
	c.invalidateList(listID)
	t := ToTask(created)
	t.ListID = listID
	return &t, nil
}

// UpdateTask updates a task's title, notes, and due date.
func (c *Client) UpdateTask(listID, taskID, title, notes, due string) (*Task, error) {
	gt := &gtasks.Task{
		Id:    taskID,
		Title: title,
		Notes: notes,
	}
	if due != "" {
		gt.Due = due + "T00:00:00.000Z"
	} else {
		gt.Due = ""
		gt.ForceSendFields = append(gt.ForceSendFields, "Due")
	}
	updated, err := c.svc.Tasks.Patch(listID, taskID, gt).Do()
	if err != nil {
		return nil, fmt.Errorf("update task: %w", err)
	}
	c.invalidateList(listID)
	t := ToTask(updated)
	t.ListID = listID
	return &t, nil
}

// PatchDueDate updates only the due date of a task.
func (c *Client) PatchDueDate(listID, taskID, due string) (*Task, error) {
	gt := &gtasks.Task{
		Due: due + "T00:00:00.000Z",
	}
	updated, err := c.svc.Tasks.Patch(listID, taskID, gt).Do()
	if err != nil {
		return nil, fmt.Errorf("patch due date: %w", err)
	}
	c.invalidateList(listID)
	t := ToTask(updated)
	t.ListID = listID
	return &t, nil
}

// CompleteTask toggles a task's completion status.
func (c *Client) CompleteTask(listID, taskID string, completed bool) (*Task, error) {
	status := "needsAction"
	if completed {
		status = "completed"
	}
	gt := &gtasks.Task{
		Id:     taskID,
		Status: status,
	}
	if !completed {
		gt.ForceSendFields = []string{"Completed"}
	}
	updated, err := c.svc.Tasks.Patch(listID, taskID, gt).Do()
	if err != nil {
		return nil, fmt.Errorf("complete task: %w", err)
	}
	c.invalidateList(listID)
	t := ToTask(updated)
	t.ListID = listID
	return &t, nil
}

// MoveTask repositions a task within a list. previousID is the task ID to
// place this task after; empty string moves it to the top.
func (c *Client) MoveTask(listID, taskID, previousID string) (*Task, error) {
	call := c.svc.Tasks.Move(listID, taskID)
	if previousID != "" {
		call = call.Previous(previousID)
	}
	moved, err := call.Do()
	if err != nil {
		return nil, fmt.Errorf("move task: %w", err)
	}
	c.invalidateList(listID)
	t := ToTask(moved)
	t.ListID = listID
	return &t, nil
}

// MoveTaskToList moves a task (and its subtasks) from one list to another by
// recreating them in the destination list and deleting from the source.
func (c *Client) MoveTaskToList(srcListID, taskID, dstListID string) (*Task, error) {
	// Get the original task
	src, err := c.svc.Tasks.Get(srcListID, taskID).Do()
	if err != nil {
		return nil, fmt.Errorf("get task for move: %w", err)
	}

	// Get subtasks before we delete anything
	subtasks, _ := c.ListSubtasks(srcListID, taskID)

	// Create parent in destination list
	dst := &gtasks.Task{
		Title: src.Title,
		Notes: src.Notes,
		Due:   src.Due,
	}
	if src.Status == "completed" {
		dst.Status = "completed"
	}
	created, err := c.svc.Tasks.Insert(dstListID, dst).Do()
	if err != nil {
		return nil, fmt.Errorf("insert task in destination list: %w", err)
	}

	// Recreate subtasks under the new parent
	var movedChildren []Task
	for _, sub := range subtasks {
		childDst := &gtasks.Task{
			Title: sub.Title,
			Notes: sub.Notes,
		}
		if sub.Due != "" {
			childDst.Due = sub.Due + "T00:00:00.000Z"
		}
		if sub.Completed {
			childDst.Status = "completed"
		}
		childCreated, err := c.svc.Tasks.Insert(dstListID, childDst).Parent(created.Id).Do()
		if err != nil {
			continue // best effort — don't fail the whole move for a subtask
		}
		ct := ToTask(childCreated)
		ct.ListID = dstListID
		movedChildren = append(movedChildren, ct)

		// Delete subtask from source
		_ = c.svc.Tasks.Delete(srcListID, sub.ID).Do()
	}

	// Delete parent from source list
	_ = c.svc.Tasks.Delete(srcListID, taskID).Do()

	c.invalidateList(srcListID)
	c.invalidateList(dstListID)

	t := ToTask(created)
	t.ListID = dstListID
	t.Children = movedChildren
	return &t, nil
}

// DeleteTask deletes a task from the given list.
func (c *Client) DeleteTask(listID, taskID string) error {
	if err := c.svc.Tasks.Delete(listID, taskID).Do(); err != nil {
		return fmt.Errorf("delete task: %w", err)
	}
	c.invalidateList(listID)
	return nil
}

// GetTask retrieves a single task.
func (c *Client) GetTask(listID, taskID string) (*Task, error) {
	gt, err := c.svc.Tasks.Get(listID, taskID).Do()
	if err != nil {
		return nil, fmt.Errorf("get task: %w", err)
	}
	t := ToTask(gt)
	t.ListID = listID
	return &t, nil
}

// invalidateList removes cached task data for a list.
func (c *Client) invalidateList(listID string) {
	c.cache.InvalidatePrefix(c.cacheKey("tasks", listID))
}

// ToTask converts a Google Tasks API task to our Task type.
func ToTask(gt *gtasks.Task) Task {
	due := ""
	if gt.Due != "" {
		// gt.Due is RFC 3339 like "2026-03-25T00:00:00.000Z"; extract date part
		if idx := strings.IndexByte(gt.Due, 'T'); idx > 0 {
			due = gt.Due[:idx]
		} else {
			due = gt.Due
		}
	}
	return Task{
		ID:        gt.Id,
		Title:     gt.Title,
		Notes:     gt.Notes,
		Due:       due,
		Completed: gt.Status == "completed",
		Parent:    gt.Parent,
	}
}

// BuildTaskTree groups a flat task list into a tree based on Parent fields.
// The API returns tasks in position order, so we preserve that ordering.
func BuildTaskTree(tasks []Task) []Task {
	childrenOf := make(map[string][]Task)
	var roots []Task
	for _, t := range tasks {
		if t.Parent == "" {
			roots = append(roots, t)
		} else {
			childrenOf[t.Parent] = append(childrenOf[t.Parent], t)
		}
	}
	for i := range roots {
		attachChildren(&roots[i], childrenOf)
	}
	return roots
}

func attachChildren(t *Task, childrenOf map[string][]Task) {
	children := childrenOf[t.ID]
	if len(children) == 0 {
		return
	}
	t.Children = children
	for i := range t.Children {
		attachChildren(&t.Children[i], childrenOf)
	}
}

// ToTaskList converts a Google Tasks API tasklist to our TaskList type.
func ToTaskList(gl *gtasks.TaskList) TaskList {
	return TaskList{
		ID:    gl.Id,
		Title: gl.Title,
	}
}

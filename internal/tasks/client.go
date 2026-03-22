package tasks

import (
	"fmt"
	"strings"

	gtasks "google.golang.org/api/tasks/v1"
)

// Task is the app's representation of a Google Task.
type Task struct {
	ID        string
	Title     string
	Notes     string
	Due       string // YYYY-MM-DD or empty
	Completed bool
	ListID    string
	ListTitle string // populated in cross-list views (e.g. Today)
}

// TaskList is the app's representation of a Google TaskList.
type TaskList struct {
	ID    string
	Title string
}

// Client wraps the Google Tasks API service.
type Client struct {
	svc *gtasks.Service
}

// NewClient creates a new Tasks API wrapper.
func NewClient(svc *gtasks.Service) *Client {
	return &Client{svc: svc}
}

// ListTaskLists returns all task lists for the authenticated user.
func (c *Client) ListTaskLists() ([]TaskList, error) {
	resp, err := c.svc.Tasklists.List().MaxResults(100).Do()
	if err != nil {
		return nil, fmt.Errorf("list tasklists: %w", err)
	}

	lists := make([]TaskList, 0, len(resp.Items))
	for _, item := range resp.Items {
		lists = append(lists, ToTaskList(item))
	}
	return lists, nil
}

// ListTasks returns all tasks in a given task list.
func (c *Client) ListTasks(listID string, showCompleted bool) ([]Task, error) {
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
	return tasks, nil
}

// ListTodayTasks returns tasks due today or overdue across all lists.
func (c *Client) ListTodayTasks(today string) ([]Task, error) {
	lists, err := c.ListTaskLists()
	if err != nil {
		return nil, err
	}

	var result []Task
	for _, list := range lists {
		tasks, err := c.ListTasks(list.ID, false)
		if err != nil {
			continue
		}
		for _, t := range tasks {
			if t.Due != "" && t.Due <= today {
				t.ListTitle = list.Title
				result = append(result, t)
			}
		}
	}
	return result, nil
}

// CreateTask creates a new task in the given list.
func (c *Client) CreateTask(listID, title, notes string) (*Task, error) {
	gt := &gtasks.Task{
		Title: title,
		Notes: notes,
	}
	created, err := c.svc.Tasks.Insert(listID, gt).Do()
	if err != nil {
		return nil, fmt.Errorf("create task: %w", err)
	}
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
	t := ToTask(updated)
	t.ListID = listID
	return &t, nil
}

// PatchDueDate updates only the due date of a task.
func (c *Client) PatchDueDate(listID, taskID, due string) (*Task, error) {
	gt := &gtasks.Task{
		Id:  taskID,
		Due: due + "T00:00:00.000Z",
	}
	updated, err := c.svc.Tasks.Patch(listID, taskID, gt).Do()
	if err != nil {
		return nil, fmt.Errorf("patch due date: %w", err)
	}
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
	t := ToTask(moved)
	t.ListID = listID
	return &t, nil
}

// DeleteTask deletes a task from the given list.
func (c *Client) DeleteTask(listID, taskID string) error {
	if err := c.svc.Tasks.Delete(listID, taskID).Do(); err != nil {
		return fmt.Errorf("delete task: %w", err)
	}
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
	}
}

// ToTaskList converts a Google Tasks API tasklist to our TaskList type.
func ToTaskList(gl *gtasks.TaskList) TaskList {
	return TaskList{
		ID:    gl.Id,
		Title: gl.Title,
	}
}

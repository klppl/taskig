package tasks

import (
	"testing"

	gtasks "google.golang.org/api/tasks/v1"
)

func TestToTask(t *testing.T) {
	gt := &gtasks.Task{
		Id:      "task1",
		Title:   "Buy milk",
		Notes:   "2% only",
		Status:  "needsAction",
		Updated: "2026-03-21T10:00:00.000Z",
	}

	task := ToTask(gt)
	if task.ID != "task1" {
		t.Fatalf("ID: got %s, want task1", task.ID)
	}
	if task.Title != "Buy milk" {
		t.Fatalf("Title: got %s, want 'Buy milk'", task.Title)
	}
	if task.Notes != "2% only" {
		t.Fatalf("Notes: got %s, want '2%% only'", task.Notes)
	}
	if task.Completed {
		t.Fatal("expected not completed")
	}
}

func TestToTaskCompleted(t *testing.T) {
	gt := &gtasks.Task{
		Id:     "task2",
		Title:  "Done thing",
		Status: "completed",
	}

	task := ToTask(gt)
	if !task.Completed {
		t.Fatal("expected completed")
	}
}

func TestToTaskList(t *testing.T) {
	gl := &gtasks.TaskList{
		Id:    "list1",
		Title: "My Tasks",
	}

	list := ToTaskList(gl)
	if list.ID != "list1" {
		t.Fatalf("ID: got %s, want list1", list.ID)
	}
	if list.Title != "My Tasks" {
		t.Fatalf("Title: got %s, want 'My Tasks'", list.Title)
	}
}

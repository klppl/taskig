package tasks

import "github.com/a-h/templ"

// View functions are set by the application during initialization to avoid
// an import cycle between the tasks and templates packages.
var (
	ViewDashboardPage      func(taskLists []TaskList, activeListID string, activeTasks []Task, hideCompleted bool) templ.Component
	ViewTaskListContent    func(listID string, listTitle string, taskItems []Task, hideCompleted bool) templ.Component
	ViewTaskItem           func(listID string, task Task, inTodayView bool) templ.Component
	ViewTaskDetailPanel    func(listID string, task Task, taskLists []TaskList) templ.Component
	ViewTaskDetailEmpty    func() templ.Component
	ViewTasklistSidebarOOB func(taskLists []TaskList, activeID string) templ.Component
	ViewTodayContent       func(taskItems []Task) templ.Component
	ViewTaskTree           func(listID string, task Task, depth int) templ.Component
)

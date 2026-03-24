# Drag Task to Sidebar List

## Overview

Allow users to drag a task from the task list panel and drop it onto a sidebar list item to move it to that list. Desktop only; single-list views only (not Today). No backend changes required.

## Interaction Flow

1. User starts dragging a task item in the middle column
2. Sidebar list items highlight as valid/invalid drop targets:
   - Valid: all lists except the current one — shown with a blue border/background
   - Invalid: the current list and Today — dimmed with reduced opacity
3. User drops the task on a valid sidebar list item
4. Frontend calls `POST /api/tasklists/{srcListId}/tasks/{taskId}/move-to-list` with `destListId`
5. Existing handler removes the task from the source list (OOB delete), updates the detail panel if open
6. On drag cancel or drag end without valid drop, all highlight classes are removed

## Technical Approach

### SortableJS Configuration

The existing Sortable instance on `#task-list` is extended with the `group` option to allow pulling tasks out:

```js
group: { name: 'tasks', pull: 'clone', put: false }
```

Using `pull: 'clone'` so SortableJS doesn't remove the DOM element itself — the server response handles removal via OOB swap.

Each sidebar list item (except Today and the active list) gets a minimal Sortable instance configured as a drop zone:

```js
Sortable.create(sidebarItem, {
  group: { name: 'tasks', pull: false, put: true },
  onAdd: function(evt) { /* trigger move-to-list */ }
});
```

### Visual Feedback

On `onStart` (drag begins):
- Add `drag-active` class to `#tasklist-sidebar`
- Each sidebar list item that is a valid target gets `drop-target` class
- Current list and Today get `drop-disabled` class

On `onEnd` (drag ends, regardless of outcome):
- Remove all `drag-active`, `drop-target`, `drop-disabled` classes

CSS classes (added to Tailwind):
- `.drop-target`: `ring-2 ring-blue-400 bg-blue-50 dark:bg-blue-900/20`
- `.drop-disabled`: `opacity-40 pointer-events-none`

### Move Request

On `onAdd` in a sidebar drop zone:
```js
fetch('/api/tasklists/' + srcListId + '/tasks/' + taskId + '/move-to-list', {
  method: 'POST',
  headers: {
    'Content-Type': 'application/x-www-form-urlencoded',
    'X-Requested-With': 'XMLHttpRequest'
  },
  body: 'destListId=' + encodeURIComponent(destListId)
})
```

The response uses HTMX OOB swaps to delete the task element and update the detail panel. Since we're using `fetch` (not HTMX), we process the response by injecting the HTML into the DOM to trigger OOB swap processing via `htmx.process()`.

### Alternative: HTMX-driven approach

Instead of raw `fetch`, we can trigger an HTMX request programmatically to let HTMX handle OOB swaps natively:

```js
htmx.ajax('POST', '/api/tasklists/' + srcListId + '/tasks/' + taskId + '/move-to-list', {
  target: '#task-' + taskId,
  swap: 'delete',
  values: { destListId: destListId },
  headers: { 'X-Requested-With': 'XMLHttpRequest' }
})
```

This is preferred since the existing handler response already uses OOB swaps that HTMX processes automatically.

## Scope Boundaries

- **Desktop only.** On mobile the sidebar is hidden; users use the existing dropdown in the detail panel.
- **Single-list views only.** Disabled in Today view (no Sortable group config applied when in Today).
- **Subtasks move automatically.** Already handled by `MoveTaskToList` in `client.go`.
- **No new backend endpoints.** Reuses `HandleMoveTaskToList`.
- **No changes to within-list reordering.** Existing drag-to-reorder continues to work.

## Error Handling

- On move failure: reload the page (matches existing pattern in the Sortable `onEnd` handler).
- Remove cloned element from sidebar on completion (SortableJS clone artifact).

## Files to Modify

| File | Change |
|------|--------|
| `templates/task_list.templ` | Extend Sortable init: add `group` option, drag start/end callbacks for sidebar highlighting |
| `templates/tasklist_sidebar.templ` | Add `data-list-id` attributes to sidebar list items for drop zone identification |
| `static/css/app.css` | Add `.drop-target` and `.drop-disabled` utility classes |

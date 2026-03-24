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

Each sidebar list item (except Today and the active list) gets a minimal Sortable instance configured as a drop zone. Sidebar items are `<button>` elements — SortableJS can use them as containers for receiving drops.

```js
Sortable.create(sidebarItem, {
  group: { name: 'tasks', pull: false, put: true },
  onAdd: function(evt) {
    evt.item.remove(); // Remove cloned task element from sidebar button immediately
    // ... trigger move-to-list request
  }
});
```

### Sidebar Drop Zone Re-initialization

Sidebar contents are replaced via OOB swap on every list navigation (`TasklistSidebarOOB`). This destroys any Sortable instances on sidebar items. The `sortableInit()` script in `task_list.templ` already runs on every list load — sidebar drop zone setup is included there, so it re-initializes after each OOB sidebar swap in the same response.

### Visual Feedback

On `onStart` (drag begins):
- Add `drag-active` class to `#tasklist-sidebar`
- Each sidebar list item that is a valid target gets `drop-target` class
- Current list and Today button get `drop-disabled` class
- Today button identified via `data-list-id="_today"` attribute

On `onEnd` (drag ends, regardless of outcome):
- Remove all `drag-active`, `drop-target`, `drop-disabled` classes

CSS classes (raw CSS in `app.css`, not Tailwind utilities, since these classes are only added via JS and wouldn't be picked up by Tailwind's scanner):

```css
.drop-target {
  outline: 2px solid #60a5fa; /* blue-400 */
  background: rgba(59, 130, 246, 0.08);
}
.dark .drop-target {
  background: rgba(59, 130, 246, 0.15);
}
.drop-disabled {
  opacity: 0.4;
  pointer-events: none;
}
```

### Move Request

Use `htmx.ajax()` so HTMX handles OOB swaps natively. The handler response contains only OOB fragments (a delete swap for the task element and an innerHTML swap for the detail panel), so we use `swap: 'none'` to avoid any primary swap — OOB processing handles everything:

```js
htmx.ajax('POST', '/api/tasklists/' + srcListId + '/tasks/' + taskId + '/move-to-list', {
  swap: 'none',
  values: { destListId: destListId },
  headers: { 'X-Requested-With': 'XMLHttpRequest' }
}).catch(function() { location.reload(); });
```

## Scope Boundaries

- **Desktop only.** On mobile the sidebar is hidden; users use the existing dropdown in the detail panel.
- **Single-list views only.** Disabled in Today view (no Sortable group config applied when in Today).
- **Only top-level tasks are draggable.** Subtasks are nested inside parent divs and are not direct children of `#task-list`, so SortableJS won't pick them up independently. When a parent is dragged, its subtasks move with it (handled by `MoveTaskToList`).
- **Completed tasks are not draggable.** The completed section is a separate container outside `#task-list`.
- **No new backend endpoints.** Reuses `HandleMoveTaskToList`.
- **No changes to within-list reordering.** Existing drag-to-reorder continues to work.

## Error Handling

- On move failure: reload the page (via `.catch()` on the `htmx.ajax()` promise, matches existing pattern).
- Clone cleanup: `evt.item.remove()` in `onAdd` before triggering the request, preventing stale cloned elements in the sidebar.

## Files to Modify

| File | Change |
|------|--------|
| `templates/task_list.templ` | Extend `sortableInit()`: add `group` option, `onStart`/`onEnd` callbacks for sidebar highlighting, sidebar drop zone initialization |
| `templates/tasklist_sidebar.templ` | Add `data-list-id` attributes to sidebar list items and `data-list-id="_today"` to the Today button |
| `static/css/app.css` | Add `.drop-target` and `.drop-disabled` CSS classes (raw CSS, not Tailwind utilities) |

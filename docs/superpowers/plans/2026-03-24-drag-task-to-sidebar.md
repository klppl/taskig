# Drag Task to Sidebar Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Allow users to drag a task from the task list and drop it on a sidebar list item to move it to that list.

**Architecture:** Extend the existing SortableJS setup with `group` config to allow cross-container dragging. Each sidebar list item becomes a drop zone. The existing `HandleMoveTaskToList` endpoint handles the backend. No new Go code needed — this is purely a frontend change across 3 files.

**Tech Stack:** SortableJS (already loaded), HTMX (`htmx.ajax`), raw CSS

**Spec:** `docs/superpowers/specs/2026-03-24-drag-task-to-sidebar-design.md`

---

### Task 1: Add `data-list-id` attributes to sidebar list items

**Files:**
- Modify: `templates/tasklist_sidebar.templ:10-48`

The sidebar buttons need `data-list-id` attributes so the JS can identify which list a drop target represents. The Today button needs `data-list-id="_today"` so it can be identified and dimmed.

- [ ] **Step 1: Add `data-list-id="_today"` to the Today button**

In `templates/tasklist_sidebar.templ`, add the attribute to the Today `<button>` (line 10):

```templ
	<button
		data-list-id="_today"
		hx-get="/api/today"
```

- [ ] **Step 2: Add `data-list-id` to each list button**

In `templates/tasklist_sidebar.templ`, add the attribute to the list `<button>` (line 28):

```templ
	for _, list := range lists {
		<button
			data-list-id={ list.ID }
			hx-get={ "/api/tasklists/" + list.ID + "/tasks?title=" + list.Title }
```

- [ ] **Step 3: Regenerate templ**

Run: `templ generate`
Expected: no errors

- [ ] **Step 4: Verify data attributes render**

Run the app (`make dev` or `make run`), open the dashboard, inspect the sidebar in browser devtools. Each sidebar button should have a `data-list-id` attribute with the list ID or `_today`.

- [ ] **Step 5: Commit**

```bash
git add templates/tasklist_sidebar.templ templates/tasklist_sidebar_templ.go
git commit -m "ui: add data-list-id attributes to sidebar items for drop targeting"
```

---

### Task 2: Add drop target CSS classes

**Files:**
- Modify: `static/css/app.css:81` (append after existing styles)

These classes are toggled via JS only (not in templ files), so they must be raw CSS — Tailwind's scanner won't find them.

- [ ] **Step 1: Add `.drop-target` and `.drop-disabled` classes**

Append to the end of `static/css/app.css`:

```css
/* === Cross-List Drag Drop Zones === */

.drop-target {
  outline: 2px solid #60a5fa;
  outline-offset: -2px;
  background: rgba(59, 130, 246, 0.08);
  border-radius: 0.5rem;
}

.dark .drop-target {
  background: rgba(59, 130, 246, 0.15);
}

.drop-disabled {
  opacity: 0.4;
  pointer-events: none;
}
```

- [ ] **Step 2: Rebuild CSS**

Run: `make css` (or `npx @tailwindcss/cli -i static/css/app.css -o static/css/dist.css`)
Expected: builds successfully, `dist.css` contains the new classes.

- [ ] **Step 3: Commit**

```bash
git add static/css/app.css
git commit -m "ui: add drop-target and drop-disabled CSS classes for cross-list drag"
```

---

### Task 3: Extend SortableJS with cross-list drag support

**Files:**
- Modify: `templates/task_list.templ:290-322`

This is the main change. The existing `sortableInit()` script is extended to:
1. Add `group` config to the task list Sortable (allows pulling tasks out)
2. On drag start, highlight valid sidebar drop targets and dim invalid ones
3. On drag end, clean up all highlight classes
4. Create Sortable drop zones on each valid sidebar list item
5. On drop (`onAdd`), remove the clone and fire `htmx.ajax` to move the task

- [ ] **Step 1: Replace the `sortableInit()` templ component**

Replace the entire `sortableInit()` in `templates/task_list.templ` (lines 290-322) with:

```templ
templ sortableInit() {
	<script>
		(function() {
			const list = document.getElementById('task-list');
			if (!list || !list.dataset.listId) return;
			const currentListId = list.dataset.listId;

			// Destroy previous instances
			if (list._sortable) list._sortable.destroy();
			if (window._sidebarSortables) {
				window._sidebarSortables.forEach(function(s) { s.destroy(); });
			}
			window._sidebarSortables = [];

			// Helpers for sidebar highlight
			function highlightSidebar() {
				var items = document.querySelectorAll('#tasklist-sidebar [data-list-id]');
				for (var i = 0; i < items.length; i++) {
					var el = items[i];
					var id = el.dataset.listId;
					if (id === currentListId || id === '_today') {
						el.classList.add('drop-disabled');
					} else {
						el.classList.add('drop-target');
					}
				}
			}

			function clearSidebar() {
				var items = document.querySelectorAll('#tasklist-sidebar [data-list-id]');
				for (var i = 0; i < items.length; i++) {
					items[i].classList.remove('drop-target', 'drop-disabled');
				}
			}

			// Within-list reorder + cross-list pull
			list._sortable = Sortable.create(list, {
				animation: 150,
				ghostClass: 'opacity-30',
				filter: '.group\\/form',
				preventOnFilter: false,
				delay: 150,
				delayOnTouchOnly: true,
				group: { name: 'tasks', pull: 'clone', put: false },
				onStart: function() { highlightSidebar(); },
				onEnd: function(evt) {
					clearSidebar();
					// If dropped back in the same list, handle reorder
					if (evt.to === list && evt.oldIndex !== evt.newIndex) {
						var item = evt.item;
						var taskId = item.dataset.taskId;
						var prev = item.previousElementSibling;
						var previousId = (prev && prev.dataset.taskId) || '';
						fetch('/api/tasklists/' + currentListId + '/tasks/' + taskId + '/move', {
							method: 'POST',
							headers: {
								'Content-Type': 'application/x-www-form-urlencoded',
								'X-Requested-With': 'XMLHttpRequest'
							},
							body: 'previous=' + encodeURIComponent(previousId)
						}).catch(function() { location.reload(); });
					}
				}
			});

			// Sidebar drop zones
			var sidebarItems = document.querySelectorAll('#tasklist-sidebar [data-list-id]');
			for (var i = 0; i < sidebarItems.length; i++) {
				var el = sidebarItems[i];
				var destId = el.dataset.listId;
				if (destId === currentListId || destId === '_today') continue;

				var s = Sortable.create(el, {
					group: { name: 'tasks', pull: false, put: true },
					onAdd: function(evt) {
						var taskId = evt.item.dataset.taskId;
						var destListId = evt.to.dataset.listId;
						evt.item.remove();
						clearSidebar();
						htmx.ajax('POST',
							'/api/tasklists/' + currentListId + '/tasks/' + taskId + '/move-to-list',
							{ swap: 'none', values: { destListId: destListId } }
						).catch(function() { location.reload(); });
					}
				});
				window._sidebarSortables.push(s);
			}
		})();
	</script>
}
```

- [ ] **Step 2: Regenerate templ**

Run: `templ generate`
Expected: no errors

- [ ] **Step 3: Manual test — drag a task to a different list**

1. Run the app (`make dev`)
2. Open a task list with multiple tasks
3. Start dragging a task — sidebar items should highlight (blue outline on valid targets, dimmed on current list and Today)
4. Drop the task on a different list — the task should disappear from the current list
5. Navigate to the destination list — the task should be there
6. Verify within-list reordering still works (drag a task up/down within the same list)

- [ ] **Step 4: Manual test — edge cases**

1. Drag and release without dropping on a target — highlights should clear, nothing moves
2. Open a task's detail panel, then drag that task to another list — detail panel should update via OOB swap
3. Try in Today view — Sortable should not initialize (no `data-list-id` on `#task-list` in Today view, so the early return `if (!list || !list.dataset.listId) return` applies)

- [ ] **Step 5: Commit**

```bash
git add templates/task_list.templ templates/task_list_templ.go
git commit -m "feat: drag tasks to sidebar lists for cross-list move"
```

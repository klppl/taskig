# CLAUDE.md

This file provides guidance to Claude Code (claude.ai/code) when working with code in this repository.

## Build & Run

```bash
# Docker (production)
docker compose up --build -d

# Local development
make generate          # Generate templ templates
make css               # Build Tailwind CSS (watch mode)
make dev               # Live reload with air
make run               # Run server directly
make build             # Build static binary (CGO_ENABLED=0)
```

The Docker build is three-stage: Node (Tailwind CSS) → Go (templ generate + compile) → Alpine runtime.

Tailwind CSS v4 uses CSS-based config (`@import "tailwindcss"` in `static/css/app.css`), not `tailwind.config.js`. It scans `.templ` files directly for class names.

## Architecture

**Stack:** Go (Echo), templ templates, HTMX, Tailwind CSS, SQLite (pure Go via modernc.org/sqlite)

**No JSON API.** All endpoints return HTML fragments. HTMX swaps them into the DOM. State-changing requests require an `X-Requested-With: XMLHttpRequest` header (set globally in `layout.templ`, enforced by `RequireXHR` middleware).

**No local task storage.** All task data comes from the Google Tasks API per request. SQLite only stores sessions (with encrypted OAuth tokens).

### Import cycle workaround

The `tasks` package defines data types (`Task`, `TaskList`) and handlers. The `templates` package imports `tasks` for those types. Handlers need to call template functions but can't import `templates` back.

Solution: `internal/tasks/views.go` declares view functions as package-level variables. `cmd/server/main.go` wires them at startup:

```go
tasks.ViewDashboardPage = templates.DashboardPage
```

**When adding a new template that handlers need to call:** add a variable in `views.go`, wire it in `main.go`.

### OOB swaps

Multiple UI regions update in a single response using HTMX out-of-band swaps. The primary response targets `hx-target`, and additional `<div hx-swap-oob="innerHTML:#id">` elements update other panels.

Use `renderOOB()` helper in `handlers.go` for this pattern. Examples: sidebar highlight updates when switching lists, task item updates when saving from the detail panel.

### Three-column layout

Sidebar (`#tasklist-sidebar`) → Task list (`#task-panel`) → Detail panel (`#detail-panel`). Clicking a task title loads its editable detail in the right panel via `hx-target="#detail-panel"`.

### Session store

Custom gorilla/sessions implementation backed by SQLite. OAuth tokens are AES-256-GCM encrypted. Signing and encryption keys are derived from `SESSION_SECRET` via HKDF-SHA256. OAuth state uses a separate short-lived cookie (not the session store).

## Key files

- `cmd/server/main.go` — route registration, view function wiring
- `internal/tasks/views.go` — view function variables (import cycle bridge)
- `internal/tasks/handlers.go` — all task HTTP handlers, `renderOOB()` helper
- `internal/tasks/client.go` — Google Tasks API wrapper, `Task`/`TaskList` types
- `internal/auth/middleware.go` — auth middleware, injects Tasks client into context
- `templates/` — `.templ` files (generate `_templ.go` via `templ generate`)

## Conventions

- Handlers get the API client from context: `svc := auth.GetTasksClient(c)`
- Use `Tasks.Patch()` not `Tasks.Update()` for partial updates (Update is PUT and clears unset fields)
- User preferences (e.g. `hide_completed`) are stored as HTTP cookies, not in the database
- The `_today` sentinel ID represents the cross-list "Today" view in sidebar/URL state
- `task.ListTitle` is only populated in cross-list views (Today) and must be threaded through request params to survive round-trips

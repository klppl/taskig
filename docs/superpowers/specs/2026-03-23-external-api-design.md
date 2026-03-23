# External API Design

## Overview

Add a JSON API (`/api/v1/`) for external tools (browser extensions, scripts) to create tasks. Authenticated via API keys, separate from the existing HTMX internal API.

## Authentication

- New `internal/apikeys` package
- SQLite table `api_keys`:
  - `id` TEXT PRIMARY KEY — random UUID
  - `user_email` TEXT NOT NULL
  - `key_hash` TEXT NOT NULL — SHA-256 of the raw key
  - `name` TEXT — optional label (e.g. "chrome extension")
  - `created_at` DATETIME NOT NULL
- Key format: `tsk_` + 32 random hex chars
- Only the hash is stored; raw key shown once at creation
- Middleware reads `Authorization: Bearer tsk_...`, hashes it, looks up user email
- User's stored OAuth token (from their web session) is used to call Google Tasks API
- Requires the user to have logged in via web UI at least once

## Endpoints

### POST `/api/v1/tasks` — Create a task

Auth: API key (Bearer token)

Request:
```json
{
  "title": "https://example.com/article",    // required
  "list_id": "MDk3...",                       // at least one of list_id/list_name required
  "list_name": "links",                       // case-insensitive match
  "notes": "optional",
  "due": "2026-03-25"                         // optional, YYYY-MM-DD
}
```

Response: `201 Created`
```json
{
  "id": "abc123",
  "title": "https://example.com/article",
  "notes": "",
  "due": "",
  "completed": false,
  "list_id": "MDk3..."
}
```

If both `list_id` and `list_name` provided, `list_id` takes precedence.

### GET `/api/v1/lists` — List task lists

Auth: API key (Bearer token)

Response: `200 OK`
```json
{
  "lists": [
    {"id": "MDk3...", "title": "links"},
    {"id": "XYz...", "title": "My Tasks"}
  ]
}
```

### POST `/api/v1/keys` — Generate API key

Auth: Web session (existing cookie auth)

Request:
```json
{
  "name": "chrome extension"   // optional label
}
```

Response: `201 Created`
```json
{
  "key": "tsk_a1b2c3d4...",
  "name": "chrome extension",
  "message": "Save this key — it won't be shown again."
}
```

### DELETE `/api/v1/keys/:id` — Revoke API key

Auth: Web session (existing cookie auth)

Response: `204 No Content`

## Error Responses

All errors return JSON:
```json
{"error": "description", "code": 404}
```

| Status | Meaning |
|--------|---------|
| 400 | Missing required field (title, list) |
| 401 | Missing or invalid API key |
| 404 | List name not found |
| 500 | Google API failure |

## New Packages & Files

- `internal/apikeys/` — key generation, hashing, DB CRUD, auth middleware
- `migrations/` — new migration for `api_keys` table
- `cmd/server/main.go` — wire `/api/v1/` route group
- `docs/api.md` — user-facing API documentation with curl examples

## Design Decisions

- **Separate route group** (`/api/v1/`) from HTMX routes (`/api/`) — HTMX routes return HTML fragments, external API returns JSON. Mixing would be confusing.
- **API key auth** over OAuth/session — browser extensions can't share session cookies cross-origin. API keys are simple and appropriate for personal use.
- **No key expiration** — YAGNI for a personal app. Keys can be revoked via DELETE.
- **No key listing endpoint** — YAGNI. Create and delete are sufficient.
- **User must have web session** — API key maps to a user whose OAuth token is stored in the session DB. The Google API calls use that token.

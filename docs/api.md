# Taskig API

A JSON API for creating tasks from external tools like browser extensions or scripts.

## Authentication

The API uses API keys for authentication. Keys are prefixed with `tsk_` and sent as Bearer tokens.

**Prerequisites:** You must log in via the web UI at least once before using the API. The API key uses your stored OAuth session to access Google Tasks.

### Creating an API Key

From the web UI (you must be logged in), call:

```bash
curl -X POST http://localhost:8080/api/v1/keys \
  -H "Content-Type: application/json" \
  -b "session=<your-session-cookie>" \
  -d '{"name": "chrome extension"}'
```

Response:
```json
{
  "id": "a1b2c3d4...",
  "key": "tsk_e5f6a7b8...",
  "name": "chrome extension",
  "message": "Save this key — it won't be shown again."
}
```

Save the `key` value — it cannot be retrieved later.

### Using an API Key

Include it as a Bearer token in the `Authorization` header:

```bash
curl http://localhost:8080/api/v1/lists \
  -H "Authorization: Bearer tsk_e5f6a7b8..."
```

### Revoking an API Key

```bash
curl -X DELETE http://localhost:8080/api/v1/keys/<key-id> \
  -b "session=<your-session-cookie>"
```

Returns `204 No Content` on success.

---

## Endpoints

### POST `/api/v1/tasks` — Create a Task

Add a task to a specific list. Ideal for saving links from a browser extension.

**Request:**

```bash
curl -X POST http://localhost:8080/api/v1/tasks \
  -H "Authorization: Bearer tsk_<your-key>" \
  -H "Content-Type: application/json" \
  -d '{
    "title": "https://example.com/interesting-article",
    "list_name": "links"
  }'
```

**Body fields:**

| Field | Type | Required | Description |
|-------|------|----------|-------------|
| `title` | string | Yes | Task title (e.g. a URL) |
| `list_id` | string | One of `list_id` or `list_name` | Google Tasks list ID |
| `list_name` | string | One of `list_id` or `list_name` | List name (case-insensitive match) |
| `notes` | string | No | Task notes/description |
| `due` | string | No | Due date in `YYYY-MM-DD` format |

If both `list_id` and `list_name` are provided, `list_id` takes precedence.

**Response:** `201 Created`

```json
{
  "id": "dGFzay0x...",
  "title": "https://example.com/interesting-article",
  "notes": "",
  "due": "",
  "completed": false,
  "list_id": "MDk3NTI..."
}
```

### GET `/api/v1/lists` — List Task Lists

Discover your task lists and their IDs.

**Request:**

```bash
curl http://localhost:8080/api/v1/lists \
  -H "Authorization: Bearer tsk_<your-key>"
```

**Response:** `200 OK`

```json
{
  "lists": [
    {"id": "MDk3NTI...", "title": "My Tasks"},
    {"id": "YWJjZGV...", "title": "links"},
    {"id": "eHl6MTI...", "title": "Shopping"}
  ]
}
```

---

## Errors

All errors return JSON:

```json
{"error": "description of what went wrong", "code": 401}
```

| Code | Meaning |
|------|---------|
| 400 | Bad request — missing `title`, or neither `list_id` nor `list_name` provided |
| 401 | Unauthorized — missing/invalid API key, or no active web session for this user |
| 404 | Not found — `list_name` doesn't match any existing list |
| 500 | Server error — Google Tasks API failure |

---

## Examples

### Save current page from a browser extension

```javascript
// Chrome extension background script
async function saveLink(url, title) {
  const response = await fetch("http://localhost:8080/api/v1/tasks", {
    method: "POST",
    headers: {
      "Authorization": "Bearer tsk_<your-key>",
      "Content-Type": "application/json"
    },
    body: JSON.stringify({
      title: url,
      notes: title,
      list_name: "links"
    })
  });
  return response.json();
}
```

### Quick-add from the command line

```bash
# Save to a file for convenience
export TASKIG_KEY="tsk_<your-key>"
export TASKIG_URL="http://localhost:8080"

# Add a link
curl -s -X POST "$TASKIG_URL/api/v1/tasks" \
  -H "Authorization: Bearer $TASKIG_KEY" \
  -H "Content-Type: application/json" \
  -d "{\"title\": \"$1\", \"list_name\": \"links\"}"
```

### Shell alias

```bash
# Add to ~/.bashrc or ~/.zshrc
taskig-add() {
  curl -s -X POST "http://localhost:8080/api/v1/tasks" \
    -H "Authorization: Bearer $TASKIG_KEY" \
    -H "Content-Type: application/json" \
    -d "{\"title\": \"$1\", \"list_name\": \"${2:-links}\", \"notes\": \"${3:-}\"}" | jq .
}

# Usage: taskig-add "https://example.com" "links" "optional notes"
```

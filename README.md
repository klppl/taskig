# Taskig

Self-hosted Google Tasks web client. Single-user, full-screen interface with three-column layout (lists / tasks / details).

**Stack:** Go, HTMX, Tailwind CSS, SQLite, templ

## Features

- Google OAuth2 sign-in with email whitelist
- Task lists, CRUD operations, inline completion toggle
- Due dates with overdue/today color indicators
- "Today" view aggregating due/overdue tasks across all lists
- Hide/show completed tasks toggle
- Dark mode (system preference)

## Setup

1. Create a Google Cloud project with the Tasks API enabled
2. Create OAuth2 credentials (redirect URI: `<BASE_URL>/auth/callback`)
3. Copy `.env.example` to `.env` and fill in values

```
cp .env.example .env
```

## Run

```
docker compose up --build -d
```

The app runs on port 8080 by default. Put it behind a reverse proxy for TLS.

## Environment Variables

| Variable | Description |
|---|---|
| `GOOGLE_CLIENT_ID` | OAuth2 client ID |
| `GOOGLE_CLIENT_SECRET` | OAuth2 client secret |
| `ALLOWED_EMAIL` | Authorized Google email |
| `SESSION_SECRET` | 32+ byte hex string for session encryption |
| `BASE_URL` | Public URL (e.g. `https://tasks.example.com`) |
| `PORT` | Listen port (default `8080`) |
| `DB_PATH` | SQLite database path (default `data/google-tasks.db`) |

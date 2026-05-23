# Smarter OLJ

Smarter OLJ is a Go service that matches OnlineJobsPH job postings to a CV using an AI model, stores match results in a local database, and exposes a small HTTP API plus a web UI to browse and export matches.

The current app is a local analyzer service. By default it creates a SQLite database on the user's machine. Postgres is still available as an advanced option. Jobs are expected to land in the database first, for example from n8n, scripts, or another scraper. The service then runs migrations, starts background matcher workers, calls Ollama for scoring, and serves the UI on port `8080` by default.

This repo is being productized into a downloadable local-first job search tool for OnlineJobsPH jobseekers. See [PRODUCTIZATION_PLAN.md](PRODUCTIZATION_PLAN.md) for the planned SQLite database, CV/profile manager, Go scraper, and pluggable AI providers.

Smarter OLJ is independent software and is not affiliated with or endorsed by OnlineJobsPH.

```mermaid
flowchart LR
  subgraph ingest [Ingest]
    N[n8n / scripts]
  end
  subgraph app [This repo]
    M[Matcher workers]
    O[Ollama API]
    H[HTTP API + web UI]
  end
  DB[(SQLite by default)]
  N --> DB
  M --> DB
  M --> O
  H --> DB
```

## Features

- **CV fit scoring** - For each pending job, the analyzer sends job text and your CV to Ollama and parses a structured result: `fit`, `score`, and `reason`.
- **Today backlog** - The matcher selects rows with `match_score IS NULL` and `posted_at` on the current calendar day, then drains that set each poll.
- **REST API** - Paginated matched jobs and CSV/XLSX export.
- **Web UI** - Static shell under `web/` served at `/`.
- **Migrations** - Embedded SQL under `internal/db/` through golang-migrate.

## Requirements

- Go, using the version declared in `go.mod`
- SQLite is created automatically by default
- Optional: PostgreSQL if `DB_DRIVER=postgres`
- Ollama running and reachable, defaulting to `http://127.0.0.1:11434`
- An Ollama model pulled locally, for example `llama3.2:3b`

## Database Schema

Migrations live in `internal/db/`. SQLite migrations run automatically against the configured local database file. Postgres migrations use the embedded SQL files. The `jobs` table includes listing fields, `posted_at`, match fields (`is_match`, `match_score`, `match_reason`), `notified`, and `analyzed_at`.

## Configuration

Copy `.env.sample` to `.env` and fill in your local values.

Do not commit real `.env` files, CV files, API keys, database dumps, or other private job-search data.

| Variable | Required | Description |
|----------|----------|-------------|
| `DB_DRIVER` | No | `sqlite` by default. Set to `postgres` for advanced Postgres mode |
| `DB_PATH` | No | SQLite path. Use `auto` or leave empty to create the database under the user's app data directory |
| `DATABASE_URL` | Postgres only | Postgres URL for migrations, for example `postgres://user:pass@localhost:5432/jobscraper?sslmode=disable` |
| `DB_HOST`, `DB_PORT`, `DB_USER`, `DB_PASSWORD`, `DB_NAME`, `DB_SSLMODE`, `DB_TIMEZONE` | Postgres only | Individual settings used by the app's GORM connection |
| `PORT` | No | HTTP listen port, default `8080` |
| `AI_PROVIDER` | No | `ollama`, `openai`, `anthropic`, or `openai_compatible`. Default `ollama` |
| `AI_BASE_URL` | No | Provider base URL. Defaults to Ollama/OpenAI/Anthropic defaults when omitted |
| `AI_MODEL` | No | Model name. Default depends on provider |
| `AI_API_KEY` | Cloud providers only | API key loaded into memory from the environment. UI-entered keys are session-only |
| `AI_THINK` | No | Set `true`, `1`, or `yes` only if your Ollama model supports thinking mode |
| `MATCHER_WORKERS` | No | Concurrent analyzer workers, default `2` |
| `MATCHER_BATCH_SIZE` | No | Max rows per DB fetch when draining pending jobs, default `100` |
| `CV_PATH` | No | Optional development fallback path to a plain-text CV. The product flow uses the active CV profile saved through the UI |
| `WEB_ROOT` | No | Optional static UI directory for development. Leave empty to use the embedded UI |

Use the **Active CV profile** panel in the UI to paste your CV or upload a `.txt` file. The saved profile is stored in the local database and used by the matcher. `CV_PATH` remains available as a development fallback; keep real CV files out of git.

Use the **AI provider** panel in the UI to select Ollama, OpenAI, Anthropic, or an OpenAI-compatible server. API keys entered in the UI are kept in memory for the current app session only. They are not stored in SQLite.

Default SQLite database locations:

```text
Windows: %LOCALAPPDATA%\SmarterOLJ\smarter-olj.db
macOS: ~/Library/Application Support/SmarterOLJ/smarter-olj.db
Linux: ~/.local/share/smarter-olj/smarter-olj.db
```

## Run

```bash
go run ./cmd/job-scraper
```

Build:

```bash
go build -o smarter-olj ./cmd/job-scraper
```

## Docker

This repo includes a `Dockerfile` and `docker-compose.yml` for running the service in a container.

### Prerequisites

- Create a local `.env` from `.env.sample`.
- Ensure Ollama is running.

Important: when the app runs inside Docker, `127.0.0.1` refers to the container itself. If your Ollama instance is running on your host machine, set:

```bash
AI_BASE_URL=http://host.docker.internal:11434
```

### Run With Docker Compose

```bash
docker compose up --build
```

Then open:

- UI: `http://localhost:8080/`
- Health: `http://localhost:8080/api/health`

## HTTP API

| Method | Path | Description |
|--------|------|-------------|
| `GET` | `/api/health` | Liveness: `{"status":"ok"}` |
| `GET` | `/api/ai/settings` | Read current in-memory AI provider settings without exposing API keys |
| `POST` | `/api/ai/settings` | Update in-memory AI provider settings for the current app session |
| `GET` | `/api/profile/active` | Read the active CV profile |
| `POST` | `/api/profile/active` | Save the active CV profile with JSON body: `{"name":"Main","cv_text":"..."}` |
| `GET` | `/api/jobs/matched` | Paginated jobs with `is_match = true`. Query: `notified` (bool), `limit` (1-100, default 20), `offset` |
| `GET` | `/api/jobs/matched/export?format=csv` or `format=xlsx` | Download up to 10k matched rows |

Open `http://localhost:8080/` for the UI. The binary serves the embedded UI by default. Set `WEB_ROOT=web` during development if you want to serve `web/index.html` from disk.

## Project Layout

```text
cmd/job-scraper/       # main: migrations, matcher goroutine, HTTP server
internal/api/          # matched jobs handlers
internal/db/           # connection, migrations, queries
internal/export/       # CSV / XLSX
internal/matcher/      # AI analyzer and polling workers
internal/models/       # data models
internal/scraper/      # OnlineJobsPH scraper foundation
internal/server/       # Gin router
web/                   # static UI embedded into the binary
```

## Matcher Behavior

- Poll interval defaults to 5 minutes between full drain cycles.
- Each cycle repeatedly fetches up to `MATCHER_BATCH_SIZE` pending today jobs until none remain, or until a page yields no successful updates.
- Jobs are candidates while `match_score` is still `NULL`.
- Successful analysis sets `is_match`, `match_score`, `match_reason`, and `analyzed_at`.

## Ingestion

This repository currently focuses on analysis and serving. Feeding `jobs`, respecting `external_id` uniqueness and the migration column set, can be done with n8n, scripts, or another scraper.

The productized version is moving OnlineJobsPH scraping into Go and avoiding storage of every scraped job before filtering. The scraper and pipeline foundations can parse OnlineJobsPH listing/detail HTML, apply local filters, store compact skipped records in `seen_jobs`, save full descriptions only for queued candidates, and let the matcher analyze queued jobs.

## License

This project is licensed under the MIT License. See [LICENSE](LICENSE).

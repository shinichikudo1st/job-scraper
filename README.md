# Smarter OLJ

Smarter OLJ is a Go service that matches OnlineJobsPH job postings to a CV using an AI model, stores match results in PostgreSQL, and exposes a small HTTP API plus a web UI to browse and export matches.

The current app is a local analyzer service. Jobs are expected to land in Postgres first, for example from n8n, scripts, or another scraper. The service then runs migrations, starts background matcher workers, calls Ollama for scoring, and serves the UI on port `8080` by default.

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
  DB[(PostgreSQL)]
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
- PostgreSQL with a `jobs` table compatible with the migrations
- Ollama running and reachable, defaulting to `http://127.0.0.1:11434`
- An Ollama model pulled locally, for example `llama3.2:3b`

## Database Schema

Migrations live in `internal/db/`. The `jobs` table includes listing fields, `posted_at`, match fields (`is_match`, `match_score`, `match_reason`), `notified`, and `analyzed_at`.

## Configuration

Copy `.env.sample` to `.env` and fill in your local values.

Do not commit real `.env` files, CV files, API keys, database dumps, or other private job-search data.

| Variable | Required | Description |
|----------|----------|-------------|
| `DATABASE_URL` | Yes | Postgres URL for migrations, for example `postgres://user:pass@localhost:5432/jobscraper?sslmode=disable` |
| `DB_HOST`, `DB_PORT`, `DB_USER`, `DB_PASSWORD`, `DB_NAME`, `DB_SSLMODE`, `DB_TIMEZONE` | Yes | Individual settings used by the app's GORM connection |
| `PORT` | No | HTTP listen port, default `8080` |
| `OLLAMA_BASE_URL` | No | Default `http://127.0.0.1:11434` |
| `OLLAMA_MODEL` | No | Default `llama3.2:3b` |
| `OLLAMA_THINK` | No | Set `true`, `1`, or `yes` only if your model supports Ollama thinking mode |
| `MATCHER_WORKERS` | No | Concurrent analyzer workers, default `2` |
| `MATCHER_BATCH_SIZE` | No | Max rows per DB fetch when draining pending jobs, default `100` |
| `CV_PATH` | No | Path to plain-text CV, default `cv.text` |
| `WEB_ROOT` | No | Static UI directory, default `web` |

Use `cv.example.text` as a template. Keep your real CV in `cv.text` or another path via `CV_PATH`, and keep real CV files out of git.

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
OLLAMA_BASE_URL=http://host.docker.internal:11434
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
| `GET` | `/api/jobs/matched` | Paginated jobs with `is_match = true`. Query: `notified` (bool), `limit` (1-100, default 20), `offset` |
| `GET` | `/api/jobs/matched/export?format=csv` or `format=xlsx` | Download up to 10k matched rows |

Open `http://localhost:8080/` for the UI when `WEB_ROOT` is set and `web/index.html` exists.

## Project Layout

```text
cmd/job-scraper/       # main: migrations, matcher goroutine, HTTP server
internal/api/          # matched jobs handlers
internal/db/           # connection, migrations, queries
internal/export/       # CSV / XLSX
internal/matcher/      # Ollama client, analyzer, polling workers
internal/models/       # Job model
internal/server/       # Gin router
web/                   # static UI
```

## Matcher Behavior

- Poll interval defaults to 5 minutes between full drain cycles.
- Each cycle repeatedly fetches up to `MATCHER_BATCH_SIZE` pending today jobs until none remain, or until a page yields no successful updates.
- Jobs are candidates while `match_score` is still `NULL`.
- Successful analysis sets `is_match`, `match_score`, `match_reason`, and `analyzed_at`.

## Ingestion

This repository currently focuses on analysis and serving. Feeding `jobs`, respecting `external_id` uniqueness and the migration column set, can be done with n8n, scripts, or another scraper.

The productized version will move OnlineJobsPH scraping into Go and avoid storing every scraped job before filtering.

## License

This project is licensed under the MIT License. See [LICENSE](LICENSE).

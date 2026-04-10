# job-scraper

Go service that **matches job postings to your CV** using a local **[Ollama](https://ollama.com/)** model, stores results in **PostgreSQL**, and exposes a small **HTTP API** plus a **web UI** to browse and export matches.

Typical flow: jobs land in Postgres (for example via **n8n** scraping **OnlineJobsPH** or any other ingest). This binary runs **migrations**, a **background matcher** (worker pool + Ollama), and an **HTTP server** on port `8080` by default.

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

- **CV fit scoring** — For each pending job, the analyzer sends job text + your CV to Ollama and parses a structured result (`fit`, `score`, `reason`).
- **“Today” backlog** — The matcher selects rows with `match_score IS NULL` and `posted_at` on the **current calendar day** (server local timezone), then **drains** that set each poll (paged fetches; default page size 100).
- **REST API** — Paginated matched jobs and CSV/XLSX export.
- **Web UI** — Static shell under `web/` served at `/`.
- **Migrations** — Embedded SQL under `internal/db/` (golang-migrate).

## Requirements

- **Go** (see `go.mod` for the toolchain version)
- **PostgreSQL** with a `jobs` table compatible with the migrations
- **Ollama** running and reachable (default `http://127.0.0.1:11434`), with your chosen model pulled (e.g. `llama3.2:3b`)

## Database schema (summary)

Migrations live in `internal/db/`. The `jobs` table includes listing fields, `posted_at`, match fields (`is_match`, `match_score`, `match_reason`), `notified`, and `analyzed_at` (time of last successful analysis write).

## Configuration

Copy `.env.sample` to `.env` and fill in values. **Do not commit `.env`.**

| Variable | Required | Description |
|----------|----------|-------------|
| `DATABASE_URL` | Yes | Postgres URL for **migrations** (e.g. `postgres://user:pass@host:5432/dbname?sslmode=disable`) |
| `DB_HOST`, `DB_PORT`, `DB_USER`, `DB_PASSWORD`, `DB_NAME`, `DB_SSLMODE`, `DB_TIMEZONE` | Yes | Individual settings used by the app’s **GORM** connection |
| `PORT` | No | HTTP listen port (default `8080`) |
| `OLLAMA_BASE_URL` | No | Default `http://127.0.0.1:11434` |
| `OLLAMA_MODEL` | No | Default `llama3.2:3b` |
| `OLLAMA_THINK` | No | Set `true` / `1` / `yes` only if your model supports Ollama’s thinking mode |
| `MATCHER_WORKERS` | No | Concurrent analyzer workers (default `2`) |
| `MATCHER_BATCH_SIZE` | No | Max rows per DB fetch when draining pending jobs (default `100`) |
| `CV_PATH` | No | Path to plain-text CV (default `cv.text`) |
| `WEB_ROOT` | No | Static UI directory (default `web`) |

Use `cv.example.text` as a template; keep your real CV in `cv.text` (or another path via `CV_PATH`) and add `cv.text` to `.gitignore` if you use that filename locally.

## Run

```bash
go run ./cmd/job-scraper
```

Build:

```bash
go build -o job-scraper ./cmd/job-scraper
```

## HTTP API

| Method | Path | Description |
|--------|------|-------------|
| `GET` | `/api/health` | Liveness (`{"status":"ok"}`) |
| `GET` | `/api/jobs/matched` | Paginated jobs with `is_match = true`. Query: `notified` (bool), `limit` (1–100, default 20), `offset` |
| `GET` | `/api/jobs/matched/export?format=csv` or `format=xlsx` | Download up to 10k matched rows |

Open `http://localhost:8080/` for the UI (when `WEB_ROOT` is set and `web/index.html` exists).

## Project layout

```
cmd/job-scraper/       # main: migrations, matcher goroutine, HTTP server
internal/api/          # matched jobs handlers
internal/db/           # connection, migrations, queries
internal/export/       # CSV / XLSX
internal/matcher/      # Ollama client, analyzer, polling workers
internal/models/       # Job model
internal/server/       # Gin router
web/                   # static UI
```

## Matcher behavior (details)

- Poll interval defaults to **5 minutes** between full drain cycles.
- Each cycle repeatedly fetches up to `MATCHER_BATCH_SIZE` pending “today” jobs until none remain, or until a page yields **no successful updates** (avoids tight loops when every row fails, e.g. Ollama down).
- Jobs are only candidates while `match_score` is still `NULL`; successful analysis sets `is_match`, `match_score`, `match_reason`, and `analyzed_at`.

## Ingestion

This repository focuses on **analysis and serving**. Feeding `jobs` (respecting `external_id` uniqueness and the column set in migrations) can be done with n8n, scripts, or another scraper—whatever fits your setup.

## License

No license file is included in this repository; add one if you intend to specify terms for reuse.

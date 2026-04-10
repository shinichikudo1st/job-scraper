# Job Matcher/Analyzer - Implementation Phases

## Overview
Go backend service running on **local machine** that:
1. Fetches unanalyzed jobs from PostgreSQL (VM)
2. Analyzes each with Ollama AI (local)
3. Updates match results in DB
4. Serves matched jobs via web UI + CSV/Excel export

---

## Phase 1: Core Matcher Service (Backend)

**Status: complete.**

### 1.1 - Database Layer
- [x] Add `analyzed_at TIMESTAMPTZ` column to `jobs` table (migration `000002_add_analyzed_at`)
- [x] Create `internal/db/queries.go` with:
  - [x] `FetchPendingJobs(conn, limit, postedAfter)` — jobs WHERE `match_score IS NULL` and `posted_at >= postedAfter`
  - [x] `UpdateJobMatch(...)` — sets `is_match`, `match_score`, `match_reason`, `analyzed_at`
  - [x] `GetMatchedJobs(...)` — `is_match=true` (for Phase 2 API)
- [x] `internal/models/job.go` — `Job` struct including `analyzed_at`

### 1.2 - Ollama Client
- [x] Create `internal/matcher/ollama_client.go`:
  - [x] `type OllamaClient struct` with `BaseURL`, `Model`, configurable HTTP client, retries, `num_predict`
  - [x] `Generate(prompt string)` — POST `/api/generate` with `num_predict` cap
  - [x] Timeouts and retries on 5xx

### 1.3 - Job Analyzer Core
- [x] Create `internal/matcher/analyzer.go`:
  - [x] Load CV from file (`NewAnalyzer` / `LoadCVText`; path via `CV_PATH`, default `cv.text`)
  - [x] `AnalyzeJob(job *models.Job, cv string) (*MatchResult, error)` — prompt + Ollama + JSON parse
  - [x] `type MatchResult struct { Fit, Score, Reason }` (+ JSON extraction when model wraps text)

### 1.4 - Worker Pool
- [x] Create `internal/matcher/worker.go`:
  - [x] `RunMatcher(ctx, conn *gorm.DB, analyzer *Analyzer, workers int)` — poll loop with **5m** default interval
  - [x] Fetch pending jobs (posted **today**, default batch **10**), worker pool via channels
  - [x] Per job: analyze → `UpdateJobMatch` (sets `analyzed_at` via DB layer)
  - [x] Log errors; continue on per-job failures
  - [x] Graceful shutdown on `ctx.Done()`

### 1.5 - Integration into `main.go`
- [x] `.env`: `OLLAMA_BASE_URL`, `OLLAMA_MODEL`, `MATCHER_WORKERS`, `CV_PATH`
- [x] After migrations: `ConnectDB()`, `NewOllamaClient`, `NewAnalyzer`, `go matcher.RunMatcher(ctx, dbConn, analyzer, workers)`
- [x] Signal-based `ctx` (`SIGINT` / `SIGTERM`) stops matcher and shuts down HTTP server gracefully

---

## Phase 2: REST API for Matched Jobs

**Status: 2.1–2.2 complete** (handlers in `internal/api/matched_jobs.go`; tests in `internal/api/matched_jobs_test.go`).

### 2.1 - API Endpoints
- [x] `GET /api/jobs/matched` — paginated matched jobs (`is_match = true`):
  - Query: `notified` (default `false`), `limit` (default `20`, max `100`), `offset` (default `0`)
  - Response JSON: `{ "items": [...], "total", "limit", "offset" }` with per-job: `id`, `title`, `company`, `salary`, `url`, `match_score`, `match_reason`, `posted_at`
- [x] `GET /api/jobs/matched/export?format=csv|xlsx` — download (optional `notified`; export capped at 10k rows)

### 2.2 - Export Logic
- [x] `internal/export/csv.go` — `ExportCSV(jobs []models.Job) ([]byte, error)`
- [x] `internal/export/excel.go` — `ExportExcel` via `github.com/xuri/excelize/v2`
- [x] `internal/db/queries.go` — `GetMatchedJobsPaginated` for list + export

---

## Phase 3: Simple Frontend UI

**Status: complete** — `web/index.html`, `WEB_ROOT` env, `internal/server.NewRouter`.

### 3.1 - Static HTML/JS Page
- [x] `web/index.html`:
  - [x] Table from `/api/jobs/matched` (limit 100)
  - [x] Columns: Title, Salary, Score, Reason, Link (opens in new tab)
  - [x] "Show notified jobs only" checkbox + posted date range (client-side filter)
  - [x] Download CSV / Excel (same `notified` flag as list)

### 3.2 - Serve Static Files
- [x] `internal/server/router.go` — `GET /` serves `web/index.html` (avoids Gin `Static("/")` vs `/api` conflict)
- [x] `cmd/job-scraper/main.go` — `WEB_ROOT` default `web`; UI at `http://localhost:8080/`

### 3.3 - Basic Styling
- [x] Tailwind CDN + dark layout, `overflow-x-auto` table (mobile-friendly)

---

## Phase 4: Enhancements (Optional)

### 4.1 - Scheduler Improvements
- [ ] Add `--matcher-only` CLI flag to run matcher without HTTP server
- [ ] Configurable schedule (cron expression or interval from env)

### 4.2 - Discord Integration (if not in n8n)
- [ ] Move Discord notification logic to Go:
  - [ ] `internal/notifier/discord.go` - batch send when `COUNT(is_match AND NOT notified) >= 5`
  - [ ] Mark `notified=true` after successful send

### 4.3 - Monitoring
- [ ] Add metrics endpoint: `/api/metrics` - pending count, analyzed today, match rate
- [ ] Structured logging (JSON logs)

### 4.4 - Advanced UI Features
- [ ] Mark job as "applied" or "dismissed" (add column + button)
- [ ] Full-text search in matched jobs
- [ ] Real-time updates (WebSocket or SSE)

---

## Dependencies

Add to `go.mod`:
```bash
go get github.com/xuri/excelize/v2  # Excel export
# (gin, godotenv, pgx already present)
```

---

## Testing Checklist

- [x] Unit tests: `AnalyzeJob` + JSON edge cases (`internal/matcher/analyzer_test.go`)
- [x] Unit tests: Ollama client + worker batch with mock repo (`ollama_client_test.go`, `worker_test.go`)
- [ ] Integration test: matcher against real Postgres + Ollama (optional)
- [ ] Manual: verify CSV/Excel downloads open correctly in Excel/Google Sheets
- [x] HTTP tests: matched list + CSV/XLSX export + invalid format (`internal/api/matched_jobs_test.go`)
- [x] Web checks: `web/index_test.go` (required strings in `index.html`)
- [x] Router checks: `internal/server/router_test.go` (health + `/` serves UI)
- [ ] End-to-end: n8n scrapes → Go analyzes → UI shows matches → export works (Phase 3+)

---

## Success Criteria

- **Done (Phase 1):** Matcher runs locally, polls DB on a 5-minute interval by default; Ollama URL is configurable (`OLLAMA_BASE_URL`, typically `http://127.0.0.1:11434`).
- **Done (Phase 2):** Matched-jobs JSON API, CSV/XLSX export endpoints (`/api/jobs/matched`, `/api/jobs/matched/export`).
- **Done (Phase 3):** Simple web UI at `http://localhost:8080/` (with `WEB_ROOT=web` from repo root).
- **Pending (overall):** Optional polish (embed static files, matcher-only mode, Discord, etc.).

# job-scraper

OnlineJobsPH (scraper) Go Backend → Go backend clean data
                                         ↓
                                   n8n Workflow
                                         ↓
                                CV Match check (AI Model)
                                         ↓
                                Store matched jobs
                                         ↓
                            Every 5 new matches → Discord

                    


Stack
Layer                           Technology

Backend / Scraper                   Go
Database                            PostgreSQL
Orchestration                       n8n
AI Matching                         Ollama local model (via n8n HTTP node or Go)
Notifications                       Discord Webhook



CURRENT SETUP:

Both N8N and Postgresql are run via docker in an ubuntu server virtual machine and Go Backend is running in the local machine


job-scanner/
├── CLAUDE.md                  ← this file
├── .env                       ← secrets (never commit)
├── cmd/
│   └── scraper/
│       └── main.go            ← entry point for scraper binary
├── internal/
│   ├── scraper/
│   │   └── onlinejobsph.go    ← scraping logic
│   ├── db/
│   │   └── postgres.go        ← DB connection, queries
│   ├── matcher/
│   │   └── AI-analyzer.go          ← CV match logic via Claude API
│   └── models/
│       └── job.go             ← Job struct
├── migrations/
│   └── 001_init.sql           ← DB schema
├── cv/
│   └── cv.txt                 ← Plain text version of your CV
|   
└── n8n/
    └── workflow.json          ← Exported n8n workflow (for backup/version control)




-- migrations/001_init.sql

CREATE TABLE jobs (
    id              SERIAL PRIMARY KEY,
    external_id     TEXT UNIQUE NOT NULL,       -- unique ID or URL hash from OnlineJobsPH
    title           TEXT NOT NULL,
    company         TEXT,
    location        TEXT,
    salary          TEXT,
    description     TEXT,
    url             TEXT NOT NULL,
    posted_at       TIMESTAMPTZ,
    scraped_at      TIMESTAMPTZ DEFAULT NOW(),
    is_match        BOOLEAN DEFAULT FALSE,
    match_score     INT,                        -- 0–100 from Claude
    match_reason    TEXT,                       -- Claude's explanation
    notified        BOOLEAN DEFAULT FALSE       -- whether this job was part of a Discord batch
);

CREATE INDEX idx_jobs_is_match ON jobs(is_match);
CREATE INDEX idx_jobs_notified ON jobs(notified);


#DATABASE
DB_HOST=172.31.186.103
DB_PORT=5432
DB_USER=shinichikudo1st
DB_PASSWORD=myjobscraperengine
DB_NAME=jobscraper
DB_SSLMODE=disable
DB_TIMEZONE=Asia/Tokyo

DATABASE_URL=postgres://shinichikudo1st:myjobscraperengine@172.31.186.103:5432/jobscraper?sslmode=disable&timezone=Asia/Tokyo


n8n Workflow Design

Use n8n to orchestrate the full pipeline

Workflow: Job Scanner Pipeline

[Schedule Trigger]
    ↓ every 30/60 min
[HTTP Request] → GET OnlineJobsPH search URL
    ↓
[HTML Extract / Code Node] → parse job listings
    ↓
[Loop Over Items]
    ↓
[Postgres Node] → INSERT ... ON CONFLICT DO NOTHING
    ↓ (if new row)
[HTTP Request] → POST AI with job + CV
    ↓
[Code Node] → parse AI JSON response
    ↓
[Postgres Node] → UPDATE jobs SET is_match, match_score, match_reason
    ↓
[Postgres Node] → SELECT COUNT(*) WHERE is_match=true AND notified=false
    ↓
[IF Node] → count >= 5?
    ↓ YES
[Postgres Node] → SELECT unnotified matched jobs
    ↓
[HTTP Request] → POST Discord Webhook
    ↓
[Postgres Node] → UPDATE jobs SET notified=true WHERE id IN (...)
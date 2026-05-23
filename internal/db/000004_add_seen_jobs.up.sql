CREATE TABLE IF NOT EXISTS seen_jobs (
    external_id   TEXT PRIMARY KEY,
    url           TEXT NOT NULL,
    title         TEXT NOT NULL,
    status        TEXT NOT NULL DEFAULT 'seen',
    first_seen_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    last_seen_at  TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE INDEX IF NOT EXISTS idx_seen_jobs_status ON seen_jobs(status);
CREATE INDEX IF NOT EXISTS idx_seen_jobs_last_seen_at ON seen_jobs(last_seen_at);

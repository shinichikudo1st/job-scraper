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
DROP INDEX IF EXISTS idx_jobs_status;

ALTER TABLE jobs
DROP COLUMN IF EXISTS analysis_last_error;

ALTER TABLE jobs
DROP COLUMN IF EXISTS analysis_retry_count;

ALTER TABLE jobs
DROP COLUMN IF EXISTS status;

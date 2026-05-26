CREATE TABLE IF NOT EXISTS scrape_runs (
    id           BIGSERIAL  PRIMARY KEY,
    source       TEXT       NOT NULL,
    keywords     TEXT       NOT NULL,
    time_posted  TEXT       NOT NULL,
    work_type    TEXT       NOT NULL,
    job_type     TEXT,
    started_at   TIMESTAMP  NOT NULL,
    finished_at  TIMESTAMP  NOT NULL,
    jobs_found   INT        NOT NULL DEFAULT 0,
    jobs_new     INT        NOT NULL DEFAULT 0,
    jobs_skipped INT        NOT NULL DEFAULT 0
);

COMMENT ON TABLE  scrape_runs              IS 'Log of each scrape run and its results';
COMMENT ON COLUMN scrape_runs.id           IS 'Internal auto-generated ID';
COMMENT ON COLUMN scrape_runs.source       IS 'Job board scraped e.g. linkedin, indeed';
COMMENT ON COLUMN scrape_runs.keywords     IS 'Search keywords used';
COMMENT ON COLUMN scrape_runs.time_posted  IS 'Time period filter used e.g. r86400';
COMMENT ON COLUMN scrape_runs.work_type    IS 'Work type filter used e.g. remote, hybrid, onsite';
COMMENT ON COLUMN scrape_runs.job_type     IS 'Job type filter if applied e.g. F=full-time, C=contract';
COMMENT ON COLUMN scrape_runs.started_at   IS 'Timestamp when the scrape started';
COMMENT ON COLUMN scrape_runs.finished_at  IS 'Timestamp when the scrape finished';
COMMENT ON COLUMN scrape_runs.jobs_found   IS 'Total jobs returned across all search pages';
COMMENT ON COLUMN scrape_runs.jobs_new     IS 'Jobs newly inserted into the database this run';
COMMENT ON COLUMN scrape_runs.jobs_skipped IS 'Jobs skipped because they already existed in the database';

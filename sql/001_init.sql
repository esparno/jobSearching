CREATE TABLE IF NOT EXISTS jobs (
    id            BIGSERIAL PRIMARY KEY,
    source        TEXT      NOT NULL,
    source_id     TEXT      NOT NULL,
    title         TEXT,
    company       TEXT,
    location      TEXT,
    url           TEXT,
    posted_date   DATE,
    first_seen    TIMESTAMP NOT NULL DEFAULT NOW(),
    last_seen     TIMESTAMP NOT NULL DEFAULT NOW(),
    UNIQUE (source, source_id)
);

COMMENT ON COLUMN jobs.id            IS 'Internal auto-generated ID';
COMMENT ON COLUMN jobs.source        IS 'Job board the listing was pulled from e.g. linkedin, indeed';
COMMENT ON COLUMN jobs.source_id     IS 'Native ID assigned by the job board';
COMMENT ON COLUMN jobs.title         IS 'Job title as listed on the board';
COMMENT ON COLUMN jobs.company       IS 'Company advertising the role';
COMMENT ON COLUMN jobs.location      IS 'Location as listed on the board';
COMMENT ON COLUMN jobs.url           IS 'Direct URL to the job listing';
COMMENT ON COLUMN jobs.posted_date   IS 'Date the job was posted according to the board';
COMMENT ON COLUMN jobs.first_seen    IS 'Timestamp when this job was first scraped';
COMMENT ON COLUMN jobs.last_seen     IS 'Timestamp of the most recent scrape run that returned this job';

CREATE TABLE IF NOT EXISTS job_details (
    id              BIGSERIAL PRIMARY KEY,
    job_id          BIGINT    NOT NULL REFERENCES jobs(id),
    source          TEXT      NOT NULL,
    source_id       TEXT      NOT NULL,
    seniority       TEXT,
    employment_type TEXT,
    work_type       TEXT,
    job_function    TEXT,
    industries      TEXT,
    description     TEXT,
    applicants      TEXT,
    salary_min      NUMERIC,
    salary_max      NUMERIC,
    salary_text     TEXT,
    description_tsv TSVECTOR GENERATED ALWAYS AS (to_tsvector('english', COALESCE(description, ''))) STORED,
    fetched_at      TIMESTAMP NOT NULL DEFAULT NOW(),
    UNIQUE (job_id),
    UNIQUE (source, source_id)
);

COMMENT ON COLUMN job_details.id              IS 'Internal auto-generated ID';
COMMENT ON COLUMN job_details.job_id          IS 'Foreign key reference to jobs.id';
COMMENT ON COLUMN job_details.source          IS 'Job board the listing was pulled from e.g. linkedin, indeed';
COMMENT ON COLUMN job_details.source_id       IS 'Native ID assigned by the job board';
COMMENT ON COLUMN job_details.seniority       IS 'Seniority level e.g. entry, mid-senior, director';
COMMENT ON COLUMN job_details.employment_type IS 'Employment type e.g. full-time, part-time, contract';
COMMENT ON COLUMN job_details.work_type       IS 'Work arrangement e.g. remote, hybrid, onsite';
COMMENT ON COLUMN job_details.job_function    IS 'Job function e.g. engineering, marketing';
COMMENT ON COLUMN job_details.industries      IS 'Industry the company operates in';
COMMENT ON COLUMN job_details.description     IS 'Full job description text';
COMMENT ON COLUMN job_details.applicants      IS 'Applicant count as reported by the board e.g. 37 applicants';
COMMENT ON COLUMN job_details.salary_min      IS 'Lower bound of the salary range if provided';
COMMENT ON COLUMN job_details.salary_max      IS 'Upper bound of the salary range if provided';
COMMENT ON COLUMN job_details.salary_text     IS 'Raw salary text as scraped, for reference and future parsing';
COMMENT ON COLUMN job_details.description_tsv  IS 'Auto-generated tsvector of description for full text search';
COMMENT ON CONSTRAINT job_details_job_id_key ON job_details IS 'Each job can only have one detail record';
COMMENT ON COLUMN job_details.fetched_at       IS 'Timestamp when the job details were fetched';

CREATE INDEX idx_job_details_description_tsv ON job_details USING GIN(description_tsv);

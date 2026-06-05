CREATE TABLE IF NOT EXISTS request_logs (
    id              BIGSERIAL  PRIMARY KEY,
    source          TEXT       NOT NULL,
    job_source_id   TEXT,
    url             TEXT       NOT NULL,
    request_headers TEXT       NOT NULL,
    status_code     INT,
    error           TEXT,
    message         TEXT       NOT NULL,
    response_body   TEXT,
    is_issue        BOOLEAN    NOT NULL DEFAULT FALSE,
    run_id          TEXT,
    logged_at       TIMESTAMP  NOT NULL DEFAULT NOW()
);

COMMENT ON TABLE  request_logs                  IS 'Log of HTTP requests made during scraping, with full response body captured on failures';
COMMENT ON COLUMN request_logs.id               IS 'Internal auto-generated ID';
COMMENT ON COLUMN request_logs.source           IS 'Job board the request was made to e.g. linkedin, indeed';
COMMENT ON COLUMN request_logs.job_source_id    IS 'Native job ID the request was fetching details for, if applicable';
COMMENT ON COLUMN request_logs.url              IS 'Full request URL including query params';
COMMENT ON COLUMN request_logs.request_headers  IS 'JSON-encoded request headers sent';
COMMENT ON COLUMN request_logs.status_code      IS 'HTTP response status code, null if a network error occurred';
COMMENT ON COLUMN request_logs.error            IS 'Error message if the request or body read failed';
COMMENT ON COLUMN request_logs.message          IS 'Human-readable description of the outcome';
COMMENT ON COLUMN request_logs.response_body    IS 'Full response body, only captured when is_issue is true';
COMMENT ON COLUMN request_logs.is_issue         IS 'True when the request resulted in an error, non-200 status, or empty parse result';
COMMENT ON COLUMN request_logs.run_id           IS 'Scrape run this request belongs to';

CREATE INDEX idx_request_logs_is_issue ON request_logs (is_issue);
CREATE INDEX idx_request_logs_source   ON request_logs (source);
CREATE INDEX idx_request_logs_run_id   ON request_logs (run_id);
ALTER TABLE request_logs ADD COLUMN IF NOT EXISTS run_id TEXT;

COMMENT ON COLUMN request_logs.run_id IS 'Scrape run this request belongs to; null for logs written before this migration';

CREATE INDEX idx_request_logs_run_id ON request_logs (run_id);
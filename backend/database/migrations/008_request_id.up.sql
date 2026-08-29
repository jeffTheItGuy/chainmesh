ALTER TABLE request_logs
ADD COLUMN IF NOT EXISTS request_id TEXT;

CREATE INDEX IF NOT EXISTS idx_request_logs_request_id
ON request_logs(request_id);
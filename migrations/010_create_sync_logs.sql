-- +goose Up
CREATE TABLE sync_logs (
    id           UUID PRIMARY KEY DEFAULT uuid_generate_v4(),
    clinic_id    UUID NOT NULL,
    provider     VARCHAR(100) NOT NULL,
    operation    VARCHAR(100) NOT NULL,   -- "sync_appointments", "fetch_lab_result"
    status       VARCHAR(50) NOT NULL,    -- "success", "failed", "partial"
    records_processed INT DEFAULT 0,
    error_message TEXT,
    started_at   TIMESTAMPTZ NOT NULL,
    completed_at TIMESTAMPTZ,
    created_at   TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE INDEX idx_sync_logs_clinic_id ON sync_logs(clinic_id);
CREATE INDEX idx_sync_logs_provider ON sync_logs(provider);
CREATE INDEX idx_sync_logs_created_at ON sync_logs(created_at);

-- +goose Down
DROP TABLE sync_logs;

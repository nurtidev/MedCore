-- +goose Up
CREATE TABLE lab_results (
    id           UUID PRIMARY KEY DEFAULT uuid_generate_v4(),
    clinic_id    UUID NOT NULL,
    patient_id   UUID NOT NULL,
    external_id  VARCHAR(255),
    lab_provider VARCHAR(100) NOT NULL,
    test_name    VARCHAR(255) NOT NULL,
    format       VARCHAR(20) NOT NULL,
    file_url     VARCHAR(500),
    data         JSONB,
    received_at  TIMESTAMPTZ NOT NULL,
    attached_at  TIMESTAMPTZ,
    created_at   TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE INDEX idx_lab_results_patient_id ON lab_results(patient_id);
CREATE INDEX idx_lab_results_clinic_id ON lab_results(clinic_id);
CREATE INDEX idx_lab_results_received_at ON lab_results(received_at);

-- +goose Down
DROP TABLE lab_results;

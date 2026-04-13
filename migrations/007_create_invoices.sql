-- +goose Up
CREATE TABLE invoices (
    id           UUID         PRIMARY KEY DEFAULT uuid_generate_v4(),
    clinic_id    UUID         NOT NULL,
    patient_id   UUID,
    service_name VARCHAR(255) NOT NULL,
    amount       NUMERIC(10,2) NOT NULL,
    currency     VARCHAR(3)   NOT NULL DEFAULT 'KZT',
    status       VARCHAR(50)  NOT NULL DEFAULT 'draft',
    due_at       TIMESTAMPTZ,
    paid_at      TIMESTAMPTZ,
    pdf_url      VARCHAR(500),
    created_at   TIMESTAMPTZ  NOT NULL DEFAULT NOW(),
    updated_at   TIMESTAMPTZ  NOT NULL DEFAULT NOW()
);

CREATE INDEX idx_invoices_clinic_id  ON invoices(clinic_id);
CREATE INDEX idx_invoices_status     ON invoices(status);
CREATE INDEX idx_invoices_patient_id ON invoices(patient_id);
CREATE INDEX idx_invoices_due_at     ON invoices(due_at) WHERE status NOT IN ('paid', 'voided');

-- +goose Down
DROP TABLE invoices;

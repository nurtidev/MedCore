-- +goose Up
CREATE TABLE payments (
    id              UUID         PRIMARY KEY DEFAULT uuid_generate_v4(),
    invoice_id      UUID         NOT NULL REFERENCES invoices(id),
    clinic_id       UUID         NOT NULL,
    patient_id      UUID,
    idempotency_key VARCHAR(255) UNIQUE NOT NULL,
    provider        VARCHAR(50)  NOT NULL,
    external_id     VARCHAR(255),
    amount          NUMERIC(10,2) NOT NULL,
    currency        VARCHAR(3)   NOT NULL DEFAULT 'KZT',
    status          VARCHAR(50)  NOT NULL DEFAULT 'pending',
    failure_reason  TEXT,
    metadata        JSONB,
    created_at      TIMESTAMPTZ  NOT NULL DEFAULT NOW(),
    updated_at      TIMESTAMPTZ  NOT NULL DEFAULT NOW()
);

CREATE UNIQUE INDEX idx_payments_idempotency ON payments(idempotency_key);
CREATE INDEX idx_payments_invoice_id        ON payments(invoice_id);
CREATE INDEX idx_payments_clinic_id         ON payments(clinic_id);
CREATE INDEX idx_payments_status            ON payments(status);
CREATE INDEX idx_payments_external_id       ON payments(external_id) WHERE external_id IS NOT NULL;

-- +goose Down
DROP TABLE payments;

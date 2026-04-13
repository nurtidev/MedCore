-- +goose Up
CREATE TABLE subscriptions (
    id                   UUID        PRIMARY KEY DEFAULT uuid_generate_v4(),
    clinic_id            UUID        NOT NULL,
    plan_id              UUID        NOT NULL REFERENCES subscription_plans(id),
    status               VARCHAR(50) NOT NULL DEFAULT 'active',
    current_period_start TIMESTAMPTZ NOT NULL,
    current_period_end   TIMESTAMPTZ NOT NULL,
    cancelled_at         TIMESTAMPTZ,
    created_at           TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at           TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE INDEX idx_subscriptions_clinic_id  ON subscriptions(clinic_id);
CREATE INDEX idx_subscriptions_status     ON subscriptions(status);
CREATE INDEX idx_subscriptions_period_end ON subscriptions(current_period_end);

-- +goose Down
DROP TABLE subscriptions;

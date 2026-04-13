-- +goose Up
CREATE EXTENSION IF NOT EXISTS "uuid-ossp";

CREATE TABLE subscription_plans (
    id            UUID PRIMARY KEY DEFAULT uuid_generate_v4(),
    tier          VARCHAR(50)   NOT NULL,
    name          VARCHAR(100)  NOT NULL,
    price_monthly NUMERIC(10,2) NOT NULL,
    currency      VARCHAR(3)    NOT NULL DEFAULT 'KZT',
    max_doctors   INT           NOT NULL DEFAULT 5,
    max_patients  INT           NOT NULL DEFAULT 500,
    features      JSONB,
    is_active     BOOLEAN       NOT NULL DEFAULT true,
    created_at    TIMESTAMPTZ   NOT NULL DEFAULT NOW()
);

-- Seed data
INSERT INTO subscription_plans (tier, name, price_monthly, max_doctors, max_patients, features) VALUES
('basic',      'Basic',      49900,  3,   300,  '["online_payments","basic_analytics"]'),
('pro',        'Pro',        99900,  10,  1000, '["online_payments","advanced_analytics","lab_integrations"]'),
('enterprise', 'Enterprise', 199900, 999, 9999, '["all_features","dedicated_support","custom_integrations"]');

-- +goose Down
DROP TABLE subscription_plans;

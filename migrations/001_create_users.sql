-- +goose Up
CREATE EXTENSION IF NOT EXISTS "uuid-ossp";

CREATE TABLE users (
    id            UUID PRIMARY KEY DEFAULT uuid_generate_v4(),
    clinic_id     UUID NOT NULL,
    email         VARCHAR(255) UNIQUE NOT NULL,
    password_hash VARCHAR(255) NOT NULL,
    first_name    VARCHAR(100) NOT NULL,
    last_name     VARCHAR(100) NOT NULL,
    iin           VARCHAR(512),       -- AES-256-GCM encrypted (ЗРК о персональных данных)
    phone         VARCHAR(20),
    role          VARCHAR(50) NOT NULL DEFAULT 'doctor',
    is_active     BOOLEAN NOT NULL DEFAULT true,
    created_at    TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at    TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE INDEX idx_users_clinic_id ON users(clinic_id);
CREATE INDEX idx_users_email     ON users(email);
CREATE INDEX idx_users_role      ON users(role);

-- +goose Down
DROP TABLE users;

-- +goose Up
CREATE TABLE integration_configs (
    id           UUID PRIMARY KEY DEFAULT uuid_generate_v4(),
    clinic_id    UUID NOT NULL,
    provider     VARCHAR(100) NOT NULL,   -- "idoctor", "olymp", "invivo"
    is_active    BOOLEAN NOT NULL DEFAULT true,
    config       JSONB NOT NULL,           -- API ключи, URL (зашифрованы)
    created_at   TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at   TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    UNIQUE(clinic_id, provider)
);

-- +goose Down
DROP TABLE integration_configs;

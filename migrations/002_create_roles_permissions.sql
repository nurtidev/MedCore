-- +goose Up
CREATE TABLE roles (
    id          UUID PRIMARY KEY DEFAULT uuid_generate_v4(),
    name        VARCHAR(50) UNIQUE NOT NULL,
    description TEXT,
    created_at  TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE TABLE permissions (
    id          UUID PRIMARY KEY DEFAULT uuid_generate_v4(),
    name        VARCHAR(100) UNIQUE NOT NULL,
    description TEXT
);

CREATE TABLE role_permissions (
    role_id       UUID REFERENCES roles(id) ON DELETE CASCADE,
    permission_id UUID REFERENCES permissions(id) ON DELETE CASCADE,
    PRIMARY KEY (role_id, permission_id)
);

-- Seed default roles
INSERT INTO roles (name, description) VALUES
    ('doctor',       'Врач — просмотр пациентов и расписания'),
    ('coordinator',  'Координатор — управление пациентами, расписанием и просмотр биллинга'),
    ('admin',        'Администратор клиники — полное управление в рамках клиники'),
    ('super_admin',  'Суперадмин SaaS-платформы — полный доступ');

-- Seed default permissions
INSERT INTO permissions (name, description) VALUES
    ('patients:read',   'Просмотр списка пациентов'),
    ('patients:write',  'Редактирование данных пациентов'),
    ('schedule:read',   'Просмотр расписания'),
    ('schedule:write',  'Редактирование расписания'),
    ('billing:read',    'Просмотр биллинга'),
    ('billing:manage',  'Управление биллингом'),
    ('analytics:read',  'Просмотр аналитики'),
    ('users:manage',    'Управление пользователями'),
    ('clinics:manage',  'Управление клиниками');

-- Seed role_permissions matrix
INSERT INTO role_permissions (role_id, permission_id)
SELECT r.id, p.id FROM roles r, permissions p
WHERE r.name = 'doctor' AND p.name IN ('patients:read', 'schedule:read');

INSERT INTO role_permissions (role_id, permission_id)
SELECT r.id, p.id FROM roles r, permissions p
WHERE r.name = 'coordinator' AND p.name IN ('patients:read', 'patients:write', 'schedule:read', 'schedule:write', 'billing:read');

INSERT INTO role_permissions (role_id, permission_id)
SELECT r.id, p.id FROM roles r, permissions p
WHERE r.name = 'admin' AND p.name IN ('patients:read', 'patients:write', 'schedule:read', 'schedule:write', 'billing:read', 'billing:manage', 'analytics:read', 'users:manage');

INSERT INTO role_permissions (role_id, permission_id)
SELECT r.id, p.id FROM roles r, permissions p
WHERE r.name = 'super_admin';

-- +goose Down
DROP TABLE role_permissions;
DROP TABLE permissions;
DROP TABLE roles;

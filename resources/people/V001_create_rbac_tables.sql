-- +migrate Up
CREATE TABLE IF NOT EXISTS people (
    id            BIGSERIAL   PRIMARY KEY,
    address       TEXT        NOT NULL UNIQUE,   -- EVM wallet address (checksummed)
    password_hash TEXT        NOT NULL DEFAULT '',
    nickname      TEXT        NOT NULL DEFAULT '',
    avatar_url    TEXT        NOT NULL DEFAULT '',
    email         TEXT        NOT NULL DEFAULT '',
    created_at    TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at    TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE INDEX IF NOT EXISTS idx_people_address ON people (address);

CREATE TABLE IF NOT EXISTS roles (
    id          BIGSERIAL   PRIMARY KEY,
    name        TEXT        NOT NULL UNIQUE,
    description TEXT        NOT NULL DEFAULT '',
    created_at  TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE TABLE IF NOT EXISTS permissions (
    id          BIGSERIAL   PRIMARY KEY,
    resource    TEXT        NOT NULL,
    action      TEXT        NOT NULL,
    description TEXT        NOT NULL DEFAULT '',
    created_at  TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    UNIQUE (resource, action)
);

CREATE TABLE IF NOT EXISTS role_permissions (
    role_id       BIGINT NOT NULL REFERENCES roles(id)       ON DELETE CASCADE,
    permission_id BIGINT NOT NULL REFERENCES permissions(id) ON DELETE CASCADE,
    PRIMARY KEY (role_id, permission_id)
);

CREATE TABLE IF NOT EXISTS user_roles (
    user_id BIGINT NOT NULL REFERENCES people(id) ON DELETE CASCADE,
    role_id BIGINT NOT NULL REFERENCES roles(id)  ON DELETE CASCADE,
    PRIMARY KEY (user_id, role_id)
);

-- +migrate Down
DROP TABLE IF EXISTS people;
DROP TABLE IF EXISTS roles;
DROP TABLE IF EXISTS permissions;
DROP TABLE IF EXISTS user_roles;
DROP TABLE IF EXISTS role_permissions;
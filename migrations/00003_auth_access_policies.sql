-- +goose Up
CREATE TABLE auth_access_policies (
    id varchar(100) PRIMARY KEY,
    kind varchar(20) NOT NULL CHECK (kind IN ('allowlist', 'blacklist')),
    principal_type varchar(20) NOT NULL CHECK (principal_type IN ('email', 'google_subject')),
    principal varchar(320) NOT NULL CHECK (length(btrim(principal)) BETWEEN 1 AND 320),
    reason varchar(500) NOT NULL CHECK (length(btrim(reason)) BETWEEN 1 AND 500),
    enabled boolean NOT NULL DEFAULT true,
    expires_at timestamptz,
    created_at timestamptz NOT NULL DEFAULT now(),
    updated_at timestamptz NOT NULL DEFAULT now(),
    CHECK (kind = 'blacklist' OR principal_type = 'email')
);

CREATE UNIQUE INDEX auth_access_policies_identity_idx
    ON auth_access_policies(kind, principal_type, principal);
CREATE INDEX auth_access_policies_active_idx
    ON auth_access_policies(enabled, expires_at);

-- +goose Down
DROP TABLE IF EXISTS auth_access_policies;

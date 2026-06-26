-- ForgeOS: environment variables
-- NOTE: `value` is stored as plaintext in this sprint. Encryption-at-rest
-- (AES-256) lands in the env-var sprint. `is_secret` controls UI masking today.
CREATE TABLE IF NOT EXISTS env_vars (
    id         UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    app_id     UUID NOT NULL REFERENCES apps(id) ON DELETE CASCADE,
    key        TEXT NOT NULL,
    value      TEXT NOT NULL,
    is_secret  BOOLEAN NOT NULL DEFAULT TRUE,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    UNIQUE(app_id, key)
);

CREATE INDEX idx_env_vars_app_id ON env_vars(app_id);

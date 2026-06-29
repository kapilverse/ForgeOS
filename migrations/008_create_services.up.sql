-- ForgeOS: managed services (Postgres, Redis)
CREATE TABLE IF NOT EXISTS services (
    id           UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    user_id      UUID NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    name         TEXT NOT NULL,
    type         TEXT NOT NULL, -- 'postgres', 'redis'
    status       TEXT NOT NULL,
    internal_url TEXT NOT NULL,
    created_at   TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    UNIQUE(user_id, name)
);

CREATE INDEX idx_services_user_id ON services(user_id);

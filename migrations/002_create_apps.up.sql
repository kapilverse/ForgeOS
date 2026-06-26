-- ForgeOS: apps
CREATE TABLE IF NOT EXISTS apps (
    id            UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    user_id       UUID NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    name          TEXT NOT NULL,
    slug          TEXT UNIQUE NOT NULL,
    description   TEXT NOT NULL DEFAULT '',
    docker_image  TEXT NOT NULL DEFAULT '',
    status        TEXT NOT NULL DEFAULT 'created',
    replicas      INT  NOT NULL DEFAULT 1,
    cpu_limit     INT  NOT NULL DEFAULT 512,
    memory_limit  INT  NOT NULL DEFAULT 512,
    port          INT  NOT NULL DEFAULT 8080,
    health_check  TEXT NOT NULL DEFAULT '/',
    created_at    TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at    TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    UNIQUE(user_id, name)
);

CREATE INDEX idx_apps_user_id ON apps(user_id);
CREATE INDEX idx_apps_slug    ON apps(slug);

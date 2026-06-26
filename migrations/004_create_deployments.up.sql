-- ForgeOS: deployments
CREATE TABLE IF NOT EXISTS deployments (
    id            UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    app_id        UUID NOT NULL REFERENCES apps(id) ON DELETE CASCADE,
    version       INT  NOT NULL,
    image_tag     TEXT NOT NULL,
    status        TEXT NOT NULL DEFAULT 'pending',
    error_message TEXT NOT NULL DEFAULT '',
    is_current    BOOLEAN NOT NULL DEFAULT FALSE,
    started_at    TIMESTAMPTZ,
    completed_at  TIMESTAMPTZ,
    created_at    TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    UNIQUE(app_id, version)
);

CREATE INDEX idx_deployments_app_id     ON deployments(app_id);
CREATE INDEX idx_deployments_is_current ON deployments(app_id) WHERE is_current = TRUE;

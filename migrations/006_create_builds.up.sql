-- ForgeOS: builds (docker build logs/status for each deployment)
CREATE TABLE IF NOT EXISTS builds (
    id            UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    deployment_id UUID NOT NULL REFERENCES deployments(id) ON DELETE CASCADE,
    status        TEXT NOT NULL DEFAULT 'pending',
    log           TEXT NOT NULL DEFAULT '',
    duration_ms   INT  NOT NULL DEFAULT 0,
    started_at    TIMESTAMPTZ,
    completed_at  TIMESTAMPTZ,
    created_at    TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    UNIQUE(deployment_id)
);

CREATE INDEX idx_builds_deployment_id ON builds(deployment_id);
CREATE INDEX idx_builds_status       ON builds(status);

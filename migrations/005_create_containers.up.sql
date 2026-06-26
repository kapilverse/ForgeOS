-- ForgeOS: containers (Docker containers ForgeOS manages)
CREATE TABLE IF NOT EXISTS containers (
    id            UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    app_id        UUID NOT NULL REFERENCES apps(id) ON DELETE CASCADE,
    deployment_id UUID NOT NULL REFERENCES deployments(id) ON DELETE CASCADE,
    container_id  TEXT NOT NULL,
    name          TEXT NOT NULL,
    status        TEXT NOT NULL DEFAULT 'starting',
    port_mapping  INT  NOT NULL DEFAULT 0,
    started_at    TIMESTAMPTZ,
    stopped_at    TIMESTAMPTZ,
    restart_count INT  NOT NULL DEFAULT 0,
    created_at    TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE INDEX idx_containers_app_id        ON containers(app_id);
CREATE INDEX idx_containers_deployment_id ON containers(deployment_id);
CREATE INDEX idx_containers_status        ON containers(status);

-- ForgeOS: add git repo source fields to apps (build-mode deployments)
ALTER TABLE apps
    ADD COLUMN IF NOT EXISTS repo_url TEXT NOT NULL DEFAULT '',
    ADD COLUMN IF NOT EXISTS branch   TEXT NOT NULL DEFAULT 'main';

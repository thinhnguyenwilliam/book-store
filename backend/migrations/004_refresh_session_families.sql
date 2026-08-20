ALTER TABLE auth.refresh_sessions
    ADD COLUMN IF NOT EXISTS family_id UUID;

UPDATE auth.refresh_sessions
SET family_id = id
WHERE family_id IS NULL;

ALTER TABLE auth.refresh_sessions
    ALTER COLUMN family_id SET NOT NULL;

CREATE INDEX IF NOT EXISTS idx_refresh_sessions_family
    ON auth.refresh_sessions (family_id, created_at DESC);

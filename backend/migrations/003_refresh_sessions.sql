CREATE TABLE IF NOT EXISTS auth.refresh_sessions (
    id UUID PRIMARY KEY,
    account_id UUID NOT NULL REFERENCES auth.accounts(id) ON DELETE CASCADE,
    token_hash CHAR(64) NOT NULL UNIQUE,
    expires_at TIMESTAMPTZ NOT NULL,
    revoked_at TIMESTAMPTZ,
    replaced_by_id UUID REFERENCES auth.refresh_sessions(id),
    last_used_at TIMESTAMPTZ,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE INDEX IF NOT EXISTS idx_refresh_sessions_account
    ON auth.refresh_sessions (account_id, created_at DESC);

CREATE INDEX IF NOT EXISTS idx_refresh_sessions_expiry
    ON auth.refresh_sessions (expires_at)
    WHERE revoked_at IS NULL;

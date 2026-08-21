CREATE TABLE IF NOT EXISTS auth.account_identities (
    provider VARCHAR(32) NOT NULL,
    subject VARCHAR(255) NOT NULL,
    account_id UUID NOT NULL REFERENCES auth.accounts(id) ON DELETE CASCADE,
    email TEXT NOT NULL,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    PRIMARY KEY (provider, subject),
    UNIQUE (provider, account_id)
);

CREATE INDEX IF NOT EXISTS idx_account_identities_account_id
    ON auth.account_identities (account_id);

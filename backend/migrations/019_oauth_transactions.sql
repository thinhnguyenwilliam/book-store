CREATE TABLE IF NOT EXISTS auth.oauth_transactions (
 state_hash varchar(64) PRIMARY KEY,
 provider varchar(32) NOT NULL,
 redirect_uri text NOT NULL,
 verifier text NOT NULL,
 create_account boolean NOT NULL,
 expires_at timestamptz NOT NULL
);
CREATE INDEX IF NOT EXISTS idx_oauth_transactions_expiry ON auth.oauth_transactions(expires_at);

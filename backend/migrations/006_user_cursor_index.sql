CREATE INDEX IF NOT EXISTS idx_user_profiles_cursor
    ON users.user_profiles (created_at DESC, id DESC);

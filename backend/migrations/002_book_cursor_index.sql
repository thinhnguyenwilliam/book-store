CREATE INDEX IF NOT EXISTS idx_books_cursor
    ON catalog.books (created_at DESC, id DESC);

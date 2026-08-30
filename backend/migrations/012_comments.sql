CREATE SCHEMA IF NOT EXISTS comments;

CREATE TABLE IF NOT EXISTS comments.comments (
    id UUID PRIMARY KEY,
    book_id UUID NOT NULL,
    author_id UUID NOT NULL,
    author_name VARCHAR(120) NOT NULL,
    parent_id UUID,
    root_id UUID NOT NULL,
    depth SMALLINT NOT NULL CHECK (depth BETWEEN 0 AND 3),
    content TEXT NOT NULL CHECK (char_length(content) BETWEEN 1 AND 2000),
    status VARCHAR(20) NOT NULL DEFAULT 'published'
        CHECK (status IN ('published', 'hidden', 'deleted')),
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    deleted_at TIMESTAMPTZ,
    CONSTRAINT fk_comments_parent
        FOREIGN KEY (parent_id) REFERENCES comments.comments(id) ON DELETE RESTRICT,
    CONSTRAINT fk_comments_root
        FOREIGN KEY (root_id) REFERENCES comments.comments(id) ON DELETE RESTRICT,
    CONSTRAINT chk_comment_tree_shape CHECK (
        (parent_id IS NULL AND root_id = id AND depth = 0)
        OR
        (parent_id IS NOT NULL AND depth BETWEEN 1 AND 3)
    )
);

CREATE INDEX IF NOT EXISTS idx_comments_book_roots_cursor
    ON comments.comments (book_id, created_at DESC, id DESC)
    WHERE parent_id IS NULL;

CREATE INDEX IF NOT EXISTS idx_comments_root_replies_cursor
    ON comments.comments (root_id, created_at ASC, id ASC)
    WHERE parent_id IS NOT NULL;

CREATE INDEX IF NOT EXISTS idx_comments_parent
    ON comments.comments (parent_id);

CREATE INDEX IF NOT EXISTS idx_comments_author
    ON comments.comments (author_id, created_at DESC);

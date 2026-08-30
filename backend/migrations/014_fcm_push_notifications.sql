CREATE TABLE IF NOT EXISTS notifications.device_installations (
    id UUID PRIMARY KEY,
    device_id UUID NOT NULL UNIQUE,
    user_id UUID NOT NULL,
    application VARCHAR(20) NOT NULL CHECK (application IN ('storefront', 'admin')),
    platform VARCHAR(20) NOT NULL CHECK (platform IN ('web', 'android', 'ios')),
    registration_token TEXT NOT NULL,
    last_seen_at TIMESTAMPTZ NOT NULL,
    disabled_at TIMESTAMPTZ,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    CHECK (LENGTH(registration_token) BETWEEN 20 AND 4096)
);

CREATE INDEX IF NOT EXISTS idx_device_installations_user_active
    ON notifications.device_installations (user_id, updated_at DESC)
    WHERE disabled_at IS NULL;

CREATE UNIQUE INDEX IF NOT EXISTS idx_device_installations_active_token
    ON notifications.device_installations (registration_token)
    WHERE disabled_at IS NULL;

CREATE TABLE IF NOT EXISTS notifications.push_deliveries (
    id UUID PRIMARY KEY,
    event_id VARCHAR(128) NOT NULL,
    notification_id UUID NOT NULL,
    user_id UUID NOT NULL,
    installation_id UUID,
    notification_type VARCHAR(100) NOT NULL,
    title VARCHAR(200) NOT NULL,
    body TEXT NOT NULL,
    data JSONB NOT NULL DEFAULT '{}'::JSONB,
    status VARCHAR(20) NOT NULL CHECK (status IN ('pending', 'sending', 'sent', 'failed', 'skipped')),
    attempts INTEGER NOT NULL DEFAULT 0,
    last_error TEXT NOT NULL DEFAULT '',
    provider_message_id VARCHAR(512) NOT NULL DEFAULT '',
    sent_at TIMESTAMPTZ,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    CONSTRAINT fk_push_delivery_notification
        FOREIGN KEY (notification_id)
        REFERENCES notifications.notifications(id)
        ON DELETE CASCADE,
    CONSTRAINT fk_push_delivery_installation
        FOREIGN KEY (installation_id)
        REFERENCES notifications.device_installations(id)
        ON DELETE SET NULL,
    UNIQUE (notification_id, installation_id)
);

CREATE INDEX IF NOT EXISTS idx_push_deliveries_retry
    ON notifications.push_deliveries (status, updated_at, id)
    WHERE status IN ('pending', 'failed', 'sending');

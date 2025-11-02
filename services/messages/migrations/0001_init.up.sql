CREATE TABLE IF NOT EXISTS messages (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    conv_id UUID NOT NULL,
    from_device_id UUID NOT NULL,
    to_device_id UUID NOT NULL,
    ciphertext BYTEA NOT NULL,
    header JSONB NOT NULL,
    sent_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    received_at TIMESTAMPTZ,
    delivered_at TIMESTAMPTZ,
    deleted_at TIMESTAMPTZ
);

CREATE INDEX IF NOT EXISTS idx_messages_to_device_sent
    ON messages (to_device_id, sent_at);

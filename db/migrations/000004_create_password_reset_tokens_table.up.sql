CREATE TABLE IF NOT EXISTS password_reset_tokens (
    token TEXT PRIMARY KEY,
    tenant_id UUID NOT NULL,
    expires_at TIMESTAMPTZ NOT NULL
);
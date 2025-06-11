CREATE TABLE IF NOT EXISTS user_verification_tokens (
    token TEXT PRIMARY KEY,
    tenant_id UUID NOT NULL,
    expires_at TIMESTAMPTZ NOT NULL
);
CREATE TABLE auth_password_resets (
    id              UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    credential_id   UUID         NOT NULL REFERENCES auth_credentials(id) ON DELETE CASCADE,
    token_hash      VARCHAR(64)  NOT NULL,
    expires_at      TIMESTAMPTZ  NOT NULL,
    used_at         TIMESTAMPTZ,
    created_at      TIMESTAMPTZ  NOT NULL DEFAULT NOW(),

    CONSTRAINT chk_not_used CHECK (used_at IS NULL OR used_at <= expires_at)
);

CREATE INDEX idx_auth_password_resets_token ON auth_password_resets (token_hash);
CREATE INDEX idx_auth_password_resets_credential ON auth_password_resets (credential_id);
CREATE INDEX idx_auth_password_resets_expires ON auth_password_resets (expires_at);

CREATE TABLE auth_email_verifications (
    id              UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    credential_id   UUID         NOT NULL REFERENCES auth_credentials(id) ON DELETE CASCADE,
    token_hash      VARCHAR(64)  NOT NULL,
    expires_at      TIMESTAMPTZ  NOT NULL,
    verified_at     TIMESTAMPTZ,
    created_at      TIMESTAMPTZ  NOT NULL DEFAULT NOW()
);

CREATE INDEX idx_auth_email_verifications_token ON auth_email_verifications (token_hash);
CREATE INDEX idx_auth_email_verifications_credential ON auth_email_verifications (credential_id);

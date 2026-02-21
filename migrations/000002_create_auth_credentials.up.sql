CREATE TABLE auth_credentials (
    id                     UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    tenant_id              UUID         NOT NULL REFERENCES auth_tenants(id),
    email                  VARCHAR(320) NOT NULL,
    password_hash          VARCHAR(255),
    provider               VARCHAR(20)  NOT NULL DEFAULT 'email',
    provider_id            VARCHAR(255),
    is_active              BOOLEAN      NOT NULL DEFAULT true,
    is_email_verified      BOOLEAN      NOT NULL DEFAULT false,
    email_verified_at      TIMESTAMPTZ,
    failed_login_attempts  INT          NOT NULL DEFAULT 0,
    locked_until           TIMESTAMPTZ,
    last_login_at          TIMESTAMPTZ,
    password_changed_at    TIMESTAMPTZ,
    created_at             TIMESTAMPTZ  NOT NULL DEFAULT NOW(),
    updated_at             TIMESTAMPTZ  NOT NULL DEFAULT NOW(),

    CONSTRAINT chk_provider CHECK (provider IN ('email', 'google')),
    CONSTRAINT chk_email_has_password CHECK (
        (provider = 'email' AND password_hash IS NOT NULL) OR
        (provider != 'email')
    ),
    CONSTRAINT chk_oauth_has_provider_id CHECK (
        (provider = 'email' AND provider_id IS NULL) OR
        (provider != 'email' AND provider_id IS NOT NULL)
    )
);

CREATE UNIQUE INDEX uq_auth_credentials_email_provider_tenant
    ON auth_credentials (email, provider, tenant_id);

CREATE UNIQUE INDEX uq_auth_credentials_provider_id_tenant
    ON auth_credentials (provider_id, provider, tenant_id)
    WHERE provider_id IS NOT NULL;

CREATE INDEX idx_auth_credentials_tenant ON auth_credentials (tenant_id);
CREATE INDEX idx_auth_credentials_email_tenant ON auth_credentials (email, tenant_id);
CREATE INDEX idx_auth_credentials_provider_tenant ON auth_credentials (provider, tenant_id);
CREATE INDEX idx_auth_credentials_locked ON auth_credentials (locked_until)
    WHERE locked_until IS NOT NULL;

CREATE TRIGGER update_auth_credentials_updated_at
    BEFORE UPDATE ON auth_credentials
    FOR EACH ROW EXECUTE FUNCTION update_updated_at_column();

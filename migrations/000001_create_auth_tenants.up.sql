CREATE EXTENSION IF NOT EXISTS "pgcrypto";

CREATE TABLE auth_tenants (
    id              UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    code            VARCHAR(50)  NOT NULL UNIQUE,
    name            VARCHAR(255) NOT NULL,
    is_active       BOOLEAN      NOT NULL DEFAULT true,
    settings        JSONB        NOT NULL DEFAULT '{}',
    created_at      TIMESTAMPTZ  NOT NULL DEFAULT NOW(),
    updated_at      TIMESTAMPTZ  NOT NULL DEFAULT NOW()
);

CREATE INDEX idx_auth_tenants_code ON auth_tenants (code);
CREATE INDEX idx_auth_tenants_is_active ON auth_tenants (is_active) WHERE is_active = true;

CREATE OR REPLACE FUNCTION update_updated_at_column()
RETURNS TRIGGER AS $$
BEGIN
    NEW.updated_at = NOW();
    RETURN NEW;
END;
$$ language 'plpgsql';

CREATE TRIGGER update_auth_tenants_updated_at
    BEFORE UPDATE ON auth_tenants
    FOR EACH ROW EXECUTE FUNCTION update_updated_at_column();

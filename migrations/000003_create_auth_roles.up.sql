CREATE TABLE auth_roles (
    id          UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    tenant_id   UUID         NOT NULL REFERENCES auth_tenants(id),
    code        VARCHAR(50)  NOT NULL,
    name        VARCHAR(100) NOT NULL,
    description TEXT,
    is_default  BOOLEAN      NOT NULL DEFAULT false,
    is_system   BOOLEAN      NOT NULL DEFAULT false,
    created_at  TIMESTAMPTZ  NOT NULL DEFAULT NOW(),
    updated_at  TIMESTAMPTZ  NOT NULL DEFAULT NOW(),

    CONSTRAINT uq_auth_roles_code_tenant UNIQUE (code, tenant_id)
);

CREATE INDEX idx_auth_roles_tenant ON auth_roles (tenant_id);
CREATE INDEX idx_auth_roles_default ON auth_roles (tenant_id, is_default) WHERE is_default = true;

CREATE TRIGGER update_auth_roles_updated_at
    BEFORE UPDATE ON auth_roles
    FOR EACH ROW EXECUTE FUNCTION update_updated_at_column();

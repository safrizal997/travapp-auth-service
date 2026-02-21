CREATE TABLE auth_role_assignments (
    id              UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    credential_id   UUID        NOT NULL REFERENCES auth_credentials(id) ON DELETE CASCADE,
    role_id         UUID        NOT NULL REFERENCES auth_roles(id) ON DELETE CASCADE,
    tenant_id       UUID        NOT NULL REFERENCES auth_tenants(id),
    assigned_by     UUID,
    assigned_at     TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    expires_at      TIMESTAMPTZ,

    CONSTRAINT uq_credential_tenant_role UNIQUE (credential_id, tenant_id)
);

CREATE INDEX idx_auth_role_assignments_credential ON auth_role_assignments (credential_id);
CREATE INDEX idx_auth_role_assignments_role ON auth_role_assignments (role_id);
CREATE INDEX idx_auth_role_assignments_tenant ON auth_role_assignments (tenant_id);
CREATE INDEX idx_auth_role_assignments_expires ON auth_role_assignments (expires_at)
    WHERE expires_at IS NOT NULL;

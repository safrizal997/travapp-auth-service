CREATE TABLE auth_login_history (
    id              UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    credential_id   UUID         REFERENCES auth_credentials(id) ON DELETE SET NULL,
    tenant_id       UUID         NOT NULL REFERENCES auth_tenants(id),
    login_method    VARCHAR(20)  NOT NULL,
    ip_address      INET         NOT NULL,
    user_agent      TEXT,
    status          VARCHAR(20)  NOT NULL,
    failure_reason  VARCHAR(100),
    metadata        JSONB        DEFAULT '{}',
    created_at      TIMESTAMPTZ  NOT NULL DEFAULT NOW()
);

CREATE INDEX idx_auth_login_history_credential ON auth_login_history (credential_id);
CREATE INDEX idx_auth_login_history_tenant ON auth_login_history (tenant_id);
CREATE INDEX idx_auth_login_history_created ON auth_login_history (created_at);
CREATE INDEX idx_auth_login_history_ip ON auth_login_history (ip_address);
CREATE INDEX idx_auth_login_history_status ON auth_login_history (status) WHERE status = 'failed';

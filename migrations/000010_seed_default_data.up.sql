-- Seed default tenant
INSERT INTO auth_tenants (id, code, name, is_active, settings) VALUES
(
    gen_random_uuid(),
    'tenant_travel_default',
    'Default Travel Tenant',
    true,
    '{
        "password_policy": {"min_length": 8, "require_uppercase": true},
        "oauth_providers": ["google"],
        "session_config": {"access_token_ttl": 900, "refresh_token_ttl": 2592000},
        "max_failed_attempts": 5,
        "lockout_duration_minutes": 30
    }'::jsonb
);

-- Seed default roles for the default tenant
INSERT INTO auth_roles (id, tenant_id, code, name, description, is_default, is_system)
SELECT
    gen_random_uuid(),
    t.id,
    r.code,
    r.name,
    r.description,
    r.is_default,
    r.is_system
FROM auth_tenants t
CROSS JOIN (VALUES
    ('super_admin', 'Super Administrator', 'Full system access', false, true),
    ('admin',       'Administrator',       'Administrative access', false, true),
    ('agent',       'Travel Agent',        'Travel agent access', false, false),
    ('traveler',    'Traveler',            'Standard traveler access', true, true)
) AS r(code, name, description, is_default, is_system)
WHERE t.code = 'tenant_travel_default';

-- Seed permissions
INSERT INTO auth_permissions (id, code, name, module, description) VALUES
(gen_random_uuid(), 'booking:create',   'Create Booking',      'booking', 'Create new bookings'),
(gen_random_uuid(), 'booking:read',     'View Booking',        'booking', 'View own bookings'),
(gen_random_uuid(), 'booking:update',   'Update Booking',      'booking', 'Update own bookings'),
(gen_random_uuid(), 'booking:delete',   'Cancel Booking',      'booking', 'Cancel own bookings'),
(gen_random_uuid(), 'booking:read:all', 'View All Bookings',   'booking', 'View all bookings'),
(gen_random_uuid(), 'user:read',        'View Own Profile',    'user',    'View own user profile'),
(gen_random_uuid(), 'user:read:all',    'View All Users',      'user',    'View all user profiles'),
(gen_random_uuid(), 'user:update',      'Update Own Profile',  'user',    'Update own user profile'),
(gen_random_uuid(), 'user:manage',      'Manage Users',        'user',    'Manage all users'),
(gen_random_uuid(), 'payment:process',  'Process Payment',     'payment', 'Process payments'),
(gen_random_uuid(), 'payment:refund',   'Issue Refund',        'payment', 'Issue payment refunds'),
(gen_random_uuid(), 'admin:settings',   'Manage Settings',     'admin',   'Manage application settings'),
(gen_random_uuid(), 'admin:roles',      'Manage Roles',        'admin',   'Manage roles and permissions'),
(gen_random_uuid(), 'tenant:manage',    'Manage Tenant Config','admin',   'Manage tenant configuration');

-- Assign all permissions to super_admin role
INSERT INTO auth_role_permissions (id, role_id, permission_id)
SELECT gen_random_uuid(), r.id, p.id
FROM auth_roles r
CROSS JOIN auth_permissions p
JOIN auth_tenants t ON r.tenant_id = t.id
WHERE r.code = 'super_admin' AND t.code = 'tenant_travel_default';

-- Assign admin permissions (all except tenant:manage)
INSERT INTO auth_role_permissions (id, role_id, permission_id)
SELECT gen_random_uuid(), r.id, p.id
FROM auth_roles r
CROSS JOIN auth_permissions p
JOIN auth_tenants t ON r.tenant_id = t.id
WHERE r.code = 'admin' AND t.code = 'tenant_travel_default'
AND p.code != 'tenant:manage';

-- Assign agent permissions
INSERT INTO auth_role_permissions (id, role_id, permission_id)
SELECT gen_random_uuid(), r.id, p.id
FROM auth_roles r
CROSS JOIN auth_permissions p
JOIN auth_tenants t ON r.tenant_id = t.id
WHERE r.code = 'agent' AND t.code = 'tenant_travel_default'
AND p.code IN ('booking:create', 'booking:read', 'booking:update', 'booking:delete', 'booking:read:all', 'user:read', 'user:read:all', 'payment:process');

-- Assign traveler permissions
INSERT INTO auth_role_permissions (id, role_id, permission_id)
SELECT gen_random_uuid(), r.id, p.id
FROM auth_roles r
CROSS JOIN auth_permissions p
JOIN auth_tenants t ON r.tenant_id = t.id
WHERE r.code = 'traveler' AND t.code = 'tenant_travel_default'
AND p.code IN ('booking:create', 'booking:read', 'booking:update', 'booking:delete', 'user:read', 'user:update', 'payment:process');

# Multi-Tenancy Guide

Dokumentasi arsitektur dan implementasi multi-tenancy pada TravApp Auth Service.

## Daftar Isi

- [Konsep](#konsep)
- [Arsitektur](#arsitektur)
  - [Strategi Isolasi](#strategi-isolasi)
  - [Data Model](#data-model)
  - [Tenant-Scoped vs Global](#tenant-scoped-vs-global)
- [Flow Tenant dalam Aplikasi](#flow-tenant-dalam-aplikasi)
  - [Request Lifecycle](#request-lifecycle)
  - [Register Flow](#register-flow)
  - [Login Flow](#login-flow)
  - [Token Flow](#token-flow)
- [Database Isolation](#database-isolation)
  - [Unique Constraints](#unique-constraints)
  - [Query Patterns](#query-patterns)
  - [Indexes](#indexes)
- [Redis Isolation](#redis-isolation)
- [RBAC per Tenant](#rbac-per-tenant)
  - [Roles](#roles)
  - [Permissions](#permissions)
  - [Role Assignment](#role-assignment)
  - [Role-Permission Matrix](#role-permission-matrix)
- [Tenant Configuration](#tenant-configuration)
- [Cross-Tenant Scenarios](#cross-tenant-scenarios)
- [Security Guarantees](#security-guarantees)
- [Onboarding Tenant Baru](#onboarding-tenant-baru)

---

## Konsep

### Apa itu Multi-Tenancy?

Multi-tenancy memungkinkan satu instance auth service digunakan oleh banyak organisasi (tenant) secara bersamaan, dengan data terisolasi per tenant.

```
┌─────────────────────────────────────────────────────────┐
│                   Auth Service (1 instance)             │
│                                                         │
│  ┌─────────────┐  ┌─────────────┐  ┌─────────────┐     │
│  │  Tenant A   │  │  Tenant B   │  │  Tenant C   │     │
│  │  (Agency X) │  │  (Agency Y) │  │  (Agency Z) │     │
│  │             │  │             │  │             │     │
│  │ Users: 50   │  │ Users: 200  │  │ Users: 30   │     │
│  │ Roles: 4    │  │ Roles: 4    │  │ Roles: 4    │     │
│  │ Configs: {} │  │ Configs: {} │  │ Configs: {} │     │
│  └─────────────┘  └─────────────┘  └─────────────┘     │
│                                                         │
│  ┌─────────────────────────────────────────────────┐    │
│  │           Shared Database & Redis                │    │
│  │       (data dipartisi by tenant_id)              │    │
│  └─────────────────────────────────────────────────┘    │
└─────────────────────────────────────────────────────────┘
```

### Terminologi

| Istilah | Keterangan |
|---------|------------|
| **Tenant** | Organisasi/bisnis yang menggunakan platform (travel agency) |
| **Credential** | User account, di-scope ke satu tenant |
| **tenant_id** | UUID yang mengidentifikasi tenant di setiap operasi |
| **Tenant isolation** | Jaminan bahwa data tenant A tidak bisa diakses oleh tenant B |

---

## Arsitektur

### Strategi Isolasi

Service ini menggunakan **shared database, shared schema** dengan **row-level tenant isolation** via kolom `tenant_id`.

```
┌──────────────────────────────────────────────┐
│            Isolation Strategies              │
│                                              │
│  ┌──────────────┐  Separate DB per tenant    │
│  │  Database    │  (TIDAK digunakan)         │
│  │  per Tenant  │                            │
│  └──────────────┘                            │
│                                              │
│  ┌──────────────┐  Separate schema per tenant│
│  │  Schema      │  (TIDAK digunakan)         │
│  │  per Tenant  │                            │
│  └──────────────┘                            │
│                                              │
│  ┌──────────────┐  Shared table, filtered    │
│  │  Row-Level   │  by tenant_id column       │
│  │  Isolation   │  ✅ DIGUNAKAN              │
│  └──────────────┘                            │
└──────────────────────────────────────────────┘
```

**Alasan memilih row-level isolation:**
- Operasional sederhana (1 database, 1 connection pool)
- Migrasi schema sekali untuk semua tenant
- Cocok untuk SaaS dengan jumlah tenant sedang (puluhan-ratusan)
- Trade-off: perlu disiplin memfilter `tenant_id` di setiap query

### Data Model

```
                    tenant_id scoping
                    ==================

auth_tenants ─────┬──→ auth_credentials
                  │         │
                  ├──→ auth_roles
                  │         │
                  ├──→ auth_role_assignments
                  │
                  └──→ auth_login_history


                    NOT tenant-scoped
                    ==================

                    auth_permissions (global)
                    auth_password_resets (via credential FK)
                    auth_email_verifications (via credential FK)
```

### Tenant-Scoped vs Global

| Tabel | Tenant-Scoped | Keterangan |
|-------|:-------------:|------------|
| `auth_tenants` | - | Self-reference, root table |
| `auth_credentials` | **Ya** | Setiap user di-scope ke 1 tenant |
| `auth_roles` | **Ya** | Setiap role di-scope ke 1 tenant |
| `auth_role_assignments` | **Ya** | Assignment user→role per tenant |
| `auth_login_history` | **Ya** | History di-scope per tenant |
| `auth_permissions` | **Tidak** | Permission definitions shared globally |
| `auth_password_resets` | Indirect | Via credential FK (implicitly tenant-scoped) |
| `auth_email_verifications` | Indirect | Via credential FK (implicitly tenant-scoped) |

---

## Flow Tenant dalam Aplikasi

### Request Lifecycle

```
┌──────────┐     ┌───────────┐     ┌──────────┐     ┌────────────┐     ┌──────────┐
│  Client  │────>│   Router  │────>│ Handler  │────>│  Use Case  │────>│Repository│
│          │     │           │     │          │     │            │     │          │
│ tenant_id│     │ middleware│     │ extract  │     │ validate   │     │ WHERE    │
│ in body  │     │ (CORS,   │     │ from DTO │     │ tenant     │     │ tenant_id│
│ or JWT   │     │  reqID)  │     │ or JWT   │     │ is active  │     │ = $1     │
└──────────┘     └───────────┘     └──────────┘     └────────────┘     └──────────┘
```

**Tenant ID masuk via:**

| Source | Kapan | Contoh |
|--------|-------|--------|
| Request body | Public endpoints (register, login, refresh) | `{"tenant_id": "uuid"}` |
| JWT claims | Authenticated endpoints (logout, change password) | `claims.tenant_id` |
| Query param | OAuth initiate | `?tenant_id=uuid` |

### Register Flow

```
1. Client mengirim POST /register dengan tenant_id di body
2. Handler extract tenant_id dari DTO
3. UseCase:
   a. Validate tenant_id format (UUID)
   b. GetActiveTenant(tenant_id) → pastikan tenant exist & active
   c. GetByEmailAndTenant(email, tenant_id, "email") → cek duplikat
   d. Create credential dengan tenant_id
   e. GetDefaultRole(tenant_id) → ambil role default tenant
   f. AssignRole(credential_id, role_id, tenant_id)
4. Credential tersimpan di-scope ke tenant tersebut
```

### Login Flow

```
1. Client mengirim POST /login dengan tenant_id di body
2. UseCase:
   a. Validate tenant aktif
   b. GetByEmailAndTenant(email, tenant_id, "email")
      → Hanya mencari credential di tenant tersebut
      → user@example.com di Tenant A ≠ user@example.com di Tenant B
   c. Verify password
   d. Generate JWT dengan tenant_id di claims
   e. Store refresh token (Redis, keyed by credential_id)
3. JWT yang dihasilkan mengandung tenant_id
```

### Token Flow

JWT claims menyertakan `tenant_id`:

```json
{
  "sub": "credential-uuid",
  "tenant_id": "tenant-uuid",
  "role": "traveler",
  "permissions": ["booking:create", "..."],
  "provider": "email",
  "token_type": "access"
}
```

Saat authenticated endpoint dipanggil:
1. Auth middleware extract JWT
2. `tenant_id` dari claims digunakan untuk validate operasi
3. Contoh: logout → validate credential + tenant match

Saat refresh token:
1. Client kirim `refresh_token` + `tenant_id` di body
2. UseCase validate bahwa token's tenant_id == request tenant_id
3. Mencegah cross-tenant token reuse

---

## Database Isolation

### Unique Constraints

Constraint memastikan tidak ada data duplikat **di dalam satu tenant**, tapi memperbolehkan data yang sama **di tenant berbeda**.

```sql
-- Satu email + provider hanya bisa 1x per tenant
UNIQUE (email, provider, tenant_id)

-- Satu OAuth provider ID hanya bisa 1x per tenant
UNIQUE (provider_id, provider, tenant_id) WHERE provider_id IS NOT NULL

-- Satu user hanya punya 1 role per tenant
UNIQUE (credential_id, tenant_id)

-- Satu role code hanya bisa 1x per tenant
UNIQUE (code, tenant_id)
```

**Implikasi:**

```
✅ user@example.com + email + Tenant A  → OK
✅ user@example.com + email + Tenant B  → OK (tenant berbeda)
✅ user@example.com + google + Tenant A → OK (provider berbeda)
❌ user@example.com + email + Tenant A  → CONFLICT (duplikat)
```

### Query Patterns

**Semua query credential dan role HARUS menyertakan tenant_id:**

```sql
-- ✅ BENAR: Query dengan tenant_id
SELECT * FROM auth_credentials
WHERE email = $1 AND tenant_id = $2 AND provider = $3;

SELECT * FROM auth_roles
WHERE code = $1 AND tenant_id = $2;

SELECT * FROM auth_role_assignments
WHERE credential_id = $1 AND tenant_id = $2;

-- ❌ SALAH: Query tanpa tenant_id (bisa leak data cross-tenant)
SELECT * FROM auth_credentials WHERE email = $1;
```

**Exception — Query by primary key (UUID):**

Beberapa query hanya filter by ID (e.g. `GetByID`). Ini aman karena:
- UUID bersifat unguessable
- Hanya digunakan setelah validasi awal (sudah terverifikasi tenant-nya)
- Digunakan untuk operasi internal (update password, verify email, etc.)

```sql
-- Aman: ID sudah divalidasi sebelumnya
SELECT * FROM auth_credentials WHERE id = $1;
UPDATE auth_credentials SET password_hash = $2 WHERE id = $1;
```

### Indexes

Semua index utama menyertakan `tenant_id` untuk performa query tenant-scoped:

```sql
-- Composite indexes dengan tenant_id
CREATE INDEX idx_auth_credentials_email_tenant ON auth_credentials(email, tenant_id);
CREATE INDEX idx_auth_credentials_provider_tenant ON auth_credentials(provider, tenant_id);
CREATE INDEX idx_auth_credentials_tenant ON auth_credentials(tenant_id);
CREATE INDEX idx_auth_roles_tenant ON auth_roles(tenant_id);
CREATE INDEX idx_auth_role_assignments_tenant ON auth_role_assignments(tenant_id);
CREATE INDEX idx_auth_login_history_tenant ON auth_login_history(tenant_id);
```

---

## Redis Isolation

Redis tidak menggunakan `tenant_id` langsung di key pattern. Isolasi dicapai melalui **credential_id** yang sudah ter-scope ke tenant.

```
Key Pattern                              Tenant Isolation
─────────────────────────────────────    ─────────────────
rt:{credentialID}:{jti}                  Via credential (credential = 1 tenant)
blacklist:at:{jti}                       Via JWT (JWT berisi tenant_id)
oauth_state:{state}                      State JSON berisi tenant_id
cache:rbac:{credentialID}                Via credential
login:{email}:{tenant_id}               Langsung di key
reset:{email}:{tenant_id}               Langsung di key
```

**Rate limiting** adalah satu-satunya yang menyertakan `tenant_id` langsung di key. Ini memastikan rate limit di Tenant A tidak mempengaruhi user yang sama di Tenant B.

```
Contoh:
login:user@example.com:tenant-a-uuid  →  count: 3
login:user@example.com:tenant-b-uuid  →  count: 0  (independent)
```

---

## RBAC per Tenant

### Roles

Setiap tenant memiliki **set role sendiri**. Saat tenant dibuat, role default di-seed:

```
Tenant A                          Tenant B
├── super_admin (system)          ├── super_admin (system)
├── admin (system)                ├── admin (system)
├── agent                         ├── agent
└── traveler (default, system)    └── traveler (default, system)
```

Meskipun code-nya sama, ini adalah **record database berbeda** (UUID berbeda, tenant_id berbeda).

**Default role** (`is_default = true`) otomatis di-assign ke user baru saat register.

**System role** (`is_system = true`) tidak bisa dihapus — menjaga integritas RBAC.

### Permissions

Permissions bersifat **global** (tidak punya `tenant_id`). Semua tenant share permission definitions yang sama:

```
auth_permissions (global)
├── booking:create
├── booking:read
├── booking:update
├── booking:delete
├── booking:read:all
├── user:read
├── user:read:all
├── user:update
├── user:manage
├── payment:process
├── payment:refund
├── admin:settings
├── admin:roles
└── tenant:manage
```

Yang **bisa berbeda** per tenant adalah mapping role → permission. Misal Tenant A bisa memberikan permission `payment:refund` ke role `agent`, sementara Tenant B tidak.

### Role Assignment

Satu user hanya punya **satu role per tenant** (enforced by unique constraint):

```sql
UNIQUE (credential_id, tenant_id)
```

Assignment mendukung **temporary roles** via `expires_at`:

```
┌────────────────────────────────────────────┐
│ User: john@example.com di Tenant A         │
│                                            │
│ Role: admin                                │
│ Assigned by: super_admin                   │
│ Assigned at: 2026-01-01                    │
│ Expires at: NULL (permanent)               │
└────────────────────────────────────────────┘

┌────────────────────────────────────────────┐
│ User: jane@example.com di Tenant A         │
│                                            │
│ Role: admin                                │
│ Assigned by: super_admin                   │
│ Assigned at: 2026-02-01                    │
│ Expires at: 2026-03-01 (temporary)         │
└────────────────────────────────────────────┘
```

Query role selalu memfilter expired assignments:

```sql
WHERE (ara.expires_at IS NULL OR ara.expires_at > NOW())
```

### Role-Permission Matrix

Default mapping yang di-seed untuk setiap tenant baru:

| Permission | super_admin | admin | agent | traveler |
|------------|:-----------:|:-----:|:-----:|:--------:|
| `booking:create` | x | x | x | x |
| `booking:read` | x | x | x | x |
| `booking:update` | x | x | x | x |
| `booking:delete` | x | x | x | x |
| `booking:read:all` | x | x | x | |
| `user:read` | x | x | x | x |
| `user:read:all` | x | x | x | |
| `user:update` | x | x | | x |
| `user:manage` | x | x | | |
| `payment:process` | x | x | x | x |
| `payment:refund` | x | x | | |
| `admin:settings` | x | x | | |
| `admin:roles` | x | x | | |
| `tenant:manage` | x | | | |

---

## Tenant Configuration

Setiap tenant memiliki `settings` (JSONB) yang mengkonfigurasi behavior auth:

```json
{
  "password_policy": {
    "min_length": 8,
    "require_uppercase": true
  },
  "oauth_providers": ["google"],
  "session_config": {
    "access_token_ttl": 900,
    "refresh_token_ttl": 2592000
  },
  "max_failed_attempts": 5,
  "lockout_duration_minutes": 30
}
```

### Setting Details

| Setting | Tipe | Default | Keterangan |
|---------|------|---------|------------|
| `password_policy.min_length` | int | 8 | Minimum panjang password |
| `password_policy.require_uppercase` | bool | true | Wajib huruf besar |
| `oauth_providers` | string[] | `["google"]` | OAuth provider yang diaktifkan |
| `session_config.access_token_ttl` | int | 900 | Access token lifetime (detik) |
| `session_config.refresh_token_ttl` | int | 2592000 | Refresh token lifetime (detik) |
| `max_failed_attempts` | int | 5 | Max gagal login sebelum lock |
| `lockout_duration_minutes` | int | 30 | Durasi account lock (menit) |

### Customisasi per Tenant

Tenant bisa memiliki konfigurasi berbeda:

```
Tenant A (Enterprise):
├── min_length: 12
├── max_failed_attempts: 3
├── lockout_duration_minutes: 60
└── access_token_ttl: 300 (5 menit, lebih ketat)

Tenant B (Startup):
├── min_length: 8
├── max_failed_attempts: 10
├── lockout_duration_minutes: 15
└── access_token_ttl: 1800 (30 menit, lebih longgar)
```

---

## Cross-Tenant Scenarios

### Skenario 1: User Terdaftar di Beberapa Tenant

```
user@example.com
├── Tenant A → credential_id: aaa-111
│   ├── role: admin
│   ├── password: (bcrypt hash A)
│   └── login history: [...]
│
└── Tenant B → credential_id: bbb-222
    ├── role: traveler
    ├── password: (bcrypt hash B, bisa berbeda)
    └── login history: [...]
```

- Password **independen** per tenant (user bisa set password berbeda)
- Role **independen** per tenant
- Login history **terpisah** per tenant
- User harus login dengan `tenant_id` spesifik

### Skenario 2: OAuth di Beberapa Tenant

```
user@gmail.com (Google OAuth)
├── Tenant A → credential_id: aaa-111
│   ├── provider: google
│   ├── provider_id: google-sub-12345
│   └── auto-verified: true
│
└── Tenant B → credential_id: bbb-222
    ├── provider: google
    ├── provider_id: google-sub-12345
    └── auto-verified: true
```

- Google provider_id **sama** tapi credential **berbeda** per tenant
- OAuth initiate memerlukan `tenant_id` di query param

### Skenario 3: Email + OAuth di Tenant Sama

```
user@example.com di Tenant A
├── credential_id: aaa-111
│   ├── provider: email
│   └── password_hash: (bcrypt)
│
└── credential_id: aaa-222
    ├── provider: google
    └── provider_id: google-sub-12345
```

- Satu email bisa punya **2 credential** di tenant yang sama (provider berbeda)
- Unique constraint: `(email, provider, tenant_id)`
- Login method menentukan credential mana yang digunakan

### Skenario 4: Refresh Token Cross-Tenant

```
❌ TIDAK BISA:

User login di Tenant A → dapat refresh_token_A
User coba refresh dengan tenant_id = Tenant B
→ 401 Unauthorized (token's tenant_id ≠ request tenant_id)
```

Refresh token menyimpan `tenant_id` di Redis value. Saat refresh, usecase memvalidasi:
```
stored_token.tenant_id == request.tenant_id
```

---

## Security Guarantees

### Data Isolation

| Guarantee | Enforcement | Layer |
|-----------|-------------|-------|
| Credential isolation | `WHERE tenant_id = $1` di semua query | Repository |
| Role isolation | Unique `(code, tenant_id)`, query filter | Database + Repository |
| Login history isolation | `tenant_id` FK + query filter | Database + Repository |
| Rate limit isolation | `{email}:{tenant_id}` key pattern | Redis |
| Token isolation | `tenant_id` di JWT claims + refresh token data | Application |

### Potential Risks & Mitigations

| Risk | Mitigation |
|------|------------|
| Developer lupa filter `tenant_id` | Unique constraints mencegah cross-tenant collision; code review |
| Tenant ID manipulation di request | Validated via UUID format + `GetActiveTenant()` check |
| JWT token reuse cross-tenant | `tenant_id` embedded di JWT, validated by consuming service |
| Refresh token reuse cross-tenant | `tenant_id` stored di Redis, validated saat refresh |
| Rate limit bypass via tenant switch | Rate limit key includes `tenant_id` |

### Rekomendasi untuk Service Lain

Service yang mengkonsumsi JWT dari auth service harus:

1. **Selalu validasi `tenant_id`** dari JWT claims
2. **Scope semua query** dengan `tenant_id` dari JWT
3. **Jangan percaya `tenant_id` dari request body/header** — gunakan yang dari JWT
4. **Log `tenant_id`** di setiap audit entry

```go
// ✅ BENAR: Gunakan tenant_id dari JWT
tenantID := c.GetString("tenant_id") // dari JWT claims via middleware
db.Query("SELECT * FROM bookings WHERE tenant_id = $1", tenantID)

// ❌ SALAH: Gunakan tenant_id dari request
tenantID := request.TenantID // bisa dimanipulasi user
```

---

## Onboarding Tenant Baru

### Langkah-langkah

```
1. Insert ke auth_tenants
   ├── Generate UUID
   ├── Set code (unique slug)
   ├── Set name
   ├── Set settings (JSONB)
   └── is_active = true

2. Seed roles untuk tenant baru
   ├── super_admin (is_system: true)
   ├── admin (is_system: true)
   ├── agent
   └── traveler (is_default: true, is_system: true)

3. Seed role_permissions untuk setiap role
   ├── super_admin → 14 permissions
   ├── admin → 13 permissions
   ├── agent → 8 permissions
   └── traveler → 7 permissions

4. (Optional) Buat super_admin credential
   ├── Register user pertama
   └── Assign role super_admin secara manual
```

### Contoh SQL

```sql
-- 1. Buat tenant
INSERT INTO auth_tenants (id, code, name, is_active, settings)
VALUES (
  gen_random_uuid(),
  'agency-xyz',
  'XYZ Travel Agency',
  true,
  '{
    "password_policy": {"min_length": 8, "require_uppercase": true},
    "oauth_providers": ["google"],
    "session_config": {"access_token_ttl": 900, "refresh_token_ttl": 2592000},
    "max_failed_attempts": 5,
    "lockout_duration_minutes": 30
  }'
);

-- 2. Seed roles (repeat for each role)
INSERT INTO auth_roles (id, tenant_id, code, name, is_default, is_system)
VALUES
  (gen_random_uuid(), '<tenant_id>', 'super_admin', 'Super Administrator', false, true),
  (gen_random_uuid(), '<tenant_id>', 'admin', 'Administrator', false, true),
  (gen_random_uuid(), '<tenant_id>', 'agent', 'Travel Agent', false, false),
  (gen_random_uuid(), '<tenant_id>', 'traveler', 'Traveler', true, true);

-- 3. Seed role_permissions
-- (gunakan subquery untuk resolve role_id dan permission_id)
INSERT INTO auth_role_permissions (id, role_id, permission_id)
SELECT gen_random_uuid(), r.id, p.id
FROM auth_roles r
CROSS JOIN auth_permissions p
WHERE r.tenant_id = '<tenant_id>'
  AND r.code = 'super_admin';
-- (repeat per role dengan filter permission yang sesuai)
```

### Deaktivasi Tenant

```sql
-- Soft deactivate — semua login akan gagal
UPDATE auth_tenants SET is_active = false WHERE id = '<tenant_id>';
```

Efek:
- `GetActiveTenant()` return error → semua register, login, refresh gagal
- Existing access token **masih valid** sampai expired (max 15 menit)
- Refresh token tidak bisa digunakan (tenant check gagal)
- Data **tidak dihapus** — bisa diaktifkan kembali

```sql
-- Reactivate
UPDATE auth_tenants SET is_active = true WHERE id = '<tenant_id>';
```

# Database Reference

Dokumentasi lengkap skema database dan Redis untuk TravApp Auth Service.

## Daftar Isi

- [Overview](#overview)
- [PostgreSQL](#postgresql)
  - [Konfigurasi Koneksi](#konfigurasi-koneksi)
  - [ER Diagram](#er-diagram)
  - [Tables](#tables)
    - [auth_tenants](#auth_tenants)
    - [auth_credentials](#auth_credentials)
    - [auth_roles](#auth_roles)
    - [auth_permissions](#auth_permissions)
    - [auth_role_permissions](#auth_role_permissions)
    - [auth_role_assignments](#auth_role_assignments)
    - [auth_login_history](#auth_login_history)
    - [auth_password_resets](#auth_password_resets)
    - [auth_email_verifications](#auth_email_verifications)
  - [Triggers](#triggers)
  - [Seed Data](#seed-data)
- [Redis](#redis)
  - [Konfigurasi Koneksi](#konfigurasi-koneksi-redis)
  - [Key Patterns](#key-patterns)
    - [Refresh Token](#refresh-token)
    - [Access Token Blacklist](#access-token-blacklist)
    - [OAuth State](#oauth-state)
    - [Rate Limiting](#rate-limiting)
    - [RBAC Cache](#rbac-cache)
  - [Lua Scripts](#lua-scripts)
- [Migrations](#migrations)

---

## Overview

| Komponen | Teknologi | Versi |
|----------|-----------|-------|
| Database | PostgreSQL | 15+ |
| Driver | pgx/v5 (pgxpool) | v5 |
| Cache/Session | Redis | 7+ |
| Driver | go-redis/v9 | v9 |
| Migrations | golang-migrate | - |

---

## PostgreSQL

### Konfigurasi Koneksi

| Parameter | Env Variable | Default | Keterangan |
|-----------|-------------|---------|------------|
| Host | `DB_HOST` | `localhost` | Database host |
| Port | `DB_PORT` | `5432` | Database port |
| User | `DB_USER` | - | **(required)** |
| Password | `DB_PASSWORD` | - | **(required)** |
| Database | `DB_NAME` | - | **(required)** |
| SSL Mode | `DB_SSL_MODE` | `disable` | `disable`, `require`, `verify-ca`, `verify-full` |
| Max Open Conns | `DB_MAX_OPEN_CONNS` | `25` | Connection pool maximum |
| Max Idle Conns | `DB_MAX_IDLE_CONNS` | `5` | Idle connections maintained |
| Conn Max Lifetime | `DB_CONN_MAX_LIFETIME` | `300s` | Connection max lifetime |

**DSN Format:**
```
postgres://user:password@localhost:5432/dbname?sslmode=disable
```

---

### ER Diagram

```
┌──────────────────────┐
│    auth_tenants      │
│──────────────────────│
│  id (PK, UUID)       │
│  code (UNIQUE)       │
│  name                │
│  is_active           │
│  settings (JSONB)    │
│  created_at          │
│  updated_at          │
└──────────┬───────────┘
           │
     ┌─────┴────────────────────────────────────┐
     │                    │                      │
     ▼                    ▼                      ▼
┌─────────────────┐  ┌──────────────┐  ┌──────────────────┐
│ auth_credentials│  │  auth_roles  │  │auth_login_history│
│─────────────────│  │──────────────│  │──────────────────│
│ id (PK)         │  │ id (PK)      │  │ id (PK)          │
│ tenant_id (FK)  │  │ tenant_id(FK)│  │ credential_id(FK)│
│ email           │  │ code         │  │ tenant_id (FK)   │
│ password_hash   │  │ name         │  │ login_method     │
│ provider        │  │ description  │  │ ip_address       │
│ provider_id     │  │ is_default   │  │ user_agent       │
│ is_active       │  │ is_system    │  │ status           │
│ is_email_verified  │ created_at   │  │ failure_reason   │
│ failed_login_*  │  │ updated_at   │  │ metadata (JSONB) │
│ locked_until    │  └──────┬───────┘  │ created_at       │
│ last_login_at   │         │          └──────────────────┘
│ password_changed│         │
│ created_at      │         │
│ updated_at      │    ┌────┴──────────────┐
└────────┬────────┘    │auth_role_permissions│
         │             │───────────────────│
         │             │ id (PK)           │
         │             │ role_id (FK)      │──→ auth_roles
         │             │ permission_id (FK)│──→ auth_permissions
         │             │ created_at        │
         │             └───────────────────┘
         │
         │         ┌───────────────────────┐
         │         │  auth_permissions     │
         │         │───────────────────────│
         │         │  id (PK)             │
         │         │  code (UNIQUE)       │ ← Global (no tenant_id)
         │         │  name                │
         │         │  module              │
         │         │  description         │
         │         │  created_at          │
         │         └───────────────────────┘
         │
         ▼
┌────────────────────────┐
│ auth_role_assignments  │
│────────────────────────│
│ id (PK)                │
│ credential_id (FK)     │──→ auth_credentials
│ role_id (FK)           │──→ auth_roles
│ tenant_id (FK)         │──→ auth_tenants
│ assigned_by            │
│ assigned_at            │
│ expires_at             │
└────────────────────────┘

┌─────────────────────────┐   ┌──────────────────────────────┐
│ auth_password_resets    │   │ auth_email_verifications      │
│─────────────────────────│   │──────────────────────────────│
│ id (PK)                 │   │ id (PK)                      │
│ credential_id (FK)      │   │ credential_id (FK)           │
│ token_hash              │   │ token_hash                   │
│ expires_at              │   │ expires_at                   │
│ used_at                 │   │ verified_at                  │
│ created_at              │   │ created_at                   │
└─────────────────────────┘   └──────────────────────────────┘
```

---

### Tables

---

#### auth_tenants

Root table untuk multi-tenancy. Setiap organisasi/bisnis yang menggunakan platform.

| Column | Type | Nullable | Default | Keterangan |
|--------|------|----------|---------|------------|
| `id` | UUID | NOT NULL | `gen_random_uuid()` | Primary key |
| `code` | VARCHAR(50) | NOT NULL | - | Unique tenant code/slug |
| `name` | VARCHAR(255) | NOT NULL | - | Nama display |
| `is_active` | BOOLEAN | NOT NULL | `true` | Status aktif |
| `settings` | JSONB | NOT NULL | `'{}'` | Konfigurasi tenant |
| `created_at` | TIMESTAMPTZ | NOT NULL | `NOW()` | Waktu dibuat |
| `updated_at` | TIMESTAMPTZ | NOT NULL | `NOW()` | Waktu diupdate (auto) |

**Constraints:**

| Nama | Tipe | Kolom |
|------|------|-------|
| `auth_tenants_pkey` | PRIMARY KEY | `id` |
| `auth_tenants_code_key` | UNIQUE | `code` |

**Indexes:**

| Nama | Kolom | Keterangan |
|------|-------|------------|
| `idx_auth_tenants_code` | `code` | Lookup by code |
| `idx_auth_tenants_is_active` | `is_active` WHERE `is_active = true` | Partial index, filter active |

**Settings JSON Structure:**
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

---

#### auth_credentials

Tabel utama user credentials. Mendukung multi-provider (email + OAuth) per tenant.

| Column | Type | Nullable | Default | Keterangan |
|--------|------|----------|---------|------------|
| `id` | UUID | NOT NULL | `gen_random_uuid()` | Primary key |
| `tenant_id` | UUID | NOT NULL | - | FK → auth_tenants(id) |
| `email` | VARCHAR(320) | NOT NULL | - | Email address |
| `password_hash` | VARCHAR(255) | **NULLABLE** | - | BCrypt hash (NULL untuk OAuth) |
| `provider` | VARCHAR(20) | NOT NULL | `'email'` | `'email'` atau `'google'` |
| `provider_id` | VARCHAR(255) | **NULLABLE** | - | OAuth provider user ID |
| `is_active` | BOOLEAN | NOT NULL | `true` | Account aktif |
| `is_email_verified` | BOOLEAN | NOT NULL | `false` | Email terverifikasi |
| `email_verified_at` | TIMESTAMPTZ | **NULLABLE** | - | Waktu verifikasi email |
| `failed_login_attempts` | INT | NOT NULL | `0` | Jumlah gagal login |
| `locked_until` | TIMESTAMPTZ | **NULLABLE** | - | Waktu lockout berakhir |
| `last_login_at` | TIMESTAMPTZ | **NULLABLE** | - | Login terakhir |
| `password_changed_at` | TIMESTAMPTZ | **NULLABLE** | - | Password terakhir diubah |
| `created_at` | TIMESTAMPTZ | NOT NULL | `NOW()` | Waktu dibuat |
| `updated_at` | TIMESTAMPTZ | NOT NULL | `NOW()` | Waktu diupdate (auto) |

**Constraints:**

| Nama | Tipe | Detail |
|------|------|--------|
| `auth_credentials_pkey` | PRIMARY KEY | `id` |
| `uq_auth_credentials_email_provider_tenant` | UNIQUE | `(email, provider, tenant_id)` |
| `uq_auth_credentials_provider_id_tenant` | UNIQUE | `(provider_id, provider, tenant_id)` WHERE `provider_id IS NOT NULL` |
| `chk_provider` | CHECK | `provider IN ('email', 'google')` |
| `chk_email_has_password` | CHECK | Email provider HARUS punya password_hash; OAuth TIDAK BOLEH |
| `chk_oauth_has_provider_id` | CHECK | Email TIDAK BOLEH punya provider_id; OAuth HARUS punya |
| FK → auth_tenants | FOREIGN KEY | `tenant_id` → `auth_tenants(id)` |

**Indexes:**

| Nama | Kolom | Keterangan |
|------|-------|------------|
| `idx_auth_credentials_tenant` | `tenant_id` | Tenant-scoped queries |
| `idx_auth_credentials_email_tenant` | `(email, tenant_id)` | Email lookup per tenant |
| `idx_auth_credentials_provider_tenant` | `(provider, tenant_id)` | Provider-based lookup |
| `idx_auth_credentials_locked` | `locked_until` WHERE `locked_until IS NOT NULL` | Partial index, locked accounts |

---

#### auth_roles

Role definitions per tenant untuk RBAC.

| Column | Type | Nullable | Default | Keterangan |
|--------|------|----------|---------|------------|
| `id` | UUID | NOT NULL | `gen_random_uuid()` | Primary key |
| `tenant_id` | UUID | NOT NULL | - | FK → auth_tenants(id) |
| `code` | VARCHAR(50) | NOT NULL | - | Role code (e.g. `admin`) |
| `name` | VARCHAR(100) | NOT NULL | - | Nama display |
| `description` | TEXT | **NULLABLE** | - | Deskripsi role |
| `is_default` | BOOLEAN | NOT NULL | `false` | Default role untuk user baru |
| `is_system` | BOOLEAN | NOT NULL | `false` | System role (tidak bisa dihapus) |
| `created_at` | TIMESTAMPTZ | NOT NULL | `NOW()` | Waktu dibuat |
| `updated_at` | TIMESTAMPTZ | NOT NULL | `NOW()` | Waktu diupdate (auto) |

**Constraints:**

| Nama | Tipe | Detail |
|------|------|--------|
| `auth_roles_pkey` | PRIMARY KEY | `id` |
| `uq_auth_roles_code_tenant` | UNIQUE | `(code, tenant_id)` |
| FK → auth_tenants | FOREIGN KEY | `tenant_id` → `auth_tenants(id)` |

**Indexes:**

| Nama | Kolom | Keterangan |
|------|-------|------------|
| `idx_auth_roles_tenant` | `tenant_id` | Tenant-scoped queries |
| `idx_auth_roles_default` | `(tenant_id, is_default)` WHERE `is_default = true` | Find default role |

---

#### auth_permissions

Permission definitions. **Global** — tidak di-scope per tenant.

| Column | Type | Nullable | Default | Keterangan |
|--------|------|----------|---------|------------|
| `id` | UUID | NOT NULL | `gen_random_uuid()` | Primary key |
| `code` | VARCHAR(100) | NOT NULL | - | Permission code (UNIQUE) |
| `name` | VARCHAR(150) | NOT NULL | - | Nama display |
| `module` | VARCHAR(50) | NOT NULL | - | Module/area grouping |
| `description` | TEXT | **NULLABLE** | - | Deskripsi permission |
| `created_at` | TIMESTAMPTZ | NOT NULL | `NOW()` | Waktu dibuat |

**Constraints:**

| Nama | Tipe | Detail |
|------|------|--------|
| `auth_permissions_pkey` | PRIMARY KEY | `id` |
| `auth_permissions_code_key` | UNIQUE | `code` |

**Indexes:**

| Nama | Kolom | Keterangan |
|------|-------|------------|
| `idx_auth_permissions_module` | `module` | Group by module |
| `idx_auth_permissions_code` | `code` | Code lookup |

---

#### auth_role_permissions

Junction table: mapping role → permission (many-to-many).

| Column | Type | Nullable | Default | Keterangan |
|--------|------|----------|---------|------------|
| `id` | UUID | NOT NULL | `gen_random_uuid()` | Primary key |
| `role_id` | UUID | NOT NULL | - | FK → auth_roles(id) ON DELETE CASCADE |
| `permission_id` | UUID | NOT NULL | - | FK → auth_permissions(id) ON DELETE CASCADE |
| `created_at` | TIMESTAMPTZ | NOT NULL | `NOW()` | Waktu dibuat |

**Constraints:**

| Nama | Tipe | Detail |
|------|------|--------|
| `auth_role_permissions_pkey` | PRIMARY KEY | `id` |
| `uq_role_permission` | UNIQUE | `(role_id, permission_id)` |
| FK → auth_roles | FOREIGN KEY | `role_id` ON DELETE CASCADE |
| FK → auth_permissions | FOREIGN KEY | `permission_id` ON DELETE CASCADE |

**Indexes:**

| Nama | Kolom |
|------|-------|
| `idx_auth_role_permissions_role` | `role_id` |
| `idx_auth_role_permissions_permission` | `permission_id` |

---

#### auth_role_assignments

Assigns credential → role per tenant. Satu user hanya punya **satu role per tenant**.

| Column | Type | Nullable | Default | Keterangan |
|--------|------|----------|---------|------------|
| `id` | UUID | NOT NULL | `gen_random_uuid()` | Primary key |
| `credential_id` | UUID | NOT NULL | - | FK → auth_credentials(id) ON DELETE CASCADE |
| `role_id` | UUID | NOT NULL | - | FK → auth_roles(id) ON DELETE CASCADE |
| `tenant_id` | UUID | NOT NULL | - | FK → auth_tenants(id) |
| `assigned_by` | UUID | **NULLABLE** | - | Admin yang assign |
| `assigned_at` | TIMESTAMPTZ | NOT NULL | `NOW()` | Waktu assign |
| `expires_at` | TIMESTAMPTZ | **NULLABLE** | - | Expiry untuk temporary role |

**Constraints:**

| Nama | Tipe | Detail |
|------|------|--------|
| `auth_role_assignments_pkey` | PRIMARY KEY | `id` |
| `uq_credential_tenant_role` | UNIQUE | `(credential_id, tenant_id)` — 1 role per user per tenant |
| FK → auth_credentials | FOREIGN KEY | `credential_id` ON DELETE CASCADE |
| FK → auth_roles | FOREIGN KEY | `role_id` ON DELETE CASCADE |
| FK → auth_tenants | FOREIGN KEY | `tenant_id` |

**Indexes:**

| Nama | Kolom | Keterangan |
|------|-------|------------|
| `idx_auth_role_assignments_credential` | `credential_id` | User lookup |
| `idx_auth_role_assignments_role` | `role_id` | Role lookup |
| `idx_auth_role_assignments_tenant` | `tenant_id` | Tenant-scoped |
| `idx_auth_role_assignments_expires` | `expires_at` WHERE `expires_at IS NOT NULL` | Partial, temporary roles |

---

#### auth_login_history

Audit trail untuk semua login attempt (berhasil dan gagal).

| Column | Type | Nullable | Default | Keterangan |
|--------|------|----------|---------|------------|
| `id` | UUID | NOT NULL | `gen_random_uuid()` | Primary key |
| `credential_id` | UUID | **NULLABLE** | - | FK → auth_credentials(id) ON DELETE SET NULL |
| `tenant_id` | UUID | NOT NULL | - | FK → auth_tenants(id) |
| `login_method` | VARCHAR(20) | NOT NULL | - | `'email'`, `'google'` |
| `ip_address` | INET | NOT NULL | - | Client IP address |
| `user_agent` | TEXT | **NULLABLE** | - | Browser user agent |
| `status` | VARCHAR(20) | NOT NULL | - | `'success'` atau `'failed'` |
| `failure_reason` | VARCHAR(100) | **NULLABLE** | - | Alasan gagal (e.g. `'invalid_password'`, `'account_locked'`) |
| `metadata` | JSONB | - | `'{}'` | Data tambahan |
| `created_at` | TIMESTAMPTZ | NOT NULL | `NOW()` | Waktu attempt |

**Constraints:**

| Nama | Tipe | Detail |
|------|------|--------|
| `auth_login_history_pkey` | PRIMARY KEY | `id` |
| FK → auth_credentials | FOREIGN KEY | `credential_id` ON DELETE SET NULL |
| FK → auth_tenants | FOREIGN KEY | `tenant_id` |

**Indexes:**

| Nama | Kolom | Keterangan |
|------|-------|------------|
| `idx_auth_login_history_credential` | `credential_id` | User history |
| `idx_auth_login_history_tenant` | `tenant_id` | Tenant-scoped |
| `idx_auth_login_history_created` | `created_at` | Time-range queries |
| `idx_auth_login_history_ip` | `ip_address` | IP-based analysis |
| `idx_auth_login_history_status` | `status` WHERE `status = 'failed'` | Partial, failed logins |

---

#### auth_password_resets

One-time password reset tokens.

| Column | Type | Nullable | Default | Keterangan |
|--------|------|----------|---------|------------|
| `id` | UUID | NOT NULL | `gen_random_uuid()` | Primary key |
| `credential_id` | UUID | NOT NULL | - | FK → auth_credentials(id) ON DELETE CASCADE |
| `token_hash` | VARCHAR(64) | NOT NULL | - | SHA256 hash dari reset token |
| `expires_at` | TIMESTAMPTZ | NOT NULL | - | Token expiry (1 jam) |
| `used_at` | TIMESTAMPTZ | **NULLABLE** | - | Waktu token digunakan |
| `created_at` | TIMESTAMPTZ | NOT NULL | `NOW()` | Waktu dibuat |

**Constraints:**

| Nama | Tipe | Detail |
|------|------|--------|
| `auth_password_resets_pkey` | PRIMARY KEY | `id` |
| `chk_not_used` | CHECK | `used_at IS NULL OR used_at <= expires_at` |
| FK → auth_credentials | FOREIGN KEY | `credential_id` ON DELETE CASCADE |

**Indexes:**

| Nama | Kolom | Keterangan |
|------|-------|------------|
| `idx_auth_password_resets_token` | `token_hash` | Token lookup |
| `idx_auth_password_resets_credential` | `credential_id` | User's resets |
| `idx_auth_password_resets_expires` | `expires_at` | Cleanup expired |

---

#### auth_email_verifications

Email verification tokens setelah registrasi.

> **Status:** Tabel ada dan consumer (VerifyEmail) berfungsi, tapi register flow **belum menyimpan** token ke tabel ini.

| Column | Type | Nullable | Default | Keterangan |
|--------|------|----------|---------|------------|
| `id` | UUID | NOT NULL | `gen_random_uuid()` | Primary key |
| `credential_id` | UUID | NOT NULL | - | FK → auth_credentials(id) ON DELETE CASCADE |
| `token_hash` | VARCHAR(64) | NOT NULL | - | SHA256 hash dari verification token |
| `expires_at` | TIMESTAMPTZ | NOT NULL | - | Token expiry (24 jam) |
| `verified_at` | TIMESTAMPTZ | **NULLABLE** | - | Waktu email diverifikasi |
| `created_at` | TIMESTAMPTZ | NOT NULL | `NOW()` | Waktu dibuat |

**Indexes:**

| Nama | Kolom | Keterangan |
|------|-------|------------|
| `idx_auth_email_verifications_token` | `token_hash` | Token lookup |
| `idx_auth_email_verifications_credential` | `credential_id` | User's verifications |

---

### Triggers

Semua tabel yang memiliki kolom `updated_at` menggunakan trigger auto-update:

```sql
CREATE OR REPLACE FUNCTION update_updated_at_column()
RETURNS TRIGGER AS $$
BEGIN
    NEW.updated_at = NOW();
    RETURN NEW;
END;
$$ LANGUAGE plpgsql;
```

| Trigger | Table | Event |
|---------|-------|-------|
| `update_auth_tenants_updated_at` | `auth_tenants` | BEFORE UPDATE |
| `update_auth_credentials_updated_at` | `auth_credentials` | BEFORE UPDATE |
| `update_auth_roles_updated_at` | `auth_roles` | BEFORE UPDATE |

---

### Seed Data

#### Default Tenant

```sql
INSERT INTO auth_tenants (code, name, is_active, settings)
VALUES ('tenant_travel_default', 'Default Travel Tenant', true, '{
  "password_policy": {"min_length": 8, "require_uppercase": true},
  "oauth_providers": ["google"],
  "session_config": {"access_token_ttl": 900, "refresh_token_ttl": 2592000},
  "max_failed_attempts": 5,
  "lockout_duration_minutes": 30
}');
```

#### Default Roles (per tenant)

| Code | Name | is_default | is_system |
|------|------|------------|-----------|
| `super_admin` | Super Administrator | false | true |
| `admin` | Administrator | false | true |
| `agent` | Travel Agent | false | false |
| `traveler` | Traveler | **true** | true |

#### Default Permissions (14 total, global)

| Code | Module | Keterangan |
|------|--------|------------|
| `booking:create` | booking | Buat booking baru |
| `booking:read` | booking | Lihat own bookings |
| `booking:update` | booking | Update own bookings |
| `booking:delete` | booking | Cancel own bookings |
| `booking:read:all` | booking | Lihat semua bookings |
| `user:read` | user | Lihat own profile |
| `user:read:all` | user | Lihat semua user profiles |
| `user:update` | user | Update own profile |
| `user:manage` | user | Manage semua users |
| `payment:process` | payment | Proses pembayaran |
| `payment:refund` | payment | Issue refunds |
| `admin:settings` | admin | Manage settings |
| `admin:roles` | admin | Manage roles & permissions |
| `tenant:manage` | admin | Manage tenant configuration |

#### Role-Permission Matrix

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

## Redis

### Konfigurasi Koneksi Redis

| Parameter | Env Variable | Default | Keterangan |
|-----------|-------------|---------|------------|
| Host | `REDIS_HOST` | `localhost` | Redis host |
| Port | `REDIS_PORT` | `6379` | Redis port |
| Password | `REDIS_PASSWORD` | (empty) | Redis auth password |
| DB | `REDIS_DB` | `1` | Database number (bukan 0) |

**Connection string:** `localhost:6379`, DB `1`

---

### Key Patterns

---

#### Refresh Token

Menyimpan refresh token data per credential per session.

| Properti | Nilai |
|----------|-------|
| **Pattern** | `rt:{credentialID}:{jti}` |
| **Type** | String (JSON) |
| **TTL** | 2,592,000 detik (30 hari) |

**Contoh key:** `rt:7c9e6679-7425-40de-944b-e07fc1f90ae7:a1b2c3d4-e5f6-7890-abcd-ef1234567890`

**Value (JSON):**
```json
{
  "credential_id": "7c9e6679-7425-40de-944b-e07fc1f90ae7",
  "tenant_id": "550e8400-e29b-41d4-a716-446655440000",
  "token_hash": "sha256-hash-of-raw-token",
  "ip_address": "192.168.1.1",
  "user_agent": "Mozilla/5.0...",
  "created_at": "2026-02-21T12:00:00Z",
  "family_id": "f1a2b3c4-d5e6-7890-abcd-ef1234567890"
}
```

**Operasi:**

| Method | Operasi Redis | Keterangan |
|--------|---------------|------------|
| `StoreRefreshToken` | `SET` + TTL | Simpan token baru |
| `GetRefreshToken` | `GET` | Ambil token by credential + jti |
| `DeleteRefreshToken` | `DEL` | Hapus satu token |
| `DeleteAllRefreshTokens` | `SCAN` rt:{credID}:* + `DEL` | Hapus semua token user (logout all) |
| `FindRefreshTokenByHash` | `SCAN` rt:* + match hash | Cari token by hash (refresh flow) |
| `DeleteRefreshTokenFamily` | `SCAN` rt:* + match familyID + `DEL` | Hapus token family (rotation detect) |

---

#### Access Token Blacklist

Menyimpan JTI dari access token yang sudah di-revoke (logout).

| Properti | Nilai |
|----------|-------|
| **Pattern** | `blacklist:at:{jti}` |
| **Type** | String |
| **TTL** | 900 detik (15 menit, = access token TTL) |
| **Value** | `"1"` (existence check only) |

**Contoh key:** `blacklist:at:a1b2c3d4-e5f6-7890-abcd-ef1234567890`

**Operasi:**

| Method | Operasi Redis | Keterangan |
|--------|---------------|------------|
| `BlacklistAccessToken` | `SET` + TTL | Blacklist token (logout) |
| `IsAccessTokenBlacklisted` | `EXISTS` | Cek apakah token sudah di-revoke |

> **Catatan:** TTL sama dengan access token lifetime. Setelah access token expired secara natural, entry blacklist otomatis dihapus Redis karena sudah tidak relevan.

---

#### OAuth State

Menyimpan OAuth state parameter untuk CSRF protection dan PKCE.

| Properti | Nilai |
|----------|-------|
| **Pattern** | `oauth_state:{state}` |
| **Type** | String (JSON) |
| **TTL** | ~600 detik (10 menit) |

**Contoh key:** `oauth_state:abc123def456`

**Value (JSON):**
```json
{
  "tenant_id": "550e8400-e29b-41d4-a716-446655440000",
  "code_verifier": "pkce-code-verifier-random-string",
  "created_at": "2026-02-21T12:00:00Z"
}
```

**Operasi:**

| Method | Operasi Redis | Keterangan |
|--------|---------------|------------|
| `StoreOAuthState` | `SET` + TTL | Simpan state saat initiate OAuth |
| `GetOAuthState` | `GET` | Ambil state saat callback |
| `DeleteOAuthState` | `DEL` | Hapus setelah digunakan (one-time) |

---

#### Rate Limiting

Counter untuk rate limiting login dan password reset.

| Properti | Nilai |
|----------|-------|
| **Pattern** | Arbitrary key (caller provides) |
| **Type** | String (integer counter) |
| **TTL** | Varies per endpoint |

**Key patterns yang digunakan:**

| Context | Key Pattern | TTL | Limit |
|---------|-------------|-----|-------|
| Login | `login:{email}:{tenant_id}` | 900s (15 menit) | 5 request |
| Password Reset | `reset:{email}:{tenant_id}` | 3600s (1 jam) | 3 request |

**Operasi:**

| Method | Operasi Redis | Keterangan |
|--------|---------------|------------|
| `IncrementRateLimit` | Lua script (INCR + EXPIRE) | Atomic increment + set TTL on first hit |
| `GetRateLimit` | `GET` | Ambil current count |

---

#### RBAC Cache

Cache untuk role dan permissions user, menghindari JOIN query ke database setiap request.

| Properti | Nilai |
|----------|-------|
| **Pattern** | `cache:rbac:{credentialID}` |
| **Type** | String (JSON) |
| **TTL** | Configurable (1-2 jam) |

**Contoh key:** `cache:rbac:7c9e6679-7425-40de-944b-e07fc1f90ae7`

**Value (JSON):**
```json
{
  "role": "admin",
  "permissions": [
    "booking:create",
    "booking:read",
    "booking:update",
    "booking:delete",
    "booking:read:all",
    "user:read",
    "user:read:all",
    "user:manage",
    "payment:process",
    "payment:refund",
    "admin:settings",
    "admin:roles"
  ]
}
```

**Operasi:**

| Method | Operasi Redis | Keterangan |
|--------|---------------|------------|
| `StoreRBACCache` | `SET` + TTL | Cache role + permissions |
| `GetRBACCache` | `GET` | Ambil cached role + permissions |

---

### Lua Scripts

#### IncrementRateLimit

Atomic increment dengan auto-expire pada counter pertama:

```lua
local current = redis.call('INCR', KEYS[1])
if current == 1 then
  redis.call('EXPIRE', KEYS[1], ARGV[1])
end
return current
```

**Kenapa Lua script?** Untuk menjamin atomicity — `INCR` dan `EXPIRE` harus dijalankan dalam satu operasi. Tanpa ini, ada race condition dimana key bisa di-increment tanpa pernah di-set expire-nya.

---

## Migrations

File migrasi terletak di `migrations/` dan dijalankan menggunakan `golang-migrate`.

```bash
# Jalankan semua migrasi
make migrate-up

# Rollback migrasi terakhir
make migrate-down
```

### Urutan Migrasi

| # | File | Tabel/Operasi |
|---|------|---------------|
| 1 | `000001_*` | `auth_tenants` + trigger + seed default tenant |
| 2 | `000002_*` | `auth_credentials` + constraints + indexes |
| 3 | `000003_*` | `auth_roles` + `auth_permissions` + `auth_role_permissions` + seed |
| 4 | `000004_*` | `auth_role_assignments` |
| 5 | `000005_*` | `auth_login_history` |
| 6 | `000006_*` | `auth_password_resets` + `auth_email_verifications` |

### Dependency Order

```
auth_tenants
├── auth_credentials (tenant_id FK)
│   ├── auth_role_assignments (credential_id FK)
│   ├── auth_login_history (credential_id FK)
│   ├── auth_password_resets (credential_id FK)
│   └── auth_email_verifications (credential_id FK)
├── auth_roles (tenant_id FK)
│   ├── auth_role_permissions (role_id FK)
│   └── auth_role_assignments (role_id FK)
└── auth_role_assignments (tenant_id FK)

auth_permissions (standalone, no FK to tenants)
└── auth_role_permissions (permission_id FK)
```

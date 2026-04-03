# Security Reference

Dokumentasi lengkap mekanisme keamanan pada TravApp Auth Service.

## Daftar Isi

- [Overview](#overview)
- [Password Security](#password-security)
  - [Hashing (BCrypt)](#hashing-bcrypt)
  - [Password Policy](#password-policy)
  - [Password Change & Reset Side Effects](#password-change--reset-side-effects)
- [JWT (JSON Web Token)](#jwt-json-web-token)
  - [Signing Algorithm](#signing-algorithm)
  - [RSA Key Management](#rsa-key-management)
  - [Access Token Claims](#access-token-claims)
  - [Token Validation](#token-validation)
  - [Algorithm Substitution Protection](#algorithm-substitution-protection)
  - [JWKS Endpoint](#jwks-endpoint)
- [Token Lifecycle & Revocation](#token-lifecycle--revocation)
  - [Access Token](#access-token)
  - [Refresh Token](#refresh-token)
  - [Token Blacklisting](#token-blacklisting)
  - [Refresh Token Rotation](#refresh-token-rotation)
- [OAuth 2.0 + PKCE](#oauth-20--pkce)
  - [PKCE Flow](#pkce-flow)
  - [State Parameter (Anti-CSRF)](#state-parameter-anti-csrf)
  - [ID Token Verification](#id-token-verification)
- [Rate Limiting](#rate-limiting)
- [Account Lockout](#account-lockout)
- [Input Validation](#input-validation)
- [Error Handling & Information Disclosure](#error-handling--information-disclosure)
  - [Error Sanitization](#error-sanitization)
  - [Anti-Enumeration](#anti-enumeration)
- [Sensitive Data Handling](#sensitive-data-handling)
- [Auth Middleware](#auth-middleware)
- [CORS](#cors)
- [Request ID Tracking](#request-id-tracking)
- [Multi-Tenant Security](#multi-tenant-security)
- [Cryptographic Primitives](#cryptographic-primitives)
- [Environment Variables](#environment-variables)
- [Known Gaps & Recommendations](#known-gaps--recommendations)

---

## Overview

| Aspek | Implementasi |
|-------|-------------|
| Password hashing | BCrypt (cost 12) |
| JWT signing | RS256 (RSA + SHA-256) |
| Token storage | Redis (hash, bukan plaintext) |
| OAuth | Google OAuth 2.0 + PKCE (S256) |
| Rate limiting | Redis counter + Lua script (atomic) |
| Account lockout | 5 failed attempts → 30 menit lock |
| Input validation | Gin binding tags |
| Error handling | AppError abstraction, sanitized responses |
| CORS | Configurable allowed origins |
| Randomness | `crypto/rand` (CSPRNG) |

---

## Password Security

### Hashing (BCrypt)

| Properti | Nilai |
|----------|-------|
| Algorithm | BCrypt (`golang.org/x/crypto/bcrypt`) |
| Cost factor | **12** (configurable via `BCRYPT_COST`) |
| Output | 60-char hash string |
| Verification | `bcrypt.CompareHashAndPassword()` (constant-time) |

**Dimana password di-hash:**

| Operasi | File |
|---------|------|
| Register | `register_usecase.go` → `hasher.Hash(input.Password)` |
| Change password | `password_usecase.go` → `hasher.Hash(input.NewPassword)` |
| Reset password | `password_usecase.go` → `hasher.Hash(input.NewPassword)` |

**Security properties:**
- Bcrypt menggunakan built-in salt (random per hash)
- Constant-time comparison mencegah timing attacks
- Cost 12 ≈ ~250ms per hash (cukup lambat untuk brute-force resistance)

### Password Policy

| Rule | Validasi | Enforcement |
|------|----------|-------------|
| Minimum length | 8 karakter | Gin binding `min=8` |
| Confirmation match | Harus sama | Gin binding `eqfield=Password` |
| Format email | Valid email | Gin binding `email` |

> **Catatan:** Tenant settings mendukung `password_policy.require_uppercase` tapi enforcement saat ini hanya via minimum length.

### Password Change & Reset Side Effects

Saat password diubah (change atau reset):

```
Password Changed/Reset
├── Hash password baru disimpan ke database
├── Semua refresh token dihapus dari Redis
│   └── User di-force logout di semua device
├── Access token yang sudah di-issue tetap valid
│   └── Sampai expired (max 15 menit)
└── Login history dicatat
```

---

## JWT (JSON Web Token)

### Signing Algorithm

| Properti | Nilai |
|----------|-------|
| Algorithm | **RS256** (RSA Signature with SHA-256) |
| Key type | Asymmetric (private/public key pair) |
| Key size | 4096-bit RSA (via `make generate-keys`) |
| Library | `github.com/golang-jwt/jwt/v5` |

**Kenapa RS256 (asymmetric)?**
- Private key hanya di auth service (untuk sign)
- Public key di-share via JWKS endpoint (untuk verify)
- Service lain bisa verify JWT **tanpa** shared secret
- Lebih aman dari HS256 untuk microservice architecture

### RSA Key Management

**Key loading at startup:**

```
1. Baca file dari path (JWT_PRIVATE_KEY_PATH, JWT_PUBLIC_KEY_PATH)
2. PEM decode
3. Parse PKCS1 (fallback ke PKCS8 untuk private key)
4. Type check: harus *rsa.PrivateKey / *rsa.PublicKey
5. Error jika format tidak valid → service gagal start
```

**Key ID (kid):**

| Properti | Nilai |
|----------|-------|
| Current kid | `"auth-key-1"` (hardcoded) |
| Di JWT header | Ya |
| Di JWKS response | Ya |

**Key rotation:**
- Saat ini: single static key, kid = `"auth-key-1"`
- Untuk rotate: deploy key baru dengan kid berbeda, maintain old key untuk verify existing tokens
- JWKS endpoint perlu diupdate untuk return multiple keys

**File locations (default):**
```
keys/
├── private.pem    ← RSA private key (sign JWT)
└── public.pem     ← RSA public key (verify JWT)
```

**Generate keys:**
```bash
make generate-keys
# Generates RSA 4096-bit key pair
```

### Access Token Claims

```json
{
  "sub": "7c9e6679-7425-40de-944b-e07fc1f90ae7",
  "tenant_id": "550e8400-e29b-41d4-a716-446655440000",
  "role": "traveler",
  "permissions": ["booking:create", "booking:read", "..."],
  "provider": "email",
  "token_type": "access",
  "jti": "unique-token-id",
  "iat": 1700000000,
  "exp": 1700000900
}
```

| Claim | Tipe | Sumber | Keterangan |
|-------|------|--------|------------|
| `sub` | UUID | Credential ID | User identifier |
| `tenant_id` | UUID | Credential | Tenant scope |
| `role` | string | Role assignment | Role code |
| `permissions` | string[] | Role permissions | Permission codes |
| `provider` | string | Credential | `"email"` atau `"google"` |
| `token_type` | string | Hardcoded | Selalu `"access"` |
| `jti` | UUID | Generated per token | Unique token ID (untuk blacklisting) |
| `iat` | int | Generated | Issued at (Unix timestamp) |
| `exp` | int | iat + TTL | Expires at (Unix timestamp) |

### Token Validation

Validasi dilakukan di auth middleware dan di `ValidateAccessToken()`:

```
1. Parse JWT dengan claims struct
2. Validate signing method = RSA (anti algorithm substitution)
3. Verify signature menggunakan public key
4. Check exp (not expired)
5. Check blacklist di Redis (jti)
6. Extract claims ke Gin context
```

### Algorithm Substitution Protection

```go
// jwt.go - ValidateAccessToken
if _, ok := token.Method.(*jwt.SigningMethodRSA); !ok {
    return nil, fmt.Errorf("unexpected signing method: %v", token.Header["alg"])
}
```

Mencegah:
- **Algorithm "none" attack**: Attacker mengirim JWT tanpa signature
- **HS256 substitution**: Attacker mengubah alg ke HS256 dan sign dengan public key
- Hanya menerima RSA signing methods

### JWKS Endpoint

```
GET /api/v1/auth/.well-known/jwks.json
```

Response:
```json
{
  "keys": [{
    "kty": "RSA",
    "use": "sig",
    "kid": "auth-key-1",
    "alg": "RS256",
    "n": "<base64url-modulus>",
    "e": "AQAB"
  }]
}
```

- Public key dalam format standard JWKS
- Service lain fetch endpoint ini untuk verify JWT
- Rekomendasi: cache public key, refresh periodik (misal setiap 1 jam)

---

## Token Lifecycle & Revocation

### Access Token

```
┌────────────────────────────────────────────────────────┐
│                   Access Token                         │
│                                                        │
│  Created: Login / OAuth callback / Token refresh       │
│  Format: JWT (RS256 signed)                            │
│  TTL: 15 menit (900 detik)                             │
│  Storage: Client-side only (JANGAN di localStorage)    │
│                                                        │
│  Revocation:                                           │
│  ├── Blacklist di Redis pada logout                    │
│  │   └── Key: blacklist:at:{jti}                       │
│  │   └── TTL: sisa lifetime token                      │
│  └── Expired secara natural setelah 15 menit           │
│                                                        │
│  ⚠️ Password change/reset: access token TIDAK          │
│     di-blacklist, masih valid sampai expired            │
└────────────────────────────────────────────────────────┘
```

### Refresh Token

```
┌────────────────────────────────────────────────────────┐
│                   Refresh Token                        │
│                                                        │
│  Created: Login / OAuth callback / Token refresh       │
│  Format: Random hex (64 chars, 32 bytes)               │
│  TTL: 30 hari (2,592,000 detik)                        │
│  Storage: Redis (sebagai SHA256 hash)                  │
│                                                        │
│  Key: rt:{credentialID}:{jti}                          │
│                                                        │
│  Revocation:                                           │
│  ├── Delete satu: logout (jika refresh_token dikirim)  │
│  ├── Delete semua: logout_all / password change/reset  │
│  └── Expired secara natural setelah 30 hari            │
│                                                        │
│  ⚠️ One-time use: setelah dipakai, token lama invalid  │
└────────────────────────────────────────────────────────┘
```

### Token Blacklisting

**Mekanisme:**

| Aksi | Access Token | Refresh Token |
|------|:------------:|:-------------:|
| `POST /logout` | Blacklisted (Redis) | Deleted (jika dikirim) |
| `POST /logout/all` | Blacklisted (current) | Semua deleted |
| Password change | Tidak di-blacklist | Semua deleted |
| Password reset | Tidak di-blacklist | Semua deleted |

**Blacklist flow:**
```
1. Logout dipanggil
2. Extract jti dari JWT claims
3. Hitung remaining TTL: token.exp - now
4. SET blacklist:at:{jti} = "1" EX {remaining_ttl}
5. Auth middleware cek EXISTS blacklist:at:{jti} setiap request
6. Jika ada → 401 "Token has been revoked"
7. Redis auto-delete setelah TTL (token sudah expired anyway)
```

### Refresh Token Rotation

Refresh token bersifat **one-time use** dengan family tracking:

```
Login
└── Refresh Token A (family: F1)
    └── Refresh → Token A deleted, Token B created (family: F1)
        └── Refresh → Token B deleted, Token C created (family: F1)

Jika Token A digunakan lagi (reuse detected):
└── Warning logged (potential token theft)
└── Return 401 "Invalid refresh token"
```

**Family-based rotation:**
- Setiap refresh token punya `family_id`
- `DeleteRefreshTokenFamily()` bisa menghapus semua token dalam satu family
- Melindungi dari replay attack saat token dicuri

---

## OAuth 2.0 + PKCE

### PKCE Flow

```
Client              Auth Service              Google
  │                      │                      │
  │  GET /oauth/google   │                      │
  │  ?tenant_id=xxx      │                      │
  │─────────────────────>│                      │
  │                      │                      │
  │                      │  Generate:           │
  │                      │  ├── state (64 hex)  │
  │                      │  ├── code_verifier   │
  │                      │  │   (32 bytes b64)  │
  │                      │  └── code_challenge  │
  │                      │      SHA256(verifier) │
  │                      │                      │
  │                      │  Store in Redis:     │
  │                      │  oauth_state:{state} │
  │                      │  {tenant_id,         │
  │                      │   code_verifier}     │
  │                      │  TTL: 10 menit       │
  │                      │                      │
  │  302 Redirect        │                      │
  │  ?code_challenge=xxx │                      │
  │  &state=xxx          │                      │
  │<─────────────────────│                      │
  │                      │                      │
  │  User consents ──────────────────────────── >│
  │                      │                      │
  │  Callback            │                      │
  │  ?code=xxx&state=xxx │                      │
  │─────────────────────>│                      │
  │                      │  Validate state      │
  │                      │  (Redis lookup)      │
  │                      │  Delete state (1x)   │
  │                      │                      │
  │                      │  Exchange code +     │
  │                      │  code_verifier ──────>│
  │                      │                      │
  │                      │  <── tokens ─────────│
  │                      │                      │
  │  200 {tokens}        │  Verify ID token     │
  │<─────────────────────│                      │
```

**PKCE parameters:**

| Parameter | Nilai | Keterangan |
|-----------|-------|------------|
| `code_challenge_method` | `S256` | SHA-256 hashing |
| `code_verifier` | 43 chars (32 bytes base64url) | Random, stored in Redis |
| `code_challenge` | 43 chars (SHA256 + base64url) | Sent to Google |
| `access_type` | `offline` | Request refresh token dari Google |
| `prompt` | `consent` | Force consent screen |

**Kenapa PKCE?**
- Mencegah authorization code interception attack
- Code verifier disimpan server-side (Redis), bukan di client
- Bahkan jika `code` dicuri, attacker tidak punya `code_verifier`

### State Parameter (Anti-CSRF)

| Properti | Nilai |
|----------|-------|
| Length | 64 hex characters (32 bytes random) |
| Storage | Redis key `oauth_state:{state}` |
| TTL | 10 menit |
| Usage | One-time (deleted setelah callback) |

**Proteksi:**
- Mencegah CSRF attack pada OAuth callback
- State di-validate sebelum code exchange
- State expired setelah 10 menit → mencegah replay

### ID Token Verification

Setelah code exchange, Google mengembalikan ID token yang di-parse:

| Check | Dilakukan | Keterangan |
|-------|:---------:|------------|
| Audience = Client ID | Ya | Mencegah token dari app lain |
| Issuer = accounts.google.com | Ya | Memastikan dari Google |
| email_verified | Ya (dibaca) | Auto-set pada credential |
| Signature verification | **Tidak** | Lihat catatan di bawah |

> **Catatan:** ID token di-parse tanpa signature verification (`jwt.WithoutClaimsValidation()`). Ini acceptable karena token diterima langsung dari Google token endpoint via HTTPS — channel sudah authenticated. Namun, untuk defense-in-depth, signature verification via Google JWKS bisa ditambahkan.

---

## Rate Limiting

### Implementasi

Rate limiting menggunakan Redis counter dengan Lua script untuk atomicity:

```lua
local current = redis.call('INCR', KEYS[1])
if current == 1 then
    redis.call('EXPIRE', KEYS[1], ARGV[1])
end
return current
```

**Kenapa Lua script?** INCR + EXPIRE harus atomic. Tanpa Lua, ada race condition dimana key bisa increment tanpa expire.

### Rate Limit Rules

| Endpoint | Key Pattern | Max Attempts | Window | Error |
|----------|-------------|:------------:|--------|-------|
| `POST /login` | `rate:login:{email}:{tenant_id}` | **5** | 15 menit | 429 Too Many Requests |
| `POST /password/forgot` | `rate:reset:{email}:{tenant_id}` | **3** | 1 jam | 429 Too Many Requests |

**Penting:**
- Rate limit di-scope per `email + tenant_id`
- User yang sama di tenant berbeda punya counter independen
- Counter auto-reset setelah window expire (Redis TTL)

### Konfigurasi

| Env Variable | Default | Keterangan |
|-------------|---------|------------|
| `RATE_LIMIT_LOGIN_MAX` | 5 | Max login attempts |
| `RATE_LIMIT_LOGIN_WINDOW` | 900 | Login window (detik) |
| `RATE_LIMIT_RESET_MAX` | 3 | Max reset requests |
| `RATE_LIMIT_RESET_WINDOW` | 3600 | Reset window (detik) |

---

## Account Lockout

### Mekanisme

```
Login attempt gagal
│
├── failed_login_attempts += 1
│
├── Jika attempts < 5:
│   └── Return 401 "Invalid email or password"
│
└── Jika attempts >= 5:
    ├── locked_until = NOW() + 30 menit
    ├── Return 423 "Account is temporarily locked..."
    └── Login ditolak sampai locked_until lewat
```

### Detail

| Properti | Nilai |
|----------|-------|
| Threshold | **5** failed attempts |
| Lock duration | **30 menit** |
| Scope | Per credential (per email + tenant) |
| Storage | `auth_credentials.failed_login_attempts` + `locked_until` |
| Reset condition | Successful login → counter = 0, locked_until = NULL |

### Flow

```
Attempt 1: Wrong password → attempts=1, no lock
Attempt 2: Wrong password → attempts=2, no lock
Attempt 3: Wrong password → attempts=3, no lock
Attempt 4: Wrong password → attempts=4, no lock
Attempt 5: Wrong password → attempts=5, LOCKED 30 min
           ↓
Attempt 6: (within 30 min) → 423 Locked (password not checked)
           ↓
(30 menit berlalu)
           ↓
Attempt 7: Correct password → attempts=0, locked_until=NULL, LOGIN OK
```

> **Catatan:** Lockout **tidak** otomatis di-reset setelah 30 menit. Counter tetap di 5. Saat lock period berakhir, login attempt berikutnya harus berhasil untuk reset counter, atau akan langsung lock lagi jika gagal.

---

## Input Validation

Semua input divalidasi menggunakan Gin binding tags sebelum masuk ke usecase layer.

### Validation Rules per Endpoint

**POST /login:**

| Field | Rules | Keterangan |
|-------|-------|------------|
| `email` | `required,email` | Format email valid |
| `password` | `required,min=8` | Minimum 8 karakter |
| `tenant_id` | `required,uuid` | Valid UUID format |

**POST /register:**

| Field | Rules | Keterangan |
|-------|-------|------------|
| `email` | `required,email` | Format email valid |
| `password` | `required,min=8` | Minimum 8 karakter |
| `password_confirmation` | `required,eqfield=Password` | Harus sama |
| `tenant_id` | `required,uuid` | Valid UUID format |

**POST /token/refresh:**

| Field | Rules | Keterangan |
|-------|-------|------------|
| `refresh_token` | `required` | Wajib ada |
| `tenant_id` | `required,uuid` | Valid UUID format |

**POST /password/change:**

| Field | Rules | Keterangan |
|-------|-------|------------|
| `current_password` | `required` | Wajib ada |
| `new_password` | `required,min=8` | Minimum 8 karakter |
| `new_password_confirmation` | `required,eqfield=NewPassword` | Harus sama |

**POST /password/forgot:**

| Field | Rules | Keterangan |
|-------|-------|------------|
| `email` | `required,email` | Format email valid |
| `tenant_id` | `required,uuid` | Valid UUID format |

**POST /password/reset:**

| Field | Rules | Keterangan |
|-------|-------|------------|
| `token` | `required` | Wajib ada |
| `email` | `required,email` | Format email valid |
| `new_password` | `required,min=8` | Minimum 8 karakter |
| `new_password_confirmation` | `required,eqfield=NewPassword` | Harus sama |

**POST /logout:**

| Field | Rules | Keterangan |
|-------|-------|------------|
| `refresh_token` | (optional) | Opsional, jika dikirim akan dihapus dari Redis |

### Validation Error Response

```json
{
  "error": "Invalid request: Key: 'LoginRequest.Email' Error:Field validation for 'Email' failed on the 'email' tag",
  "code": 400,
  "request_id": "..."
}
```

---

## Error Handling & Information Disclosure

### Error Sanitization

Semua error di-wrap dalam `AppError` sebelum dikembalikan ke client:

```
┌────────────────────┐     ┌──────────────┐     ┌───────────────┐
│  Internal Error    │────>│   AppError   │────>│  HTTP Response│
│  (detail teknis)   │     │  (sanitized) │     │  (user-safe)  │
│                    │     │              │     │               │
│  "pq: duplicate    │     │  Code: 409   │     │  {            │
│   key violates..." │     │  Msg: "Email │     │    "error":   │
│                    │     │   already    │     │    "Email..." │
│                    │     │   exists"    │     │    "code": 409│
│                    │     │  Err: (hidden│     │  }            │
│                    │     │   from JSON) │     │               │
└────────────────────┘     └──────────────┘     └───────────────┘
```

**AppError JSON serialization:**
```go
type AppError struct {
    Code    int    `json:"code"`
    Message string `json:"message"`
    Err     error  `json:"-"`     // ← TIDAK di-serialize ke JSON
}
```

- Internal error (`Err`) di-tag `json:"-"` → tidak pernah di-expose ke client
- Hanya `Code` dan `Message` yang dikirim
- Unknown errors → generic 500 "Internal server error"

### Error Types

| Function | HTTP Code | Kapan Digunakan |
|----------|:---------:|-----------------|
| `NewBadRequest` | 400 | Validasi gagal, input tidak valid |
| `NewUnauthorized` | 401 | Credentials salah, token invalid |
| `NewForbidden` | 403 | Tidak punya akses |
| `NewNotFound` | 404 | Resource tidak ditemukan |
| `NewConflict` | 409 | Data duplikat (email exists) |
| `NewLocked` | 423 | Account terkunci |
| `NewTooManyRequests` | 429 | Rate limit terlampaui |
| `NewInternal` | 500 | Error internal (pesan generic) |
| `NewInternalWithMessage` | 500 | Error internal (pesan custom) |

### Anti-Enumeration

**Forgot Password (`POST /password/forgot`):**

```
Email ditemukan    → 200 "If an account with that email exists..."
Email TIDAK ada    → 200 "If an account with that email exists..."
                         (pesan SAMA persis)
```

- Response selalu 200 dengan pesan identical
- Attacker tidak bisa membedakan email yang terdaftar vs tidak
- Mencegah email enumeration attack

**Login (`POST /login`):**

```
Email tidak ada    → 401 "Invalid email or password"
Password salah     → 401 "Invalid email or password"
                         (pesan SAMA persis)
```

- Pesan error sama untuk email salah dan password salah
- Attacker tidak bisa membedakan mana yang salah

---

## Sensitive Data Handling

### Data yang Tidak Di-expose

| Data | Di Response | Di Log | Di Redis |
|------|:-----------:|:------:|:--------:|
| Password (plaintext) | Tidak | Tidak | Tidak |
| Password hash | Tidak | Tidak | Tidak |
| Provider ID | Tidak | Tidak | Tidak |
| Refresh token (raw) | Ya (saat login) | Tidak (hanya prefix) | Sebagai SHA256 hash |
| Access token (raw) | Ya (saat login) | Tidak | Tidak pernah disimpan |
| RSA private key | Tidak | Tidak | Tidak |

### Token Storage Security

```
Raw refresh token     →  SHA256 hash  →  Stored in Redis
(dikirim ke client)      (server-side)   (rt:{id}:{jti})

Jika Redis compromised:
├── Attacker dapat SHA256 hash
├── Tidak bisa reverse ke raw token (one-way hash)
└── Tidak bisa menggunakan hash untuk refresh
```

### Credential Entity JSON Tags

```go
type AuthCredential struct {
    PasswordHash  *string  `json:"-"`   // Never serialized
    ProviderID    *string  `json:"-"`   // Never serialized
}
```

---

## Auth Middleware

Middleware untuk endpoint yang memerlukan authentication:

```
Request masuk
│
├── 1. Extract Authorization header
│   └── Tidak ada → 401 "Missing authorization header"
│
├── 2. Parse "Bearer {token}" format
│   └── Format salah → 401 "Missing authorization header"
│
├── 3. Validate JWT (signature, expiry, claims)
│   └── Invalid → 401 "Invalid or expired token"
│
├── 4. Check blacklist di Redis (by jti)
│   └── Blacklisted → 401 "Token has been revoked"
│
├── 5. Set claims ke Gin context:
│   ├── credential_id
│   ├── tenant_id
│   ├── role
│   ├── permissions
│   ├── provider
│   ├── jti
│   └── token_exp
│
└── 6. Next handler
```

**Protected endpoints:**

| Endpoint | Keterangan |
|----------|------------|
| `POST /api/v1/auth/logout` | Logout session saat ini |
| `POST /api/v1/auth/logout/all` | Logout semua session |
| `POST /api/v1/auth/password/change` | Ubah password |

---

## CORS

### Konfigurasi

| Header | Nilai |
|--------|-------|
| `Access-Control-Allow-Methods` | `GET, POST, PUT, PATCH, DELETE, OPTIONS` |
| `Access-Control-Allow-Headers` | `Origin, Content-Type, Accept, Authorization, X-Request-ID` |
| `Access-Control-Allow-Credentials` | `true` |
| `Access-Control-Max-Age` | `86400` (1 hari) |

**Origin matching:**
- Support wildcard `"*"` (allow all origins)
- Exact match untuk specific origins
- Configurable via `CORS_ALLOWED_ORIGINS` env var

**Preflight handling:**
- `OPTIONS` request → HTTP 204 No Content (tanpa body)

---

## Request ID Tracking

Setiap request memiliki unique ID untuk tracing dan debugging:

```
1. Cek header X-Request-ID dari client
2. Jika ada → gunakan sebagai request ID
3. Jika tidak ada → generate UUID baru
4. Set di Gin context: c.Set("request_id", id)
5. Set di response header: X-Request-ID
6. Include di error responses: "request_id" field
```

**Berguna untuk:**
- Correlate logs across services
- Debug specific request failures
- Client-side error reporting

---

## Multi-Tenant Security

### Isolation Points

| Layer | Mekanisme | Detail |
|-------|-----------|--------|
| Database | `WHERE tenant_id = $1` | Semua query credential/role filtered |
| Database | Unique constraints | `(email, provider, tenant_id)` |
| Redis rate limit | Key includes tenant_id | `rate:login:{email}:{tenant_id}` |
| JWT | `tenant_id` di claims | Embedded saat sign, validated saat verify |
| Refresh token | `tenant_id` di Redis value | Validated saat refresh |

### Tenant Validation

Setiap request yang memerlukan tenant:
```
1. Validate UUID format (Gin binding `uuid`)
2. GetActiveTenant(tenant_id)
   ├── Tenant tidak ada → 400 "Invalid or inactive tenant"
   └── Tenant is_active=false → 400 "Invalid or inactive tenant"
```

### Cross-Tenant Protection

```
✅ user@example.com login di Tenant A → OK
✅ user@example.com login di Tenant B → OK (credential berbeda)
❌ Refresh token dari Tenant A dipakai di Tenant B → 401
❌ JWT dari Tenant A verify resource di Tenant B → Claims mismatch
```

Lihat [MULTITENANCY.md](./MULTITENANCY.md) untuk detail lengkap.

---

## Cryptographic Primitives

Semua randomness menggunakan `crypto/rand` (CSPRNG — Cryptographically Secure Pseudo-Random Number Generator).

| Fungsi | Input | Output | Digunakan Untuk |
|--------|-------|--------|-----------------|
| `GenerateRandomHex(32)` | 32 bytes random | 64 hex chars | Refresh token, reset token, verification token, OAuth state |
| `GenerateCodeVerifier()` | 32 bytes random | ~43 chars (base64url) | PKCE code verifier |
| `GenerateCodeChallenge(v)` | code verifier string | 43 chars (SHA256 + base64url) | PKCE code challenge |
| `SHA256Hash(data)` | arbitrary string | 64 hex chars | Token hashing sebelum storage |

**Security properties:**
- `crypto/rand` menggunakan OS entropy source (`/dev/urandom` di Linux, `CryptGenRandom` di Windows)
- Tidak predictable, tidak bisa di-seed
- Aman untuk semua cryptographic use cases

---

## Environment Variables

Semua security-related configuration via environment variables:

### JWT & Keys

| Variable | Default | Keterangan |
|----------|---------|------------|
| `JWT_PRIVATE_KEY_PATH` | - | Path ke RSA private key (required) |
| `JWT_PUBLIC_KEY_PATH` | - | Path ke RSA public key (required) |
| `JWT_ACCESS_TOKEN_TTL` | `900` | Access token lifetime (detik) |
| `JWT_REFRESH_TOKEN_TTL` | `2592000` | Refresh token lifetime (detik) |

### Password

| Variable | Default | Keterangan |
|----------|---------|------------|
| `BCRYPT_COST` | `12` | BCrypt cost factor |

### Rate Limiting

| Variable | Default | Keterangan |
|----------|---------|------------|
| `RATE_LIMIT_LOGIN_MAX` | `5` | Max login attempts per window |
| `RATE_LIMIT_LOGIN_WINDOW` | `900` | Login rate limit window (detik) |
| `RATE_LIMIT_RESET_MAX` | `3` | Max password reset per window |
| `RATE_LIMIT_RESET_WINDOW` | `3600` | Reset rate limit window (detik) |

### OAuth

| Variable | Default | Keterangan |
|----------|---------|------------|
| `GOOGLE_CLIENT_ID` | - | Google OAuth client ID (required) |
| `GOOGLE_CLIENT_SECRET` | - | Google OAuth client secret (required) |
| `GOOGLE_REDIRECT_URI` | - | OAuth callback URL (required) |

### Infrastructure

| Variable | Default | Keterangan |
|----------|---------|------------|
| `DB_SSL_MODE` | `disable` | PostgreSQL SSL mode |
| `CORS_ALLOWED_ORIGINS` | - | Comma-separated allowed origins |
| `APP_ENV` | `development` | `development` atau `production` |

### Production Recommendations

```env
# Minimum security settings untuk production
APP_ENV=production
DB_SSL_MODE=require
BCRYPT_COST=12
JWT_ACCESS_TOKEN_TTL=900
JWT_REFRESH_TOKEN_TTL=2592000
RATE_LIMIT_LOGIN_MAX=5
CORS_ALLOWED_ORIGINS=https://app.example.com
```

---

## Known Gaps & Recommendations

### Gap 1: Email Sending Belum Diimplementasi

| Status | Detail |
|--------|--------|
| **Impact** | Medium-High |
| **Affected** | Email verification, password reset |
| **Current** | Token di-generate dan di-log ke console, tapi TIDAK dikirim via email |
| **Risiko** | User tidak bisa verify email atau reset password di production |
| **Rekomendasi** | Implement SMTP service atau gunakan third-party (SendGrid, AWS SES) |

### Gap 2: Email Verification Tidak Tersimpan saat Register

| Status | Detail |
|--------|--------|
| **Impact** | High |
| **Affected** | Register flow |
| **Current** | `register_usecase.go` generate verification token tapi TIDAK simpan ke `auth_email_verifications` table |
| **Risiko** | VerifyEmail endpoint tidak berfungsi (no data to consume) |
| **Rekomendasi** | Tambahkan `pool.CreateEmailVerification()` call di register usecase |

### Gap 3: Google ID Token Signature Tidak Diverifikasi

| Status | Detail |
|--------|--------|
| **Impact** | Low |
| **Current** | ID token di-parse tanpa signature verification |
| **Mitigasi** | Token diterima via HTTPS dari Google endpoint (channel trusted) |
| **Rekomendasi** | Tambahkan signature verification via Google JWKS untuk defense-in-depth |

### Gap 4: Security Headers Tidak Di-set

| Status | Detail |
|--------|--------|
| **Impact** | Low (jika di-handle di API gateway) |
| **Missing** | `X-Frame-Options`, `X-Content-Type-Options`, `Strict-Transport-Security`, `Content-Security-Policy` |
| **Rekomendasi** | Set di API gateway/reverse proxy, atau tambahkan middleware di service |

### Gap 5: Request Body Size Limit

| Status | Detail |
|--------|--------|
| **Impact** | Low (jika di-handle di reverse proxy) |
| **Current** | Tidak ada explicit body size limit |
| **Risiko** | Large payload attack (DoS) |
| **Rekomendasi** | Set `MaxMultipartMemory` di Gin, atau limit di reverse proxy (nginx: `client_max_body_size`) |

### Gap 6: Password Change Tidak Blacklist Access Token

| Status | Detail |
|--------|--------|
| **Impact** | Low |
| **Current** | Password change/reset menghapus refresh token tapi TIDAK blacklist access token |
| **Risiko** | Stolen access token masih valid sampai expired (max 15 menit) |
| **Mitigasi** | TTL access token pendek (15 menit) membatasi exposure window |
| **Rekomendasi** | Blacklist access token saat password change/reset untuk immediate revocation |

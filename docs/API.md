# API Reference

Dokumentasi lengkap seluruh endpoint TravApp Auth Service.

**Base URL:** `http://localhost:8080`
**Content-Type:** `application/json`
**Swagger UI:** `http://localhost:8080/swagger/index.html`

## Daftar Isi

- [Authentication](#authentication)
- [Error Response Format](#error-response-format)
- [Endpoints](#endpoints)
  - [Health Check](#health-check)
  - [Register](#register)
  - [Login](#login)
  - [Google OAuth - Initiate](#google-oauth---initiate)
  - [Google OAuth - Callback](#google-oauth---callback)
  - [Refresh Token](#refresh-token)
  - [JWKS](#jwks)
  - [Verify Email](#verify-email)
  - [Forgot Password](#forgot-password)
  - [Reset Password](#reset-password)
  - [Change Password](#change-password)
  - [Logout](#logout)
  - [Logout All](#logout-all)

---

## Authentication

Endpoint yang ditandai **Auth Required** memerlukan header:

```
Authorization: Bearer <access_token>
```

Access token didapat dari response login, Google OAuth callback, atau token refresh.

---

## Error Response Format

Semua error menggunakan format yang sama:

```json
{
  "error": "Pesan error yang user-friendly",
  "code": 401,
  "request_id": "550e8400-e29b-41d4-a716-446655440000"
}
```

| Field | Tipe | Keterangan |
|-------|------|------------|
| `error` | string | Pesan error untuk ditampilkan ke user |
| `code` | int | HTTP status code |
| `request_id` | string | Request ID untuk debugging (opsional) |

### HTTP Status Codes

| Code | Nama | Keterangan |
|------|------|------------|
| 200 | OK | Request berhasil |
| 201 | Created | Resource berhasil dibuat |
| 302 | Found | Redirect (OAuth) |
| 400 | Bad Request | Input tidak valid |
| 401 | Unauthorized | Credentials salah / token invalid |
| 404 | Not Found | Route tidak ditemukan |
| 409 | Conflict | Data sudah ada (email duplicate) |
| 423 | Locked | Account terkunci |
| 429 | Too Many Requests | Rate limit terlampaui |
| 500 | Internal Server Error | Server error |
| 503 | Service Unavailable | Health check gagal |

---

## Endpoints

---

### Health Check

Cek status service, database, dan Redis.

```
GET /health
```

**Auth Required:** No

**Response (200):**
```json
{
  "status": "healthy",
  "database": true,
  "redis": true
}
```

**Response (503):**
```json
{
  "status": "unhealthy",
  "database": true,
  "redis": false
}
```

---

### Register

Mendaftarkan akun baru dengan email dan password.

```
POST /api/v1/auth/register
```

**Auth Required:** No

**Request Body:**

| Field | Tipe | Required | Validasi |
|-------|------|----------|----------|
| `email` | string | Ya | Format email valid |
| `password` | string | Ya | Minimum 8 karakter |
| `password_confirmation` | string | Ya | Harus sama dengan `password` |
| `tenant_id` | string | Ya | UUID format |

**Contoh Request:**
```json
{
  "email": "user@example.com",
  "password": "MyStr0ng!Pass",
  "password_confirmation": "MyStr0ng!Pass",
  "tenant_id": "550e8400-e29b-41d4-a716-446655440000"
}
```

**Response (201):**
```json
{
  "message": "Registration successful. Please verify your email.",
  "credential_id": "7c9e6679-7425-40de-944b-e07fc1f90ae7"
}
```

**Possible Errors:**

| Code | Error | Keterangan |
|------|-------|------------|
| 400 | Invalid request: ... | Validasi gagal (email format, password terlalu pendek, dll) |
| 400 | Invalid tenant_id | UUID tidak valid |
| 400 | Invalid or inactive tenant | Tenant tidak ditemukan atau tidak aktif |
| 409 | An account with this email already exists | Email sudah terdaftar di tenant ini |

---

### Login

Login dengan email dan password, mendapatkan access token dan refresh token.

```
POST /api/v1/auth/login
```

**Auth Required:** No

**Request Body:**

| Field | Tipe | Required | Validasi |
|-------|------|----------|----------|
| `email` | string | Ya | Format email valid |
| `password` | string | Ya | Minimum 8 karakter |
| `tenant_id` | string | Ya | UUID format |

**Contoh Request:**
```json
{
  "email": "user@example.com",
  "password": "MyStr0ng!Pass",
  "tenant_id": "550e8400-e29b-41d4-a716-446655440000"
}
```

**Response (200):**
```json
{
  "access_token": "eyJhbGciOiJSUzI1NiIsI...",
  "refresh_token": "a1b2c3d4e5f6...",
  "token_type": "Bearer",
  "expires_in": 900
}
```

| Field | Tipe | Keterangan |
|-------|------|------------|
| `access_token` | string | JWT untuk authorization header (TTL: 15 menit) |
| `refresh_token` | string | Token untuk mendapatkan access token baru (TTL: 30 hari) |
| `token_type` | string | Selalu `"Bearer"` |
| `expires_in` | int | Access token lifetime dalam detik |

**Possible Errors:**

| Code | Error | Keterangan |
|------|-------|------------|
| 400 | Invalid request: ... | Validasi gagal |
| 400 | Invalid or inactive tenant | Tenant tidak valid |
| 401 | Invalid email or password | Email/password salah atau user tidak ditemukan |
| 401 | Account is suspended | Account di-nonaktifkan |
| 401 | Please verify your email before logging in | Email belum diverifikasi |
| 423 | Account is temporarily locked... | Terlalu banyak gagal login (5x → lock 30 menit) |
| 429 | Too many login attempts... | Rate limit (5 percobaan per 15 menit) |

---

### Google OAuth - Initiate

Memulai flow Google OAuth2. Redirect user ke Google consent screen.

```
GET /api/v1/auth/oauth/google?tenant_id=<uuid>
```

**Auth Required:** No

**Query Parameters:**

| Parameter | Tipe | Required | Keterangan |
|-----------|------|----------|------------|
| `tenant_id` | string | Ya | UUID tenant |

**Response (302):**

Redirect ke Google OAuth2 consent page:
```
https://accounts.google.com/o/oauth2/auth?
  client_id=xxx&
  redirect_uri=xxx&
  response_type=code&
  scope=openid+email&
  state=xxx&
  code_challenge=xxx&
  code_challenge_method=S256
```

**Possible Errors:**

| Code | Error | Keterangan |
|------|-------|------------|
| 400 | tenant_id query parameter is required | Parameter tidak ada |
| 400 | Invalid tenant_id | UUID format salah |
| 400 | Invalid or inactive tenant | Tenant tidak valid |

---

### Google OAuth - Callback

Callback dari Google setelah user consent. Menukar authorization code menjadi token.

```
GET /api/v1/auth/oauth/google/callback?code=<code>&state=<state>
```

**Auth Required:** No

**Query Parameters:**

| Parameter | Tipe | Required | Keterangan |
|-----------|------|----------|------------|
| `code` | string | Ya | Authorization code dari Google |
| `state` | string | Ya | State parameter untuk CSRF protection |

**Response (200):**
```json
{
  "access_token": "eyJhbGciOiJSUzI1NiIsI...",
  "refresh_token": "a1b2c3d4e5f6...",
  "token_type": "Bearer",
  "expires_in": 900
}
```

Response sama dengan [Login](#login).

**Catatan:**
- Jika user belum punya akun, otomatis di-register (provider="google", email auto-verified)
- Jika user sudah ada, langsung login

**Possible Errors:**

| Code | Error | Keterangan |
|------|-------|------------|
| 400 | Missing code or state parameter | Parameter tidak lengkap |
| 400 | OAuth error: ... | Error dari Google (user deny, dll) |
| 400 | Invalid or expired OAuth state | State kadaluarsa (>10 menit) atau tidak valid |
| 401 | Account is suspended | Account di-nonaktifkan |

---

### Refresh Token

Menukar refresh token lama dengan access token + refresh token baru.

```
POST /api/v1/auth/token/refresh
```

**Auth Required:** No

**Request Body:**

| Field | Tipe | Required | Validasi |
|-------|------|----------|----------|
| `refresh_token` | string | Ya | - |
| `tenant_id` | string | Ya | UUID format |

**Contoh Request:**
```json
{
  "refresh_token": "a1b2c3d4e5f6...",
  "tenant_id": "550e8400-e29b-41d4-a716-446655440000"
}
```

**Response (200):**
```json
{
  "access_token": "eyJhbGciOiJSUzI1NiIsI...",
  "refresh_token": "f6e5d4c3b2a1...",
  "token_type": "Bearer",
  "expires_in": 900
}
```

**Penting:** Refresh token bersifat **one-time use**. Setelah dipakai, token lama tidak valid lagi. Simpan refresh token baru dari response.

**Possible Errors:**

| Code | Error | Keterangan |
|------|-------|------------|
| 400 | Invalid request: ... | Validasi gagal |
| 400 | Invalid tenant_id | UUID format salah |
| 401 | Invalid refresh token | Token tidak valid, sudah dipakai, atau tenant mismatch |
| 401 | Account not found or suspended | Credential tidak aktif |

---

### JWKS

Mendapatkan public key (JSON Web Key Set) untuk verifikasi JWT.

```
GET /api/v1/auth/.well-known/jwks.json
```

**Auth Required:** No

**Response (200):**
```json
{
  "keys": [
    {
      "kty": "RSA",
      "use": "sig",
      "kid": "auth-key-1",
      "alg": "RS256",
      "n": "base64url-encoded-modulus...",
      "e": "AQAB"
    }
  ]
}
```

| Field | Keterangan |
|-------|------------|
| `kty` | Key type (selalu "RSA") |
| `use` | Key usage (selalu "sig" = signature) |
| `kid` | Key ID untuk key rotation |
| `alg` | Algorithm (selalu "RS256") |
| `n` | RSA modulus (base64url) |
| `e` | RSA exponent (base64url) |

**Penggunaan:** Service lain bisa fetch endpoint ini untuk memverifikasi JWT tanpa shared secret.

---

### Verify Email

Memverifikasi email address setelah registrasi.

```
GET /api/v1/auth/verify-email?token=<token>&email=<email>
```

**Auth Required:** No

**Query Parameters:**

| Parameter | Tipe | Required | Keterangan |
|-----------|------|----------|------------|
| `token` | string | Ya | Verification token dari registrasi |
| `email` | string | Ya | Email address yang diverifikasi |

**Response (200):**
```json
{
  "message": "Email verified successfully"
}
```

**Possible Errors:**

| Code | Error | Keterangan |
|------|-------|------------|
| 400 | Missing token or email parameter | Parameter tidak lengkap |
| 400 | Invalid or expired verification token | Token tidak ditemukan |
| 400 | Verification token has expired | Token kadaluarsa (>24 jam) |
| 400 | Email has already been verified | Email sudah terverifikasi |

---

### Forgot Password

Meminta link reset password. Response selalu 200 untuk mencegah email enumeration.

```
POST /api/v1/auth/password/forgot
```

**Auth Required:** No

**Request Body:**

| Field | Tipe | Required | Validasi |
|-------|------|----------|----------|
| `email` | string | Ya | Format email valid |
| `tenant_id` | string | Ya | UUID format |

**Contoh Request:**
```json
{
  "email": "user@example.com",
  "tenant_id": "550e8400-e29b-41d4-a716-446655440000"
}
```

**Response (200):**
```json
{
  "message": "If an account with that email exists, a password reset link has been sent."
}
```

> **Catatan:** Response selalu 200 dan pesan yang sama, baik email ditemukan maupun tidak. Ini untuk mencegah email enumeration attack.

**Possible Errors:**

| Code | Error | Keterangan |
|------|-------|------------|
| 400 | Invalid request: ... | Validasi gagal |
| 400 | Invalid tenant_id | UUID format salah |
| 429 | Too many password reset requests... | Rate limit (3 request per jam) |

---

### Reset Password

Menggunakan reset token untuk mengatur password baru.

```
POST /api/v1/auth/password/reset
```

**Auth Required:** No

**Request Body:**

| Field | Tipe | Required | Validasi |
|-------|------|----------|----------|
| `token` | string | Ya | Reset token dari forgot password |
| `email` | string | Ya | Format email valid |
| `new_password` | string | Ya | Minimum 8 karakter |
| `new_password_confirmation` | string | Ya | Harus sama dengan `new_password` |

**Contoh Request:**
```json
{
  "token": "a1b2c3d4e5f6...",
  "email": "user@example.com",
  "new_password": "NewStr0ng!Pass",
  "new_password_confirmation": "NewStr0ng!Pass"
}
```

**Response (200):**
```json
{
  "message": "Password has been reset successfully"
}
```

**Side effects:**
- Semua refresh token dihapus (force re-login di semua device)

**Possible Errors:**

| Code | Error | Keterangan |
|------|-------|------------|
| 400 | Invalid request: ... | Validasi gagal |
| 400 | Invalid or expired reset token | Token tidak ditemukan |
| 400 | Reset token has expired | Token kadaluarsa (>1 jam) |
| 400 | Reset token has already been used | Token sudah digunakan |

---

### Change Password

Mengubah password user yang sedang login.

```
POST /api/v1/auth/password/change
```

**Auth Required:** Ya

**Request Headers:**
```
Authorization: Bearer <access_token>
Content-Type: application/json
```

**Request Body:**

| Field | Tipe | Required | Validasi |
|-------|------|----------|----------|
| `current_password` | string | Ya | - |
| `new_password` | string | Ya | Minimum 8 karakter |
| `new_password_confirmation` | string | Ya | Harus sama dengan `new_password` |

**Contoh Request:**
```json
{
  "current_password": "OldPass123!",
  "new_password": "NewStr0ng!Pass",
  "new_password_confirmation": "NewStr0ng!Pass"
}
```

**Response (200):**
```json
{
  "message": "Password changed successfully"
}
```

**Side effects:**
- Semua refresh token dihapus (force re-login di semua device)

**Possible Errors:**

| Code | Error | Keterangan |
|------|-------|------------|
| 400 | Invalid request: ... | Validasi gagal |
| 400 | Cannot change password for OAuth accounts | Account Google tidak punya password |
| 401 | Missing authorization header | Header Authorization tidak ada |
| 401 | Invalid or expired token | Token JWT tidak valid |
| 401 | Token has been revoked | Token sudah di-blacklist |
| 401 | Current password is incorrect | Password lama salah |
| 404 | Credential not found | User tidak ditemukan |

---

### Logout

Logout dari session saat ini. Invalidates access token dan opsional refresh token.

```
POST /api/v1/auth/logout
```

**Auth Required:** Ya

**Request Headers:**
```
Authorization: Bearer <access_token>
Content-Type: application/json
```

**Request Body (opsional):**

| Field | Tipe | Required | Keterangan |
|-------|------|----------|------------|
| `refresh_token` | string | Tidak | Jika dikirim, refresh token juga dihapus dari Redis |

**Contoh Request:**
```json
{
  "refresh_token": "a1b2c3d4e5f6..."
}
```

Atau tanpa body (hanya invalidate access token):
```json
{}
```

**Response (200):**
```json
{
  "message": "Logged out successfully"
}
```

**Side effects:**
- Access token di-blacklist di Redis (sampai waktu expiry-nya)
- Refresh token dihapus dari Redis (jika dikirim)
- Login history dicatat dengan status "logout"

**Possible Errors:**

| Code | Error | Keterangan |
|------|-------|------------|
| 401 | Missing authorization header | Header Authorization tidak ada |
| 401 | Invalid or expired token | Token JWT tidak valid |
| 401 | Token has been revoked | Token sudah di-blacklist |
| 401 | Invalid credential | Credential ID dari token tidak valid |
| 401 | Invalid tenant | Tenant ID dari token tidak valid |

---

### Logout All

Logout dari semua session (semua device). Menghapus semua refresh token user.

```
POST /api/v1/auth/logout/all
```

**Auth Required:** Ya

**Request Headers:**
```
Authorization: Bearer <access_token>
```

**Request Body:** Tidak diperlukan

**Response (200):**
```json
{
  "message": "All sessions logged out successfully"
}
```

**Side effects:**
- **Semua** refresh token untuk user ini dihapus dari Redis
- Access token saat ini di-blacklist
- Login history dicatat

**Catatan:** Access token di device lain masih valid sampai expired (max 15 menit). Hanya refresh token yang langsung di-invalidate.

**Possible Errors:**

| Code | Error | Keterangan |
|------|-------|------------|
| 401 | Missing authorization header | Header Authorization tidak ada |
| 401 | Invalid or expired token | Token JWT tidak valid |
| 401 | Token has been revoked | Token sudah di-blacklist |
| 401 | Invalid credential | Credential ID dari token tidak valid |
| 401 | Invalid tenant | Tenant ID dari token tidak valid |

---

## JWT Access Token Claims

Setelah decode JWT access token, payload berisi:

```json
{
  "sub": "7c9e6679-7425-40de-944b-e07fc1f90ae7",
  "tenant_id": "550e8400-e29b-41d4-a716-446655440000",
  "role": "traveler",
  "permissions": [
    "booking:create",
    "booking:read",
    "booking:update",
    "booking:delete",
    "user:read",
    "user:update",
    "payment:process"
  ],
  "provider": "email",
  "token_type": "access",
  "jti": "unique-token-id",
  "iat": 1700000000,
  "exp": 1700000900
}
```

| Claim | Tipe | Keterangan |
|-------|------|------------|
| `sub` | string (UUID) | Credential ID (user identifier) |
| `tenant_id` | string (UUID) | Tenant ID |
| `role` | string | Role code (`super_admin`, `admin`, `agent`, `traveler`) |
| `permissions` | string[] | Permission codes yang dimiliki user |
| `provider` | string | Auth provider (`email` atau `google`) |
| `token_type` | string | Selalu `"access"` |
| `jti` | string (UUID) | Unique token identifier (untuk blacklisting) |
| `iat` | int | Issued at (Unix timestamp) |
| `exp` | int | Expires at (Unix timestamp) |

---

## Rate Limits

| Endpoint | Limit | Window | Keterangan |
|----------|-------|--------|------------|
| `POST /auth/login` | 5 request | 15 menit | Per email + tenant |
| `POST /auth/password/forgot` | 3 request | 1 jam | Per email + tenant |

Saat rate limit terlampaui, response:
```json
{
  "error": "Too many login attempts. Please try again later.",
  "code": 429
}
```

---

## Quick Reference

| Method | Endpoint | Auth | Keterangan |
|--------|----------|------|------------|
| `GET` | `/health` | No | Health check |
| `POST` | `/api/v1/auth/register` | No | Register akun baru |
| `POST` | `/api/v1/auth/login` | No | Login email + password |
| `GET` | `/api/v1/auth/oauth/google` | No | Initiate Google OAuth |
| `GET` | `/api/v1/auth/oauth/google/callback` | No | Google OAuth callback |
| `POST` | `/api/v1/auth/token/refresh` | No | Refresh token pair |
| `GET` | `/api/v1/auth/.well-known/jwks.json` | No | Public key (JWKS) |
| `GET` | `/api/v1/auth/verify-email` | No | Verify email address |
| `POST` | `/api/v1/auth/password/forgot` | No | Request password reset |
| `POST` | `/api/v1/auth/password/reset` | No | Reset password with token |
| `POST` | `/api/v1/auth/password/change` | **Yes** | Change password |
| `POST` | `/api/v1/auth/logout` | **Yes** | Logout current session |
| `POST` | `/api/v1/auth/logout/all` | **Yes** | Logout all sessions |

---

## Authentication Flows

### Flow 1: Email Registration + Login

```
Client                          Auth Service                    Database/Redis
  │                                  │                               │
  │  1. POST /register               │                               │
  │  {email, password, tenant_id}    │                               │
  │─────────────────────────────────>│                               │
  │                                  │  Validate tenant              │
  │                                  │──────────────────────────────>│
  │                                  │  Hash password (bcrypt)       │
  │                                  │  Create credential            │
  │                                  │──────────────────────────────>│
  │                                  │  Generate verification token  │
  │  201 {message, credential_id}    │  (TODO: send email)           │
  │<─────────────────────────────────│                               │
  │                                  │                               │
  │  2. GET /verify-email            │                               │
  │  ?token=xxx&email=xxx            │                               │
  │─────────────────────────────────>│                               │
  │                                  │  Verify token                 │
  │                                  │──────────────────────────────>│
  │                                  │  Mark email_verified = true   │
  │  200 {message}                   │──────────────────────────────>│
  │<─────────────────────────────────│                               │
  │                                  │                               │
  │  3. POST /login                  │                               │
  │  {email, password, tenant_id}    │                               │
  │─────────────────────────────────>│                               │
  │                                  │  Check rate limit (Redis)     │
  │                                  │──────────────────────────────>│
  │                                  │  Verify password (bcrypt)     │
  │                                  │  Generate JWT + refresh token │
  │                                  │  Store refresh token (Redis)  │
  │  200 {access_token,              │──────────────────────────────>│
  │       refresh_token,             │                               │
  │       token_type, expires_in}    │                               │
  │<─────────────────────────────────│                               │
```

### Flow 2: Google OAuth (Login + Auto-Register)

```
Client                   Auth Service              Google              Database/Redis
  │                           │                      │                      │
  │  1. GET /oauth/google     │                      │                      │
  │  ?tenant_id=xxx           │                      │                      │
  │──────────────────────────>│                      │                      │
  │                           │  Generate PKCE       │                      │
  │                           │  code_verifier       │                      │
  │                           │  Store state (Redis) │                      │
  │                           │─────────────────────────────────────────── >│
  │  302 Redirect             │                      │                      │
  │<──────────────────────────│                      │                      │
  │                           │                      │                      │
  │  2. User consents on Google                      │                      │
  │──────────────────────────────────────────────── >│                      │
  │                           │                      │                      │
  │  3. Redirect to callback  │                      │                      │
  │  ?code=xxx&state=xxx      │                      │                      │
  │──────────────────────────>│                      │                      │
  │                           │  Validate state      │                      │
  │                           │─────────────────────────────────────────── >│
  │                           │  Exchange code       │                      │
  │                           │─────────────────── >│                      │
  │                           │  Get user info       │                      │
  │                           │<─────────────────── │                      │
  │                           │                      │                      │
  │                           │  Find or create      │                      │
  │                           │  credential          │                      │
  │                           │  (auto-verified)     │                      │
  │                           │─────────────────────────────────────────── >│
  │                           │  Generate JWT +      │                      │
  │                           │  refresh token       │                      │
  │  200 {access_token,       │  Store (Redis)       │                      │
  │       refresh_token, ...} │─────────────────────────────────────────── >│
  │<──────────────────────────│                      │                      │
```

### Flow 3: Token Refresh (Rotation)

```
Client                          Auth Service                    Redis
  │                                  │                            │
  │  POST /token/refresh             │                            │
  │  {refresh_token, tenant_id}      │                            │
  │─────────────────────────────────>│                            │
  │                                  │  Lookup refresh token      │
  │                                  │───────────────────────────>│
  │                                  │  Validate token + tenant   │
  │                                  │  Delete old refresh token  │
  │                                  │───────────────────────────>│
  │                                  │  Generate new JWT          │
  │                                  │  Generate new refresh token│
  │                                  │  Store new refresh token   │
  │  200 {access_token,              │───────────────────────────>│
  │       refresh_token (NEW),       │                            │
  │       token_type, expires_in}    │                            │
  │<─────────────────────────────────│                            │
```

> **Penting:** Refresh token bersifat one-time use. Jika client mengirim token yang sudah pernah dipakai, request akan gagal (401).

### Flow 4: Password Reset

```
Client                          Auth Service                    Database
  │                                  │                            │
  │  1. POST /password/forgot        │                            │
  │  {email, tenant_id}              │                            │
  │─────────────────────────────────>│                            │
  │                                  │  Lookup credential         │
  │                                  │───────────────────────────>│
  │                                  │  Generate reset token      │
  │                                  │  Store to auth_password_   │
  │                                  │  reset (TTL 1 jam)         │
  │  200 {message}                   │───────────────────────────>│
  │<─────────────────────────────────│                            │
  │  (selalu 200, anti-enumeration)  │  (TODO: send email)        │
  │                                  │                            │
  │  2. POST /password/reset         │                            │
  │  {token, email,                  │                            │
  │   new_password,                  │                            │
  │   new_password_confirmation}     │                            │
  │─────────────────────────────────>│                            │
  │                                  │  Validate reset token      │
  │                                  │───────────────────────────>│
  │                                  │  Hash new password         │
  │                                  │  Update credential         │
  │                                  │───────────────────────────>│
  │                                  │  Revoke ALL refresh tokens │
  │  200 {message}                   │  Mark token used           │
  │<─────────────────────────────────│                            │
```

---

## Token Lifecycle

### Access Token

| Properti | Nilai |
|----------|-------|
| Format | JWT (RS256 signed) |
| Lifetime | **15 menit** (900 detik) |
| Storage (client) | Memory / secure variable (JANGAN simpan di localStorage) |
| Revocation | Blacklist di Redis via `/logout` |
| Verification | Via JWKS endpoint (`/.well-known/jwks.json`) |

### Refresh Token

| Properti | Nilai |
|----------|-------|
| Format | Random hex string |
| Lifetime | **30 hari** |
| Storage (client) | Secure HTTP-only cookie / encrypted storage |
| Rotation | One-time use, setiap refresh menghasilkan token baru |
| Revocation | Hapus dari Redis via `/logout`, `/logout/all`, password reset, atau password change |

### Token Refresh Strategy (Client-Side)

```
┌──────────────────────────────────────────────────────┐
│ Recommended: Proactive Refresh                       │
│                                                      │
│ 1. Decode access_token (tanpa verify, hanya baca)    │
│ 2. Cek field `exp` (expiry timestamp)                │
│ 3. Jika exp - now < 60 detik → panggil /token/refresh│
│ 4. Gunakan access_token baru untuk request berikutnya│
│ 5. Simpan refresh_token baru, buang yang lama        │
└──────────────────────────────────────────────────────┘

┌──────────────────────────────────────────────────────┐
│ Fallback: Reactive Refresh                           │
│                                                      │
│ 1. Kirim request dengan access_token                 │
│ 2. Jika response 401 → panggil /token/refresh        │
│ 3. Retry request awal dengan access_token baru       │
│ 4. Jika /token/refresh juga 401 → redirect ke login  │
└──────────────────────────────────────────────────────┘
```

---

## Multi-Tenant

Service ini mendukung multi-tenant. Setiap user (credential) di-scope ke satu tenant.

### Konsep

- **Tenant** = organisasi/bisnis yang menggunakan platform (misal: travel agency A, travel agency B)
- Satu email bisa terdaftar di **beberapa tenant** yang berbeda
- `tenant_id` wajib di setiap request registrasi, login, dan token refresh
- JWT claims menyertakan `tenant_id` untuk authorization di service lain

### Contoh Multi-Tenant

```
user@example.com terdaftar di:
├── Tenant A (agency-alpha)  → credential_id: aaa-...
│   └── role: admin
└── Tenant B (agency-beta)   → credential_id: bbb-...
    └── role: traveler
```

User harus login dengan `tenant_id` yang spesifik. Tidak ada "global login".

---

## Security Considerations

### Password Policy

- Minimum **8 karakter**
- Di-hash menggunakan **bcrypt** (cost factor default)
- Password lama tidak pernah disimpan dalam plaintext

### Account Lockout

- Setelah **5 kali gagal login** → account terkunci selama **30 menit**
- Counter direset setelah login berhasil
- Lockout bersifat per-credential (per email + tenant)

### Anti-Enumeration

- `POST /password/forgot` **selalu** mengembalikan 200 dan pesan yang sama, baik email ditemukan atau tidak
- Mencegah attacker mengetahui email mana yang terdaftar

### OAuth Security

- **PKCE** (Proof Key for Code Exchange) digunakan untuk Google OAuth
  - `code_challenge_method`: S256
  - Mencegah authorization code interception
- **State parameter** dengan expiry 10 menit (anti-CSRF)
- State disimpan di Redis, divalidasi dan dihapus setelah digunakan (one-time use)

### Token Security

- JWT ditandatangani dengan **RSA256** (asymmetric) — private key hanya di auth service
- Service lain memverifikasi token menggunakan public key dari JWKS endpoint
- Refresh token bersifat **one-time use** (rotation) — mengurangi risiko token theft
- Access token blacklisting via Redis untuk logout segera

---

## Integration Guide

### Untuk Service Lain (Verifikasi JWT)

Service lain yang menerima request dari user harus memverifikasi JWT:

1. **Fetch JWKS** dari `GET /api/v1/auth/.well-known/jwks.json`
   - Cache public key (refresh periodik, misal setiap 1 jam)

2. **Verifikasi JWT:**
   - Validate signature menggunakan public key (RS256)
   - Cek `exp` belum lewat
   - Cek `token_type` == `"access"`
   - Cek `tenant_id` sesuai konteks

3. **Gunakan claims** untuk authorization:
   ```
   sub          → user ID (credential_id)
   tenant_id    → tenant scope
   role         → role-based access control
   permissions  → fine-grained permission check
   ```

### Contoh: Go Middleware (Pseudocode)

```go
func AuthMiddleware(jwksURL string) gin.HandlerFunc {
    // Fetch & cache JWKS
    keySet := fetchJWKS(jwksURL)

    return func(c *gin.Context) {
        token := extractBearerToken(c)
        if token == "" {
            c.AbortWithStatusJSON(401, gin.H{"error": "Missing authorization header"})
            return
        }

        claims, err := verifyJWT(token, keySet)
        if err != nil {
            c.AbortWithStatusJSON(401, gin.H{"error": "Invalid or expired token"})
            return
        }

        // Set claims ke context untuk handler
        c.Set("credential_id", claims.Subject)
        c.Set("tenant_id", claims.TenantID)
        c.Set("role", claims.Role)
        c.Set("permissions", claims.Permissions)
        c.Next()
    }
}
```

### Contoh: cURL Requests

**Register:**
```bash
curl -X POST http://localhost:8080/api/v1/auth/register \
  -H "Content-Type: application/json" \
  -d '{
    "email": "user@example.com",
    "password": "MyStr0ng!Pass",
    "password_confirmation": "MyStr0ng!Pass",
    "tenant_id": "550e8400-e29b-41d4-a716-446655440000"
  }'
```

**Login:**
```bash
curl -X POST http://localhost:8080/api/v1/auth/login \
  -H "Content-Type: application/json" \
  -d '{
    "email": "user@example.com",
    "password": "MyStr0ng!Pass",
    "tenant_id": "550e8400-e29b-41d4-a716-446655440000"
  }'
```

**Authenticated Request (contoh: change password):**
```bash
curl -X POST http://localhost:8080/api/v1/auth/password/change \
  -H "Content-Type: application/json" \
  -H "Authorization: Bearer eyJhbGciOiJSUzI1NiIsI..." \
  -d '{
    "current_password": "MyStr0ng!Pass",
    "new_password": "NewStr0ng!Pass",
    "new_password_confirmation": "NewStr0ng!Pass"
  }'
```

**Refresh Token:**
```bash
curl -X POST http://localhost:8080/api/v1/auth/token/refresh \
  -H "Content-Type: application/json" \
  -d '{
    "refresh_token": "a1b2c3d4e5f6...",
    "tenant_id": "550e8400-e29b-41d4-a716-446655440000"
  }'
```

**Logout:**
```bash
curl -X POST http://localhost:8080/api/v1/auth/logout \
  -H "Authorization: Bearer eyJhbGciOiJSUzI1NiIsI..." \
  -H "Content-Type: application/json" \
  -d '{"refresh_token": "a1b2c3d4e5f6..."}'
```

**Logout All Sessions:**
```bash
curl -X POST http://localhost:8080/api/v1/auth/logout/all \
  -H "Authorization: Bearer eyJhbGciOiJSUzI1NiIsI..."
```

---

## Roles & Permissions

### Available Roles

| Role | Code | Keterangan |
|------|------|------------|
| Super Admin | `super_admin` | Full access ke semua resource |
| Admin | `admin` | Manage tenant, users, dan settings |
| Agent | `agent` | Manage bookings dan travelers |
| Traveler | `traveler` | End user, akses terbatas |

### Permission Format

Permission menggunakan format `resource:action`:

```
booking:create    → Buat booking baru
booking:read      → Lihat booking
booking:update    → Update booking
booking:delete    → Hapus booking
user:read         → Lihat data user
user:update       → Update data user
payment:process   → Proses pembayaran
```

### Authorization Check (di Service Lain)

```go
// Cek role
if claims.Role != "admin" && claims.Role != "super_admin" {
    // forbidden
}

// Cek permission spesifik
if !contains(claims.Permissions, "booking:create") {
    // forbidden
}
```

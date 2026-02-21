# Authentication Flows

Dokumen ini menjelaskan seluruh alur autentikasi yang didukung oleh TravApp Auth Service.

## Daftar Isi

- [1. Email Registration](#1-email-registration)
- [2. Email Verification](#2-email-verification)
- [3. Email Login](#3-email-login)
- [4. Google OAuth Login](#4-google-oauth-login)
- [5. Token Refresh](#5-token-refresh)
- [6. Logout (Single Session)](#6-logout-single-session)
- [7. Logout All Sessions](#7-logout-all-sessions)
- [8. Forgot Password](#8-forgot-password)
- [9. Reset Password](#9-reset-password)
- [10. Change Password](#10-change-password)

---

## 1. Email Registration

Mendaftarkan akun baru dengan email dan password.

```
Client                      Auth Service                   PostgreSQL
  |                              |                              |
  |  POST /auth/register         |                              |
  |  {email, password,           |                              |
  |   password_confirmation,     |                              |
  |   tenant_id}                 |                              |
  |----------------------------->|                              |
  |                              |  Validate tenant (active?)   |
  |                              |----------------------------->|
  |                              |<-----------------------------|
  |                              |                              |
  |                              |  Check email conflict        |
  |                              |----------------------------->|
  |                              |<-----------------------------|
  |                              |                              |
  |                              |  Hash password (BCrypt)      |
  |                              |                              |
  |                              |  Create credential           |
  |                              |  (is_email_verified=false)   |
  |                              |----------------------------->|
  |                              |<-----------------------------|
  |                              |                              |
  |                              |  Assign default role         |
  |                              |  ("traveler")                |
  |                              |----------------------------->|
  |                              |<-----------------------------|
  |                              |                              |
  |                              |  Generate verification token |
  |                              |  Save to email_verification  |
  |                              |----------------------------->|
  |                              |<-----------------------------|
  |                              |                              |
  |  201 {credential_id,         |                              |
  |       verification_token}    |                              |
  |<-----------------------------|                              |
```

**Catatan penting:**
- Password di-hash menggunakan BCrypt (cost=12 di production)
- Email **belum terverifikasi** setelah register — user harus verify email dulu
- Setiap user otomatis mendapat role default tenant ("traveler")
- Verification token berlaku 24 jam

**Request:**
```json
POST /api/v1/auth/register
{
  "email": "user@example.com",
  "password": "MyStr0ng!Pass",
  "password_confirmation": "MyStr0ng!Pass",
  "tenant_id": "uuid-tenant"
}
```

**Response (201):**
```json
{
  "credential_id": "uuid-credential",
  "verification_token": "hex-64-chars"
}
```

---

## 2. Email Verification

Memverifikasi email setelah registrasi.

```
Client                      Auth Service                   PostgreSQL
  |                              |                              |
  |  GET /auth/verify-email      |                              |
  |  ?token=xxx&email=xxx        |                              |
  |----------------------------->|                              |
  |                              |  SHA256(token) → lookup      |
  |                              |----------------------------->|
  |                              |<-----------------------------|
  |                              |                              |
  |                              |  Check: expired? used?       |
  |                              |                              |
  |                              |  Update credential           |
  |                              |  is_email_verified = true    |
  |                              |----------------------------->|
  |                              |<-----------------------------|
  |                              |                              |
  |                              |  Mark token as verified      |
  |                              |----------------------------->|
  |                              |<-----------------------------|
  |                              |                              |
  |  200 {message: "verified"}   |                              |
  |<-----------------------------|                              |
```

**Catatan:**
- Token disimpan sebagai SHA256 hash di database (bukan plaintext)
- Token hanya bisa digunakan 1x (one-time use)
- Token kadaluarsa setelah 24 jam

---

## 3. Email Login

Login menggunakan email dan password.

```
Client                      Auth Service           Redis              PostgreSQL
  |                              |                    |                    |
  |  POST /auth/login            |                    |                    |
  |  {email, password,           |                    |                    |
  |   tenant_id}                 |                    |                    |
  |----------------------------->|                    |                    |
  |                              |  Validate tenant   |                    |
  |                              |-------------------------------------->|
  |                              |<--------------------------------------|
  |                              |                    |                    |
  |                              |  Check rate limit  |                    |
  |                              |  (5 per 15 min)    |                    |
  |                              |------------------->|                    |
  |                              |<-------------------|                    |
  |                              |                    |                    |
  |                              |  Get credential    |                    |
  |                              |-------------------------------------->|
  |                              |<--------------------------------------|
  |                              |                    |                    |
  |                              |  Check account lock|                    |
  |                              |  (locked_until)    |                    |
  |                              |                    |                    |
  |                              |  BCrypt compare    |                    |
  |                              |  password          |                    |
  |                              |                    |                    |
  |                              |  Check is_active   |                    |
  |                              |  Check email_verified                   |
  |                              |                    |                    |
  |                              |  Reset failed      |                    |
  |                              |  attempts          |                    |
  |                              |-------------------------------------->|
  |                              |                    |                    |
  |                              |  Get role +        |                    |
  |                              |  permissions       |                    |
  |                              |-------------------------------------->|
  |                              |<--------------------------------------|
  |                              |                    |                    |
  |                              |  Generate JWT      |                    |
  |                              |  (RS256, 15 min)   |                    |
  |                              |                    |                    |
  |                              |  Generate refresh  |                    |
  |                              |  token (random hex)|                    |
  |                              |                    |                    |
  |                              |  Store refresh     |                    |
  |                              |  token in Redis    |                    |
  |                              |------------------->|                    |
  |                              |<-------------------|                    |
  |                              |                    |                    |
  |                              |  Record login      |                    |
  |                              |  history           |                    |
  |                              |-------------------------------------->|
  |                              |                    |                    |
  |  200 {access_token,          |                    |                    |
  |       refresh_token,         |                    |                    |
  |       token_type: "Bearer",  |                    |                    |
  |       expires_in: 900}       |                    |                    |
  |<-----------------------------|                    |                    |
```

**Security checks (berurutan):**
1. Tenant valid & active
2. Rate limit (max 5 percobaan per 15 menit per email+tenant)
3. Credential ditemukan
4. Account tidak ter-lock (lockout 30 menit setelah 5x gagal)
5. Password benar (BCrypt compare)
6. Account aktif (`is_active = true`)
7. Email terverifikasi (`is_email_verified = true`)

**Mekanisme lockout:**
- Setiap password salah → `failed_login_attempts` bertambah 1
- Jika `failed_login_attempts >= 5` → account di-lock selama 30 menit (`locked_until`)
- Login berhasil → `failed_login_attempts` di-reset ke 0

**JWT Access Token claims:**
```json
{
  "sub": "credential-uuid",
  "tenant_id": "tenant-uuid",
  "role": "traveler",
  "permissions": ["booking:create", "booking:read", ...],
  "provider": "email",
  "jti": "unique-token-id",
  "exp": 1234567890,
  "iat": 1234567890
}
```

---

## 4. Google OAuth Login

Login/register otomatis via Google OAuth2 dengan PKCE.

### 4a. Initiate (Redirect ke Google)

```
Client                      Auth Service                   Redis
  |                              |                            |
  |  GET /auth/oauth/google      |                            |
  |  ?tenant_id=xxx              |                            |
  |----------------------------->|                            |
  |                              |  Validate tenant           |
  |                              |                            |
  |                              |  Generate state (random)   |
  |                              |  Generate code_verifier    |
  |                              |  Generate code_challenge   |
  |                              |  (S256)                    |
  |                              |                            |
  |                              |  Store state + verifier    |
  |                              |  (TTL 10 min)              |
  |                              |--------------------------->|
  |                              |<---------------------------|
  |                              |                            |
  |  302 Redirect →              |                            |
  |  accounts.google.com/o/      |                            |
  |  oauth2/auth?                |                            |
  |    client_id=xxx&            |                            |
  |    redirect_uri=xxx&         |                            |
  |    scope=openid email&       |                            |
  |    state=xxx&                |                            |
  |    code_challenge=xxx&       |                            |
  |    code_challenge_method=S256|                            |
  |<-----------------------------|                            |
```

### 4b. Callback (Google → Auth Service)

```
Google                      Auth Service           Redis              PostgreSQL
  |                              |                    |                    |
  |  GET /auth/oauth/google/     |                    |                    |
  |  callback?code=xxx&state=xxx |                    |                    |
  |----------------------------->|                    |                    |
  |                              |  Validate state    |                    |
  |                              |------------------->|                    |
  |                              |<-------------------|                    |
  |                              |                    |                    |
  |                              |  Delete state      |                    |
  |                              |------------------->|                    |
  |                              |                    |                    |
  |                              |  Exchange code →   |                    |
  |                              |  Google token      |                    |
  |                              |  endpoint          |                    |
  |                              |  (with verifier)   |                    |
  |                              |                    |                    |
  |                              |  Verify ID token   |                    |
  |                              |  (iss, aud, exp)   |                    |
  |                              |                    |                    |
  |                              |  Lookup by         |                    |
  |                              |  provider_id       |                    |
  |                              |-------------------------------------->|
  |                              |<--------------------------------------|
  |                              |                    |                    |
  |                          [if new user]            |                    |
  |                              |  Create credential |                    |
  |                              |  (provider=google, |                    |
  |                              |   email_verified   |                    |
  |                              |   =true)           |                    |
  |                              |-------------------------------------->|
  |                              |  Assign default    |                    |
  |                              |  role              |                    |
  |                              |-------------------------------------->|
  |                              |                    |                    |
  |                              |  Generate tokens   |                    |
  |                              |  (same as login)   |                    |
  |                              |                    |                    |
  |  200 {access_token,          |                    |                    |
  |       refresh_token, ...}    |                    |                    |
  |<-----------------------------|                    |                    |
```

**Catatan:**
- Google OAuth menggabungkan login + auto-register dalam satu flow
- User Google otomatis `is_email_verified = true`
- PKCE (S256) digunakan untuk keamanan tambahan
- State disimpan di Redis dengan TTL 10 menit
- Jika user sudah ada (by `provider_id`), langsung login
- Jika user baru, otomatis di-register lalu login

---

## 5. Token Refresh

Menukar refresh token lama dengan access token + refresh token baru (rotation).

```
Client                      Auth Service           Redis              PostgreSQL
  |                              |                    |                    |
  |  POST /auth/token/refresh    |                    |                    |
  |  {refresh_token, tenant_id}  |                    |                    |
  |----------------------------->|                    |                    |
  |                              |  SHA256(token) →   |                    |
  |                              |  find in Redis     |                    |
  |                              |------------------->|                    |
  |                              |<-------------------|                    |
  |                              |                    |                    |
  |                              |  Validate tenant   |                    |
  |                              |  matches           |                    |
  |                              |                    |                    |
  |                              |  Delete old token  |                    |
  |                              |  (consume)         |                    |
  |                              |------------------->|                    |
  |                              |                    |                    |
  |                              |  Get credential    |                    |
  |                              |  (check active)    |                    |
  |                              |-------------------------------------->|
  |                              |<--------------------------------------|
  |                              |                    |                    |
  |                              |  Get role +        |                    |
  |                              |  permissions       |                    |
  |                              |-------------------------------------->|
  |                              |<--------------------------------------|
  |                              |                    |                    |
  |                              |  Generate new      |                    |
  |                              |  access token      |                    |
  |                              |                    |                    |
  |                              |  Generate new      |                    |
  |                              |  refresh token     |                    |
  |                              |  (same family_id)  |                    |
  |                              |                    |                    |
  |                              |  Store new refresh |                    |
  |                              |------------------->|                    |
  |                              |<-------------------|                    |
  |                              |                    |                    |
  |  200 {access_token,          |                    |                    |
  |       refresh_token,         |                    |                    |
  |       token_type, expires_in}|                    |                    |
  |<-----------------------------|                    |                    |
```

**Mekanisme keamanan:**
- **Rotation**: Setiap refresh menghasilkan token baru, token lama dihapus
- **Family-based**: Semua rotasi token dari satu login session berbagi `family_id`
- **One-time use**: Refresh token hanya bisa dipakai 1x
- **Refresh token TTL**: 30 hari (default)

---

## 6. Logout (Single Session)

Logout dari sesi saat ini.

```
Client                      Auth Service                   Redis
  |                              |                            |
  |  POST /auth/logout           |                            |
  |  Authorization: Bearer xxx   |                            |
  |  {refresh_token?}            |                            |
  |----------------------------->|                            |
  |                              |  Extract credential_id,    |
  |                              |  tenant_id, jti, expiry    |
  |                              |  from JWT                  |
  |                              |                            |
  |                           [if refresh_token provided]     |
  |                              |  Find & delete refresh     |
  |                              |  token from Redis          |
  |                              |--------------------------->|
  |                              |<---------------------------|
  |                              |                            |
  |                           [if access token not expired]   |
  |                              |  Blacklist access token    |
  |                              |  (JTI, TTL=remaining)      |
  |                              |--------------------------->|
  |                              |<---------------------------|
  |                              |                            |
  |                              |  Record logout history     |
  |                              |                            |
  |  200 {message: "logged out"} |                            |
  |<-----------------------------|                            |
```

**Catatan:**
- Membutuhkan valid access token (middleware auth)
- Refresh token opsional — jika dikirim, dihapus dari Redis
- Access token di-blacklist di Redis sampai expiry time-nya habis
- Middleware auth akan cek blacklist setiap request

---

## 7. Logout All Sessions

Logout dari semua sesi (semua device).

```
Client                      Auth Service                   Redis
  |                              |                            |
  |  POST /auth/logout/all       |                            |
  |  Authorization: Bearer xxx   |                            |
  |----------------------------->|                            |
  |                              |  Delete ALL refresh        |
  |                              |  tokens for credential     |
  |                              |--------------------------->|
  |                              |<---------------------------|
  |                              |                            |
  |                              |  Blacklist current         |
  |                              |  access token              |
  |                              |--------------------------->|
  |                              |<---------------------------|
  |                              |                            |
  |                              |  Record logout history     |
  |                              |                            |
  |  200 {message: "logged out"} |                            |
  |<-----------------------------|                            |
```

**Catatan:**
- Semua refresh token di semua device langsung dihapus
- Access token di device lain masih valid sampai expired (max 15 menit)
- Untuk invalidasi access token di semua device secara instan, perlu implementasi tambahan

---

## 8. Forgot Password

Meminta link reset password via email.

```
Client                      Auth Service           Redis              PostgreSQL
  |                              |                    |                    |
  |  POST /auth/password/forgot  |                    |                    |
  |  {email, tenant_id}          |                    |                    |
  |----------------------------->|                    |                    |
  |                              |  Check rate limit  |                    |
  |                              |  (3 per 1 hour)    |                    |
  |                              |------------------->|                    |
  |                              |<-------------------|                    |
  |                              |                    |                    |
  |                              |  Increment rate    |                    |
  |                              |  limit counter     |                    |
  |                              |------------------->|                    |
  |                              |                    |                    |
  |                              |  Find credential   |                    |
  |                              |-------------------------------------->|
  |                              |<--------------------------------------|
  |                              |                    |                    |
  |                           [if credential not found]                    |
  |  200 {} (silent success)     |  (prevent email    |                    |
  |<-----------------------------|   enumeration)     |                    |
  |                              |                    |                    |
  |                           [if credential found]   |                    |
  |                              |  Generate reset    |                    |
  |                              |  token             |                    |
  |                              |  Save SHA256 hash  |                    |
  |                              |  (expires 1 hour)  |                    |
  |                              |-------------------------------------->|
  |                              |<--------------------------------------|
  |                              |                    |                    |
  |  200 {reset_token}           |                    |                    |
  |<-----------------------------|                    |                    |
```

**Security:**
- Rate limited: max 3 request per email per jam
- Jika email tidak ditemukan, tetap return 200 (mencegah email enumeration)
- Token kadaluarsa setelah 1 jam
- Token disimpan sebagai SHA256 hash

> **TODO**: Saat ini reset_token dikembalikan langsung di response. Di production, token harus dikirim via email, bukan di response body.

---

## 9. Reset Password

Menggunakan token dari forgot password untuk set password baru.

```
Client                      Auth Service                   PostgreSQL
  |                              |                              |
  |  POST /auth/password/reset   |                              |
  |  {token, email,              |                              |
  |   new_password,              |                              |
  |   new_password_confirmation} |                              |
  |----------------------------->|                              |
  |                              |  SHA256(token) → lookup      |
  |                              |----------------------------->|
  |                              |<-----------------------------|
  |                              |                              |
  |                              |  Check: valid? expired?      |
  |                              |  already used?               |
  |                              |                              |
  |                              |  Hash new password           |
  |                              |                              |
  |                              |  Update credential password  |
  |                              |----------------------------->|
  |                              |<-----------------------------|
  |                              |                              |
  |                              |  Mark token as used          |
  |                              |----------------------------->|
  |                              |                              |
  |                              |  Revoke all refresh tokens   |
  |                              |  (force re-login)            |
  |                              |                              |
  |  200 {message: "reset ok"}   |                              |
  |<-----------------------------|                              |
```

**Catatan:**
- Token one-time use — setelah dipakai, ditandai `used_at`
- Semua refresh token dihapus setelah reset (force re-login di semua device)

---

## 10. Change Password

Mengubah password saat sudah login (authenticated).

```
Client                      Auth Service           Redis              PostgreSQL
  |                              |                    |                    |
  |  POST /auth/password/change  |                    |                    |
  |  Authorization: Bearer xxx   |                    |                    |
  |  {current_password,          |                    |                    |
  |   new_password,              |                    |                    |
  |   new_password_confirmation} |                    |                    |
  |----------------------------->|                    |                    |
  |                              |  Get credential    |                    |
  |                              |-------------------------------------->|
  |                              |<--------------------------------------|
  |                              |                    |                    |
  |                              |  Check not OAuth   |                    |
  |                              |  account (must     |                    |
  |                              |  have password)    |                    |
  |                              |                    |                    |
  |                              |  BCrypt compare    |                    |
  |                              |  current password  |                    |
  |                              |                    |                    |
  |                              |  Hash new password |                    |
  |                              |                    |                    |
  |                              |  Update password   |                    |
  |                              |-------------------------------------->|
  |                              |<--------------------------------------|
  |                              |                    |                    |
  |                              |  Revoke all        |                    |
  |                              |  refresh tokens    |                    |
  |                              |------------------->|                    |
  |                              |                    |                    |
  |  200 {message: "changed"}    |                    |                    |
  |<-----------------------------|                    |                    |
```

**Catatan:**
- Membutuhkan valid access token (authenticated)
- OAuth accounts (Google) tidak bisa change password (return 400)
- Harus menyertakan current password yang benar
- Semua refresh token dihapus setelah change (force re-login)

---

## JWKS Endpoint

Public key untuk verifikasi JWT bisa diakses di:

```
GET /api/v1/auth/.well-known/jwks.json
```

Response:
```json
{
  "keys": [
    {
      "kty": "RSA",
      "use": "sig",
      "alg": "RS256",
      "kid": "1",
      "n": "base64url-encoded-modulus",
      "e": "AQAB"
    }
  ]
}
```

Service lain di ekosistem TravApp bisa menggunakan endpoint ini untuk memverifikasi JWT tanpa perlu shared secret.

---

## Token Lifecycle Summary

| Token | Storage | TTL | Penggunaan |
|-------|---------|-----|------------|
| Access Token (JWT) | Client-side | 15 menit | Authorization header setiap request |
| Refresh Token | Redis (hash) | 30 hari | Menukar access token baru |
| Email Verification | PostgreSQL (hash) | 24 jam | Verifikasi email setelah register |
| Password Reset | PostgreSQL (hash) | 1 jam | Reset password |
| OAuth State | Redis | 10 menit | CSRF protection pada OAuth flow |

---

## Error Responses

Semua error mengikuti format standar:

```json
{
  "error": "Pesan error yang user-friendly"
}
```

| HTTP Code | Keterangan |
|-----------|------------|
| 400 | Bad Request — input tidak valid |
| 401 | Unauthorized — credentials salah atau token invalid |
| 409 | Conflict — email sudah terdaftar |
| 423 | Locked — account ter-lock karena terlalu banyak gagal login |
| 429 | Too Many Requests — rate limit terlampaui |
| 500 | Internal Server Error — error di server |

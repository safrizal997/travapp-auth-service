# Architecture

Dokumen ini menjelaskan arsitektur, pola desain, dan keputusan teknis TravApp Auth Service.

## Daftar Isi

- [Overview](#overview)
- [Tech Stack](#tech-stack)
- [Project Structure](#project-structure)
- [Layered Architecture](#layered-architecture)
- [Dependency Injection](#dependency-injection)
- [Multi-Tenancy](#multi-tenancy)
- [Authentication & Authorization](#authentication--authorization)
- [Token Management](#token-management)
- [Security Mechanisms](#security-mechanisms)
- [Error Handling](#error-handling)
- [Middleware Pipeline](#middleware-pipeline)
- [Database Design](#database-design)
- [Redis Data Model](#redis-data-model)
- [Logging](#logging)
- [Testing Strategy](#testing-strategy)

---

## Overview

TravApp Auth Service adalah microservice autentikasi dan otorisasi yang dirancang untuk platform travel multi-tenant. Service ini menangani seluruh siklus hidup autentikasi: registrasi, login (email + Google OAuth), manajemen token JWT, reset password, dan verifikasi email.

```
┌─────────────────────────────────────────────────────────┐
│                     Client Apps                         │
│              (Web, Mobile, Other Services)               │
└──────────────────────┬──────────────────────────────────┘
                       │ HTTPS
                       ▼
┌─────────────────────────────────────────────────────────┐
│                  Auth Service (Gin)                      │
│  ┌────────────┐  ┌──────────┐  ┌─────────────────────┐  │
│  │ Middleware  │→ │ Handlers │→ │     Use Cases       │  │
│  │ (CORS,Auth,│  │ (5 files)│  │  (Business Logic)   │  │
│  │  ReqID,    │  └──────────┘  └──────────┬──────────┘  │
│  │  RateLimit)│                            │             │
│  └────────────┘                            ▼             │
│                          ┌─────────────────────────────┐ │
│                          │     Repository Interfaces    │ │
│                          └──────────┬──────────────────┘ │
└─────────────────────────────────────┼────────────────────┘
                                      │
                    ┌─────────────────┼─────────────────┐
                    ▼                                    ▼
          ┌──────────────────┐                ┌──────────────────┐
          │   PostgreSQL 16   │                │    Redis 7        │
          │                  │                │                  │
          │ - Credentials    │                │ - Refresh tokens │
          │ - Tenants        │                │ - Blacklist      │
          │ - Roles/Perms    │                │ - Rate limits    │
          │ - Login history  │                │ - OAuth state    │
          │ - Password reset │                │ - RBAC cache     │
          │ - Email verify   │                │                  │
          └──────────────────┘                └──────────────────┘
```

---

## Tech Stack

| Komponen | Teknologi | Versi | Tujuan |
|----------|-----------|-------|--------|
| Language | Go | 1.25+ | Performance, concurrency, static typing |
| HTTP Framework | Gin | v1.x | Routing, middleware, JSON binding |
| Database | PostgreSQL | 16 | Persistent data storage |
| Cache/Store | Redis | 7 | Token storage, rate limiting, caching |
| DB Driver | pgx/v5 | v5.x | PostgreSQL driver (connection pooling) |
| Redis Client | go-redis/v9 | v9.x | Redis client |
| JWT | golang-jwt/jwt/v5 | v5.x | JWT signing & validation |
| Config | Viper | - | Environment variable & .env loading |
| Logging | Zap | - | Structured logging |
| Swagger | swaggo/swag | - | API documentation generation |
| Mocking | go.uber.org/mock | - | Interface mocking for tests |
| Migration | golang-migrate | - | Database schema migrations |

---

## Project Structure

```
travapp-auth-service/
├── cmd/
│   └── server/
│       └── main.go                 # Entry point, DI wiring, server bootstrap
│
├── internal/                       # Private application code
│   ├── config/
│   │   └── config.go               # Viper-based config (env vars + .env)
│   │
│   ├── domain/                     # Domain layer (entities + interfaces)
│   │   ├── entity/                 # Domain models (8 entities)
│   │   │   ├── credential.go
│   │   │   ├── tenant.go
│   │   │   ├── role.go
│   │   │   ├── role_assignment.go
│   │   │   ├── permission.go
│   │   │   ├── login_history.go
│   │   │   ├── password_reset.go
│   │   │   └── email_verification.go
│   │   │
│   │   └── repository/             # Repository interfaces (contracts)
│   │       ├── credential_repo.go
│   │       ├── tenant_repo.go
│   │       ├── role_repo.go
│   │       ├── token_repo.go
│   │       └── login_history_repo.go
│   │
│   ├── usecase/                    # Business logic layer (6 use cases)
│   │   ├── login_usecase.go
│   │   ├── register_usecase.go
│   │   ├── password_usecase.go
│   │   ├── token_usecase.go
│   │   ├── logout_usecase.go
│   │   └── oauth_usecase.go
│   │
│   ├── delivery/http/              # HTTP delivery layer
│   │   ├── router.go               # Gin router + route registration
│   │   ├── handler/                # HTTP handlers (5 files)
│   │   │   ├── auth_handler.go     # Login + Register
│   │   │   ├── oauth_handler.go    # Google OAuth
│   │   │   ├── token_handler.go    # Refresh + JWKS
│   │   │   ├── logout_handler.go   # Logout + LogoutAll
│   │   │   └── password_handler.go # Change/Forgot/Reset Password + VerifyEmail
│   │   ├── dto/                    # Data Transfer Objects
│   │   │   ├── request.go          # 7 request DTOs
│   │   │   └── response.go         # 4 response DTOs
│   │   └── middleware/             # HTTP middleware
│   │       ├── auth_middleware.go   # JWT validation + blacklist check
│   │       ├── cors.go             # CORS headers
│   │       ├── request_id.go       # X-Request-ID generation
│   │       └── rate_limiter.go     # Rate limiter (placeholder)
│   │
│   ├── repository/                 # Repository implementations
│   │   ├── postgres/               # PostgreSQL implementations
│   │   │   ├── credential_repo.go
│   │   │   ├── tenant_repo.go
│   │   │   ├── role_repo.go
│   │   │   ├── login_history_repo.go
│   │   │   └── password_reset_repo.go
│   │   └── redis/                  # Redis implementations
│   │       └── token_repo.go
│   │
│   ├── pkg/                        # Internal shared packages
│   │   ├── apperror/errors.go      # AppError with HTTP status codes
│   │   ├── jwt/jwt.go              # RSA JWT Manager (RS256, JWKS)
│   │   ├── hash/bcrypt.go          # BCrypt hasher
│   │   ├── oauth/google.go         # Google OAuth2 + PKCE
│   │   └── crypto/random.go        # Random hex, SHA256, code verifier
│   │
│   ├── mocks/                      # Generated mocks (7 files)
│   └── testutil/testutil.go        # Shared test helpers
│
├── docs/                           # Swagger generated docs
├── keys/                           # RSA key pair (gitignored)
├── migrations/                     # SQL migration files (10 up + 10 down)
├── Dockerfile                      # Multi-stage build
├── docker-compose.yml              # Full stack (PG + Redis + App)
├── Makefile                        # Build, test, migrate commands
└── .env                            # Environment config (gitignored)
```

---

## Layered Architecture

Service mengikuti **Clean Architecture** yang dimodifikasi dengan 4 layer:

```
┌─────────────────────────────────────────────────┐
│              Delivery (HTTP)                      │
│  router.go → middleware → handler → dto          │
│  Tanggung jawab: HTTP parsing, validation,       │
│  response formatting                             │
├─────────────────────────────────────────────────┤
│              Use Case (Business Logic)            │
│  login_usecase.go, register_usecase.go, etc.     │
│  Tanggung jawab: Business rules, orchestration,  │
│  error decisions                                 │
├─────────────────────────────────────────────────┤
│              Domain (Entities + Interfaces)       │
│  entity/*.go, repository/*.go                    │
│  Tanggung jawab: Data structures, contracts      │
├─────────────────────────────────────────────────┤
│              Infrastructure (Implementations)     │
│  repository/postgres/*.go, repository/redis/*.go │
│  Tanggung jawab: Database queries, Redis ops     │
└─────────────────────────────────────────────────┘
```

### Aturan Dependency

```
Delivery → Use Case → Domain ← Infrastructure
```

- **Delivery** tergantung pada Use Case (concrete types) dan Domain (entities)
- **Use Case** tergantung pada Domain (interfaces + entities)
- **Infrastructure** mengimplementasikan Domain interfaces
- **Domain** tidak tergantung pada layer lain (paling stabil)

### Catatan: Handler → Use Case Coupling

Handlers menerima **concrete use case types** (bukan interface):

```go
type AuthHandler struct {
    loginUC    *usecase.LoginUseCase    // concrete, bukan interface
    registerUC *usecase.RegisterUseCase
}
```

Ini adalah keputusan desain yang disengaja untuk menjaga kesederhanaan. Use case sudah menerima repository interfaces, sehingga testability tetap terjaga di level use case. Handler tests menggunakan real use case + mocked repositories.

---

## Dependency Injection

Seluruh dependency wiring dilakukan secara manual di `cmd/server/main.go` (no DI framework):

```
main.go
  ├─ config.Load()
  ├─ zap.Logger
  ├─ pgxpool.Pool
  ├─ redis.Client
  ├─ jwt.Manager (RSA keys)
  ├─ hash.BCryptHasher
  ├─ oauth.GoogleConfig
  │
  ├─ Repositories (postgres + redis)
  │   ├─ CredentialRepo(dbPool)
  │   ├─ TenantRepo(dbPool)
  │   ├─ RoleRepo(dbPool)
  │   ├─ LoginHistoryRepo(dbPool)
  │   ├─ PasswordResetRepo(dbPool)
  │   └─ TokenRepo(redisClient)
  │
  ├─ Use Cases
  │   ├─ LoginUseCase(credRepo, tenantRepo, roleRepo, tokenRepo, histRepo, hasher, jwt, logger, ...)
  │   ├─ RegisterUseCase(credRepo, tenantRepo, roleRepo, hasher, logger, emailVerifStore, baseURL)
  │   ├─ OAuthUseCase(credRepo, tenantRepo, roleRepo, tokenRepo, histRepo, google, jwt, logger, ...)
  │   ├─ TokenUseCase(credRepo, roleRepo, tokenRepo, jwt, logger, accessTTL, refreshTTL)
  │   ├─ LogoutUseCase(tokenRepo, histRepo, logger)
  │   └─ PasswordUseCase(credRepo, tenantRepo, tokenRepo, hasher, logger, resetStore, baseURL)
  │
  ├─ Handlers
  │   ├─ AuthHandler(loginUC, registerUC, logger)
  │   ├─ OAuthHandler(oauthUC, logger)
  │   ├─ TokenHandler(tokenUC, jwtManager, logger)
  │   ├─ LogoutHandler(logoutUC, logger)
  │   └─ PasswordHandler(passwordUC, logger)
  │
  └─ Router(handlers, jwtManager, tokenRepo, dbPool, redisClient)
```

---

## Multi-Tenancy

Setiap data credential (user) di-scope ke tenant tertentu:

```
┌─────────────────────────────────────────┐
│            auth_tenants                  │
│  id | code                   | is_active │
│  ── | ────────────────────── | ───────── │
│  A  | tenant_travel_default  | true      │
│  B  | tenant_corporate       | true      │
└─────────────────────────────────────────┘

┌─────────────────────────────────────────┐
│          auth_credentials               │
│  id | tenant_id | email        | ...    │
│  ── | ───────── | ──────────── | ────── │
│  1  | A         | user@ex.com  | ...    │
│  2  | B         | user@ex.com  | ...    │  ← email sama, tenant beda = OK
└─────────────────────────────────────────┘
```

**Implikasi:**
- Login memerlukan `tenant_id` — email yang sama di tenant berbeda = user berbeda
- Roles dan permissions juga per-tenant
- Refresh token menyimpan `tenant_id` dan divalidasi saat refresh
- JWKS berlaku global (key pair yang sama untuk semua tenant)

---

## Authentication & Authorization

### JWT Access Token

- **Algorithm**: RS256 (RSA-SHA256, asymmetric)
- **TTL**: 15 menit (default)
- **Signing**: Private key (hanya di auth service)
- **Verification**: Public key (dishare via JWKS endpoint)

**Claims structure:**
```json
{
  "sub": "credential-uuid",
  "tenant_id": "tenant-uuid",
  "role": "traveler",
  "permissions": ["booking:create", "booking:read"],
  "provider": "email",
  "token_type": "access",
  "jti": "unique-token-id",
  "iat": 1234567890,
  "exp": 1234567890
}
```

### RBAC (Role-Based Access Control)

```
Credential ──(1:1)──→ RoleAssignment ──(N:1)──→ Role ──(N:N)──→ Permission
                           │
                           └── scoped to tenant_id
```

**Hierarchy:**
```
super_admin  → 14 permissions (semua)
admin        → 13 permissions (semua kecuali tenant:manage)
agent        →  8 permissions (booking + user read + payment)
traveler     →  7 permissions (booking own + user own + payment)
```

Permission codes menggunakan format `module:action[:scope]`:
- `booking:create` — CRUD operations
- `booking:read:all` — Elevated scope
- `admin:settings` — Admin-level actions
- `tenant:manage` — Super admin only

---

## Token Management

### Refresh Token Flow

```
                    ┌──────────────────────────────────────┐
                    │           Redis                       │
                    │                                      │
Login ──generates──→│  rt:{credID}:{jti1} = {              │
                    │    credential_id, tenant_id,         │
                    │    token_hash, family_id: "F1",      │
                    │    ip_address, user_agent, created_at │
                    │  }  TTL=30d                          │
                    │                                      │
Refresh ──rotates──→│  DELETE rt:{credID}:{jti1}           │
                    │  SET    rt:{credID}:{jti2} = {...}   │
                    │         (same family_id: "F1")       │
                    │                                      │
Logout ──deletes───→│  DELETE rt:{credID}:{jti2}           │
                    │                                      │
LogoutAll ─────────→│  DELETE rt:{credID}:*                │
                    └──────────────────────────────────────┘
```

**Key design decisions:**
- Refresh token disimpan sebagai **SHA256 hash** (client memegang plaintext, Redis memegang hash)
- **Family-based rotation**: Setiap login session membuat `family_id` baru, rotasi mempertahankan `family_id` yang sama
- **One-time use**: Token lama dihapus sebelum token baru dibuat
- Lookup by hash menggunakan `FindRefreshTokenByHash` (scan `rt:{credID}:*` keys)

### Access Token Blacklisting

```
Logout ──blacklists──→ Redis SET bl:{jti} = "1" TTL=remaining_expiry
                              │
AuthMiddleware ──checks──→────┘  IsAccessTokenBlacklisted(jti)
```

---

## Security Mechanisms

### 1. Rate Limiting

| Endpoint | Max | Window | Redis Key |
|----------|-----|--------|-----------|
| Login | 5 | 15 min | `rate:login:{email}:{tenant_id}` |
| Forgot Password | 3 | 1 hour | `rate:reset:{email}:{tenant_id}` |

Rate limiting diimplementasi di level **use case** menggunakan Redis INCR + TTL.

### 2. Account Lockout

```
failed_login_attempts >= 5  →  locked_until = now + 30 minutes
successful login            →  failed_login_attempts = 0
```

Disimpan di kolom `auth_credentials.failed_login_attempts` dan `locked_until`.

### 3. Password Security

- BCrypt hashing (cost=12 di production, cost=4 di test)
- Minimum 8 karakter (validated di DTO binding)
- Password confirmation required (register, reset, change)

### 4. Token Security

- Refresh token: 32-byte random hex (64 chars), stored as SHA256 hash
- Verification/reset tokens: 32-byte random hex, stored as SHA256 hash
- OAuth state: 32-byte random hex, 10-min TTL
- PKCE code verifier: 43-byte URL-safe random, S256 challenge

### 5. Email Enumeration Prevention

- Register: Returns 409 Conflict (unavoidable, user needs to know)
- Forgot Password: Always returns 200 regardless of email existence
- Login: Generic "Invalid email or password" message

### 6. CORS

```
Allowed Origins:  ["*"]  (configurable)
Allowed Methods:  GET, POST, PUT, PATCH, DELETE, OPTIONS
Allowed Headers:  Origin, Content-Type, Accept, Authorization, X-Request-ID
Credentials:      true
Max Age:          86400s (24h)
```

---

## Error Handling

### Pattern Per Layer

```
Repository Layer:
  return fmt.Errorf("context: %w", err)     ← wrap raw errors

Use Case Layer:
  return apperror.NewUnauthorized("msg")    ← create AppError with HTTP code
  return apperror.NewInternal(err)          ← 500 + wrapped error

Handler Layer:
  handleError(c, err)                       ← maps AppError → HTTP response
```

### AppError Type

```go
type AppError struct {
    Code    int    // HTTP status code
    Message string // User-facing message
    Err     error  // Wrapped internal error (not exposed to client)
}
```

**Factory functions:**

| Function | HTTP Code | Penggunaan |
|----------|-----------|------------|
| `NewBadRequest(msg)` | 400 | Input validation, invalid data |
| `NewUnauthorized(msg)` | 401 | Wrong credentials, invalid token |
| `NewForbidden(msg)` | 403 | Insufficient permissions |
| `NewNotFound(msg)` | 404 | Resource not found |
| `NewConflict(msg)` | 409 | Duplicate email |
| `NewLocked(msg)` | 423 | Account locked |
| `NewTooManyRequests(msg)` | 429 | Rate limit exceeded |
| `NewInternal(err)` | 500 | Internal server error |

### Response Format

```json
{
  "error": "User-facing error message",
  "code": 401,
  "request_id": "uuid-from-middleware"
}
```

`request_id` memudahkan debugging: client bisa melaporkan ID ini, dan backend bisa trace di logs.

---

## Middleware Pipeline

Request melewati middleware secara berurutan:

```
Request
  │
  ▼
┌─────────────┐
│ gin.Recovery │  Panic recovery → 500
└──────┬──────┘
       ▼
┌─────────────┐
│  RequestID  │  Generate/preserve X-Request-ID
└──────┬──────┘
       ▼
┌─────────────┐
│    CORS     │  Set CORS headers, handle OPTIONS preflight
└──────┬──────┘
       ▼
┌─────────────┐
│ RateLimiter │  Global rate limiter (currently passthrough)
└──────┬──────┘
       ▼
┌─────────────┐
│   Router    │  Route matching
└──────┬──────┘
       ▼
  [Protected routes only]
┌──────────────┐
│AuthMiddleware│  JWT validation → blacklist check → set context
└──────┬───────┘
       ▼
┌─────────────┐
│   Handler   │  Business logic execution
└─────────────┘
```

**AuthMiddleware** untuk protected routes:
1. Extract `Authorization` header (accepts with or without `Bearer ` prefix)
2. Validate JWT signature + expiry (RS256)
3. Check blacklist di Redis (`IsAccessTokenBlacklisted`)
4. Set context: `credential_id`, `tenant_id`, `role`, `permissions`, `provider`, `jti`, `token_exp`

---

## Database Design

### ER Diagram

```
auth_tenants
  ├── id (PK, UUID)
  ├── code (UNIQUE)
  ├── name
  ├── is_active
  └── settings (JSONB)
       │
       │ 1:N
       ▼
auth_credentials                         auth_roles
  ├── id (PK, UUID)                        ├── id (PK, UUID)
  ├── tenant_id (FK → tenants) ◄───────── ├── tenant_id (FK → tenants)
  ├── email                                ├── code
  ├── password_hash (nullable)             ├── name
  ├── provider ("email"/"google")          ├── is_default
  ├── provider_id (nullable)               └── is_system
  ├── is_active                                 │
  ├── is_email_verified                         │ N:N
  ├── failed_login_attempts                     ▼
  ├── locked_until                      auth_role_permissions
  └── ...                                 ├── role_id (FK → roles)
       │                                  └── permission_id (FK → permissions)
       │ 1:N                                        │
       ▼                                            ▼
auth_role_assignments                   auth_permissions
  ├── credential_id (FK → credentials)    ├── id (PK, UUID)
  ├── role_id (FK → roles)                ├── code (UNIQUE)
  └── tenant_id (FK → tenants)            ├── name
       │                                  └── module
       │ 1:N
       ▼
auth_login_history
  ├── credential_id (FK, nullable)
  ├── tenant_id (FK)
  ├── login_method
  ├── status ("success"/"failed"/"logout")
  └── failure_reason

auth_password_resets               auth_email_verifications
  ├── credential_id (FK)             ├── credential_id (FK)
  ├── token_hash                     ├── token_hash
  ├── expires_at                     ├── expires_at
  └── used_at (nullable)             └── verified_at (nullable)
```

### Unique Constraints

- `auth_credentials`: UNIQUE(tenant_id, email, provider)
- `auth_credentials`: UNIQUE(tenant_id, provider_id, provider) WHERE provider_id IS NOT NULL
- `auth_roles`: UNIQUE(tenant_id, code)
- `auth_permissions`: UNIQUE(code)

---

## Redis Data Model

| Key Pattern | Tipe | TTL | Keterangan |
|-------------|------|-----|------------|
| `rt:{credID}:{jti}` | Hash/String | 30 days | Refresh token data (JSON) |
| `bl:{jti}` | String | Remaining expiry | Blacklisted access token |
| `oauth:state:{state}` | Hash/String | 10 min | OAuth state + code verifier |
| `rate:login:{email}:{tenantID}` | Counter | 15 min | Login rate limit counter |
| `rate:reset:{email}:{tenantID}` | Counter | 1 hour | Password reset rate limit |
| `rbac:{credID}` | Hash/String | Configurable | Cached role + permissions |

---

## Logging

### Logger Setup

```go
// Development: human-readable, colored, stack traces
logger, _ = zap.NewDevelopment()

// Production: structured JSON, no color
logger, _ = zap.NewProduction()
```

Ditentukan oleh `APP_ENV` environment variable.

### Logging Convention

```go
// Use case layer: log operational events
uc.logger.Info("Password changed", zap.String("credential_id", id))
uc.logger.Error("Failed to record login history", zap.Error(err))

// Non-critical errors: log but don't return error
if err := uc.loginHistRepo.Create(ctx, history); err != nil {
    uc.logger.Error("Failed to record login history", zap.Error(err))
    // login tetap berhasil
}
```

**Pattern**: Business-critical errors → return error ke caller. Non-critical errors (audit logging, cleanup) → log saja.

---

## Testing Strategy

### Approach

```
                    ┌──────────────────────┐
  Unit Tests ──────→│   Use Cases (mocked) │  ← Primary target (business logic)
                    │   Pkg functions       │  ← Pure functions (no deps)
                    │   Middleware          │  ← HTTP behavior
                    │   Handlers           │  ← HTTP ↔ Use Case integration
                    └──────────────────────┘
                    ┌──────────────────────┐
  Not Tested ──────→│   Repositories       │  ← Requires real DB/Redis
  (by design)       │   main.go            │  ← Bootstrap code
                    │   config.go          │  ← Viper loading
                    └──────────────────────┘
```

### Mock Generation

Menggunakan `go.uber.org/mock` (mockgen) untuk generate mock dari repository interfaces:

```
internal/mocks/
  ├── mock_credential_repo.go       ← dari CredentialRepository interface
  ├── mock_tenant_repo.go           ← dari TenantRepository interface
  ├── mock_role_repo.go             ← dari RoleRepository interface
  ├── mock_token_repo.go            ← dari TokenRepository interface
  ├── mock_login_history_repo.go    ← dari LoginHistoryRepository interface
  ├── mock_email_verification_store.go  ← dari EmailVerificationStore interface
  └── mock_password_reset_store.go      ← dari PasswordResetStore interface
```

### Test Helpers

```go
// internal/testutil/testutil.go
GenerateTestRSAKeys(t)   → temp PEM files (2048-bit, fast)
NewTestJWTManager(t)      → JWT Manager with test keys
NewTestLogger()           → zap.NewNop() (silent)
NewTestHasher()           → BCryptHasher cost=4 (fast)
MakeTestCredential()      → Pre-built entity for tests
MakeTestTenant()
MakeTestRole()
```

### Coverage

- **Target**: 85%+ pada testable code
- **Aktual**: ~90% pada kode yang bisa di-test (pkg, usecase, middleware, handler)
- **Excluded**: Repository implementations (butuh real DB), main.go, config, OAuth HTTP calls

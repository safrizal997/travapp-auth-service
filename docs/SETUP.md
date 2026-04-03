# Setup & Deployment Guide

Panduan lengkap untuk menjalankan TravApp Auth Service di lingkungan development dan production.

## Daftar Isi

- [Prerequisites](#prerequisites)
- [Quick Start (Docker)](#quick-start-docker)
- [Manual Setup](#manual-setup)
- [Environment Variables](#environment-variables)
- [RSA Key Generation](#rsa-key-generation)
- [Database Migration](#database-migration)
- [Running the Service](#running-the-service)
- [Verifying the Setup](#verifying-the-setup)
- [Production Deployment](#production-deployment)
- [Troubleshooting](#troubleshooting)

---

## Prerequisites

| Tool | Versi Minimum | Keterangan |
|------|---------------|------------|
| Go | 1.25+ | Runtime & build |
| PostgreSQL | 16+ | Database utama |
| Redis | 7+ | Token storage, rate limiting, caching |
| OpenSSL | - | Generate RSA keys |
| golang-migrate | - | Database migrations |
| Docker & Docker Compose | - | Opsional, untuk containerized setup |

### Install Tools

```bash
# golang-migrate (macOS)
brew install golang-migrate

# golang-migrate (Linux)
curl -L https://github.com/golang-migrate/migrate/releases/download/v4.17.0/migrate.linux-amd64.tar.gz | tar xvz
sudo mv migrate /usr/local/bin/

# golang-migrate (Windows - via scoop)
scoop install migrate

# swag (Swagger generator) — opsional
go install github.com/swaggo/swag/cmd/swag@latest

# mockgen (untuk development/testing)
go install go.uber.org/mock/mockgen@latest
```

---

## Quick Start (Docker)

Cara tercepat untuk menjalankan seluruh stack:

```bash
# 1. Clone repository
git clone <repo-url>
cd travapp-auth-service

# 2. Generate RSA keys
make generate-keys

# 3. Copy dan edit environment file
cp .env.example .env
# Edit .env sesuai kebutuhan

# 4. Jalankan semua service
make docker-up

# 5. Jalankan migration
make migrate-up

# 6. Verifikasi
curl http://localhost:8080/health
```

Docker Compose akan menjalankan:
- **PostgreSQL 16** (port 5433 → 5432 internal)
- **Redis 7** (port 6379)
- **Auth Service** (port 8080)

Untuk menghentikan:
```bash
make docker-down
```

---

## Manual Setup

### 1. Clone & Install Dependencies

```bash
git clone <repo-url>
cd travapp-auth-service
go mod download
```

### 2. Setup PostgreSQL

```bash
# Buat database dan user
psql -U postgres
```

```sql
CREATE USER auth_user WITH PASSWORD 'auth_secret';
CREATE DATABASE auth_db OWNER auth_user;
GRANT ALL PRIVILEGES ON DATABASE auth_db TO auth_user;
```

### 3. Setup Redis

Pastikan Redis berjalan di `localhost:6379` (default). Tidak perlu konfigurasi khusus.

```bash
# Verifikasi Redis
redis-cli ping
# Harus return: PONG
```

### 4. Generate RSA Keys

```bash
make generate-keys
```

Ini akan membuat:
- `keys/private.pem` — RSA 4096-bit private key (untuk sign JWT)
- `keys/public.pem` — RSA public key (untuk verify JWT, dishare via JWKS)

> **PENTING**: Jangan commit file `keys/` ke repository. Pastikan sudah ada di `.gitignore`.

### 5. Konfigurasi Environment

```bash
cp .env.example .env
```

Edit `.env` sesuai setup lokal (lihat [Environment Variables](#environment-variables)).

### 6. Jalankan Migration

```bash
make migrate-up
```

Migration akan membuat tabel-tabel dan seed data default:
- 1 default tenant (`tenant_travel_default`)
- 4 roles: `super_admin`, `admin`, `agent`, `traveler`
- 14 permissions (booking, user, payment, admin modules)
- Role-permission mappings

### 7. Build & Run

```bash
# Development (hot reload tidak tersedia, jalankan manual)
make run

# Atau build binary
make build
./bin/auth-service
```

---

## Environment Variables

### Application

| Variable | Default | Keterangan |
|----------|---------|------------|
| `APP_PORT` | `8080` | Port HTTP server |
| `APP_ENV` | `development` | `development` atau `production` |
| `APP_BASE_URL` | `http://localhost:8080` | Base URL untuk link di email (verification, reset) |

### Database (PostgreSQL)

| Variable | Default | Keterangan |
|----------|---------|------------|
| `DB_HOST` | `localhost` | PostgreSQL host |
| `DB_PORT` | `5432` | PostgreSQL port |
| `DB_USER` | - | Database username |
| `DB_PASSWORD` | - | Database password |
| `DB_NAME` | - | Database name |
| `DB_SSL_MODE` | `disable` | SSL mode (`disable`, `require`, `verify-full`) |
| `DB_MAX_OPEN_CONNS` | `25` | Max open connections |
| `DB_MAX_IDLE_CONNS` | `5` | Max idle connections |
| `DB_CONN_MAX_LIFETIME` | `300` | Connection max lifetime (detik) |

### Redis

| Variable | Default | Keterangan |
|----------|---------|------------|
| `REDIS_HOST` | `localhost` | Redis host |
| `REDIS_PORT` | `6379` | Redis port |
| `REDIS_PASSWORD` | _(kosong)_ | Redis password |
| `REDIS_DB` | `1` | Redis database number |

### JWT

| Variable | Default | Keterangan |
|----------|---------|------------|
| `JWT_PRIVATE_KEY_PATH` | - | Path ke RSA private key PEM |
| `JWT_PUBLIC_KEY_PATH` | - | Path ke RSA public key PEM |
| `JWT_ACCESS_TOKEN_TTL` | `900` | Access token TTL (detik) = 15 menit |
| `JWT_REFRESH_TOKEN_TTL` | `2592000` | Refresh token TTL (detik) = 30 hari |

### Google OAuth

| Variable | Default | Keterangan |
|----------|---------|------------|
| `GOOGLE_CLIENT_ID` | - | Google OAuth Client ID |
| `GOOGLE_CLIENT_SECRET` | - | Google OAuth Client Secret |
| `GOOGLE_REDIRECT_URI` | - | Callback URL (e.g., `http://localhost:8080/api/v1/auth/oauth/google/callback`) |

Untuk mendapatkan credentials Google OAuth:
1. Buka [Google Cloud Console](https://console.cloud.google.com/)
2. Buat project atau pilih project yang ada
3. Aktifkan Google+ API
4. Buat OAuth 2.0 Client ID (Web Application)
5. Tambahkan redirect URI

### BCrypt

| Variable | Default | Keterangan |
|----------|---------|------------|
| `BCRYPT_COST` | `12` | BCrypt hashing cost (4-31). Lebih tinggi = lebih aman tapi lebih lambat |

### Rate Limiting

| Variable | Default | Keterangan |
|----------|---------|------------|
| `RATE_LIMIT_LOGIN_MAX` | `5` | Max login attempts per window |
| `RATE_LIMIT_LOGIN_WINDOW` | `900` | Login rate limit window (detik) = 15 menit |
| `RATE_LIMIT_RESET_MAX` | `3` | Max password reset requests per window |
| `RATE_LIMIT_RESET_WINDOW` | `3600` | Reset rate limit window (detik) = 1 jam |

### Contoh `.env` Lengkap

```env
# Application
APP_PORT=8080
APP_ENV=development
APP_BASE_URL=http://localhost:8080

# Database
DB_HOST=localhost
DB_PORT=5433
DB_USER=auth_user
DB_PASSWORD=auth_secret
DB_NAME=auth_db
DB_SSL_MODE=disable

# Redis
REDIS_HOST=localhost
REDIS_PORT=6379
REDIS_PASSWORD=
REDIS_DB=1

# JWT
JWT_PRIVATE_KEY_PATH=keys/private.pem
JWT_PUBLIC_KEY_PATH=keys/public.pem
JWT_ACCESS_TOKEN_TTL=900
JWT_REFRESH_TOKEN_TTL=2592000

# Google OAuth
GOOGLE_CLIENT_ID=your-client-id.apps.googleusercontent.com
GOOGLE_CLIENT_SECRET=your-client-secret
GOOGLE_REDIRECT_URI=http://localhost:8080/api/v1/auth/oauth/google/callback

# BCrypt
BCRYPT_COST=12

# Rate Limiting
RATE_LIMIT_LOGIN_MAX=5
RATE_LIMIT_LOGIN_WINDOW=900
RATE_LIMIT_RESET_MAX=3
RATE_LIMIT_RESET_WINDOW=3600
```

---

## RSA Key Generation

Auth service menggunakan RSA 4096-bit key pair untuk JWT signing:

```bash
# Menggunakan Makefile
make generate-keys

# Atau manual
mkdir -p keys
openssl genrsa -out keys/private.pem 4096
openssl rsa -in keys/private.pem -pubout -out keys/public.pem
```

**Key distribution:**
- `private.pem` — HANYA di auth service (untuk sign JWT)
- `public.pem` — Bisa di-share ke service lain (untuk verify JWT), atau gunakan JWKS endpoint

---

## Database Migration

### Menjalankan Migration

```bash
# Migrate up (apply all pending migrations)
make migrate-up

# Migrate down (rollback all)
make migrate-down
```

### Urutan Migration

| No | Nama | Keterangan |
|----|------|------------|
| 000001 | `create_auth_tenants` | Tabel tenant (multi-tenant support) |
| 000002 | `create_auth_credentials` | Tabel user credentials |
| 000003 | `create_auth_roles` | Tabel roles per tenant |
| 000004 | `create_auth_permissions` | Tabel permissions |
| 000005 | `create_auth_role_permissions` | Mapping role ↔ permission |
| 000006 | `create_auth_role_assignments` | Mapping credential ↔ role |
| 000007 | `create_auth_login_history` | Log history login/logout |
| 000008 | `create_auth_password_resets` | Token reset password |
| 000009 | `create_auth_email_verifications` | Token verifikasi email |
| 000010 | `seed_default_data` | Seed tenant, roles, permissions |

### Data Default (Seed)

Setelah migration, database berisi:

**Tenant:**
- `tenant_travel_default` — Default Travel Tenant

**Roles:**
| Code | Nama | Default | System |
|------|------|---------|--------|
| `super_admin` | Super Administrator | No | Yes |
| `admin` | Administrator | No | Yes |
| `agent` | Travel Agent | No | No |
| `traveler` | Traveler | **Yes** | Yes |

**Permissions (14 total):**
- `booking:create`, `booking:read`, `booking:update`, `booking:delete`, `booking:read:all`
- `user:read`, `user:read:all`, `user:update`, `user:manage`
- `payment:process`, `payment:refund`
- `admin:settings`, `admin:roles`
- `tenant:manage`

---

## Running the Service

### Development

```bash
# Langsung run
make run

# Atau build lalu jalankan
make build
./bin/auth-service
```

Service akan start di `http://localhost:8080`.

### Startup Flow

Saat service start, urutan inisialisasi:

1. Load config dari `.env` + environment variables
2. Setup logger (Development/Production mode)
3. Connect ke PostgreSQL (connection pool)
4. Connect ke Redis
5. Load RSA keys → JWT Manager
6. Initialize BCrypt hasher
7. Initialize Google OAuth config
8. Wire repositories, usecases, handlers
9. Setup Gin router + middleware (CORS, RequestID, RateLimiter)
10. Start HTTP server dengan graceful shutdown

### Available Endpoints

Setelah running, endpoint yang tersedia:

| Method | Path | Keterangan |
|--------|------|------------|
| GET | `/health` | Health check |
| GET | `/swagger/*any` | Swagger UI |
| POST | `/api/v1/auth/login` | Email login |
| POST | `/api/v1/auth/register` | Register |
| GET | `/api/v1/auth/oauth/google` | Initiate Google OAuth |
| GET | `/api/v1/auth/oauth/google/callback` | Google OAuth callback |
| POST | `/api/v1/auth/token/refresh` | Refresh tokens |
| GET | `/api/v1/auth/.well-known/jwks.json` | Public key (JWKS) |
| GET | `/api/v1/auth/verify-email` | Verify email |
| POST | `/api/v1/auth/password/forgot` | Forgot password |
| POST | `/api/v1/auth/password/reset` | Reset password |
| POST | `/api/v1/auth/logout` | Logout (auth required) |
| POST | `/api/v1/auth/logout/all` | Logout all (auth required) |
| POST | `/api/v1/auth/password/change` | Change password (auth required) |

---

## Verifying the Setup

### 1. Health Check

```bash
curl http://localhost:8080/health
# Expected: {"status":"ok"}
```

### 2. Swagger UI

Buka browser: `http://localhost:8080/swagger/index.html`

### 3. Register User

```bash
# Dapatkan tenant_id dari database terlebih dahulu
psql -h localhost -p 5433 -U auth_user -d auth_db -c \
  "SELECT id FROM auth_tenants WHERE code = 'tenant_travel_default';"

# Register
curl -X POST http://localhost:8080/api/v1/auth/register \
  -H "Content-Type: application/json" \
  -d '{
    "email": "test@example.com",
    "password": "Test1234!",
    "password_confirmation": "Test1234!",
    "tenant_id": "<tenant-id-dari-query>"
  }'
```

### 4. Verify Email

```bash
# Gunakan verification_token dari response register
curl "http://localhost:8080/api/v1/auth/verify-email?token=<verification_token>&email=test@example.com"
```

### 5. Login

```bash
curl -X POST http://localhost:8080/api/v1/auth/login \
  -H "Content-Type: application/json" \
  -d '{
    "email": "test@example.com",
    "password": "Test1234!",
    "tenant_id": "<tenant-id>"
  }'
# Expected: {access_token, refresh_token, token_type, expires_in}
```

### 6. JWKS

```bash
curl http://localhost:8080/api/v1/auth/.well-known/jwks.json
```

---

## Production Deployment

### Checklist

- [ ] Set `APP_ENV=production` (menggunakan structured JSON logging)
- [ ] Generate RSA keys yang kuat (4096-bit) dan simpan di secure location
- [ ] Gunakan strong passwords untuk PostgreSQL dan Redis
- [ ] Set `DB_SSL_MODE=require` atau `verify-full`
- [ ] Set `REDIS_PASSWORD` yang kuat
- [ ] Set `BCRYPT_COST=12` atau lebih tinggi
- [ ] Konfigurasi `GOOGLE_REDIRECT_URI` ke production domain
- [ ] Setup reverse proxy (nginx) di depan service
- [ ] Jangan expose port PostgreSQL dan Redis ke public
- [ ] Setup monitoring dan alerting
- [ ] Backup database secara berkala

### Docker Production Build

```bash
docker build -t auth-service:latest .
```

Image menggunakan multi-stage build:
1. **Builder stage**: `golang:1.25-alpine` — compile binary
2. **Runner stage**: `alpine:3.19` — minimal runtime (~15MB)

Binary dikompilasi dengan `-ldflags="-s -w"` (stripped debug info, smaller size).

### Running Tests

```bash
# Semua tests
make test

# Dengan coverage
go test ./internal/... -coverprofile=coverage.out
go tool cover -func=coverage.out | tail -1

# Coverage report (HTML)
go tool cover -html=coverage.out
```

### Generate Swagger Docs

```bash
make swagger
# Generates docs/swagger.json dan docs/swagger.yaml
```

---

## Troubleshooting

### "connection refused" pada PostgreSQL

```
Pastikan PostgreSQL berjalan dan port benar:
- Docker Compose: port 5433 (external) → 5432 (internal)
- Manual: default port 5432
```

### "WRONGPASS" atau "ERR invalid password" pada Redis

```
Set REDIS_PASSWORD di .env jika Redis menggunakan password.
Untuk development tanpa password, biarkan REDIS_PASSWORD kosong.
```

### "failed to load RSA keys"

```
Pastikan file keys/private.pem dan keys/public.pem ada:
  make generate-keys

Pastikan path di JWT_PRIVATE_KEY_PATH dan JWT_PUBLIC_KEY_PATH benar.
```

### Migration gagal

```
Pastikan database dan user sudah dibuat:
  psql -U postgres -c "CREATE USER auth_user WITH PASSWORD 'auth_secret';"
  psql -U postgres -c "CREATE DATABASE auth_db OWNER auth_user;"

Jika migration stuck (dirty state):
  migrate -path migrations -database "postgres://..." force <version>
```

### "Too many login attempts"

```
Rate limit 5 percobaan per 15 menit. Tunggu 15 menit atau flush Redis:
  redis-cli -n 1 KEYS "rate:login:*"
  redis-cli -n 1 DEL "rate:login:email@example.com:tenant-id"
```

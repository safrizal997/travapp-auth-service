# TravApp Auth Service

Multi-tenant authentication microservice untuk platform TravApp. Dibangun dengan Go (Gin), PostgreSQL, dan Redis.

## Features

- Email/password authentication (register, login, verify email)
- Google OAuth 2.0 dengan PKCE
- JWT access token (RS256) + refresh token rotation
- RBAC (Role-Based Access Control) per tenant
- JWKS endpoint untuk verifikasi token oleh service lain
- Password reset & change flow
- Rate limiting & account lockout
- Multi-session logout

## Tech Stack

| Komponen | Teknologi |
|----------|-----------|
| Language | Go 1.25 |
| Framework | Gin |
| Database | PostgreSQL (pgx/v5) |
| Cache | Redis (go-redis/v9) |
| JWT | golang-jwt/v5 (RS256) |
| Docs | Swagger (swaggo/swag) |

## Quick Start

```bash
# Generate RSA keys
make generate-keys

# Start database & Redis
make docker-up

# Run migrations
make migrate-up

# Run service
make run
```

Service berjalan di `http://localhost:8080`
Swagger UI di `http://localhost:8080/swagger/index.html`

## Documentation

| Dokumen | Keterangan |
|---------|------------|
| [API Reference](docs/API.md) | Endpoint, request/response, error codes |
| [Database](docs/DATABASE.md) | Schema PostgreSQL & Redis key patterns |
| [Multi-Tenancy](docs/MULTITENANCY.md) | Arsitektur & isolasi multi-tenant |
| [Security](docs/SECURITY.md) | Mekanisme keamanan & konfigurasi |

## Makefile Commands

```bash
make build            # Build binary
make run              # Run service
make test             # Run tests
make swagger          # Generate Swagger docs
make migrate-up       # Run migrations
make migrate-down     # Rollback migrations
make docker-up        # Start PostgreSQL & Redis
make docker-down      # Stop containers
make generate-keys    # Generate RSA 4096-bit key pair
```

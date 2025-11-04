# VTB Multibank Service App - AI Agent Instructions

## Project Overview

This is a Go backend service for VTB Multibank hackathon project using Fiber web framework. The service integrates with VTB's Virtual Bank API and provides a multibank aggregation platform. Currently in early development with auth feature branch active.

## Architecture & Structure

```
backend/
├── cmd/main.go              # Application entry point
├── config/                  # YAML configs (local.yaml, prod.yaml)
├── internal/
│   ├── config/              # Config loading & validation
│   ├── handlers/            # HTTP handlers (empty - pending impl)
│   ├── models/              # Domain models (empty - pending impl)
│   ├── services/auth/       # Auth service (empty - pending impl)
│   └── clients/             # External API clients (empty - pending impl)
└── pkg/logger/              # Zerolog wrapper with custom formatting

frontend/                    # Empty - pending implementation
```

**Key Design Pattern**: Clean architecture with domain-driven layers. Each `internal/*` package is isolated.

## Critical Configuration Setup

**MUST-HAVE for running the app**:

1. **Environment file**: Copy `.env.example` to `.env` in `/backend` directory
2. **CONFIG_PATH**: Must be set (e.g., `config/local.yaml`)
3. **Required env vars**: `CLIENT_ID`, `CLIENT_SECRET`, `POSTGRES_URL`

The config system uses **dual-source loading**:
- YAML file for base config (via `CONFIG_PATH` env var)
- Environment variables for secrets/overrides
- Package: `github.com/ilyakaznacheev/cleanenv`

## Running & Building

**Development** (PowerShell):
```powershell
cd backend
go mod tidy
go run .\cmd\main.go
```

**Testing**:
```powershell
cd backend
go test .\internal\config\...  # Config package tests
```

**Build**:
```powershell
cd backend
go build -o bin/server.exe .\cmd\main.go
```

## Code Conventions

### Logging
- Use **zerolog** for all logging (not standard `log`)
- Custom logger helpers: `logger.LogServerStart()`, `logger.LogServerListening()`
- Format: JSON for prod (`LOG_FORMAT=json`), console for dev (`LOG_FORMAT=console`)
- Levels: 0=trace, 1=debug, 2=info (see `.env.example`)

### Configuration
- All configs use struct tags: `yaml:"field" env:"ENV_VAR" env-default:"value"`
- Helper methods on `HTTPServer`: `GetHost()`, `GetPort()` parse the `address` field
- Call `config.Init()` before `config.MustLoad()` to load `.env` file

### HTTP Server
- Framework: **Fiber v2** (not standard `net/http`)
- Middleware: `fiberzerolog` for request logging, `recover` for panic recovery
- Route grouping: Use `app.Group("/prefix")` pattern (see `/auth` group in `main.go`)

### Import Paths
- Module: `github.com/kirill010106/multibank_service_app/backend`
- Internal imports: `github.com/kirill010106/multibank_service_app/backend/internal/...`

## External Dependencies

- **VTB Virtual Bank API**: `https://vbank.open.bankingapi.ru` (see `config/*.yaml`)
- **PostgreSQL**: Connection string in `POSTGRES_URL` env var (field: `storage_url` in yaml)
- **Go Fiber**: v2.52.9 with fasthttp under the hood
- **Zerolog**: Structured logging library

## Common Pitfalls

1. **Don't use `log` package** - use `zerolog` logger instance
2. **Environment file location**: Must be `/backend/.env` (not repo root)
3. **CONFIG_PATH is mandatory** - app will `log.Fatal` if not set
4. **Test naming**: Use table-driven tests with `t.Run(tt.name, ...)` pattern (see `config_test.go`)
5. **Fiber != net/http** - Use `fiber.Ctx` not `http.ResponseWriter`

## Development Context

- **Current branch**: `feature/backend-auth` - implementing authentication
- **Empty directories**: `handlers/`, `models/`, `services/auth/`, `clients/`, `frontend/` are scaffolded but not implemented
- **Next steps**: Auth service implementation, database integration, VTB API client

## Testing Approach

- Unit tests colocated with code (`config_test.go` in `internal/config/`)
- Table-driven test pattern with descriptive test names
- Run tests with: `go test ./...` from `/backend`

---

When implementing new features:
1. Follow the layered architecture (handlers → services → clients)
2. Add appropriate zerolog logging statements
3. Use Fiber's context methods (`c.JSON()`, `c.Status()`, etc.)
4. Update YAML configs for new environment-specific settings
5. Add unit tests using table-driven approach

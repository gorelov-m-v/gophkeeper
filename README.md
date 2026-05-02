# GophKeeper

GophKeeper is a Go password manager with a `gRPC + PostgreSQL` server and a cross-platform CLI client. The client also ships with an optional TUI built on Bubble Tea.

The current architecture follows a `cmd -> config -> handler/service/repository` layout similar to `practicum-metrics`, uses `zap` for infrastructure logging, and keeps secret contents opaque to the server.

## What Is Stored

Supported secret types:

- login/password pairs
- arbitrary text
- arbitrary binary blobs
- bank card data

Each secret also carries free-form metadata, but metadata is encrypted together with the typed secret body inside the client-side payload envelope.

## Security Model

- Transport security is enabled by default. The server expects TLS certificates unless `--allow-insecure-grpc=true` is explicitly set.
- Secret encryption happens on the client. The server stores only encrypted `payload` bytes and does not see decrypted secret bodies or metadata.
- The client derives an AES-256-GCM key from the master password with Argon2id.
- Authentication uses JWT access/refresh tokens.

## Sync Model

The client keeps a per-user local cache under the client config directory:

- `cache/<user_id>/secrets.json` contains cached secrets
- `cache/<user_id>/sync.json` contains the last sync cursor

Behavior:

- `sync` performs a delta request using the stored cursor
- `add`, `update`, and `delete` write to the server first and then mirror the result into the local cache
- `list` and `get` read from the local cache only
- optimistic locking conflicts are returned as “run sync first” errors

## Project Layout

- [cmd/client/main.go](/C:/Users/User1/Desktop/gophkeeper/cmd/client/main.go): CLI entrypoint
- [cmd/server/main.go](/C:/Users/User1/Desktop/gophkeeper/cmd/server/main.go): server entrypoint
- [cmd/staticlint/main.go](/C:/Users/User1/Desktop/gophkeeper/cmd/staticlint/main.go): custom lint runner
- [api/proto/auth.proto](/C:/Users/User1/Desktop/gophkeeper/api/proto/auth.proto): auth API
- [api/proto/secrets.proto](/C:/Users/User1/Desktop/gophkeeper/api/proto/secrets.proto): secrets API
- [internal/server/app/app.go](/C:/Users/User1/Desktop/gophkeeper/internal/server/app/app.go): server wiring
- [internal/client/store/store.go](/C:/Users/User1/Desktop/gophkeeper/internal/client/store/store.go): local cache and sync cursor storage
- [internal/client/tui/app.go](/C:/Users/User1/Desktop/gophkeeper/internal/client/tui/app.go): TUI app shell

## Configuration

Both client and server use the same precedence order:

`defaults -> JSON config -> env -> flags`

### Server

Supported settings:

- `grpc_address`
- `database_dsn`
- `migrations_path`
- `jwt_secret`
- `jwt_access_ttl`
- `jwt_refresh_ttl`
- `tls_cert_file`
- `tls_key_file`
- `allow_insecure_grpc`

Example:

```json
{
  "grpc_address": "localhost:3200",
  "database_dsn": "postgres://postgres:postgres@localhost:5432/gophkeeper?sslmode=disable",
  "migrations_path": "migrations",
  "jwt_secret": "change-me-please-123456",
  "jwt_access_ttl": "15m",
  "jwt_refresh_ttl": "168h",
  "allow_insecure_grpc": true
}
```

Run locally without TLS:

```bash
go run ./cmd/server \
  --database-dsn "postgres://postgres:postgres@localhost:5432/gophkeeper?sslmode=disable" \
  --jwt-secret "change-me-please-123456" \
  --allow-insecure-grpc=true
```

### Client

Supported settings:

- `server_address`
- `config_dir`
- `tls_ca_file`
- `insecure`

Example:

```json
{
  "server_address": "localhost:3200",
  "config_dir": ".gophkeeper",
  "insecure": true
}
```

## Common Commands

Register and set a master password:

```bash
go run ./cmd/client --insecure register --email user@example.com
```

Add a secret and sync:

```bash
go run ./cmd/client --insecure add --type text --name note --text "hello"
go run ./cmd/client --insecure sync
```

Read local cache and launch TUI:

```bash
go run ./cmd/client --insecure list
go run ./cmd/client --insecure get note
go run ./cmd/client --insecure tui
```

Print build metadata:

```bash
go run ./cmd/client version
```

## Build

Client build info is injected via `ldflags`:

```bash
go build -ldflags "-X github.com/user/gophkeeper/internal/version.Version=1.0.0 -X github.com/user/gophkeeper/internal/version.Date=2026-04-24T00:00:00Z -X github.com/user/gophkeeper/internal/version.Commit=$(git rev-parse --short HEAD)" -o bin/gophkeeper ./cmd/client
```

The CI workflow builds the client on Linux, Windows, and macOS.

## Quality Gates

Local commands:

```bash
go test ./...
go test -coverprofile=coverage.out ./...
go tool cover -func=coverage.out
go run ./cmd/staticlint ./...
```

Notes:

- `go test -race ./...` requires `CGO_ENABLED=1`
- current total statement coverage is above 70%

## TUI

The TUI is an optional shell over the same local cache and sync layer used by the CLI:

- `s` syncs from the server
- `n` creates a secret
- `enter` opens details from the local cache
- `d` deletes the selected secret after confirmation

## Development Notes

- After proto changes, regenerate code with `buf generate`
- Database migrations live in [migrations/000002_create_secrets.up.sql](/C:/Users/User1/Desktop/gophkeeper/migrations/000002_create_secrets.up.sql)
- The active schema stores only server-visible attributes and encrypted payload bytes

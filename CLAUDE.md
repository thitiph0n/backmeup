# backmeup

Scheduled backup tool. Supports postgres/mysql/minio → local storage. Cron-driven, YAML config, optional HTTP server for health/metrics.

## Module

`github.com/username/backmeup` — Go 1.23, toolchain 1.24.2

## Commands

```sh
make dev          # go run cmd/backmeup/main.go
make test         # go test -v ./...
make ittest-up    # docker-compose up integration test env
make ittest-down  # tear down integration test env
```

## Package layout

| Package | Role |
|---|---|
| `internal/config` | Load/validate YAML config, env var interpolation `${VAR}` |
| `internal/backup` | `Executor` interface + postgres/mysql/minio impls |
| `internal/scheduler` | gocron wrapper, job status callbacks |
| `internal/server` | HTTP server — `/health`, `/metrics` |
| `internal/retention` | Apply count/days retention after backup |
| `internal/notification` | Discord + webhook notifications |
| `internal/storage` | Local filesystem helpers |

## Config structure

```yaml
version: "1"
server:
  enabled: true
  port: 8080
storage:
  type: local      # only "local" supported
  local:
    directory: /backups
    max_size: 10GB
jobs:
  - name: my-db
    type: postgres  # postgres | mysql | minio
    schedule: "0 2 * * *"
    retention_policy:
      type: count   # count | days
      value: 7
    notification:
      enabled: false
```

## Conventions

- No `any` — use concrete types or generics
- No inline comments unless logic is non-obvious
- Always end files with newline
- New backup type → implement `backup.Executor`, register in `backup.CreateExecutor`
- New storage type → add to `config.StorageConfig`, update `config.Validate()` and `backup.BaseExecutor.GetBackupDestination()`

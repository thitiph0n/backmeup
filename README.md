![Back Me Up Cover](cover-image.svg)

# Back Me Up

Lightweight, configuration-driven backup tool for databases and object storage.

## Features

- YAML config — GitOps friendly, version-controllable
- **Sources**: PostgreSQL (`pg_dump`), MySQL (`mysqldump`), MinIO (`mc mirror`)
- **Scheduling**: cron syntax per job
- **Retention**: count-based or days-based cleanup
- **Storage**: local filesystem
- **HTTP server**: `/health` and `/metrics` (JSON)
- Graceful shutdown — waits for in-progress backups (5 min grace period)

## Installation

### Docker Compose

```yaml
services:
  backmeup:
    image: username/backmeup:latest
    restart: unless-stopped
    ports:
      - "8080:8080"
    volumes:
      - ./config/config.yaml:/app/config/config.yaml
      - ./backups:/app/backups
    environment:
      - POSTGRES_PASSWORD=securepassword
      - MYSQL_PASSWORD=securepassword
      - MINIO_ACCESS_KEY=accesskey
      - MINIO_SECRET_KEY=secretkey
```

```bash
docker-compose up -d
```

### Docker

```bash
docker run -p 8080:8080 \
  -v /path/to/config.yaml:/app/config/config.yaml \
  -v /path/to/backups:/app/backups \
  username/backmeup:latest
```

### From Source

```bash
git clone https://github.com/thitiph0n/backmeup.git
cd backmeup
go mod download
go build -o backmeup cmd/main.go
./backmeup --config /path/to/config.yaml
```

## Configuration

Full example at `docs/example-config.yml`.

```yaml
version: "1"
server:
  enabled: true
  port: 8080

storage:
  type: local
  local:
    directory: /backups
    max_size: 100GB

jobs:
  - name: my-postgres
    type: postgres
    postgres_config:
      connection_string: "postgresql://user:password@localhost:5432/dbname"
    schedule: "0 0 * * *"
    retention_policy:
      type: count
      value: 5
    notification:
      enabled: true
      discord:
        when: [success, failure]
        webhook_url: "https://discord.com/api/webhooks/..."
```

## Development

```bash
make dev        # go run cmd/backmeup/main.go
make test       # go test -v ./...
make ittest-up  # start integration test env
```

## Contributing

1. Fork the project
2. Create a feature branch (`git checkout -b feature/my-feature`)
3. Commit changes and open a Pull Request

## License

MIT — see [LICENSE](LICENSE).

# Back Me Up

A lightweight, configuration-driven backup management solution for your databases and object storage.

## Overview

Back Me Up provides a simple, GitOps-friendly tool to create, manage, and monitor backups for your critical data sources. Supporting PostgreSQL, MySQL, and MinIO out of the box, it offers flexible scheduling, retention policies, and detailed backup history tracking.

## Features

- **GitOps-Friendly Configuration**: Define all backup jobs in YAML configuration for version control
- **Multiple Data Sources**:
  - PostgreSQL databases
  - MySQL databases
  - MinIO object storage
- **Flexible Backup Policies**:
  - Custom scheduling using cron syntax
  - File retention management
  - Configurable snapshot count
- **State Management**: SQLite-based tracking of backup operations
- **Graceful Handling**: Proper handling of in-progress backups during deployments
- **Backup History**: Track all backup operations with detailed logs and statistics
- **Monitoring & Alerting**:
  - Storage capacity monitoring with soft limits
  - Prometheus metrics endpoint for monitoring
  - Discord notifications for backup status
  - Webhook support for integration with other services

## Tech Stack

- Go
- SQLite (for state tracking)
- gocron (scheduler)

## Installation

### Using Docker Compose (recommended)

```yaml
# docker-compose.yml
version: "3.8"

services:
  backmeup:
    image: username/backmeup:latest
    container_name: backmeup
    restart: unless-stopped
    ports:
      - "8080:8080"
    volumes:
      - ./config/config.yaml:/app/config/config.yaml
      - ./backups:/app/backups
      - ./data:/app/data
    environment:
      - POSTGRES_PASSWORD=securepassword
      - MYSQL_PASSWORD=anothersecurepassword
      - MINIO_ACCESS_KEY=accesskey
      - MINIO_SECRET_KEY=secretkey
      - DISCORD_ALERT_WEBHOOK_URL=https://discord.com/api/webhooks/...
      - DISCORD_SUCCESS_WEBHOOK_URL=https://discord.com/api/webhooks/...
      - DISCORD_STORAGE_WEBHOOK_URL=https://discord.com/api/webhooks/...
      - WEBHOOK_AUTH_TOKEN=auth_token_for_external_service
    networks:
      - backup-network

networks:
  backup-network:
    name: backup-network
```

Run with:

```bash
docker-compose up -d
```

### Using Docker (alternative)

```bash
docker run -p 8080:8080 \
  -v /path/to/config.yaml:/app/config/config.yaml \
  -v /path/to/backups:/app/backups \
  -v /path/to/state:/app/data \
  username/backmeup:latest
```

### Manual Installation

```bash
# Clone the repository
git clone https://github.com/username/backmeup.git
cd backmeup

# Install dependencies
go mod download

# Build the application
go build -o backmeup cmd/main.go

# Run the application
./backmeup --config /path/to/config.yaml
```

## Configuration

Create a `config.yaml` file following our configuration format. An example configuration file is provided in the `docs/` directory.

### Example Configuration

Below is an example of the configuration format:

```yaml
version: "1.0"
server:
  enabled: true
  port: 8080

storage:
  type: local
  local:
    directory: /path/to/storage
    max_size: 100GB

jobs:
  - name: "example job"
    description: "This is an example job"
    type: "postgres"
    postgres_config:
      connection_string: "postgresql://user:password@localhost:5432/dbname"
    schedule: "0 0 * * *"
    retention_policy:
      type: "count"
      value: 5
    notification:
      enabled: true
      discord:
        when:
          - "success"
          - "failure"
        webhook_url: "https://discord.com/api/webhooks/..."
```

This configuration:

- Sets up a server on port 8080
- Configures local storage with a maximum size of 100GB
- Creates a PostgreSQL backup job that runs daily at midnight
- Keeps 5 most recent backups
- Sends Discord notifications on success or failure

You can find the full example configuration file at `docs/example-config.yml`

## Implementation Details

### Backup Commands

#### PostgreSQL

Uses `pg_dump` with optimization flags suitable for medium-sized databases.

#### MySQL

Uses `mysqldump` with transaction consistency and piped compression.

#### MinIO

Uses `mc mirror --preserve` for efficient object storage backup with metadata preservation.

### State Management

Back Me Up uses SQLite to track:

- Backup job history
- Running/completed/failed states
- File sizes and durations
- Retention cleanup operations

### Monitoring & Notifications

#### Storage Monitoring

- Monitors disk usage of backup storage
- Alerts when usage exceeds configured soft limits
- Exposes Prometheus metrics at `/metrics` endpoint

#### Discord Integration

- Sends notifications to multiple Discord channels
- Channel-specific job filtering (notify different channels about different jobs)
- Customizable event triggers per channel (success, failure, warnings, storage alerts)
- Includes job details, duration, error messages, and storage metrics

#### Webhook Support

- HTTP POST callbacks to external services
- Configurable payload format and headers
- Supports authentication via header tokens

### Deployment Considerations

- Container restarts will gracefully wait for in-progress backups
- Failed or interrupted backups are properly recorded
- Default grace period of 5 minutes for backups to complete during shutdown

## Development

```bash
cd backmeup
go run cmd/main.go --config dev-config.yaml
```

## Contributing

Contributions are welcome! Please feel free to submit a Pull Request.

1. Fork the project
2. Create your feature branch (`git checkout -b feature/amazing-feature`)
3. Commit your changes (`git commit -m 'Add some amazing feature'`)
4. Push to the branch (`git push origin feature/amazing-feature`)
5. Open a Pull Request

## License

This project is licensed under the MIT License - see the LICENSE file for details.

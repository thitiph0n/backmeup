# BackMeUp - Backup Management Tool

BackMeUp is a flexible and robust backup management tool designed to simplify database backups, particularly PostgreSQL and MySQL databases.

## Table of Contents

1. [Quick Start](#quick-start)
2. [Running with Docker](#running-with-docker)
3. [Configuration](#configuration)
4. [PostgreSQL Backups](#postgresql-backups)
5. [Backup Storage Options](#backup-storage-options)
6. [Scheduling](#scheduling)
7. [Notification System](#notification-system)
8. [Retention Policies](#retention-policies)
9. [Monitoring and Healthchecks](#monitoring-and-healthchecks)
10. [MinIO Backups and Restoration](#minio-backups-and-restoration)
11. [PostgreSQL Backups and Restoration](#postgresql-backups-and-restoration)
12. [MySQL Backups and Restoration](#mysql-backups-and-restoration)

## Quick Start

1. Clone the repository
2. Create a configuration file based on the example
3. Run the application

```bash
# Copy the example configuration file
cp example/config.example.yml config.yml

# Edit the configuration to match your requirements
vim config.yml

# Run the application
./backmeup -config config.yml
```

## Running with Docker

BackMeUp provides an official Docker image for easy deployment:

```bash
# Pull the image
docker pull username/backmeup:latest

# Run with a custom configuration file
docker run -v $(pwd)/config.yml:/app/config.yml -v $(pwd)/backups:/backups username/backmeup:latest
```

Or build your own:

```bash
# Build the Docker image
docker build -t backmeup:latest .

# Run with a custom configuration file
docker run -v $(pwd)/config.yml:/app/config.yml -v $(pwd)/backups:/backups backmeup:latest
```

## Configuration

BackMeUp uses a YAML configuration file to define backup jobs, storage options, and notification settings. Here's an example configuration:

```yaml
version: "1.0"
server:
  enabled: true
  port: 8080

storage:
  type: local
  local:
    directory: /backups
    max_size: 100GB

jobs:
  - name: "postgres_daily_backup"
    description: "Daily PostgreSQL database backup"
    type: "postgres"
    postgres_config:
      host: "localhost"
      port: "5432"
      user: "postgres"
      password: "${POSTGRES_PASSWORD}"
      database: "mydb"
      options:
        schema-only: "" # No value needed for boolean flags
        exclude-table: "logs" # Exclude specific tables
    schedule: "0 0 * * *" # Run at midnight every day
    retention_policy:
      type: "count"
      value: 7 # Keep last 7 backups
    notification:
      enabled: true
      discord:
        when:
          - "success"
          - "failure"
        webhook_url: "${DISCORD_WEBHOOK_URL}"
```

## PostgreSQL Backups

BackMeUp supports PostgreSQL database backups using the following configuration:

```yaml
jobs:
  - name: "postgres_backup"
    type: "postgres"
    postgres_config:
      host: "db.example.com" # Database host
      port: "5432" # Database port
      user: "postgres" # Database username
      password: "secret" # Database password or ${ENV_VAR}
      database: "mydatabase" # Database name
      options: # Additional pg_dump options
        schema-only: "" # Backup schema only, no data
        exclude-table: "logs" # Exclude specific tables
        format: "custom" # Use custom format (c|d|t|p)
```

The `options` field allows you to specify any pg_dump option. Options without values should use an empty string (`""`).

BackMeUp uses the `pg_dump` command-line tool, so make sure it's available in your environment or use the provided Docker image.

## Backup Storage Options

BackMeUp supports multiple storage backends:

### Local Storage

```yaml
storage:
  type: local
  local:
    directory: /path/to/backups
    max_size: 100GB
```

### MinIO / S3 Compatible Storage

```yaml
storage:
  type: s3
  s3:
    endpoint: "s3.amazonaws.com"
    access_key: "${S3_ACCESS_KEY}"
    secret_key: "${S3_SECRET_KEY}"
    bucket: "my-backups"
    region: "us-east-1"
    secure: true
```

## Scheduling

BackMeUp uses cron expressions for scheduling backups:

```yaml
schedule: "0 0 * * *" # Run at midnight every day
```

Common cron expressions:

- `0 0 * * *` - Daily at midnight
- `0 0 * * 0` - Weekly on Sunday at midnight
- `0 0 1 * *` - Monthly on the 1st at midnight
- `0 */6 * * *` - Every 6 hours

## Notification System

BackMeUp supports sending notifications for backup status:

```yaml
notification:
  enabled: true
  discord:
    when:
      - "success"
      - "failure"
    webhook_url: "https://discord.com/api/webhooks/..."
```

## Retention Policies

Control how many backups are kept with retention policies:

```yaml
retention_policy:
  type: "count" # Keep a specific number of backups
  value: 7 # Keep last 7 backups
```

```yaml
retention_policy:
  type: "days" # Keep backups for a number of days
  value: 30 # Keep backups for 30 days
```

## Monitoring and Healthchecks

BackMeUp provides an HTTP server for monitoring and healthchecks:

```yaml
server:
  enabled: true
  port: 8080
```

Endpoints:

- `/health` - Returns 200 OK if the application is running
- `/metrics` - Returns Prometheus-compatible metrics
- `/jobs` - Returns information about configured jobs

You can disable the server by setting `server.enabled` to `false`.

## MinIO Backups and Restoration

BackMeUp supports backing up MinIO object storage using the MinIO Client (mc) tool.

### Configuration

```yaml
jobs:
  - name: "minio_backup"
    description: "Daily MinIO bucket backup"
    type: "minio"
    minio_config:
      endpoint: "minio.example.com:9000"
      access_key: "${MINIO_ACCESS_KEY}"
      secret_key: "${MINIO_SECRET_KEY}"
      bucket_name: "my-bucket"
      use_ssl: true
      source_folder: "data" # Optional: backup only a specific folder in the bucket
    schedule: "0 0 * * *" # Run at midnight every day
    retention_policy:
      type: "count"
      value: 7 # Keep last 7 backups
```

### Backup Process

When the MinIO backup job executes, it:

1. Creates a timestamped directory for the backup
2. Configures the MinIO Client (mc) with your server credentials
3. Uses `mc mirror --preserve` to create an exact copy of all files from the specified bucket/folder while maintaining all metadata and file attributes

### How to Restore from MinIO Backup

To restore data from a MinIO backup:

1. **Install MinIO Client**: If not already installed

   ```bash
   # For macOS
   brew install minio/stable/mc

   # For Linux
   wget https://dl.min.io/client/mc/release/linux-amd64/mc
   chmod +x mc
   sudo mv mc /usr/local/bin/
   ```

2. **Configure MinIO Client**:

   ```bash
   mc alias set myminio https://minio.example.com:9000 ACCESSKEY SECRETKEY
   ```

3. **Locate your backup**: Find the backup directory you want to restore from

   ```bash
   ls /backups/{job_name}/
   ```

4. **Mirror files back to MinIO**:

   ```bash
   mc mirror --preserve /backups/{job_name}/minio_backup_{timestamp}/ myminio/bucket/
   ```

5. **Verify the restoration**: List files in your bucket to confirm

   ```bash
   mc ls myminio/bucket/
   ```

For selective restoration, you can specify specific paths:

```bash
mc mirror --preserve /backups/{job_name}/minio_backup_{timestamp}/specific/folder/ myminio/bucket/specific/folder/
```

## PostgreSQL Backups and Restoration

BackMeUp creates PostgreSQL backups using the `pg_dump` utility.

### How to Restore PostgreSQL Backup

To restore a PostgreSQL backup:

1. **Locate your backup**: Find the SQL dump file

   ```bash
   ls /backups/{job_name}/
   ```

2. **Restore using psql**:

   ```bash
   psql -h hostname -U username -d database_name -f /backups/{job_name}/pg_backup_{timestamp}.sql
   ```

   Or using the pg_restore tool (for custom format backups):

   ```bash
   pg_restore -h hostname -U username -d database_name /backups/{job_name}/pg_backup_{timestamp}.dump
   ```

## MySQL Backups and Restoration

BackMeUp creates MySQL backups using the `mysqldump` utility.

### How to Restore MySQL Backup

To restore a MySQL backup:

1. **Locate your backup**: Find the SQL dump file

   ```bash
   ls /backups/{job_name}/
   ```

2. **Restore using mysql client**:

   ```bash
   mysql -h hostname -u username -p database_name < /backups/{job_name}/mysql_backup_{timestamp}.sql
   ```

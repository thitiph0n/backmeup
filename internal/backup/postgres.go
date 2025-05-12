package backup

import (
	"context"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"

	"github.com/username/backmeup/internal/config"
)

// PostgresExecutor implements backup execution for PostgreSQL databases
type PostgresExecutor struct {
	BaseExecutor
}

// NewPostgresExecutor creates a new PostgreSQL backup executor
func NewPostgresExecutor(jobConfig config.JobConfig, storageConfig config.StorageConfig) (Executor, error) {
	if jobConfig.PostgresConfig == nil {
		return nil, fmt.Errorf("missing PostgreSQL configuration for job: %s", jobConfig.Name)
	}

	return &PostgresExecutor{
		BaseExecutor: BaseExecutor{
			Config:        jobConfig,
			StorageConfig: storageConfig,
		},
	}, nil
}

// Execute performs a PostgreSQL database backup
func (p *PostgresExecutor) Execute(ctx context.Context) error {
	p.LogBackupInfo("Starting PostgreSQL backup")

	// Generate a filename for the backup
	filename := p.GenerateBackupFileName("pg_backup", ".sql")

	// Build the full path where the backup will be stored
	backupPath, err := p.BuildBackupFilePath(filename)
	if err != nil {
		return fmt.Errorf("failed to prepare backup path: %w", err)
	}

	// Create the output file
	backupFile, err := os.Create(backupPath)
	if err != nil {
		return fmt.Errorf("failed to create backup file: %w", err)
	}
	defer backupFile.Close()

	// Set up the pg_dump command with connection parameters
	cmdArgs := []string{}

	// Initialize connection parameters
	host := p.Config.PostgresConfig.Host
	port := p.Config.PostgresConfig.Port
	user := p.Config.PostgresConfig.User
	password := p.Config.PostgresConfig.Password
	dbname := p.Config.PostgresConfig.Database

	// Add connection parameters to command
	cmdArgs = append(cmdArgs, "-h", host)

	if port != "" {
		cmdArgs = append(cmdArgs, "-p", port)
	} else {
		// Default PostgreSQL port if not specified
		cmdArgs = append(cmdArgs, "-p", "5432")
	}

	if user != "" {
		cmdArgs = append(cmdArgs, "-U", user)
	}

	cmdArgs = append(cmdArgs, "-d", dbname)

	// Add common pg_dump options for better backups
	cmdArgs = append(cmdArgs,
		"--no-password", // Never prompt for password (use PGPASSWORD env var)
		"--clean",       // Add DROP statements
		"--if-exists",   // Use IF EXISTS with DROP statements
		"--no-owner",    // Skip commands to set ownership
		"--compress=9",  // Maximum compression level
	)

	// Apply any additional options from the configuration
	for key, value := range p.Config.PostgresConfig.Options {
		if value == "" {
			cmdArgs = append(cmdArgs, fmt.Sprintf("--%s", key))
		} else {
			cmdArgs = append(cmdArgs, fmt.Sprintf("--%s=%s", key, value))
		}
	}

	// Create environment for the command
	env := os.Environ()
	if password != "" {
		env = append(env, fmt.Sprintf("PGPASSWORD=%s", password))
	}

	// Set up the pg_dump command
	cmd := exec.CommandContext(ctx, "pg_dump", cmdArgs...)
	cmd.Env = env
	cmd.Stdout = backupFile
	cmd.Stderr = os.Stderr

	// Execute the pg_dump command
	p.LogBackupInfo(fmt.Sprintf("Running pg_dump to %s", backupPath))
	if err := cmd.Run(); err != nil {
		return fmt.Errorf("pg_dump failed: %w", err)
	}

	// Check if the backup file was created successfully
	info, err := os.Stat(backupPath)
	if err != nil {
		return fmt.Errorf("failed to verify backup file: %w", err)
	}

	p.LogBackupInfo(fmt.Sprintf("PostgreSQL backup completed successfully: %s (%.2f MB)",
		filepath.Base(backupPath), float64(info.Size())/(1024*1024)))

	return nil
}

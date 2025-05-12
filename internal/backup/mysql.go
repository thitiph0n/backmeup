package backup

import (
	"context"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"

	"github.com/username/backmeup/internal/config"
)

// MySQLExecutor implements backup execution for MySQL databases
type MySQLExecutor struct {
	BaseExecutor
}

// NewMySQLExecutor creates a new MySQL backup executor
func NewMySQLExecutor(jobConfig config.JobConfig, storageConfig config.StorageConfig) (Executor, error) {
	if jobConfig.MySQLConfig == nil {
		return nil, fmt.Errorf("missing MySQL configuration for job: %s", jobConfig.Name)
	}

	return &MySQLExecutor{
		BaseExecutor: BaseExecutor{
			Config:        jobConfig,
			StorageConfig: storageConfig,
		},
	}, nil
}

// Execute performs a MySQL database backup
func (m *MySQLExecutor) Execute(ctx context.Context) error {
	m.LogBackupInfo("Starting MySQL backup")

	// Generate a filename for the backup
	filename := m.GenerateBackupFileName("mysql_backup", ".sql")

	// Build the full path where the backup will be stored
	backupPath, err := m.BuildBackupFilePath(filename)
	if err != nil {
		return fmt.Errorf("failed to prepare backup path: %w", err)
	}

	// Parse the connection string to extract credentials
	// Assume format: "mysql://user:pass@host:port/dbname"
	connStr := m.Config.MySQLConfig.ConnectionString

	// Extract database name from connection string
	parts := strings.Split(connStr, "/")
	if len(parts) < 2 {
		return fmt.Errorf("invalid MySQL connection string format")
	}
	dbName := parts[len(parts)-1]

	// Extract user, password, host from connection string
	authParts := strings.Split(parts[0], "@")
	if len(authParts) < 2 {
		return fmt.Errorf("invalid MySQL connection string format")
	}

	// Extract host (and potentially port)
	hostPart := authParts[1]

	// Extract user and password
	userPassPart := strings.TrimPrefix(authParts[0], "mysql://")
	userPassSplit := strings.Split(userPassPart, ":")
	if len(userPassSplit) < 2 {
		return fmt.Errorf("invalid MySQL connection string format")
	}
	user := userPassSplit[0]
	pass := userPassSplit[1]

	// Create the output file
	backupFile, err := os.Create(backupPath)
	if err != nil {
		return fmt.Errorf("failed to create backup file: %w", err)
	}
	defer backupFile.Close()

	// Set up the mysqldump command
	cmd := exec.CommandContext(ctx, "mysqldump",
		"--user="+user,
		"--password="+pass,
		"--host="+hostPart,
		"--databases", dbName,
		"--single-transaction",
		"--quick",
	)

	cmd.Stdout = backupFile
	cmd.Stderr = os.Stderr

	// Execute the mysqldump command
	m.LogBackupInfo(fmt.Sprintf("Running mysqldump to %s", backupPath))
	if err := cmd.Run(); err != nil {
		return fmt.Errorf("mysqldump failed: %w", err)
	}

	// Check if the backup file was created successfully
	info, err := os.Stat(backupPath)
	if err != nil {
		return fmt.Errorf("failed to verify backup file: %w", err)
	}

	m.LogBackupInfo(fmt.Sprintf("MySQL backup completed successfully: %s (%.2f MB)",
		filepath.Base(backupPath), float64(info.Size())/(1024*1024)))

	return nil
}

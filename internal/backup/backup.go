package backup

import (
	"fmt"
	"log"
	"os"
	"path/filepath"
	"time"

	"github.com/username/backmeup/internal/config"
)

// Executor is the interface for all backup executors
type Executor interface {
	// Execute runs the backup operation
	Execute() error
}

// BaseExecutor contains common functionality for all backup executors
type BaseExecutor struct {
	Config        config.JobConfig
	StorageConfig config.StorageConfig
}

// GenerateBackupFileName generates a timestamped filename for the backup
func (b *BaseExecutor) GenerateBackupFileName(prefix string, extension string) string {
	timestamp := time.Now().Format("20060102-150405")
	return fmt.Sprintf("%s_%s%s", prefix, timestamp, extension)
}

// GetBackupDestination returns the path where backups should be stored
func (b *BaseExecutor) GetBackupDestination() (string, error) {
	if b.StorageConfig.Type != "local" {
		return "", fmt.Errorf("only local storage is currently supported")
	}

	dir := b.StorageConfig.Local.Directory

	// Create the directory if it doesn't exist
	if err := os.MkdirAll(dir, 0755); err != nil {
		return "", fmt.Errorf("failed to create backup directory: %w", err)
	}

	return dir, nil
}

// BuildBackupFilePath constructs the full path for a backup file
func (b *BaseExecutor) BuildBackupFilePath(fileName string) (string, error) {
	destDir, err := b.GetBackupDestination()
	if err != nil {
		return "", err
	}

	// Create a job-specific subdirectory
	jobDir := filepath.Join(destDir, b.Config.Name)
	if err := os.MkdirAll(jobDir, 0755); err != nil {
		return "", fmt.Errorf("failed to create job directory: %w", err)
	}

	return filepath.Join(jobDir, fileName), nil
}

// LogBackupInfo logs information about the backup
func (b *BaseExecutor) LogBackupInfo(message string) {
	log.Printf("[Job: %s] %s", b.Config.Name, message)
}

// CreateExecutor creates the appropriate backup executor for a job
func CreateExecutor(jobConfig config.JobConfig, storageConfig config.StorageConfig) (Executor, error) {
	// Create the appropriate executor based on job type
	switch jobConfig.Type {
	case "postgres":
		return NewPostgresExecutor(jobConfig, storageConfig)
	case "mysql":
		return NewMySQLExecutor(jobConfig, storageConfig)
	case "minio":
		return NewMinioExecutor(jobConfig, storageConfig)
	default:
		return nil, fmt.Errorf("unsupported job type: %s", jobConfig.Type)
	}
}

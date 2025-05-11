package backup

import (
	"bytes"
	"context"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"time"

	"github.com/minio/minio-go/v7"
	"github.com/minio/minio-go/v7/pkg/credentials"
	"github.com/username/backmeup/internal/config"
)

// MinioExecutor implements backup execution for MinIO object storage
type MinioExecutor struct {
	BaseExecutor
	client *minio.Client
}

// NewMinioExecutor creates a new MinIO backup executor
func NewMinioExecutor(jobConfig config.JobConfig, storageConfig config.StorageConfig) (Executor, error) {
	if jobConfig.MinIOConfig == nil {
		return nil, fmt.Errorf("missing MinIO configuration for job: %s", jobConfig.Name)
	}

	// Initialize MinIO client - we'll keep this for operations that might require the SDK
	client, err := minio.New(jobConfig.MinIOConfig.Endpoint, &minio.Options{
		Creds:  credentials.NewStaticV4(jobConfig.MinIOConfig.AccessKey, jobConfig.MinIOConfig.SecretKey, ""),
		Secure: jobConfig.MinIOConfig.UseSSL,
	})
	if err != nil {
		return nil, fmt.Errorf("failed to create MinIO client: %w", err)
	}

	return &MinioExecutor{
		BaseExecutor: BaseExecutor{
			Config:        jobConfig,
			StorageConfig: storageConfig,
		},
		client: client,
	}, nil
}

// checkMCInstalled verifies if MinIO Client (mc) is installed
func (m *MinioExecutor) checkMCInstalled() error {
	cmd := exec.Command("mc", "version")
	err := cmd.Run()
	if err != nil {
		return fmt.Errorf("MinIO Client (mc) is not installed or not in PATH. Please install mc tool: %w", err)
	}
	return nil
}

// configureMC sets up mc config with MinIO server credentials
func (m *MinioExecutor) configureMC(ctx context.Context) (string, error) {
	cfg := m.Config.MinIOConfig

	// Create a unique alias for this backup job
	alias := fmt.Sprintf("backmeup-%s", m.Config.Name)

	var stdout, stderr bytes.Buffer

	// Configure mc with server details
	cmd := exec.CommandContext(ctx, "mc", "alias", "set", alias,
		cfg.Endpoint, cfg.AccessKey, cfg.SecretKey)

	cmd.Stdout = &stdout
	cmd.Stderr = &stderr

	if err := cmd.Run(); err != nil {
		return "", fmt.Errorf("failed to configure mc: %w, stderr: %s", err, stderr.String())
	}

	return alias, nil
}

// Execute performs a backup of MinIO bucket data using mc sync
func (m *MinioExecutor) Execute() error {
	m.LogBackupInfo("Starting MinIO backup using mc sync")

	// Check if mc is installed
	if err := m.checkMCInstalled(); err != nil {
		return err
	}

	ctx, cancel := context.WithTimeout(context.Background(), 24*time.Hour)
	defer cancel()

	cfg := m.Config.MinIOConfig

	// Generate a timestamped directory for this backup
	timestamp := time.Now().Format("20060102-150405")
	backupDirName := fmt.Sprintf("minio_backup_%s", timestamp)

	// Build the full path where the backup will be stored
	destDir, err := m.GetBackupDestination()
	if err != nil {
		return fmt.Errorf("failed to get backup destination: %w", err)
	}

	// Create job-specific directory
	jobDir := filepath.Join(destDir, m.Config.Name)
	if err := os.MkdirAll(jobDir, 0755); err != nil {
		return fmt.Errorf("failed to create job directory: %w", err)
	}

	// Create timestamp-specific directory for this backup
	backupDir := filepath.Join(jobDir, backupDirName)
	if err := os.MkdirAll(backupDir, 0755); err != nil {
		return fmt.Errorf("failed to create backup directory: %w", err)
	}

	// Configure mc with the MinIO server
	alias, err := m.configureMC(ctx)
	if err != nil {
		return err
	}

	// Build the source path for mc sync
	sourcePath := fmt.Sprintf("%s/%s", alias, cfg.BucketName)
	if cfg.SourceFolder != "" {
		// Ensure the source folder has a trailing slash for mc
		if !strings.HasSuffix(cfg.SourceFolder, "/") {
			sourcePath = fmt.Sprintf("%s/%s/", sourcePath, cfg.SourceFolder)
		} else {
			sourcePath = fmt.Sprintf("%s/%s", sourcePath, cfg.SourceFolder)
		}
	}

	m.LogBackupInfo(fmt.Sprintf("Syncing from %s to %s", sourcePath, backupDir))

	var stdout, stderr bytes.Buffer

	// Execute mc sync command
	// We use --newer-than=0 to sync all files regardless of their modification time
	cmd := exec.CommandContext(ctx, "mc", "cp", "--recursive", sourcePath, backupDir)
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr

	// Start executing the command
	if err := cmd.Start(); err != nil {
		return fmt.Errorf("failed to start mc sync: %w", err)
	}

	// Log progress periodically
	done := make(chan struct{})
	go func() {
		defer close(done)
		ticker := time.NewTicker(10 * time.Second)
		defer ticker.Stop()

		for {
			select {
			case <-ticker.C:
				m.LogBackupInfo("MC sync in progress...")
			case <-ctx.Done():
				return
			case <-done:
				return
			}
		}
	}()

	// Wait for the command to complete
	err = cmd.Wait()
	done <- struct{}{}

	if err != nil {
		return fmt.Errorf("mc sync failed: %w, stderr: %s", err, stderr.String())
	}

	// Log completion
	m.LogBackupInfo(fmt.Sprintf("MinIO backup completed successfully to %s", backupDir))
	m.LogBackupInfo(fmt.Sprintf("mc output: %s", stdout.String()))

	return nil
}

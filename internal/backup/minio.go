package backup

import (
	"context"
	"fmt"
	"io"
	"os"
	"path/filepath"
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

	// Initialize MinIO client
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

// Execute performs a backup of MinIO bucket data
func (m *MinioExecutor) Execute() error {
	m.LogBackupInfo("Starting MinIO backup")

	ctx := context.Background()
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

	// Check if the bucket exists
	exists, err := m.client.BucketExists(ctx, cfg.BucketName)
	if err != nil {
		return fmt.Errorf("failed to check if bucket exists: %w", err)
	}
	if !exists {
		return fmt.Errorf("bucket %s does not exist", cfg.BucketName)
	}

	// Create a channel to receive objects
	objectsCh := m.client.ListObjects(ctx, cfg.BucketName, minio.ListObjectsOptions{
		Recursive: true,
		Prefix:    cfg.SourceFolder,
	})

	// Keep track of statistics
	var totalSize int64
	var fileCount int

	// Download each object
	for object := range objectsCh {
		if object.Err != nil {
			return fmt.Errorf("error listing objects: %w", object.Err)
		}

		// Generate local path for this object
		relativePath := object.Key
		if cfg.SourceFolder != "" {
			relativePath = relativePath[len(cfg.SourceFolder):]
		}

		// Ensure the relative path is not empty
		if len(relativePath) == 0 {
			continue
		}

		// Remove leading slash if present
		if relativePath[0] == '/' {
			relativePath = relativePath[1:]
		}

		localPath := filepath.Join(backupDir, relativePath)

		// Create parent directory if it doesn't exist
		if err := os.MkdirAll(filepath.Dir(localPath), 0755); err != nil {
			return fmt.Errorf("failed to create directory for object: %w", err)
		}

		// Skip directories (which have zero size)
		if object.Size == 0 && object.Key[len(object.Key)-1] == '/' {
			continue
		}

		// Download the object
		obj, err := m.client.GetObject(ctx, cfg.BucketName, object.Key, minio.GetObjectOptions{})
		if err != nil {
			return fmt.Errorf("failed to get object %s: %w", object.Key, err)
		}

		// Create the local file
		localFile, err := os.Create(localPath)
		if err != nil {
			return fmt.Errorf("failed to create local file %s: %w", localPath, err)
		}

		// Copy the object content to local file
		written, err := io.Copy(localFile, obj)
		localFile.Close()
		if err != nil {
			return fmt.Errorf("failed to download object %s: %w", object.Key, err)
		}

		// Update statistics
		totalSize += written
		fileCount++

		if fileCount%10 == 0 {
			m.LogBackupInfo(fmt.Sprintf("Downloaded %d files (%.2f MB)",
				fileCount, float64(totalSize)/(1024*1024)))
		}
	}

	m.LogBackupInfo(fmt.Sprintf("MinIO backup completed successfully: %d files (%.2f MB) to %s",
		fileCount, float64(totalSize)/(1024*1024), backupDir))

	return nil
}

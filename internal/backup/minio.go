package backup

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"sync/atomic"
	"time"

	"github.com/minio/minio-go/v7"
	"github.com/minio/minio-go/v7/pkg/credentials"
	"github.com/thitiph0n/backmeup/internal/config"
	"github.com/thitiph0n/backmeup/internal/storage"
	"github.com/thitiph0n/backmeup/internal/storage/localfs"
)

const (
	minioDownloadWorkers   = 4
	minioProgressLogPeriod = 10 * time.Second
)

type objectStore interface {
	ListObjects(ctx context.Context, bucketName string, opts minio.ListObjectsOptions) <-chan minio.ObjectInfo
	FGetObject(ctx context.Context, bucketName, objectName, filePath string, opts minio.GetObjectOptions) error
}

type MinioExecutor struct {
	BaseExecutor
	client objectStore
}

func NewMinioExecutor(jobConfig config.JobConfig, store storage.Storage) (Executor, error) {
	if jobConfig.MinIOConfig == nil {
		return nil, fmt.Errorf("missing MinIO configuration for job: %s", jobConfig.Name)
	}

	client, err := minio.New(jobConfig.MinIOConfig.Endpoint, &minio.Options{
		Creds:  credentials.NewStaticV4(jobConfig.MinIOConfig.AccessKey, jobConfig.MinIOConfig.SecretKey, ""),
		Secure: jobConfig.MinIOConfig.UseSSL,
	})
	if err != nil {
		return nil, fmt.Errorf("failed to create MinIO client: %w", err)
	}

	return &MinioExecutor{
		BaseExecutor: BaseExecutor{
			Config:  jobConfig,
			Storage: store,
		},
		client: client,
	}, nil
}

func (m *MinioExecutor) Execute(ctx context.Context) (err error) {
	m.LogBackupInfo("Starting MinIO backup")

	cfg := m.Config.MinIOConfig

	backupDirName := localfs.GenerateFileName("minio_backup", "")

	backupDir, err := m.Storage.NewDir(m.Config.Name, backupDirName)
	if err != nil {
		return fmt.Errorf("failed to prepare backup directory: %w", err)
	}
	defer func() {
		if err != nil {
			m.discardBackup(backupDirName)
		}
	}()

	prefix := sourcePrefix(cfg.SourceFolder)
	m.LogBackupInfo(fmt.Sprintf("Downloading s3://%s/%s to %s", cfg.BucketName, prefix, backupDir))

	count, err := m.downloadAll(ctx, backupDir, prefix)
	if err != nil {
		return fmt.Errorf("minio download failed: %w", err)
	}

	if count == 0 {
		return fmt.Errorf("no objects found in s3://%s/%s", cfg.BucketName, prefix)
	}

	if err = m.Storage.Commit(m.Config.Name, backupDirName); err != nil {
		return err
	}

	m.LogBackupInfo(fmt.Sprintf("MinIO backup completed successfully: %s (%d objects)", backupDirName, count))

	return nil
}

func (m *MinioExecutor) downloadAll(ctx context.Context, backupDir, prefix string) (int64, error) {
	ctx, cancel := context.WithCancel(ctx)
	defer cancel()

	var (
		firstErr   error
		errOnce    sync.Once
		downloaded atomic.Int64
		wg         sync.WaitGroup
	)
	fail := func(err error) {
		errOnce.Do(func() {
			firstErr = err
			cancel()
		})
	}

	jobs := make(chan minio.ObjectInfo)
	for i := 0; i < minioDownloadWorkers; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for obj := range jobs {
				if err := m.downloadObject(ctx, backupDir, prefix, obj); err != nil {
					fail(err)
					continue
				}
				downloaded.Add(1)
			}
		}()
	}

	stopProgress := make(chan struct{})
	go func() {
		ticker := time.NewTicker(minioProgressLogPeriod)
		defer ticker.Stop()
		for {
			select {
			case <-ticker.C:
				m.LogBackupInfo(fmt.Sprintf("MinIO download in progress: %d objects so far", downloaded.Load()))
			case <-stopProgress:
				return
			}
		}
	}()

	listing := m.client.ListObjects(ctx, m.Config.MinIOConfig.BucketName, minio.ListObjectsOptions{
		Prefix:    prefix,
		Recursive: true,
	})
feed:
	for obj := range listing {
		if obj.Err != nil {
			fail(fmt.Errorf("failed to list objects: %w", obj.Err))
			break
		}
		if strings.HasSuffix(obj.Key, "/") {
			continue
		}
		select {
		case jobs <- obj:
		case <-ctx.Done():
			break feed
		}
	}
	close(jobs)
	wg.Wait()
	close(stopProgress)

	if firstErr != nil {
		return 0, firstErr
	}
	if err := ctx.Err(); err != nil {
		return 0, err
	}
	return downloaded.Load(), nil
}

func (m *MinioExecutor) downloadObject(ctx context.Context, backupDir, prefix string, obj minio.ObjectInfo) error {
	dest, err := objectDestination(backupDir, prefix, obj.Key)
	if err != nil {
		return err
	}

	if err := os.MkdirAll(filepath.Dir(dest), 0755); err != nil {
		return fmt.Errorf("failed to create directory for %s: %w", obj.Key, err)
	}

	if err := m.client.FGetObject(ctx, m.Config.MinIOConfig.BucketName, obj.Key, dest, minio.GetObjectOptions{}); err != nil {
		return fmt.Errorf("failed to download %s: %w", obj.Key, err)
	}

	if !obj.LastModified.IsZero() {
		if err := os.Chtimes(dest, obj.LastModified, obj.LastModified); err != nil {
			return fmt.Errorf("failed to set modification time for %s: %w", obj.Key, err)
		}
	}

	return nil
}

func sourcePrefix(folder string) string {
	folder = strings.Trim(folder, "/")
	if folder == "" {
		return ""
	}
	return folder + "/"
}

func objectDestination(backupDir, prefix, key string) (string, error) {
	rel := filepath.FromSlash(strings.TrimPrefix(key, prefix))
	if !filepath.IsLocal(rel) {
		return "", fmt.Errorf("object key %q escapes the backup directory", key)
	}
	return filepath.Join(backupDir, rel), nil
}

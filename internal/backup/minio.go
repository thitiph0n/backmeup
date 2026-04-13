package backup

import (
	"bytes"
	"context"
	"fmt"
	"os/exec"
	"strings"
	"time"

	"github.com/minio/minio-go/v7"
	"github.com/minio/minio-go/v7/pkg/credentials"
	"github.com/thitiph0n/backmeup/internal/config"
	"github.com/thitiph0n/backmeup/internal/storage"
	"github.com/thitiph0n/backmeup/internal/storage/localfs"
)

type MinioExecutor struct {
	BaseExecutor
	client *minio.Client
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

func (m *MinioExecutor) checkMCInstalled() error {
	cmd := exec.Command("mc", "version")
	err := cmd.Run()
	if err != nil {
		return fmt.Errorf("MinIO Client (mc) is not installed or not in PATH. Please install mc tool: %w", err)
	}
	return nil
}

func (m *MinioExecutor) configureMC(ctx context.Context) (string, error) {
	cfg := m.Config.MinIOConfig

	alias := fmt.Sprintf("backmeup-%s", m.Config.Name)

	var stdout, stderr bytes.Buffer

	endpoint := cfg.Endpoint
	if !strings.HasPrefix(endpoint, "http://") && !strings.HasPrefix(endpoint, "https://") {
		if cfg.UseSSL {
			endpoint = "https://" + endpoint
		} else {
			endpoint = "http://" + endpoint
		}
	}

	schemeAndHost := endpoint

	slashPos := 0
	if strings.HasPrefix(endpoint, "https://") {
		slashPos = 8
	} else if strings.HasPrefix(endpoint, "http://") {
		slashPos = 7
	}

	if pathSlashPos := strings.Index(endpoint[slashPos:], "/"); pathSlashPos != -1 {
		schemeAndHost = endpoint[:slashPos+pathSlashPos+1]
	} else if !strings.HasSuffix(endpoint, "/") {
		schemeAndHost = endpoint + "/"
	}

	endpoint = schemeAndHost

	cmd := exec.CommandContext(ctx, "mc", "alias", "set", alias,
		endpoint, cfg.AccessKey, cfg.SecretKey)

	cmd.Stdout = &stdout
	cmd.Stderr = &stderr

	m.LogBackupInfo(fmt.Sprintf("Configuring MinIO client with endpoint: %s", endpoint))

	if err := cmd.Run(); err != nil {
		return "", fmt.Errorf("failed to configure mc: %w, stderr: %s", err, stderr.String())
	}

	return alias, nil
}

func (m *MinioExecutor) Execute(ctx context.Context) error {
	m.LogBackupInfo("Starting MinIO backup using mc mirror")

	if err := m.checkMCInstalled(); err != nil {
		return err
	}

	cfg := m.Config.MinIOConfig

	backupDirName := localfs.GenerateFileName("minio_backup", "")

	backupDir, err := m.Storage.NewDir(m.Config.Name, backupDirName)
	if err != nil {
		return fmt.Errorf("failed to prepare backup directory: %w", err)
	}

	alias, err := m.configureMC(ctx)
	if err != nil {
		return err
	}

	sourcePath := fmt.Sprintf("%s/%s", alias, cfg.BucketName)
	if cfg.SourceFolder != "" {
		if !strings.HasSuffix(cfg.SourceFolder, "/") {
			sourcePath = fmt.Sprintf("%s/%s/", sourcePath, cfg.SourceFolder)
		} else {
			sourcePath = fmt.Sprintf("%s/%s", sourcePath, cfg.SourceFolder)
		}
	}

	m.LogBackupInfo(fmt.Sprintf("Mirroring from %s to %s", sourcePath, backupDir))

	var stdout, stderr bytes.Buffer

	cmd := exec.CommandContext(ctx, "mc", "mirror", "--preserve", sourcePath, backupDir)
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr

	if err := cmd.Start(); err != nil {
		return fmt.Errorf("failed to start mc mirror: %w", err)
	}

	done := make(chan struct{})
	go func() {
		defer close(done)
		ticker := time.NewTicker(10 * time.Second)
		defer ticker.Stop()

		for {
			select {
			case <-ticker.C:
				m.LogBackupInfo("MC mirror in progress...")
			case <-ctx.Done():
				return
			case <-done:
				return
			}
		}
	}()

	err = cmd.Wait()
	done <- struct{}{}

	if err != nil {
		return fmt.Errorf("mc mirror failed: %w, stderr: %s", err, stderr.String())
	}

	m.LogBackupInfo(fmt.Sprintf("MinIO backup completed successfully to %s", backupDir))
	m.LogBackupInfo(fmt.Sprintf("mc output: %s", stdout.String()))

	return nil
}

package backup

import (
	"context"
	"fmt"
	"log"
	"os/exec"

	"github.com/thitiph0n/backmeup/internal/config"
	"github.com/thitiph0n/backmeup/internal/storage"
	"github.com/thitiph0n/backmeup/internal/storage/localfs"
)

type Executor interface {
	Execute(ctx context.Context) error
}

type BaseExecutor struct {
	Config  config.JobConfig
	Storage storage.Storage
}

func (b *BaseExecutor) LogBackupInfo(message string) {
	log.Printf("[Job: %s] %s", b.Config.Name, message)
}

func (b *BaseExecutor) dumpToFile(cmd *exec.Cmd, filename string) error {
	writer, err := b.Storage.NewWriter(b.Config.Name, filename)
	if err != nil {
		return fmt.Errorf("failed to prepare backup file: %w", err)
	}

	cmd.Stdout = writer
	runErr := cmd.Run()
	if closeErr := writer.Close(); runErr == nil {
		runErr = closeErr
	}
	if runErr == nil {
		if err := b.Storage.Commit(b.Config.Name, filename); err != nil {
			b.discardBackup(filename)
			return err
		}
		return nil
	}

	b.discardBackup(filename)
	return fmt.Errorf("%s failed: %w", cmd.Args[0], runErr)
}

func (b *BaseExecutor) discardBackup(name string) {
	if err := b.Storage.Discard(b.Config.Name, name); err != nil {
		b.LogBackupInfo(fmt.Sprintf("Failed to remove incomplete backup %s: %v", name, err))
	}
}

func CreateExecutor(jobConfig config.JobConfig, storageConfig config.StorageConfig) (Executor, error) {
	store := localfs.New(storageConfig.Local)

	switch jobConfig.Type {
	case "postgres":
		return NewPostgresExecutor(jobConfig, store)
	case "mysql":
		return NewMySQLExecutor(jobConfig, store)
	case "minio":
		return NewMinioExecutor(jobConfig, store)
	default:
		return nil, fmt.Errorf("unsupported job type: %s", jobConfig.Type)
	}
}

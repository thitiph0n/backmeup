package backup

import (
	"context"
	"fmt"
	"log"

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

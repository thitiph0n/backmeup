package storage

import (
	"io"
	"time"
)

type BackupEntry struct {
	Key     string
	ModTime time.Time
	Size    int64
}

type Storage interface {
	NewWriter(jobName, fileName string) (io.WriteCloser, error)
	NewDir(jobName, dirName string) (string, error)
	List(jobName string) ([]BackupEntry, error)
	Delete(entry BackupEntry) error
}

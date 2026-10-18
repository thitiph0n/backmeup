package localfs

import (
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/thitiph0n/backmeup/internal/config"
	"github.com/thitiph0n/backmeup/internal/storage"
)

const partialSuffix = ".partial"

var _ storage.Storage = (*Storage)(nil)

type Storage struct {
	directory string
}

func New(cfg config.LocalConfig) *Storage {
	return &Storage{directory: cfg.Directory}
}

func (s *Storage) NewWriter(jobName, fileName string) (io.WriteCloser, error) {
	jobDir := filepath.Join(s.directory, jobName)
	if err := os.MkdirAll(jobDir, 0755); err != nil {
		return nil, fmt.Errorf("failed to create job directory: %w", err)
	}
	return os.Create(filepath.Join(jobDir, fileName+partialSuffix))
}

func (s *Storage) NewDir(jobName, dirName string) (string, error) {
	dir := filepath.Join(s.directory, jobName, dirName+partialSuffix)
	if err := os.MkdirAll(dir, 0755); err != nil {
		return "", fmt.Errorf("failed to create backup directory: %w", err)
	}
	return dir, nil
}

func (s *Storage) List(jobName string) ([]storage.BackupEntry, error) {
	jobDir := filepath.Join(s.directory, jobName)
	if _, err := os.Stat(jobDir); os.IsNotExist(err) {
		return nil, nil
	}
	entries, err := os.ReadDir(jobDir)
	if err != nil {
		return nil, fmt.Errorf("failed to read directory: %w", err)
	}
	backups := make([]storage.BackupEntry, 0, len(entries))
	for _, e := range entries {
		if strings.HasSuffix(e.Name(), partialSuffix) {
			continue
		}
		info, err := e.Info()
		if err != nil {
			continue
		}
		backups = append(backups, storage.BackupEntry{
			Key:     filepath.Join(jobDir, e.Name()),
			ModTime: info.ModTime(),
			Size:    info.Size(),
		})
	}
	return backups, nil
}

func (s *Storage) Delete(entry storage.BackupEntry) error {
	return os.RemoveAll(entry.Key)
}

func (s *Storage) Commit(jobName, name string) error {
	path, err := s.entryPath(jobName, name)
	if err != nil {
		return err
	}
	if err := os.Rename(path+partialSuffix, path); err != nil {
		return fmt.Errorf("failed to finalize backup %s: %w", name, err)
	}
	return nil
}

func (s *Storage) Discard(jobName, name string) error {
	path, err := s.entryPath(jobName, name)
	if err != nil {
		return err
	}
	return os.RemoveAll(path + partialSuffix)
}

func (s *Storage) SweepPartial(jobName string) error {
	jobDir := filepath.Join(s.directory, jobName)
	entries, err := os.ReadDir(jobDir)
	if os.IsNotExist(err) {
		return nil
	}
	if err != nil {
		return fmt.Errorf("failed to read directory: %w", err)
	}
	for _, e := range entries {
		if !strings.HasSuffix(e.Name(), partialSuffix) {
			continue
		}
		if err := os.RemoveAll(filepath.Join(jobDir, e.Name())); err != nil {
			return fmt.Errorf("failed to remove stale partial backup %s: %w", e.Name(), err)
		}
	}
	return nil
}

func (s *Storage) entryPath(jobName, name string) (string, error) {
	if name == "." || !filepath.IsLocal(name) || filepath.Base(name) != name {
		return "", fmt.Errorf("invalid backup name: %q", name)
	}
	return filepath.Join(s.directory, jobName, name), nil
}

func GenerateFileName(prefix, extension string) string {
	return fmt.Sprintf("%s_%s%s", prefix, time.Now().Format("20060102-150405"), extension)
}

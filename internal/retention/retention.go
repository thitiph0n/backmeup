package retention

import (
	"fmt"
	"log"
	"os"
	"path/filepath"
	"sort"
	"time"

	"github.com/username/backmeup/internal/config"
)

// Manager handles the enforcement of retention policies
type Manager struct {
	StorageConfig config.StorageConfig
}

// NewManager creates a new retention manager
func NewManager(storageConfig config.StorageConfig) *Manager {
	return &Manager{
		StorageConfig: storageConfig,
	}
}

// ApplyRetentionPolicy applies the retention policy to the given job
func (m *Manager) ApplyRetentionPolicy(jobConfig config.JobConfig) error {
	if m.StorageConfig.Type != "local" {
		return fmt.Errorf("only local storage is currently supported")
	}

	// Get the job's backup directory
	jobDir := filepath.Join(m.StorageConfig.Local.Directory, jobConfig.Name)

	// If directory doesn't exist, nothing to do
	if _, err := os.Stat(jobDir); os.IsNotExist(err) {
		return nil
	}

	switch jobConfig.RetentionPolicy.Type {
	case "count":
		return m.applyCountBasedRetention(jobDir, jobConfig.Name, jobConfig.RetentionPolicy.Value)
	case "days":
		return m.applyDaysBasedRetention(jobDir, jobConfig.Name, jobConfig.RetentionPolicy.Value)
	default:
		return fmt.Errorf("unsupported retention policy type: %s", jobConfig.RetentionPolicy.Type)
	}
}

// applyCountBasedRetention keeps the N most recent backups and deletes the rest
func (m *Manager) applyCountBasedRetention(jobDir, jobName string, keepCount int) error {
	// List files in the job directory
	backupFiles, err := m.listBackupFiles(jobDir, jobName)
	if err != nil {
		return fmt.Errorf("failed to list backup files: %w", err)
	}

	// If we have fewer backups than the retention count, nothing to do
	if len(backupFiles) <= keepCount {
		return nil
	}

	// Sort files by modification time (newest first)
	sort.Slice(backupFiles, func(i, j int) bool {
		return backupFiles[i].ModTime.After(backupFiles[j].ModTime)
	})

	// Delete all but the newest 'keepCount' files
	for i := keepCount; i < len(backupFiles); i++ {
		filePath := backupFiles[i].Path
		if err := os.Remove(filePath); err != nil {
			log.Printf("Warning: failed to delete old backup file %s: %v", filePath, err)
			continue
		}
		log.Printf("[Job: %s] Deleted old backup: %s", jobName, filepath.Base(filePath))
	}

	log.Printf("[Job: %s] Retention policy applied: kept %d of %d backups",
		jobName, keepCount, len(backupFiles))

	return nil
}

// applyDaysBasedRetention deletes backups older than the specified number of days
func (m *Manager) applyDaysBasedRetention(jobDir, jobName string, keepDays int) error {
	// List files in the job directory
	backupFiles, err := m.listBackupFiles(jobDir, jobName)
	if err != nil {
		return fmt.Errorf("failed to list backup files: %w", err)
	}

	// Calculate cutoff time
	cutoffTime := time.Now().AddDate(0, 0, -keepDays)
	deletedCount := 0

	// Delete files older than the cutoff time
	for _, file := range backupFiles {
		if file.ModTime.Before(cutoffTime) {
			if err := os.Remove(file.Path); err != nil {
				log.Printf("Warning: failed to delete old backup file %s: %v", file.Path, err)
				continue
			}
			deletedCount++
			log.Printf("[Job: %s] Deleted backup older than %d days: %s",
				jobName, keepDays, filepath.Base(file.Path))
		}
	}

	log.Printf("[Job: %s] Retention policy applied: deleted %d backups older than %d days",
		jobName, deletedCount, keepDays)

	return nil
}

// BackupFile represents a backup file with metadata
type BackupFile struct {
	Path    string
	ModTime time.Time
	Size    int64
}

// listBackupFiles returns a list of backup files in the directory
func (m *Manager) listBackupFiles(dir, jobName string) ([]BackupFile, error) {
	var files []BackupFile

	entries, err := os.ReadDir(dir)
	if err != nil {
		return nil, fmt.Errorf("failed to read directory %s: %w", dir, err)
	}

	for _, entry := range entries {
		if entry.IsDir() {
			// For MinIO backups that are stored in directories
			dirPath := filepath.Join(dir, entry.Name())
			if isBackupDir(entry.Name()) {
				info, err := entry.Info()
				if err != nil {
					log.Printf("Warning: failed to get info for directory %s: %v", dirPath, err)
					continue
				}
				files = append(files, BackupFile{
					Path:    dirPath,
					ModTime: info.ModTime(),
					Size:    info.Size(),
				})
			}
		} else {
			// Regular backup files
			if isBackupFile(entry.Name()) {
				info, err := entry.Info()
				if err != nil {
					log.Printf("Warning: failed to get info for file %s: %v", entry.Name(), err)
					continue
				}
				files = append(files, BackupFile{
					Path:    filepath.Join(dir, entry.Name()),
					ModTime: info.ModTime(),
					Size:    info.Size(),
				})
			}
		}
	}

	return files, nil
}

// isBackupFile checks if a filename matches the pattern of backup files
func isBackupFile(filename string) bool {
	// Check if the filename matches any of our backup file patterns
	patterns := []string{
		"pg_backup_",
		"mysql_backup_",
	}

	for _, pattern := range patterns {
		if len(filename) > len(pattern) && filename[:len(pattern)] == pattern {
			return true
		}
	}
	return false
}

// isBackupDir checks if a directory name matches the pattern of backup directories
func isBackupDir(dirname string) bool {
	// Check if the directory name matches any of our backup directory patterns
	patterns := []string{
		"minio_backup_",
	}

	for _, pattern := range patterns {
		if len(dirname) > len(pattern) && dirname[:len(pattern)] == pattern {
			return true
		}
	}
	return false
}

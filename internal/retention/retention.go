package retention

import (
	"fmt"
	"log"
	"sort"
	"time"

	"github.com/thitiph0n/backmeup/internal/config"
	"github.com/thitiph0n/backmeup/internal/storage"
)

type Manager struct {
	storage storage.Storage
}

func NewManager(s storage.Storage) *Manager {
	return &Manager{storage: s}
}

func (m *Manager) ApplyRetentionPolicy(jobConfig config.JobConfig) error {
	switch jobConfig.RetentionPolicy.Type {
	case "count":
		return m.applyCountBasedRetention(jobConfig.Name, jobConfig.RetentionPolicy.Value)
	case "days":
		return m.applyDaysBasedRetention(jobConfig.Name, jobConfig.RetentionPolicy.Value)
	default:
		return fmt.Errorf("unsupported retention policy type: %s", jobConfig.RetentionPolicy.Type)
	}
}

func (m *Manager) applyCountBasedRetention(jobName string, keepCount int) error {
	entries, err := m.storage.List(jobName)
	if err != nil {
		return fmt.Errorf("failed to list backup files: %w", err)
	}

	if len(entries) <= keepCount {
		return nil
	}

	sort.Slice(entries, func(i, j int) bool {
		return entries[i].ModTime.After(entries[j].ModTime)
	})

	for i := keepCount; i < len(entries); i++ {
		if err := m.storage.Delete(entries[i]); err != nil {
			log.Printf("Warning: failed to delete old backup %s: %v", entries[i].Key, err)
			continue
		}
		log.Printf("[Job: %s] Deleted old backup: %s", jobName, entries[i].Key)
	}

	log.Printf("[Job: %s] Retention policy applied: kept %d of %d backups",
		jobName, keepCount, len(entries))

	return nil
}

func (m *Manager) applyDaysBasedRetention(jobName string, keepDays int) error {
	entries, err := m.storage.List(jobName)
	if err != nil {
		return fmt.Errorf("failed to list backup files: %w", err)
	}

	cutoffTime := time.Now().AddDate(0, 0, -keepDays)
	deletedCount := 0

	for _, entry := range entries {
		if entry.ModTime.Before(cutoffTime) {
			if err := m.storage.Delete(entry); err != nil {
				log.Printf("Warning: failed to delete old backup %s: %v", entry.Key, err)
				continue
			}
			deletedCount++
			log.Printf("[Job: %s] Deleted backup older than %d days: %s",
				jobName, keepDays, entry.Key)
		}
	}

	log.Printf("[Job: %s] Retention policy applied: deleted %d backups older than %d days",
		jobName, deletedCount, keepDays)

	return nil
}

package scheduler

import (
	"context"
	"fmt"
	"log"
	"time"

	"github.com/go-co-op/gocron"
	"github.com/username/backmeup/internal/config"
	"github.com/username/backmeup/internal/retention"
)

// BackupExecutor defines the interface for backup executors
type BackupExecutor interface {
	Execute(ctx context.Context) error
}

// JobScheduler manages backup jobs scheduling
type JobScheduler struct {
	scheduler     *gocron.Scheduler
	jobs          map[string]BackupExecutor
	jobConfigs    map[string]config.JobConfig
	retentionMgr  *retention.Manager
	storageConfig config.StorageConfig
}

// NewJobScheduler creates a new job scheduler
func NewJobScheduler(storageConfig config.StorageConfig) *JobScheduler {
	return &JobScheduler{
		scheduler:     gocron.NewScheduler(time.Local),
		jobs:          make(map[string]BackupExecutor),
		jobConfigs:    make(map[string]config.JobConfig),
		retentionMgr:  retention.NewManager(storageConfig),
		storageConfig: storageConfig,
	}
}

// AddJob adds a backup job to the scheduler
func (js *JobScheduler) AddJob(jobConfig config.JobConfig, executor BackupExecutor) error {
	jobName := jobConfig.Name

	// Add the job to the scheduler
	job, err := js.scheduler.Cron(jobConfig.Schedule).Do(func() {
		log.Printf("Running backup job: %s (%s)", jobName, jobConfig.Type)

		// Create a context with timeout for this backup job
		ctx, cancel := context.WithTimeout(context.Background(), 12*time.Hour)
		defer cancel()

		if err := executor.Execute(ctx); err != nil {
			log.Printf("Error executing backup job %s: %v", jobName, err)
		} else {
			log.Printf("Backup job %s completed successfully", jobName)

			// Apply retention policy after successful backup
			log.Printf("Applying retention policy for job %s: Keep %d %s",
				jobName, jobConfig.RetentionPolicy.Value, jobConfig.RetentionPolicy.Type)

			if err := js.retentionMgr.ApplyRetentionPolicy(jobConfig); err != nil {
				log.Printf("Error applying retention policy for job %s: %v", jobName, err)
			}
		}
	})

	if err != nil {
		return fmt.Errorf("failed to schedule job %s: %w", jobName, err)
	}

	// Set job metadata for better logging/tracking
	job.Tag(jobName)

	// Store the executor and job config
	js.jobs[jobName] = executor
	js.jobConfigs[jobName] = jobConfig

	return nil
}

// Start begins the job scheduler
func (js *JobScheduler) Start() {
	js.scheduler.StartAsync()
	log.Printf("Job scheduler started with %d jobs", len(js.jobs))
}

// Stop stops the job scheduler
func (js *JobScheduler) Stop() {
	js.scheduler.Stop()
	log.Printf("Job scheduler stopped")
}

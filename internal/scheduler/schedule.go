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
	callbacks     []JobStatusCallback
}

// NewJobScheduler creates a new job scheduler
func NewJobScheduler(storageConfig config.StorageConfig) *JobScheduler {
	return &JobScheduler{
		scheduler:     gocron.NewScheduler(time.Local),
		jobs:          make(map[string]BackupExecutor),
		jobConfigs:    make(map[string]config.JobConfig),
		retentionMgr:  retention.NewManager(storageConfig),
		storageConfig: storageConfig,
		callbacks:     make([]JobStatusCallback, 0),
	}
}

// AddJob adds a backup job to the scheduler
func (js *JobScheduler) AddJob(jobConfig config.JobConfig, executor BackupExecutor) error {
	jobName := jobConfig.Name

	// Add the job to the scheduler
	job, err := js.scheduler.Cron(jobConfig.Schedule).Do(func() {
		log.Printf("Running backup job: %s (%s)", jobName, jobConfig.Type)

		// Notify that job is running
		for _, callback := range js.callbacks {
			callback(jobName, StatusRunning, time.Now())
		}

		// Create a context with timeout for this backup job
		ctx, cancel := context.WithTimeout(context.Background(), 12*time.Hour)
		defer cancel()

		if err := executor.Execute(ctx); err != nil {
			log.Printf("Error executing backup job %s: %v", jobName, err)

			// Notify of error
			for _, callback := range js.callbacks {
				callback(jobName, StatusError, time.Now())
			}
		} else {
			log.Printf("Backup job %s completed successfully", jobName)

			// Apply retention policy after successful backup
			log.Printf("Applying retention policy for job %s: Keep %d %s",
				jobName, jobConfig.RetentionPolicy.Value, jobConfig.RetentionPolicy.Type)

			if err := js.retentionMgr.ApplyRetentionPolicy(jobConfig); err != nil {
				log.Printf("Error applying retention policy for job %s: %v", jobName, err)
				// Retention errors don't change the backup job status to error
			}

			// Notify of completion
			for _, callback := range js.callbacks {
				callback(jobName, StatusComplete, time.Now())
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

	// Initialize job in PENDING status for any registered callbacks
	for _, callback := range js.callbacks {
		callback(jobName, StatusPending, time.Now())
	}

	return nil
}

// Start begins the job scheduler
func (js *JobScheduler) Start() {
	js.scheduler.StartAsync()
	log.Printf("Job scheduler started with %d jobs", len(js.jobs))

	// Notify all callbacks that scheduler is running
	for _, callback := range js.callbacks {
		// Special "scheduler" job name to indicate the scheduler itself
		callback("scheduler", StatusRunning, time.Now())
	}
}

// Stop stops the job scheduler
func (js *JobScheduler) Stop() {
	js.scheduler.Stop()
	log.Printf("Job scheduler stopped")

	// Notify all callbacks that scheduler is stopped
	for _, callback := range js.callbacks {
		// Special "scheduler" job name to indicate the scheduler itself
		callback("scheduler", StatusStopped, time.Now())
	}
}

// JobStatusCallback is a function that receives job status updates
type JobStatusCallback func(jobName string, status string, timestamp time.Time)

// JobStatusListener receives notifications about job status changes
type JobStatusListener struct {
	callbacks []JobStatusCallback
}

// JobStatus constants
const (
	StatusRunning  = "RUNNING"
	StatusPending  = "PENDING"
	StatusError    = "ERROR"
	StatusComplete = "COMPLETE"
	StatusStopped  = "STOPPED"
)

// RegisterStatusCallback registers a callback function for job status updates
func (js *JobScheduler) RegisterStatusCallback(callback JobStatusCallback) {
	// Add the callback to our list
	js.callbacks = append(js.callbacks, callback)

	// Initialize with current job statuses
	for jobName := range js.jobs {
		// Set all jobs to PENDING initially
		callback(jobName, StatusPending, time.Now())
	}

	// In a real implementation, we would hook this into the job execution system
	// to provide real-time updates when jobs start/complete/fail
}

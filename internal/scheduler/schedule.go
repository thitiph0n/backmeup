package scheduler

import (
	"context"
	"fmt"
	"log"
	"time"

	"github.com/go-co-op/gocron"
	"github.com/thitiph0n/backmeup/internal/config"
	"github.com/thitiph0n/backmeup/internal/retention"
	"github.com/thitiph0n/backmeup/internal/storage/localfs"
)

type BackupExecutor interface {
	Execute(ctx context.Context) error
}

type JobScheduler struct {
	scheduler    *gocron.Scheduler
	jobs         map[string]BackupExecutor
	jobConfigs   map[string]config.JobConfig
	retentionMgr *retention.Manager
	callbacks    []JobStatusCallback
}

func NewJobScheduler(storageConfig config.StorageConfig) *JobScheduler {
	store := localfs.New(storageConfig.Local)
	return &JobScheduler{
		scheduler:    gocron.NewScheduler(time.Local),
		jobs:         make(map[string]BackupExecutor),
		jobConfigs:   make(map[string]config.JobConfig),
		retentionMgr: retention.NewManager(store),
		callbacks:    make([]JobStatusCallback, 0),
	}
}

func (js *JobScheduler) AddJob(jobConfig config.JobConfig, executor BackupExecutor) error {
	jobName := jobConfig.Name

	job, err := js.scheduler.Cron(jobConfig.Schedule).Do(func() {
		log.Printf("Running backup job: %s (%s)", jobName, jobConfig.Type)

		for _, callback := range js.callbacks {
			callback(jobName, StatusRunning, time.Now())
		}

		ctx, cancel := context.WithTimeout(context.Background(), 12*time.Hour)
		defer cancel()

		if err := executor.Execute(ctx); err != nil {
			log.Printf("Error executing backup job %s: %v", jobName, err)

			for _, callback := range js.callbacks {
				callback(jobName, StatusError, time.Now())
			}
		} else {
			log.Printf("Backup job %s completed successfully", jobName)

			log.Printf("Applying retention policy for job %s: Keep %d %s",
				jobName, jobConfig.RetentionPolicy.Value, jobConfig.RetentionPolicy.Type)

			if err := js.retentionMgr.ApplyRetentionPolicy(jobConfig); err != nil {
				log.Printf("Error applying retention policy for job %s: %v", jobName, err)
			}

			for _, callback := range js.callbacks {
				callback(jobName, StatusComplete, time.Now())
			}
		}
	})

	if err != nil {
		return fmt.Errorf("failed to schedule job %s: %w", jobName, err)
	}

	job.Tag(jobName)

	js.jobs[jobName] = executor
	js.jobConfigs[jobName] = jobConfig

	for _, callback := range js.callbacks {
		callback(jobName, StatusPending, time.Now())
	}

	return nil
}

func (js *JobScheduler) Start() {
	js.scheduler.StartAsync()
	log.Printf("Job scheduler started with %d jobs", len(js.jobs))

	for _, callback := range js.callbacks {
		callback("scheduler", StatusRunning, time.Now())
	}
}

func (js *JobScheduler) Stop() {
	js.scheduler.Stop()
	log.Printf("Job scheduler stopped")

	for _, callback := range js.callbacks {
		callback("scheduler", StatusStopped, time.Now())
	}
}

type JobStatusCallback func(jobName string, status string, timestamp time.Time)

const (
	StatusRunning  = "RUNNING"
	StatusPending  = "PENDING"
	StatusError    = "ERROR"
	StatusComplete = "COMPLETE"
	StatusStopped  = "STOPPED"
)

func (js *JobScheduler) RegisterStatusCallback(callback JobStatusCallback) {
	js.callbacks = append(js.callbacks, callback)

	for jobName := range js.jobs {
		callback(jobName, StatusPending, time.Now())
	}
}

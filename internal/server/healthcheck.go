package server

import (
	"encoding/json"
	"net/http"
	"sync"
	"time"

	"github.com/username/backmeup/internal/scheduler"
)

// JobStatus represents the status of a backup job
type JobStatus string

// JobStatusTracker keeps track of job execution status
type JobStatusTracker struct {
	mu                 sync.RWMutex
	jobStatuses        map[string]JobStatus
	statusUpdated      time.Time
	isSchedulerRunning bool
}

// Health statuses for jobs and scheduler
const (
	StatusRunning  JobStatus = "RUNNING"
	StatusPending  JobStatus = "PENDING"
	StatusError    JobStatus = "ERROR"
	StatusStopped  JobStatus = "STOPPED"
	StatusComplete JobStatus = "COMPLETE"
)

// NewJobStatusTracker creates a new job status tracker
func NewJobStatusTracker() *JobStatusTracker {
	return &JobStatusTracker{
		jobStatuses:        make(map[string]JobStatus),
		statusUpdated:      time.Now(),
		isSchedulerRunning: false,
	}
}

// UpdateJobStatus updates the status of a job
func (jst *JobStatusTracker) UpdateJobStatus(jobName string, status JobStatus) {
	jst.mu.Lock()
	defer jst.mu.Unlock()

	jst.jobStatuses[jobName] = status
	jst.statusUpdated = time.Now()
}

// SetSchedulerRunning sets the running state of the scheduler
func (jst *JobStatusTracker) SetSchedulerRunning(isRunning bool) {
	jst.mu.Lock()
	defer jst.mu.Unlock()

	jst.isSchedulerRunning = isRunning
}

// GetAllStatuses returns the status of all jobs
func (jst *JobStatusTracker) GetAllStatuses() map[string]string {
	jst.mu.RLock()
	defer jst.mu.RUnlock()

	// Create the initial response with scheduler status
	result := make(map[string]string)

	// Add scheduler status
	if jst.isSchedulerRunning {
		result["scheduler"] = string(StatusRunning)
	} else {
		result["scheduler"] = string(StatusStopped)
	}

	// Add all job statuses
	for job, status := range jst.jobStatuses {
		result[job] = string(status)
	}

	return result
}

// isHealthy returns true if the system is healthy
// A healthy system has a running scheduler and no jobs in error state
func (jst *JobStatusTracker) isHealthy() bool {
	jst.mu.RLock()
	defer jst.mu.RUnlock()

	if !jst.isSchedulerRunning {
		return false
	}

	for _, status := range jst.jobStatuses {
		if status == StatusError {
			return false
		}
	}

	return true
}

// HealthCheckHandler handles health check requests
func (jst *JobStatusTracker) HealthCheckHandler(w http.ResponseWriter, r *http.Request) {
	// Set content type
	w.Header().Set("Content-Type", "application/json")

	// Determine HTTP status code based on health status
	if jst.isHealthy() {
		w.WriteHeader(http.StatusOK)
	} else {
		w.WriteHeader(http.StatusServiceUnavailable)
	}

	// Encode job statuses as JSON
	statuses := jst.GetAllStatuses()
	if err := json.NewEncoder(w).Encode(statuses); err != nil {
		w.WriteHeader(http.StatusInternalServerError)
		json.NewEncoder(w).Encode(map[string]string{
			"error": "Failed to encode job statuses",
		})
	}
}

// RegisterJobStatusUpdate registers a job status update function with a scheduler
func RegisterJobStatusUpdate(js *scheduler.JobScheduler, jst *JobStatusTracker) {
	// Set scheduler as running
	jst.SetSchedulerRunning(true)

	// Register callback for job status updates
	js.RegisterStatusCallback(func(jobName string, status string, timestamp time.Time) {
		var jobStatus JobStatus

		// Map scheduler status to our status enum
		switch status {
		case scheduler.StatusRunning:
			jobStatus = StatusRunning
		case scheduler.StatusPending:
			jobStatus = StatusPending
		case scheduler.StatusError:
			jobStatus = StatusError
		case scheduler.StatusComplete:
			jobStatus = StatusComplete
		default:
			jobStatus = StatusPending
		}

		// Update job status in our tracker
		jst.UpdateJobStatus(jobName, jobStatus)
	})
}

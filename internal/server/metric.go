package server

import (
	"encoding/json"
	"net/http"
	"sync"
	"time"
)

// JobMetrics stores metrics for a job
type JobMetrics struct {
	LastRunDuration    time.Duration `json:"lastRunDuration"`
	AverageRunDuration time.Duration `json:"averageRunDuration"`
	TotalRuns          int           `json:"totalRuns"`
	SuccessfulRuns     int           `json:"successfulRuns"`
	FailedRuns         int           `json:"failedRuns"`
	LastRunTime        time.Time     `json:"lastRunTime"`
	TotalBackupSize    int64         `json:"totalBackupSize"`
	LastBackupSize     int64         `json:"lastBackupSize"`
}

// MetricsCollector collects metrics for jobs
type MetricsCollector struct {
	mu      sync.RWMutex
	metrics map[string]JobMetrics
}

// NewMetricsCollector creates a new metrics collector
func NewMetricsCollector() *MetricsCollector {
	return &MetricsCollector{
		metrics: make(map[string]JobMetrics),
	}
}

// UpdateJobMetrics updates metrics for a job run
func (mc *MetricsCollector) UpdateJobMetrics(jobName string, duration time.Duration, success bool, backupSize int64) {
	mc.mu.Lock()
	defer mc.mu.Unlock()

	// Get existing metrics or create new
	metrics, exists := mc.metrics[jobName]
	if !exists {
		metrics = JobMetrics{}
	}

	// Update metrics
	metrics.LastRunDuration = duration
	metrics.TotalRuns++
	metrics.LastRunTime = time.Now()
	metrics.LastBackupSize = backupSize
	metrics.TotalBackupSize += backupSize

	// Update success/failure counts
	if success {
		metrics.SuccessfulRuns++
	} else {
		metrics.FailedRuns++
	}

	// Calculate average run duration
	metrics.AverageRunDuration = time.Duration(
		(metrics.AverageRunDuration.Nanoseconds()*(int64(metrics.TotalRuns)-1) +
			duration.Nanoseconds()) / int64(metrics.TotalRuns))

	// Store updated metrics
	mc.metrics[jobName] = metrics
}

// GetJobMetrics returns metrics for a specific job
func (mc *MetricsCollector) GetJobMetrics(jobName string) (JobMetrics, bool) {
	mc.mu.RLock()
	defer mc.mu.RUnlock()

	metrics, exists := mc.metrics[jobName]
	return metrics, exists
}

// GetAllJobMetrics returns metrics for all jobs
func (mc *MetricsCollector) GetAllJobMetrics() map[string]JobMetrics {
	mc.mu.RLock()
	defer mc.mu.RUnlock()

	// Create a copy of the metrics map
	result := make(map[string]JobMetrics, len(mc.metrics))
	for job, metrics := range mc.metrics {
		result[job] = metrics
	}

	return result
}

// MetricsHandler handles requests for metrics
func (mc *MetricsCollector) MetricsHandler(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")

	metrics := mc.GetAllJobMetrics()
	if err := json.NewEncoder(w).Encode(metrics); err != nil {
		w.WriteHeader(http.StatusInternalServerError)
		json.NewEncoder(w).Encode(map[string]string{
			"error": "Failed to encode metrics",
		})
	}
}

package server

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"time"

	"github.com/username/backmeup/internal/scheduler"
)

// HTTPServer represents the HTTP server for BackMeUp
type HTTPServer struct {
	server           *http.Server
	statusTracker    *JobStatusTracker
	metricsCollector *MetricsCollector
}

// NewHTTPServer creates a new HTTP server
func NewHTTPServer(port int, jobScheduler *scheduler.JobScheduler) *HTTPServer {
	// Create a new status tracker
	statusTracker := NewJobStatusTracker()

	// Create a new metrics collector
	metricsCollector := NewMetricsCollector()

	// Register with the job scheduler to receive status updates
	RegisterJobStatusUpdate(jobScheduler, statusTracker)

	// Create a new HTTP server
	mux := http.NewServeMux()

	// Create the server
	srv := &HTTPServer{
		statusTracker:    statusTracker,
		metricsCollector: metricsCollector,
		server: &http.Server{
			Addr:         fmt.Sprintf(":%d", port),
			Handler:      mux,
			ReadTimeout:  15 * time.Second,
			WriteTimeout: 15 * time.Second,
			IdleTimeout:  60 * time.Second,
		},
	}

	// Register routes
	mux.HandleFunc("/health", statusTracker.HealthCheckHandler)
	mux.HandleFunc("/metrics", metricsCollector.MetricsHandler)

	return srv
}

// Start starts the HTTP server
func (s *HTTPServer) Start() error {
	log.Printf("Starting HTTP server on %s", s.server.Addr)
	return s.server.ListenAndServe()
}

// Shutdown gracefully shuts down the HTTP server
func (s *HTTPServer) Shutdown(ctx context.Context) error {
	log.Println("Shutting down HTTP server")
	s.statusTracker.SetSchedulerRunning(false)
	return s.server.Shutdown(ctx)
}

// UpdateJobStatus updates the status of a job manually
func (s *HTTPServer) UpdateJobStatus(jobName string, status string) {
	// Map the string status to our JobStatus type
	var jobStatus JobStatus
	switch status {
	case "RUNNING":
		jobStatus = StatusRunning
	case "PENDING":
		jobStatus = StatusPending
	case "ERROR":
		jobStatus = StatusError
	case "COMPLETE":
		jobStatus = StatusComplete
	case "STOPPED":
		jobStatus = StatusStopped
	default:
		jobStatus = StatusPending
	}

	s.statusTracker.UpdateJobStatus(jobName, jobStatus)
}

// GetHealthStatusJSON returns the current health status as a JSON string
func (s *HTTPServer) GetHealthStatusJSON() ([]byte, error) {
	statuses := s.statusTracker.GetAllStatuses()
	return json.Marshal(statuses)
}

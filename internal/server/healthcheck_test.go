package server

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/stretchr/testify/suite"
)

func TestHealthCheckHandler(t *testing.T) {
	suite.Run(t, new(HealthCheckTestSuite))
}

// HealthCheckTestSuite is a test suite for the health check handler
type HealthCheckTestSuite struct {
	suite.Suite
	tracker *JobStatusTracker
}

// SetupTest runs before each test
func (s *HealthCheckTestSuite) SetupTest() {
	s.tracker = NewJobStatusTracker()
}

// TestHealthySystem tests the health check handler with a healthy system
func (s *HealthCheckTestSuite) TestHealthySystem() {
	// Set scheduler status to running
	s.tracker.SetSchedulerRunning(true)

	// Add some job statuses
	s.tracker.UpdateJobStatus("job1", StatusRunning)
	s.tracker.UpdateJobStatus("job2", StatusPending)
	s.tracker.UpdateJobStatus("job3", StatusComplete)

	// Create a new request
	req := httptest.NewRequest("GET", "/health", nil)
	w := httptest.NewRecorder()

	// Call the handler
	s.tracker.HealthCheckHandler(w, req)

	// Check response status code
	s.Equal(http.StatusOK, w.Code)

	// Parse the response body
	var response map[string]string
	err := json.Unmarshal(w.Body.Bytes(), &response)
	s.NoError(err)

	// Check that scheduler status is included
	s.Equal(string(StatusRunning), response["scheduler"])

	// Check that job statuses are included
	expectedStatuses := map[string]string{
		"job1": string(StatusRunning),
		"job2": string(StatusPending),
		"job3": string(StatusComplete),
	}

	for job, expectedStatus := range expectedStatuses {
		s.Equal(expectedStatus, response[job])
	}
}

// TestUnhealthySystemWithErrorJob tests the health check handler with a job in error state
func (s *HealthCheckTestSuite) TestUnhealthySystemWithErrorJob() {
	// Set scheduler status to running
	s.tracker.SetSchedulerRunning(true)

	// Add some job statuses - including an error
	s.tracker.UpdateJobStatus("job1", StatusRunning)
	s.tracker.UpdateJobStatus("job2", StatusError)
	s.tracker.UpdateJobStatus("job3", StatusComplete)

	// Create a new request
	req := httptest.NewRequest("GET", "/health", nil)
	w := httptest.NewRecorder()

	// Call the handler
	s.tracker.HealthCheckHandler(w, req)

	// Check response status code - should be 503 due to error job
	s.Equal(http.StatusServiceUnavailable, w.Code)

	// Parse the response body
	var response map[string]string
	err := json.Unmarshal(w.Body.Bytes(), &response)
	s.NoError(err)

	// Check that the error job status is included
	s.Equal(string(StatusError), response["job2"])
}

// TestUnhealthySystemWithStoppedScheduler tests the health check handler with stopped scheduler
func (s *HealthCheckTestSuite) TestUnhealthySystemWithStoppedScheduler() {
	// Set scheduler status to stopped
	s.tracker.SetSchedulerRunning(false)

	// Add some job statuses
	s.tracker.UpdateJobStatus("job1", StatusRunning)
	s.tracker.UpdateJobStatus("job2", StatusPending)

	// Create a new request
	req := httptest.NewRequest("GET", "/health", nil)
	w := httptest.NewRecorder()

	// Call the handler
	s.tracker.HealthCheckHandler(w, req)

	// Check response status code - should be 503 due to stopped scheduler
	s.Equal(http.StatusServiceUnavailable, w.Code)

	// Parse the response body
	var response map[string]string
	err := json.Unmarshal(w.Body.Bytes(), &response)
	s.NoError(err)

	// Check that scheduler status is included
	s.Equal(string(StatusStopped), response["scheduler"])
}

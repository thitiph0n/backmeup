package main

import (
	"context"
	"flag"
	"fmt"
	"log"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/username/backmeup/internal/backup"
	"github.com/username/backmeup/internal/config"
	"github.com/username/backmeup/internal/scheduler"
	"github.com/username/backmeup/internal/server"
)

func main() {
	// Define command-line flags
	configPath := flag.String("config", "config.yml", "Path to configuration file")
	flag.Parse()

	// Load configuration
	cfg, err := config.LoadConfig(*configPath)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error loading config: %v\n", err)
		os.Exit(1)
	}

	// Validate configuration
	if err := cfg.Validate(); err != nil {
		fmt.Fprintf(os.Stderr, "Invalid configuration: %v\n", err)
		os.Exit(1)
	}

	log.Printf("Configuration loaded successfully!")

	// Create the job scheduler with storage configuration
	jobScheduler := scheduler.NewJobScheduler(cfg.Storage)

	// Add each job from the configuration
	for i, jobConfig := range cfg.Jobs {
		log.Printf("Configuring job #%d: %s (%s)", i+1, jobConfig.Name, jobConfig.Type)
		log.Printf("  Schedule: %s", jobConfig.Schedule)
		log.Printf("  Retention policy: Keep %d %s", jobConfig.RetentionPolicy.Value,
			jobConfig.RetentionPolicy.Type)

		// Create the appropriate backup executor
		executor, err := backup.CreateExecutor(jobConfig, cfg.Storage)
		if err != nil {
			log.Printf("Error creating executor for job %s: %v", jobConfig.Name, err)
			continue
		}

		// Add the job to the scheduler
		if err := jobScheduler.AddJob(jobConfig, executor); err != nil {
			log.Printf("Error adding job %s to scheduler: %v", jobConfig.Name, err)
			continue
		}

		log.Printf("Job %s added to scheduler successfully", jobConfig.Name)
	}

	// Start the scheduler
	jobScheduler.Start()
	log.Printf("Backup scheduler started.")

	// Variables for HTTP server
	var httpServer *server.HTTPServer
	var httpErrCh chan error

	// Check if HTTP server should be started
	if cfg.Server.Enabled {
		log.Printf("Starting HTTP server for health monitoring...")
		httpServer, httpErrCh = startHTTPServer(cfg, jobScheduler)
	} else {
		log.Printf("HTTP server disabled in config. Skipping...")
	}

	// Wait for termination signal or HTTP server error
	sigCh := make(chan os.Signal, 1)
	signal.Notify(sigCh, os.Interrupt, syscall.SIGTERM)

	// Block until we receive a signal or HTTP server error
	if cfg.Server.Enabled {
		select {
		case <-sigCh:
			log.Printf("Received termination signal...")
		case err := <-httpErrCh:
			log.Printf("HTTP server error: %v", err)
		}
	} else {
		// If HTTP server is disabled, just wait for the signal
		<-sigCh
		log.Printf("Received termination signal...")
	}

	log.Printf("Shutting down...")

	// Shutdown HTTP server gracefully if it's running
	if cfg.Server.Enabled && httpServer != nil {
		log.Printf("Shutting down HTTP server...")
		ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
		defer cancel()

		if err := httpServer.Shutdown(ctx); err != nil {
			log.Printf("HTTP server shutdown error: %v", err)
		}
	}

	// Stop the scheduler
	jobScheduler.Stop()
	log.Printf("Shutdown complete.")
}

// startHTTPServer starts the HTTP server for health checks and metrics
// It returns the server instance and an error channel that will receive any server errors
func startHTTPServer(cfg *config.Config, jobScheduler *scheduler.JobScheduler) (*server.HTTPServer, chan error) {
	// Create a new HTTP server
	httpServer := server.NewHTTPServer(cfg.Server.Port, jobScheduler)

	// Channel to receive errors from the HTTP server
	errChan := make(chan error, 1)

	// Start the HTTP server in a goroutine
	go func() {
		log.Printf("Starting HTTP server on port %d", cfg.Server.Port)
		if err := httpServer.Start(); err != nil {
			errChan <- err
		}
	}()

	// Return the server and error channel
	return httpServer, errChan
}

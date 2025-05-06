package main

import (
	"flag"
	"fmt"
	"log"
	"os"
	"os/signal"
	"syscall"

	"github.com/username/backmeup/internal/backup"
	"github.com/username/backmeup/internal/config"
	"github.com/username/backmeup/internal/scheduler"
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
	log.Printf("Backup scheduler started. Press Ctrl+C to exit.")

	// Wait for termination signal
	sigCh := make(chan os.Signal, 1)
	signal.Notify(sigCh, os.Interrupt, syscall.SIGTERM)
	<-sigCh

	log.Printf("Shutting down...")
	jobScheduler.Stop()
	log.Printf("Shutdown complete.")
}

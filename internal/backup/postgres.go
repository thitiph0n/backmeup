package backup

import (
	"context"
	"fmt"
	"os"
	"os/exec"

	"github.com/thitiph0n/backmeup/internal/config"
	"github.com/thitiph0n/backmeup/internal/storage"
	"github.com/thitiph0n/backmeup/internal/storage/localfs"
)

type PostgresExecutor struct {
	BaseExecutor
}

func NewPostgresExecutor(jobConfig config.JobConfig, store storage.Storage) (Executor, error) {
	if jobConfig.PostgresConfig == nil {
		return nil, fmt.Errorf("missing PostgreSQL configuration for job: %s", jobConfig.Name)
	}

	return &PostgresExecutor{
		BaseExecutor: BaseExecutor{
			Config:  jobConfig,
			Storage: store,
		},
	}, nil
}

func (p *PostgresExecutor) Execute(ctx context.Context) error {
	p.LogBackupInfo("Starting PostgreSQL backup")

	filename := localfs.GenerateFileName("pg_backup", ".sql")

	writer, err := p.Storage.NewWriter(p.Config.Name, filename)
	if err != nil {
		return fmt.Errorf("failed to prepare backup file: %w", err)
	}
	defer writer.Close()

	cmdArgs := []string{}

	host := p.Config.PostgresConfig.Host
	port := p.Config.PostgresConfig.Port
	user := p.Config.PostgresConfig.User
	password := p.Config.PostgresConfig.Password
	dbname := p.Config.PostgresConfig.Database

	cmdArgs = append(cmdArgs, "-h", host)

	if port != "" {
		cmdArgs = append(cmdArgs, "-p", port)
	} else {
		cmdArgs = append(cmdArgs, "-p", "5432")
	}

	if user != "" {
		cmdArgs = append(cmdArgs, "-U", user)
	}

	cmdArgs = append(cmdArgs, "-d", dbname)

	cmdArgs = append(cmdArgs,
		"--no-password",
		"--clean",
		"--if-exists",
		"--no-owner",
		"--compress=9",
	)

	for key, value := range p.Config.PostgresConfig.Options {
		if value == "" {
			cmdArgs = append(cmdArgs, fmt.Sprintf("--%s", key))
		} else {
			cmdArgs = append(cmdArgs, fmt.Sprintf("--%s=%s", key, value))
		}
	}

	env := os.Environ()
	if password != "" {
		env = append(env, fmt.Sprintf("PGPASSWORD=%s", password))
	}

	cmd := exec.CommandContext(ctx, "pg_dump", cmdArgs...)
	cmd.Env = env
	cmd.Stdout = writer
	cmd.Stderr = os.Stderr

	p.LogBackupInfo(fmt.Sprintf("Running pg_dump to %s", filename))
	if err := cmd.Run(); err != nil {
		return fmt.Errorf("pg_dump failed: %w", err)
	}

	p.LogBackupInfo(fmt.Sprintf("PostgreSQL backup completed successfully: %s", filename))

	return nil
}

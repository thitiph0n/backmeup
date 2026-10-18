package backup

import (
	"context"
	"fmt"
	"os"
	"os/exec"
	"strings"

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

	filename := localfs.GenerateFileName("pg_backup", dumpExtension(p.Config.PostgresConfig.Options))

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
	)

	if _, ok := p.Config.PostgresConfig.Options["compress"]; !ok {
		cmdArgs = append(cmdArgs, "--compress=9")
	}

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
	cmd.Stderr = os.Stderr

	p.LogBackupInfo(fmt.Sprintf("Running pg_dump to %s", filename))
	if err := p.dumpToFile(cmd, filename); err != nil {
		return err
	}

	p.LogBackupInfo(fmt.Sprintf("PostgreSQL backup completed successfully: %s", filename))

	return nil
}

func dumpExtension(options map[string]string) string {
	if config.IsCustomPostgresFormat(options["format"]) {
		return ".dump"
	}

	compress, ok := options["compress"]
	if !ok {
		return ".sql.gz"
	}
	switch method := strings.ToLower(compress); {
	case method == "0" || method == "none":
		return ".sql"
	case strings.HasPrefix(method, "lz4"):
		return ".sql.lz4"
	case strings.HasPrefix(method, "zstd"):
		return ".sql.zst"
	default:
		return ".sql.gz"
	}
}

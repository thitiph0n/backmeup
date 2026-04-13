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

type MySQLExecutor struct {
	BaseExecutor
}

func NewMySQLExecutor(jobConfig config.JobConfig, store storage.Storage) (Executor, error) {
	if jobConfig.MySQLConfig == nil {
		return nil, fmt.Errorf("missing MySQL configuration for job: %s", jobConfig.Name)
	}

	return &MySQLExecutor{
		BaseExecutor: BaseExecutor{
			Config:  jobConfig,
			Storage: store,
		},
	}, nil
}

func (m *MySQLExecutor) Execute(ctx context.Context) error {
	m.LogBackupInfo("Starting MySQL backup")

	filename := localfs.GenerateFileName("mysql_backup", ".sql")

	writer, err := m.Storage.NewWriter(m.Config.Name, filename)
	if err != nil {
		return fmt.Errorf("failed to prepare backup file: %w", err)
	}
	defer writer.Close()

	connStr := m.Config.MySQLConfig.ConnectionString

	parts := strings.Split(connStr, "/")
	if len(parts) < 2 {
		return fmt.Errorf("invalid MySQL connection string format")
	}
	dbName := parts[len(parts)-1]

	authParts := strings.Split(parts[0], "@")
	if len(authParts) < 2 {
		return fmt.Errorf("invalid MySQL connection string format")
	}

	hostPart := authParts[1]

	userPassPart := strings.TrimPrefix(authParts[0], "mysql://")
	userPassSplit := strings.Split(userPassPart, ":")
	if len(userPassSplit) < 2 {
		return fmt.Errorf("invalid MySQL connection string format")
	}
	user := userPassSplit[0]
	pass := userPassSplit[1]

	cmd := exec.CommandContext(ctx, "mysqldump",
		"--user="+user,
		"--password="+pass,
		"--host="+hostPart,
		"--databases", dbName,
		"--single-transaction",
		"--quick",
	)

	cmd.Stdout = writer
	cmd.Stderr = os.Stderr

	m.LogBackupInfo(fmt.Sprintf("Running mysqldump to %s", filename))
	if err := cmd.Run(); err != nil {
		return fmt.Errorf("mysqldump failed: %w", err)
	}

	m.LogBackupInfo(fmt.Sprintf("MySQL backup completed successfully: %s", filename))

	return nil
}

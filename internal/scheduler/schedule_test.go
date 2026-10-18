package scheduler

import (
	"context"
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/thitiph0n/backmeup/internal/config"
)

type noopExecutor struct{}

func (noopExecutor) Execute(context.Context) error { return nil }

func TestStart_SweepsStalePartialBackups(t *testing.T) {
	dir := t.TempDir()
	jobDir := filepath.Join(dir, "job")
	require.NoError(t, os.MkdirAll(filepath.Join(jobDir, "stale_dir.partial"), 0755))
	require.NoError(t, os.WriteFile(filepath.Join(jobDir, "stale.sql.partial"), []byte("x"), 0644))
	require.NoError(t, os.WriteFile(filepath.Join(jobDir, "good.sql"), []byte("x"), 0644))

	js := NewJobScheduler(config.StorageConfig{Local: config.LocalConfig{Directory: dir}})
	require.NoError(t, js.AddJob(config.JobConfig{Name: "job", Schedule: "0 0 1 1 *"}, noopExecutor{}))

	js.Start()
	defer js.Stop()

	entries, err := os.ReadDir(jobDir)
	require.NoError(t, err)
	require.Len(t, entries, 1)
	assert.Equal(t, "good.sql", entries[0].Name())
}

func TestStop_CancelsJobContext(t *testing.T) {
	js := NewJobScheduler(config.StorageConfig{Local: config.LocalConfig{Directory: t.TempDir()}})

	js.Stop()

	assert.Error(t, js.rootCtx.Err())
}

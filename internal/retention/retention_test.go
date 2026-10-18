package retention

import (
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/thitiph0n/backmeup/internal/config"
	"github.com/thitiph0n/backmeup/internal/storage/localfs"
)

func TestApplyRetentionPolicy_IgnoresPartialBackups(t *testing.T) {
	dir := t.TempDir()
	store := localfs.New(config.LocalConfig{Directory: dir})
	jobDir := filepath.Join(dir, "job")

	now := time.Now()
	for i, name := range []string{"old.sql", "mid.sql", "new.sql"} {
		w, err := store.NewWriter("job", name)
		require.NoError(t, err)
		require.NoError(t, w.Close())
		require.NoError(t, store.Commit("job", name))
		mtime := now.Add(time.Duration(i-3) * time.Hour)
		require.NoError(t, os.Chtimes(filepath.Join(jobDir, name), mtime, mtime))
	}
	crashed, err := store.NewWriter("job", "crashed.sql")
	require.NoError(t, err)
	defer crashed.Close()

	err = NewManager(store).ApplyRetentionPolicy(config.JobConfig{
		Name:            "job",
		RetentionPolicy: config.RetentionPolicy{Type: "count", Value: 2},
	})
	require.NoError(t, err)

	assert.NoFileExists(t, filepath.Join(jobDir, "old.sql"))
	assert.FileExists(t, filepath.Join(jobDir, "mid.sql"))
	assert.FileExists(t, filepath.Join(jobDir, "new.sql"))
	assert.FileExists(t, filepath.Join(jobDir, "crashed.sql.partial"))
}

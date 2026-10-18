package backup

import (
	"os"
	"os/exec"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/thitiph0n/backmeup/internal/config"
	"github.com/thitiph0n/backmeup/internal/storage/localfs"
)

func newBaseExecutor(t *testing.T) (*BaseExecutor, string) {
	t.Helper()
	dir := t.TempDir()
	return &BaseExecutor{
		Config:  config.JobConfig{Name: "job"},
		Storage: localfs.New(config.LocalConfig{Directory: dir}),
	}, dir
}

func TestDumpToFile_Success(t *testing.T) {
	b, dir := newBaseExecutor(t)

	err := b.dumpToFile(exec.Command("sh", "-c", "echo data"), "ok.sql")
	require.NoError(t, err)

	content, err := os.ReadFile(filepath.Join(dir, "job", "ok.sql"))
	require.NoError(t, err)
	assert.Equal(t, "data\n", string(content))
}

func TestDumpToFile_RemovesFileOnFailure(t *testing.T) {
	b, dir := newBaseExecutor(t)

	err := b.dumpToFile(exec.Command("sh", "-c", "echo partial; exit 1"), "bad.sql")
	require.ErrorContains(t, err, "sh failed")

	_, statErr := os.Stat(filepath.Join(dir, "job", "bad.sql"))
	assert.True(t, os.IsNotExist(statErr))
}

func TestDumpExtension(t *testing.T) {
	tests := map[string]string{
		"":          ".sql",
		"plain":     ".sql",
		"p":         ".sql",
		"custom":    ".dump",
		"c":         ".dump",
		"Custom":    ".dump",
		"tar":       ".tar",
		"t":         ".tar",
		"directory": ".sql",
	}
	for format, want := range tests {
		t.Run(format, func(t *testing.T) {
			assert.Equal(t, want, dumpExtension(format))
		})
	}
}

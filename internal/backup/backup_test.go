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

func TestDumpToFile_FailureLeavesNothingBehind(t *testing.T) {
	b, dir := newBaseExecutor(t)

	require.Error(t, b.dumpToFile(exec.Command("sh", "-c", "echo partial; exit 1"), "bad.sql"))

	entries, err := os.ReadDir(filepath.Join(dir, "job"))
	require.NoError(t, err)
	assert.Empty(t, entries)
}

func TestDumpToFile_InProgressFileIsPartial(t *testing.T) {
	b, dir := newBaseExecutor(t)
	marker := filepath.Join(t.TempDir(), "seen")
	script := "echo data; ls " + filepath.Join(dir, "job") + " > " + marker

	require.NoError(t, b.dumpToFile(exec.Command("sh", "-c", script), "run.sql"))

	seen, err := os.ReadFile(marker)
	require.NoError(t, err)
	assert.Equal(t, "run.sql.partial\n", string(seen))
}

func TestDumpExtension(t *testing.T) {
	tests := []struct {
		name    string
		options map[string]string
		want    string
	}{
		{"default is gzipped plain", nil, ".sql.gz"},
		{"plain", map[string]string{"format": "plain"}, ".sql.gz"},
		{"plain short", map[string]string{"format": "p"}, ".sql.gz"},
		{"plain explicit level", map[string]string{"compress": "5"}, ".sql.gz"},
		{"plain gzip method", map[string]string{"compress": "gzip:6"}, ".sql.gz"},
		{"plain uncompressed", map[string]string{"compress": "0"}, ".sql"},
		{"plain none", map[string]string{"compress": "none"}, ".sql"},
		{"plain lz4", map[string]string{"compress": "lz4"}, ".sql.lz4"},
		{"plain zstd", map[string]string{"compress": "zstd:3"}, ".sql.zst"},
		{"custom", map[string]string{"format": "custom"}, ".dump"},
		{"custom short", map[string]string{"format": "c"}, ".dump"},
		{"custom uppercase", map[string]string{"format": "Custom"}, ".dump"},
		{"custom ignores compress", map[string]string{"format": "c", "compress": "0"}, ".dump"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			assert.Equal(t, tt.want, dumpExtension(tt.options))
		})
	}
}

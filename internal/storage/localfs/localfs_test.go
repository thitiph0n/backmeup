package localfs

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/thitiph0n/backmeup/internal/config"
)

func newStorage(t *testing.T) (*Storage, string) {
	t.Helper()
	dir := t.TempDir()
	return New(config.LocalConfig{Directory: dir}), dir
}

func TestGenerateFileName(t *testing.T) {
	tests := []struct {
		prefix    string
		extension string
	}{
		{"pg_backup", ".sql"},
		{"mysql_backup", ".sql"},
		{"minio_backup", ""},
	}
	for _, tt := range tests {
		t.Run(tt.prefix, func(t *testing.T) {
			result := GenerateFileName(tt.prefix, tt.extension)
			assert.True(t, strings.HasPrefix(result, tt.prefix+"_"))
			assert.True(t, strings.HasSuffix(result, tt.extension))
			trimmed := strings.TrimSuffix(strings.TrimPrefix(result, tt.prefix+"_"), tt.extension)
			_, err := time.Parse("20060102-150405", trimmed)
			assert.NoError(t, err)
		})
	}
}

func TestNewWriter(t *testing.T) {
	s, dir := newStorage(t)

	w, err := s.NewWriter("myjob", "backup.sql")
	require.NoError(t, err)
	defer w.Close()

	_, err = os.Stat(filepath.Join(dir, "myjob", "backup.sql"))
	assert.NoError(t, err)

	_, err = w.Write([]byte("test data"))
	assert.NoError(t, err)
}

func TestNewWriter_Error(t *testing.T) {
	tmp := t.TempDir()
	readOnly := filepath.Join(tmp, "readonly")
	require.NoError(t, os.Mkdir(readOnly, 0555))

	s := New(config.LocalConfig{Directory: filepath.Join(readOnly, "storage")})
	_, err := s.NewWriter("job", "file.sql")
	require.Error(t, err)
}

func TestNewDir(t *testing.T) {
	s, dir := newStorage(t)

	result, err := s.NewDir("myjob", "minio_backup_20240101-120000")
	require.NoError(t, err)

	expected := filepath.Join(dir, "myjob", "minio_backup_20240101-120000")
	assert.Equal(t, expected, result)

	info, err := os.Stat(result)
	require.NoError(t, err)
	assert.True(t, info.IsDir())
}

func TestNewDir_Error(t *testing.T) {
	tmp := t.TempDir()
	readOnly := filepath.Join(tmp, "readonly")
	require.NoError(t, os.Mkdir(readOnly, 0555))

	s := New(config.LocalConfig{Directory: filepath.Join(readOnly, "storage")})
	_, err := s.NewDir("job", "backup_dir")
	require.Error(t, err)
}

func TestList_Empty(t *testing.T) {
	s, _ := newStorage(t)
	entries, err := s.List("nonexistent_job")
	require.NoError(t, err)
	assert.Nil(t, entries)
}

func TestList(t *testing.T) {
	s, _ := newStorage(t)

	w1, err := s.NewWriter("myjob", "pg_backup_20240101-120000.sql")
	require.NoError(t, err)
	w1.Close()

	w2, err := s.NewWriter("myjob", "pg_backup_20240102-120000.sql")
	require.NoError(t, err)
	w2.Close()

	entries, err := s.List("myjob")
	require.NoError(t, err)
	assert.Len(t, entries, 2)
	for _, e := range entries {
		assert.NotEmpty(t, e.Key)
		assert.False(t, e.ModTime.IsZero())
	}
}

func TestDelete_File(t *testing.T) {
	s, _ := newStorage(t)

	w, err := s.NewWriter("myjob", "backup.sql")
	require.NoError(t, err)
	w.Close()

	entries, err := s.List("myjob")
	require.NoError(t, err)
	require.Len(t, entries, 1)

	require.NoError(t, s.Delete(entries[0]))

	_, err = os.Stat(entries[0].Key)
	assert.True(t, os.IsNotExist(err))
}

func TestDelete_Dir(t *testing.T) {
	s, _ := newStorage(t)

	dirPath, err := s.NewDir("myjob", "minio_backup_20240101-120000")
	require.NoError(t, err)

	f, err := os.Create(filepath.Join(dirPath, "data.bin"))
	require.NoError(t, err)
	f.Close()

	entries, err := s.List("myjob")
	require.NoError(t, err)
	require.Len(t, entries, 1)

	require.NoError(t, s.Delete(entries[0]))

	_, err = os.Stat(entries[0].Key)
	assert.True(t, os.IsNotExist(err))
}
